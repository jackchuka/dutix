package shared

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/domain"
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
