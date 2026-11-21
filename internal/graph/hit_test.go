package graph

import (
	"math"
	"testing"
)

func TestHitTestNodesFirstInReverseDrawOrder(t *testing.T) {
	scene := manualScene(
		[]Node{
			{ID: 91, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 100, Y: 100}}},
			{ID: 17, Bounds: Rect{Min: Point{X: 50, Y: 0}, Max: Point{X: 150, Y: 100}}},
		},
		[]Edge{{ID: 44, Points: []Point{{X: 75, Y: 0}, {X: 75, Y: 100}}}},
	)
	cam := NewCamera(Rect{Min: Point{}, Max: Point{X: 300, Y: 200}})
	for _, query := range []SpatialQuery{NewLinearIndex(scene), NewSpatialIndex(scene)} {
		if hit := HitTest(scene, query, cam, Point{X: 75, Y: 50}, 10); hit != (Hit{Kind: HitNode, Index: 1}) {
			t.Fatalf("%T overlapping node hit = %+v, want offset 1 over the edge", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 10, Y: 10}, 10); hit != (Hit{Kind: HitNode, Index: 0}) {
			t.Fatalf("%T first node hit = %+v", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 100, Y: 100}, 10); hit != (Hit{Kind: HitNode, Index: 1}) {
			t.Fatalf("%T inclusive boundary hit = %+v", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 299, Y: 199}, 10); hit != (Hit{Kind: HitNone, Index: -1}) {
			t.Fatalf("%T empty hit = %+v", query, hit)
		}
	}
}

func TestHitTestNearestEdgeToleranceAndDeterministicTie(t *testing.T) {
	scene := manualScene(nil, []Edge{
		{ID: 1000, Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}},
		{ID: 2000, Points: []Point{{X: 0, Y: 2}, {X: 100, Y: 2}}},
		{ID: 3000, Points: []Point{{X: 0, Y: 20}, {X: 100, Y: 20}}},
	})
	cam := NewCamera(Rect{Min: Point{}, Max: Point{X: 200, Y: 100}})
	for _, query := range []SpatialQuery{NewLinearIndex(scene), NewSpatialIndex(scene)} {
		if hit := HitTest(scene, query, cam, Point{X: 50, Y: 1}, 3); hit != (Hit{Kind: HitEdge, Index: 1}) {
			t.Fatalf("%T equal-distance tie = %+v, want later offset 1", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 50, Y: 0.2}, 3); hit != (Hit{Kind: HitEdge, Index: 0}) {
			t.Fatalf("%T nearest edge = %+v", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 50, Y: 10}, 3); hit != (Hit{Kind: HitNone, Index: -1}) {
			t.Fatalf("%T outside edge tolerance = %+v", query, hit)
		}
		if hit := HitTest(scene, query, cam, Point{X: 50, Y: 0}, 0); hit != (Hit{Kind: HitEdge, Index: 0}) {
			t.Fatalf("%T exact zero-tolerance edge = %+v", query, hit)
		}
	}
}

func TestHitTestActualMultiSegmentDistanceAndDegenerateEdge(t *testing.T) {
	scene := manualScene(nil, []Edge{
		{ID: 5, Points: []Point{{X: -5, Y: 0}, {X: -5, Y: 30}, {X: 35, Y: 30}, {X: 35, Y: 0}}},
		{ID: 9, Points: []Point{{X: -10, Y: -10}, {X: 5, Y: 5}, {X: 25, Y: -10}}},
		{ID: 12, Points: []Point{{X: 50, Y: 50}, {X: 50, Y: 50}}},
	})
	cam := NewCamera(Rect{Min: Point{}, Max: Point{X: 200, Y: 100}})
	index := NewSpatialIndex(scene)
	if hit := HitTest(scene, index, cam, Point{X: 10, Y: 10}, 0.5); hit != (Hit{Kind: HitNone, Index: -1}) {
		t.Fatalf("AABB false positive returned %+v", hit)
	}
	if hit := HitTest(scene, index, cam, Point{X: 0, Y: 0}, 0.1); hit != (Hit{Kind: HitEdge, Index: 1}) {
		t.Fatalf("multi-segment crossing hit = %+v", hit)
	}
	if hit := HitTest(scene, index, cam, Point{X: 50.5, Y: 50}, 0.6); hit != (Hit{Kind: HitEdge, Index: 2}) {
		t.Fatalf("degenerate edge within tolerance = %+v", hit)
	}
	if hit := HitTest(scene, index, cam, Point{X: 50.5, Y: 50}, 0.4); hit != (Hit{Kind: HitNone, Index: -1}) {
		t.Fatalf("degenerate edge outside tolerance = %+v", hit)
	}
}

func TestHitTestToleranceUsesViewportDIPAcrossZoom(t *testing.T) {
	scene := manualScene(nil, []Edge{
		{ID: 1, Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}},
	})
	cam := NewCamera(Rect{Min: Point{}, Max: Point{X: 200, Y: 100}})
	cam.ZoomAt(Point{}, 2)
	query := NewSpatialIndex(scene)
	point := Point{X: 50, Y: 1}
	if hit := HitTest(scene, query, cam, point, 0.8); hit.Kind != HitNone {
		t.Fatalf("0.8 DIP tolerance at 2x hit %+v", hit)
	}
	if hit := HitTest(scene, query, cam, point, 1.2); hit != (Hit{Kind: HitEdge, Index: 0}) {
		t.Fatalf("1.2 DIP tolerance at 2x = %+v", hit)
	}
	if hit := HitTest(scene, query, cam, point, math.NaN()); hit.Kind != HitNone {
		t.Fatalf("NaN tolerance hit %+v", hit)
	}
}
