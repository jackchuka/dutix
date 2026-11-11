package cli

import (
	"io"
	"testing"

	"github.com/jackchuka/dutix/internal/cli/shared"
)

func TestRootCommand_UsageErrors(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			if got := ExitCode(err); got != shared.ExitUsage {
				t.Errorf("ExitCode(%v) = %d, want %d", err, got, shared.ExitUsage)
			}
		})
	}
}
