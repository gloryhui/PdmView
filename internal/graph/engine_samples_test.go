//go:build pdm_layout_check

package graph

import (
	"math"
	"os"
	"reflect"
	"slices"
	"testing"

	"pdmview/internal/pdm"
)

func TestPhase5SampleCamera(t *testing.T) {
	model := openEngineSample(t)
	const padding = 24.0
	viewport := Rect{Min: Point{X: 37, Y: 51}, Max: Point{X: 1061, Y: 819}}
	camera := NewDiagramCamera(viewport)
	legacy := NewCamera(viewport)
	var diagrams, nodes, legacyClipped int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		mapped := BuildDiagramGraph(model, diagram)
		if len(mapped.Scene.Nodes) == 0 {
			continue
		}
		legacy.Fit(mapped.Scene.Bounds, padding)
		if !sampleFitsCamera(legacy, mapped.Scene.Bounds, padding) {
			legacyClipped++
		}
		camera.FitBounds(mapped.Scene.Bounds, padding)
		if !sampleFitsCamera(camera, mapped.Scene.Bounds, padding) {
			t.Fatalf("diagram %d did not fit the viewport with padding", diagrams+1)
		}
		for _, node := range mapped.Scene.Nodes {
			screen := camera.WorldToScreen(rectMiddle(node.Bounds))
			if roundTrip := camera.WorldToScreen(camera.ScreenToWorld(screen)); math.Hypot(screen.X-roundTrip.X, screen.Y-roundTrip.Y) > 1e-7 {
				t.Fatalf("diagram %d node %d screen round trip exceeded tolerance", diagrams+1, node.ID)
			}
			nodes++
		}
		diagrams++
	}
	if diagrams == 0 {
		t.Fatal("sample has no populated diagram")
	}
	t.Logf("1024x768 DIP viewport, 24 DIP padding: fitted_diagrams=%d node_round_trips=%d prototype_clipped_diagrams=%d", diagrams, nodes, legacyClipped)
}

func TestPhase5SampleIndex(t *testing.T) {
	model := openEngineSample(t)
	var pointQueries, viewQueries, replacements int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		scene := BuildDiagramGraph(model, diagram).Scene
		if len(scene.Nodes) == 0 {
			continue
		}
		index := NewSpatialIndex(scene)
		linear := NewLinearIndex(scene)
		camera := NewDiagramCamera(Rect{Max: Point{X: 1024, Y: 768}})
		camera.FitBounds(scene.Bounds, 24)
		camera.ZoomAt(Point{X: 500, Y: 300}, 2)
		view := camera.VisibleWorld()
		if !slices.Equal(index.QueryNodes(view, nil), linear.QueryNodes(view, nil)) ||
			!slices.Equal(index.QueryEdges(view, nil), linear.QueryEdges(view, nil)) {
			t.Fatal("viewport query differs from linear oracle")
		}
		viewQueries++
		for _, node := range scene.Nodes {
			point := rectMiddle(node.Bounds)
			got := index.QueryPoint(point, nil)
			want := linear.QueryNodes(Rect{Min: point, Max: point}, nil)
			if !slices.Equal(got, want) || !slices.Contains(got, node.ID) {
				t.Fatalf("node %d center did not match the linear oracle", node.ID)
			}
			pointQueries++
		}
		replacement := Scene{Nodes: scene.Nodes[1:]}
		index.ReplaceScene(replacement)
		linear = NewLinearIndex(replacement)
		if !slices.Equal(index.QueryNodes(scene.Bounds, nil), linear.QueryNodes(scene.Bounds, nil)) ||
			len(index.QueryNodes(scene.Bounds, nil)) != len(scene.Nodes)-1 || len(index.QueryEdges(scene.Bounds, nil)) != 0 {
			t.Fatal("scene replacement retained the removed node or old edges")
		}
		replacements++
	}
	if pointQueries == 0 {
		t.Fatal("sample has no populated diagram")
	}
	t.Logf("BVH matched linear oracle: point_queries=%d viewport_queries=%d deletion_snapshots=%d", pointQueries, viewQueries, replacements)
}

func TestPhase5SampleHit(t *testing.T) {
	model := openEngineSample(t)
	viewport := Rect{Min: Point{X: 37, Y: 51}, Max: Point{X: 1061, Y: 819}}
	var checked int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		if len(diagram.TableSymbols) == 0 {
			continue
		}
		mapped := BuildDiagramGraph(model, diagram)
		index := NewSpatialIndex(mapped.Scene)
		camera := NewDiagramCamera(viewport)
		for offset, node := range mapped.Scene.Nodes {
			center := rectMiddle(node.Bounds)
			winner := offset
			for next := offset + 1; next < len(mapped.Scene.Nodes); next++ {
				other := mapped.Scene.Nodes[next].Bounds
				if center.X >= other.Min.X && center.X <= other.Max.X &&
					center.Y >= other.Min.Y && center.Y <= other.Max.Y {
					winner = next
				}
			}
			for _, factor := range []float64{0.5, 1, 2} {
				camera.FitBounds(mapped.Scene.Bounds, 24)
				screen := camera.WorldToScreen(center)
				camera.ZoomAt(camera.ScreenToViewport(screen), factor)
				hit := mapped.HitTest(index, camera, screen, 5)
				want := diagram.TableSymbols[winner]
				if hit.Kind != HitNode || hit.Index != winner || hit.DiagramID != diagram.ID ||
					hit.TableID != want.TableID || hit.SymbolID != want.ID {
					t.Fatalf("node %d failed indexed screen hit at %.1fx fit scale", offset, factor)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("sample has no populated diagram")
	}
	t.Logf("node hits checked at 0.5x, 1x and 2x fitted zoom: %d", checked)
}

func TestPhase5SampleEnginePaths(t *testing.T) {
	model := openEngineSample(t)
	var diagrams, preserved, points int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		if len(diagram.TableSymbols) == 0 {
			continue
		}
		diagrams++
		projection := MapDiagram(model, diagram)
		prepared := BuildDiagramGraph(model, diagram)
		if prepared.DiagramID != projection.DiagramID || prepared.AutoLayout != projection.AutoLayout ||
			!reflect.DeepEqual(prepared.Scene.Nodes, projection.Scene.Nodes) ||
			!reflect.DeepEqual(prepared.NodeSources, projection.NodeSources) ||
			!reflect.DeepEqual(prepared.EdgeSources, projection.EdgeSources) ||
			len(prepared.Scene.Edges) != len(projection.Scene.Edges) {
			t.Fatalf("prepared scene changed the explicit symbols of diagram %d", diagrams)
		}
		for index, edge := range projection.Scene.Edges {
			if len(edge.Points) == 0 {
				continue
			}
			got := prepared.Scene.Edges[index]
			if got.ID != edge.ID || got.From != edge.From || got.To != edge.To || !slices.Equal(got.Points, edge.Points) {
				t.Fatalf("diagram %d stored path %d changed in the graph engine", diagrams, index)
			}
			preserved++
			points += len(edge.Points)
		}
	}
	if diagrams == 0 {
		t.Fatal("sample has no explicitly placed table symbols")
	}
	t.Logf("prepared diagrams=%d preserved_source_paths=%d preserved_source_points=%d", diagrams, preserved, points)
}

func TestPhase5SampleRoutingWithPathsOmitted(t *testing.T) {
	model := openEngineSample(t)
	var available, routed, segments int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		if len(diagram.TableSymbols) == 0 || len(diagram.ReferenceSymbols) == 0 {
			continue
		}
		withoutPaths := *diagram
		withoutPaths.ReferenceSymbols = slices.Clone(diagram.ReferenceSymbols)
		for index := range withoutPaths.ReferenceSymbols {
			withoutPaths.ReferenceSymbols[index].Points = nil
		}
		prepared := BuildDiagramGraph(model, &withoutPaths)
		if len(prepared.Scene.Edges) != len(diagram.ReferenceSymbols) {
			t.Fatal("fallback altered the number of existing relationship symbols")
		}
		for index, edge := range prepared.Scene.Edges {
			available++
			if len(edge.Points) == 0 {
				continue
			}
			if len(prepared.EdgeSources[index].Points) != 0 {
				t.Fatalf("fallback path %d became purported source geometry", index)
			}
			segments += checkGeneratedPath(t, prepared.Scene, index)
			routed++
		}
	}
	if available == 0 {
		t.Skip("sample has no relationship symbols")
	}
	if routed == 0 {
		t.Fatal("no fallback relationship could be routed")
	}
	t.Logf("in-memory paths omitted=%d routed=%d empty=%d orthogonal_segments_checked=%d", available, routed, available-routed, segments)
}

func TestPhase5DenseRoutes(t *testing.T) {
	for _, count := range []int{50, 150, 300, 500} {
		scene := routeBenchmarkScene(count)
		routeMissingEdges(&scene)
		segments := 0
		for offset := range scene.Edges {
			segments += checkGeneratedPath(t, scene, offset)
		}
		t.Logf("dense nodes=%d routed_edges=%d orthogonal_segments_checked=%d", count, len(scene.Edges), segments)
	}
}

func checkGeneratedPath(t *testing.T, scene Scene, index int) int {
	t.Helper()
	edge := scene.Edges[index]
	if len(edge.Points) < 2 || edge.From < 0 || edge.From >= len(scene.Nodes) ||
		edge.To < 0 || edge.To >= len(scene.Nodes) {
		t.Fatalf("fallback path %d has no valid endpoints in %d-node scene", index, len(scene.Nodes))
	}
	if !sampleOnTablePerimeter(edge.Points[0], scene.Nodes[edge.From].Bounds) ||
		!sampleOnTablePerimeter(edge.Points[len(edge.Points)-1], scene.Nodes[edge.To].Bounds) {
		t.Fatalf("fallback path %d does not join both table boundaries", index)
	}
	for segment := 1; segment < len(edge.Points); segment++ {
		start, end := edge.Points[segment-1], edge.Points[segment]
		if start == end || (start.X != end.X && start.Y != end.Y) {
			t.Fatalf("fallback path %d has a non-orthogonal or zero-length segment", index)
		}
		for _, node := range scene.Nodes {
			if sampleSegmentCrossesInterior(start, end, node.Bounds) {
				t.Fatalf("fallback path %d crosses the interior of a table", index)
			}
		}
	}
	return len(edge.Points) - 1
}

func sampleOnTablePerimeter(point Point, bounds Rect) bool {
	return (point.X == bounds.Min.X || point.X == bounds.Max.X) && point.Y >= bounds.Min.Y && point.Y <= bounds.Max.Y ||
		(point.Y == bounds.Min.Y || point.Y == bounds.Max.Y) && point.X >= bounds.Min.X && point.X <= bounds.Max.X
}

func sampleSegmentCrossesInterior(start, end Point, bounds Rect) bool {
	if start.X == end.X {
		return start.X > bounds.Min.X && start.X < bounds.Max.X &&
			max(min(start.Y, end.Y), bounds.Min.Y) < min(max(start.Y, end.Y), bounds.Max.Y)
	}
	return start.Y > bounds.Min.Y && start.Y < bounds.Max.Y &&
		max(min(start.X, end.X), bounds.Min.X) < min(max(start.X, end.X), bounds.Max.X)
}

func openEngineSample(t *testing.T) *pdm.Model {
	t.Helper()
	path := os.Getenv("PDM_LAYOUT_SAMPLE")
	if path == "" {
		t.Skip("set PDM_LAYOUT_SAMPLE to a local XML export")
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal("could not open the local XML export")
	}
	defer input.Close()
	document, err := pdm.Parse(input)
	if err != nil {
		t.Fatal("PDM parser rejected the local XML export")
	}
	return &document.Model
}

func sampleDiagrams(contents *pdm.Contents) []*pdm.Diagram {
	var diagrams []*pdm.Diagram
	for index := range contents.Diagrams {
		diagrams = append(diagrams, &contents.Diagrams[index])
	}
	for index := range contents.Packages {
		diagrams = append(diagrams, sampleDiagrams(&contents.Packages[index].Contents)...)
	}
	return diagrams
}

func sampleFitsCamera(camera *Camera, world Rect, padding float64) bool {
	viewport := camera.ViewportBounds()
	leftTop := camera.WorldToScreen(world.Min)
	rightBottom := camera.WorldToScreen(world.Max)
	const epsilon = 1e-7
	return leftTop.X >= viewport.Min.X+padding-epsilon &&
		leftTop.Y >= viewport.Min.Y+padding-epsilon &&
		rightBottom.X <= viewport.Max.X-padding+epsilon &&
		rightBottom.Y <= viewport.Max.Y-padding+epsilon
}
