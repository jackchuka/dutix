package domain

import (
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
// - Execute items sequentially (not concurrently)
// - Call progress callback before each item
// - Continue on failures (don't fail fast)
// - Capture detailed results for each item
func (a *Applier) Apply(plan *Plan, progress ProgressCallback) ([]ApplyResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("plan cannot be nil")
	}

	if len(plan.Items) == 0 {
		return nil, fmt.Errorf("plan has no items")
	}

	results := make([]ApplyResult, len(plan.Items))

	for i, item := range plan.Items {
		// Call progress callback if provided
		if progress != nil {
			progress(i, item)
		}

		// Apply the item
		result := a.applyItem(item)
		results[i] = result

		// Update plan item status in place
		plan.Items[i].Status = result.Item.Status
	}

	return results, nil
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
