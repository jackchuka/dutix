package domain

import (
	"reflect"
	"testing"

	"github.com/jackchuka/dutix/internal/macos"
	mocks "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

func TestExporter_ExportUserDefaults(t *testing.T) {
	handlers := []macos.Handler{
		{Kind: macos.HandlerScheme, Identifier: "https", BundleID: "com.google.chrome"},
		{Kind: macos.HandlerUTI, Identifier: "public.html", BundleID: "com.google.chrome"},
		{Kind: macos.HandlerUTI, Identifier: "com.apple.default-app.web-browser", BundleID: "com.google.chrome"},
		{Kind: macos.HandlerExtension, Identifier: "tf", BundleID: "com.microsoft.vscode"},
		{Kind: macos.HandlerScheme, Identifier: "slack", BundleID: "com.tinyspeck.slackmacgap"},
		{Kind: macos.HandlerScheme, Identifier: "gone", BundleID: "com.example.gone"},
	}
	setup := func(m *mocks.MockBridge) {
		m.EXPECT().UserHandlers().Return(handlers, nil)
		m.EXPECT().ListAppsForScheme("https").Return([]string{ChromeApp.Path, SafariApp.Path}, nil).AnyTimes()
		m.EXPECT().ListAppsForUTI("public.html").Return([]string{ChromeApp.Path, SafariApp.Path}, nil).AnyTimes()
		m.EXPECT().ListAppsForScheme("slack").Return([]string{"/Applications/Slack.app"}, nil).AnyTimes()
		m.EXPECT().ListAppsForScheme("gone").Return([]string{"/a.app", "/b.app"}, nil).AnyTimes()
		m.EXPECT().ApplicationsForBundleID("com.google.chrome").Return([]string{ChromeApp.Path}, nil).AnyTimes()
		m.EXPECT().AppInfoAtPath(ChromeApp.Path).Return(macos.AppInfo{Name: ChromeApp.Name, BundleID: ChromeApp.BundleID}, nil).AnyTimes()
		m.EXPECT().ApplicationsForBundleID("com.microsoft.vscode").Return([]string{VSCodeApp.Path}, nil).AnyTimes()
		m.EXPECT().AppInfoAtPath(VSCodeApp.Path).Return(macos.AppInfo{Name: VSCodeApp.Name, BundleID: VSCodeApp.BundleID}, nil).AnyTimes()
		m.EXPECT().ApplicationsForBundleID("com.tinyspeck.slackmacgap").Return([]string{"/Applications/Slack.app"}, nil).AnyTimes()
		m.EXPECT().AppInfoAtPath("/Applications/Slack.app").Return(macos.AppInfo{Name: "Slack", BundleID: "com.tinyspeck.slackmacgap"}, nil).AnyTimes()
		m.EXPECT().ApplicationsForBundleID("com.example.gone").Return(nil, nil).AnyTimes()
	}

	tests := []struct {
		name string
		opts ExportOptions
		want map[string]AppDefaults
	}{
		{
			name: "choices only",
			want: map[string]AppDefaults{
				"com.google.Chrome":    {UTIs: []string{"public.html"}, Schemes: []string{"https"}},
				"com.microsoft.VSCode": {Extensions: []string{"tf"}},
			},
		},
		{
			name: "all",
			opts: ExportOptions{All: true},
			want: map[string]AppDefaults{
				"com.google.Chrome":         {UTIs: []string{"public.html"}, Schemes: []string{"https"}},
				"com.microsoft.VSCode":      {Extensions: []string{"tf"}},
				"com.tinyspeck.slackmacgap": {Schemes: []string{"slack"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mocks.NewMockBridge(ctrl)
			setup(m)

			f, err := NewExporter(m).ExportUserDefaults(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.Defaults, tt.want) {
				t.Errorf("Defaults = %+v, want %+v", f.Defaults, tt.want)
			}
			if f.Names["com.google.Chrome"] != ChromeApp.Name {
				t.Errorf("Names = %v", f.Names)
			}
		})
	}
}

func TestExporter_ExportTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	m.EXPECT().ResolveUTIsForExtension("md").Return([]string{"net.daringfireball.markdown"}, nil)
	m.EXPECT().GetDefaultAppForUTI("net.daringfireball.markdown").Return(VSCodeApp.Path, nil)
	m.EXPECT().GetDefaultAppForScheme("http").Return(ChromeApp.Path, nil)
	m.EXPECT().GetDefaultAppForScheme("nodefault").Return("", nil)
	m.EXPECT().AppInfoAtPath(VSCodeApp.Path).Return(macos.AppInfo{Name: VSCodeApp.Name, BundleID: VSCodeApp.BundleID}, nil)
	m.EXPECT().AppInfoAtPath(ChromeApp.Path).Return(macos.AppInfo{Name: ChromeApp.Name, BundleID: ChromeApp.BundleID}, nil)

	f, err := NewExporter(m).ExportTargets([]Target{
		{Kind: TargetKindExtension, Identifier: "md"},
		{Kind: TargetKindScheme, Identifier: "http"},
		{Kind: TargetKindScheme, Identifier: "nodefault"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]AppDefaults{
		"com.microsoft.VSCode": {Extensions: []string{"md"}},
		"com.google.Chrome":    {Schemes: []string{"http"}},
	}
	if !reflect.DeepEqual(f.Defaults, want) {
		t.Errorf("Defaults = %+v, want %+v", f.Defaults, want)
	}
}
