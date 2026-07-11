package formatter

import (
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/rodaine/table"
)

// Theme defines consistent styling across all formatters
type Theme struct {
	Header      func(format string, a ...any) string
	FirstColumn func(format string, a ...any) string
	Label       func(format string, a ...any) string
	Success     func(format string, a ...any) string
	Warning     func(format string, a ...any) string
	Error       func(format string, a ...any) string
	Faint       func(format string, a ...any) string
}

// defaultTheme returns the default color theme
func defaultTheme() Theme {
	return Theme{
		Header:      color.New(color.FgCyan, color.Underline).SprintfFunc(),
		FirstColumn: color.New(color.FgYellow).SprintfFunc(),
		Label:       color.New(color.FgYellow).SprintfFunc(),
		Success:     color.New(color.FgGreen).SprintfFunc(),
		Warning:     color.New(color.FgYellow).SprintfFunc(),
		Error:       color.New(color.FgRed).SprintfFunc(),
		Faint:       color.New(color.Faint).SprintfFunc(),
	}
}

// formatStatus formats a plan item status with appropriate color and icon
func (t Theme) formatStatus(status domain.PlanItemStatus) string {
	switch status {
	case domain.StatusSuccess:
		return t.Success("✓ %s", status)
	case domain.StatusSkipped:
		return t.Faint("%s", status)
	case domain.StatusPending:
		return t.Warning("→ %s", status)
	case domain.StatusFailed:
		return t.Error("✗ %s", status)
	default:
		return string(status)
	}
}

// newStyledTable creates a table with standard theme styling
func (t Theme) newStyledTable(w io.Writer, headers ...any) table.Table {
	tbl := table.New(headers...)
	tbl.WithHeaderFormatter(t.Header).WithFirstColumnFormatter(t.FirstColumn)
	tbl.WithWriter(w)
	return tbl
}

// Formatter defines the interface for output formatting
type Formatter interface {
	FormatApps(apps []domain.App) error
	FormatPlan(plan *domain.Plan) error
	FormatAppDetails(
		app domain.App,
		supportedTypes []domain.DocumentType,
		defaultTypes []domain.DocumentType,
	) error
	FormatTargetDetails(
		target domain.Target,
		defaultApp string,
		resolvedUTIs []string,
		availableApps []string,
	) error
	FormatExtensionDefaults(rows []domain.ExtensionDefault) error
}

// New creates a formatter based on the specified format type
func New(format string, w io.Writer) Formatter {
	if w == nil {
		w = os.Stdout
	}

	switch strings.ToLower(format) {
	case "json":
		return &JSONFormatter{w: w}
	case "yaml":
		return &YAMLFormatter{w: w}
	default:
		return &TableFormatter{w: w}
	}
}
