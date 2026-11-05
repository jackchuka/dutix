//go:build darwin

package macos

import (
	"errors"
	"fmt"
	"path/filepath"

	handler "github.com/jackchuka/macos-apphandlers-bridge"
)

type AppInfo struct {
	Name     string
	Path     string
	BundleID string
	Version  string // CFBundleVersion; only filled by AppInfoAtPath
}

type DocumentType struct {
	Name       string
	UTIs       []string
	Extensions []string
}

// Bridge defines the interface for macOS bridge operations
// This is the common interface used by Planner, Applier, and Verifier
//
//go:generate mockgen -source=$GOFILE -package=mock_$GOPACKAGE -destination=./mock/mock_$GOFILE
type Bridge interface {
	// List applications installed
	ListAllApplications() ([]AppInfo, error)

	// Listing operations
	ListAppsForUTI(uti string) ([]string, error)
	ListAppsForScheme(scheme string) ([]string, error)

	// Query operations (used by Planner and Verifier)
	GetDefaultAppForUTI(uti string) (string, error)
	GetDefaultAppForScheme(scheme string) (string, error)
	ResolveUTIsForExtension(ext string) ([]string, error)

	// Mutation operations (used by Applier)
	SetDefaultForUTI(appPath, uti string) error
	SetDefaultForScheme(appPath, scheme string) error
	SetDefaultForExtensions(appPath string, exts []string) error

	// PromptsForDefaultAppChanges reports whether macOS shows a confirmation
	// dialog for each SetDefaultForUTI / SetDefaultForScheme change
	PromptsForDefaultAppChanges() bool

	// App information operations
	ListDefaultDocumentTypesForApp(appPath string) ([]DocumentType, error)
	ListSupportedDocumentTypes(appPath string) ([]DocumentType, error)

	// LaunchServices registration operations

	// AppInfoAtPath reads an application bundle's Info.plist, whether or not
	// LaunchServices knows about the bundle
	AppInfoAtPath(path string) (AppInfo, error)
	// ApplicationsForBundleID lists the registered copies of a bundle ID in
	// LaunchServices preference order; the first one is what defaults resolve to
	ApplicationsForBundleID(bundleID string) ([]string, error)
	// RegisterApplication registers an application bundle with LaunchServices
	RegisterApplication(path string) error
	// UnregisterApplication removes an application bundle from LaunchServices
	UnregisterApplication(path string) error
}

// ErrUserDeclined means the user kept the current application in the macOS
// confirmation dialog shown for default app changes (macOS 26.4 and later)
var ErrUserDeclined = errors.New("kept the current application in the confirmation dialog")

func wrapDeclined(err error) error {
	if handler.IsUserDeclined(err) {
		return ErrUserDeclined
	}
	return err
}

// bridge provides a high-level interface to macOS default application management
type bridge struct {
	// Optional: cache for resolved defaults (future optimization)
}

// NewBridge creates a new macOS client
func NewBridge() *bridge {
	return &bridge{}
}

// ListAllApplications lists all installed applications on macOS
func (c *bridge) ListAllApplications() ([]AppInfo, error) {
	apps, err := handler.ListAllApplications()
	if err != nil {
		return nil, err
	}

	var result []AppInfo
	for _, app := range apps {
		result = append(result, AppInfo{
			Name:     app.Name,
			Path:     app.Path,
			BundleID: app.BundleID,
		})
	}

	return result, nil
}

// GetDefaultAppForUTI returns the default application path for a UTI
func (c *bridge) GetDefaultAppForUTI(uti string) (string, error) {
	return handler.GetDefaultAppForUTI(uti)
}

// GetDefaultAppForScheme returns the default application path for a URL scheme
func (c *bridge) GetDefaultAppForScheme(scheme string) (string, error) {
	return handler.GetDefaultAppForScheme(scheme)
}

// SetDefaultForUTI sets the default application for a UTI
func (c *bridge) SetDefaultForUTI(appPath, uti string) error {
	// Normalize the app path
	absPath, err := filepath.Abs(appPath)
	if err != nil {
		return fmt.Errorf("invalid app path: %w", err)
	}

	return wrapDeclined(handler.SetDefaultForUTI(absPath, uti))
}

// SetDefaultForScheme sets the default application for a URL scheme
func (c *bridge) SetDefaultForScheme(appPath, scheme string) error {
	// Normalize the app path
	absPath, err := filepath.Abs(appPath)
	if err != nil {
		return fmt.Errorf("invalid app path: %w", err)
	}

	return wrapDeclined(handler.SetDefaultForScheme(absPath, scheme))
}

// PromptsForDefaultAppChanges reports whether macOS asks the user to confirm
// each UTI / URL scheme default change (macOS 26.4 and later)
func (c *bridge) PromptsForDefaultAppChanges() bool {
	return handler.PromptsForDefaultAppChanges()
}

// SetDefaultForExtensions sets the default application for file extensions
// by tag, the same way Finder's "Change All" does. Used for extensions that
// resolve to dynamic UTIs, which SetDefaultForUTI cannot set. It restarts lsd
// once per call, so callers pass all extensions for an app together.
func (c *bridge) SetDefaultForExtensions(appPath string, exts []string) error {
	absPath, err := filepath.Abs(appPath)
	if err != nil {
		return fmt.Errorf("invalid app path: %w", err)
	}

	return handler.SetDefaultForExtensions(absPath, exts)
}

// ResolveUTIsForExtension resolves a file extension to one or more UTIs
func (c *bridge) ResolveUTIsForExtension(extension string) ([]string, error) {
	return handler.ResolveUTIsForExtension(extension)
}

// ListAppsForUTI returns all applications that can open a UTI
func (c *bridge) ListAppsForUTI(uti string) ([]string, error) {
	return handler.ListAppsForUTI(uti)
}

// ListAppsForScheme returns all applications that can handle a URL scheme
func (c *bridge) ListAppsForScheme(scheme string) ([]string, error) {
	return handler.ListAppsForScheme(scheme)
}

// ListDefaultDocumentTypesForApp returns the UTIs and URL schemes supported by an application
func (c *bridge) ListDefaultDocumentTypesForApp(appPath string) ([]DocumentType, error) {
	docuementTypes, err := handler.ListDefaultDocumentTypes(appPath)
	if err != nil {
		return nil, err
	}

	var result []DocumentType
	for _, dt := range docuementTypes {
		result = append(result, DocumentType{
			Name:       dt.TypeName,
			UTIs:       dt.UTIs,
			Extensions: dt.Extensions,
		})
	}
	return result, nil
}

// ListSupportedDocumentTypes returns all document types an application can handle
func (c *bridge) ListSupportedDocumentTypes(appPath string) ([]DocumentType, error) {
	documentTypes, err := handler.ListSupportedDocumentTypes(appPath)
	if err != nil {
		return nil, err
	}

	var result []DocumentType
	for _, dt := range documentTypes {
		result = append(result, DocumentType{
			Name:       dt.TypeName,
			UTIs:       dt.UTIs,
			Extensions: dt.Extensions,
		})
	}
	return result, nil
}

// AppInfoAtPath reads the name, bundle ID, and CFBundleVersion of the
// application bundle at path, whether or not LaunchServices knows about it
func (c *bridge) AppInfoAtPath(path string) (AppInfo, error) {
	info, err := handler.GetBundleInfo(path)
	if err != nil {
		return AppInfo{}, err
	}
	return AppInfo{
		Name:     info.Name,
		Path:     path,
		BundleID: info.BundleID,
		Version:  info.Version,
	}, nil
}

// ApplicationsForBundleID returns every registered application with the
// bundle ID, in LaunchServices preference order: the first one is the copy
// that defaults resolve to
func (c *bridge) ApplicationsForBundleID(bundleID string) ([]string, error) {
	return handler.ListAppsForBundleID(bundleID)
}

// RegisterApplication registers the bundle at path with LaunchServices
func (c *bridge) RegisterApplication(path string) error {
	return handler.RegisterApp(path)
}

// UnregisterApplication removes the bundle at path from the LaunchServices
// database, so it no longer competes with other copies of the same bundle ID
func (c *bridge) UnregisterApplication(path string) error {
	return handler.UnregisterApp(path)
}
