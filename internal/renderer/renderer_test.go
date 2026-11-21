package renderer

import (
	"testing"

	"pdmview/internal/graph"

	"github.com/egoist/mygo/ui"
)

func TestRendererVisibleEdgesUseExactViewportCrossings(t *testing.T) {
	scene := graph.Scene{
		Edges: []graph.Edge{
			{ID: 1, Points: []graph.Point{{X: -50, Y: 50}, {X: 250, Y: 50}}},
			{ID: 2, Points: []graph.Point{{X: -10, Y: 0}, {X: -10, Y: 150}, {X: 210, Y: 150}, {X: 210, Y: 0}}},
			{ID: 3, Points: []graph.Point{{X: 100, Y: 50}, {X: 100, Y: 50}}},
		},
		Bounds: graph.Rect{Min: graph.Point{X: -50, Y: 0}, Max: graph.Point{X: 250, Y: 150}},
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 200, Y: 100}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	stats := FrameStats{}
	_ = host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 200, 100, &stats)
	if stats.VisibleEdges != 2 {
		t.Fatalf("visible edges = %d, want crossing edge and zero-length edge only", stats.VisibleEdges)
	}
}

func TestRendererFieldRowsFollowLODHysteresis(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 400, Y: 300}})
	setScale(camera, 0.30)
	renderer := New(scene, graph.NewSpatialIndex(scene))
	selected := graph.Hit{Kind: graph.HitNone, Index: -1}
	stats := FrameStats{}
	tt := host(renderer, camera, selected, 400, 300, &stats)

	check := func(scale float64, want LOD, rowsPerNode int) {
		t.Helper()
		setScale(camera, scale)
		tt.Frame()
		if stats.LOD != want {
			t.Fatalf("scale %v: LOD = %q, want %q", scale, stats.LOD, want)
		}
		if stats.FieldRows != stats.VisibleNodes*rowsPerNode {
			t.Fatalf("scale %v: field rows = %d for %d nodes, want %d per node",
				scale, stats.FieldRows, stats.VisibleNodes, rowsPerNode)
		}
	}

	check(0.30, Far, 0)
	check(0.40, Far, 0)
	check(0.45, Medium, 2)
	check(0.80, Medium, 2)
	check(0.90, Near, 5)
	check(0.80, Near, 5)
	check(0.74, Medium, 2)
	check(0.40, Medium, 2)
	check(0.35, Far, 0)
	check(0.90, Near, 5)
	check(0.35, Far, 0)
}

func TestRendererSkipsDetailForNodesJustOutsideViewport(t *testing.T) {
	scene := graph.Scene{Nodes: []graph.Node{
		{Bounds: graph.Rect{Min: graph.Point{X: 10, Y: 10}, Max: graph.Point{X: 195, Y: 95}}, Fields: []graph.Field{{Name: "visible"}}},
		{Bounds: graph.Rect{Min: graph.Point{X: 200.5, Y: 10}, Max: graph.Point{X: 395, Y: 95}}, Fields: []graph.Field{{Name: "outside"}}},
	}}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 200, Y: 100}})
	canvas := New(scene, graph.NewSpatialIndex(scene))
	var stats FrameStats
	host(canvas, camera, graph.Hit{}, 200, 100, &stats)
	if stats.LOD != Near || stats.VisibleNodes != 1 || stats.FieldRows != 1 {
		t.Fatalf("detailed work should be limited to the visible node: %+v", stats)
	}
}

func TestRendererGridTileCacheReuseThemeAndDPIInvalidation(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 300, Y: 200}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	selected := graph.Hit{Kind: graph.HitNone, Index: -1}
	stats := FrameStats{}
	var history []FrameStats
	tt := hostRecording(renderer, camera, selected, 300, 200, &stats, &history)
	if len(history) == 0 || history[0].CacheMisses != 1 || history[0].CacheHits != 0 {
		t.Fatalf("first draw history = %+v", history)
	}
	for _, frame := range history[1:] {
		if frame.CacheMisses != 0 || frame.CacheHits != 1 {
			t.Fatalf("unchanged replay cache = %+v", frame)
		}
	}

	history = history[:0]
	tt.Frame()
	if got := history[len(history)-1]; got.CacheMisses != 0 || got.CacheHits != 1 {
		t.Fatalf("second draw cache = %+v", got)
	}
	history = history[:0]
	tt.SetScale(2)
	if got := history[len(history)-1]; got.CacheMisses != 1 || got.CacheHits != 0 {
		t.Fatalf("DPI change cache = %+v", got)
	}
	history = history[:0]
	tt.SetDark(true)
	if got := history[len(history)-1]; got.CacheMisses != 1 || got.CacheHits != 0 {
		t.Fatalf("theme change cache = %+v", got)
	}
}

func TestRendererGridTilesAreScreenSpaceAndBoundedAtFarZoom(t *testing.T) {
	scene, err := graph.GenerateDefault(500, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 800, Y: 600}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	selected := graph.Hit{Kind: graph.HitNone, Index: -1}
	stats := FrameStats{}
	tt := host(renderer, camera, selected, 800, 600, &stats)
	setScale(camera, 1)
	tt.Frame()
	nearTiles := renderer.lastGridTiles
	setScale(camera, 0.02)
	tt.Frame()
	farTiles := renderer.lastGridTiles
	if nearTiles != farTiles {
		t.Fatalf("grid density changed with camera zoom: 1x=%d .02x=%d", nearTiles, farTiles)
	}
	if farTiles < 1 || farTiles > 200 {
		t.Fatalf("far-zoom grid tiles = %d outside bounded screen-space count", farTiles)
	}
	if stats.VisibleNodes == 0 {
		t.Fatal("far zoom unexpectedly culled the whole scene")
	}
}

func TestRendererRetainsTextLayoutsAndSelection(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 400, Y: 300}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	title := renderer.nodes[0].name.full
	field := renderer.nodes[0].fieldLabels[0].full
	stats := FrameStats{}
	selected := graph.Hit{Kind: graph.HitNode, Index: 0}
	tt := host(renderer, camera, selected, 400, 300, &stats)
	tt.Frame()
	if renderer.nodes[0].name.full != title || renderer.nodes[0].fieldLabels[0].full != field {
		t.Fatal("Draw reshaped retained text layouts")
	}
	if stats.VisibleNodes == 0 || stats.FieldRows == 0 {
		t.Fatalf("selected near scene stats = %+v", stats)
	}
}

func TestCanvasLabelFitsCardBeforePainting(t *testing.T) {
	font := ui.Font{Size: 11.5}
	ellipsis := ui.ShapeText("…", font, 0)
	ellipsisWidth, _ := ellipsis.Size()
	const name = "客户订单明细与履约记录 / Customer Order Fulfillment Detail 001"
	const cardWidth = 160
	layout := shapeLabel(name, font, cardWidth, ellipsis)
	width, _ := layout.Size()
	if width <= ellipsisWidth || width > cardWidth {
		t.Fatalf("fitted label width = %v, want between ellipsis and card width", width)
	}
	if got := shapeLabel(name, font, ellipsisWidth/2, ellipsis); got != nil {
		t.Fatal("a card narrower than the ellipsis should omit its label")
	}
}

func TestRendererShowsCodeTypesAndKeyMarkers(t *testing.T) {
	scene := graph.Scene{Nodes: []graph.Node{{
		ID:     0,
		Bounds: graph.Rect{Min: graph.Point{X: 0, Y: 0}, Max: graph.Point{X: 260, Y: 180}},
		Name:   "Orders",
		Code:   "ORDERS",
		Fields: []graph.Field{
			{Name: "id", DataType: "INTEGER", PrimaryKey: true},
			{Name: "customer_id", DataType: "BIGINT", ForeignKey: true},
			{Name: "amount", DataType: "DECIMAL(10, 2)"},
		},
	}}}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 320, Y: 220}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	var stats FrameStats
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 320, 220, &stats)
	visual := renderer.nodes[0]
	name, code := nodeDisplayText(scene.Nodes[0])
	if name != "Orders" || code != "ORDERS" ||
		visual.name.full == nil || visual.code.full == nil {
		t.Fatalf("retained node labels = %+v, name=%q code=%q", visual, name, code)
	}
	wantFields := []string{"PK id  INTEGER", "FK customer_id  BIGINT", "amount  DECIMAL(10, 2)"}
	for offset, want := range wantFields {
		if got := fieldDisplayText(scene.Nodes[0].Fields[offset]); got != want {
			t.Fatalf("field %d text = %q, want %q", offset, got, want)
		}
		if visual.fieldLabels[offset].full == nil {
			t.Fatalf("field %d was not pre-shaped", offset)
		}
	}
	if stats.FieldRows != 3 {
		t.Fatalf("actual field rows = %d, want 3", stats.FieldRows)
	}
}

func TestNodeDisplayTextIsSingleLineAndDeduplicated(t *testing.T) {
	node := graph.Node{
		Name: "Order\nHeader",
		Code: "Order Header",
	}
	name, code := nodeDisplayText(node)
	if name != "Order Header" || code != "" {
		t.Fatalf("displayed node text = %q / %q", name, code)
	}
	if node.Name != "Order\nHeader" || node.Code != "Order Header" {
		t.Fatal("node source text was mutated")
	}

	field := graph.Field{
		Name:       "account\nid",
		DataType:   "BIGINT\r\nUNSIGNED",
		PrimaryKey: true,
		ForeignKey: true,
	}
	if got := fieldDisplayText(field); got != "PK/FK account id  BIGINT UNSIGNED" {
		t.Fatalf("field display text = %q", got)
	}
	if field.Name != "account\nid" || field.DataType != "BIGINT\r\nUNSIGNED" {
		t.Fatal("field source text was mutated")
	}
}

func TestRendererClipsTextByRetainedWidthAndHeight(t *testing.T) {
	scene := graph.Scene{Nodes: []graph.Node{
		{
			ID:     0,
			Bounds: graph.Rect{Min: graph.Point{X: 0, Y: 0}, Max: graph.Point{X: 18, Y: 100}},
			Name:   "Too Wide",
			Fields: []graph.Field{{Name: "field", DataType: "INTEGER"}},
		},
		{
			ID:     1,
			Bounds: graph.Rect{Min: graph.Point{X: 40, Y: 0}, Max: graph.Point{X: 240, Y: 8}},
			Name:   "Too Short",
			Code:   "SHORT",
			Fields: []graph.Field{{Name: "hidden"}},
		},
	}}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 260, Y: 120}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	var stats FrameStats
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 260, 120, &stats)
	if renderer.nodes[0].name.pick(2, 93) != nil ||
		renderer.nodes[0].fieldLabels[0].pick(2, fieldRowHeight) != nil {
		t.Fatal("too-wide text selected a layout")
	}
	if renderer.nodes[1].name.pick(184, 1) != nil ||
		renderer.nodes[1].code.pick(184, 1) != nil {
		t.Fatal("too-short text selected a layout")
	}
	if stats.FieldRows != 0 {
		t.Fatalf("FieldRows counted undrawn rows: %+v", stats)
	}
}

func TestNodeCardHoverAndSelectionStyles(t *testing.T) {
	theme := ui.LightTheme()
	baseFill, baseBorder, baseWidth := nodeCardStyle(theme, false, false)
	hoverFill, hoverBorder, hoverWidth := nodeCardStyle(theme, false, true)
	selectedFill, selectedBorder, selectedWidth := nodeCardStyle(theme, true, true)
	if baseFill == hoverFill && baseBorder == hoverBorder && baseWidth == hoverWidth {
		t.Fatal("hover style has no visible difference")
	}
	if selectedBorder != theme.Accent || selectedWidth != selectedLineWidth {
		t.Fatalf("selected style = %+v %v", selectedBorder, selectedWidth)
	}
	if selectedFill == hoverFill && selectedBorder == hoverBorder && selectedWidth == hoverWidth {
		t.Fatal("selection does not take priority over hover")
	}
}

func TestDrawWithHoverMatchesStatsAndRetainsLayouts(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 400, Y: 300}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	name := renderer.nodes[0].name.full
	field := renderer.nodes[0].fieldLabels[0].full
	var plain, hover FrameStats
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 400, 300, &plain)
	hostWithHover(
		renderer,
		camera,
		graph.Hit{Kind: graph.HitNode, Index: 0},
		graph.Hit{Kind: graph.HitNode, Index: 1},
		400,
		300,
		&hover,
	)
	if plain.VisibleNodes != hover.VisibleNodes || plain.VisibleEdges != hover.VisibleEdges ||
		plain.FieldRows != hover.FieldRows || plain.LOD != hover.LOD {
		t.Fatalf("DrawWithHover stats = %+v, plain = %+v", hover, plain)
	}
	if renderer.nodes[0].name.full != name || renderer.nodes[0].fieldLabels[0].full != field {
		t.Fatal("DrawWithHover reshaped retained layouts")
	}
}

func host(canvas *Canvas, camera *graph.Camera, selected graph.Hit, width, height int, stats *FrameStats) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			*stats = canvas.Draw(p, r, camera, selected)
		})
	}, width, height)
}

func hostRecording(canvas *Canvas, camera *graph.Camera, selected graph.Hit, width, height int, stats *FrameStats, history *[]FrameStats) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			frame := canvas.Draw(p, r, camera, selected)
			*stats = frame
			*history = append(*history, frame)
		})
	}, width, height)
}

func hostWithHover(
	canvas *Canvas,
	camera *graph.Camera,
	selected, hovered graph.Hit,
	width, height int,
	stats *FrameStats,
) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			*stats = canvas.DrawWithHover(p, r, camera, selected, hovered)
		})
	}, width, height)
}

func setScale(camera *graph.Camera, scale float64) {
	camera.ZoomAt(graph.Point{}, scale/camera.Scale())
}
