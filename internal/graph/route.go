package graph

import (
	"container/heap"
	"sort"
)

const (
	routeClearance  = 8.0
	routePortMargin = 6.0
	// Search is deliberately bounded: too many local obstacles, too many
	// compressed coordinates, or too many expanded grid cells fails this
	// attempt. Failure leaves the edge's Points empty rather than drawing a
	// line through a table.
	routeSearchMaxObstacles = 256
	routeSearchMaxCoords    = 192
	routeSearchMaxExpands   = 30000
)

var routeSearchMargins = [...]float64{256, 1024, 4096}

type routeSide uint8

const (
	routeLeft routeSide = iota
	routeRight
	routeTop
	routeBottom
)

type routeEndpoint struct {
	bounds  Rect
	ordinal int
	total   int
}

type routePlan struct {
	fromPort   Point
	fromEscape Point
	toPort     Point
	toEscape   Point
}

// routeMissingEdges computes deterministic orthogonal display paths only for
// edges whose Points are empty. Existing paths are never modified. Each path
// must leave and enter through outward boundary stubs and pass the final
// full-scene interior check; otherwise that edge keeps empty Points.
func routeMissingEdges(scene *Scene) {
	if scene == nil || len(scene.Nodes) == 0 || len(scene.Edges) == 0 {
		return
	}
	var missing []int
	counts := make([]int, len(scene.Nodes))
	for offset, edge := range scene.Edges {
		if len(edge.Points) > 0 || edge.From < 0 || edge.From >= len(scene.Nodes) ||
			edge.To < 0 || edge.To >= len(scene.Nodes) {
			continue
		}
		missing = append(missing, offset)
		counts[edge.From]++
		counts[edge.To]++
	}
	if len(missing) == 0 {
		return
	}

	index := NewSpatialIndex(Scene{Nodes: scene.Nodes})
	ordinals := make([]int, len(scene.Nodes))
	for _, offset := range missing {
		edge := &scene.Edges[offset]
		from := routeEndpoint{scene.Nodes[edge.From].Bounds, ordinals[edge.From], counts[edge.From]}
		ordinals[edge.From]++
		to := routeEndpoint{scene.Nodes[edge.To].Bounds, ordinals[edge.To], counts[edge.To]}
		ordinals[edge.To]++
		fromSide, toSide := chooseRouteSides(from.bounds, to.bounds, edge.From == edge.To)
		edge.Points = routeBetweenEndpoints(scene, index, from, to, fromSide, toSide)
	}
}

func routeBetweenEndpoints(scene *Scene, index *SpatialIndex, from, to routeEndpoint, preferredFrom, preferredTo routeSide) []Point {
	const sideCount routeSide = 4
	remaining := routeSearchMaxExpands
	for fromOffset := routeSide(0); fromOffset < sideCount; fromOffset++ {
		fromSide := (preferredFrom + fromOffset) % sideCount
		for toOffset := routeSide(0); toOffset < sideCount; toOffset++ {
			toSide := (preferredTo + toOffset) % sideCount
			fromPort, fromEscape := from.port(fromSide, routeStub(from.bounds, to.bounds, fromSide, toSide))
			toPort, toEscape := to.port(toSide, routeStub(to.bounds, from.bounds, toSide, fromSide))
			plan := routePlan{fromPort: fromPort, fromEscape: fromEscape, toPort: toPort, toEscape: toEscape}
			if path := routeEdgePath(scene, index, plan, &remaining); path != nil {
				return path
			}
		}
	}
	return nil
}

func (endpoint routeEndpoint) port(side routeSide, stub float64) (Point, Point) {
	minimum, maximum := endpoint.bounds.Min.Y, endpoint.bounds.Max.Y
	if side == routeTop || side == routeBottom {
		minimum, maximum = endpoint.bounds.Min.X, endpoint.bounds.Max.X
	}
	coordinate := routePortCoordinate(minimum, maximum, endpoint.total, endpoint.ordinal)
	return routePort(endpoint.bounds, side, coordinate, stub)
}

func chooseRouteSides(from, to Rect, self bool) (routeSide, routeSide) {
	if self {
		return routeRight, routeRight
	}
	dx := midpoint(to.Min.X, to.Max.X) - midpoint(from.Min.X, from.Max.X)
	dy := midpoint(to.Min.Y, to.Max.Y) - midpoint(from.Min.Y, from.Max.Y)
	if abs(dx) >= abs(dy) {
		if dx >= 0 {
			return routeRight, routeLeft
		}
		return routeLeft, routeRight
	}
	if dy >= 0 {
		return routeBottom, routeTop
	}
	return routeTop, routeBottom
}

func routePortCoordinate(minimum, maximum float64, total, ordinal int) float64 {
	if total <= 1 || !finite(minimum) || !finite(maximum) || maximum <= minimum {
		return midpoint(minimum, maximum)
	}
	margin := min(routePortMargin, (maximum-minimum)/4)
	span := maximum - minimum - 2*margin
	if span <= 0 {
		return midpoint(minimum, maximum)
	}
	return minimum + margin + float64(ordinal+1)/float64(total+1)*span
}

func routeStub(bounds, other Rect, side, otherSide routeSide) float64 {
	var gap float64
	switch side {
	case routeRight:
		if otherSide == routeLeft {
			gap = other.Min.X - bounds.Max.X
		}
	case routeLeft:
		if otherSide == routeRight {
			gap = bounds.Min.X - other.Max.X
		}
	case routeBottom:
		if otherSide == routeTop {
			gap = other.Min.Y - bounds.Max.Y
		}
	case routeTop:
		if otherSide == routeBottom {
			gap = bounds.Min.Y - other.Max.Y
		}
	}
	if gap > 0 && gap < 2*routeClearance {
		return gap * 0.25
	}
	return routeClearance
}

func routePort(bounds Rect, side routeSide, coordinate, stub float64) (Point, Point) {
	switch side {
	case routeLeft:
		return Point{X: bounds.Min.X, Y: coordinate},
			Point{X: bounds.Min.X - stub, Y: coordinate}
	case routeRight:
		return Point{X: bounds.Max.X, Y: coordinate},
			Point{X: bounds.Max.X + stub, Y: coordinate}
	case routeTop:
		return Point{X: coordinate, Y: bounds.Min.Y},
			Point{X: coordinate, Y: bounds.Min.Y - stub}
	default:
		return Point{X: coordinate, Y: bounds.Max.Y},
			Point{X: coordinate, Y: bounds.Max.Y + stub}
	}
}

func routeEdgePath(scene *Scene, index *SpatialIndex, plan routePlan, remaining *int) []Point {
	if !routeSegmentClearScene(plan.fromPort, plan.fromEscape, scene, index) ||
		!routeSegmentClearScene(plan.toEscape, plan.toPort, scene, index) {
		return nil
	}
	middle := quickRoute(plan.fromEscape, plan.toEscape, scene, index)
	if middle == nil {
		middle = searchRoute(plan.fromEscape, plan.toEscape, scene, index, remaining)
	}
	if middle == nil {
		return nil
	}
	path := make([]Point, 0, len(middle)+2)
	path = append(path, plan.fromPort)
	path = append(path, middle...)
	path = append(path, plan.toPort)
	path = compactRoute(path)
	if len(path) < 2 || !routePathClear(path, scene, index) {
		return nil
	}
	return path
}

func quickRoute(from, to Point, scene *Scene, index *SpatialIndex) []Point {
	if (from.X == to.X || from.Y == to.Y) && routeSegmentClearScene(from, to, scene, index) {
		return compactRoute([]Point{from, to})
	}
	candidates := [2]Point{
		{X: to.X, Y: from.Y},
		{X: from.X, Y: to.Y},
	}
	for _, bend := range candidates {
		path := []Point{from, bend, to}
		if routePathClear(path, scene, index) {
			return compactRoute(path)
		}
	}
	return nil
}

func searchRoute(from, to Point, scene *Scene, index *SpatialIndex, remaining *int) []Point {
	for _, margin := range routeSearchMargins {
		if *remaining <= 0 {
			break
		}
		path := compressedSearch(from, to, scene, index, margin, remaining)
		if path != nil && routePathClear(path, scene, index) {
			return path
		}
	}
	return nil
}

func compressedSearch(from, to Point, scene *Scene, index *SpatialIndex, margin float64, remaining *int) []Point {
	searchBounds := Rect{
		Min: Point{X: min(from.X, to.X) - margin, Y: min(from.Y, to.Y) - margin},
		Max: Point{X: max(from.X, to.X) + margin, Y: max(from.Y, to.Y) + margin},
	}
	offsets := index.QueryNodes(searchBounds, nil)
	if len(offsets) > routeSearchMaxObstacles {
		return nil
	}
	obstacles := make([]Rect, 0, len(offsets))
	for _, offset := range offsets {
		if offset >= 0 && offset < len(scene.Nodes) {
			obstacles = append(obstacles, scene.Nodes[offset].Bounds)
		}
	}

	xs := []float64{from.X, to.X, searchBounds.Min.X, searchBounds.Max.X}
	ys := []float64{from.Y, to.Y, searchBounds.Min.Y, searchBounds.Max.Y}
	for _, obstacle := range obstacles {
		addRouteCoordinate(&xs, obstacle.Min.X-routeClearance, searchBounds.Min.X, searchBounds.Max.X)
		addRouteCoordinate(&xs, obstacle.Min.X, searchBounds.Min.X, searchBounds.Max.X)
		addRouteCoordinate(&xs, obstacle.Max.X, searchBounds.Min.X, searchBounds.Max.X)
		addRouteCoordinate(&xs, obstacle.Max.X+routeClearance, searchBounds.Min.X, searchBounds.Max.X)
		addRouteCoordinate(&ys, obstacle.Min.Y-routeClearance, searchBounds.Min.Y, searchBounds.Max.Y)
		addRouteCoordinate(&ys, obstacle.Min.Y, searchBounds.Min.Y, searchBounds.Max.Y)
		addRouteCoordinate(&ys, obstacle.Max.Y, searchBounds.Min.Y, searchBounds.Max.Y)
		addRouteCoordinate(&ys, obstacle.Max.Y+routeClearance, searchBounds.Min.Y, searchBounds.Max.Y)
	}
	xs = sortedUnique(xs)
	ys = sortedUnique(ys)
	if len(xs) > routeSearchMaxCoords || len(ys) > routeSearchMaxCoords {
		return nil
	}

	startX := sort.SearchFloat64s(xs, from.X)
	startY := sort.SearchFloat64s(ys, from.Y)
	endX := sort.SearchFloat64s(xs, to.X)
	endY := sort.SearchFloat64s(ys, to.Y)
	if startX >= len(xs) || xs[startX] != from.X ||
		startY >= len(ys) || ys[startY] != from.Y ||
		endX >= len(xs) || xs[endX] != to.X ||
		endY >= len(ys) || ys[endY] != to.Y {
		return nil
	}
	start := routeGridKey{x: startX, y: startY}
	end := routeGridKey{x: endX, y: endY}
	if routePointBlockedScene(from, scene, index) || routePointBlockedScene(to, scene, index) {
		return nil
	}
	if start == end {
		return []Point{from}
	}

	open := &routePriorityQueue{}
	heap.Init(open)
	gScore := map[routeGridKey]float64{start: 0}
	parent := make(map[routeGridKey]routeGridKey)
	closed := make(map[routeGridKey]bool)
	heap.Push(open, routeQueueItem{
		key: start,
		g:   0,
		f:   abs(from.X-to.X) + abs(from.Y-to.Y),
	})
	for open.Len() > 0 && *remaining > 0 {
		item := heap.Pop(open).(routeQueueItem)
		if best, ok := gScore[item.key]; !ok || item.g != best || closed[item.key] {
			continue
		}
		if item.key == end {
			return reconstructRoute(pathPoints(parent, start, end), xs, ys)
		}
		closed[item.key] = true
		*remaining -= 1

		current := Point{X: xs[item.key.x], Y: ys[item.key.y]}
		for _, neighbor := range [4]routeGridKey{
			{x: item.key.x - 1, y: item.key.y},
			{x: item.key.x + 1, y: item.key.y},
			{x: item.key.x, y: item.key.y - 1},
			{x: item.key.x, y: item.key.y + 1},
		} {
			if neighbor.x < 0 || neighbor.x >= len(xs) || neighbor.y < 0 || neighbor.y >= len(ys) {
				continue
			}
			next := Point{X: xs[neighbor.x], Y: ys[neighbor.y]}
			if !routeSegmentClearScene(current, next, scene, index) {
				continue
			}
			cost := abs(next.X-current.X) + abs(next.Y-current.Y)
			nextG := item.g + cost
			if best, exists := gScore[neighbor]; exists && nextG >= best {
				continue
			}
			gScore[neighbor] = nextG
			parent[neighbor] = item.key
			heap.Push(open, routeQueueItem{
				key: neighbor,
				g:   nextG,
				f:   nextG + abs(next.X-to.X) + abs(next.Y-to.Y),
			})
		}
	}
	return nil
}

func pathPoints(parent map[routeGridKey]routeGridKey, start, end routeGridKey) []routeGridKey {
	path := []routeGridKey{end}
	for path[len(path)-1] != start {
		previous, ok := parent[path[len(path)-1]]
		if !ok {
			return nil
		}
		path = append(path, previous)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func reconstructRoute(keys []routeGridKey, xs, ys []float64) []Point {
	if len(keys) == 0 {
		return nil
	}
	points := make([]Point, len(keys))
	for offset, key := range keys {
		points[offset] = Point{X: xs[key.x], Y: ys[key.y]}
	}
	return compactRoute(points)
}

func addRouteCoordinate(values *[]float64, value, minimum, maximum float64) {
	if finite(value) && value >= minimum && value <= maximum {
		*values = append(*values, value)
	}
}

func sortedUnique(values []float64) []float64 {
	sort.Float64s(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func routePathClear(points []Point, scene *Scene, index *SpatialIndex) bool {
	for offset := 1; offset < len(points); offset++ {
		if !routeSegmentClearScene(points[offset-1], points[offset], scene, index) {
			return false
		}
	}
	return true
}

func routeSegmentClearScene(a, b Point, scene *Scene, index *SpatialIndex) bool {
	if a == b {
		return true
	}
	if a.X != b.X && a.Y != b.Y {
		return false
	}
	area := routeSegmentBounds(a, b)
	var offsets [8]int
	for _, offset := range index.QueryNodes(area, offsets[:0]) {
		if offset >= 0 && offset < len(scene.Nodes) &&
			routeSegmentCrossesInterior(a, b, scene.Nodes[offset].Bounds) {
			return false
		}
	}
	return true
}

func routeSegmentBounds(a, b Point) Rect {
	return Rect{
		Min: Point{X: min(a.X, b.X), Y: min(a.Y, b.Y)},
		Max: Point{X: max(a.X, b.X), Y: max(a.Y, b.Y)},
	}
}

func routeSegmentCrossesInterior(a, b Point, rect Rect) bool {
	// Table boundaries are shared channels: only the strict rectangle interior
	// is invalid. Axis-aligned segments tangent to an edge remain valid.
	if a.Y == b.Y {
		if a.Y <= rect.Min.Y || a.Y >= rect.Max.Y {
			return false
		}
		left, right := min(a.X, b.X), max(a.X, b.X)
		return max(left, rect.Min.X) < min(right, rect.Max.X)
	}
	if a.X == b.X {
		if a.X <= rect.Min.X || a.X >= rect.Max.X {
			return false
		}
		top, bottom := min(a.Y, b.Y), max(a.Y, b.Y)
		return max(top, rect.Min.Y) < min(bottom, rect.Max.Y)
	}
	return false
}

func routePointBlockedScene(point Point, scene *Scene, index *SpatialIndex) bool {
	for _, offset := range index.QueryPoint(point, nil) {
		if offset >= 0 && offset < len(scene.Nodes) {
			bounds := scene.Nodes[offset].Bounds
			if routePointStrictlyInside(point, bounds) {
				return true
			}
		}
	}
	return false
}

func routePointStrictlyInside(point Point, bounds Rect) bool {
	return point.X > bounds.Min.X && point.X < bounds.Max.X &&
		point.Y > bounds.Min.Y && point.Y < bounds.Max.Y
}

func compactRoute(points []Point) []Point {
	if len(points) == 0 {
		return nil
	}
	result := make([]Point, 0, len(points))
	for _, point := range points {
		if len(result) > 0 && result[len(result)-1] == point {
			continue
		}
		if len(result) >= 2 && routeCollinearBetween(result[len(result)-2], result[len(result)-1], point) {
			result[len(result)-1] = point
			continue
		}
		result = append(result, point)
	}
	return result
}

func routeCollinearBetween(a, b, c Point) bool {
	if a.X == b.X && b.X == c.X {
		return b.Y >= min(a.Y, c.Y) && b.Y <= max(a.Y, c.Y)
	}
	if a.Y == b.Y && b.Y == c.Y {
		return b.X >= min(a.X, c.X) && b.X <= max(a.X, c.X)
	}
	return false
}

type routeGridKey struct {
	x int
	y int
}

type routeQueueItem struct {
	key routeGridKey
	g   float64
	f   float64
}

type routePriorityQueue []routeQueueItem

func (q routePriorityQueue) Len() int { return len(q) }

func (q routePriorityQueue) Less(i, j int) bool {
	a, b := q[i], q[j]
	if a.f != b.f {
		return a.f < b.f
	}
	if a.g != b.g {
		return a.g > b.g
	}
	if a.key.x != b.key.x {
		return a.key.x < b.key.x
	}
	return a.key.y < b.key.y
}

func (q routePriorityQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *routePriorityQueue) Push(value any) {
	*q = append(*q, value.(routeQueueItem))
}

func (q *routePriorityQueue) Pop() any {
	old := *q
	last := old[len(old)-1]
	*q = old[:len(old)-1]
	return last
}
