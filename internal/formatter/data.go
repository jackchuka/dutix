package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jackchuka/dutix/internal/domain"
	"gopkg.in/yaml.v3"
)

// APIVersion is the version of the JSON and YAML output schema
const APIVersion = 1

// Envelope is the single document every command prints with --output json
// or yaml: the command's data, an error, or both (e.g. a report of a set
// where some items failed)
type Envelope struct {
	APIVersion int        `json:"api_version" yaml:"api_version"`
	Command    string     `json:"command" yaml:"command"`
	Data       any        `json:"data,omitempty" yaml:"data,omitempty"`
	Error      *ErrorInfo `json:"error,omitempty" yaml:"error,omitempty"`
}

// ErrorInfo describes why a command failed
type ErrorInfo struct {
	Code     string `json:"code" yaml:"code"`
	ExitCode int    `json:"exit_code" yaml:"exit_code"`
	Message  string `json:"message" yaml:"message"`
}

// WriteEnvelope writes env as JSON or YAML
func WriteEnvelope(format string, w io.Writer, env Envelope) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(env)
	case "yaml":
		encoder := yaml.NewEncoder(w)
		if err := encoder.Encode(env); err != nil {
			return err
		}
		return encoder.Close()
	default:
		return fmt.Errorf("unsupported output format for encoding: %s", format)
	}
}

// AppDetails is the data of apps show
type AppDetails struct {
	App              domain.App            `json:"app" yaml:"app"`
	SupportedTypes   []domain.DocumentType `json:"supported_types" yaml:"supported_types"`
	DefaultTypes     []domain.DocumentType `json:"default_types" yaml:"default_types"`
	RegisteredCopies []domain.AppCopy      `json:"registered_copies" yaml:"registered_copies"`
}

// TargetDetails is the data of targets show
type TargetDetails struct {
	Target        domain.Target `json:"target" yaml:"target"`
	DefaultApp    string        `json:"default_app,omitempty" yaml:"default_app,omitempty"`
	ResolvedUTIs  []string      `json:"resolved_utis" yaml:"resolved_utis"`
	AvailableApps []string      `json:"available_apps" yaml:"available_apps"`
}

// DataFormatter hands each command's data to emit, to be written in the
// envelope once the command finishes
type DataFormatter struct {
	emit func(any)
}

// NewData returns a Formatter that passes data to emit
func NewData(emit func(any)) *DataFormatter {
	return &DataFormatter{emit: emit}
}

func (f *DataFormatter) FormatApps(apps []domain.App) error {
	f.emit(nonNil(apps))
	return nil
}

func (f *DataFormatter) FormatAppDetails(
	app domain.App,
	supportedTypes []domain.DocumentType,
	defaultTypes []domain.DocumentType,
	copies []domain.AppCopy,
) error {
	f.emit(AppDetails{
		App:              app,
		SupportedTypes:   nonNil(supportedTypes),
		DefaultTypes:     nonNil(defaultTypes),
		RegisteredCopies: nonNil(copies),
	})
	return nil
}

func (f *DataFormatter) FormatTargetDetails(
	target domain.Target,
	defaultApp string,
	resolvedUTIs []string,
	availableApps []string,
) error {
	f.emit(TargetDetails{
		Target:        target,
		DefaultApp:    defaultApp,
		ResolvedUTIs:  nonNil(resolvedUTIs),
		AvailableApps: nonNil(availableApps),
	})
	return nil
}

func (f *DataFormatter) FormatExtensionDefaults(rows []domain.ExtensionDefault) error {
	f.emit(nonNil(rows))
	return nil
}

// nonNil makes empty lists encode as [] rather than null
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
