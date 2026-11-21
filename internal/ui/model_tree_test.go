package appui

import (
	"fmt"
	"testing"

	"pdmview/internal/graph"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestModelTreeHierarchyDisclosureStableSelectionAndLocale(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	token := workspace.BeginOpen()
	if !workspace.PublishBoards(token, BuildBoardSet(modelNavDocument())) {
		t.Fatal("model boards were rejected")
	}
	tester := ui.NewTester(View(state, workspace), 1000, 700)

	root, ok := tester.AccessByRole(ui.RoleTreeItem, "Sales")
	if !ok || !root.Expanded || !root.Expandable {
		t.Fatalf("model root = %+v, %v", root, ok)
	}
	pkg, ok := tester.AccessByRole(ui.RoleTreeItem, "Package A")
	if !ok || pkg.Expanded || !pkg.Expandable {
		t.Fatalf("package initial disclosure = %+v, %v", pkg, ok)
	}
	if tester.HasText("Customers") {
		t.Fatal("collapsed package table is visible")
	}
	if got := workspace.outline.Item(0); got != ModelRootKey {
		t.Fatalf("root outline key = %q", got)
	}

	if err := tester.Click("Package A"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, ui.KeyRight)
	tester.Frame()
	pkg, ok = tester.AccessByRole(ui.RoleTreeItem, "Package A")
	if !ok || !pkg.Expanded {
		t.Fatalf("package did not expand through native disclosure: %+v, %v", pkg, ok)
	}
	if !tester.HasText("Customers") {
		t.Fatalf("expanded package table missing: %q", tester.Texts())
	}

	if err := tester.Click("Customers"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if workspace.selectedTableID != "customers" ||
		!workspace.navChosen.Has(TableNavKey("customers")) {
		t.Fatalf("table selection = %q chosen=%v", workspace.selectedTableID, workspace.navChosen.Has(TableNavKey("customers")))
	}
	if workspace.canvas.selection.Kind != graph.HitNode {
		t.Fatalf("canvas selection = %+v", workspace.canvas.selection)
	}
	if info, ok := tester.AccessByRole(ui.RoleTreeItem, "Customers"); !ok || !info.Selected {
		t.Fatalf("table row selected state = %+v, %v", info, ok)
	}
	workspace.outline.Open.Remove(PackageNavKey("package-a"))
	workspace.navChosen.Clear()
	workspace.navChosen.Add(PackageNavKey("package-a"))
	workspace.reconcileOutlineSelection(true)
	if workspace.selectedTableID != "customers" ||
		!workspace.navChosen.Has(TableNavKey("customers")) ||
		workspace.canvas.selection.Kind != graph.HitNode {
		t.Fatal("collapsing a package cleared its selected child table")
	}
	workspace.outline.Open.Add(PackageNavKey("package-a"))

	scale, origin := workspace.canvas.camera.Scale(), workspace.canvas.camera.VisibleWorld().Min
	selection := workspace.canvas.selection
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !workspace.outline.Open.Has(PackageNavKey("package-a")) ||
		!workspace.navChosen.Has(TableNavKey("customers")) ||
		workspace.canvas.camera.Scale() != scale ||
		workspace.canvas.camera.VisibleWorld().Min != origin ||
		workspace.canvas.selection != selection {
		t.Fatal("locale change lost tree expansion, table selection, or camera")
	}
}

func TestLocateTableCurrentCrossBoardStaleAndUnplaced(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	token := workspace.BeginOpen()
	workspace.PublishBoards(token, BuildBoardSet(modelNavDocument()))

	if !workspace.LocateTable("customers") || workspace.ActiveBoard() != 0 {
		t.Fatalf("current-board locate active=%d", workspace.ActiveBoard())
	}
	if got := selectedTableIDOnCanvas(workspace.canvas); got != "customers" {
		t.Fatalf("current-board selected table = %q", got)
	}

	if !workspace.LocateTable("orders") || workspace.ActiveBoard() != 1 {
		t.Fatalf("cross-board locate active=%d", workspace.ActiveBoard())
	}
	if got := selectedTableIDOnCanvas(workspace.canvas); got != "orders" {
		t.Fatalf("cross-board selected table = %q", got)
	}
	if !workspace.LocateTable("customers") || workspace.ActiveBoard() != 1 {
		t.Fatal("current diagram was not preferred for a shared table")
	}

	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(modelNavDocument()))
	descriptor, ok := workspace.BoardDescriptor(1)
	if !ok {
		t.Fatal("second board descriptor missing")
	}
	staleToken, ok := workspace.BeginBoardSwitch(1)
	if !ok {
		t.Fatal("switch did not start")
	}
	if !workspace.LocateTable("customers") {
		t.Fatal("last table selection failed")
	}
	if workspace.PublishPreparedBoard(staleToken, PrepareBoard(descriptor)) {
		t.Fatal("stale cross-board completion was accepted")
	}
	if workspace.ActiveBoard() != 0 || selectedTableIDOnCanvas(workspace.canvas) != "customers" {
		t.Fatal("stale completion replaced the last table selection")
	}

	nodesBefore := len(workspace.canvas.scene.Nodes)
	if workspace.LocateTable("package-child-table") {
		t.Fatal("unplaced table reported a diagram")
	}
	if !workspace.unplacedTable || workspace.selectedTableName != "Child Table" ||
		workspace.canvas.selection.Kind != graph.HitNone ||
		len(workspace.canvas.scene.Nodes) != nodesBefore {
		t.Fatalf("unplaced state = %+v selection=%+v nodes=%d",
			workspace, workspace.canvas.selection, len(workspace.canvas.scene.Nodes))
	}
	tester := ui.NewTester(View(state, workspace), 1000, 700)
	tester.Frame()
	if !tester.HasText("Child Table") || !tester.HasText("Not on a diagram") {
		t.Fatalf("unplaced details missing: %q", tester.Texts())
	}
}

func TestCanvasSelectionSynchronizesDomainTableAndClears(t *testing.T) {
	workspace := NewWorkspace()
	token := workspace.BeginOpen()
	workspace.PublishBoards(token, BuildBoardSet(modelNavDocument()))
	view := workspace.canvas
	if offset, ok := view.tableNodeOffset("customers"); ok {
		view.selectTableNode(offset)
		view.reportTableSelection()
	} else {
		t.Fatal("customers node missing")
	}
	if workspace.selectedTableID != "customers" || !workspace.navChosen.Has(TableNavKey("customers")) {
		t.Fatalf("canvas-to-domain selection = %q", workspace.selectedTableID)
	}
	view.clearSelection()
	view.reportTableSelection()
	if workspace.selectedTableID != "" || workspace.navChosen.Has(TableNavKey("customers")) {
		t.Fatal("canvas clear did not clear domain/table selection")
	}
	if !workspace.outline.Open.Has(PackageNavKey("package-a")) {
		t.Fatal("canvas selection did not reveal its package")
	}
}

func TestVisibleBoardCancelsPendingTableLocation(t *testing.T) {
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(modelNavDocument()))
	var pendingToken uint64
	workspace.SetBoardSelector(func(index int) {
		pendingToken, _ = workspace.BeginBoardSwitch(index)
	})
	workspace.LocateTable("customers")
	if !workspace.LocateTable("orders") || pendingToken == 0 {
		t.Fatal("cross-board locate did not start")
	}
	workspace.ActivateBoard(0)
	descriptor, _ := workspace.BoardDescriptor(1)
	if workspace.PublishPreparedBoard(pendingToken, PrepareBoard(descriptor)) ||
		workspace.selectedTableID != "" || workspace.canvas.selection.Kind != graph.HitNone {
		t.Fatal("canceled locate retained its table or published the destination board")
	}
}

func TestOwnerScopedSymbolLessFallbackLocatesTable(t *testing.T) {
	document := &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model", Name: "Model"},
		Contents: pdm.Contents{
			Packages: []pdm.Package{{
				Metadata: pdm.Metadata{ID: "package"},
				Contents: pdm.Contents{
					Tables: []pdm.Table{{Metadata: pdm.Metadata{ID: "owned-table", Name: "Owned"}}},
					Diagrams: []pdm.Diagram{{
						Metadata: pdm.Metadata{ID: "owner-fallback", Name: "Fallback"},
						OwnerID:  "package",
					}},
				},
			}},
		},
	}}
	set := BuildBoardSet(document)
	if !containsTableID(set.Boards[0].TableIDs, "owned-table") {
		t.Fatalf("owner fallback board table IDs = %v", set.Boards[0].TableIDs)
	}
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), set)
	if !workspace.LocateTable("owned-table") {
		t.Fatal("owner fallback table did not locate")
	}
	source, ok := workspace.canvas.diagram.NodeSource(workspace.canvas.selection.Index)
	if !ok || source.TableID != "owned-table" || source.SymbolID != "" {
		t.Fatalf("owner fallback source = %+v, %v", source, ok)
	}
}

func TestBuildBoardSetBuildsLargeCatalogOfScreen(t *testing.T) {
	model := pdm.Model{Metadata: pdm.Metadata{ID: "model", Name: "Large"}}
	for index := 0; index < 1000; index++ {
		model.Contents.Tables = append(model.Contents.Tables, pdm.Table{
			Metadata: pdm.Metadata{ID: fmt.Sprintf("table-%04d", index), Name: fmt.Sprintf("Table %04d", index)},
		})
	}
	set := BuildBoardSet(&pdm.Document{Model: model})
	if set.Catalog == nil || len(set.Catalog.nodes) != 1001 || set.Catalog.root != ModelRootKey {
		t.Fatalf("large catalog = %+v", set.Catalog)
	}
}

func modelNavDocument() *pdm.Document {
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model", Name: "Sales"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{
				{Metadata: pdm.Metadata{ID: "root-table", Name: "Root Table"}},
			},
			Packages: []pdm.Package{
				{Metadata: pdm.Metadata{ID: "package-a", Name: "Package A"}, Contents: pdm.Contents{
					Tables: []pdm.Table{
						{Metadata: pdm.Metadata{ID: "customers", Name: "Customers"}},
					},
					Packages: []pdm.Package{{
						Metadata: pdm.Metadata{ID: "package-child", Name: "Package Child"},
						Contents: pdm.Contents{Tables: []pdm.Table{
							{Metadata: pdm.Metadata{ID: "package-child-table", Name: "Child Table"}},
						}},
					}},
				}},
				{Metadata: pdm.Metadata{ID: "package-b", Name: "Package B"}, Contents: pdm.Contents{
					Tables: []pdm.Table{{Metadata: pdm.Metadata{ID: "orders", Name: "Orders"}}},
				}},
			},
			Diagrams: []pdm.Diagram{
				{
					Metadata: pdm.Metadata{ID: "diagram-a", Name: "Diagram A"},
					OwnerID:  "model",
					TableSymbols: []pdm.TableSymbol{{
						Metadata: pdm.Metadata{ID: "customers-symbol"},
						TableID:  "customers",
						Rect:     boardRect(0, 0, 100, 100),
					}},
				},
				{
					Metadata: pdm.Metadata{ID: "diagram-b", Name: "Diagram B"},
					OwnerID:  "model",
					TableSymbols: []pdm.TableSymbol{
						{Metadata: pdm.Metadata{ID: "orders-symbol"}, TableID: "orders", Rect: boardRect(200, 0, 300, 100)},
						{Metadata: pdm.Metadata{ID: "customers-b-symbol"}, TableID: "customers", Rect: boardRect(0, 0, 100, 100)},
					},
				},
			},
			DefaultDiagramID: "diagram-a",
		},
	}}
}

func selectedTableIDOnCanvas(view *canvasView) string {
	if view.selection.Kind != graph.HitNode {
		return ""
	}
	source, ok := view.diagram.NodeSource(view.selection.Index)
	if !ok {
		return ""
	}
	return source.TableID
}
