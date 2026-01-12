package apps

import (
	"fmt"
	"strings"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	var dryRun bool
	var skipConfirmation bool

	cmd := &cobra.Command{
		Use:   "migrate <from-app> <to-app>",
		Short: "Migrate file type associations from one app to another",
		Long: `Finds all file types currently handled by the source app and sets the target app
as the default handler for those types.

This is useful when switching to a new application and wanting to transfer all
file associations automatically.

Examples:
  dutix apps migrate "TextEdit" "Visual Studio Code"
  dutix apps migrate Safari Chrome --dry-run
  dutix apps migrate "Old App" "New App" --yes`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			fromAppQuery := args[0]
			toAppQuery := args[1]

			// List all applications
			macosApps, err := ctx.Bridge.ListAllApplications()
			if err != nil {
				return fmt.Errorf("failed to list applications: %w", err)
			}

			// Convert to domain apps
			apps := domain.FromMacOSAppInfos(macosApps)

			// Find source app
			fromApp, err := domain.FindApp(apps, fromAppQuery)
			if err != nil {
				return err
			}

			// Find target app
			toApp, err := domain.FindApp(apps, toAppQuery)
			if err != nil {
				return err
			}
			// Get document types supported by source app
			fromDocTypes, err := ctx.Bridge.ListSupportedDocumentTypes(fromApp.Path)
			if err != nil {
				return fmt.Errorf("failed to get document types for %s: %w", fromApp.Name, err)
			}

			if len(fromDocTypes) == 0 {
				if !ctx.QuietMode {
					fmt.Printf("%s does not declare any document type associations.\n", fromApp.Name)
				}
				return nil
			}

			// Only migrate extensions that target app actually supports
			var targets []domain.Target

			// ignore duplicate extensions
			seenExts := make(map[string]bool)
			for _, dt := range fromDocTypes {
				for _, ext := range dt.Extensions {
					if seenExts[ext] {
						continue
					}
					seenExts[ext] = true
					targets = append(targets, domain.Target{
						Kind:       domain.TargetKindExtension,
						Identifier: ext,
						Extension:  ext,
					})
				}
			}

			if len(targets) == 0 {
				if !ctx.QuietMode {
					fmt.Printf("No compatible extensions found between %s and %s.\n", fromApp.Name, toApp.Name)
				}
				return nil
			}

			// Build plan
			plan, err := ctx.Planner.BuildPlan(toApp, targets)
			if err != nil {
				// Check if all extensions were filtered out due to system UTIs
				if err == domain.ErrPlanNoValidItems {
					if !ctx.QuietMode {
						fmt.Printf("No file types to migrate from %s to %s.\n", fromApp.Name, toApp.Name)
						fmt.Println("Note: Some extensions may have been skipped because they resolve to system-internal UTIs.")
					}
					return nil
				}
				return fmt.Errorf("failed to build migration plan: %w", err)
			}

			// Display preview
			if !ctx.QuietMode {
				fmt.Printf("Migrating file associations from %s to %s\n\n", fromApp.Name, toApp.Name)
				f := formatter.New(ctx.OutputMode, nil)
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
			if !skipConfirmation && !ctx.QuietMode {
				fmt.Printf("Migrate %d file type(s) to %s? (y/N): ", len(plan.Items), toApp.Name)
				var response string
				_, err := fmt.Scanln(&response)
				if err != nil && err.Error() != "unexpected newline" {
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
				return fmt.Errorf("failed to apply migration: %w", err)
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

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&skipConfirmation, "yes", false, "Skip confirmation prompt")

	return cmd
}
