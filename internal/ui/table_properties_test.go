package appui

import (
	"fmt"
	"strings"
	"testing"

	"pdmview/internal/graph"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
)

func TestTablePropertiesDoubleClickEvent(t *testing.T) {
	scene := graph.Scene{Nodes: []graph.Node{{
		ID:     0,
		Bounds: graph.Rect{Min: graph.Point{X: 0, Y: 0}, Max: graph.Point{X: 100, Y: 100}},
	}}}
	view := &canvasView{
		scene: scene,
		index: graph.NewSpatialIndex(scene),
		diagram: graph.DiagramGraph{
			Scene:       scene,
			NodeSources: []graph.DiagramNodeSource{{SymbolID: "symbol", TableID: "orders"}},
		},
		camera: graph.NewCamera(graph.Rect{Max: graph.Point{X: 800, Y: 600}}),
	}
	var opened []string
	view.onTableOpen = func(tableID string) { opened = append(opened, tableID) }

	click := func(clicks int, canceled bool) {
		view.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Y: 50, Button: 0})
		view.handleInput(ui.InputEvent{
			Kind: ui.InputPointerUp, X: 50, Y: 50, Button: 0, Clicks: clicks, Canceled: canceled,
		})
	}

	click(1, false)
	if len(opened) != 0 {
		t.Fatalf("single click opened %v", opened)
	}
	click(2, false)
	if len(opened) != 1 || opened[0] != "orders" {
		t.Fatalf("double click opened %v", opened)
	}
	click(2, true)
	if len(opened) != 1 {
		t.Fatalf("canceled double click opened %v", opened)
	}

	view.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Y: 50, Button: 0})
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerMove, X: 90, Y: 50})
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 90, Y: 50, Button: 0, Clicks: 2})
	if len(opened) != 1 {
		t.Fatalf("drag opened %v", opened)
	}

	view.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 500, Y: 500, Button: 0})
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 500, Y: 500, Button: 0, Clicks: 2})
	if len(opened) != 1 {
		t.Fatalf("blank double click opened %v", opened)
	}

	view.demo = true
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Y: 50, Button: 0})
	view.handleInput(ui.InputEvent{Kind: ui.InputPointerUp, X: 50, Y: 50, Button: 0, Clicks: 2})
	if len(opened) != 1 {
		t.Fatalf("sample double click opened %v", opened)
	}
}

func TestTablePropertiesModalTabsRowsPreviewAndLocale(t *testing.T) {
	state := newState(t, "en-US")
	document := tablePropertiesDocument()
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(document))
	workspace.openTableProperties("orders")
	if !workspace.tableProperties.open {
		t.Fatal("table properties did not open")
	}
	workspace.openTableProperties("missing")
	if !workspace.tableProperties.open || workspace.tableProperties.tableID != "orders" {
		t.Fatal("unresolved table replaced the open modal")
	}

	tester := ui.NewTester(View(state, workspace), 800, 600)
	tester.Frame()
	for _, text := range []string{"General", "Columns", "Indexes", "Keys", "Preview", "Close", "Orders", "Order records", "Sales"} {
		if !tester.HasText(text) {
			t.Fatalf("missing modal text %q: %q", text, tester.Texts())
		}
	}
	if workspace.tableProperties.tab != 0 || !workspace.tableProperties.hasDetail ||
		workspace.tableProperties.detail.ID != "orders" {
		t.Fatalf("initial modal state = %+v", workspace.tableProperties)
	}

	if err := tester.Click("Columns"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if workspace.tableProperties.tab != 1 || len(workspace.tableProperties.detail.Columns) != 21 {
		t.Fatalf("columns state = %+v", workspace.tableProperties)
	}
	if !tester.HasText("Column 01") || !tester.HasText("COL_01") {
		t.Fatalf("first column missing: %q", tester.Texts())
	}
	listBounds, ok := modalListBounds(tester)
	if !ok {
		t.Fatal("columns list not found")
	}
	tester.Scroll(listBounds.X+listBounds.W/2, listBounds.Y+listBounds.H/2, 0, 2000)
	tester.Frame()
	if !tester.HasText("Column 21") {
		t.Fatalf("last column missing after scrolling: %q", tester.Texts())
	}

	if err := tester.Click("Indexes"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, text := range []string{"Index Orders", "Unique", "DESC", "ASC"} {
		if !tester.HasText(text) {
			t.Errorf("missing index text %q: %q", text, tester.Texts())
		}
	}
	if err := tester.Click("Keys"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, text := range []string{"PK Orders", "Primary", "Column 01"} {
		if !tester.HasText(text) {
			t.Errorf("missing key text %q: %q", text, tester.Texts())
		}
	}

	if err := tester.Click("Preview"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	table := workspace.detailIndex.tables["orders"]
	wantSQL := sqlgen.GenerateTable(*table).SQL
	if !workspace.tableProperties.previewLoaded || workspace.tableProperties.previewEditor == nil ||
		!workspace.tableProperties.previewEditor.ReadOnly ||
		workspace.tableProperties.previewEditor.Buffer.String() != wantSQL {
		t.Fatalf("preview state = %+v", workspace.tableProperties)
	}
	editor := workspace.tableProperties.previewEditor
	var copied string
	workspace.SetClipboardWriter(func(text string) error {
		copied = text
		return nil
	})
	if err := tester.Click("Copy SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if copied != wantSQL || !tester.HasText("SQL copied") {
		t.Fatalf("preview copy = %q, texts %q", copied, tester.Texts())
	}

	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	for _, text := range []string{"常规", "字段", "索引", "键", "预览", "关闭", "复制 SQL"} {
		if !tester.HasText(text) {
			t.Fatalf("missing Chinese table properties text %q: %q", text, tester.Texts())
		}
	}
	if workspace.tableProperties.previewEditor != editor ||
		workspace.tableProperties.previewEditor.Buffer.String() != wantSQL {
		t.Fatal("locale change regenerated the table SQL")
	}

	workspace.openTableProperties("other")
	tester.Frame()
	if workspace.tableProperties.tableID != "other" || workspace.tableProperties.tab != 0 ||
		workspace.tableProperties.previewLoaded || !workspace.tableProperties.hasDetail {
		t.Fatalf("new table retained old state: %+v", workspace.tableProperties)
	}
	if !tester.HasText("常规") {
		t.Fatalf("new table did not open General: %q", tester.Texts())
	}

	if err := tester.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if workspace.tableProperties.open {
		t.Fatal("close button left the modal open")
	}
	workspace.openTableProperties("orders")
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if workspace.tableProperties.open {
		t.Fatal("Escape left the modal open")
	}
}

func TestTablePropertiesFitsMinimumContentHeight(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(tablePropertiesDocument()))
	workspace.openTableProperties("orders")
	tester := ui.NewTester(View(state, workspace), 800, 572)
	tester.Frame()
	closeBounds, ok := tester.Find("Close")
	if !ok || closeBounds.X < 0 || closeBounds.Y < 0 ||
		closeBounds.X+closeBounds.W > 800 || closeBounds.Y+closeBounds.H > 572 {
		t.Fatalf("close button outside 800x572: %+v, %v", closeBounds, ok)
	}
	if err := tester.Click("Columns"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	listBounds, ok := modalListBounds(tester)
	if !ok || listBounds.X < 0 || listBounds.Y < 0 ||
		listBounds.X+listBounds.W > 800 || listBounds.Y+listBounds.H > 572 {
		t.Fatalf("columns list outside 800x572: %+v, %v", listBounds, ok)
	}
}

func TestTablePropertiesClearsForNewDocument(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(tablePropertiesDocument()))
	workspace.openTableProperties("orders")
	tester := ui.NewTester(View(state, workspace), 800, 600)
	tester.Frame()
	if err := tester.Click("Preview"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !workspace.tableProperties.previewLoaded {
		t.Fatal("preview was not generated")
	}

	token := workspace.BeginOpen()
	if workspace.tableProperties.open || workspace.tableProperties.previewLoaded ||
		workspace.tableProperties.tableID != "" {
		t.Fatalf("new open retained table properties: %+v", workspace.tableProperties)
	}
	workspace.PublishBoards(token, BuildBoardSet(tablePropertiesDocument()))
	if workspace.tableProperties.open || workspace.tableProperties.previewLoaded {
		t.Fatalf("new document retained table properties: %+v", workspace.tableProperties)
	}
	workspace.openTableProperties("orders")
	if !workspace.tableProperties.open || workspace.tableProperties.tab != 0 ||
		workspace.tableProperties.previewLoaded {
		t.Fatalf("same table ID reused old preview: %+v", workspace.tableProperties)
	}
}

func tablePropertiesDocument() *pdm.Document {
	yes, no := true, false
	text := func(value string) pdm.OptionalText {
		return pdm.OptionalText{Value: value, Present: true}
	}
	columns := make([]pdm.Column, 21)
	for index := range columns {
		mandatory := &yes
		if index%2 == 1 {
			mandatory = &no
		}
		columns[index] = pdm.Column{
			Metadata: pdm.Metadata{
				ID:      fmt.Sprintf("col-%02d", index+1),
				Name:    fmt.Sprintf("Column %02d", index+1),
				Code:    fmt.Sprintf("COL_%02d", index+1),
				Comment: text(fmt.Sprintf("Comment %02d", index+1)),
			},
			DataType:  text("DECIMAL"),
			Length:    text("12"),
			Precision: text("12"),
			Scale:     text("2"),
			Mandatory: mandatory,
		}
	}
	columns[2].Code = "COL_03_LONG_CODE_FOR_TOOLTIP"
	columns[2].Comment = text(strings.Repeat("Long column comment ", 12))
	orders := pdm.Table{
		Metadata: pdm.Metadata{ID: "orders", Name: "Orders", Code: "orders", Comment: text("Order records")},
		Columns:  columns,
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "pk-orders", Name: "PK Orders", Code: "pk_orders"},
			ColumnIDs: []string{"col-01"},
		}},
		PrimaryKeyID: "pk-orders",
		Indexes: []pdm.Index{{
			Metadata: pdm.Metadata{ID: "idx-orders", Name: "Index Orders", Code: "idx_orders"},
			Unique:   &yes,
			Columns: []pdm.IndexColumn{
				{ColumnID: "col-01", SortOrder: text("DESC")},
				{ColumnID: "col-02", SortOrder: text("ASC")},
			},
		}},
	}
	other := pdm.Table{
		Metadata: pdm.Metadata{ID: "other", Name: "Other", Code: "other"},
		Columns:  []pdm.Column{{Metadata: pdm.Metadata{ID: "other-id", Name: "Other ID"}, DataType: text("INT")}},
	}
	parent := pdm.Table{
		Metadata: pdm.Metadata{ID: "parent", Name: "Parent", Code: "parent"},
		Columns:  []pdm.Column{{Metadata: pdm.Metadata{ID: "parent-id", Name: "Parent ID"}, DataType: text("INT")}},
	}
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{parent},
			Packages: []pdm.Package{{
				Metadata: pdm.Metadata{ID: "package-sales", Name: "Sales"},
				Contents: pdm.Contents{Tables: []pdm.Table{orders, other}},
			}},
			References: []pdm.Reference{{
				Metadata:      pdm.Metadata{ID: "fk-orders-parent", Name: "Orders parent"},
				ChildTableID:  "orders",
				ParentTableID: "parent",
				Joins: []pdm.ReferenceJoin{{
					ParentColumnID: "parent-id",
					ChildColumnID:  "col-02",
				}},
			}},
		},
	}}
}

func modalListBounds(tester *ui.Tester) (ui.Rect, bool) {
	best := ui.Rect{}
	for _, info := range tester.AccessAll() {
		if info.Role == ui.RoleList && info.Bounds.W > best.W {
			best = info.Bounds
		}
	}
	return best, best.W > 0 && best.H > 0
}
