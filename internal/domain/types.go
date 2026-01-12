package domain

import (
	"slices"
	"time"

	"github.com/jackchuka/dutix/internal/macos"
)

// TargetKind represents the type of default handler target
type TargetKind string

const (
	// TargetKindUTI represents a Uniform Type Identifier target
	TargetKindUTI TargetKind = "uti"
	// TargetKindExtension represents a file extension target
	TargetKindExtension TargetKind = "extension"
	// TargetKindScheme represents a URL scheme target
	TargetKindScheme TargetKind = "scheme"
)

// Target represents something that can have a default handler
type Target struct {
	Kind         TargetKind `yaml:"kind"`                    // Type of target (UTI, extension, or scheme)
	Identifier   string     `yaml:"identifier"`              // The actual identifier (e.g., "txt", "public.plain-text", "http")
	Extension    string     `yaml:"extension,omitempty"`     // Extension provided for display purposes
	ResolvedUTIs []string   `yaml:"resolved_utis,omitempty"` // For extension targets, the resolved UTI(s)
}

// App represents an application
type App struct {
	Name     string `yaml:"name"`                // Display name (e.g., "Visual Studio Code")
	BundleID string `yaml:"bundle_id,omitempty"` // Bundle identifier (optional, e.g., "com.microsoft.VSCode")
	Path     string `yaml:"path"`                // Canonical path to .app bundle (e.g., "/Applications/Visual Studio Code.app")
}

// PlanItemStatus represents the execution status of a plan item
type PlanItemStatus string

const (
	// StatusPending indicates the item has not been processed yet
	StatusPending PlanItemStatus = "pending"
	// StatusInProgress indicates the item is currently being processed
	StatusInProgress PlanItemStatus = "in_progress"
	// StatusSuccess indicates the item was successfully applied
	StatusSuccess PlanItemStatus = "success"
	// StatusFailed indicates the item failed to apply
	StatusFailed PlanItemStatus = "failed"
	// StatusSkipped indicates the item was skipped (e.g., no-op)
	StatusSkipped PlanItemStatus = "skipped"
)

// PlanItem represents a single change to apply
type PlanItem struct {
	Target     Target         // What to change
	CurrentApp *App           // Current default app (nil if none)
	DesiredApp *App           // Desired default app
	Status     PlanItemStatus // Execution status
	Warning    string         // Optional warning message
}

// Plan represents a complete set of changes
type Plan struct {
	Items     []PlanItem        // The items to apply
	CreatedAt time.Time         // When the plan was created
	Metadata  map[string]string // Optional metadata (user, host, etc.)
}

// ApplyResult represents the outcome of applying a plan item
type ApplyResult struct {
	Item     PlanItem // The original plan item
	Applied  bool     // Whether the change was applied
	Verified bool     // Whether the change was verified
	Error    error    // Error if any
	Notes    string   // Additional notes (e.g., "User declined permission prompt")
}

// IsNoOp returns true if this plan item is a no-op (current == desired)
func (p *PlanItem) IsNoOp() bool {
	if p.CurrentApp == nil || p.DesiredApp == nil {
		return false
	}
	return p.CurrentApp.Path == p.DesiredApp.Path
}

// NeedsUserConfirmation returns true if this target may require user confirmation
func (t *Target) NeedsUserConfirmation() bool {
	// System-protected schemes that typically require user confirmation
	protectedSchemes := []string{"http", "https", "ftp", "ftps"}

	if t.Kind == TargetKindScheme {
		if slices.Contains(protectedSchemes, t.Identifier) {
			return true
		}
	}

	return false
}

// NewPlan creates a new plan
func NewPlan() *Plan {
	return &Plan{
		Items:     make([]PlanItem, 0),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
}

// AddItem adds an item to the plan
func (p *Plan) AddItem(item PlanItem) {
	p.Items = append(p.Items, item)
}

// Stats returns statistics about the plan
func (p *Plan) Stats() (pending, success, failed, skipped int) {
	for _, item := range p.Items {
		switch item.Status {
		case StatusPending:
			pending++
		case StatusSuccess:
			success++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		}
	}
	return
}

// HasWarnings returns true if any items have warnings
func (p *Plan) HasWarnings() bool {
	for _, item := range p.Items {
		if item.Warning != "" {
			return true
		}
	}
	return false
}

type DocumentType struct {
	Name       string   // e.g., "Plain Text Document"
	UTIs       []string // e.g., ["public.plain-text"]
	Extensions []string // e.g., ["txt", "text"]
}

func FromMacOSDocumentTypes(dts []macos.DocumentType) []DocumentType {
	var result []DocumentType
	for _, dt := range dts {
		result = append(result, fromMacOSDocumentType(dt))
	}
	return result
}

func fromMacOSDocumentType(dt macos.DocumentType) DocumentType {
	return DocumentType{
		Name:       dt.Name,
		UTIs:       dt.UTIs,
		Extensions: dt.Extensions,
	}
}
