package main

import (
	"os"
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo/ui"
)

type fakeWindow struct {
	destroyed bool
}

func (w fakeWindow) IsDestroyed() bool {
	return w.destroyed
}

func TestWindowOptions(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	opts := windowOptions(state, appui.NewWorkspace())

	if opts.Title != appName {
		t.Fatalf("title = %q, want %q", opts.Title, appName)
	}
	if opts.Width != defaultWidth || opts.Height != defaultHeight {
		t.Fatalf("initial size = %dx%d, want %dx%d", opts.Width, opts.Height, defaultWidth, defaultHeight)
	}
	if opts.MinWidth != minWidth || opts.MinHeight != minHeight {
		t.Fatalf("minimum size = %dx%d, want %dx%d", opts.MinWidth, opts.MinHeight, minWidth, minHeight)
	}
	if !opts.UseContentSize {
		t.Fatal("UseContentSize is false")
	}
	if opts.Content == nil {
		t.Fatal("WindowOptions.Content is nil")
	}
}

func TestMinimumViewShowsLocalizedWorkspace(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	workspace := appui.NewWorkspace()
	tt := ui.NewTester(appui.View(state, workspace), 800, 600)

	if !tt.HasText("Interface language: English") ||
		!tt.HasText("Navigation") || !tt.HasText("Diagram canvas") || !tt.HasText("Details") {
		t.Fatalf("English minimum view missing; texts=%q", tt.Texts())
	}

	state.SelectLanguage("zh-CN")
	tt.Frame()
	if !tt.HasText("当前界面语言：简体中文。语言偏好已保存。") || !tt.HasText("导航") {
		t.Fatalf("Chinese status missing; texts=%q", tt.Texts())
	}
	if appui.RootKey == "" || appui.StatusKey == "" {
		t.Fatal("stable view keys are empty")
	}
}

func TestOptionalPositionalPDMPath(t *testing.T) {
	if _, ok := optionalPDMPath(nil); ok {
		t.Fatal("empty args selected a path")
	}
	if _, ok := optionalPDMPath([]string{"-style"}); ok {
		t.Fatal("flag selected as a path")
	}
	if _, ok := optionalPDMPath([]string{"one.pdm", "two.pdm"}); ok {
		t.Fatal("multiple args selected a path")
	}
	if got, ok := optionalPDMPath([]string{"one.pdm"}); !ok || got != "one.pdm" {
		t.Fatalf("positional path = %q, %v", got, ok)
	}
}

func TestApplyBoardResultGuardsStaleDestroyedAndFailure(t *testing.T) {
	workspace := appui.NewWorkspace()
	stale := workspace.BeginOpen()
	current := workspace.BeginOpen()
	set := appui.BuildBoardSet(nil)
	if applyBoardResult(fakeWindow{}, workspace, stale, set, nil) {
		t.Fatal("stale load result was published")
	}
	if workspace.BoardCount() != 1 || workspace.ActiveBoard() != 0 {
		t.Fatal("stale result changed the demo workspace")
	}
	if applyBoardResult(fakeWindow{destroyed: true}, workspace, current, set, nil) {
		t.Fatal("destroyed-window result was published")
	}
	if !applyBoardResult(fakeWindow{}, workspace, current, set, nil) {
		t.Fatal("current load result was rejected")
	}
	before := workspace.ActiveBoard()
	workspace.ReportOpenError()
	if workspace.ActiveBoard() != before || workspace.BoardCount() != 1 {
		t.Fatal("open error replaced the active board")
	}
}

func TestLoadBoardSetRejectsDirectoryBeforeReading(t *testing.T) {
	if _, err := loadBoardSet(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as a PDM file")
	}
	if info, err := os.Stat(t.TempDir()); err != nil || !info.IsDir() {
		t.Fatal("temporary directory check failed")
	}
}
