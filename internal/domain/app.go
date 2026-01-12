package domain

import (
	"fmt"
	"strings"

	"github.com/jackchuka/dutix/internal/macos"
)

// FromMacOSAppInfos converts macOS AppInfo to domain App
func FromMacOSAppInfos(infos []macos.AppInfo) []App {
	var apps []App
	for _, info := range infos {
		apps = append(apps, fromMacOSAppInfo(info))
	}
	return apps
}

func fromMacOSAppInfo(info macos.AppInfo) App {
	return App{
		Name:     info.Name,
		Path:     info.Path,
		BundleID: info.BundleID,
	}
}

// FilterApps filters apps by search query (case-insensitive)
func FilterApps(apps []App, query string) []App {
	if query == "" {
		return apps
	}

	query = strings.ToLower(query)
	var filtered []App

	for _, app := range apps {
		// Match against name or path
		if strings.Contains(strings.ToLower(app.Name), query) ||
			strings.Contains(strings.ToLower(app.Path), query) {
			filtered = append(filtered, app)
		}
	}

	return filtered
}

// FindApp finds a single app by query with smart matching:
// 1. Filters apps by query (case-insensitive, matches name or path)
// 2. If no matches, returns error
// 3. If one match, returns that app
// 4. If multiple matches, tries to find exact name match (case-insensitive)
// 5. If still multiple matches, returns error listing matches
func FindApp(apps []App, query string) (*App, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Filter apps by query
	filtered := FilterApps(apps, query)

	if len(filtered) == 0 {
		return nil, fmt.Errorf("application not found: %s", query)
	}

	if len(filtered) == 1 {
		return &filtered[0], nil
	}

	// Multiple matches - try exact name match
	queryLower := strings.ToLower(query)
	for _, app := range filtered {
		if strings.ToLower(app.Name) == queryLower {
			return &app, nil
		}
	}

	// No exact match - return error listing matches
	var names []string
	for _, app := range filtered {
		names = append(names, app.Name)
	}
	return nil, fmt.Errorf("multiple applications found matching: %s.\nMatches: %s", query, strings.Join(names, ", "))
}
