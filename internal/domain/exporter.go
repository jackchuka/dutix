package domain

import (
	"fmt"
	"strings"

	"github.com/jackchuka/dutix/internal/macos"
)

// Exporter reads the current default applications into a defaults file
type Exporter struct {
	bridge macos.Bridge
	apps   map[string]*App // Installed app by lowercase bundle ID; nil if not installed
}

// NewExporter creates a new exporter
func NewExporter(bridge macos.Bridge) *Exporter {
	return &Exporter{bridge: bridge, apps: make(map[string]*App)}
}

// ExportOptions control which user overrides ExportUserDefaults includes
type ExportOptions struct {
	// All includes URL schemes and UTIs that only one installed app can
	// handle, which apps usually claim for themselves (e.g. slack://)
	All bool
}

// ExportUserDefaults exports the default application overrides stored in
// the LaunchServices preferences: the defaults chosen in Finder or System
// Settings, set by dutix, or claimed by apps. Overrides for apps that are not
// installed are left out, as are system entries derived from other defaults
// (com.apple.default-app.*).
func (e *Exporter) ExportUserDefaults(opts ExportOptions) (*DefaultsFile, error) {
	handlers, err := e.bridge.UserHandlers()
	if err != nil {
		return nil, err
	}

	f := NewDefaultsFile()
	for _, h := range handlers {
		target := Target{Identifier: h.Identifier}
		switch h.Kind {
		case macos.HandlerExtension:
			target.Kind = TargetKindExtension
		case macos.HandlerUTI:
			if strings.HasPrefix(h.Identifier, "com.apple.default-app.") {
				continue
			}
			target.Kind = TargetKindUTI
		case macos.HandlerScheme:
			target.Kind = TargetKindScheme
		default:
			continue
		}

		// Extension overrides are only written by "Change All" or dutix, so
		// they are always a choice
		if !opts.All && target.Kind != TargetKindExtension && !e.hasAlternatives(target) {
			continue
		}

		app := e.installedApp(h.BundleID)
		if app == nil {
			continue
		}
		f.Add(*app, target)
	}
	return f, nil
}

// ExportTargets exports the current default application of each target
func (e *Exporter) ExportTargets(targets []Target) (*DefaultsFile, error) {
	f := NewDefaultsFile()
	for _, target := range targets {
		path, err := e.defaultAppPath(target)
		if err != nil {
			return nil, err
		}
		if path == "" {
			continue
		}
		info, err := e.bridge.AppInfoAtPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", path, err)
		}
		f.Add(fromMacOSAppInfo(info), target)
	}
	return f, nil
}

func (e *Exporter) defaultAppPath(target Target) (string, error) {
	switch target.Kind {
	case TargetKindExtension:
		utis, err := e.bridge.ResolveUTIsForExtension(target.Identifier)
		if err != nil {
			return "", fmt.Errorf("failed to resolve extension %s: %w", target.Identifier, err)
		}
		if len(utis) == 0 {
			return "", nil
		}
		path, _ := e.bridge.GetDefaultAppForUTI(utis[0])
		return path, nil
	case TargetKindUTI:
		path, _ := e.bridge.GetDefaultAppForUTI(target.Identifier)
		return path, nil
	case TargetKindScheme:
		path, _ := e.bridge.GetDefaultAppForScheme(target.Identifier)
		return path, nil
	default:
		return "", fmt.Errorf("unsupported target kind: %s", target.Kind)
	}
}

// hasAlternatives reports whether more than one installed app can handle
// target, so the default is a choice rather than the only app available
func (e *Exporter) hasAlternatives(target Target) bool {
	var apps []string
	switch target.Kind {
	case TargetKindUTI:
		apps, _ = e.bridge.ListAppsForUTI(target.Identifier)
	case TargetKindScheme:
		apps, _ = e.bridge.ListAppsForScheme(target.Identifier)
	}
	return len(apps) > 1
}

// installedApp returns the app that bundleID resolves to, with the bundle
// ID's proper case (LaunchServices preferences store it lowercase), or nil
// if no copy is installed
func (e *Exporter) installedApp(bundleID string) *App {
	key := strings.ToLower(bundleID)
	if app, ok := e.apps[key]; ok {
		return app
	}
	var app *App
	paths, _ := e.bridge.ApplicationsForBundleID(bundleID)
	for _, p := range paths {
		info, err := e.bridge.AppInfoAtPath(p)
		if err != nil {
			continue
		}
		a := fromMacOSAppInfo(info)
		a.Path = p
		app = &a
		break
	}
	e.apps[key] = app
	return app
}
