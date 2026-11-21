package appui

import (
	"os"
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

func TestWorkspaceLayoutBothLocalesAndWidths(t *testing.T) {
	tests := []struct {
		name      string
		systemTag string
		width     int
		height    int
		headings  []string
	}{
		{name: "english wide", systemTag: "en-US", width: 1200, height: 800, headings: []string{"Navigation", "Diagram canvas", "Details"}},
		{name: "chinese minimum", systemTag: "zh-Hans-CN", width: 800, height: 600, headings: []string{"导航", "图表画布", "详情"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := newState(t, tc.systemTag)
			tt := ui.NewTester(View(state, NewWorkspace()), tc.width, tc.height)
			for _, heading := range tc.headings {
				if !tt.HasText(heading) {
					t.Errorf("heading %q missing; texts=%q", heading, tt.Texts())
				}
			}
			if got := len(splitterBounds(tt)); got != 2 {
				t.Fatalf("splitters = %d, want 2", got)
			}
			status, ok := tt.Find(state.StatusText())
			if !ok {
				t.Fatalf("status not found; texts=%q", tt.Texts())
			}
			if status.X < 0 || status.Y < 0 || status.X+status.W > float32(tc.width)+1 ||
				status.Y+status.H > float32(tc.height)+1 {
				t.Fatalf("status outside %dx%d: %+v", tc.width, tc.height, status)
			}
		})
	}
}

func TestWorkspaceSplitterDragKeyboardAndLocaleRefresh(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	tt := ui.NewTester(View(state, workspace), 1200, 800)
	splitters := splitterBounds(tt)
	if len(splitters) != 2 {
		t.Fatalf("splitters = %d, want 2", len(splitters))
	}

	navBefore := workspace.NavigationPercent
	drag(t, tt, splitters[0], 64, 0)
	if workspace.NavigationPercent <= navBefore {
		t.Fatalf("navigation splitter drag kept percent at %v", workspace.NavigationPercent)
	}
	navAfterDrag := workspace.NavigationPercent
	tt.Key(0, ui.KeyRight)
	if workspace.NavigationPercent <= navAfterDrag {
		t.Fatalf("navigation splitter keyboard move kept percent at %v", workspace.NavigationPercent)
	}

	splitters = splitterBounds(tt)
	if len(splitters) != 2 {
		t.Fatalf("splitters after drag = %d, want 2", len(splitters))
	}
	canvasBefore := workspace.CanvasPercent
	drag(t, tt, splitters[1], -48, 0)
	if workspace.CanvasPercent >= canvasBefore {
		t.Fatalf("details splitter drag kept canvas percent at %v", workspace.CanvasPercent)
	}

	navKept, canvasKept := workspace.NavigationPercent, workspace.CanvasPercent
	state.SelectLanguage(i18n.ZHCN)
	tt.Frame()
	if workspace.NavigationPercent != navKept || workspace.CanvasPercent != canvasKept {
		t.Fatalf("locale refresh changed splits: nav %v->%v canvas %v->%v",
			navKept, workspace.NavigationPercent, canvasKept, workspace.CanvasPercent)
	}
	if !tt.HasText("导航") || !tt.HasText("图表画布") || !tt.HasText("详情") {
		t.Fatalf("Chinese workspace labels missing after refresh; texts=%q", tt.Texts())
	}
}

func TestLongWarningClampsAtMinimumWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"language":`), 0o600); err != nil {
		t.Fatal(err)
	}
	state := app.New(app.NewStore(path), "en-US")
	tt := ui.NewTester(View(state, NewWorkspace()), 800, 600)
	status, ok := tt.Find(state.StatusText())
	if !ok {
		t.Fatalf("warning text not found; texts=%q", tt.Texts())
	}
	if status.Y+status.H > 601 {
		t.Fatalf("warning leaves the 800x600 window: %+v", status)
	}
}

func newState(t *testing.T, systemTag string) *app.State {
	t.Helper()
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	return app.New(app.NewStore(path), systemTag)
}

func splitterBounds(tt *ui.Tester) []ui.Rect {
	var bounds []ui.Rect
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RoleSplitter {
			bounds = append(bounds, info.Bounds)
		}
	}
	return bounds
}

func drag(t *testing.T, tt *ui.Tester, r ui.Rect, dx, dy float32) {
	t.Helper()
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x+dx, y+dy)
	tt.Release(x+dx, y+dy)
}
