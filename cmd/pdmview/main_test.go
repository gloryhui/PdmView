package main

import (
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

func TestWindowOptions(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	opts := windowOptions(state)

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

func TestMinimumViewShowsLocalizedStatus(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	tt := ui.NewTester(view(state), 640, 400)

	if !tt.HasText("PdmView") || !tt.HasText("Interface language: English") {
		t.Fatalf("English minimum view missing; texts=%q", tt.Texts())
	}
	if texts := tt.Texts(); len(texts) != 2 {
		t.Fatalf("view texts = %q, want brand and status", texts)
	}

	state.SelectLanguage(i18n.ZHCN)
	tt = ui.NewTester(view(state), 640, 400)
	if !tt.HasText("当前界面语言：简体中文。语言偏好已保存。") {
		t.Fatalf("Chinese status missing; texts=%q", tt.Texts())
	}
	if appRootKey == "" || appStatusKey == "" {
		t.Fatal("stable view keys are empty")
	}
}
