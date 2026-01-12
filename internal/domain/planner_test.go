package domain

import (
	"testing"

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

func TestPlanner_BuildPlan_Warnings(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

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
