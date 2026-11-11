package formatter

import (
	"bytes"
	"testing"
)

func TestWriteEnvelope(t *testing.T) {
	env := Envelope{
		APIVersion: APIVersion,
		Command:    "set",
		Data:       map[string]int{"items": 2},
		Error:      &ErrorInfo{Code: "apply_failed", ExitCode: 5, Message: "failed to apply 1 of 2 item(s)"},
	}
	tests := []struct {
		format string
		want   string
	}{
		{format: "json", want: `{
  "api_version": 1,
  "command": "set",
  "data": {
    "items": 2
  },
  "error": {
    "code": "apply_failed",
    "exit_code": 5,
    "message": "failed to apply 1 of 2 item(s)"
  }
}
`},
		{format: "yaml", want: `api_version: 1
command: set
data:
    items: 2
error:
    code: apply_failed
    exit_code: 5
    message: failed to apply 1 of 2 item(s)
`},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteEnvelope(tt.format, &buf, env); err != nil {
				t.Fatal(err)
			}
			if buf.String() != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", buf.String(), tt.want)
			}
		})
	}
}

func TestDataFormatter_EmptyListsAreArrays(t *testing.T) {
	var got any
	f := NewData(func(d any) { got = d })
	_ = f.FormatApps(nil)

	var buf bytes.Buffer
	if err := WriteEnvelope("json", &buf, Envelope{APIVersion: APIVersion, Command: "apps list", Data: got}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"data": []`)) {
		t.Errorf("empty app list did not encode as []:\n%s", buf.String())
	}
}
