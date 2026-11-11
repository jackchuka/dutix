package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

// ApplyReport is the data of apply for the JSON or YAML output
type ApplyReport struct {
	DryRun  bool            `json:"dry_run" yaml:"dry_run"`
	Apps    []domain.Report `json:"apps" yaml:"apps"`
	Missing []string        `json:"missing,omitempty" yaml:"missing,omitempty"` // Apps in the file that aren't installed
}

// appPlan is the plan for one app of the defaults file
type appPlan struct {
	app    *domain.App
	plan   *domain.Plan
	copies *shared.Copies
}

// NewApplyCommand creates the apply command
func NewApplyCommand() *cobra.Command {
	var dryRun bool
	var skipConfirmation bool

	cmd := &cobra.Command{
		Use:   "apply <file>",
		Short: "Set default applications from a defaults file",
		Long: `Sets the default applications listed in a defaults file, as written by
"dutix export". Use "-" to read the file from stdin.

Targets that already have the right default are skipped, so applying the
same file again changes nothing. Apps in the file that aren't installed are
skipped with a warning. An app listed by name or path that matches several
apps is an error, and nothing is changed.

File format:

  version: 1
  defaults:
    com.google.Chrome:
      schemes: [http, https]
      utis: [public.html]
    com.microsoft.VSCode:
      extensions: [md, tf]

Apps can be given as bundle IDs, bundle paths, or names, as on the command line.

With --output json or yaml, only a report of the outcome is printed, for
scripts; this needs --yes or --dry-run.

Examples:
  dutix apply defaults.yaml --dry-run
  dutix apply defaults.yaml
  dutix apply defaults.yaml --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			if !ctx.Table() && !skipConfirmation && !dryRun {
				return shared.Usagef("--output %s needs --yes or --dry-run", ctx.OutputMode)
			}

			file, err := readDefaultsFile(args[0])
			if err != nil {
				return err
			}

			// Resolve every app before changing anything
			resolver := domain.NewResolver(ctx.Bridge)
			report := ApplyReport{DryRun: dryRun, Apps: []domain.Report{}}
			var plans []appPlan
			for _, key := range file.Apps() {
				app, err := resolver.Resolve(key)
				if errors.Is(err, domain.ErrAppNotFound) {
					report.Missing = append(report.Missing, key)
					ctx.Warnf("%s is not installed; skipping it", key)
					continue
				}
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}

				d := file.Defaults[key]
				if len(d.Extensions)+len(d.UTIs)+len(d.Schemes) == 0 {
					continue
				}
				targets, err := shared.ParseTargetSpecs(d.Extensions, d.UTIs, d.Schemes)
				if err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
				plan, err := ctx.Planner.BuildPlan(app, targets)
				if errors.Is(err, domain.ErrPlanNoValidItems) {
					ctx.Warnf("%s: none of its targets can be set; skipping it", key)
					continue
				}
				if err != nil {
					return fmt.Errorf("%s: failed to build plan: %w", key, err)
				}
				plans = append(plans, appPlan{app: app, plan: plan})
			}

			if len(plans) == 0 {
				if !ctx.Table() {
					ctx.SetData(report)
				} else if ctx.Human() {
					fmt.Println("Nothing to apply.")
				}
				return nil
			}

			// Preview
			changes := false
			combined := domain.NewPlan()
			for i := range plans {
				p := &plans[i]
				// With --yes, each app's results follow right away instead
				if ctx.Human() && (dryRun || !skipConfirmation) {
					ctx.Section()
					fmt.Printf("%s (%s)\n", p.app.Name, p.app.Path)
					if p.plan.Settled() {
						fmt.Printf("Targets: %d (all already set)\n", len(p.plan.Items))
					} else if err := formatter.NewTable(nil).FormatPlan(p.plan); err != nil {
						return err
					}
				}
				p.copies = shared.PreviewCopies(ctx, p.app, shared.CopyOptions{})
				pending, _, _, _ := p.plan.Stats()
				changes = changes || pending > 0 || p.copies.WillChange()
				combined.Items = append(combined.Items, p.plan.Items...)
			}
			shared.PrintConfirmationNotice(ctx, combined)

			if dryRun {
				if !ctx.Table() {
					for _, p := range plans {
						report.Apps = append(report.Apps, p.copies.Report(shared.PlanResults(p.plan), true))
					}
					ctx.SetData(report)
					return nil
				}
				ctx.Section()
				fmt.Println("Dry run - no changes made.")
				return nil
			}

			if !changes {
				if !ctx.Table() {
					for _, p := range plans {
						report.Apps = append(report.Apps, p.copies.Report(shared.PlanResults(p.plan), false))
					}
					ctx.SetData(report)
				} else if ctx.Human() {
					ctx.Section()
					fmt.Println("All defaults are already set.")
				}
				return nil
			}

			if !skipConfirmation {
				ctx.Section()
				fmt.Print("Apply these changes? (y/N): ")
				response, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				response = strings.ToLower(strings.TrimSpace(response))
				if response != "y" && response != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			var all []domain.ApplyResult
			for _, p := range plans {
				results, err := p.copies.Apply(p.plan)
				if err != nil {
					return fmt.Errorf("%s: failed to apply changes: %w", p.app.Name, err)
				}
				all = append(all, results...)
				report.Apps = append(report.Apps, p.copies.Report(results, false))
				if ctx.Human() {
					title := fmt.Sprintf("%s (%s):", p.app.Name, p.app.Path)
					if err := shared.DisplayResults(ctx, title, results); err != nil {
						return err
					}
				}
			}
			if !ctx.Table() {
				ctx.SetData(report)
			}
			return shared.CheckResults(all)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().BoolVar(&skipConfirmation, "yes", false, "Skip confirmation prompt")

	return cmd
}

// readDefaultsFile reads the defaults file at path, or stdin for "-". An
// unreadable or invalid file is a usage error.
func readDefaultsFile(path string) (*domain.DefaultsFile, error) {
	r := os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, shared.Usagef("cannot read defaults file: %v", err)
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	file, err := domain.LoadDefaultsFile(r)
	if err != nil {
		return nil, shared.Usagef("%s: %v", path, err)
	}
	return file, nil
}
