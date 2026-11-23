package appui

import (
	"math"
	"testing"

	"pdmview/internal/graph"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

func TestCanvasSelectionPanZoomAndFit(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(testBoardDocument()))
	tester := ui.NewTester(View(state, workspace), 1200, 800)
	view := workspace.canvas

	clickNode := func() {
		t.Helper()
		point := view.camera.WorldToScreen(graph.Point{X: 50, Y: -50})
		tester.ClickAt(float32(point.X), float32(point.Y))
		if node := view.selectedNode(); node == nil || node.ID != 0 {
			t.Fatalf("canvas selection = %+v, want node 0", view.selection)
		}
	}
	clickNode()
	if !tester.HasText(view.scene.Nodes[0].Name) {
		t.Fatal("selected node is missing from the workspace")
	}

	start := graph.Point{X: view.viewport.Min.X + 40, Y: view.viewport.Min.Y + 50}
	before := view.camera.VisibleWorld().Min
	scale := view.camera.Scale()
	tester.Press(float32(start.X), float32(start.Y))
	tester.Move(float32(start.X)+40, float32(start.Y)+25)
	tester.Release(float32(start.X)+40, float32(start.Y)+25)
	if got, want := view.camera.VisibleWorld().Min, (graph.Point{X: before.X - 40/scale, Y: before.Y - 25/scale}); math.Hypot(got.X-want.X, got.Y-want.Y) > 1e-8 {
		t.Fatalf("dragged world origin = %+v, want %+v", got, want)
	}
	clickNode()

	anchor := graph.Point{X: 175, Y: 130}
	world := view.camera.ViewportToWorld(anchor)
	view.handleInput(ui.InputEvent{Kind: ui.InputScroll, X: 175, Y: 130, DY: -40, Mods: ui.Ctrl})
	zoomed := view.camera.ViewportToWorld(anchor)
	if view.camera.Scale() <= scale || math.Hypot(zoomed.X-world.X, zoomed.Y-world.Y) > 1e-8 {
		t.Fatalf("cursor zoom moved the anchor: %+v to %+v, scale %g", world, zoomed, view.camera.Scale())
	}

	if err := tester.Click("Fit"); err != nil {
		t.Fatal(err)
	}
	fitted := view.camera.WorldToViewport(graph.Point{X: 50, Y: -50})
	if math.Abs(fitted.X-view.viewport.Width()/2) > 1e-8 || math.Abs(fitted.Y-view.viewport.Height()/2) > 1e-8 {
		t.Fatalf("fit did not center the selected node: %+v", fitted)
	}
	if err := tester.Click("All"); err != nil {
		t.Fatal(err)
	}
	worldBounds := view.camera.VisibleWorld()
	sceneBounds := view.scene.Bounds
	if worldBounds.Min.X > sceneBounds.Min.X || worldBounds.Min.Y > sceneBounds.Min.Y ||
		worldBounds.Max.X < sceneBounds.Max.X || worldBounds.Max.Y < sceneBounds.Max.Y {
		t.Fatalf("show all omitted part of the scene: view=%+v scene=%+v", worldBounds, sceneBounds)
	}

	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !tester.HasText("全部") || !tester.HasText("适应") || !tester.HasText("Diagram A") {
		t.Fatalf("Chinese canvas controls missing: %v", tester.Texts())
	}
	if view.camera.VisibleWorld() != worldBounds || view.selectedNode() == nil {
		t.Fatal("switching language changed the camera or selected node")
	}
}

func TestCanvasClearsHoverWhenPointerLeaves(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(testBoardDocument()))
	tester := ui.NewTester(View(state, workspace), 1200, 800)
	view := workspace.canvas
	point := view.camera.WorldToScreen(graph.Point{X: 50, Y: -50})
	tester.Move(float32(point.X), float32(point.Y))
	if view.hovered.Kind != graph.HitNode || view.hovered.Index != 0 {
		t.Fatalf("hover = %+v, want table 0", view.hovered)
	}
	tester.Move(4, 4)
	if view.hovered.Kind != graph.HitNone {
		t.Fatalf("hover when pointer leaves = %+v", view.hovered)
	}
}
