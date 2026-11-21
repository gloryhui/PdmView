package appui

import (
	"testing"

	"pdmview/internal/graph"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestBuildBoardSetModelThenRecursivePackagesAndPackageOwnedDefault(t *testing.T) {
	document := &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Diagrams: []pdm.Diagram{
				{Metadata: pdm.Metadata{ID: "root-a", Name: "Root A"}},
				{Metadata: pdm.Metadata{ID: "root-b", Name: "Root B"}},
			},
			Packages: []pdm.Package{
				{Metadata: pdm.Metadata{ID: "package-a"}, Contents: pdm.Contents{
					Diagrams: []pdm.Diagram{{Metadata: pdm.Metadata{ID: "package-a-board", Name: "Package A Board"}}},
					Packages: []pdm.Package{{
						Metadata: pdm.Metadata{ID: "nested"},
						Contents: pdm.Contents{Diagrams: []pdm.Diagram{{
							Metadata: pdm.Metadata{ID: "nested-board", Name: "Nested Board"},
						}}},
					}},
				}},
				{Metadata: pdm.Metadata{ID: "package-b"}, Contents: pdm.Contents{
					Diagrams: []pdm.Diagram{{Metadata: pdm.Metadata{ID: "package-b-board", Name: "Package B Board"}}},
				}},
			},
			DefaultDiagramID: "nested-board",
		},
	}}
	set := BuildBoardSet(document)
	got := make([]string, len(set.Boards))
	for offset, board := range set.Boards {
		got[offset] = board.ID
	}
	want := []string{"root-a", "root-b", "package-a-board", "nested-board", "package-b-board"}
	if !equalBoardIDs(got, want) {
		t.Fatalf("board order = %v, want %v", got, want)
	}
	if set.Default != 3 || set.Prepared.Board.ID != "nested-board" ||
		set.Prepared.Renderer == nil || set.Prepared.Index == nil {
		t.Fatalf("package-owned default = %+v", set)
	}
	if DiagramButtonKey(0) != "pdmview.workspace.diagram.0" || ResetViewKey != "pdmview.canvas.reset" {
		t.Fatal("stable board/reset keys changed")
	}
}

func TestBuildBoardSetFallbackWhenModelHasNoDiagrams(t *testing.T) {
	document := testBoardDocument()
	document.Model.Contents.Diagrams = nil
	document.Model.Contents.DefaultDiagramID = ""
	set := BuildBoardSet(document)
	if len(set.Boards) != 1 || !set.Boards[0].Fallback || set.Default != 0 {
		t.Fatalf("fallback catalog = %+v", set)
	}
	if set.Prepared.Board.Diagram != nil || set.Prepared.Renderer == nil ||
		len(set.Prepared.Graph.Scene.Nodes) != 2 {
		t.Fatalf("fallback prepared board = %+v", set.Prepared)
	}
}

func TestWorkspacePublishesRealBoardsAndStaleSwitchCompletion(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	if workspace.BoardCount() != 1 || workspace.ActiveBoard() != 0 {
		t.Fatal("workspace did not start with only the demo")
	}
	set := BuildBoardSet(testBoardDocument())
	token := workspace.BeginOpen()
	if !workspace.PublishBoards(token, set) {
		t.Fatal("published board set was rejected")
	}
	if workspace.BoardCount() != 2 || workspace.ActiveBoard() != 0 {
		t.Fatalf("real catalog after open = %d boards, active %d", workspace.BoardCount(), workspace.ActiveBoard())
	}

	tester := ui.NewTester(View(state, workspace), 800, 600)
	view := workspace.canvas
	if view.viewport.Min.X <= 0 {
		t.Fatalf("canvas origin is not offset: %+v", view.viewport)
	}
	visible, bounds := view.camera.VisibleWorld(), view.scene.Bounds
	if visible.Min.X > bounds.Min.X || visible.Min.Y > bounds.Min.Y ||
		visible.Max.X < bounds.Max.X || visible.Max.Y < bounds.Max.Y {
		t.Fatalf("default real board is not fitted: visible=%+v bounds=%+v", visible, bounds)
	}
	if tester.HasText(state.Text(i18n.KeySampleDiagram, nil)) {
		t.Fatal("demo remained in the catalog after opening a model")
	}

	world := graph.Point{X: 50, Y: 50}
	screen := view.camera.WorldToScreen(world)
	local := view.camera.ScreenToViewport(screen)
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerMove, X: float32(local.X), Y: float32(local.Y)})
	if view.hovered.Kind != graph.HitNode || view.hovered.Index != 0 {
		t.Fatalf("formal hover through nonzero origin = %+v", view.hovered)
	}
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: float32(local.X), Y: float32(local.Y), Button: 0})
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerUp, X: float32(local.X), Y: float32(local.Y), Button: 0})
	if view.selection.Kind != graph.HitNode || view.selection.Index != 0 {
		t.Fatalf("formal selection through nonzero origin = %+v", view.selection)
	}

	descriptor, ok := workspace.BoardDescriptor(1)
	if !ok {
		t.Fatal("second board descriptor missing")
	}
	oldRenderer := view.renderer
	firstToken, _ := workspace.BeginBoardSwitch(1)
	secondToken, _ := workspace.BeginBoardSwitch(1)
	prepared := PrepareBoard(descriptor)
	if workspace.PublishPreparedBoard(firstToken, prepared) {
		t.Fatal("stale board switch was published")
	}
	if view.selection.Kind != graph.HitNode || view.renderer != oldRenderer {
		t.Fatal("stale board switch changed the active board")
	}
	if !workspace.PublishPreparedBoard(secondToken, prepared) {
		t.Fatal("current board switch was rejected")
	}
	if workspace.ActiveBoard() != 1 || view.selection.Kind != graph.HitNone ||
		view.hovered.Kind != graph.HitNone || view.renderer == oldRenderer {
		t.Fatalf("board switch state = active:%d selection:%+v hover:%+v",
			workspace.ActiveBoard(), view.selection, view.hovered)
	}
	pendingToken, ok := workspace.BeginBoardSwitch(0)
	if !ok {
		t.Fatal("pending switch was not started")
	}
	workspace.ActivateBoard(1)
	firstDescriptor, _ := workspace.BoardDescriptor(0)
	if workspace.PublishPreparedBoard(pendingToken, PrepareBoard(firstDescriptor)) {
		t.Fatal("visible-board click did not cancel the pending switch")
	}
	if workspace.ActiveBoard() != 1 {
		t.Fatal("last user intent did not keep the visible board")
	}
	view.selection = graph.Hit{Kind: graph.HitNode, Index: 0}
	view.hovered = graph.Hit{Kind: graph.HitNode, Index: 0}
	view.pointerDown, view.dragging = true, true
	view.resetView()
	if view.selection.Kind != graph.HitNone || view.hovered.Kind != graph.HitNone ||
		view.pointerDown || view.dragging {
		t.Fatal("reset did not clear interaction state")
	}
}

func TestWorkspaceAutoLayoutFallbackStatusAndLocale(t *testing.T) {
	state := newState(t, "en-US")
	document := testBoardDocument()
	document.Model.Contents.Diagrams[0].TableSymbols = append(
		document.Model.Contents.Diagrams[0].TableSymbols,
		pdm.TableSymbol{Metadata: pdm.Metadata{ID: "missing-rect"}, TableID: "table-b"},
	)
	document.Model.Contents.Diagrams = document.Model.Contents.Diagrams[:1]
	document.Model.Contents.Packages = nil
	workspace := NewWorkspace()
	token := workspace.BeginOpen()
	workspace.PublishBoards(token, BuildBoardSet(document))
	tester := ui.NewTester(View(state, workspace), 800, 600)
	if !tester.HasText("Includes automatically arranged tables") {
		t.Fatalf("English auto-layout status missing: %q", tester.Texts())
	}
	if info, ok := tester.Access("Diagram A"); !ok || !info.Selected {
		t.Fatalf("active real board is not AccessSelected: %+v, %v", info, ok)
	}

	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !tester.HasText("含自动布局的表") || !tester.HasText("重置视图") {
		t.Fatalf("Chinese board controls/status missing: %q", tester.Texts())
	}

	fallbackModel := testBoardDocument().Model
	fallbackModel.Contents.Diagrams = nil
	fallbackModel.Contents.DefaultDiagramID = ""
	fallback := BuildBoardSet(&pdm.Document{Model: fallbackModel})
	fallbackWorkspace := NewWorkspace()
	token = fallbackWorkspace.BeginOpen()
	fallbackWorkspace.PublishBoards(token, fallback)
	fallbackTester := ui.NewTester(View(state, fallbackWorkspace), 800, 600)
	if !fallbackTester.HasText("无画板 · 自动布局") {
		t.Fatalf("fallback status missing: %q", fallbackTester.Texts())
	}
}

func TestCanvasControlsFitAtMinimumWindowBothLocales(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.EN, i18n.ZHCN} {
		state := newState(t, string(locale))
		if locale == i18n.ZHCN {
			state.SelectLanguage(locale)
		}
		tester := ui.NewTester(View(state, NewWorkspace()), 800, 600)
		for _, label := range []string{
			state.Text(i18n.KeyOpenPDM, nil),
			state.Text(i18n.KeyFitSelection, nil),
			state.Text(i18n.KeyShowAll, nil),
			state.Text(i18n.KeyResetView, nil),
		} {
			bounds, ok := tester.Find(label)
			if !ok || bounds.X < 0 || bounds.Y < 0 ||
				bounds.X+bounds.W > 801 || bounds.Y+bounds.H > 601 {
				t.Fatalf("%s control %q outside 800x600: %+v, %v", locale, label, bounds, ok)
			}
		}
	}
}

func testBoardDocument() *pdm.Document {
	tableA := pdm.Table{Metadata: pdm.Metadata{ID: "table-a", Name: "A", Code: "A"}}
	tableB := pdm.Table{Metadata: pdm.Metadata{ID: "table-b", Name: "B", Code: "B"}}
	diagramA := pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram-a", Name: "Diagram A"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "a-symbol"}, TableID: "table-a", Rect: boardRect(0, 0, 100, 100)},
			{Metadata: pdm.Metadata{ID: "b-symbol"}, TableID: "table-b", Rect: boardRect(200, 0, 300, 100)},
		},
	}
	diagramB := pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram-b", Name: "Diagram B"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "a-symbol-b"}, TableID: "table-a", Rect: boardRect(0, 0, 120, 120)},
		},
	}
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables:           []pdm.Table{tableA, tableB},
			Diagrams:         []pdm.Diagram{diagramA, diagramB},
			DefaultDiagramID: "diagram-a",
		},
	}}
}

func boardRect(minX, minY, maxX, maxY int64) *pdm.Rect {
	return &pdm.Rect{
		Min: pdm.Point{X: minX, Y: minY},
		Max: pdm.Point{X: maxX, Y: maxY},
	}
}

func equalBoardIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for offset := range got {
		if got[offset] != want[offset] {
			return false
		}
	}
	return true
}
