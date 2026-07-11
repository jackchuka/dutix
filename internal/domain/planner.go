package domain

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jackchuka/dutix/internal/macos"
)

// systemUTIsToSkip contains UTIs that should not be migrated
// These are typically system/internal UTIs that aren't meant for file type associations
var systemUTIsToSkip = map[string]bool{
	"com.apple.file-system-plug-in":       true,
	"com.apple.application-file":          true,
	"com.apple.application-bundle":        true,
	"com.apple.systempreference.prefpane": true,
	"com.apple.plugin":                    true,
	"com.apple.bundle":                    true,
	"public.html":                         true, // Fails with NSCocoaErrorDomain 256
}

// shouldSkipUTI returns true if the UTI should be skipped during migration
func shouldSkipUTI(uti string) bool {
	// Check exact match
	if systemUTIsToSkip[uti] {
		return true
	}

	// Skip generic system UTIs
	if strings.HasPrefix(uti, "com.apple.system") {
		return true
	}

	return false
}

// isDynamicUTI reports whether a UTI is a dynamically-generated identifier.
// macOS synthesizes "dyn.*" UTIs for file extensions that no installed
// application declares. A default handler cannot be registered for a dynamic
// UTI (NSWorkspace returns NSCocoaErrorDomain code 256), so these must not be
// applied.
func isDynamicUTI(uti string) bool {
	return strings.HasPrefix(uti, "dyn.")
}

// Planner is responsible for building execution plans
type Planner struct {
	bridge macos.Bridge
}

// NewPlanner creates a new planner
func NewPlanner(bridge macos.Bridge) *Planner {
	return &Planner{
		bridge: bridge,
	}
}

var (
	ErrPlanNoTargets    = fmt.Errorf("no targets provided")
	ErrPlanNoValidItems = fmt.Errorf("no valid plan items created")
)

// BuildPlan creates a plan from a desired app and list of targets
//
// The plan will:
// - Expand extension targets to UTI targets
// - Query current defaults for all targets
// - Compare current vs desired
// - Filter out no-op changes
// - Add warnings for problematic targets
func (p *Planner) BuildPlan(desiredApp *App, targets []Target) (*Plan, error) {
	if desiredApp == nil {
		return nil, fmt.Errorf("desired app cannot be nil")
	}

	if len(targets) == 0 {
		return nil, ErrPlanNoTargets
	}

	plan := NewPlan()

	// Process each target
	for _, target := range targets {
		// Resolve extension targets to UTI targets
		resolvedTargets, err := p.resolveTarget(target)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve target %s: %w", target.Identifier, err)
		}

		// Create plan items for each resolved target
		for _, resolvedTarget := range resolvedTargets {
			item, err := p.createPlanItem(desiredApp, resolvedTarget)
			if err != nil {
				// Log the error but continue with other items
				// In a real implementation, we might want to collect errors
				continue
			}

			// Skip no-op items
			if item.IsNoOp() {
				item.Status = StatusSkipped
			}

			// Add warnings for problematic targets
			p.addWarnings(&item)

			// Dynamic UTIs cannot be assigned a default handler; skip them
			// with a clear explanation instead of failing at apply time.
			if resolvedTarget.Kind == TargetKindUTI && isDynamicUTI(resolvedTarget.Identifier) {
				item.Status = StatusSkipped
				name := resolvedTarget.Identifier
				if resolvedTarget.Extension != "" {
					name = "." + resolvedTarget.Extension
				}
				item.Warning = fmt.Sprintf("%s is not registered by any application (dynamic UTI); no default can be set", name)
			}

			plan.AddItem(item)
		}
	}

	if len(plan.Items) == 0 {
		return nil, ErrPlanNoValidItems
	}

	return plan, nil
}

// resolveTarget expands extension targets to UTI targets
// For UTI and scheme targets, returns the target as-is
func (p *Planner) resolveTarget(target Target) ([]Target, error) {
	switch target.Kind {
	case TargetKindExtension:
		// Resolve extension to UTIs
		utis, err := p.bridge.ResolveUTIsForExtension(target.Identifier)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve extension: %w", err)
		}

		if len(utis) == 0 {
			return nil, fmt.Errorf("no UTIs found for extension: %s", target.Identifier)
		}

		// Create a UTI target for each resolved UTI, filtering out system UTIs
		var resolvedTargets []Target
		for _, uti := range utis {
			// Skip system UTIs that shouldn't be migrated
			if shouldSkipUTI(uti) {
				continue
			}

			resolvedTargets = append(resolvedTargets, Target{
				Kind:         TargetKindUTI,
				Identifier:   uti,
				Extension:    target.Extension, // preserve original extension as name
				ResolvedUTIs: []string{uti},
			})
		}

		return resolvedTargets, nil

	case TargetKindUTI, TargetKindScheme:
		// Already resolved
		return []Target{target}, nil

	default:
		return nil, fmt.Errorf("unknown target kind: %s", target.Kind)
	}
}

// createPlanItem creates a plan item for a target by querying the current default
func (p *Planner) createPlanItem(desiredApp *App, target Target) (PlanItem, error) {
	item := PlanItem{
		Target:     target,
		DesiredApp: desiredApp,
		Status:     StatusPending,
	}

	// Query current default
	var currentAppPath string
	var err error

	switch target.Kind {
	case TargetKindUTI:
		currentAppPath, err = p.bridge.GetDefaultAppForUTI(target.Identifier)
	case TargetKindScheme:
		currentAppPath, err = p.bridge.GetDefaultAppForScheme(target.Identifier)
	default:
		return item, fmt.Errorf("unsupported target kind: %s", target.Kind)
	}

	// If we can get the current app, populate it
	if err == nil && currentAppPath != "" {
		item.CurrentApp = &App{
			Path: currentAppPath,
			Name: extractAppName(currentAppPath),
		}
	}
	// If there's no current app or we can't determine it, that's okay
	// (item.CurrentApp will be nil)

	return item, nil
}

// addWarnings adds warnings to a plan item for known issues
func (p *Planner) addWarnings(item *PlanItem) {
	if item.Target.NeedsUserConfirmation() {
		item.Warning = fmt.Sprintf("Setting default for %s may require user confirmation", item.Target.Identifier)
	}

	// Warn if extension resolved to multiple UTIs (if we had that info)
	if len(item.Target.ResolvedUTIs) > 1 {
		item.Warning = fmt.Sprintf("Extension maps to multiple UTIs: %v", item.Target.ResolvedUTIs)
	}
}

// extractAppName extracts the application name from a path
// e.g., "/Applications/Visual Studio Code.app" -> "Visual Studio Code"
func extractAppName(path string) string {
	base := filepath.Base(path)
	// Remove .app extension
	if len(base) > 4 && base[len(base)-4:] == ".app" {
		return base[:len(base)-4]
	}
	return base
}
