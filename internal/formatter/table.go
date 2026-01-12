package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/rodaine/table"
)

// TableFormatter formats output as human-readable tables
type TableFormatter struct {
	w io.Writer
}

// Helper: format app name for display
func formatAppName(app *domain.App, theme Theme) string {
	if app == nil {
		return theme.Faint("-")
	}
	return app.Name
}

// Helper: build default types lookup set
func buildDefaultSet(defaults []domain.DocumentType) map[string]struct{} {
	defaultSet := make(map[string]struct{})
	for _, dt := range defaults {
		defaultSet[dt.Name] = struct{}{}
	}
	return defaultSet
}

// Helper: extract app name from path
func extractAppName(path string) string {
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	return strings.TrimSuffix(name, ".app")
}

func (f *TableFormatter) FormatApps(apps []domain.App) error {
	if len(apps) == 0 {
		_, _ = fmt.Fprintln(f.w, "No applications found.")
		return nil
	}

	theme := defaultTheme()
	tbl := theme.newStyledTable(f.w, "Name", "Bundle ID", "Path")

	for _, app := range apps {
		bundleID := app.BundleID
		if bundleID == "" {
			bundleID = theme.Faint("-")
		}
		tbl.AddRow(app.Name, bundleID, app.Path)
	}

	tbl.Print()
	_, _ = fmt.Fprintf(f.w, "\n%s\n", theme.Faint("Total: %d applications", len(apps)))
	return nil
}

func (f *TableFormatter) FormatPlan(plan *domain.Plan) error {
	if plan == nil || len(plan.Items) == 0 {
		_, _ = fmt.Fprintln(f.w, "No plan items.")
		return nil
	}

	theme := defaultTheme()
	tbl := theme.newStyledTable(f.w, "Target", "Extension", "Current", "Desired", "Status")

	warnings := f.addPlanRows(tbl, plan.Items, theme)
	tbl.Print()

	f.printWarnings(warnings, theme)
	f.printStats(plan, theme)

	return nil
}

// Helper: add plan rows to table and collect warnings
func (f *TableFormatter) addPlanRows(tbl table.Table, items []domain.PlanItem, theme Theme) []string {
	var warnings []string
	for _, item := range items {
		targetStr := fmt.Sprintf("%s:%s", item.Target.Kind, item.Target.Identifier)
		currentApp := formatAppName(item.CurrentApp, theme)
		desiredApp := formatAppName(item.DesiredApp, theme)
		statusStr := theme.formatStatus(item.Status)

		tbl.AddRow(targetStr, item.Target.Extension, currentApp, desiredApp, statusStr)

		if item.Warning != "" {
			warnings = append(warnings, fmt.Sprintf("%s: %s", targetStr, item.Warning))
		}
	}
	return warnings
}

// Helper: print warnings if any exist
func (f *TableFormatter) printWarnings(warnings []string, theme Theme) {
	if len(warnings) == 0 {
		return
	}

	_, _ = fmt.Fprintf(f.w, "\n%s\n", theme.Warning("Warnings:"))
	for _, warning := range warnings {
		_, _ = fmt.Fprintf(f.w, "  ⚠  %s\n", warning)
	}
}

// Helper: print plan statistics
func (f *TableFormatter) printStats(plan *domain.Plan, theme Theme) {
	pending, success, failed, skipped := plan.Stats()
	stats := fmt.Sprintf("Stats: %d pending, %d success, %d failed, %d skipped",
		pending, success, failed, skipped)
	_, _ = fmt.Fprintf(f.w, "\n%s\n", theme.Faint(stats))
}

func (f *TableFormatter) FormatAppDetails(
	app domain.App,
	supportedTypes []domain.DocumentType,
	defaultTypes []domain.DocumentType,
) error {
	theme := defaultTheme()

	f.printAppInfo(app, theme)
	_, _ = fmt.Fprintln(f.w)
	f.printDocumentTypes(supportedTypes, defaultTypes, theme)

	return nil
}

// Helper: print app info table
func (f *TableFormatter) printAppInfo(app domain.App, theme Theme) {
	infoTbl := theme.newStyledTable(f.w, "Property", "Value")
	infoTbl.AddRow("Name", app.Name)
	infoTbl.AddRow("Path", app.Path)
	infoTbl.AddRow("Bundle ID", app.BundleID)
	infoTbl.Print()
}

// Helper: print document types table
func (f *TableFormatter) printDocumentTypes(
	supportedTypes []domain.DocumentType,
	defaultTypes []domain.DocumentType,
	theme Theme,
) {
	tbl := theme.newStyledTable(f.w, "Document Type", "Extensions", "Primary UTI", "Is Default")

	defaultSet := buildDefaultSet(defaultTypes)

	for _, dt := range supportedTypes {
		// Format extensions as comma-separated list
		exts := formatExtensions(dt.Extensions)

		// Get primary (first) UTI
		primaryUTI := ""
		if len(dt.UTIs) > 0 {
			primaryUTI = dt.UTIs[0]
			// Add indicator if there are more UTIs
			if len(dt.UTIs) > 1 {
				primaryUTI = fmt.Sprintf("%s (+%d)", primaryUTI, len(dt.UTIs)-1)
			}
		}

		// Check if default
		isDefault := ""
		if _, exists := defaultSet[dt.Name]; exists {
			isDefault = theme.Success("✓")
		}

		tbl.AddRow(dt.Name, exts, primaryUTI, isDefault)
	}
	tbl.Print()
}

// Helper: format extensions as comma-separated list
func formatExtensions(extensions []string) string {
	if len(extensions) == 0 {
		return "-"
	}

	// Show up to 5 extensions, then add "..."
	if len(extensions) <= 5 {
		return strings.Join(extensions, ", ")
	}

	return strings.Join(extensions[:5], ", ") + fmt.Sprintf(" (+%d more)", len(extensions)-5)
}

func (f *TableFormatter) FormatTargetDetails(
	target domain.Target,
	defaultApp string,
	resolvedUTIs []string,
	availableApps []string,
) error {
	theme := defaultTheme()

	f.printTargetInfo(target, defaultApp, resolvedUTIs, theme)

	if len(availableApps) > 0 {
		_, _ = fmt.Fprintln(f.w)
		f.printAvailableApps(availableApps, defaultApp, theme)
	}

	return nil
}

// Helper: print target info table
func (f *TableFormatter) printTargetInfo(
	target domain.Target,
	defaultApp string,
	resolvedUTIs []string,
	theme Theme,
) {
	infoTbl := theme.newStyledTable(f.w, "Property", "Value")
	infoTbl.AddRow("Kind", target.Kind)
	infoTbl.AddRow("Identifier", target.Identifier)

	// Add resolved UTIs if any
	if len(resolvedUTIs) > 0 {
		infoTbl.AddRow("Resolved UTI", resolvedUTIs[0])
		for i := 1; i < len(resolvedUTIs); i++ {
			infoTbl.AddRow("", resolvedUTIs[i])
		}
	}

	// Add default app
	if defaultApp != "" {
		infoTbl.AddRow("Current Default", extractAppName(defaultApp))
	} else {
		infoTbl.AddRow("Current Default", theme.Faint("(none set)"))
	}

	infoTbl.Print()
}

// Helper: print available apps table
func (f *TableFormatter) printAvailableApps(availableApps []string, defaultApp string, theme Theme) {
	appTbl := theme.newStyledTable(f.w, "Current", "Application", "Path")

	for _, appPath := range availableApps {
		appName := extractAppName(appPath)
		if appPath == defaultApp {
			appTbl.AddRow("✓", appName, appPath)
		} else {
			appTbl.AddRow("", appName, appPath)
		}
	}

	appTbl.Print()
}
