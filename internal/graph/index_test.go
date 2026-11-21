package graph

import (
	"reflect"
	"slices"
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
