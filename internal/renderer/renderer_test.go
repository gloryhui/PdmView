package renderer

import (
	"math"
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

func TestEdgeScreenProjectionUsesWorldToScreenAndFloat64Intermediate(t *testing.T) {
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{X: 100, Y: 50}, Max: graph.Point{X: 900, Y: 650}})
	camera.ZoomAt(graph.Point{}, 2)
	points := edgeScreenPoints(nil, camera, []graph.Point{{X: 10, Y: 20}, {X: 15, Y: 25}})
	if len(points) != 2 || points[0] != [2]float32{120, 90} ||
		points[1] != [2]float32{130, 100} {
		t.Fatalf("screen projection = %+v", points)
	}

	largeCamera := graph.NewDiagramCamera(graph.Rect{Min: graph.Point{X: 100, Y: 50}, Max: graph.Point{X: 900, Y: 650}})
	largeBounds := graph.Rect{
		Min: graph.Point{X: 9_000_000_000_000, Y: -9_000_000_000_000},
		Max: graph.Point{X: 9_000_000_001_000, Y: -8_999_999_999_000},
	}
	largeCamera.FitBounds(largeBounds, 40)
	largePoints := edgeScreenPoints(points[:0], largeCamera, []graph.Point{largeBounds.Min, largeBounds.Max})
	if len(largePoints) != 2 {
		t.Fatalf("large projection length = %d", len(largePoints))
	}
	for _, point := range largePoints {
		if math.IsNaN(float64(point[0])) || math.IsInf(float64(point[0]), 0) ||
			math.IsNaN(float64(point[1])) || math.IsInf(float64(point[1]), 0) {
			t.Fatalf("large projection is not finite: %+v", point)
		}
	}
	if &largePoints[0] != &points[0] {
		t.Fatal("edge screen projection did not reuse scratch")
	}
}

func TestEdgeArrowRequiresUsableTerminalTangentAndClampsLength(t *testing.T) {
	arrow := edgeArrow(nil, [][2]float32{{0, 0}, {10, 0}})
	if len(arrow) != 3 || arrow[1] != [2]float32{10, 0} {
		t.Fatalf("terminal arrow = %+v", arrow)
	}
	if !near32(arrow[0][0], 3) || !near32(arrow[0][1], 3.5) ||
		!near32(arrow[2][0], 3) || !near32(arrow[2][1], -3.5) {
		t.Fatalf("arrow V = %+v", arrow)
	}
	if got := edgeArrow(nil, [][2]float32{{0, 0}, {5, 0}}); len(got) != 0 {
		t.Fatalf("too-short terminal segment made arrow: %+v", got)
	}
	if got := edgeArrow(nil, [][2]float32{{0, 0}, {10, 0}, {10, 4}}); len(got) != 0 {
		t.Fatalf("short final bend reused earlier tangent: %+v", got)
	}
	if got := edgeArrow(nil, [][2]float32{{0, 0}, {10, 0}, {10, 0}}); len(got) != 3 {
		t.Fatalf("duplicate endpoint skipped valid tangent: %+v", got)
	}
	clamped := edgeArrow(nil, [][2]float32{{0, 0}, {6, 0}})
	if len(clamped) != 3 || clamped[0][0] < 0 || clamped[0][0] >= 6 {
		t.Fatalf("short segment arrow was not clamped: %+v", clamped)
	}
}

func TestEdgeHighlightPolicyAndBoundedWidths(t *testing.T) {
	edge := graph.Edge{From: 0, To: 1}
	if !edgeHighlighted(edge, 2, graph.Hit{Kind: graph.HitEdge, Index: 2}) ||
		edgeHighlighted(edge, 2, graph.Hit{Kind: graph.HitEdge, Index: 1}) {
		t.Fatal("selected edge highlighting is not offset-specific")
	}
	if !edgeHighlighted(edge, 2, graph.Hit{Kind: graph.HitNode, Index: 0}) ||
		!edgeHighlighted(edge, 2, graph.Hit{Kind: graph.HitNode, Index: 1}) ||
		edgeHighlighted(edge, 2, graph.Hit{Kind: graph.HitNode, Index: 3}) {
		t.Fatal("adjacent table highlighting is incorrect")
	}
	for _, lod := range []LOD{Far, Medium, Near} {
		width := edgeLineWidth(lod, false)
		if width <= 0 || !finite32(width) || width > edgeStrokeWidth {
			t.Fatalf("LOD %q width = %v", lod, width)
		}
		if edgeLineWidth(lod, true) != selectedLineWidth {
			t.Fatalf("highlighted LOD %q width = %v", lod, edgeLineWidth(lod, true))
		}
	}
	wantArrowEnvelope := math.Hypot(arrowLengthDIP, arrowHalfWidthDIP) + arrowLineWidthDIP/2
	if got := edgeCullPaddingDIP(Near); got < wantArrowEnvelope ||
		got < float64(selectedLineWidth)/2 {
		t.Fatalf("arrow cull padding = %v, want at least %v", got, wantArrowEnvelope)
	}
}

func TestRendererArrowLODAndEndpointCulling(t *testing.T) {
	scene := graph.Scene{Edges: []graph.Edge{{
		ID:       0,
		Directed: true,
		Points:   []graph.Point{{X: 50, Y: -2}, {X: 150, Y: -2}},
	}}}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 200, Y: 100}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	var stats FrameStats
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 200, 100, &stats)
	if stats.VisibleEdges != 1 || stats.Arrows != 1 {
		t.Fatalf("arrow-envelope culling stats = %+v", stats)
	}

	setScale(camera, 0.30)
	stats = FrameStats{}
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 200, 100, &stats)
	if stats.LOD != Far || stats.Arrows != 0 || stats.VisibleEdges != 1 {
		t.Fatalf("far LOD arrow stats = %+v", stats)
	}

	undirected := scene
	undirected.Edges = append([]graph.Edge(nil), scene.Edges...)
	undirected.Edges[0].Directed = false
	nearRenderer := New(undirected, graph.NewSpatialIndex(undirected))
	nearCamera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 200, Y: 100}})
	stats = FrameStats{}
	host(nearRenderer, nearCamera, graph.Hit{Kind: graph.HitNone, Index: -1}, 200, 100, &stats)
	if stats.Arrows != 0 {
		t.Fatalf("undirected edge drew arrow: %+v", stats)
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

func near32(a, b float32) bool {
	return math.Abs(float64(a-b)) <= 1e-5
}

func finite32(v float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}
