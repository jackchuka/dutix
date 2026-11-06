package shared

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/spf13/cobra"
)

// CopyOptions are the flags controlling LaunchServices registration of the
// application and of other copies of its bundle ID
type CopyOptions struct {
	PruneOthers bool
	NoRegister  bool
}

// AddCopyFlags registers --prune-others and --no-register on cmd
func AddCopyFlags(cmd *cobra.Command, opts *CopyOptions) {
	cmd.Flags().BoolVar(&opts.PruneOthers, "prune-others", false,
		"After applying, unregister other copies with the same bundle ID so defaults resolve to this one")
	cmd.Flags().BoolVar(&opts.NoRegister, "no-register", false,
		"Don't register the app with LaunchServices before applying")
}

func warnf(format string, a ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", a...)
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
		warnf("%v", err)
		return c
	}
	c.report = &report

	others := report.Others()
	registered := report.Registered()
	if !ctx.QuietMode {
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
		if !registered && !opts.NoRegister {
			fmt.Printf("%s is not registered with LaunchServices yet; it will be registered before applying.\n", app.Path)
		}
		if len(others) > 0 || !registered {
			fmt.Println()
		}
	}

	if w := report.Winner(); w != nil && !w.Chosen && !opts.PruneOthers {
		warnf("%s%s outranks %s; defaults will resolve to it. Use --prune-others.",
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
	return (!c.opts.NoRegister && !c.report.Registered()) ||
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
// Functions" in the macos-apphandlers-bridge fork's README). Changes made
// after the restart, including re-registering a dropped registration, are
// saved.
func (c *Copies) Apply(plan *domain.Plan) ([]domain.ApplyResult, error) {
	b := c.ctx.Bridge
	if c.managed && !c.opts.NoRegister && (c.report == nil || !c.report.Registered()) {
		if err := b.RegisterApplication(c.app.Path); err != nil {
			warnf("failed to register %s: %v", c.app.Path, err)
		} else if !c.ctx.QuietMode {
			fmt.Printf("Registered %s\n\n", c.app.Path)
		}
	}

	results, err := c.ctx.Applier.Apply(plan, nil)
	if err != nil || !c.managed {
		return results, err
	}

	res, ensureErr := domain.EnsureWinner(b, c.app, domain.EnsureOptions{
		Register: !c.opts.NoRegister, // again, in case an lsd restart dropped it
		Prune:    c.opts.PruneOthers,
	})
	if ensureErr != nil {
		warnf("%v", ensureErr)
	}
	if !c.ctx.QuietMode {
		for _, p := range res.Unregistered {
			fmt.Printf("Unregistered %s\n", p)
		}
	}
	if w := res.Winner(); w != nil && !w.Chosen {
		hint := " Use --prune-others to unregister other copies."
		if c.opts.PruneOthers {
			hint = ""
		}
		warnf("defaults for %s resolve to %s%s, not %s.%s",
			c.app.BundleID, w.Path, versionSuffix(w.Version), c.app.Path, hint)
	}
	return results, nil
}
