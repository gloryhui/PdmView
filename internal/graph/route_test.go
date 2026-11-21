package graph

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"pdmview/internal/pdm"
)

func TestRouteMissingEdgesAvoidsObstaclesAndPreservesExistingPath(t *testing.T) {
	existing := []Point{{X: 10, Y: 10}, {X: 20, Y: 10}}
	scene := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 180, Y: -60}, Max: Point{X: 320, Y: 160}}},
			{ID: 2, Bounds: Rect{Min: Point{X: 400, Y: 0}, Max: Point{X: 500, Y: 100}}},
		},
		Edges: []Edge{
			{ID: 0, From: 0, To: 2},
			{ID: 1, From: 0, To: 2, Points: append([]Point(nil), existing...)},
		},
	}

	routeMissingEdges(&scene)
	assertRouteClear(t, scene, 0)
	if !reflect.DeepEqual(scene.Edges[1].Points, existing) {
		t.Fatalf("existing path changed: %+v", scene.Edges[1].Points)
	}
	if scene.Edges[0].Points[0].X != 100 || scene.Edges[0].Points[len(scene.Edges[0].Points)-1].X != 400 {
		t.Fatalf("route endpoints = %+v", scene.Edges[0].Points)
	}
}

func TestRouteMissingEdgesMultipleBlockersDeterministic(t *testing.T) {
	makeScene := func() Scene {
		return Scene{
			Nodes: []Node{
				{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
				{ID: 1, Bounds: Rect{Min: Point{X: 180, Y: -260}, Max: Point{X: 320, Y: -40}}},
				{ID: 2, Bounds: Rect{Min: Point{X: 180, Y: 40}, Max: Point{X: 320, Y: 260}}},
				{ID: 3, Bounds: Rect{Min: Point{X: 400, Y: 0}, Max: Point{X: 500, Y: 100}}},
			},
			Edges: []Edge{{ID: 0, From: 0, To: 3}},
		}
	}
	first, second := makeScene(), makeScene()
	routeMissingEdges(&first)
	routeMissingEdges(&second)
	if !reflect.DeepEqual(first.Edges[0].Points, second.Edges[0].Points) {
		t.Fatalf("routing is not deterministic:\n%+v\n%+v", first.Edges[0].Points, second.Edges[0].Points)
	}
	assertRouteClear(t, first, 0)
}

func TestRouteMissingEdgesUnsafeOrNarrowLeavesPathEmpty(t *testing.T) {
	enclosedTarget := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 300, Y: 300}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 50, Y: 50}, Max: Point{X: 100, Y: 100}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 1}},
	}
	routeMissingEdges(&enclosedTarget)
	if len(enclosedTarget.Edges[0].Points) != 0 {
		t.Fatalf("enclosed endpoint route was fabricated: %+v", enclosedTarget.Edges[0].Points)
	}

	contained := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 20, Y: 20}, Max: Point{X: 40, Y: 40}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 300, Y: 0}, Max: Point{X: 400, Y: 100}}},
			{ID: 2, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 1}},
	}
	routeMissingEdges(&contained)
	if len(contained.Edges[0].Points) != 0 {
		t.Fatalf("contained endpoint route was fabricated: %+v", contained.Edges[0].Points)
	}

	invalid := Scene{
		Nodes: []Node{{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 10, Y: 10}}}},
		Edges: []Edge{{ID: 0, From: -1, To: 0}},
	}
	routeMissingEdges(&invalid)
	if len(invalid.Edges[0].Points) != 0 {
		t.Fatalf("invalid endpoint route = %+v", invalid.Edges[0].Points)
	}
}

func TestRouteMissingEdgesUsesValidNarrowChannelAndAlternateSides(t *testing.T) {
	narrow := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 105, Y: 0}, Max: Point{X: 205, Y: 100}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 1}},
	}
	routeMissingEdges(&narrow)
	assertRouteClear(t, narrow, 0)

	sideBlocked := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 100, Y: 0}, Max: Point{X: 120, Y: 100}}},
			{ID: 2, Bounds: Rect{Min: Point{X: 300, Y: 0}, Max: Point{X: 400, Y: 100}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 2}},
	}
	routeMissingEdges(&sideBlocked)
	assertRouteClear(t, sideBlocked, 0)
}

func TestRouteMissingEdgesSelfLoopSharedPortsAndCrossings(t *testing.T) {
	scene := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 200, Y: 0}, Max: Point{X: 300, Y: 100}}},
		},
		Edges: []Edge{
			{ID: 0, From: 0, To: 0},
			{ID: 1, From: 0, To: 1},
			{ID: 2, From: 0, To: 1},
			{ID: 3, From: 1, To: 0},
		},
	}
	routeMissingEdges(&scene)
	for offset := range scene.Edges {
		assertRouteClear(t, scene, offset)
	}
	self := scene.Edges[0].Points
	if len(self) < 3 || self[0] == self[len(self)-1] {
		t.Fatalf("self-loop endpoints not distinct: %+v", self)
	}
	if scene.Edges[1].Points[0] == scene.Edges[2].Points[0] {
		t.Fatalf("shared source-side ports overlap: %+v", scene.Edges[1].Points[0])
	}
	if scene.Edges[1].Points[len(scene.Edges[1].Points)-1] ==
		scene.Edges[2].Points[len(scene.Edges[2].Points)-1] {
		t.Fatalf("shared destination-side ports overlap")
	}
}

func TestRouteMissingEdgesUsesDistinctPortsWhenPreferredSideIsBlocked(t *testing.T) {
	scene := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 100, Y: -20}, Max: Point{X: 160, Y: 120}}},
			{ID: 2, Bounds: Rect{Min: Point{X: 250, Y: -30}, Max: Point{X: 350, Y: 40}}},
			{ID: 3, Bounds: Rect{Min: Point{X: 250, Y: 60}, Max: Point{X: 350, Y: 130}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 2}, {ID: 1, From: 0, To: 3}},
	}
	routeMissingEdges(&scene)
	assertRouteClear(t, scene, 0)
	assertRouteClear(t, scene, 1)
	first, second := scene.Edges[0].Points[0], scene.Edges[1].Points[0]
	if first.Y != 0 || second.Y != 0 || first == second {
		t.Fatalf("alternate top-side ports are not distinct: %+v and %+v", first, second)
	}
}

func TestCompactRouteDoesNotRemoveUTurn(t *testing.T) {
	points := []Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 0}}
	compacted := compactRoute(points)
	if !reflect.DeepEqual(compacted, points) {
		t.Fatalf("U-turn was compacted: %+v", compacted)
	}
	if got := compactRoute([]Point{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 10, Y: 0}}); !reflect.DeepEqual(
		got,
		[]Point{{X: 0, Y: 0}, {X: 10, Y: 0}},
	) {
		t.Fatalf("between-point compaction = %+v", got)
	}
}

func TestRouteMissingEdgesAllowsRelationshipsToCross(t *testing.T) {
	scene := Scene{
		Nodes: []Node{
			{ID: 0, Bounds: Rect{Min: Point{X: 0, Y: 90}, Max: Point{X: 20, Y: 110}}},
			{ID: 1, Bounds: Rect{Min: Point{X: 180, Y: 90}, Max: Point{X: 200, Y: 110}}},
			{ID: 2, Bounds: Rect{Min: Point{X: 90, Y: 0}, Max: Point{X: 110, Y: 20}}},
			{ID: 3, Bounds: Rect{Min: Point{X: 90, Y: 180}, Max: Point{X: 110, Y: 200}}},
		},
		Edges: []Edge{{ID: 0, From: 0, To: 1}, {ID: 1, From: 2, To: 3}},
	}
	routeMissingEdges(&scene)
	horizontal := []Point{{X: 20, Y: 100}, {X: 180, Y: 100}}
	vertical := []Point{{X: 100, Y: 20}, {X: 100, Y: 180}}
	if !slices.Equal(scene.Edges[0].Points, horizontal) || !slices.Equal(scene.Edges[1].Points, vertical) {
		t.Fatalf("expected both orthogonal relationships to be retained: %+v", scene.Edges)
	}
}

func TestBuildDiagramGraphExplicitUsesOnlyReferenceSymbols(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{
				mappingTable("table-a", "A", "A"),
				mappingTable("table-b", "B", "B"),
				mappingTable("blocker", "Blocker", "BLOCKER"),
			},
			References: []pdm.Reference{
				{Metadata: pdm.Metadata{ID: "logical-only"}, ChildTableID: "table-a", ParentTableID: "table-b"},
			},
		},
	}
	existing := []pdm.Point{{X: 10, Y: 10}, {X: 20, Y: 10}}
	controls := []pdm.Point{{X: 15, Y: 15}}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "symbol-a"}, TableID: "table-a", Rect: mappingRect(0, 0, 100, 100)},
			{Metadata: pdm.Metadata{ID: "symbol-blocker"}, TableID: "blocker", Rect: mappingRect(180, -60, 320, 160)},
			{Metadata: pdm.Metadata{ID: "symbol-b"}, TableID: "table-b", Rect: mappingRect(400, 0, 500, 100)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{
			{
				Metadata:            pdm.Metadata{ID: "edge-existing"},
				ReferenceID:         "reference-existing",
				SourceSymbolID:      "symbol-a",
				DestinationSymbolID: "symbol-b",
				Points:              append([]pdm.Point(nil), existing...),
				ControlPoints:       append([]pdm.Point(nil), controls...),
			},
			{
				Metadata:            pdm.Metadata{ID: "edge-missing"},
				ReferenceID:         "reference-missing",
				SourceSymbolID:      "symbol-a",
				DestinationSymbolID: "symbol-b",
			},
		},
	}

	result := BuildDiagramGraph(model, diagram)
	if len(result.Scene.Edges) != 2 {
		t.Fatalf("explicit diagram added logical fallback edges: %+v", result.Scene.Edges)
	}
	if !reflect.DeepEqual(result.Scene.Edges[0].Points, graphPoints(existing)) {
		t.Fatalf("existing path changed: %+v", result.Scene.Edges[0].Points)
	}
	if !reflect.DeepEqual(result.EdgeSources[0].Points, existing) ||
		!reflect.DeepEqual(result.EdgeSources[0].ControlPoints, controls) {
		t.Fatalf("existing source geometry changed: %+v", result.EdgeSources[0])
	}
	assertRouteClear(t, result.Scene, 1)
	if result.Scene.Bounds != sceneBounds(result.Scene.Nodes, result.Scene.Edges) {
		t.Fatalf("bounds not recomputed: %+v", result.Scene.Bounds)
	}
}

func TestBuildDiagramGraphFallbackAddsUniqueLogicalReferences(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{
				mappingTable("one", "One", "ONE"),
				mappingTable("two", "Two", "TWO"),
			},
			References: []pdm.Reference{
				{Metadata: pdm.Metadata{ID: "covered"}, ChildTableID: "one", ParentTableID: "two"},
				{Metadata: pdm.Metadata{ID: "added"}, ChildTableID: "two", ParentTableID: "one"},
				{Metadata: pdm.Metadata{ID: "missing-end"}, ChildTableID: "one", ParentTableID: "missing"},
			},
		},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "empty-diagram"},
		OwnerID:  "model",
		ReferenceSymbols: []pdm.ReferenceSymbol{{
			Metadata:    pdm.Metadata{ID: "existing-symbol"},
			ReferenceID: "covered",
		}},
	}

	result := BuildDiagramGraph(model, diagram)
	if len(result.Scene.Edges) != 2 || len(result.EdgeSources) != 2 {
		t.Fatalf("fallback edges/sources = %d/%d", len(result.Scene.Edges), len(result.EdgeSources))
	}
	if result.EdgeSources[0].ReferenceID != "covered" ||
		result.EdgeSources[1].ReferenceID != "added" ||
		result.EdgeSources[1].SymbolID != "" {
		t.Fatalf("fallback source identity = %+v", result.EdgeSources)
	}
	one, _ := visibleTableOffset(result, "one")
	two, _ := visibleTableOffset(result, "two")
	if result.Scene.Edges[1].From != two || result.Scene.Edges[1].To != one {
		t.Fatalf("logical reference direction = %+v, want child %d -> parent %d",
			result.Scene.Edges[1], two, one)
	}

	nilDiagram := BuildDiagramGraph(model, nil)
	if len(nilDiagram.EdgeSources) != 2 ||
		!slices.Equal(
			[]string{nilDiagram.EdgeSources[0].ReferenceID, nilDiagram.EdgeSources[1].ReferenceID},
			[]string{"covered", "added"},
		) {
		t.Fatalf("nil diagram fallback = %+v", nilDiagram.EdgeSources)
	}
}

func visibleTableOffset(result DiagramGraph, tableID string) (int, bool) {
	for offset := range result.Scene.Nodes {
		source, ok := result.NodeSource(offset)
		if ok && source.TableID == tableID {
			return offset, true
		}
	}
	return 0, false
}

func assertRouteClear(t *testing.T, scene Scene, edgeOffset int) {
	t.Helper()
	if edgeOffset < 0 || edgeOffset >= len(scene.Edges) {
		t.Fatalf("edge offset %d out of range", edgeOffset)
	}
	points := scene.Edges[edgeOffset].Points
	if len(points) < 2 {
		t.Fatalf("edge %d was not routed: %+v", edgeOffset, points)
	}
	index := NewSpatialIndex(Scene{Nodes: scene.Nodes})
	if !routePathClear(points, &scene, index) {
		t.Fatalf("edge %d crosses a node interior: %+v", edgeOffset, points)
	}
	for offset := 1; offset < len(points); offset++ {
		if points[offset-1].X != points[offset].X && points[offset-1].Y != points[offset].Y {
			t.Fatalf("edge %d contains a diagonal segment: %+v", edgeOffset, points)
		}
	}
}

func BenchmarkRouteMissingEdges(b *testing.B) {
	for _, nodeCount := range []int{50, 150, 300, 500} {
		b.Run(fmt.Sprintf("nodes-%d/edges-%d", nodeCount, 2*nodeCount), func(b *testing.B) {
			base := routeBenchmarkScene(nodeCount)
			totalRouted := 0
			b.ReportAllocs()
			for b.Loop() {
				scene := Scene{Nodes: base.Nodes, Edges: slices.Clone(base.Edges)}
				routeMissingEdges(&scene)
				totalRouted += countRoutedEdges(scene)
			}
			b.ReportMetric(float64(totalRouted)/float64(b.N), "routed/op")
		})
	}
}

func routeBenchmarkScene(nodeCount int) Scene {
	generated, err := GenerateDefault(nodeCount, DensityDense)
	if err != nil {
		panic(err)
	}
	for offset := range generated.Edges {
		generated.Edges[offset].Points = nil
	}
	return generated
}

func countRoutedEdges(scene Scene) int {
	count := 0
	for _, edge := range scene.Edges {
		if len(edge.Points) > 0 {
			count++
		}
	}
	return count
}
