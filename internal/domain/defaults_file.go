package domain

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultsFileVersion is the version of the defaults file format
const DefaultsFileVersion = 1

// DefaultsFile is the desired default applications, as written by export and
// read by apply. Each key of Defaults names an app the way the command line
// does: a bundle ID, a bundle path, or a name.
type DefaultsFile struct {
	Version  int                    `yaml:"version" json:"version"`
	Defaults map[string]AppDefaults `yaml:"defaults" json:"defaults"`

	// Names are app display names by key, written as comments
	Names map[string]string `yaml:"-" json:"-"`
}

// AppDefaults is the targets an app should be the default for
type AppDefaults struct {
	Extensions []string `yaml:"extensions,omitempty" json:"extensions,omitempty"`
	UTIs       []string `yaml:"utis,omitempty" json:"utis,omitempty"`
	Schemes    []string `yaml:"schemes,omitempty" json:"schemes,omitempty"`
}

// NewDefaultsFile returns an empty defaults file
func NewDefaultsFile() *DefaultsFile {
	return &DefaultsFile{
		Version:  DefaultsFileVersion,
		Defaults: make(map[string]AppDefaults),
		Names:    make(map[string]string),
	}
}

// Add records that app should be the default for target
func (f *DefaultsFile) Add(app App, target Target) {
	key := app.BundleID
	if key == "" {
		key = app.Path
	}
	if app.Name != "" {
		f.Names[key] = app.Name
	}
	d := f.Defaults[key]
	switch target.Kind {
	case TargetKindExtension:
		d.Extensions = appendUnique(d.Extensions, target.Identifier)
	case TargetKindUTI:
		d.UTIs = appendUnique(d.UTIs, target.Identifier)
	case TargetKindScheme:
		d.Schemes = appendUnique(d.Schemes, target.Identifier)
	}
	f.Defaults[key] = d
}

func appendUnique(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	list = append(list, s)
	slices.Sort(list)
	return list
}

// Apps returns the app keys in a stable order
func (f *DefaultsFile) Apps() []string {
	keys := make([]string, 0, len(f.Defaults))
	for k := range f.Defaults {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	return keys
}

// LoadDefaultsFile reads and validates a defaults file
func LoadDefaultsFile(r io.Reader) (*DefaultsFile, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	var f DefaultsFile
	if err := decoder.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("defaults file is empty")
		}
		return nil, fmt.Errorf("invalid defaults file: %w", err)
	}
	if f.Version != DefaultsFileVersion {
		return nil, fmt.Errorf("unsupported defaults file version %d: expected %d", f.Version, DefaultsFileVersion)
	}

	// A target listed under two apps has no single desired default
	owners := make(map[string]string)
	for _, app := range f.Apps() {
		if strings.TrimSpace(app) == "" {
			return nil, fmt.Errorf("invalid defaults file: empty app")
		}
		d := f.Defaults[app]
		for _, t := range d.keys() {
			if other, ok := owners[t]; ok {
				return nil, fmt.Errorf("invalid defaults file: %s is listed under both %s and %s", t, other, app)
			}
			owners[t] = app
		}
	}
	return &f, nil
}

// keys names each target as kind:identifier, normalized the way the command
// line parses them
func (d AppDefaults) keys() []string {
	var keys []string
	for _, e := range d.Extensions {
		keys = append(keys, "extension:"+strings.ToLower(strings.TrimPrefix(strings.TrimSpace(e), ".")))
	}
	for _, u := range d.UTIs {
		keys = append(keys, "uti:"+strings.TrimSpace(u))
	}
	for _, s := range d.Schemes {
		keys = append(keys, "scheme:"+strings.ToLower(strings.TrimSpace(s)))
	}
	return keys
}

// Encode writes the defaults file as YAML, with each app's name as a comment
func (f *DefaultsFile) Encode(w io.Writer) error {
	defaults := &yaml.Node{Kind: yaml.MappingNode}
	for _, app := range f.Apps() {
		var value yaml.Node
		if err := value.Encode(f.Defaults[app]); err != nil {
			return err
		}
		for i := 1; i < len(value.Content); i += 2 {
			value.Content[i].Style = yaml.FlowStyle
		}
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: app}
		if name := f.Names[app]; name != "" && name != app {
			key.LineComment = name
		}
		defaults.Content = append(defaults.Content, key, &value)
	}

	doc := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "version"},
		{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprint(f.Version)},
		{Kind: yaml.ScalarNode, Value: "defaults"},
		defaults,
	}}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}
