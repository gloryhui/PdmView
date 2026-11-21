package graph

import (
	"math"
	"reflect"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func TestQueryOffsetsPrefixInclusiveBoundsAndOrdering(t *testing.T) {
	scene := manualScene(
		[]Node{
			{ID: 900, Bounds: Rect{Min: Point{X: 10, Y: 0}, Max: Point{X: 20, Y: 10}}},
			{ID: 100, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 10, Y: 10}}},
			{ID: 500, Bounds: Rect{Min: Point{X: 30, Y: 0}, Max: Point{X: 40, Y: 10}}},
		},
		[]Edge{
			{ID: 800, From: 100, To: 500, Points: []Point{{X: 0, Y: 5}, {X: 40, Y: 5}}},
			{ID: 300, From: 100, To: 900, Points: []Point{{X: 10, Y: 10}, {X: 10, Y: 20}}},
		},
	)
	for _, query := range []SpatialQuery{NewLinearIndex(scene), NewSpatialIndex(scene)} {
		prefix := []int{77, 88}
		nodes := query.QueryNodes(Rect{Min: Point{X: 10, Y: 5}, Max: Point{X: 10, Y: 5}}, append([]int(nil), prefix...))
		if !slices.Equal(nodes, []int{77, 88, 0, 1}) {
			t.Fatalf("point node query = %v, want prefix then offsets 0,1", nodes)
		}
		edges := query.QueryEdges(Rect{Min: Point{X: 10, Y: 10}, Max: Point{X: 10, Y: 10}}, append([]int(nil), prefix...))
		if !slices.Equal(edges, []int{77, 88, 1}) {
			t.Fatalf("point edge query = %v, want prefix then offset 1", edges)
		}
		if !slices.Equal(prefix, []int{77, 88}) {
			t.Fatalf("query changed caller prefix through shared storage: %v", prefix)
		}
	}
}

func TestPrecisePolylineRectQueries(t *testing.T) {
	scene := manualScene(nil, []Edge{
		{ID: 101, Points: []Point{{X: -10, Y: 0}, {X: 5, Y: 15}, {X: 25, Y: 0}}},
		{ID: 202, Points: []Point{{X: -5, Y: 0}, {X: -5, Y: 30}, {X: 35, Y: 30}, {X: 35, Y: 0}}},
		{ID: 303, Points: []Point{{X: -10, Y: 10}, {X: 10, Y: 10}}},
		{ID: 404, Points: []Point{{X: 15, Y: 15}, {X: 15, Y: 15}}},
		{ID: 505, Points: []Point{{X: 50, Y: 50}, {X: 50, Y: 50}}},
	})
	area := Rect{Min: Point{X: 10, Y: 10}, Max: Point{X: 20, Y: 20}}
	want := []int{0, 2, 3}
	for _, query := range []SpatialQuery{NewLinearIndex(scene), NewSpatialIndex(scene)} {
		if got := query.QueryEdges(area, nil); !slices.Equal(got, want) {
			t.Fatalf("%T precise edges = %v, want %v", query, got, want)
		}
	}

	thin := Rect{Min: Point{X: 5, Y: 10}, Max: Point{X: 20, Y: 10}}
	for _, query := range []SpatialQuery{NewLinearIndex(scene), NewSpatialIndex(scene)} {
		if got := query.QueryEdges(thin, nil); !slices.Equal(got, []int{0, 2}) {
			t.Fatalf("%T tangent edges = %v, want 0,2", query, got)
		}
	}
}

func TestIndexedAndLinearAgreeAcrossScalesAndManyRectangles(t *testing.T) {
	for _, count := range []int{50, 150, 300, 500} {
		t.Run(densityCountName(DensityDense, count), func(t *testing.T) {
			scene, err := GenerateDefault(count, DensityDense)
			if err != nil {
				t.Fatal(err)
			}
			linear := NewLinearIndex(scene)
			index := NewSpatialIndex(scene)
			for i, area := range queryRectangles(scene) {
				prefix := []int{-10, i}
				gotNodes := index.QueryNodes(area, append([]int(nil), prefix...))
				wantNodes := linear.QueryNodes(area, append([]int(nil), prefix...))
				if !reflect.DeepEqual(gotNodes, wantNodes) {
					t.Fatalf("rect %d node mismatch: index=%v linear=%v area=%+v", i, gotNodes, wantNodes, area)
				}
				gotEdges := index.QueryEdges(area, append([]int(nil), prefix...))
				wantEdges := linear.QueryEdges(area, append([]int(nil), prefix...))
				if !reflect.DeepEqual(gotEdges, wantEdges) {
					t.Fatalf("rect %d edge mismatch: index=%v linear=%v area=%+v", i, gotEdges, wantEdges, area)
				}
			}
		})
	}
}

func TestSpatialIndexReplaceSceneUpdatesBoundsOffsetsAndEdges(t *testing.T) {
	firstScene := manualScene(
		[]Node{
			{ID: 10, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 10, Y: 10}}},
			{ID: 20, Bounds: Rect{Min: Point{X: 100, Y: 100}, Max: Point{X: 110, Y: 110}}},
		},
		[]Edge{
			{ID: 30, Points: []Point{{X: 0, Y: 5}, {X: 10, Y: 5}}},
			{ID: 40, Points: []Point{{X: 100, Y: 105}, {X: 110, Y: 105}}},
		},
	)
	index := NewSpatialIndex(firstScene)
	if got := index.QueryPoint(Point{X: 5, Y: 5}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("first scene point = %v", got)
	}
	if got := index.QueryPoint(Point{X: 105, Y: 105}, nil); !slices.Equal(got, []int{1}) {
		t.Fatalf("first scene second point = %v", got)
	}

	secondScene := manualScene(
		[]Node{
			{ID: 50, Bounds: Rect{Min: Point{X: 300, Y: 300}, Max: Point{X: 310, Y: 310}}},
			{ID: 10, Bounds: Rect{Min: Point{X: 200, Y: 200}, Max: Point{X: 210, Y: 210}}},
		},
		[]Edge{
			{ID: 60, Points: []Point{{X: 200, Y: 205}, {X: 210, Y: 205}}},
		},
	)
	index.ReplaceScene(secondScene)

	if got := index.QueryPoint(Point{X: 5, Y: 5}, nil); len(got) != 0 {
		t.Fatalf("old node bound still indexed: %v", got)
	}
	if got := index.QueryPoint(Point{X: 105, Y: 105}, nil); len(got) != 0 {
		t.Fatalf("deleted node still indexed: %v", got)
	}
	if got := index.QueryPoint(Point{X: 205, Y: 205}, nil); !slices.Equal(got, []int{1}) {
		t.Fatalf("moved/reordered node offset = %v, want 1", got)
	}
	if got := index.QueryPoint(Point{X: 305, Y: 305}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("new node offset = %v, want 0", got)
	}
	oldEdgeArea := Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 120, Y: 120}}
	if got := index.QueryEdges(oldEdgeArea, nil); len(got) != 0 {
		t.Fatalf("old edge bounds still indexed: %v", got)
	}
	if got := index.QueryEdges(Rect{Min: Point{X: 205, Y: 205}, Max: Point{X: 205, Y: 205}}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("replacement edge offset = %v", got)
	}
}

func TestSpatialIndexQueryPointInclusivePrefixOrderAndNonFinite(t *testing.T) {
	index := NewSpatialIndex(manualScene([]Node{
		{ID: 1, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 10, Y: 10}}},
		{ID: 2, Bounds: Rect{Min: Point{X: 10, Y: 5}, Max: Point{X: 20, Y: 15}}},
		{ID: 3, Bounds: Rect{Min: Point{X: 30, Y: 30}, Max: Point{X: 40, Y: 40}}},
	}, nil))

	prefix := []int{99, 77}
	got := index.QueryPoint(Point{X: 10, Y: 10}, append([]int(nil), prefix...))
	if !slices.Equal(got, []int{99, 77, 0, 1}) {
		t.Fatalf("inclusive point prefixes/order = %v", got)
	}
	if !slices.Equal(prefix, []int{99, 77}) {
		t.Fatalf("caller prefix changed: %v", prefix)
	}
	if got := index.QueryPoint(Point{X: 25, Y: 25}, append([]int(nil), prefix...)); !slices.Equal(got, prefix) {
		t.Fatalf("empty point result = %v", got)
	}

	for _, point := range []Point{
		{X: math.NaN(), Y: 1},
		{X: 1, Y: math.Inf(1)},
		{X: math.Inf(-1), Y: 1},
	} {
		dst := append([]int(nil), prefix...)
		result := index.QueryPoint(point, dst)
		if len(result) != len(dst) || &result[0] != &dst[0] || !slices.Equal(result, prefix) {
			t.Fatalf("non-finite point %+v changed dst: %v -> %v", point, dst, result)
		}
	}
}

func TestSpatialIndexZeroValueEmptyAndCallerIsolation(t *testing.T) {
	prefix := []int{4, 2}
	var zero SpatialIndex
	if got := zero.QueryNodes(sceneWideRect(), append([]int(nil), prefix...)); !slices.Equal(got, prefix) {
		t.Fatalf("zero index nodes = %v", got)
	}
	if got := zero.QueryEdges(sceneWideRect(), append([]int(nil), prefix...)); !slices.Equal(got, prefix) {
		t.Fatalf("zero index edges = %v", got)
	}
	if got := zero.QueryPoint(Point{}, append([]int(nil), prefix...)); !slices.Equal(got, prefix) {
		t.Fatalf("zero index point = %v", got)
	}
	zero.ReplaceScene(Scene{})
	if got := zero.QueryPoint(Point{X: 1, Y: 1}, nil); len(got) != 0 {
		t.Fatalf("empty replacement point = %v", got)
	}
	var nilIndex *SpatialIndex
	if got := nilIndex.QueryPoint(Point{}, nil); len(got) != 0 {
		t.Fatalf("nil index point = %v", got)
	}

	scene := manualScene(
		[]Node{{ID: 1, Bounds: Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 10, Y: 10}}}},
		[]Edge{{ID: 2, Points: []Point{{X: 0, Y: 5}, {X: 10, Y: 5}}}},
	)
	index := NewSpatialIndex(scene)
	scene.Nodes[0].Bounds = Rect{Min: Point{X: 100, Y: 100}, Max: Point{X: 110, Y: 110}}
	scene.Edges[0].Points[0] = Point{X: 100, Y: 105}
	scene.Edges[0].Points[1] = Point{X: 110, Y: 105}
	if got := index.QueryPoint(Point{X: 5, Y: 5}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("index aliased caller nodes: %v", got)
	}
	if got := index.QueryEdges(Rect{Min: Point{X: 5, Y: 5}, Max: Point{X: 5, Y: 5}}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("index aliased caller edges: %v", got)
	}
	if got := index.QueryPoint(Point{X: 105, Y: 105}, nil); len(got) != 0 {
		t.Fatalf("caller mutation leaked into index: %v", got)
	}
	index.ReplaceScene(scene)
	if got := index.QueryPoint(Point{X: 105, Y: 105}, nil); !slices.Equal(got, []int{0}) {
		t.Fatalf("explicit replacement did not publish caller scene: %v", got)
	}
}

func TestSpatialIndexConcurrentQueriesSeeCompleteSnapshots(t *testing.T) {
	sceneA := snapshotScene(7, 3)
	sceneB := snapshotScene(11, 5)
	index := NewSpatialIndex(sceneA)
	all := sceneWideRect()

	var failed atomic.Bool
	var readers sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for query := 0; query < 300; query++ {
				nodes := index.QueryNodes(all, nil)
				if len(nodes) != 7 && len(nodes) != 11 {
					failed.Store(true)
					return
				}
				edges := index.QueryEdges(all, nil)
				if len(edges) != 3 && len(edges) != 5 {
					failed.Store(true)
					return
				}
			}
		}()
	}

	for iteration := 0; iteration < 200; iteration++ {
		if iteration%2 == 0 {
			index.ReplaceScene(sceneB)
		} else {
			index.ReplaceScene(sceneA)
		}
		runtime.Gosched()
	}
	readers.Wait()
	if failed.Load() {
		t.Fatal("concurrent query observed a partial or mixed snapshot")
	}
}

func BenchmarkSpatialQueries(b *testing.B) {
	scene, err := GenerateDefault(500, DensityDense)
	if err != nil {
		b.Fatal(err)
	}
	linear := NewLinearIndex(scene)
	index := NewSpatialIndex(scene)
	localized := Rect{
		Min: scene.Bounds.Min,
		Max: Point{X: scene.Bounds.Min.X + 900, Y: scene.Bounds.Min.Y + 600},
	}
	overview := scene.Bounds

	for _, tc := range []struct {
		name  string
		query SpatialQuery
		area  Rect
	}{
		{name: "localized/linear", query: linear, area: localized},
		{name: "localized/index", query: index, area: localized},
		{name: "overview/linear", query: linear, area: overview},
		{name: "overview/index", query: index, area: overview},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			dst := make([]int, 0, 1024)
			for b.Loop() {
				dst = tc.query.QueryNodes(tc.area, dst[:0])
				dst = tc.query.QueryEdges(tc.area, dst)
			}
			benchmarkQueryResult = len(dst)
		})
	}
}

var benchmarkQueryResult int

func BenchmarkSpatialQueryPoint500(b *testing.B) {
	scene, err := GenerateDefault(500, DensityDense)
	if err != nil {
		b.Fatal(err)
	}
	point := rectMiddle(scene.Nodes[250].Bounds)
	pointRect := Rect{Min: point, Max: point}
	linear := NewLinearIndex(scene)
	index := NewSpatialIndex(scene)

	for _, tc := range []struct {
		name  string
		query func([]int) []int
	}{
		{name: "point/linear", query: func(dst []int) []int { return linear.QueryNodes(pointRect, dst) }},
		{name: "point/index", query: func(dst []int) []int { return index.QueryPoint(point, dst) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkPointResult = len(tc.query(nil))
			}
		})
	}
}

func BenchmarkSpatialIndexReplaceScene(b *testing.B) {
	scene, err := GenerateDefault(500, DensityDense)
	if err != nil {
		b.Fatal(err)
	}
	index := NewSpatialIndex(scene)
	b.ReportAllocs()
	for b.Loop() {
		index.ReplaceScene(scene)
	}
	benchmarkReplaceSceneNodes = len(scene.Nodes)
}

var (
	benchmarkPointResult       int
	benchmarkReplaceSceneNodes int
)

func queryRectangles(scene Scene) []Rect {
	rects := []Rect{scene.Bounds}
	stepX := max(scene.Bounds.Width()/12, 1)
	stepY := max(scene.Bounds.Height()/12, 1)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			x := scene.Bounds.Min.X + float64(col)*stepX/2
			y := scene.Bounds.Min.Y + float64(row)*stepY/2
			rects = append(rects, Rect{
				Min: Point{X: x, Y: y},
				Max: Point{X: x + stepX*1.75, Y: y + stepY*1.75},
			})
		}
	}
	for _, offset := range []int{0, len(scene.Nodes) / 2, len(scene.Nodes) - 1} {
		p := rectMiddle(scene.Nodes[offset].Bounds)
		rects = append(rects, Rect{Min: p, Max: p})
		rects = append(rects, Rect{
			Min: Point{X: scene.Nodes[offset].Bounds.Max.X, Y: scene.Nodes[offset].Bounds.Min.Y},
			Max: Point{X: scene.Nodes[offset].Bounds.Max.X, Y: scene.Nodes[offset].Bounds.Max.Y},
		})
	}
	return rects
}

func densityCountName(density Density, count int) string {
	return density.String() + "-" + intString(count)
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func manualScene(nodes []Node, edges []Edge) Scene {
	scene := Scene{Nodes: nodes, Edges: edges}
	if len(nodes) > 0 {
		scene.Bounds = nodes[0].Bounds
		for _, node := range nodes[1:] {
			scene.Bounds = scene.Bounds.Union(node.Bounds)
		}
	}
	for _, edge := range edges {
		if len(edge.Points) == 0 {
			continue
		}
		bounds := edge.Bounds()
		if scene.Bounds == (Rect{}) {
			scene.Bounds = bounds
		} else {
			scene.Bounds = scene.Bounds.Union(bounds)
		}
	}
	return scene
}

func sceneWideRect() Rect {
	return Rect{Min: Point{X: -1000, Y: -1000}, Max: Point{X: 1000, Y: 1000}}
}

func snapshotScene(nodeCount, edgeCount int) Scene {
	nodes := make([]Node, nodeCount)
	for node := range nodes {
		x := float64(node * 20)
		nodes[node] = Node{
			ID:     node,
			Bounds: Rect{Min: Point{X: x, Y: 0}, Max: Point{X: x + 10, Y: 10}},
		}
	}
	edges := make([]Edge, edgeCount)
	for edge := range edges {
		edges[edge] = Edge{
			ID:     edge,
			Points: []Point{{X: 0, Y: 5}, {X: float64(nodeCount*20 + 10), Y: 5}},
		}
	}
	return manualScene(nodes, edges)
}
