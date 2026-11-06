package formatter

import (
	"bytes"
	"fmt"
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
	tbl.WithWriter(trimWriter{w})
	return tbl
}

// trimWriter drops trailing spaces from each line. The table library pads
// every column, including the last, to the column's width.
type trimWriter struct {
	w io.Writer
}

func (t trimWriter) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p))
	for _, line := range bytes.SplitAfter(p, []byte("\n")) {
		if body, ok := bytes.CutSuffix(line, []byte("\n")); ok {
			out = append(out, bytes.TrimRight(body, " ")...)
			out = append(out, '\n')
		} else {
			out = append(out, line...)
		}
	}
	if _, err := t.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Formatter defines the interface for output formatting
type Formatter interface {
	FormatApps(apps []domain.App) error
	FormatPlan(plan *domain.Plan) error
	FormatAppDetails(
		app domain.App,
		supportedTypes []domain.DocumentType,
		defaultTypes []domain.DocumentType,
		copies []domain.AppCopy,
	) error
	FormatTargetDetails(
		target domain.Target,
		defaultApp string,
		resolvedUTIs []string,
		availableApps []string,
	) error
	FormatExtensionDefaults(rows []domain.ExtensionDefault) error
}

// IsTable reports whether format is the human-readable table format, which is
// also the fallback for unknown formats
func IsTable(format string) bool {
	switch strings.ToLower(format) {
	case "json", "yaml":
		return false
	default:
		return true
	}
}

// Encode writes data as JSON or YAML, for commands whose output has no table
// form
func Encode(format string, w io.Writer, data any) error {
	if w == nil {
		w = os.Stdout
	}
	switch strings.ToLower(format) {
	case "json":
		return (&JSONFormatter{w: w}).encodeData(data)
	case "yaml":
		return (&YAMLFormatter{w: w}).encodeData(data)
	default:
		return fmt.Errorf("unsupported output format for encoding: %s", format)
	}
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
