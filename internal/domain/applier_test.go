package domain

import (
	"fmt"
	"testing"

	dmacos "github.com/jackchuka/dutix/internal/macos"
	macos "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

func TestApplier_Apply(t *testing.T) {
	tests := []struct {
		name           string
		plan           *Plan
		wantResultLen  int
		wantSuccessful int
		wantErr        bool
		setupMock      func(*macos.MockBridge)
	}{
		{
			name: "single UTI item",
			plan: &Plan{
				Items: []PlanItem{
					{
						Target:     PlainTextUTITarget,
						DesiredApp: VSCodeApp,
						Status:     StatusPending,
					},
				},
			},
			wantResultLen:  1,
			wantSuccessful: 1,
			wantErr:        false,
			setupMock: func(m *macos.MockBridge) {
				m.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(nil)
			},
		},
		{
			name: "single scheme item",
			plan: &Plan{
				Items: []PlanItem{
					{
						Target:     HTTPSchemeTarget,
						DesiredApp: ChromeApp,
						Status:     StatusPending,
					},
				},
			},
			wantResultLen:  1,
			wantSuccessful: 1,
			wantErr:        false,
			setupMock: func(m *macos.MockBridge) {
				m.EXPECT().SetDefaultForScheme(ChromeApp.Path, "http").Return(nil)
			},
		},
		{
			name: "dynamic UTI item set by extension",
			plan: &Plan{
				Items: []PlanItem{
					{
						Target:     Target{Kind: TargetKindUTI, Identifier: "dyn.age80q4pxra", Extension: "fish"},
						DesiredApp: VSCodeApp,
						Status:     StatusPending,
					},
				},
			},
			wantResultLen:  1,
			wantSuccessful: 1,
			wantErr:        false,
			setupMock: func(m *macos.MockBridge) {
				m.EXPECT().SetDefaultForExtensions(VSCodeApp.Path, []string{"fish"}).Return(nil)
			},
		},
		{
			name: "multiple items",
			plan: &Plan{
				Items: []PlanItem{
					{
						Target:     PlainTextUTITarget,
						DesiredApp: VSCodeApp,
						Status:     StatusPending,
					},
					{
						Target:     HTTPSchemeTarget,
						DesiredApp: ChromeApp,
						Status:     StatusPending,
					},
				},
			},
			wantResultLen:  2,
			wantSuccessful: 2,
			wantErr:        false,
			setupMock: func(m *macos.MockBridge) {
				m.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(nil)
				m.EXPECT().SetDefaultForScheme(ChromeApp.Path, "http").Return(nil)
			},
		},
		{
			name: "skipped item",
			plan: &Plan{
				Items: []PlanItem{
					{
						Target:     PlainTextUTITarget,
						DesiredApp: VSCodeApp,
						Status:     StatusSkipped,
					},
				},
			},
			wantResultLen:  1,
			wantSuccessful: 0, // Skipped doesn't count as successful
			wantErr:        false,
			setupMock:      func(m *macos.MockBridge) {},
		},
		{
			name:      "nil plan",
			plan:      nil,
			wantErr:   true,
			setupMock: func(m *macos.MockBridge) {},
		},
		{
			name: "empty plan",
			plan: &Plan{
				Items: []PlanItem{},
			},
			wantErr:   true,
			setupMock: func(m *macos.MockBridge) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bridge := macos.NewMockBridge(ctrl)
			tt.setupMock(bridge)

			applier := NewApplier(bridge)

			var progressCalls int
			progressCallback := func(index int, item PlanItem) {
				progressCalls++
			}

			results, err := applier.Apply(tt.plan, progressCallback)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Apply() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Apply() unexpected error: %v", err)
				return
			}

			if len(results) != tt.wantResultLen {
				t.Errorf("Apply() got %d results, want %d", len(results), tt.wantResultLen)
			}

			// Count successful applications
			successful := 0
			for _, result := range results {
				if result.Applied && result.Item.Status == StatusSuccess {
					successful++
				}
			}

			if successful != tt.wantSuccessful {
				t.Errorf("Apply() got %d successful, want %d", successful, tt.wantSuccessful)
			}

			// Verify progress callback was called
			if !tt.wantErr && progressCalls != len(results) {
				t.Errorf("Progress callback called %d times, want %d", progressCalls, len(results))
			}
		})
	}
}

func TestApplier_Apply_BridgeIntegration(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := macos.NewMockBridge(ctrl)

	// Set up expectations for both calls
	bridge.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(nil)
	bridge.EXPECT().SetDefaultForScheme(ChromeApp.Path, "http").Return(nil)

	applier := NewApplier(bridge)

	plan := &Plan{
		Items: []PlanItem{
			{
				Target:     PlainTextUTITarget,
				DesiredApp: VSCodeApp,
				Status:     StatusPending,
			},
			{
				Target:     HTTPSchemeTarget,
				DesiredApp: ChromeApp,
				Status:     StatusPending,
			},
		},
	}

	results, err := applier.Apply(plan, nil)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Verify all results were applied successfully
	for i, result := range results {
		if !result.Applied {
			t.Errorf("Result %d was not applied", i)
		}
		if result.Item.Status != StatusSuccess {
			t.Errorf("Result %d status = %s, want %s", i, result.Item.Status, StatusSuccess)
		}
	}
}

func TestApplier_Apply_ContinuesOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := macos.NewMockBridge(ctrl)

	// First call will fail, second will succeed
	bridge.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(fmt.Errorf("simulated failure"))
	bridge.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.html").Return(nil)

	applier := NewApplier(bridge)

	plan := &Plan{
		Items: []PlanItem{
			{
				Target: Target{
					Kind:       TargetKindUTI,
					Identifier: "public.plain-text",
				},
				DesiredApp: VSCodeApp,
				Status:     StatusPending,
			},
			{
				Target: Target{
					Kind:       TargetKindUTI,
					Identifier: "public.html",
				},
				DesiredApp: VSCodeApp,
				Status:     StatusPending,
			},
		},
	}

	results, err := applier.Apply(plan, nil)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First should fail
	if results[0].Item.Status != StatusFailed {
		t.Errorf("Result 0 should be failed, got %s", results[0].Item.Status)
	}

	if results[0].Error == nil {
		t.Error("Result 0 should have error")
	}

	// Second should succeed
	if results[1].Item.Status != StatusSuccess {
		t.Errorf("Result 1 should be success, got %s", results[1].Item.Status)
	}

	if results[1].Error != nil {
		t.Errorf("Result 1 should not have error: %v", results[1].Error)
	}
}

func TestApplier_Apply_InvalidDesiredApp(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := macos.NewMockBridge(ctrl)

	applier := NewApplier(bridge)

	plan := &Plan{
		Items: []PlanItem{
			{
				Target:     PlainTextUTITarget,
				DesiredApp: nil, // Invalid
				Status:     StatusPending,
			},
		},
	}

	results, err := applier.Apply(plan, nil)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Item.Status != StatusFailed {
		t.Errorf("Result should be failed, got %s", results[0].Item.Status)
	}

	if results[0].Error == nil {
		t.Error("Result should have error for invalid app")
	}
}

func TestApplier_Apply_BatchesExtensionItems(t *testing.T) {
	dynItem := func(uti, ext string) PlanItem {
		return PlanItem{
			Target:     Target{Kind: TargetKindUTI, Identifier: uti, Extension: ext},
			DesiredApp: VSCodeApp,
			Status:     StatusPending,
		}
	}

	tests := []struct {
		name       string
		batchErr   error
		wantStatus []PlanItemStatus
	}{
		{
			name:       "batch succeeds",
			wantStatus: []PlanItemStatus{StatusSuccess, StatusSuccess, StatusSuccess, StatusSkipped, StatusSuccess},
		},
		{
			name:       "batch fails",
			batchErr:   fmt.Errorf("not applied"),
			wantStatus: []PlanItemStatus{StatusFailed, StatusSuccess, StatusFailed, StatusSkipped, StatusFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			bridge := macos.NewMockBridge(ctrl)

			skipped := dynItem("dyn.age80c65u", "asp")
			skipped.Status = StatusSkipped

			plan := &Plan{Items: []PlanItem{
				dynItem("dyn.age81k3u", "tf"),
				{Target: PlainTextUTITarget, DesiredApp: VSCodeApp, Status: StatusPending},
				dynItem("dyn.age80q4pxra", "fish"),
				skipped,
				dynItem("dyn.age80e2py", "bat"),
			}}

			gomock.InOrder(
				bridge.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(nil),
				bridge.EXPECT().SetDefaultForExtensions(VSCodeApp.Path, []string{"tf", "fish", "bat"}).Return(tt.batchErr).Times(1),
			)

			var progressed []int
			results, err := NewApplier(bridge).Apply(plan, func(i int, _ PlanItem) { progressed = append(progressed, i) })
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			for i, want := range tt.wantStatus {
				if results[i].Item.Status != want || plan.Items[i].Status != want {
					t.Errorf("item %d: got result %s / plan %s, want %s", i, results[i].Item.Status, plan.Items[i].Status, want)
				}
			}
			if len(progressed) != len(plan.Items) {
				t.Errorf("progress called %d times, want %d", len(progressed), len(plan.Items))
			}
		})
	}
}

func TestPlan_ConfirmableChanges(t *testing.T) {
	plan := &Plan{Items: []PlanItem{
		{Target: PlainTextUTITarget, Status: StatusPending},
		{Target: HTTPSchemeTarget, Status: StatusPending},
		{Target: Target{Kind: TargetKindUTI, Identifier: "dyn.age81k3u", Extension: "tf"}, Status: StatusPending},
		{Target: PlainTextUTITarget, Status: StatusSkipped},
	}}

	if got := plan.ConfirmableChanges(); got != 2 {
		t.Errorf("ConfirmableChanges() = %d, want 2", got)
	}
}

func TestApplier_Apply_UserDeclined(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := macos.NewMockBridge(ctrl)
	bridge.EXPECT().SetDefaultForUTI(VSCodeApp.Path, "public.plain-text").Return(dmacos.ErrUserDeclined)

	plan := &Plan{Items: []PlanItem{{Target: PlainTextUTITarget, DesiredApp: VSCodeApp, Status: StatusPending}}}
	results, err := NewApplier(bridge).Apply(plan, nil)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	r := results[0]
	if r.Item.Status != StatusSkipped || !r.Declined || r.Error != nil {
		t.Errorf("declined item: status=%s declined=%v err=%v, want skipped/true/nil", r.Item.Status, r.Declined, r.Error)
	}
}
