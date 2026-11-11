package shared

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/jackchuka/dutix/internal/macos"
	"github.com/spf13/cobra"
)

type contextKey string

const cliContextKey contextKey = "cli-context"

// Context holds shared services and configuration for CLI commands
type Context struct {
	Bridge  macos.Bridge
	Planner *domain.Planner
	Applier *domain.Applier

	// Global flags
	DebugMode  bool
	QuietMode  bool
	OutputMode string

	// Result receives the data for the JSON or YAML output
	Result *Result

	started bool // Some output was printed, so the next section needs a gap
}

// Table reports whether output is human-readable rather than JSON or YAML
func (c *Context) Table() bool {
	return formatter.IsTable(c.OutputMode)
}

// Result holds the data a command produced for the JSON or YAML envelope
type Result struct {
	Data any
}

// Formatter returns the formatter for the output mode: tables on stdout, or
// data for the envelope
func (c *Context) Formatter() formatter.Formatter {
	if c.Table() {
		return formatter.NewTable(nil)
	}
	return formatter.NewData(c.SetData)
}

// SetData sets the data for the JSON or YAML envelope
func (c *Context) SetData(data any) {
	c.Result.Data = data
}

// Human reports whether to print human-readable messages: output is a table
// and --quiet is off. Warnings are printed either way, to stderr.
func (c *Context) Human() bool {
	return c.Table() && !c.QuietMode
}

// Section starts a block of output. Blocks are separated by one blank line,
// with none before the first or after the last.
func (c *Context) Section() {
	if !c.Human() {
		return
	}
	if c.started {
		fmt.Println()
	}
	c.started = true
}

// Warnf prints a warning to stderr as its own section
func (c *Context) Warnf(format string, a ...any) {
	c.Section()
	_, _ = fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", a...)
}

// GetContext retrieves the CLI context from a cobra command
func GetContext(cmd *cobra.Command) (*Context, error) {
	ctx, ok := cmd.Context().Value(cliContextKey).(*Context)
	if !ok {
		return nil, fmt.Errorf("CLI context not found")
	}
	return ctx, nil
}

// GetContextKey returns the context key for storing CLI context
func GetContextKey() contextKey {
	return cliContextKey
}
