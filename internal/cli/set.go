package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

// NewSetCommand creates the set command
func NewSetCommand() *cobra.Command {
	var extensions []string
	var utis []string
	var schemes []string
	var dryRun bool
	var skipConfirmation bool

	cmd := &cobra.Command{
		Use:   "set <app>",
		Short: "Set default application for targets",
		Long: `Sets the default application for specified file extensions, UTIs, or URL schemes.

You can specify targets using:
  --extensions: File extensions (e.g., txt,md,json)
  --utis: Direct UTI identifiers (e.g., public.plain-text)
  --schemes: URL schemes (e.g., http,https)

Examples:
  dutix set "Visual Studio Code" --extensions txt,md,json
  dutix set Safari --schemes http,https
  dutix set TextEdit --extensions txt --dry-run
  dutix set Finder --extensions zip,tar --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			appName := args[0]

			// Parse target specifications
			targets, err := shared.ParseTargetSpecs(extensions, utis, schemes)
			if err != nil {
				return err
			}

			if len(targets) == 0 {
				return fmt.Errorf("no targets specified: use --extensions, --utis, or --schemes")
			}

			// Find the application
			apps, err := ctx.Bridge.ListAllApplications()
			if err != nil {
				return fmt.Errorf("failed to scan applications: %w", err)
			}
			domainApps := domain.FromMacOSAppInfos(apps)

			// Find the application
			selectedApp, err := domain.FindApp(domainApps, appName)
			if err != nil {
				return err
			}

			// Build the plan
			plan, err := ctx.Planner.BuildPlan(selectedApp, targets)
			if err != nil {
				if err == domain.ErrPlanNoTargets || err == domain.ErrPlanNoValidItems {
					return err
				}
				return fmt.Errorf("failed to build plan: %w", err)
			}

			// Display preview
			if !ctx.QuietMode {
				fmt.Printf("Application: %s\n", selectedApp.Name)
				fmt.Printf("Path: %s\n", selectedApp.Path)
				fmt.Printf("Targets: %d\n\n", len(plan.Items))

				f := formatter.New("table", nil)
				if err := f.FormatPlan(plan); err != nil {
					return err
				}
				fmt.Println()
			}

			// Dry run - just preview
			if dryRun {
				fmt.Println("Dry run - no changes made.")
				return nil
			}

			// Confirm unless --yes flag
			if !skipConfirmation {
				fmt.Print("Apply these changes? (y/N): ")
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Apply the plan
			results, err := ctx.Applier.Apply(plan, nil)
			if err != nil {
				return fmt.Errorf("failed to apply changes: %w", err)
			}

			// Display results
			if !ctx.QuietMode {
				if err := shared.DisplayResults(results); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&extensions, "extensions", nil, "File extensions (comma-separated, e.g., txt,md,json)")
	cmd.Flags().StringSliceVar(&utis, "utis", nil, "UTI identifiers (comma-separated)")
	cmd.Flags().StringSliceVar(&schemes, "schemes", nil, "URL schemes (comma-separated, e.g., http,https)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&skipConfirmation, "yes", false, "Skip confirmation prompt")

	return cmd
}
