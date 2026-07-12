package domain

import (
	"errors"
	"testing"

	"github.com/jackchuka/dutix/internal/macos"
	mocks "github.com/jackchuka/dutix/internal/macos/mock"
	"go.uber.org/mock/gomock"
)

func TestExtensionLister_AggregatesAndResolves(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

	bridge.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"},
		{Name: "TextEdit", Path: "/Applications/TextEdit.app"},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/Visual Studio Code.app").Return([]macos.DocumentType{
		{Name: "Text", UTIs: []string{"public.plain-text"}, Extensions: []string{"txt", "md"}},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/TextEdit.app").Return([]macos.DocumentType{
		{Name: "Text", UTIs: []string{"public.plain-text"}, Extensions: []string{"txt"}},
	}, nil)

	bridge.EXPECT().ResolveUTIsForExtension("md").Return([]string{"net.daringfireball.markdown"}, nil)
	bridge.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("net.daringfireball.markdown").Return("/Applications/Visual Studio Code.app", nil)
	bridge.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)

	lister := NewExtensionLister(bridge)
	rows, err := lister.ListExtensionDefaults("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if rows[0].Extension != "md" || rows[0].UTI != "net.daringfireball.markdown" {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[0].DefaultApp == nil || rows[0].DefaultApp.Name != "Visual Studio Code" {
		t.Errorf("row0 default = %+v", rows[0].DefaultApp)
	}
	if rows[1].Extension != "txt" || rows[1].DefaultApp == nil || rows[1].DefaultApp.Name != "TextEdit" {
		t.Errorf("row1 = %+v", rows[1])
	}
}

func TestExtensionLister_FilterSubstring(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

	bridge.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "App", Path: "/Applications/App.app"},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/App.app").Return([]macos.DocumentType{
		{Extensions: []string{"txt", "json", "textbundle"}},
	}, nil)
	// Filtering happens BEFORE resolution: only "txt" survives the "txt" pattern
	// ("textbundle" has no "txt" substring, "json" doesn't match), so only "txt"
	// is resolved. Do NOT set expectations for the filtered-out extensions.
	bridge.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)

	lister := NewExtensionLister(bridge)
	rows, err := lister.ListExtensionDefaults("txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].Extension != "txt" {
		t.Fatalf("got %+v, want single txt row", rows)
	}
}

func TestExtensionLister_MultipleUTIsAndCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

	bridge.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "A", Path: "/Applications/A.app"},
		{Name: "B", Path: "/Applications/B.app"},
	}, nil)
	// Both apps declare "cfg"; dedup to one extension.
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/A.app").Return([]macos.DocumentType{
		{Extensions: []string{"cfg"}},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/B.app").Return([]macos.DocumentType{
		{Extensions: []string{"cfg"}},
	}, nil)
	// cfg resolves to two UTIs (unsorted input): b.uti, a.uti
	bridge.EXPECT().ResolveUTIsForExtension("cfg").Return([]string{"b.uti", "a.uti"}, nil)
	// Each unique UTI queried exactly once (cache).
	bridge.EXPECT().GetDefaultAppForUTI("a.uti").Return("/Applications/A.app", nil).Times(1)
	bridge.EXPECT().GetDefaultAppForUTI("b.uti").Return("", nil).Times(1)

	lister := NewExtensionLister(bridge)
	rows, err := lister.ListExtensionDefaults("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	// Sorted by UTI within extension: a.uti then b.uti
	if rows[0].UTI != "a.uti" || rows[0].DefaultApp == nil {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].UTI != "b.uti" || rows[1].DefaultApp != nil {
		t.Errorf("row1 = %+v", rows[1])
	}
}

func TestExtensionLister_NoUTIYieldsEmptyRow(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

	bridge.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "A", Path: "/Applications/A.app"},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/A.app").Return([]macos.DocumentType{
		{Extensions: []string{"weird"}},
	}, nil)
	bridge.EXPECT().ResolveUTIsForExtension("weird").Return(nil, nil)

	lister := NewExtensionLister(bridge)
	rows, err := lister.ListExtensionDefaults("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].Extension != "weird" || rows[0].UTI != "" || rows[0].DefaultApp != nil {
		t.Fatalf("got %+v, want single empty-uti row", rows)
	}
}

func TestExtensionLister_SkipsUnqueryableApps(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)

	bridge.EXPECT().ListAllApplications().Return([]macos.AppInfo{
		{Name: "Bad", Path: "/Applications/Bad.app"},
		{Name: "Good", Path: "/Applications/Good.app"},
	}, nil)
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/Bad.app").Return(nil, errors.New("boom"))
	bridge.EXPECT().ListSupportedDocumentTypes("/Applications/Good.app").Return([]macos.DocumentType{
		{Extensions: []string{"txt"}},
	}, nil)
	bridge.EXPECT().ResolveUTIsForExtension("txt").Return([]string{"public.plain-text"}, nil)
	bridge.EXPECT().GetDefaultAppForUTI("public.plain-text").Return("/Applications/TextEdit.app", nil)

	lister := NewExtensionLister(bridge)
	rows, err := lister.ListExtensionDefaults("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].Extension != "txt" {
		t.Fatalf("got %+v, want single txt row", rows)
	}
}

func TestExtensionLister_ListAppsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	bridge := mocks.NewMockBridge(ctrl)
	bridge.EXPECT().ListAllApplications().Return(nil, errors.New("boom"))

	lister := NewExtensionLister(bridge)
	if _, err := lister.ListExtensionDefaults(""); err == nil {
		t.Fatal("expected error, got nil")
	}
}
