package domain

import (
	"errors"
	"fmt"

	"github.com/jackchuka/dutix/internal/logger"
	"github.com/jackchuka/dutix/internal/macos"
)

// ProgressCallback is called before processing each item
type ProgressCallback func(index int, item PlanItem)

// Applier is responsible for executing plans
type Applier struct {
	bridge macos.Bridge
}

// NewApplier creates a new applier
func NewApplier(bridge macos.Bridge) *Applier {
	return &Applier{
		bridge: bridge,
	}
}

// Apply executes a plan and returns results
//
// The applier will:
//   - Execute items sequentially (not concurrently)
//   - Set extension-keyed items in one batch per app after the others, since
//     each batch restarts lsd and repeated restarts are throttled
//   - Call progress callback before each item
//   - Continue on failures (don't fail fast)
//   - Capture detailed results for each item
func (a *Applier) Apply(plan *Plan, progress ProgressCallback) ([]ApplyResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan cannot be nil")
	}

	if len(plan.Items) == 0 {
		return nil, fmt.Errorf("plan has no items")
	}

	results := make([]ApplyResult, len(plan.Items))
	byApp := make(map[string][]int)
	var apps []string

	for i, item := range plan.Items {
		if item.Status != StatusSkipped && item.Target.SetsByExtension() && item.DesiredApp != nil && item.DesiredApp.Path != "" {
			if _, ok := byApp[item.DesiredApp.Path]; !ok {
				apps = append(apps, item.DesiredApp.Path)
			}
			byApp[item.DesiredApp.Path] = append(byApp[item.DesiredApp.Path], i)
			continue
		}

		if progress != nil {
			progress(i, item)
		}
		results[i] = a.applyItem(item)
	}

	for _, appPath := range apps {
		a.applyExtensionBatch(plan, appPath, byApp[appPath], results, progress)
	}

	for i := range plan.Items {
		plan.Items[i].Status = results[i].Item.Status
	}

	return results, nil
}

// applyExtensionBatch sets every extension-keyed item for one app in a single bridge call
func (a *Applier) applyExtensionBatch(plan *Plan, appPath string, indexes []int, results []ApplyResult, progress ProgressCallback) {
	exts := make([]string, 0, len(indexes))
	for _, i := range indexes {
		if progress != nil {
			progress(i, plan.Items[i])
		}
		exts = append(exts, plan.Items[i].Target.Extension)
	}

	logger.Debug("Setting default for extensions", "extensions", exts, "app", appPath)
	err := a.bridge.SetDefaultForExtensions(appPath, exts)

	for _, i := range indexes {
		result := ApplyResult{Item: plan.Items[i]}
		if err != nil {
			result.Error = err
			result.Item.Status = StatusFailed
			result.Notes = fmt.Sprintf("Failed to apply: %v", err)
			logger.Error("Failed to apply item", err, "extension", plan.Items[i].Target.Extension, "app", appPath)
		} else {
			result.Applied = true
			result.Item.Status = StatusSuccess
			result.Notes = "Applied successfully"
		}
		results[i] = result
	}
}

// applyItem applies a single plan item
func (a *Applier) applyItem(item PlanItem) ApplyResult {
	logger.Info("Applying item",
		"kind", item.Target.Kind,
		"identifier", item.Target.Identifier,
		"app", item.DesiredApp,
	)

	result := ApplyResult{
		Item:     item,
		Applied:  false,
		Verified: false,
	}

	// Skip if already skipped
	if item.Status == StatusSkipped {
		result.Item.Status = StatusSkipped
		result.Notes = "Skipped (no-op)"
		logger.Info("Item skipped (no-op)", "identifier", item.Target.Identifier)
		return result
	}

	// Mark as in progress
	result.Item.Status = StatusInProgress

	// Validate inputs
	if item.DesiredApp == nil || item.DesiredApp.Path == "" {
		result.Error = fmt.Errorf("invalid desired app")
		result.Item.Status = StatusFailed
		logger.Error("Invalid desired app", result.Error, "identifier", item.Target.Identifier)
		return result
	}

	// Apply based on target kind
	var err error
	switch item.Target.Kind {
	case TargetKindUTI:
		logger.Debug("Setting default for UTI",
			"uti", item.Target.Identifier,
			"app", item.DesiredApp.Path,
		)
		err = a.bridge.SetDefaultForUTI(item.DesiredApp.Path, item.Target.Identifier)
	case TargetKindScheme:
		logger.Debug("Setting default for scheme",
			"scheme", item.Target.Identifier,
			"app", item.DesiredApp.Path,
		)
		err = a.bridge.SetDefaultForScheme(item.DesiredApp.Path, item.Target.Identifier)
	default:
		err = fmt.Errorf("unsupported target kind: %s", item.Target.Kind)
	}

	if errors.Is(err, macos.ErrUserDeclined) {
		result.Declined = true
		result.Item.Status = StatusSkipped
		result.Notes = "Kept the current application in the confirmation dialog"
		logger.Info("Item declined by user", "identifier", item.Target.Identifier)
		return result
	}

	if err != nil {
		result.Error = err
		result.Item.Status = StatusFailed
		result.Notes = fmt.Sprintf("Failed to apply: %v", err)
		logger.Error("Failed to apply item", err,
			"kind", item.Target.Kind,
			"identifier", item.Target.Identifier,
			"app", item.DesiredApp.Path,
		)
		return result
	}

	// Success
	result.Applied = true
	result.Item.Status = StatusSuccess
	result.Notes = "Applied successfully"
	logger.Info("Item applied successfully",
		"identifier", item.Target.Identifier,
		"app", item.DesiredApp.Path,
	)

	return result
}
