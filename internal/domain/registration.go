package domain

import (
	"errors"
	"fmt"

	"github.com/jackchuka/dutix/internal/macos"
)

// AppCopy is one registered copy of a bundle ID
type AppCopy struct {
	Path    string `yaml:"path" json:"path"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"` // CFBundleVersion
	Winner  bool   `yaml:"winner" json:"winner"`                       // Defaults for the bundle ID resolve to this copy
	Chosen  bool   `yaml:"chosen" json:"chosen"`                       // This is the app given on the command line
}

// CopyReport describes the LaunchServices copies of an app's bundle ID.
// macOS stores defaults by bundle ID and resolves them to the highest ranked
// copy (highest CFBundleVersion, then path order), so the chosen copy only
// takes effect if it wins.
type CopyReport struct {
	Copies []AppCopy // Registered copies in LaunchServices preference order
}

// Registered reports whether the chosen copy is registered
func (r CopyReport) Registered() bool {
	for _, c := range r.Copies {
		if c.Chosen {
			return true
		}
	}
	return false
}

// Winner returns the copy defaults resolve to, or nil if none is registered
func (r CopyReport) Winner() *AppCopy {
	if len(r.Copies) == 0 {
		return nil
	}
	return &r.Copies[0]
}

// Others returns the registered copies other than the chosen one
func (r CopyReport) Others() []AppCopy {
	var others []AppCopy
	for _, c := range r.Copies {
		if !c.Chosen {
			others = append(others, c)
		}
	}
	return others
}

// InspectCopies lists the registered copies of app's bundle ID, with the
// CFBundleVersion of each
func InspectCopies(b macos.Bridge, app *App) (CopyReport, error) {
	report, err := listCopies(b, app)
	if err != nil {
		return report, err
	}
	report.fillVersions(b, app)
	return report, nil
}

// listCopies lists the registered copies of app's bundle ID without reading
// their bundles
func listCopies(b macos.Bridge, app *App) (CopyReport, error) {
	if app == nil || app.BundleID == "" {
		return CopyReport{}, fmt.Errorf("application has no bundle ID")
	}
	paths, err := b.ApplicationsForBundleID(app.BundleID)
	if err != nil {
		return CopyReport{}, fmt.Errorf("failed to list copies of %s: %w", app.BundleID, err)
	}

	chosen := canonicalPath(app.Path)
	report := CopyReport{}
	for i, p := range paths {
		report.Copies = append(report.Copies, AppCopy{
			Path:   p,
			Winner: i == 0,
			Chosen: canonicalPath(p) == chosen,
		})
	}
	return report, nil
}

// fillVersions reads each copy's CFBundleVersion
func (r CopyReport) fillVersions(b macos.Bridge, app *App) {
	for i := range r.Copies {
		c := &r.Copies[i]
		if c.Chosen && app.Version != "" {
			c.Version = app.Version
		} else if info, err := b.AppInfoAtPath(c.Path); err == nil {
			c.Version = info.Version
		}
	}
}

// EnsureOptions controls EnsureWinner
type EnsureOptions struct {
	Register bool // Register the chosen copy with LaunchServices if it isn't
	Prune    bool // Unregister every other copy of the bundle ID
}

// EnsureResult is what EnsureWinner did, and the copies afterwards
type EnsureResult struct {
	CopyReport
	Unregistered []string // Copies that were unregistered
	DidRegister  bool     // Whether the chosen copy was registered by this call
}

// Changed reports whether EnsureWinner changed the LaunchServices database
func (r EnsureResult) Changed() bool {
	return r.DidRegister || len(r.Unregistered) > 0
}

// EnsureWinner tries to make app the copy its bundle ID resolves to: with
// Register it registers app.Path if it isn't registered yet, and with Prune it
// unregisters the other copies. It returns what it changed and the copies
// afterwards, plus any registration errors.
//
// Prune only runs once app.Path is listed as registered. Otherwise it would
// unregister every copy and leave the bundle ID with no app to open its files.
//
// Call it with Prune only after setting defaults; see Copies.Apply.
func EnsureWinner(b macos.Bridge, app *App, opts EnsureOptions) (EnsureResult, error) {
	before, err := listCopies(b, app)
	if err != nil {
		return EnsureResult{}, err
	}

	var res EnsureResult
	var errs []error
	if opts.Register && !before.Registered() {
		if err := b.RegisterApplication(app.Path); err != nil {
			errs = append(errs, fmt.Errorf("failed to register %s: %w", app.Path, err))
		} else {
			res.DidRegister = true
			// LaunchServices ignores some locations, so check that the
			// registration took before pruning against it
			if opts.Prune {
				after, err := listCopies(b, app)
				if err != nil {
					errs = append(errs, err)
				} else {
					before = after
				}
			}
		}
	}
	if opts.Prune {
		if before.Registered() {
			for _, c := range before.Others() {
				if err := b.UnregisterApplication(c.Path); err != nil {
					errs = append(errs, fmt.Errorf("failed to unregister %s: %w", c.Path, err))
					continue
				}
				res.Unregistered = append(res.Unregistered, c.Path)
			}
		} else {
			errs = append(errs, fmt.Errorf("not unregistering other copies: %s is not registered with LaunchServices", app.Path))
		}
	}

	res.CopyReport = before
	if res.Changed() {
		after, err := listCopies(b, app)
		if err != nil {
			errs = append(errs, err)
		}
		res.CopyReport = after
	}
	res.fillVersions(b, app)
	return res, errors.Join(errs...)
}
