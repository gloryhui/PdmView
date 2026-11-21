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
	title := renderer.nodes[0].title
	field := renderer.nodes[0].fields[0]
	stats := FrameStats{}
	selected := graph.Hit{Kind: graph.HitNode, Index: 0}
	tt := host(renderer, camera, selected, 400, 300, &stats)
	tt.Frame()
	if renderer.nodes[0].title != title || renderer.nodes[0].fields[0] != field {
		t.Fatal("Draw reshaped retained text layouts")
	}
	if stats.VisibleNodes == 0 || stats.FieldRows == 0 {
		t.Fatalf("selected near scene stats = %+v", stats)
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

func setScale(camera *graph.Camera, scale float64) {
	camera.ZoomAt(graph.Point{}, scale/camera.Scale())
}
