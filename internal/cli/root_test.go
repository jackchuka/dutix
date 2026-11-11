package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"gopkg.in/yaml.v3"
)

func TestRun_UsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown subcommand", args: []string{"nope"}},
		{name: "unknown flag", args: []string{"set", "Safari", "--nope"}},
		{name: "missing argument", args: []string{"set"}},
		{name: "too many arguments", args: []string{"apps", "show", "a", "b"}},
		{name: "extra argument to a group", args: []string{"apps", "nope"}},
		{name: "invalid output", args: []string{"--output", "xml", "version"}},
		{name: "no targets", args: []string{"set", "Safari", "--dry-run"}},
		{name: "missing defaults file", args: []string{"apply", "/nonexistent/defaults.yaml", "--dry-run"}},
		{name: "export --all with targets", args: []string{"export", "--all", "--schemes", "http"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Run(tt.args, io.Discard, io.Discard); got != shared.ExitUsage {
				t.Errorf("Run(%v) = %d, want %d", tt.args, got, shared.ExitUsage)
			}
		})
	}
}

type envelope struct {
	APIVersion int            `json:"api_version" yaml:"api_version"`
	Command    string         `json:"command" yaml:"command"`
	Data       map[string]any `json:"data" yaml:"data"`
	Error      *struct {
		Code     string `json:"code" yaml:"code"`
		ExitCode int    `json:"exit_code" yaml:"exit_code"`
		Message  string `json:"message" yaml:"message"`
	} `json:"error" yaml:"error"`
}

func TestRun_Envelope(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		unmarshal   func([]byte, any) error
		wantExit    int
		wantCommand string
		wantData    string // a key that data must have
		wantCode    string
	}{
		{
			name:        "json data",
			args:        []string{"-o", "json", "version"},
			unmarshal:   json.Unmarshal,
			wantCommand: "version",
			wantData:    "go_version",
		},
		{
			name:        "yaml data",
			args:        []string{"-o", "YAML", "version"},
			unmarshal:   yaml.Unmarshal,
			wantCommand: "version",
			wantData:    "go_version",
		},
		{
			name:        "json error",
			args:        []string{"-o", "json", "set", "Safari", "--yes"},
			unmarshal:   json.Unmarshal,
			wantExit:    shared.ExitUsage,
			wantCommand: "set",
			wantCode:    "usage",
		},
		{
			name:        "json error from a subcommand group",
			args:        []string{"-o", "json", "apps", "nope"},
			unmarshal:   json.Unmarshal,
			wantExit:    shared.ExitUsage,
			wantCommand: "apps",
			wantCode:    "usage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := Run(tt.args, &stdout, &stderr); got != tt.wantExit {
				t.Fatalf("Run(%v) = %d, want %d; stderr: %s", tt.args, got, tt.wantExit, stderr.String())
			}

			var env envelope
			if err := tt.unmarshal(stdout.Bytes(), &env); err != nil {
				t.Fatalf("stdout is not one document: %v\n%s", err, stdout.String())
			}
			if env.APIVersion != 1 || env.Command != tt.wantCommand {
				t.Errorf("api_version = %d, command = %q; want 1, %q", env.APIVersion, env.Command, tt.wantCommand)
			}
			if tt.wantData != "" {
				if _, ok := env.Data[tt.wantData]; !ok || env.Error != nil {
					t.Errorf("data = %v, error = %v; want data with %q and no error", env.Data, env.Error, tt.wantData)
				}
			}
			if tt.wantCode != "" {
				if env.Error == nil || env.Error.Code != tt.wantCode || env.Error.ExitCode != tt.wantExit || env.Error.Message == "" {
					t.Errorf("error = %+v, want code %q and exit code %d", env.Error, tt.wantCode, tt.wantExit)
				}
				if !strings.HasPrefix(stderr.String(), "Error: ") {
					t.Errorf("stderr = %q, want the error", stderr.String())
				}
			}
		})
	}
}

func TestRun_TableHelpPrintsNoEnvelope(t *testing.T) {
	var stdout bytes.Buffer
	if got := Run([]string{"-o", "json", "apps"}, &stdout, io.Discard); got != 0 {
		t.Fatalf("Run() = %d, want 0", got)
	}
	if strings.Contains(stdout.String(), "api_version") {
		t.Errorf("help printed an envelope:\n%s", stdout.String())
	}
}
