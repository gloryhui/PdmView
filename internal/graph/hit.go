package graph

import "math"

type HitKind uint8

const (
	HitNone HitKind = iota
	HitNode
	HitEdge
)

// Hit identifies the scene slice offset of the hit element. HitNone uses -1.
type Hit struct {
	Kind  HitKind
	Index int
}

// HitTest hit-tests a viewport-local DIP point. Nodes win first, scanning their
// scene offsets in reverse draw order. Otherwise the nearest actual edge
// segment within edgeToleranceDIP wins; equal edge distances choose the later
// scene offset, matching topmost draw order.
func HitTest(scene Scene, query SpatialQuery, camera *Camera, viewport Point, edgeToleranceDIP float64) Hit {
	none := Hit{Kind: HitNone, Index: -1}
	if camera == nil || !finite(viewport.X) || !finite(viewport.Y) {
		return none
	}
	if query == nil {
		query = NewLinearIndex(scene)
	}
	world := camera.ViewportToWorld(viewport)
	nodeArea := Rect{Min: world, Max: world}
	nodes := query.QueryNodes(nodeArea, make([]int, 0, 8))
	for i := len(nodes) - 1; i >= 0; i-- {
		offset := nodes[i]
		if offset >= 0 && offset < len(scene.Nodes) && pointInRect(world, scene.Nodes[offset].Bounds) {
			return Hit{Kind: HitNode, Index: offset}
		}
	}

	tolerance := edgeToleranceDIP
	if !finite(tolerance) || tolerance < 0 {
		tolerance = 0
	}
	worldTolerance := tolerance / camera.Scale()
	area := Rect{
		Min: Point{X: world.X - worldTolerance, Y: world.Y - worldTolerance},
		Max: Point{X: world.X + worldTolerance, Y: world.Y + worldTolerance},
	}
	bestOffset, bestDistance := -1, math.Inf(1)
	toleranceSq := worldTolerance * worldTolerance
	for _, offset := range query.QueryEdges(area, nil) {
		if offset < 0 || offset >= len(scene.Edges) {
			continue
		}
		distance := polylineDistanceSquared(scene.Edges[offset].Points, world)
		if distance > toleranceSq {
			continue
		}
		if distance < bestDistance || distance == bestDistance && offset > bestOffset {
			bestOffset, bestDistance = offset, distance
		}
	}
	if bestOffset >= 0 {
		return Hit{Kind: HitEdge, Index: bestOffset}
	}
	return none
}

func polylineDistanceSquared(points []Point, p Point) float64 {
	if len(points) == 0 {
		return math.Inf(1)
	}
	if len(points) == 1 {
		return pointDistanceSquared(points[0], p)
	}
	best := math.Inf(1)
	for i := 1; i < len(points); i++ {
		best = min(best, pointSegmentDistanceSquared(p, points[i-1], points[i]))
	}
	return best
}

func pointSegmentDistanceSquared(p, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lengthSq := dx*dx + dy*dy
	if lengthSq == 0 {
		return pointDistanceSquared(p, a)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lengthSq
	t = min(max(t, 0), 1)
	return pointDistanceSquared(p, Point{X: a.X + t*dx, Y: a.Y + t*dy})
}

func pointDistanceSquared(a, b Point) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return dx*dx + dy*dy
}
