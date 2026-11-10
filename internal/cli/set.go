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
	var copyOpts shared.CopyOptions

	cmd := &cobra.Command{
		Use:   "set <app|bundle-path|bundle-id>",
		Short: "Set default application for targets",
		Long: `Sets the default application for specified file extensions, UTIs, or URL schemes.

The application can be given as:
  - a path to an .app bundle (e.g. ~/Applications/Foo.app);
    used as-is even if it isn't in /Applications, and registered with
    LaunchServices before applying
  - a bundle identifier (e.g. com.microsoft.VSCode)
  - a name or path substring (e.g. "Visual Studio Code")

macOS stores defaults by bundle ID. When several copies share a bundle ID,
defaults resolve to the one with the highest CFBundleVersion (then by path).
Use --prune-others to unregister the other copies, so the chosen one wins.

With --output json or yaml, only a report of the outcome is printed, for
scripts; this needs --yes or --dry-run.

You can specify targets using:
  --extensions: File extensions (e.g., txt,md,json)
  --utis: Direct UTI identifiers (e.g., public.plain-text)
  --schemes: URL schemes (e.g., http,https)

Examples:
  dutix set "Visual Studio Code" --extensions txt,md,json
  dutix set Safari --schemes http,https
  dutix set TextEdit --extensions txt --dry-run
  dutix set Finder --extensions zip,tar --yes
  dutix set com.microsoft.VSCode --extensions md
  dutix set ~/Applications/Foo.app --extensions md --prune-others --yes
  dutix --output json set Safari --schemes http --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			appName := args[0]

			// JSON and YAML output have no room for the prompt
			if !ctx.Table() && !skipConfirmation && !dryRun {
				return fmt.Errorf("--output %s needs --yes or --dry-run", ctx.OutputMode)
			}

			// Parse target specifications
			targets, err := shared.ParseTargetSpecs(extensions, utis, schemes)
			if err != nil {
				return err
			}

			if len(targets) == 0 {
				return fmt.Errorf("no targets specified: use --extensions, --utis, or --schemes")
			}

			// Find the application
			selectedApp, err := domain.ResolveApp(ctx.Bridge, appName)
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

			// Display preview. With --yes, the results table follows right
			// away, so the plan table is only shown for --dry-run or before
			// the prompt.
			if ctx.Human() {
				ctx.Section()
				fmt.Printf("Application: %s\n", selectedApp.Name)
				fmt.Printf("Path: %s\n", selectedApp.Path)
				if selectedApp.BundleID != "" {
					fmt.Printf("Bundle ID: %s\n", selectedApp.BundleID)
				}
				if plan.Settled() {
					fmt.Printf("Targets: %d (all already set)\n", len(plan.Items))
				} else {
					fmt.Printf("Targets: %d\n", len(plan.Items))
					if dryRun || !skipConfirmation {
						ctx.Section()
						f := formatter.New("table", nil)
						if err := f.FormatPlan(plan); err != nil {
							return err
						}
					}
				}
			}

			copies := shared.PreviewCopies(ctx, selectedApp, copyOpts)
			shared.PrintConfirmationNotice(ctx, plan)

			// Dry run - just preview
			if dryRun {
				if !ctx.Table() {
					return copies.PrintReport(shared.PlanResults(plan), true)
				}
				ctx.Section()
				fmt.Println("Dry run - no changes made.")
				return nil
			}

			// Confirm unless --yes flag, or there is nothing to confirm
			pending, _, _, _ := plan.Stats()
			if !skipConfirmation && (pending > 0 || copies.WillChange()) {
				ctx.Section()
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
			results, err := copies.Apply(plan)
			if err != nil {
				return fmt.Errorf("failed to apply changes: %w", err)
			}

			// Display results
			if !ctx.Table() {
				return copies.PrintReport(results, false)
			}
			if ctx.Human() {
				if err := shared.DisplayResults(ctx, results); err != nil {
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
	shared.AddCopyFlags(cmd, &copyOpts)

	return cmd
}
