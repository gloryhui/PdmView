package appui

import (
	"os"
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	"pdmview/internal/graph"
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
	drag(t, tt, splitters[0], 32, 0)
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

func TestWorkspacePaneBoundsToolbarAndCanvasState(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	tester := ui.NewTester(View(state, workspace), 800, 600)
	splitters := splitterBounds(tester)
	if len(splitters) != 2 {
		t.Fatalf("splitters = %d", len(splitters))
	}
	if splitters[0].X < 199 || splitters[0].X > 361 {
		t.Fatalf("navigation pane width outside limits: %+v", splitters[0])
	}
	detailsWidth := 800 - splitters[1].X
	if detailsWidth < 219 || detailsWidth > 421 {
		t.Fatalf("details pane width outside limits: %+v", splitters[1])
	}
	if scrolls := len(scrollAreas(tester)); scrolls < 2 {
		t.Fatalf("pane scroll areas = %d, want navigation and details", scrolls)
	}
	canvasLeft, canvasRight := splitters[0].X, splitters[1].X
	for _, label := range []string{"Zoom out", "Zoom in", "Fit selection", "Show all", "Reset view"} {
		info, ok := tester.Access(label)
		if !ok || info.Bounds.X < canvasLeft-0.5 ||
			info.Bounds.X+info.Bounds.W > canvasRight+0.5 ||
			info.Bounds.Y+info.Bounds.H > 601 {
			t.Fatalf("800x600 canvas tool %q outside canvas %.2f..%.2f: %+v, %v",
				label, canvasLeft, canvasRight, info.Bounds, ok)
		}
	}

	view := workspace.canvas
	point := view.camera.WorldToScreen(graph.Point{X: 100, Y: 55})
	tester.ClickAt(float32(point.X), float32(point.Y))
	if view.selection.Kind != graph.HitNode {
		t.Fatalf("selection before resize = %+v", view.selection)
	}
	view.camera.ZoomAt(graph.Point{X: 10, Y: 10}, 1.5)
	scale, origin := view.camera.Scale(), view.camera.VisibleWorld().Min

	tester.SetSize(1000, 700)
	tester.Frame()
	if view.camera.Scale() != scale || view.camera.VisibleWorld().Min != origin ||
		view.selection.Kind != graph.HitNode {
		t.Fatalf("resize changed canvas state: scale=%v origin=%+v selection=%+v",
			view.camera.Scale(), view.camera.VisibleWorld().Min, view.selection)
	}

	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if view.camera.Scale() != scale || view.camera.VisibleWorld().Min != origin ||
		view.selection.Kind != graph.HitNode {
		t.Fatal("locale change altered canvas state")
	}
	tester.SetSize(1600, 900)
	tester.Frame()
	splitters = splitterBounds(tester)
	if splitters[0].X > 361 || 1600-splitters[1].X > 421 ||
		splitters[1].X-splitters[0].X < canvasMinWidth-1 {
		t.Fatalf("large-window pane allocation outside limits: %+v", splitters)
	}

	splitters = splitterBounds(tester)
	canvasLeft, canvasRight = splitters[0].X, splitters[1].X
	for _, label := range []string{"缩小", "放大", "适应选中", "显示全部", "重置视图"} {
		info, ok := tester.Access(label)
		if !ok || info.Bounds.X < canvasLeft-0.5 ||
			info.Bounds.X+info.Bounds.W > canvasRight+0.5 ||
			info.Bounds.Y+info.Bounds.H > 901 {
			t.Fatalf("canvas tool %q outside canvas bounds %.2f..%.2f: %+v, %v",
				label, canvasLeft, canvasRight, info.Bounds, ok)
		}
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

func scrollAreas(tt *ui.Tester) []ui.Rect {
	var bounds []ui.Rect
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RoleScroll {
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
