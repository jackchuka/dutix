package formatter

import (
	"bytes"
	"testing"
)

func TestTrimWriter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trailing spaces", "a  b   \n", "a  b\n"},
		{"several lines", "a  \nb \n", "a\nb\n"},
		{"no newline kept as is", "a  ", "a  "},
		{"inner spaces kept", "  a  b\n", "  a  b\n"},
		{"blank line", "   \n", "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := trimWriter{&buf}.Write([]byte(tt.in))
			if err != nil {
				t.Fatalf("Write() error: %v", err)
			}
			if n != len(tt.in) {
				t.Errorf("Write() = %d, want %d", n, len(tt.in))
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("wrote %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlanTableHasNoTrailingSpaces(t *testing.T) {
	var buf bytes.Buffer
	tbl := defaultTheme().newStyledTable(&buf, "Target", "Status")
	tbl.AddRow("uti:com.adobe.pdf", "skipped")
	tbl.AddRow("scheme:http", "pending")
	tbl.Print()

	for i, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		if bytes.HasSuffix(line, []byte(" ")) {
			t.Errorf("line %d has trailing spaces: %q", i, line)
		}
	}
}
