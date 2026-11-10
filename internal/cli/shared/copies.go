package shared

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

// CopyOptions are the flags controlling other copies of the app's bundle ID
type CopyOptions struct {
	PruneOthers bool
}

// AddCopyFlags registers --prune-others on cmd
func AddCopyFlags(cmd *cobra.Command, opts *CopyOptions) {
	cmd.Flags().BoolVar(&opts.PruneOthers, "prune-others", false,
		"After applying, unregister other copies with the same bundle ID so defaults resolve to this one")
}

func versionSuffix(v string) string {
	if v == "" {
		return ""
	}
	return fmt.Sprintf(" (CFBundleVersion %s)", v)
}

// Copies applies a plan for an app and manages the other registered copies
// of its bundle ID
type Copies struct {
	ctx     *Context
	app     *domain.App
	opts    CopyOptions
	managed bool               // The app has a bundle ID, so copies can be managed
	report  *domain.CopyReport // Copies before applying; nil if listing them failed

	registered   bool     // Apply registered the app
	unregistered []string // Apply unregistered these other copies
}

// Report describes what applying plan did, for JSON or YAML output.
// results are the apply results, or the plan items for a dry run.
func (c *Copies) Report(results []domain.ApplyResult, dryRun bool) domain.Report {
	r := domain.NewReport(c.app, results, dryRun)
	r.Registered = c.registered
	r.Unregistered = c.unregistered
	return r
}

// PrintReport prints the report as JSON or YAML
func (c *Copies) PrintReport(results []domain.ApplyResult, dryRun bool) error {
	return formatter.Encode(c.ctx.OutputMode, nil, c.Report(results, dryRun))
}

// PlanResults wraps plan items as results, to report a plan without applying it
func PlanResults(plan *domain.Plan) []domain.ApplyResult {
	results := make([]domain.ApplyResult, len(plan.Items))
	for i, item := range plan.Items {
		results[i] = domain.ApplyResult{Item: item}
	}
	return results
}

// PreviewCopies describes the other registered copies of app's bundle ID
// before any change is made, and returns a Copies to apply the plan with
func PreviewCopies(ctx *Context, app *domain.App, opts CopyOptions) *Copies {
	c := &Copies{ctx: ctx, app: app, opts: opts, managed: app.BundleID != ""}
	if !c.managed {
		return c
	}
	report, err := domain.InspectCopies(ctx.Bridge, app)
	if err != nil {
		ctx.Warnf("%v", err)
		return c
	}
	c.report = &report

	others := report.Others()
	registered := report.Registered()
	if ctx.Human() {
		if len(others) > 0 || !registered {
			ctx.Section()
		}
		if len(others) > 0 {
			if opts.PruneOthers {
				fmt.Println("Other registered copies (will be unregistered):")
			} else {
				fmt.Println("Other registered copies:")
			}
			for _, o := range others {
				fmt.Printf("  %s%s\n", o.Path, versionSuffix(o.Version))
			}
		}
		if !registered {
			fmt.Printf("%s is not registered with LaunchServices yet; it will be registered before applying.\n", app.Path)
		}
	}

	if w := report.Winner(); w != nil && !w.Chosen && !opts.PruneOthers {
		ctx.Warnf("%s%s outranks %s; defaults will resolve to it. Use --prune-others.",
			w.Path, versionSuffix(w.Version), app.Path)
	}
	return c
}

// WillChange reports whether Apply will register the app or unregister other
// copies of its bundle ID, apart from any change in the plan
func (c *Copies) WillChange() bool {
	if !c.managed {
		return false
	}
	if c.report == nil {
		return true
	}
	return !c.report.Registered() ||
		(c.opts.PruneOthers && len(c.report.Others()) > 0)
}

// Apply applies plan. It first registers the app if needed, since defaults
// are set by path, and afterwards unregisters the other copies (with
// --prune-others) and warns if another copy still wins.
//
// Other copies are unregistered only after applying. LaunchServices saves
// registration changes some seconds after they're made, and setting a
// default by extension restarts lsd, which drops unsaved changes. An
// unregistration made before applying would be dropped, and repeating it
// right after the restart was not saved either (macOS 27.2; see "Registration
// Functions" in the macos-apphandlers-bridge README). Changes made
// after the restart, including re-registering a dropped registration, are
// saved.
func (c *Copies) Apply(plan *domain.Plan) ([]domain.ApplyResult, error) {
	b := c.ctx.Bridge
	if c.managed && (c.report == nil || !c.report.Registered()) {
		if err := b.RegisterApplication(c.app.Path); err != nil {
			c.ctx.Warnf("failed to register %s: %v", c.app.Path, err)
		} else {
			c.registered = true
			if c.ctx.Human() {
				c.ctx.Section()
				fmt.Printf("Registered %s\n", c.app.Path)
			}
		}
	}

	results, err := c.ctx.Applier.Apply(plan, nil)
	if err != nil || !c.managed {
		return results, err
	}

	// Registers again if an lsd restart dropped the registration above
	res, ensureErr := domain.EnsureWinner(b, c.app, domain.EnsureOptions{Prune: c.opts.PruneOthers})
	if ensureErr != nil {
		c.ctx.Warnf("%v", ensureErr)
	}
	c.registered = c.registered || res.DidRegister
	c.unregistered = res.Unregistered
	if c.ctx.Human() && (res.DidRegister || len(res.Unregistered) > 0) {
		c.ctx.Section()
		if res.DidRegister {
			fmt.Printf("Registered %s\n", c.app.Path)
		}
		for _, p := range res.Unregistered {
			fmt.Printf("Unregistered %s\n", p)
		}
	}
	if w := res.Winner(); w != nil && !w.Chosen {
		hint := " Use --prune-others to unregister other copies."
		if c.opts.PruneOthers {
			hint = ""
		}
		c.ctx.Warnf("defaults for %s resolve to %s%s, not %s.%s",
			c.app.BundleID, w.Path, versionSuffix(w.Version), c.app.Path, hint)
	}
	return results, nil
}
