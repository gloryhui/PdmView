package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWindowOptions(t *testing.T) {
	state := newAppState()
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

func TestBrandOnlyView(t *testing.T) {
	state := newAppState()
	state.locale = "en"
	tt := ui.NewTester(state.view, 640, 400)

	if !tt.HasText(appBrand) {
		t.Fatalf("brand text missing; texts=%q", tt.Texts())
	}
	if texts := tt.Texts(); len(texts) != 1 {
		t.Fatalf("view texts = %q, want only the app brand", texts)
	}
	if appRootKey == "" {
		t.Fatal("appRootKey is empty")
	}
}
