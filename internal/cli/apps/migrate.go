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
	var copyOpts shared.CopyOptions

	cmd := &cobra.Command{
		Use:   "migrate <from-app> <to-app>",
		Short: "Migrate file type associations from one app to another",
		Long: `Finds all file types currently handled by the source app and sets the target app
as the default handler for those types.

This is useful when switching to a new application and wanting to transfer all
file associations automatically.

Either app can be given as a path to an .app bundle, a bundle identifier, or a
name. <to-app> is registered with LaunchServices before applying if needed;
--prune-others also unregisters other copies of its bundle ID afterwards.

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

			// JSON and YAML output have no room for the prompt
			if !ctx.Table() && !skipConfirmation && !dryRun {
				return shared.Usagef("--output %s needs --yes or --dry-run", ctx.OutputMode)
			}

			// Find source app
			resolver := domain.NewResolver(ctx.Bridge)
			fromApp, err := resolver.Resolve(fromAppQuery)
			if err != nil {
				return err
			}

			// Find target app
			toApp, err := resolver.Resolve(toAppQuery)
			if err != nil {
				return err
			}

			// nothingToMigrate explains why there is nothing to do, or prints
			// an empty report for JSON or YAML output
			nothingToMigrate := func(lines ...string) error {
				if !ctx.Table() {
					ctx.SetData(domain.NewReport(toApp, nil, dryRun))
					return nil
				}
				if !ctx.QuietMode {
					for _, line := range lines {
						fmt.Println(line)
					}
				}
				return nil
			}

			// Get document types supported by source app
			fromDocTypes, err := ctx.Bridge.ListSupportedDocumentTypes(fromApp.Path)
			if err != nil {
				return fmt.Errorf("failed to get document types for %s: %w", fromApp.Name, err)
			}

			if len(fromDocTypes) == 0 {
				return nothingToMigrate(fmt.Sprintf("%s does not declare any document type associations.", fromApp.Name))
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
				return nothingToMigrate(fmt.Sprintf("No compatible extensions found between %s and %s.", fromApp.Name, toApp.Name))
			}

			// Build plan
			plan, err := ctx.Planner.BuildPlan(toApp, targets)
			if err != nil {
				// Check if all extensions were filtered out due to system UTIs
				if err == domain.ErrPlanNoValidItems {
					return nothingToMigrate(
						fmt.Sprintf("No file types to migrate from %s to %s.", fromApp.Name, toApp.Name),
						"Note: Some extensions may have been skipped because they resolve to system-internal UTIs.",
					)
				}
				return fmt.Errorf("failed to build migration plan: %w", err)
			}

			// Display preview. With --yes, the results table follows right
			// away, so the plan table is only shown for --dry-run or before
			// the prompt.
			if ctx.Human() {
				ctx.Section()
				fmt.Printf("Migrating file associations from %s to %s\n", fromApp.Name, toApp.Name)
				if plan.Settled() {
					fmt.Printf("%s already opens all %d file type(s).\n", toApp.Name, len(plan.Items))
				} else if dryRun || !skipConfirmation {
					ctx.Section()
					f := formatter.NewTable(nil)
					if err := f.FormatPlan(plan); err != nil {
						return err
					}
				}
			}

			copies := shared.PreviewCopies(ctx, toApp, copyOpts)
			shared.PrintConfirmationNotice(ctx, plan)

			// Dry run - just preview
			if dryRun {
				if !ctx.Table() {
					copies.SetReport(shared.PlanResults(plan), true)
					return nil
				}
				ctx.Section()
				fmt.Println("Dry run - no changes made.")
				return nil
			}

			// Confirm unless --yes flag, or there is nothing to confirm
			pending, _, _, _ := plan.Stats()
			if !skipConfirmation && !ctx.QuietMode && (pending > 0 || copies.WillChange()) {
				ctx.Section()
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
			results, err := copies.Apply(plan)
			if err != nil {
				return fmt.Errorf("failed to apply migration: %w", err)
			}

			// Display results
			if !ctx.Table() {
				copies.SetReport(results, false)
			} else if ctx.Human() {
				if err := shared.DisplayResults(ctx, results); err != nil {
					return err
				}
			}

			return shared.CheckResults(results)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&skipConfirmation, "yes", false, "Skip confirmation prompt")
	shared.AddCopyFlags(cmd, &copyOpts)

	return cmd
}
