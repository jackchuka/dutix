package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackchuka/dutix/internal/macos"
	mocks "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

const (
	chosenPath = "/nix/store/aaa-foo/Applications/Foo.app"
	newerPath  = "/nix/store/zzz-foo/Applications/Foo.app"
	otherPath  = "/Applications/Foo.app"
)

var chosenApp = &App{Name: "Foo", BundleID: "org.example.foo", Path: chosenPath, Version: "1"}

func versionsByPath(m *mocks.MockBridge) {
	versions := map[string]string{newerPath: "2", otherPath: "1"}
	m.EXPECT().AppInfoAtPath(gomock.Any()).DoAndReturn(func(p string) (macos.AppInfo, error) {
		return macos.AppInfo{Path: p, BundleID: "org.example.foo", Version: versions[p]}, nil
	}).AnyTimes()
}

func TestInspectCopies(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	versionsByPath(m)
	m.EXPECT().ApplicationsForBundleID("org.example.foo").Return([]string{newerPath, chosenPath + "/", otherPath}, nil)

	r, err := InspectCopies(m, chosenApp)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Registered() || chosenWins(r) {
		t.Errorf("Registered=%v chosenWins=%v, want true,false", r.Registered(), chosenWins(r))
	}
	if w := r.Winner(); w == nil || w.Path != newerPath || w.Version != "2" {
		t.Errorf("Winner = %+v", w)
	}
	if others := r.Others(); len(others) != 2 || others[0].Path != newerPath || others[1].Path != otherPath {
		t.Errorf("Others = %+v", others)
	}
	if !r.Copies[1].Chosen || r.Copies[1].Version != "1" {
		t.Errorf("chosen copy = %+v", r.Copies[1])
	}
}

func TestInspectCopies_NoBundleID(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockBridge(ctrl)
	if _, err := InspectCopies(m, &App{Path: chosenPath}); err == nil {
		t.Error("want error for app without bundle ID")
	}
}

func TestEnsureWinner(t *testing.T) {
	const id = "org.example.foo"
	tests := []struct {
		name           string
		opts           EnsureOptions
		setup          func(m *mocks.MockBridge)
		wantWins       bool
		wantErr        string
		wantWinner     string
		wantUnreg      []string
		wantRegistered bool
	}{
		{
			name: "already registered and winning: no changes",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{chosenPath}, nil).Times(1)
			},
			wantWins:   true,
			wantWinner: chosenPath,
		},
		{
			name: "registers when not registered",
			opts: EnsureOptions{},
			setup: func(m *mocks.MockBridge) {
				gomock.InOrder(
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{otherPath}, nil),
					m.EXPECT().RegisterApplication(chosenPath).Return(nil),
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{chosenPath, otherPath}, nil),
				)
			},
			wantWins:       true,
			wantWinner:     chosenPath,
			wantRegistered: true,
		},
		{
			name: "outranked without prune leaves others registered",
			opts: EnsureOptions{},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath, chosenPath}, nil)
			},
			wantWinner: newerPath,
		},
		{
			name: "prune unregisters exactly the others",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				gomock.InOrder(
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath, chosenPath, otherPath}, nil),
					m.EXPECT().UnregisterApplication(newerPath).Return(nil),
					m.EXPECT().UnregisterApplication(otherPath).Return(nil),
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{chosenPath}, nil),
				)
			},
			wantWins:   true,
			wantWinner: chosenPath,
			wantUnreg:  []string{newerPath, otherPath},
		},
		{
			name: "prune and register an unregistered copy",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				gomock.InOrder(
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath}, nil),
					m.EXPECT().RegisterApplication(chosenPath).Return(nil),
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath, chosenPath}, nil),
					m.EXPECT().UnregisterApplication(newerPath).Return(nil),
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{chosenPath}, nil),
				)
			},
			wantWins:       true,
			wantWinner:     chosenPath,
			wantUnreg:      []string{newerPath},
			wantRegistered: true,
		},
		{
			name: "prune keeps others when registering fails",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath}, nil)
				m.EXPECT().RegisterApplication(chosenPath).Return(errors.New("OSStatus -10811"))
			},
			wantErr:    "not unregistering other copies",
			wantWinner: newerPath,
		},
		{
			name: "prune keeps others when the registration doesn't take",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				gomock.InOrder(
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath}, nil),
					m.EXPECT().RegisterApplication(chosenPath).Return(nil),
					m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath}, nil).Times(2),
				)
			},
			wantErr:        "not unregistering other copies",
			wantWinner:     newerPath,
			wantRegistered: true,
		},
		{
			name: "register error surfaces",
			opts: EnsureOptions{},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath}, nil)
				m.EXPECT().RegisterApplication(chosenPath).Return(errors.New("OSStatus -10811"))
			},
			wantErr:    "failed to register",
			wantWinner: newerPath,
		},
		{
			name: "still outranked after prune",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath, chosenPath}, nil).Times(2)
				m.EXPECT().UnregisterApplication(newerPath).Return(nil)
			},
			wantWinner: newerPath,
			wantUnreg:  []string{newerPath},
		},
		{
			name: "unregister error surfaces",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return([]string{newerPath, chosenPath}, nil)
				m.EXPECT().UnregisterApplication(newerPath).Return(errors.New("exit status 1"))
			},
			wantErr:    "failed to unregister",
			wantWinner: newerPath,
		},
		{
			name: "lookup error",
			opts: EnsureOptions{Prune: true},
			setup: func(m *mocks.MockBridge) {
				m.EXPECT().ApplicationsForBundleID(id).Return(nil, errors.New("boom"))
			},
			wantErr: "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mocks.NewMockBridge(ctrl)
			versionsByPath(m)
			tt.setup(m)

			r, err := EnsureWinner(m, chosenApp, tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if chosenWins(r.CopyReport) != tt.wantWins {
				t.Errorf("ChosenWins = %v, want %v", chosenWins(r.CopyReport), tt.wantWins)
			}
			if tt.wantWinner != "" {
				if w := r.Winner(); w == nil || w.Path != tt.wantWinner {
					t.Errorf("Winner = %+v, want %s", w, tt.wantWinner)
				}
			}
			if strings.Join(r.Unregistered, ",") != strings.Join(tt.wantUnreg, ",") {
				t.Errorf("Unregistered = %v, want %v", r.Unregistered, tt.wantUnreg)
			}
			if r.DidRegister != tt.wantRegistered {
				t.Errorf("DidRegister = %v, want %v", r.DidRegister, tt.wantRegistered)
			}
			if r.Changed() != (tt.wantRegistered || len(tt.wantUnreg) > 0) {
				t.Errorf("Changed = %v", r.Changed())
			}
		})
	}
}

func chosenWins(r CopyReport) bool {
	w := r.Winner()
	return w != nil && w.Chosen
}
