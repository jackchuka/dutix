package formatter

import (
	"encoding/json"
	"io"

	"github.com/jackchuka/dutix/internal/domain"
	"gopkg.in/yaml.v3"
)

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	w io.Writer
}

// encodeData is a helper to encode any data as JSON with consistent formatting
func (f *JSONFormatter) encodeData(data any) error {
	encoder := json.NewEncoder(f.w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func (f *JSONFormatter) FormatApps(apps []domain.App) error {
	return f.encodeData(apps)
}

func (f *JSONFormatter) FormatPlan(plan *domain.Plan) error {
	return f.encodeData(plan)
}

func (f *JSONFormatter) FormatAppDetails(
	app domain.App,
	supportedTypes []domain.DocumentType,
	defaultTypes []domain.DocumentType,
) error {
	data := map[string]any{
		"app":            app,
		"supportedTypes": supportedTypes,
		"defaultTypes":   defaultTypes,
	}
	return f.encodeData(data)
}

func (f *JSONFormatter) FormatTargetDetails(
	target domain.Target,
	defaultApp string,
	resolvedUTIs []string,
	availableApps []string,
) error {
	data := map[string]any{
		"target":        target,
		"defaultApp":    defaultApp,
		"resolvedUTIs":  resolvedUTIs,
		"availableApps": availableApps,
	}
	return f.encodeData(data)
}

func (f *JSONFormatter) FormatExtensionDefaults(rows []domain.ExtensionDefault) error {
	return f.encodeData(rows)
}

// YAMLFormatter formats output as YAML
type YAMLFormatter struct {
	w io.Writer
}

// encodeData is a helper to encode any data as YAML with consistent formatting
func (f *YAMLFormatter) encodeData(data any) error {
	encoder := yaml.NewEncoder(f.w)
	defer func() { _ = encoder.Close() }()
	return encoder.Encode(data)
}

func (f *YAMLFormatter) FormatApps(apps []domain.App) error {
	return f.encodeData(apps)
}

func (f *YAMLFormatter) FormatPlan(plan *domain.Plan) error {
	return f.encodeData(plan)
}

func (f *YAMLFormatter) FormatAppDetails(
	app domain.App,
	supportedTypes []domain.DocumentType,
	defaultTypes []domain.DocumentType,
) error {
	data := map[string]any{
		"app":            app,
		"supportedTypes": supportedTypes,
		"defaultTypes":   defaultTypes,
	}
	return f.encodeData(data)
}

func (f *YAMLFormatter) FormatTargetDetails(
	target domain.Target,
	defaultApp string,
	resolvedUTIs []string,
	availableApps []string,
) error {
	data := map[string]any{
		"target":        target,
		"defaultApp":    defaultApp,
		"resolvedUTIs":  resolvedUTIs,
		"availableApps": availableApps,
	}
	return f.encodeData(data)
}

func (f *YAMLFormatter) FormatExtensionDefaults(rows []domain.ExtensionDefault) error {
	return f.encodeData(rows)
}
