package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jackchuka/dutix/internal/cli/apps"
	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/cli/targets"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/jackchuka/dutix/internal/logger"
	"github.com/jackchuka/dutix/internal/macos"
	"github.com/spf13/cobra"
)

// Run runs dutix with args and returns the exit code. With --output json or
// yaml, it prints one envelope to stdout holding the command's data, its
// error, or both; errors also go to stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	result := &shared.Result{}
	root := newRootCommand(result)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	cmd, err := root.ExecuteC()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
	}

	output, _ := root.PersistentFlags().GetString("output")
	if formatter.IsTable(output) || (err == nil && result.Data == nil) {
		return shared.ExitCode(err)
	}

	env := formatter.Envelope{
		APIVersion: formatter.APIVersion,
		Command:    strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name())),
		Data:       result.Data,
	}
	if err != nil {
		env.Error = shared.ErrorInfo(err)
	}
	if werr := formatter.WriteEnvelope(output, stdout, env); werr != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", werr)
		if err == nil {
			return shared.ExitError
		}
	}
	return shared.ExitCode(err)
}

func newRootCommand(result *shared.Result) *cobra.Command {
	var debugMode bool
	var quietMode bool
	var outputMode string

	rootCmd := &cobra.Command{
		Use:   "dutix",
		Short: "Default UTI eXtension handler manager for macOS",
		Long: `dutix is a CLI tool for managing default application handlers
for file types, UTIs, and URL schemes on macOS.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(outputMode) {
			case "table", "json", "yaml":
			default:
				return shared.Usagef("invalid --output %q: use table, json, or yaml", outputMode)
			}

			// Initialize logger
			if err := logger.Init(debugMode); err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			// Initialize CLI context with services
			ctx := &shared.Context{
				Bridge:     macos.NewBridge(),
				DebugMode:  debugMode,
				QuietMode:  quietMode,
				OutputMode: strings.ToLower(outputMode),
				Result:     result,
			}
			ctx.Planner = domain.NewPlanner(ctx.Bridge)
			ctx.Applier = domain.NewApplier(ctx.Bridge)

			// Store in command context for subcommands to access
			cmd.SetContext(context.WithValue(cmd.Context(), shared.GetContextKey(), ctx))

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default action when no subcommand is specified: Show help
			return cmd.Help()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().StringVarP(&outputMode, "output", "o", "table", "Output format: table, json, yaml")

	// Add subcommands
	rootCmd.AddCommand(apps.NewCommand())
	rootCmd.AddCommand(targets.NewCommand())
	rootCmd.AddCommand(NewSetCommand())
	rootCmd.AddCommand(NewExportCommand())
	rootCmd.AddCommand(NewApplyCommand())
	rootCmd.AddCommand(NewVersionCommand())

	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &shared.UsageError{Err: err}
	})
	markArgErrorsAsUsage(rootCmd)

	return rootCmd
}

// markArgErrorsAsUsage makes argument validation errors of cmd and its
// subcommands UsageErrors. Commands without an Args validator get NoArgs, and
// command groups print their help when run, so an unknown subcommand is
// reported the same way: cobra skips argument validation for commands that
// can't run.
func markArgErrorsAsUsage(cmd *cobra.Command) {
	if !cmd.Runnable() {
		cmd.RunE = func(c *cobra.Command, _ []string) error {
			return c.Help()
		}
	}
	validate := cmd.Args
	if validate == nil {
		validate = cobra.NoArgs
	}
	cmd.Args = func(c *cobra.Command, args []string) error {
		if err := validate(c, args); err != nil {
			return &shared.UsageError{Err: err}
		}
		return nil
	}
	for _, sub := range cmd.Commands() {
		markArgErrorsAsUsage(sub)
	}
}
