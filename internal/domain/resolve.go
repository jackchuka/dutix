package domain

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackchuka/dutix/internal/macos"
)

var bundleIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// Resolver turns app arguments into Apps. It scans the installed
// applications at most once, so resolving several arguments (as migrate does)
// costs one scan.
type Resolver struct {
	bridge  macos.Bridge
	scanned []macos.AppInfo
}

// NewResolver returns a Resolver using b
func NewResolver(b macos.Bridge) *Resolver {
	return &Resolver{bridge: b}
}

// ResolveApp resolves a single app argument; see Resolver.Resolve
func ResolveApp(b macos.Bridge, query string) (*App, error) {
	return NewResolver(b).Resolve(query)
}

// Resolve turns an app argument into an App. The argument may be:
//
//  1. A path to an application bundle: it contains "/", starts with "~", or
//     ends in ".app". The path is expanded and its symlinks resolved, and the
//     bundle is used even if LaunchServices or the app scan don't know it. An
//     explicit path (starting with "/", "~", "./" or "../") that doesn't
//     exist is an error. Any other argument that doesn't exist as a path
//     (e.g. "Utilities/Terminal" or "Safari.app") falls back to matching.
//  2. A bundle identifier (e.g. com.microsoft.VSCode): resolved to the copy
//     LaunchServices prefers, then to an exact bundle ID match in the app scan.
//  3. Anything else: a case-insensitive substring of an app's name or path,
//     as matched by FindApp.
func (r *Resolver) Resolve(query string) (*App, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	if looksLikePath(query) {
		app, err := resolveAppPath(r.bridge, query)
		if err == nil || isExplicitPath(query) || !errors.Is(err, os.ErrNotExist) {
			return app, err
		}
		// Not a path after all, e.g. "Utilities/Terminal": match by name or path
	}

	if bundleIDPattern.MatchString(query) {
		if app, err := r.resolveBundleID(query); err != nil || app != nil {
			return app, err
		}
	}

	apps, err := r.scan()
	if err != nil {
		return nil, err
	}
	app, err := FindApp(FromMacOSAppInfos(apps), query)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func (r *Resolver) scan() ([]macos.AppInfo, error) {
	if r.scanned == nil {
		apps, err := r.bridge.ListAllApplications()
		if err != nil {
			return nil, fmt.Errorf("failed to scan applications: %w", err)
		}
		r.scanned = apps
	}
	return r.scanned, nil
}

// isExplicitPath reports whether query can only be meant as a path
func isExplicitPath(query string) bool {
	for _, prefix := range []string{"/", "~", "./", "../"} {
		if strings.HasPrefix(query, prefix) {
			return true
		}
	}
	return query == "." || query == ".."
}

func looksLikePath(query string) bool {
	return strings.Contains(query, "/") || strings.HasPrefix(query, "~") ||
		strings.HasSuffix(strings.ToLower(query), ".app")
}

// expandPath expands a leading "~" or "~user", makes the path absolute, and
// resolves symlinks. LaunchServices records and reports the resolved path, so
// a symlink (such as a Home Manager link into /nix/store) is registered and
// compared as its target. Defaults are stored by bundle ID, so if the target
// later disappears, defaults fall back to the next registered copy.
func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		name, rest, _ := strings.Cut(p[1:], "/")
		var home string
		if name == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot expand ~: %w", err)
			}
			home = h
		} else {
			u, err := user.Lookup(name)
			if err != nil {
				return "", fmt.Errorf("cannot expand ~%s: %w", name, err)
			}
			home = u.HomeDir
		}
		p = filepath.Join(home, rest)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func resolveAppPath(b macos.Bridge, query string) (*App, error) {
	path, err := expandPath(query)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w at %s: %w", ErrAppNotFound, query, os.ErrNotExist)
		}
		return nil, fmt.Errorf("invalid application path %s: %w", query, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("invalid application path %s: %w", query, err)
	}
	if !info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".app") {
		return nil, fmt.Errorf("not an application bundle (expected a .app directory): %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err != nil {
		return nil, fmt.Errorf("not an application bundle (no Contents/Info.plist): %s", path)
	}

	appInfo, err := b.AppInfoAtPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read application bundle %s: %w", path, err)
	}
	if appInfo.BundleID == "" {
		return nil, fmt.Errorf("application bundle has no CFBundleIdentifier: %s", path)
	}

	app := fromMacOSAppInfo(appInfo)
	app.Path = path
	return &app, nil
}

// resolveBundleID returns the highest-ranked registered copy of the bundle
// ID that can still be read (LaunchServices can list stale entries, e.g. for
// a deleted app), else an exact match from the app scan. It returns nil, nil
// when nothing has the bundle ID, so the caller can fall back to name matching.
func (r *Resolver) resolveBundleID(bundleID string) (*App, error) {
	paths, _ := r.bridge.ApplicationsForBundleID(bundleID)
	for _, p := range paths {
		appInfo, err := r.bridge.AppInfoAtPath(p)
		if err != nil {
			continue
		}
		app := fromMacOSAppInfo(appInfo)
		app.Path = p
		if app.BundleID == "" {
			app.BundleID = bundleID
		}
		return &app, nil
	}

	apps, err := r.scan()
	if err != nil {
		return nil, err
	}
	for _, info := range apps {
		if strings.EqualFold(info.BundleID, bundleID) {
			app := fromMacOSAppInfo(info)
			return &app, nil
		}
	}
	return nil, nil
}

// canonicalPath resolves symlinks when the path exists, so paths from
// LaunchServices and from the command line compare equal
func canonicalPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}
