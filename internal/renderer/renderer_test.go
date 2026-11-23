package renderer

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"pdmview/internal/graph"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestSavedTableStyleAndMoreThanFiveFieldsAtMediumZoom(t *testing.T) {
	savedFill := &pdm.RGBColor{R: 255, G: 255, B: 127}
	savedBorder := &pdm.RGBColor{R: 64, G: 96, B: 192}
	presentation := &graph.TablePresentation{
		Style: pdm.TableAppearance{Fill: savedFill, Border: savedBorder},
		Title: "Saved Table",
	}
	for field := range 12 {
		presentation.Columns = append(presentation.Columns, graph.ColumnPresentation{
			Name: fmt.Sprintf("Field %d", field+1), DataType: "varchar(100)",
		})
	}
	scene := graph.Scene{Nodes: []graph.Node{{
		Bounds:       graph.Rect{Min: graph.Point{X: 10, Y: 10}, Max: graph.Point{X: 246, Y: 310}},
		Presentation: presentation,
	}}}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 300, Y: 300}})
	setScale(camera, 0.65)
	renderer := New(scene, graph.NewSpatialIndex(scene))
	stats := FrameStats{}
	tester := host(renderer, camera, graph.Hit{Kind: graph.HitNode, Index: 0}, 300, 300, &stats)
	if stats.LOD != Medium || stats.FieldRows != 12 {
		t.Fatalf("style preview ignored saved rows at fit zoom: %+v", stats)
	}
	image := tester.Image()
	if fill := image.RGBAAt(50, 190); fill != (color.RGBA{R: 255, G: 255, B: 127, A: 255}) {
		t.Fatalf("saved fill was replaced in selection: %+v", fill)
	}
}

func TestPresentationDrawsAllRowsAndFontScalesWithZoom(t *testing.T) {
	columns := make([]graph.ColumnPresentation, 21)
	for index := range columns {
		columns[index] = graph.ColumnPresentation{
			Name:     fmt.Sprintf("Column %02d", index+1),
			DataType: "INTEGER",
		}
	}
	presentation := &graph.TablePresentation{Title: "All Rows", Columns: columns}
	scene := graph.Scene{Nodes: []graph.Node{{
		Bounds:       graph.Rect{Min: graph.Point{X: 10, Y: 10}, Max: graph.Point{X: 310, Y: 510}},
		Presentation: presentation,
	}}}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 400, Y: 600}})
	setScale(camera, 0.65)
	renderer := New(scene, graph.NewSpatialIndex(scene))
	stats := FrameStats{}
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 400, 600, &stats)
	if stats.LOD != Medium || stats.FieldRows != 21 {
		t.Fatalf("all-row presentation = %+v, want Medium with 21 rows", stats)
	}
	visual := renderer.nodeVisual(0)
	if visual == nil || visual.table == nil || len(visual.table.rows) != 21 ||
		visual.table.rows[0].name == nil || visual.table.rows[20].name == nil {
		t.Fatalf("presentation rows = %+v", visual)
	}

	zoom1 := tableZoom(camera, visual.table)
	setScale(camera, 1.2)
	zoom2 := tableZoom(camera, visual.table)
	setScale(camera, 2.4)
	zoom3 := tableZoom(camera, visual.table)
	if zoom1 <= 0 || zoom1 >= zoom2 || zoom2 >= zoom3 {
		t.Fatalf("table zoom not monotonic: %v, %v, %v", zoom1, zoom2, zoom3)
	}
}

func TestPDMTableTextSurvivesThreeZoomOutNotchesBeyondEagerPrefill(t *testing.T) {
	const (
		targetOffset = 64
		referenceW   = 236
		initialDIP   = 85.0
	)
	nodes := make([]graph.Node, targetOffset+nodeVisualLimit+1)
	for offset := range nodes {
		x := 10000.0 + float64(offset*240)
		nodes[offset] = graph.Node{
			ID:     offset,
			Bounds: graph.Rect{Min: graph.Point{X: x}, Max: graph.Point{X: x + referenceW, Y: 144}},
		}
	}
	nodes[targetOffset] = graph.Node{
		ID:     targetOffset,
		Bounds: graph.Rect{Min: graph.Point{X: 10, Y: 10}, Max: graph.Point{X: 246, Y: 154}},
		Presentation: &graph.TablePresentation{
			Style: pdm.TableAppearance{
				Fill:        &pdm.RGBColor{R: 255, G: 255, B: 255},
				Border:      &pdm.RGBColor{},
				TitleColor:  &pdm.RGBColor{},
				ColumnColor: &pdm.RGBColor{},
			},
			Title: "Wide PDM Table",
			Columns: []graph.ColumnPresentation{
				{Name: "user_id", DataType: "BIGINT"},
				{Name: "display", DataType: "VARCHAR"},
			},
		},
	}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 200, Y: 100}})
	setScale(camera, initialDIP/referenceW)
	scene := graph.Scene{Nodes: nodes}
	renderer := New(scene, graph.NewSpatialIndex(scene))
	if renderer.nodeVisual(targetOffset) != nil {
		t.Fatal("target unexpectedly received eager prefill")
	}
	if prepared := renderer.PrepareVisible(camera, graph.Hit{}); prepared != 1 {
		t.Fatalf("baseline preparation = %d, want 1", prepared)
	}
	stats := FrameStats{}
	tt := host(renderer, camera, graph.Hit{}, 200, 100, &stats)
	tt.SetScale(2)
	if stats.LOD != Medium || stats.FieldRows != 2 || stats.UnpreparedNodes != 0 {
		t.Fatalf("baseline PDM text = %+v", stats)
	}

	for offset := targetOffset + 1; offset < len(scene.Nodes) && renderer.nodeVisual(targetOffset) != nil; offset++ {
		renderer.PrepareNode(offset)
	}
	if renderer.nodeVisual(targetOffset) != nil {
		t.Fatal("target was not evicted before the far-zoom preparation check")
	}

	step := math.Exp(-0.36)
	for notch := 1; notch <= 3; notch++ {
		setScale(camera, camera.Scale()*step)
		prepared := renderer.PrepareVisible(camera, graph.Hit{})
		tt.Frame()
		if notch == 1 && prepared == 0 {
			t.Fatal("far LOD skipped preparation for eligible PDM table text")
		}
		if stats.LOD != Far || stats.VisibleNodes != 1 || stats.FieldRows != 2 ||
			stats.UnpreparedNodes != 0 {
			t.Fatalf("notch %d PDM text = %+v", notch, stats)
		}
	}

	tableRect := canvasRect(camera, nodes[targetOffset].Bounds)
	if tableRect.W < 27 || tableRect.W > 30 || tableRect.H < 8 {
		t.Fatalf("three-notch table size = %.3fx%.3f DIP", tableRect.W, tableRect.H)
	}
	imageFrame := tt.Image()
	titleBand := ui.Rect{
		X: tableRect.X + tableRect.W*0.08,
		Y: tableRect.Y + 0.5,
		W: tableRect.W * 0.84,
		H: 2.5,
	}
	if !imageHasPixelInDIPRect(imageFrame, titleBand, 2, func(pixel color.RGBA) bool {
		return pixel.R < 180 && pixel.G < 180 && pixel.B < 180
	}) {
		t.Fatalf("three-notch PDM table title was not rasterized in %+v", titleBand)
	}
	fieldBand := ui.Rect{
		X: tableRect.X + 0.5,
		Y: tableRect.Y + 3.2,
		W: tableRect.W - 1,
		H: tableRect.H - 4.2,
	}
	if !imageHasPixelInDIPRect(imageFrame, fieldBand, 2, func(pixel color.RGBA) bool {
		return pixel.R < 180 && pixel.G < 180 && pixel.B < 180
	}) {
		t.Fatalf("three-notch PDM table fields were not rasterized in %+v", fieldBand)
	}

	setScale(camera, (tableTextMinWidth-0.2)/referenceW)
	renderer.PrepareVisible(camera, graph.Hit{})
	tt.Frame()
	if stats.FieldRows != 0 {
		t.Fatalf("below-threshold PDM table text remained visible: %+v", stats)
	}
}

func TestOverviewKeepsTextForAllVisiblePDMTables(t *testing.T) {
	const tableCount = 300
	presentation := &graph.TablePresentation{
		Title:   "Stock",
		Columns: []graph.ColumnPresentation{{Name: "id", DataType: "INT"}},
	}
	scene := graph.Scene{Nodes: make([]graph.Node, tableCount)}
	for index := range scene.Nodes {
		x, y := float64(index%20*245), float64(index/20*152)
		scene.Nodes[index] = graph.Node{
			ID: index,
			Bounds: graph.Rect{
				Min: graph.Point{X: x, Y: y},
				Max: graph.Point{X: x + 236, Y: y + 144},
			},
			Presentation: presentation,
		}
	}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 700, Y: 300}})
	setScale(camera, 0.12)
	renderer := New(scene, graph.NewSpatialIndex(scene))
	renderer.PrepareVisible(camera, graph.Hit{})
	var stats FrameStats
	tt := host(renderer, camera, graph.Hit{}, 700, 300, &stats)
	if stats.VisibleNodes != tableCount || stats.FieldRows != tableCount || stats.UnpreparedNodes != 0 {
		t.Fatalf("dense overview text = %+v", stats)
	}
	setScale(camera, 0.03)
	renderer.PrepareVisible(camera, graph.Hit{})
	tt.Frame()
	if stats.FieldRows != 0 || len(renderer.nodes) > nodeVisualLimit {
		t.Fatalf("tiny overview should prune excess cached layouts: frame=%+v cached=%d", stats, len(renderer.nodes))
	}
}

func TestTinyPDMCardsSkipPreparationAndStatsAtMediumAndNear(t *testing.T) {
	for _, lod := range []LOD{Medium, Near} {
		nodes := make([]graph.Node, 65)
		for offset := range nodes {
			nodes[offset] = graph.Node{
				ID:     offset,
				Bounds: graph.Rect{Max: graph.Point{X: 236, Y: 80}},
			}
		}
		nodes[64] = graph.Node{
			ID:     64,
			Bounds: graph.Rect{Min: graph.Point{X: 10}, Max: graph.Point{X: 33.75, Y: 20}},
			Presentation: &graph.TablePresentation{
				Title: "Tiny",
				Columns: []graph.ColumnPresentation{
					{Name: "id", DataType: "INT"},
				},
			},
		}
		scene := graph.Scene{Nodes: nodes}
		camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 100, Y: 40}})
		camera.ZoomAt(graph.Point{}, 0.8)
		renderer := New(scene, graph.NewSpatialIndex(scene))
		renderer.lod = lod
		if prepared := renderer.PrepareVisible(camera, graph.Hit{}); prepared != 0 {
			t.Fatalf("LOD %q prepared %d tiny PDM cards", lod, prepared)
		}
		stats := FrameStats{}
		host(renderer, camera, graph.Hit{}, 100, 40, &stats)
		if stats.LOD != lod || stats.FieldRows != 0 || stats.UnpreparedNodes != 0 ||
			renderer.nodeVisual(64) != nil {
			t.Fatalf("LOD %q tiny PDM card = %+v", lod, stats)
		}
	}
}

func TestShortMultilineAnnotationRemainsVisibleThroughThreeZoomOutNotches(t *testing.T) {
	const (
		referenceW = 236.0
		initialDIP = 85.0
	)
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 100, Y: 40}})
	setScale(camera, initialDIP/referenceW)
	annotation := graph.Annotation{
		Bounds: graph.Rect{
			Min: graph.Point{X: 10, Y: 10},
			Max: graph.Point{X: 10 + referenceW, Y: 10 + 11/camera.Scale()},
		},
		Text:      "First\nSecond",
		Fill:      &pdm.RGBColor{},
		Border:    &pdm.RGBColor{},
		TextColor: &pdm.RGBColor{R: 255, G: 255, B: 255},
	}
	scene := graph.Scene{Annotations: []graph.Annotation{annotation}}
	renderer := New(scene, graph.NewSpatialIndex(scene))
	step := math.Exp(-0.36)
	stats := FrameStats{}
	tt := host(renderer, camera, graph.Hit{}, 100, 40, &stats)
	tt.SetScale(2)

	baseline := canvasRect(camera, annotation.Bounds)
	if baseline.H < 10.9 || baseline.H > 11.1 {
		t.Fatalf("baseline annotation height = %.3f DIP", baseline.H)
	}
	for notch := 1; notch <= 3; notch++ {
		setScale(camera, camera.Scale()*step)
		tt.Frame()
	}

	rect := canvasRect(camera, annotation.Bounds)
	if rect.H < 3.7 || rect.H > 3.8 {
		t.Fatalf("three-notch annotation height = %.3f DIP", rect.H)
	}
	imageFrame := tt.Image()
	insetX := min(0.5, rect.W/8)
	insetY := min(0.25, rect.H/8)
	middle := rect.Y + rect.H/2
	lineBands := []ui.Rect{
		{
			X: rect.X + insetX,
			Y: rect.Y + insetY,
			W: rect.W - 2*insetX,
			H: middle - rect.Y - 2*insetY,
		},
		{
			X: rect.X + insetX,
			Y: middle,
			W: rect.W - 2*insetX,
			H: rect.Y + rect.H - middle - insetY,
		},
	}
	for index, band := range lineBands {
		if !imageHasPixelInDIPRect(imageFrame, band, 2, func(pixel color.RGBA) bool {
			return pixel.R > 80 && pixel.G > 80 && pixel.B > 80
		}) {
			t.Fatalf("three-notch annotation line %d was not visible in %+v", index+1, band)
		}
	}

}

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
	if stats.LOD != Medium || stats.VisibleNodes != 1 || stats.FieldRows != 1 {
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

func TestRendererGridTilesAreBoundedAtNearAndFarZoom(t *testing.T) {
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
	if nearTiles < 1 || nearTiles > 200 || farTiles < 1 || farTiles > 200 {
		t.Fatalf("grid tiles outside bounded screen-space count: 1x=%d .02x=%d", nearTiles, farTiles)
	}
	if stats.VisibleNodes == 0 {
		t.Fatal("far zoom unexpectedly culled the whole scene")
	}
}

func TestRendererGridFollowsWorldCoordinates(t *testing.T) {
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 180, Y: 160}})
	renderer := New(graph.Scene{}, nil)
	tt := host(renderer, camera, graph.Hit{}, 180, 160, &FrameStats{})
	tt.SetDark(false)
	assertLine := func(x int) {
		t.Helper()
		img := tt.Image()
		if line, bg := img.RGBAAt(x, 20), img.RGBAAt(x+4, 20); line.R >= bg.R {
			t.Fatalf("expected a grid line at x=%d: line=%v background=%v", x, line, bg)
		}
	}
	assertLine(64)
	camera.Pan(graph.Point{X: 29})
	tt.Frame()
	assertLine(35)
	camera.ZoomAt(graph.Point{}, 2)
	tt.Frame()
	assertLine(6)
}

func TestRendererThemeKeepsWhiteTableAndBlackMultilineNoteVisible(t *testing.T) {
	white := &pdm.RGBColor{R: 255, G: 255, B: 255}
	black := &pdm.RGBColor{}
	scene := graph.Scene{
		Nodes: []graph.Node{{
			Bounds: graph.Rect{Min: graph.Point{X: 16, Y: 72}, Max: graph.Point{X: 148, Y: 122}},
			Presentation: &graph.TablePresentation{
				Style: pdm.TableAppearance{Fill: white, Border: white},
			},
		}},
		Annotations: []graph.Annotation{{
			Bounds: graph.Rect{Min: graph.Point{X: 16, Y: 16}, Max: graph.Point{X: 148, Y: 66}},
			Text:   "Stock\r\nWarehouse", Fill: black, Border: black, TextColor: black,
		}},
	}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: 180, Y: 160}})
	tt := host(New(scene, nil), camera, graph.Hit{}, 180, 160, &FrameStats{})
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		img := tt.Image()
		bg, grid := img.RGBAAt(170, 30), img.RGBAAt(128, 10)
		if bg.R == 255 || bg.R == 0 || (grid.R > bg.R) != dark || grid == bg {
			t.Fatalf("dark=%t unexpected canvas colors: background=%v grid=%v", dark, bg, grid)
		}
		fill, border, shadow := img.RGBAAt(20, 84), img.RGBAAt(16, 84), img.RGBAAt(40, 123)
		if fill.R != 255 || border == fill || border == bg || shadow.R >= bg.R {
			t.Fatalf("dark=%t expected original white fill, distinct border and shadow: fill=%v border=%v shadow=%v", dark, fill, border, shadow)
		}
		if pixel := img.RGBAAt(20, 25); pixel.R != 0 || pixel.G != 0 || pixel.B != 0 {
			t.Fatalf("dark=%t black annotation fill changed: %v", dark, pixel)
		}
		if edge := img.RGBAAt(16, 37); dark && edge.R <= bg.R {
			t.Fatalf("dark canvas needs a brighter annotation outline: outline=%v background=%v", edge, bg)
		}
		for _, band := range [][2]int{{22, 41}, {42, 62}} {
			visibleText := false
			for y := band[0]; y < band[1] && !visibleText; y++ {
				for x := 36; x < 128; x++ {
					if img.RGBAAt(x, y).R > 180 {
						visibleText = true
						break
					}
				}
			}
			if !visibleText {
				t.Fatalf("dark=%t missing contrasting annotation text in band %v", dark, band)
			}
		}
	}
}

func TestRendererRetainsTextLayoutsAndSelection(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 400, Y: 300}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	visual := renderer.nodeVisual(0)
	if visual == nil {
		t.Fatal("node 0 was not prepared")
	}
	title := visual.name.full
	field := visual.fieldLabels[0].full
	stats := FrameStats{}
	selected := graph.Hit{Kind: graph.HitNode, Index: 0}
	tt := host(renderer, camera, selected, 400, 300, &stats)
	tt.Frame()
	if renderer.nodeVisual(0).name.full != title || renderer.nodeVisual(0).fieldLabels[0].full != field {
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
	visual := renderer.nodeVisual(0)
	if visual == nil {
		t.Fatal("node 0 was not prepared")
	}
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
	first, second := renderer.nodeVisual(0), renderer.nodeVisual(1)
	if first == nil || second == nil {
		t.Fatal("small-card fixture nodes were not prepared")
	}
	if first.name.pick(2, 93) != nil ||
		first.fieldLabels[0].pick(2, fieldRowHeight) != nil {
		t.Fatal("too-wide text selected a layout")
	}
	if second.name.pick(184, 1) != nil ||
		second.code.pick(184, 1) != nil {
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
	name := renderer.nodeVisual(0).name.full
	field := renderer.nodeVisual(0).fieldLabels[0].full
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
	if renderer.nodeVisual(0).name.full != name || renderer.nodeVisual(0).fieldLabels[0].full != field {
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

func TestSceneCalibratedLODNearForScaledPDMUnits(t *testing.T) {
	scene := graph.Scene{Nodes: []graph.Node{{
		ID:     0,
		Bounds: graph.Rect{Min: graph.Point{X: 1_000_000, Y: 2_000_000}, Max: graph.Point{X: 1_002_360, Y: 2_001_440}},
		Name:   "Scaled PDM Table",
		Fields: []graph.Field{{Name: "id", DataType: "BIGINT"}, {Name: "name", DataType: "VARCHAR"}},
	}}}
	camera := graph.NewDiagramCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 760, Y: 560}})
	camera.FitBounds(scene.Nodes[0].Bounds, 20)
	renderer := New(scene, graph.NewSpatialIndex(scene))
	var stats FrameStats
	host(renderer, camera, graph.Hit{Kind: graph.HitNone, Index: -1}, 760, 560, &stats)
	if stats.LOD != Near || stats.FieldRows == 0 {
		t.Fatalf("scaled PDM LOD = %+v, camera scale=%v", stats, camera.Scale())
	}
}

func TestPrepareVisibleRetainsDesiredSetAndSkipsFarLOD(t *testing.T) {
	nodes := make([]graph.Node, 300)
	for offset := range nodes {
		x := float64(offset * 40)
		nodes[offset] = graph.Node{
			ID:     offset,
			Bounds: graph.Rect{Min: graph.Point{X: x, Y: 0}, Max: graph.Point{X: x + 20, Y: 10}},
			Name:   fmt.Sprintf("node-%03d", offset),
		}
	}
	scene := graph.Scene{Nodes: nodes}
	renderer := New(scene, graph.NewSpatialIndex(scene))
	for offset := 0; offset < 256; offset++ {
		renderer.PrepareNode(offset)
	}

	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 2000, Y: 100}})
	camera.ZoomAt(graph.Point{}, 10)
	camera.Pan(graph.Point{X: 20000, Y: 0})
	visible := renderer.index.QueryNodes(camera.VisibleWorld(), nil)
	prepared := renderer.PrepareVisible(camera, graph.Hit{Kind: graph.HitNode, Index: 260})
	if prepared < 1 {
		t.Fatalf("selected-node preparation count = %d", prepared)
	}
	for _, offset := range visible {
		if renderer.nodeVisual(offset) == nil {
			t.Fatalf("visible node %d was evicted", offset)
		}
	}
	if renderer.nodeVisual(260) == nil {
		t.Fatal("selected node was not retained")
	}
	if renderer.nodeVisual(0) != nil {
		t.Fatal("non-desired cached node was not evicted")
	}
	if renderer.nodeVisual(1) == nil || renderer.nodeVisual(255) == nil {
		t.Fatal("offscreen visuals were discarded while capacity remained")
	}
	nodesBeforeReuse := len(renderer.nodes)
	labelsBeforeReuse := len(renderer.labels.entries)
	if got := renderer.PrepareVisible(camera, graph.Hit{Kind: graph.HitNode, Index: 260}); got != 0 {
		t.Fatalf("repeat preparation shaped %d cached nodes", got)
	}
	if len(renderer.nodes) != nodesBeforeReuse || len(renderer.labels.entries) != labelsBeforeReuse {
		t.Fatal("repeat preparation churned cached node or label state")
	}

	farCamera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 2000, Y: 100}})
	farCamera.Pan(graph.Point{X: 2000, Y: 0})
	before := len(renderer.nodes)
	if got := renderer.PrepareVisible(farCamera, graph.Hit{Kind: graph.HitNone, Index: -1}); got != 0 {
		t.Fatalf("far-LOD prepared %d nodes", got)
	}
	if len(renderer.nodes) != before {
		t.Fatal("far-LOD preparation churned the cache")
	}
}

func TestRendererTextCachesAreBoundedInternedAndDeduplicated(t *testing.T) {
	nodes := make([]graph.Node, 600)
	for offset := range nodes {
		nodes[offset] = graph.Node{
			ID:     offset,
			Bounds: graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 100, Y: 100}},
			Name:   fmt.Sprintf("n%d", offset),
		}
	}
	scene := graph.Scene{Nodes: nodes}
	renderer := New(scene, graph.NewSpatialIndex(scene))
	if len(renderer.nodes) != prefillNodeLimit ||
		len(renderer.labels.entries) > prefillNodeLimit {
		t.Fatalf("eager preparation = nodes:%d labels:%d", len(renderer.nodes), len(renderer.labels.entries))
	}
	for offset := 0; offset < len(nodes); offset++ {
		renderer.PrepareNode(offset)
	}
	if len(renderer.nodes) > nodeVisualLimit || len(renderer.labels.entries) > labelInternLimit {
		t.Fatalf("cache bounds = nodes:%d labels:%d", len(renderer.nodes), len(renderer.labels.entries))
	}
	if renderer.nodeVisual(0) != nil || renderer.nodeVisual(599) == nil {
		t.Fatal("node cache eviction is not deterministic FIFO")
	}

	duplicate := graph.Scene{Nodes: []graph.Node{
		{ID: 0, Bounds: graph.Rect{Max: graph.Point{X: 100, Y: 100}}, Name: "Same", Fields: []graph.Field{{Name: "id", DataType: "INT"}}},
		{ID: 1, Bounds: graph.Rect{Min: graph.Point{X: 120}, Max: graph.Point{X: 220, Y: 100}}, Name: "Same", Fields: []graph.Field{{Name: "id", DataType: "INT"}}},
	}}
	shared := New(duplicate, graph.NewSpatialIndex(duplicate))
	first, second := shared.nodeVisual(0), shared.nodeVisual(1)
	if first == nil || second == nil ||
		first.name.full != second.name.full ||
		first.fieldLabels[0].full != second.fieldLabels[0].full {
		t.Fatal("identical text/font layouts were not interned")
	}
}

func TestPreviewTextCapAndTextBoundsWithoutCardClip(t *testing.T) {
	long := strings.Repeat("x", maxPreviewRunes+100)
	cache := newLabelCache(8)
	visual := cache.label(long, ui.Font{Size: 10})
	if visual == nil || visual.full == nil {
		t.Fatal("preview label was not shaped")
	}
	for key := range cache.entries {
		if utf8.RuneCountInString(key.text) > maxPreviewRunes+1 {
			t.Fatalf("preview key runes = %d", utf8.RuneCountInString(key.text))
		}
	}
	for _, variant := range visual.variants {
		if variant.layout == visual.full {
			t.Fatal("full and narrowed layout pointers were duplicated")
		}
	}

	scene := graph.Scene{Nodes: []graph.Node{{
		ID:     0,
		Bounds: graph.Rect{Min: graph.Point{Y: 10}, Max: graph.Point{X: 120, Y: 85}},
		Name:   "A very long table name that needs a narrowed layout",
		Code:   "VERY_LONG_CODE",
		Fields: []graph.Field{
			{Name: "identifier", DataType: "BIGINT", PrimaryKey: true},
			{Name: "description", DataType: "VARCHAR(255)"},
		},
	}}}
	camera := graph.NewCamera(graph.Rect{Min: graph.Point{}, Max: graph.Point{X: 200, Y: 100}})
	renderer := New(scene, graph.NewSpatialIndex(scene))
	nodeCard := renderer.nodeVisual(0)
	if nodeCard == nil {
		t.Fatal("small card was not prepared")
	}
	screenMin := camera.WorldToScreen(scene.Nodes[0].Bounds.Min)
	screenMax := camera.WorldToScreen(scene.Nodes[0].Bounds.Max)
	rect := ui.Rect{
		X: float32(screenMin.X),
		Y: float32(screenMin.Y),
		W: float32(screenMax.X - screenMin.X),
		H: float32(screenMax.Y - screenMin.Y),
	}
	availableWidth := rect.W - 2*labelInset
	title := nodeCard.name.pick(availableWidth, rect.H-titleTop-2)
	if !layoutWithin(title, rect.X+labelInset, rect.Y+titleTop, availableWidth, rect.H-titleTop-2) {
		t.Fatalf("title escaped card bounds: %+v", rect)
	}
	rows := min(len(nodeCard.fieldLabels), int((rect.H-nodeCard.fieldTop-2)/fieldRowHeight))
	for i := 0; i < rows; i++ {
		label := nodeCard.fieldLabels[i].pick(availableWidth, fieldRowHeight)
		if !layoutWithin(label, rect.X+labelInset, rect.Y+nodeCard.fieldTop+float32(i)*fieldRowHeight, availableWidth, fieldRowHeight) {
			t.Fatalf("field row %d escaped card bounds", i)
		}
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

func layoutWithin(layout *ui.TextLayout, x, y, width, height float32) bool {
	if layout == nil {
		return true
	}
	w, h := layout.Size()
	return w <= width && h <= height && x >= 0 && y >= 0
}

func imageHasPixelInDIPRect(
	img *image.RGBA,
	rect ui.Rect,
	pixelScale float64,
	match func(color.RGBA) bool,
) bool {
	x0 := max(0, int(math.Ceil(float64(rect.X)*pixelScale)))
	y0 := max(0, int(math.Ceil(float64(rect.Y)*pixelScale)))
	x1 := min(img.Rect.Dx(), int(math.Floor(float64(rect.X+rect.W)*pixelScale)))
	y1 := min(img.Rect.Dy(), int(math.Floor(float64(rect.Y+rect.H)*pixelScale)))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if match(img.RGBAAt(x, y)) {
				return true
			}
		}
	}
	return false
}
