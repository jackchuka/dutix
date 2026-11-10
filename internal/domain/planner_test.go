package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackchuka/dutix/internal/macos"
	mocks "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

func TestPlanner_BuildPlan(t *testing.T) {
	tests := []struct {
		name        string
		desiredApp  *App
		targets     []Target
		wantItemLen int
		wantErr     bool
		setupMock   func(*mocks.MockBridge)
	}{
		{
			name:       "single UTI target",
			desiredApp: VSCodeApp,
			targets: []Target{
				PlainTextUTITarget,
			},
			wantItemLen: 1,
			wantErr:     false,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)
			},
		},
		{
			name:       "single extension target (resolves to one UTI)",
			desiredApp: VSCodeApp,
			targets: []Target{
				TxtExtensionTarget,
			},
			wantItemLen: 1,
			wantErr:     false,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
				m.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)
			},
		},
		{
			name:       "single scheme target",
			desiredApp: ChromeApp,
			targets: []Target{
				HTTPSchemeTarget,
			},
			wantItemLen: 1,
			wantErr:     false,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().GetDefaultAppForScheme("http").Return("/Applications/Safari.app", nil)
			},
		},
		{
			name:       "multiple targets of different kinds",
			desiredApp: VSCodeApp,
			targets: []Target{
				PlainTextUTITarget,
				HTTPSchemeTarget,
				TxtExtensionTarget,
			},
			wantItemLen: 3, // txt extension resolves to 1 UTI
			wantErr:     false,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)
				m.EXPECT().GetDefaultAppForScheme("http").Return("/Applications/Safari.app", nil)
				m.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
				m.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)
			},
		},
		{
			name:       "nil desired app",
			desiredApp: nil,
			targets: []Target{
				PlainTextUTITarget,
			},
			wantErr:   true,
			setupMock: func(m *mocks.MockBridge) {},
		},
		{
			name:       "empty targets",
			desiredApp: VSCodeApp,
			targets:    []Target{},
			wantErr:    true,
			setupMock:  func(m *mocks.MockBridge) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bridge := mocks.NewMockBridge(ctrl)
			stubBundleIDs(bridge)
			tt.setupMock(bridge)

			planner := NewPlanner(bridge)

			plan, err := planner.BuildPlan(tt.desiredApp, tt.targets)

			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildPlan() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("BuildPlan() unexpected error: %v", err)
				return
			}

			if plan == nil {
				t.Fatal("BuildPlan() returned nil plan")
			}

			if len(plan.Items) != tt.wantItemLen {
				t.Errorf("BuildPlan() got %d items, want %d", len(plan.Items), tt.wantItemLen)
			}

			// Verify all items have desired app set
			for i, item := range plan.Items {
				if item.DesiredApp == nil {
					t.Errorf("Item %d has nil DesiredApp", i)
				} else if item.DesiredApp.Path != tt.desiredApp.Path {
					t.Errorf("Item %d has wrong DesiredApp: got %s, want %s",
						i, item.DesiredApp.Path, tt.desiredApp.Path)
				}
			}
		})
	}
}

func TestPlanner_BuildPlan_NoOpFiltering(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	stubBundleIDs(bridge)

	// Setup: txt extension resolves to public.plain-text UTI
	// TextEdit is the current default, and we're trying to set it as desired (no-op)
	bridge.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)

	planner := NewPlanner(bridge)

	// Set TextEdit as the desired app, which is already the default for txt
	plan, err := planner.BuildPlan(TextEditApp, []Target{TxtExtensionTarget})
	if err != nil {
		t.Fatalf("BuildPlan() error: %v", err)
	}

	if len(plan.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(plan.Items))
	}

	item := plan.Items[0]
	if item.Status != StatusSkipped {
		t.Errorf("Expected item to be skipped (no-op), got status: %s", item.Status)
	}

	if !item.IsNoOp() {
		t.Errorf("Item should be no-op: current=%v, desired=%v",
			item.CurrentApp, item.DesiredApp)
	}
}

func TestPlanner_BuildPlan_DynamicUTISetByExtension(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	stubBundleIDs(bridge)

	// "fish" is not declared by any app, so it resolves to a dynamic UTI.
	bridge.EXPECT().ResolveUTIsForExtension("fish").Return([]string{"dyn.age80q4pxra"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("dyn.age80q4pxra").Return("/Applications/kitty.app", nil)

	planner := NewPlanner(bridge)

	plan, err := planner.BuildPlan(VSCodeApp, []Target{
		{Kind: TargetKindExtension, Identifier: "fish", Extension: "fish"},
	})
	if err != nil {
		t.Fatalf("BuildPlan() error: %v", err)
	}

	if len(plan.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(plan.Items))
	}

	item := plan.Items[0]
	if item.Status != StatusPending {
		t.Errorf("dynamic UTI item from an extension should be pending, got status %s", item.Status)
	}
	if !item.Target.SetsByExtension() {
		t.Error("dynamic UTI item from an extension should be set by extension")
	}
	if item.Warning == "" {
		t.Error("dynamic UTI item should have a warning explaining it is set by extension")
	}
}

func TestPlanner_BuildPlan_DynamicUTIWithoutExtensionSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	stubBundleIDs(bridge)

	bridge.EXPECT().GetDefaultAppForUTI("dyn.age80q4pxra").Return("", fmt.Errorf("not found"))

	planner := NewPlanner(bridge)

	plan, err := planner.BuildPlan(VSCodeApp, []Target{
		{Kind: TargetKindUTI, Identifier: "dyn.age80q4pxra"},
	})
	if err != nil {
		t.Fatalf("BuildPlan() error: %v", err)
	}

	item := plan.Items[0]
	if item.Status != StatusSkipped {
		t.Errorf("dynamic UTI given directly should be skipped, got status %s", item.Status)
	}
	if item.Warning == "" {
		t.Error("dynamic UTI given directly should have a warning pointing to --extensions")
	}
}

func TestPlanner_BuildPlan_Warnings(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	stubBundleIDs(bridge)

	bridge.EXPECT().GetDefaultAppForScheme("http").Return("/Applications/Safari.app", nil)
	bridge.EXPECT().GetDefaultAppForScheme("https").Return("/Applications/Safari.app", nil)

	planner := NewPlanner(bridge)

	// HTTP and HTTPS schemes should have warnings
	plan, err := planner.BuildPlan(ChromeApp, []Target{HTTPSchemeTarget, HTTPSSchemeTarget})
	if err != nil {
		t.Fatalf("BuildPlan() error: %v", err)
	}

	if len(plan.Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(plan.Items))
	}

	for i, item := range plan.Items {
		if item.Warning == "" {
			t.Errorf("Item %d: expected warning for scheme %s, got none",
				i, item.Target.Identifier)
		}
	}

	if !plan.HasWarnings() {
		t.Error("Plan.HasWarnings() should return true")
	}
}

func TestPlanner_BuildPlan_NoConfirmationWarningWhenUnchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	stubBundleIDs(bridge)

	bridge.EXPECT().GetDefaultAppForScheme("http").Return("/Applications/Google Chrome.app", nil)

	planner := NewPlanner(bridge)

	plan, err := planner.BuildPlan(ChromeApp, []Target{HTTPSchemeTarget})
	if err != nil {
		t.Fatalf("BuildPlan() error: %v", err)
	}

	item := plan.Items[0]
	if item.Status != StatusSkipped {
		t.Fatalf("expected item to be skipped, got status %s", item.Status)
	}
	if item.Warning != "" {
		t.Errorf("unchanged scheme should have no warning, got %q", item.Warning)
	}
	if !plan.Settled() {
		t.Error("Plan.Settled() should return true when every item is unchanged")
	}
}

func TestNewReport(t *testing.T) {
	results := []ApplyResult{
		{Item: PlanItem{
			Target:     PlainTextUTITarget,
			CurrentApp: TextEditApp,
			DesiredApp: VSCodeApp,
			Status:     StatusSuccess,
		}},
		{Item: PlanItem{Target: HTTPSchemeTarget, DesiredApp: VSCodeApp, Status: StatusSkipped}, Declined: true},
		{Item: PlanItem{Target: HTTPSSchemeTarget, DesiredApp: VSCodeApp, Status: StatusFailed}, Error: errors.New("boom")},
	}

	r := NewReport(VSCodeApp, results, false)

	if r.App.Path != VSCodeApp.Path || r.DryRun {
		t.Errorf("unexpected report header: %+v", r)
	}
	want := []ReportItem{
		{Target: "uti:public.plain-text", Current: TextEditApp.Path, Status: StatusSuccess},
		{Target: "scheme:http", Status: StatusSkipped, Declined: true},
		{Target: "scheme:https", Status: StatusFailed, Error: "boom"},
	}
	if len(r.Items) != len(want) {
		t.Fatalf("got %d items, want %d", len(r.Items), len(want))
	}
	for i := range want {
		if r.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, r.Items[i], want[i])
		}
	}

	if empty := NewReport(VSCodeApp, nil, true); empty.Items == nil || !empty.DryRun {
		t.Errorf("empty report should have an empty item list and keep DryRun: %+v", empty)
	}
}

func TestPlan_Settled(t *testing.T) {
	tests := []struct {
		name  string
		items []PlanItem
		want  bool
	}{
		{"all skipped", []PlanItem{{Status: StatusSkipped}, {Status: StatusSkipped}}, true},
		{"one pending", []PlanItem{{Status: StatusSkipped}, {Status: StatusPending}}, false},
		{"skipped with warning", []PlanItem{{Status: StatusSkipped, Warning: "use --extensions"}}, false},
		{"applied", []PlanItem{{Status: StatusSuccess}}, false},
		{"failed", []PlanItem{{Status: StatusFailed}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := NewPlan()
			for _, item := range tt.items {
				plan.AddItem(item)
			}
			if got := plan.Settled(); got != tt.want {
				t.Errorf("Settled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanner_ResolveTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      Target
		wantCount   int
		wantUTIKind bool
		wantErr     bool
		setupMock   func(*mocks.MockBridge)
	}{
		{
			name:        "extension resolves to UTI",
			target:      TxtExtensionTarget,
			wantCount:   1,
			wantUTIKind: true,
			wantErr:     false,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
			},
		},
		{
			name:        "UTI target stays as UTI",
			target:      PlainTextUTITarget,
			wantCount:   1,
			wantUTIKind: true,
			wantErr:     false,
			setupMock:   func(m *mocks.MockBridge) {},
		},
		{
			name:        "scheme target stays as scheme",
			target:      HTTPSchemeTarget,
			wantCount:   1,
			wantUTIKind: false,
			wantErr:     false,
			setupMock:   func(m *mocks.MockBridge) {},
		},
		{
			name: "unknown extension",
			target: Target{
				Kind:       TargetKindExtension,
				Identifier: "unknownext",
			},
			wantErr: true,
			setupMock: func(m *mocks.MockBridge) {
				m.EXPECT().ResolveUTIsForExtension("unknownext").Return(nil, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bridge := mocks.NewMockBridge(ctrl)
			stubBundleIDs(bridge)
			tt.setupMock(bridge)

			planner := NewPlanner(bridge)

			resolved, err := planner.resolveTarget(tt.target)

			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveTarget() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("resolveTarget() unexpected error: %v", err)
				return
			}

			if len(resolved) != tt.wantCount {
				t.Errorf("resolveTarget() got %d targets, want %d", len(resolved), tt.wantCount)
			}

			if tt.wantUTIKind && resolved[0].Kind != TargetKindUTI {
				t.Errorf("resolveTarget() got kind %s, want %s", resolved[0].Kind, TargetKindUTI)
			}
		})
	}
}

func TestPlanner_ExtractAppName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{
			path: "/Applications/Visual Studio Code.app",
			want: "Visual Studio Code",
		},
		{
			path: "/System/Applications/TextEdit.app",
			want: "TextEdit",
		},
		{
			path: "/Applications/Safari.app",
			want: "Safari",
		},
		{
			path: "NoAppExtension",
			want: "NoAppExtension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractAppName(tt.path)
			if got != tt.want {
				t.Errorf("extractAppName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlan_Stats(t *testing.T) {
	plan := NewPlan()

	plan.AddItem(PlanItem{Status: StatusPending})
	plan.AddItem(PlanItem{Status: StatusSuccess})
	plan.AddItem(PlanItem{Status: StatusSuccess})
	plan.AddItem(PlanItem{Status: StatusFailed})
	plan.AddItem(PlanItem{Status: StatusSkipped})

	pending, success, failed, skipped := plan.Stats()

	if pending != 1 {
		t.Errorf("Stats() pending = %d, want 1", pending)
	}
	if success != 2 {
		t.Errorf("Stats() success = %d, want 2", success)
	}
	if failed != 1 {
		t.Errorf("Stats() failed = %d, want 1", failed)
	}
	if skipped != 1 {
		t.Errorf("Stats() skipped = %d, want 1", skipped)
	}
}

// stubBundleIDs answers AppInfoAtPath for the fixture apps, which the planner
// uses to compare the current default's bundle ID with the desired app's
func stubBundleIDs(m *mocks.MockBridge) {
	ids := map[string]string{
		"/Applications/TextEdit.app":           "com.apple.TextEdit",
		"/Applications/Safari.app":             "com.apple.Safari",
		"/Applications/Visual Studio Code.app": "com.microsoft.VSCode",
		"/Applications/Google Chrome.app":      "com.google.Chrome",
		"/Applications/kitty.app":              "net.kovidgoyal.kitty",
	}
	m.EXPECT().AppInfoAtPath(gomock.Any()).DoAndReturn(func(p string) (macos.AppInfo, error) {
		if id, ok := ids[p]; ok {
			return macos.AppInfo{Path: p, BundleID: id}, nil
		}
		return macos.AppInfo{}, errors.New("not found")
	}).AnyTimes()
}

func TestPlanner_BuildPlan_SameBundleIDIsUnchanged(t *testing.T) {
	const storeVSCode = "/nix/store/old-vscode/Applications/Visual Studio Code.app"
	desired := &App{Name: "Code", BundleID: "com.microsoft.VSCode", Path: "/nix/store/new-vscode/Applications/Visual Studio Code.app"}

	tests := []struct {
		name       string
		current    string
		info       macos.AppInfo
		infoErr    error
		wantStatus PlanItemStatus
		wantNote   bool
	}{
		{name: "other copy, same bundle ID", current: storeVSCode,
			info: macos.AppInfo{BundleID: "com.microsoft.VSCode"}, wantStatus: StatusSkipped, wantNote: true},
		{name: "same bundle ID, different case", current: storeVSCode,
			info: macos.AppInfo{BundleID: "COM.MICROSOFT.VSCODE"}, wantStatus: StatusSkipped, wantNote: true},
		{name: "different bundle ID", current: "/Applications/TextEdit.app",
			info: macos.AppInfo{BundleID: "com.apple.TextEdit"}, wantStatus: StatusPending},
		{name: "unreadable current app", current: storeVSCode,
			infoErr: errors.New("gone"), wantStatus: StatusPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bridge := mocks.NewMockBridge(ctrl)
			bridge.EXPECT().GetDefaultAppForUTI("public.plain-text").Return(tt.current, nil)
			bridge.EXPECT().AppInfoAtPath(tt.current).Return(tt.info, tt.infoErr)

			plan, err := NewPlanner(bridge).BuildPlan(desired, []Target{PlainTextUTITarget})
			if err != nil {
				t.Fatal(err)
			}
			item := plan.Items[0]
			if item.Status != tt.wantStatus {
				t.Errorf("Status = %s, want %s", item.Status, tt.wantStatus)
			}
			if (item.Note != "") != tt.wantNote || item.SameBundleID() != tt.wantNote {
				t.Errorf("Note = %q, SameBundleID = %v, want note %v", item.Note, item.SameBundleID(), tt.wantNote)
			}
			if tt.wantNote && !strings.Contains(item.Note, tt.current) {
				t.Errorf("Note %q should name the copy that currently opens it", item.Note)
			}

			// Unchanged items never reach SetDefault (no macOS confirmation dialog)
			if tt.wantStatus == StatusSkipped {
				results, err := NewApplier(bridge).Apply(plan, nil)
				if err != nil || results[0].Applied {
					t.Errorf("Apply() = %+v, %v; want skipped", results, err)
				}
			}
		})
	}
}

func TestPlanner_BuildPlan_SameBundleIDByExtension(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	desired := &App{Name: "kitty", BundleID: "net.kovidgoyal.kitty", Path: "/nix/store/new-kitty/Applications/kitty.app"}

	bridge.EXPECT().ResolveUTIsForExtension("fish").Return([]string{"dyn.age80q4pxra"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("dyn.age80q4pxra").Return("/nix/store/old-kitty/Applications/kitty.app", nil)
	bridge.EXPECT().AppInfoAtPath("/nix/store/old-kitty/Applications/kitty.app").
		Return(macos.AppInfo{BundleID: "net.kovidgoyal.kitty"}, nil)

	plan, err := NewPlanner(bridge).BuildPlan(desired, []Target{
		{Kind: TargetKindExtension, Identifier: "fish", Extension: "fish"},
	})
	if err != nil {
		t.Fatal(err)
	}
	item := plan.Items[0]
	if item.Status != StatusSkipped || !item.SameBundleID() {
		t.Errorf("item = %+v; want skipped as the same bundle ID", item)
	}
	if item.Warning != "" {
		t.Errorf("unchanged item should not warn that it will be set by extension: %q", item.Warning)
	}

	// Nothing to set, so SetDefaultForExtensions (and its lsd restart) doesn't run
	if _, err := NewApplier(bridge).Apply(plan, nil); err != nil {
		t.Fatal(err)
	}
}

func TestPlanner_BundleIDLookupIsCached(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	const current = "/Applications/TextEdit.app"

	bridge.EXPECT().GetDefaultAppForUTI(gomock.Any()).Return(current, nil).Times(2)
	bridge.EXPECT().AppInfoAtPath(current).Return(macos.AppInfo{BundleID: "com.apple.TextEdit"}, nil).Times(1)

	_, err := NewPlanner(bridge).BuildPlan(VSCodeApp, []Target{
		PlainTextUTITarget,
		{Kind: TargetKindUTI, Identifier: "public.rtf"},
	})
	if err != nil {
		t.Fatal(err)
	}
}
