package domain

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultsFile_EncodeLoadRoundTrip(t *testing.T) {
	f := NewDefaultsFile()
	f.Add(*VSCodeApp, Target{Kind: TargetKindExtension, Identifier: "tf"})
	f.Add(*VSCodeApp, Target{Kind: TargetKindExtension, Identifier: "md"})
	f.Add(*VSCodeApp, Target{Kind: TargetKindExtension, Identifier: "md"})
	f.Add(*ChromeApp, Target{Kind: TargetKindScheme, Identifier: "https"})
	f.Add(*ChromeApp, Target{Kind: TargetKindScheme, Identifier: "http"})
	f.Add(*ChromeApp, Target{Kind: TargetKindUTI, Identifier: "public.html"})

	var buf bytes.Buffer
	if err := f.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	want := `version: 1
defaults:
  com.google.Chrome: # Google Chrome
    utis: [public.html]
    schemes: [http, https]
  com.microsoft.VSCode: # Visual Studio Code
    extensions: [md, tf]
`
	if buf.String() != want {
		t.Fatalf("Encode() =\n%s\nwant\n%s", buf.String(), want)
	}

	loaded, err := LoadDefaultsFile(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Defaults["com.microsoft.VSCode"].Extensions; strings.Join(got, ",") != "md,tf" {
		t.Errorf("extensions = %v", got)
	}
	if got := loaded.Defaults["com.google.Chrome"].Schemes; strings.Join(got, ",") != "http,https" {
		t.Errorf("schemes = %v", got)
	}
}

func TestLoadDefaultsFile_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "defaults file is empty"},
		{name: "wrong version", input: "version: 2\ndefaults: {}\n", wantErr: "unsupported defaults file version 2"},
		{name: "missing version", input: "defaults: {}\n", wantErr: "unsupported defaults file version 0"},
		{name: "unknown field", input: "version: 1\ndefault: {}\n", wantErr: "field default not found"},
		{name: "unknown target kind", input: "version: 1\ndefaults:\n  TextEdit:\n    mimes: [text/plain]\n", wantErr: "field mimes not found"},
		{
			name:    "target under two apps",
			input:   "version: 1\ndefaults:\n  TextEdit:\n    extensions: [txt]\n  Preview:\n    extensions: [.TXT]\n",
			wantErr: "extension:txt is listed under both Preview and TextEdit",
		},
		{name: "empty app", input: "version: 1\ndefaults:\n  \"\":\n    extensions: [txt]\n", wantErr: "empty app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadDefaultsFile(strings.NewReader(tt.input))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("LoadDefaultsFile() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
