package domain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jackchuka/dutix/internal/macos"
)

// ExtensionDefault pairs a discovered file extension with a UTI it resolves to
// and the current default application for that UTI.
type ExtensionDefault struct {
	Extension  string `yaml:"extension" json:"extension"`
	UTI        string `yaml:"uti,omitempty" json:"uti,omitempty"`
	DefaultApp *App   `yaml:"default_app,omitempty" json:"default_app,omitempty"`
}

// ExtensionLister aggregates extensions from installed apps and resolves their
// default handlers.
type ExtensionLister struct {
	bridge macos.Bridge
}

// NewExtensionLister creates a new ExtensionLister.
func NewExtensionLister(bridge macos.Bridge) *ExtensionLister {
	return &ExtensionLister{bridge: bridge}
}

// ListExtensionDefaults returns one row per (extension, UTI) pair discovered
// across all installed applications, optionally filtered by a case-insensitive
// substring pattern. Per-app and per-extension errors are skipped.
func (l *ExtensionLister) ListExtensionDefaults(pattern string) ([]ExtensionDefault, error) {
	apps, err := l.bridge.ListAllApplications()
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	extSet := make(map[string]struct{})
	for _, app := range apps {
		docTypes, err := l.bridge.ListSupportedDocumentTypes(app.Path)
		if err != nil {
			continue
		}
		for _, dt := range docTypes {
			for _, ext := range dt.Extensions {
				ext = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(ext, ".")))
				if ext != "" {
					extSet[ext] = struct{}{}
				}
			}
		}
	}

	patternLower := strings.ToLower(pattern)
	exts := make([]string, 0, len(extSet))
	for ext := range extSet {
		if pattern == "" || strings.Contains(ext, patternLower) {
			exts = append(exts, ext)
		}
	}
	sort.Strings(exts)

	defaultCache := make(map[string]*App)
	var rows []ExtensionDefault
	for _, ext := range exts {
		utis, err := l.bridge.ResolveUTIsForExtension(ext)
		if err != nil || len(utis) == 0 {
			rows = append(rows, ExtensionDefault{Extension: ext})
			continue
		}
		sort.Strings(utis)
		for _, uti := range utis {
			app, cached := defaultCache[uti]
			if !cached {
				app = l.defaultAppForUTI(uti)
				defaultCache[uti] = app
			}
			rows = append(rows, ExtensionDefault{Extension: ext, UTI: uti, DefaultApp: app})
		}
	}

	return rows, nil
}

func (l *ExtensionLister) defaultAppForUTI(uti string) *App {
	appPath, err := l.bridge.GetDefaultAppForUTI(uti)
	if err != nil || appPath == "" {
		return nil
	}
	return &App{Path: appPath, Name: extractAppName(appPath)}
}
