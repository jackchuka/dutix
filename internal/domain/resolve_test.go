package domain

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackchuka/dutix/internal/macos"
	mocks "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

// makeBundle creates a minimal app bundle at dir/rel and returns its
// symlink-resolved path (t.TempDir lives under /var -> /private/var)
func makeBundle(t *testing.T, dir, rel string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Join(p, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "Contents", "Info.plist"), []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// expectInfo makes AppInfoAtPath return a bundle ID derived from the bundle name
func expectInfo(m *mocks.MockBridge) {
	m.EXPECT().AppInfoAtPath(gomock.Any()).DoAndReturn(func(p string) (macos.AppInfo, error) {
		name := strings.TrimSuffix(filepath.Base(p), ".app")
		return macos.AppInfo{Name: name, Path: p, BundleID: "org.example." + name, Version: "1"}, nil
	}).AnyTimes()
}

func TestResolveApp_Paths(t *testing.T) {
	dir := t.TempDir()
	foo := makeBundle(t, dir, "Foo.app")
	helper := makeBundle(t, dir, "Foo.app/Contents/Helpers/Bar.app")
	if err := os.Symlink(foo, filepath.Join(dir, "Link.app")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Empty.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.app"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Chdir(dir)

	tests := []struct {
		name     string
		query    string
		wantPath string
		wantErr  string
	}{
		{name: "absolute", query: foo, wantPath: foo},
		{name: "trailing slash", query: foo + "/", wantPath: foo},
		{name: "unclean", query: filepath.Join(dir, "Foo.app", "Contents", "..") + "/.", wantPath: foo},
		{name: "relative", query: "./Foo.app", wantPath: foo},
		{name: "bare .app in cwd", query: "Foo.app", wantPath: foo},
		{name: "tilde", query: "~/Foo.app", wantPath: foo},
		{name: "symlink resolves to target", query: filepath.Join(dir, "Link.app"), wantPath: foo},
		{name: "nested helper app", query: helper, wantPath: helper},
		{name: "missing explicit path", query: filepath.Join(dir, "Missing.app"), wantErr: "application not found"},
		{name: "missing tilde path", query: "~/Missing.app", wantErr: "application not found"},
		{name: "missing ./ path", query: "./Missing.app", wantErr: "application not found"},
		{name: "unknown ~user", query: "~no-such-user-dutix/Foo.app", wantErr: "cannot expand ~no-such-user-dutix"},
		{name: "no Info.plist", query: filepath.Join(dir, "Empty.app"), wantErr: "no Contents/Info.plist"},
		{name: "regular file", query: filepath.Join(dir, "file.app"), wantErr: "not an application bundle"},
		{name: "not a .app directory", query: dir, wantErr: "not an application bundle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mocks.NewMockBridge(ctrl)
			expectInfo(m)
			// Paths never fall back to the app scan
			m.EXPECT().ListAllApplications().Times(0)

			app, err := ResolveApp(m, tt.query)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveApp(%q) error = %v, want containing %q", tt.query, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveApp(%q) error = %v", tt.query, err)
			}
			if app.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", app.Path, tt.wantPath)
			}
			if app.BundleID == "" || app.Version != "1" {
				t.Errorf("app info not taken from bundle: %+v", app)
			}
		})
	}
}

func TestResolveApp_PathWithoutBundleID(t *testing.T) {
	dir := t.TempDir()
	foo := makeBundle(t, dir, "Foo.app")

	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	m.EXPECT().AppInfoAtPath(foo).Return(macos.AppInfo{Name: "Foo", Path: foo}, nil)

	if _, err := ResolveApp(m, foo); err == nil || !strings.Contains(err.Error(), "no CFBundleIdentifier") {
		t.Fatalf("error = %v, want missing bundle ID", err)
	}
}

func TestResolveApp_PathInfoError(t *testing.T) {
	dir := t.TempDir()
	foo := makeBundle(t, dir, "Foo.app")

	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	m.EXPECT().AppInfoAtPath(foo).Return(macos.AppInfo{}, errors.New("boom"))

	if _, err := ResolveApp(m, foo); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want bridge error", err)
	}
}

func TestResolveApp_BareAppNameFallsBackToScan(t *testing.T) {
	t.Chdir(t.TempDir())

	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	// "Safari.app" is also a valid bundle ID, so it is tried as one first
	m.EXPECT().ApplicationsForBundleID("Safari.app").Return(nil, nil)
	m.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "Safari", Path: "/Applications/Safari.app", BundleID: "com.apple.Safari"},
		{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", BundleID: "com.apple.TextEdit"},
	}, nil)

	app, err := ResolveApp(m, "Safari.app")
	if err != nil {
		t.Fatal(err)
	}
	if app.Path != "/Applications/Safari.app" {
		t.Errorf("got %+v", app)
	}
}

func TestResolveApp_BundleID(t *testing.T) {
	t.Chdir(t.TempDir())
	scan := []macos.AppInfo{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app", BundleID: "com.microsoft.VSCode"},
		{Name: "Code Helper", Path: "/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper.app", BundleID: "com.microsoft.VSCode.helper"},
	}

	t.Run("LaunchServices rank order", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockBridge(ctrl)
		m.EXPECT().ApplicationsForBundleID("com.microsoft.VSCode").
			Return([]string{"/nix/store/bbb-vscode/Applications/Visual Studio Code.app", "/nix/store/aaa-vscode/Applications/Visual Studio Code.app"}, nil)
		m.EXPECT().AppInfoAtPath("/nix/store/bbb-vscode/Applications/Visual Studio Code.app").
			Return(macos.AppInfo{Name: "Code", BundleID: "com.microsoft.VSCode", Version: "2"}, nil)
		m.EXPECT().ListAllApplications().Times(0)

		app, err := ResolveApp(m, "com.microsoft.VSCode")
		if err != nil {
			t.Fatal(err)
		}
		if app.Path != "/nix/store/bbb-vscode/Applications/Visual Studio Code.app" || app.Version != "2" {
			t.Errorf("got %+v", app)
		}
	})

	t.Run("scan fallback, case-insensitive", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockBridge(ctrl)
		m.EXPECT().ApplicationsForBundleID("com.microsoft.vscode").Return(nil, nil)
		m.EXPECT().ListAllApplications().Return(scan, nil)

		app, err := ResolveApp(m, "com.microsoft.vscode")
		if err != nil {
			t.Fatal(err)
		}
		if app.Path != "/Applications/Visual Studio Code.app" {
			t.Errorf("got %+v", app)
		}
	})

	t.Run("unknown bundle ID falls back to name match", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockBridge(ctrl)
		m.EXPECT().ApplicationsForBundleID("com.example.nothing").Return(nil, nil)
		m.EXPECT().ListAllApplications().Return(scan, nil)

		if _, err := ResolveApp(m, "com.example.nothing"); err == nil || !strings.Contains(err.Error(), "application not found") {
			t.Fatalf("error = %v, want not found", err)
		}
	})
}

func TestResolveApp_Name(t *testing.T) {
	t.Chdir(t.TempDir())
	scan := []macos.AppInfo{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app", BundleID: "com.microsoft.VSCode"},
		{Name: "Code Helper", Path: "/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper.app", BundleID: "com.microsoft.VSCode.helper"},
		{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", BundleID: "com.apple.TextEdit"},
	}

	tests := []struct {
		query    string
		wantPath string
		wantErr  string
	}{
		{query: "textedit", wantPath: "/System/Applications/TextEdit.app"},
		{query: "visual studio code", wantPath: "/Applications/Visual Studio Code.app"},
		{query: "Code", wantErr: "multiple applications found"},
		{query: "Nope", wantErr: "application not found"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mocks.NewMockBridge(ctrl)
			m.EXPECT().ListAllApplications().Return(scan, nil)

			app, err := ResolveApp(m, tt.query)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if app.Path != tt.wantPath {
				t.Errorf("got %+v", app)
			}
		})
	}
}

func TestResolveApp_TildeUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	dir, err := os.MkdirTemp(u.HomeDir, ".dutix-test-")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	foo := makeBundle(t, dir, "Foo.app")

	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	expectInfo(m)

	query := "~" + u.Username + "/" + filepath.Base(dir) + "/Foo.app"
	app, err := ResolveApp(m, query)
	if err != nil {
		t.Fatal(err)
	}
	if app.Path != foo {
		t.Errorf("Path = %q, want %q", app.Path, foo)
	}
}

func TestResolveApp_RelativeSubstringFallsBackToScan(t *testing.T) {
	t.Chdir(t.TempDir())

	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	m.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "Terminal", Path: "/System/Applications/Utilities/Terminal.app", BundleID: "com.apple.Terminal"},
		{Name: "TextEdit", Path: "/System/Applications/TextEdit.app", BundleID: "com.apple.TextEdit"},
	}, nil)

	app, err := ResolveApp(m, "Utilities/Terminal")
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != "Terminal" {
		t.Errorf("got %+v", app)
	}
}

func TestResolveApp_BundleIDSkipsStaleEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	m.EXPECT().ApplicationsForBundleID("org.example.foo").
		Return([]string{"/nix/store/gone-foo/Applications/Foo.app", "/Applications/Foo.app"}, nil)
	m.EXPECT().AppInfoAtPath("/nix/store/gone-foo/Applications/Foo.app").
		Return(macos.AppInfo{}, errors.New("Application not found"))
	m.EXPECT().AppInfoAtPath("/Applications/Foo.app").
		Return(macos.AppInfo{Name: "Foo", BundleID: "org.example.foo"}, nil)
	m.EXPECT().ListAllApplications().Times(0)

	app, err := ResolveApp(m, "org.example.foo")
	if err != nil {
		t.Fatal(err)
	}
	if app.Path != "/Applications/Foo.app" {
		t.Errorf("got %+v", app)
	}
}
