package appui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"pdmview/internal/graph"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestSearchIndexAllTermsCaseInsensitivePartialAndDeduped(t *testing.T) {
	if SearchKey != "pdmview.workspace.search" ||
		SearchResultKey("table") != "pdmview.workspace.search.result.table" {
		t.Fatal("stable automation search keys changed")
	}
	set := BuildBoardSet(searchDocument())
	if set.Search == nil {
		t.Fatal("search index missing from the board set")
	}
	tests := []struct {
		query string
		want  string
	}{
		{query: "CUSTOMERMASTER", want: "customer"},
		{query: "cust_mas", want: "customer"},
		{query: "party", want: "customer"},
		{query: "DISPLAYNAME", want: "customer"},
		{query: "display_n", want: "customer"},
		{query: "shown", want: "customer"},
		{query: "secondary", want: "customer"},
		{query: "orderhead", want: "order"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results := set.Search.search(context.Background(), tc.query)
			if len(results) != 1 || results[0].TableID != tc.want {
				t.Fatalf("search %q = %+v, want only %q", tc.query, results, tc.want)
			}
		})
	}
	if results := set.Search.search(context.Background(), "absent"); len(results) != 0 {
		t.Fatalf("missing query returned %+v", results)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if results := set.Search.search(ctx, "customer"); results != nil {
		t.Fatalf("canceled search returned %+v", results)
	}
}

func TestSearchCancellationInsideOneLargeTable(t *testing.T) {
	table := pdm.Table{Metadata: pdm.Metadata{ID: "large", Name: "Large"}}
	for offset := 0; offset < searchCancelCheckInterval*3; offset++ {
		table.Columns = append(table.Columns, pdm.Column{
			Metadata: pdm.Metadata{
				ID:   fmt.Sprintf("column-%d", offset),
				Name: fmt.Sprintf("Column %d", offset),
			},
		})
	}
	index := buildSearchIndex(&pdm.Model{Contents: pdm.Contents{Tables: []pdm.Table{table}}})
	ctx := &delayedCancelContext{done: make(chan struct{})}
	if results := index.search(ctx, "no-match"); results != nil {
		t.Fatalf("large-table canceled search returned %+v", results)
	}
	if ctx.calls < 3 {
		t.Fatalf("search checked cancellation %d times, want checks inside terms", ctx.calls)
	}
}

func TestSearchWorkspaceStaleQueryDocumentAndNoResults(t *testing.T) {
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(searchDocument()))

	first, ok := workspace.BeginSearch("customer")
	if !ok {
		t.Fatal("first search did not start")
	}
	second, ok := workspace.BeginSearch("order")
	if !ok {
		t.Fatal("second search did not start")
	}
	if workspace.PublishSearchResults(first, []SearchResult{{TableID: "customer", Name: "CustomerMaster"}}) {
		t.Fatal("stale query result was published")
	}
	if !workspace.PublishSearchResults(second, second.Run()) {
		t.Fatal("current query result was rejected")
	}
	if len(workspace.searchResults) != 1 || workspace.searchResults[0].TableID != "order" {
		t.Fatalf("current results = %+v", workspace.searchResults)
	}

	staleDocument, ok := workspace.BeginSearch("customer")
	if !ok {
		t.Fatal("document-stale search did not start")
	}
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(searchDocument()))
	if workspace.PublishSearchResults(staleDocument, []SearchResult{{TableID: "customer", Name: "CustomerMaster"}}) {
		t.Fatal("stale document result was published")
	}

	empty, ok := workspace.BeginSearch("no-match")
	if !ok {
		t.Fatal("empty-result search did not start")
	}
	if !workspace.PublishSearchResults(empty, empty.Run()) {
		t.Fatal("empty-result search was rejected")
	}
	if !workspace.searchFinished || len(workspace.searchResults) != 0 {
		t.Fatalf("no-result state = finished:%v results:%+v", workspace.searchFinished, workspace.searchResults)
	}
}

func TestSearchUIResultLocaleAndNoResultFeedback(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(searchDocument()))
	tester := ui.NewTester(View(state, workspace), 1000, 700)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})

	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tester.Type("party")
	tester.Frame()
	if workspace.searchQuery != "party" ||
		len(workspace.searchResults) != 1 ||
		workspace.searchResults[0].TableID != "customer" {
		t.Fatalf("typed search = query:%q results:%+v", workspace.searchQuery, workspace.searchResults)
	}
	if !tester.HasText("CustomerMaster") {
		t.Fatalf("search result missing: %q", tester.Texts())
	}

	if err := tester.Click("CustomerMaster"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if workspace.selectedTableID != "customer" || workspace.ActiveBoard() != 0 {
		t.Fatalf("search selection = table:%q board:%d", workspace.selectedTableID, workspace.ActiveBoard())
	}
	offset, ok := workspace.canvas.tableNodeOffset("customer")
	if !ok {
		t.Fatal("selected search table has no canvas node")
	}
	node := workspace.canvas.scene.Nodes[offset]
	center := graph.Point{
		X: (node.Bounds.Min.X + node.Bounds.Max.X) / 2,
		Y: (node.Bounds.Min.Y + node.Bounds.Max.Y) / 2,
	}
	screen := workspace.canvas.camera.WorldToViewport(center)
	if distance(screen, graph.Point{
		X: workspace.canvas.viewport.Width() / 2,
		Y: workspace.canvas.viewport.Height() / 2,
	}) > 0.01 {
		t.Fatalf("reduced-motion search did not center the table: %+v", screen)
	}

	scale, origin := workspace.canvas.camera.Scale(), workspace.canvas.camera.VisibleWorld().Min
	selection := workspace.canvas.selection
	query, results := workspace.searchQuery, append([]SearchResult(nil), workspace.searchResults...)
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if workspace.searchQuery != query ||
		len(workspace.searchResults) != len(results) ||
		workspace.searchResults[0] != results[0] ||
		workspace.canvas.camera.Scale() != scale ||
		workspace.canvas.camera.VisibleWorld().Min != origin ||
		workspace.canvas.selection != selection {
		t.Fatal("locale change lost search state, selection, or viewport")
	}
	if !tester.HasText("搜索") || !tester.HasText("CustomerMaster") {
		t.Fatalf("localized search UI missing: %q", tester.Texts())
	}

	empty, ok := workspace.BeginSearch("no-match")
	if !ok || !workspace.PublishSearchResults(empty, empty.Run()) {
		t.Fatal("empty search did not complete")
	}
	tester.Frame()
	if !tester.HasText("无结果") {
		t.Fatalf("no-result feedback missing: %q", tester.Texts())
	}
}

func TestSearchResultsVirtualized(t *testing.T) {
	model := pdm.Model{Metadata: pdm.Metadata{ID: "model", Name: "Large Search"}}
	for offset := 0; offset < 1000; offset++ {
		model.Contents.Tables = append(model.Contents.Tables, pdm.Table{
			Metadata: pdm.Metadata{
				ID:   fmt.Sprintf("result-%04d", offset),
				Name: fmt.Sprintf("Result %04d", offset),
			},
		})
	}
	model.Contents.Diagrams = []pdm.Diagram{{
		Metadata: pdm.Metadata{ID: "board", Name: "Board"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{{
			Metadata: pdm.Metadata{ID: "first-symbol"},
			TableID:  "result-0000",
			Rect:     boardRect(0, 0, 100, 100),
		}},
	}}
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(&pdm.Document{Model: model}))
	request, ok := workspace.BeginSearch("result")
	if !ok || !workspace.PublishSearchResults(request, request.Run()) {
		t.Fatal("large search did not complete")
	}
	if len(workspace.searchResults) != 1000 {
		t.Fatalf("large search results = %d", len(workspace.searchResults))
	}
	tester := ui.NewTester(View(newState(t, "en-US"), workspace), 1000, 700)
	visible := 0
	for _, text := range tester.Texts() {
		if strings.HasPrefix(text, "Result ") {
			visible++
		}
	}
	if visible == 0 || visible >= len(workspace.searchResults) {
		t.Fatalf("virtual list rendered %d of %d result labels", visible, len(workspace.searchResults))
	}
	if visible > 64 {
		t.Fatalf("virtual list rendered too many result labels: %d", visible)
	}
}

func TestSearchLocateCrossBoardAndInterruptibleMotion(t *testing.T) {
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(searchDocument()))
	tester := ui.NewTester(View(newState(t, "en-US"), workspace), 1000, 700)

	if !workspace.LocateSearchResult("customer") || workspace.canvas.cameraTween == nil {
		t.Fatal("current-board search locate did not start camera motion")
	}
	tester.Frame()
	if !workspace.LocateSearchResult("order") || workspace.ActiveBoard() != 1 ||
		workspace.canvas.cameraTween == nil {
		t.Fatalf("cross-board search locate = active:%d tween:%v",
			workspace.ActiveBoard(), workspace.canvas.cameraTween)
	}
	if !workspace.LocateSearchResult("customer") || workspace.ActiveBoard() != 0 ||
		workspace.canvas.cameraTween == nil {
		t.Fatalf("interrupting search locate = active:%d tween:%v",
			workspace.ActiveBoard(), workspace.canvas.cameraTween)
	}
	tester.Frame()
	if offset, ok := workspace.canvas.tableNodeOffset("customer"); !ok ||
		workspace.canvas.selection != (graph.Hit{Kind: graph.HitNode, Index: offset}) {
		t.Fatalf("interrupted motion settled on %+v", workspace.canvas.selection)
	}
	if !workspace.LocateSearchResult("order") || workspace.canvas.cameraTween == nil {
		t.Fatal("camera tween did not restart")
	}
	workspace.canvas.clearSelection()
	if workspace.canvas.cameraTween != nil {
		t.Fatal("clearing selection did not cancel camera motion")
	}

	nodesBefore := len(workspace.canvas.scene.Nodes)
	if workspace.LocateSearchResult("unplaced") ||
		!workspace.unplacedTable ||
		workspace.selectedTableID != "unplaced" ||
		workspace.canvas.cameraTween != nil ||
		len(workspace.canvas.scene.Nodes) != nodesBefore {
		t.Fatalf("unplaced search locate = table:%q unplaced:%v tween:%v",
			workspace.selectedTableID, workspace.unplacedTable, workspace.canvas.cameraTween)
	}
}

func searchDocument() *pdm.Document {
	customer := pdm.Table{
		Metadata: pdm.Metadata{
			ID:      "customer",
			Name:    "CustomerMaster",
			Code:    "CUST_MASTER",
			Comment: pdm.OptionalText{Value: "Party records", Present: true},
		},
		Columns: []pdm.Column{{
			Metadata: pdm.Metadata{
				ID:      "customer-display-name",
				Name:    "DisplayName",
				Code:    "DISPLAY_NM",
				Comment: pdm.OptionalText{Value: "Shown label", Present: true},
			},
		}},
	}
	order := pdm.Table{
		Metadata: pdm.Metadata{
			ID:   "order",
			Name: "OrderHeader",
			Code: "ORDER_HEADER",
		},
	}
	unplaced := pdm.Table{
		Metadata: pdm.Metadata{ID: "unplaced", Name: "Unplaced Table"},
	}
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model", Name: "Search Model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{customer, order},
			Packages: []pdm.Package{{
				Metadata: pdm.Metadata{ID: "package", Name: "Package"},
				Contents: pdm.Contents{Tables: []pdm.Table{{
					Metadata: pdm.Metadata{
						ID:      "customer",
						Name:    "CustomerMaster",
						Code:    "CUST_MASTER",
						Comment: pdm.OptionalText{Value: "Secondary match", Present: true},
					},
				}, unplaced}},
			}},
			Diagrams: []pdm.Diagram{
				{
					Metadata: pdm.Metadata{ID: "customer-board", Name: "Customer Board"},
					OwnerID:  "model",
					TableSymbols: []pdm.TableSymbol{{
						Metadata: pdm.Metadata{ID: "customer-symbol"},
						TableID:  "customer",
						Rect:     boardRect(0, 0, 100, 100),
					}},
				},
				{
					Metadata: pdm.Metadata{ID: "order-board", Name: "Order Board"},
					OwnerID:  "model",
					TableSymbols: []pdm.TableSymbol{{
						Metadata: pdm.Metadata{ID: "order-symbol"},
						TableID:  "order",
						Rect:     boardRect(200, 0, 300, 100),
					}},
				},
			},
			DefaultDiagramID: "customer-board",
		},
	}}
}

func distance(a, b graph.Point) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

type delayedCancelContext struct {
	done  chan struct{}
	calls int
}

func (ctx *delayedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (ctx *delayedCancelContext) Done() <-chan struct{} {
	ctx.calls++
	if ctx.calls == 3 {
		close(ctx.done)
	}
	return ctx.done
}

func (ctx *delayedCancelContext) Err() error {
	select {
	case <-ctx.done:
		return context.Canceled
	default:
		return nil
	}
}

func (ctx *delayedCancelContext) Value(any) any { return nil }
