package graph

import (
	"sort"
	"sync/atomic"
)

// SpatialQuery finds scene slice offsets whose geometry intersects area.
// Results are appended to dst and sorted ascending per call; existing dst
// values are preserved verbatim.
type SpatialQuery interface {
	QueryNodes(area Rect, dst []int) []int
	QueryEdges(area Rect, dst []int) []int
}

// LinearIndex is the reference SpatialQuery implementation.
type LinearIndex struct {
	nodes []Node
	edges []Edge
}

func NewLinearIndex(scene Scene) *LinearIndex {
	return &LinearIndex{nodes: cloneNodes(scene.Nodes), edges: cloneEdges(scene.Edges)}
}

func (l *LinearIndex) QueryNodes(area Rect, dst []int) []int {
	area = orderedRect(area)
	start := len(dst)
	for offset, node := range l.nodes {
		if rectsIntersectInclusive(node.Bounds, area) {
			dst = append(dst, offset)
		}
	}
	sort.Ints(dst[start:])
	return dst
}

func (l *LinearIndex) QueryEdges(area Rect, dst []int) []int {
	area = orderedRect(area)
	start := len(dst)
	for offset, edge := range l.edges {
		if rectsIntersectInclusive(edge.Bounds(), area) && polylineIntersectsRect(edge.Points, area) {
			dst = append(dst, offset)
		}
	}
	sort.Ints(dst[start:])
	return dst
}

// SpatialIndex atomically publishes an immutable BVH snapshot over node bounds
// and edge sampled-polyline AABBs. Edge queries then verify the actual
// polyline.
type SpatialIndex struct {
	snapshot atomic.Pointer[spatialSnapshot]
}

type spatialSnapshot struct {
	nodes         []Node
	edges         []Edge
	node          *aabbTree
	edge          *aabbTree
	allNodeBounds Rect
	allEdgeBounds Rect
	hasNodes      bool
	hasEdges      bool
}

func NewSpatialIndex(scene Scene) *SpatialIndex {
	index := &SpatialIndex{}
	index.ReplaceScene(scene)
	return index
}

// ReplaceScene clones the scene, builds both trees, then publishes the complete
// immutable snapshot in one atomic store.
func (s *SpatialIndex) ReplaceScene(scene Scene) {
	if s == nil {
		return
	}
	s.snapshot.Store(buildSpatialSnapshot(scene))
}

func buildSpatialSnapshot(scene Scene) *spatialSnapshot {
	snapshot := &spatialSnapshot{
		nodes: cloneNodes(scene.Nodes),
		edges: cloneEdges(scene.Edges),
	}
	nodeItems := make([]indexedRect, 0, len(snapshot.nodes))
	for offset, node := range snapshot.nodes {
		nodeItems = append(nodeItems, indexedRect{offset: offset, bounds: node.Bounds})
		if !snapshot.hasNodes {
			snapshot.allNodeBounds, snapshot.hasNodes = node.Bounds, true
		} else {
			snapshot.allNodeBounds = snapshot.allNodeBounds.Union(node.Bounds)
		}
	}
	edgeItems := make([]indexedRect, 0, len(snapshot.edges))
	for offset, edge := range snapshot.edges {
		if len(edge.Points) > 0 {
			bounds := edge.Bounds()
			if !snapshot.hasEdges {
				snapshot.allEdgeBounds, snapshot.hasEdges = bounds, true
			} else {
				snapshot.allEdgeBounds = snapshot.allEdgeBounds.Union(bounds)
			}
		}
		switch len(edge.Points) {
		case 0:
		case 1:
			p := edge.Points[0]
			edgeItems = append(edgeItems, indexedRect{offset: offset, bounds: Rect{Min: p, Max: p}})
		default:
			for i := 1; i < len(edge.Points); i++ {
				a, b := edge.Points[i-1], edge.Points[i]
				edgeItems = append(edgeItems, indexedRect{
					offset: offset,
					bounds: Rect{
						Min: Point{X: min(a.X, b.X), Y: min(a.Y, b.Y)},
						Max: Point{X: max(a.X, b.X), Y: max(a.Y, b.Y)},
					},
				})
			}
		}
	}
	snapshot.node = buildAABBTree(nodeItems)
	snapshot.edge = buildAABBTree(edgeItems)
	return snapshot
}

func (s *SpatialIndex) QueryNodes(area Rect, dst []int) []int {
	return s.currentSnapshot().queryNodes(orderedRect(area), dst)
}

func (s *SpatialIndex) QueryEdges(area Rect, dst []int) []int {
	return s.currentSnapshot().queryEdges(orderedRect(area), dst)
}

// QueryPoint finds nodes whose inclusive bounds contain point. Non-finite
// points leave dst unchanged.
func (s *SpatialIndex) QueryPoint(point Point, dst []int) []int {
	if !finite(point.X) || !finite(point.Y) {
		return dst
	}
	area := Rect{Min: point, Max: point}
	return s.currentSnapshot().queryNodes(area, dst)
}

func (s *SpatialIndex) currentSnapshot() *spatialSnapshot {
	if s == nil {
		return &emptySpatialSnapshot
	}
	if snapshot := s.snapshot.Load(); snapshot != nil {
		return snapshot
	}
	return &emptySpatialSnapshot
}

var emptySpatialSnapshot spatialSnapshot

func (s *spatialSnapshot) queryNodes(area Rect, dst []int) []int {
	if s.hasNodes && rectContainsInclusive(area, s.allNodeBounds) {
		for offset := range s.nodes {
			dst = append(dst, offset)
		}
		return dst
	}
	start := len(dst)
	dst = s.node.query(area, dst)
	candidates := dst[start:]
	filtered := candidates[:0]
	for _, offset := range candidates {
		if offset >= 0 && offset < len(s.nodes) && rectsIntersectInclusive(s.nodes[offset].Bounds, area) {
			filtered = append(filtered, offset)
		}
	}
	dst = dst[:start+len(filtered)]
	sort.Ints(dst[start:])
	return dst
}

func (s *spatialSnapshot) queryEdges(area Rect, dst []int) []int {
	if s.hasEdges && rectContainsInclusive(area, s.allEdgeBounds) {
		for offset, edge := range s.edges {
			if len(edge.Points) > 0 {
				dst = append(dst, offset)
			}
		}
		return dst
	}
	start := len(dst)
	dst = s.edge.query(area, dst)
	candidates := dst[start:]
	sort.Ints(candidates)
	unique := candidates[:0]
	for _, offset := range candidates {
		if len(unique) == 0 || unique[len(unique)-1] != offset {
			unique = append(unique, offset)
		}
	}
	candidates = unique
	filtered := candidates[:0]
	for _, offset := range candidates {
		if offset >= 0 && offset < len(s.edges) &&
			polylineIntersectsRect(s.edges[offset].Points, area) {
			filtered = append(filtered, offset)
		}
	}
	dst = dst[:start+len(filtered)]
	sort.Ints(dst[start:])
	return dst
}

type indexedRect struct {
	offset int
	bounds Rect
}

type aabbNode struct {
	bounds Rect
	left   *aabbNode
	right  *aabbNode
	items  []indexedRect
}

type aabbTree struct {
	root *aabbNode
}

func buildAABBTree(items []indexedRect) *aabbTree {
	if len(items) == 0 {
		return &aabbTree{}
	}
	items = append([]indexedRect(nil), items...)
	return &aabbTree{root: buildAABBNode(items)}
}

func buildAABBNode(items []indexedRect) *aabbNode {
	node := &aabbNode{bounds: items[0].bounds}
	for _, item := range items[1:] {
		node.bounds = node.bounds.Union(item.bounds)
	}
	if len(items) <= 8 {
		node.items = append([]indexedRect(nil), items...)
		return node
	}

	centerBounds := rectCenterBounds(items)
	if centerBounds.Width() >= centerBounds.Height() {
		sort.Slice(items, func(i, j int) bool {
			ci, cj := rectMiddle(items[i].bounds), rectMiddle(items[j].bounds)
			if ci.X != cj.X {
				return ci.X < cj.X
			}
			return items[i].offset < items[j].offset
		})
	} else {
		sort.Slice(items, func(i, j int) bool {
			ci, cj := rectMiddle(items[i].bounds), rectMiddle(items[j].bounds)
			if ci.Y != cj.Y {
				return ci.Y < cj.Y
			}
			return items[i].offset < items[j].offset
		})
	}
	mid := len(items) / 2
	node.left = buildAABBNode(items[:mid])
	node.right = buildAABBNode(items[mid:])
	return node
}

func rectCenterBounds(items []indexedRect) Rect {
	first := rectMiddle(items[0].bounds)
	bounds := Rect{Min: first, Max: first}
	for _, item := range items[1:] {
		p := rectMiddle(item.bounds)
		bounds.Min.X, bounds.Min.Y = min(bounds.Min.X, p.X), min(bounds.Min.Y, p.Y)
		bounds.Max.X, bounds.Max.Y = max(bounds.Max.X, p.X), max(bounds.Max.Y, p.Y)
	}
	return bounds
}

func (t *aabbTree) query(area Rect, dst []int) []int {
	if t == nil || t.root == nil {
		return dst
	}
	var buffer [64]*aabbNode
	stack := buffer[:1]
	stack[0] = t.root
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !rectsIntersectInclusive(node.bounds, area) {
			continue
		}
		if node.left == nil {
			for _, item := range node.items {
				if rectsIntersectInclusive(item.bounds, area) {
					dst = append(dst, item.offset)
				}
			}
			continue
		}
		stack = append(stack, node.left, node.right)
	}
	return dst
}

func rectsIntersectInclusive(a, b Rect) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X &&
		a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y
}

func rectContainsInclusive(outer, inner Rect) bool {
	return outer.Min.X <= inner.Min.X && outer.Min.Y <= inner.Min.Y &&
		outer.Max.X >= inner.Max.X && outer.Max.Y >= inner.Max.Y
}

func polylineIntersectsRect(points []Point, rect Rect) bool {
	switch len(points) {
	case 0:
		return false
	case 1:
		return pointInRect(points[0], rect)
	}
	for i := 1; i < len(points); i++ {
		if segmentIntersectsRect(points[i-1], points[i], rect) {
			return true
		}
	}
	return false
}

func segmentIntersectsRect(a, b Point, r Rect) bool {
	if pointInRect(a, r) || pointInRect(b, r) {
		return true
	}
	corners := [4]Point{
		{X: r.Min.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Max.Y},
		{X: r.Min.X, Y: r.Max.Y},
	}
	for i := range corners {
		if segmentsIntersectInclusive(a, b, corners[i], corners[(i+1)%len(corners)]) {
			return true
		}
	}
	return false
}

func segmentsIntersectInclusive(a, b, c, d Point) bool {
	if a == b {
		return pointOnSegment(a, c, d)
	}
	if c == d {
		return pointOnSegment(c, a, b)
	}
	d1 := cross(c, d, a)
	d2 := cross(c, d, b)
	d3 := cross(a, b, c)
	d4 := cross(a, b, d)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	return d1 == 0 && pointOnSegment(a, c, d) ||
		d2 == 0 && pointOnSegment(b, c, d) ||
		d3 == 0 && pointOnSegment(c, a, b) ||
		d4 == 0 && pointOnSegment(d, a, b)
}

func pointOnSegment(p, a, b Point) bool {
	return cross(a, b, p) == 0 &&
		p.X >= min(a.X, b.X) && p.X <= max(a.X, b.X) &&
		p.Y >= min(a.Y, b.Y) && p.Y <= max(a.Y, b.Y)
}

func pointInRect(p Point, r Rect) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X &&
		p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

func cross(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func rectMiddle(r Rect) Point {
	return Point{X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
}

func cloneNodes(nodes []Node) []Node {
	cloned := make([]Node, len(nodes))
	copy(cloned, nodes)
	for i := range cloned {
		cloned[i].Fields = append([]Field(nil), nodes[i].Fields...)
	}
	return cloned
}

func cloneEdges(edges []Edge) []Edge {
	cloned := make([]Edge, len(edges))
	copy(cloned, edges)
	for i := range cloned {
		cloned[i].Points = append([]Point(nil), edges[i].Points...)
	}
	return cloned
}
