package main

import (
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo/ui"
)

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
