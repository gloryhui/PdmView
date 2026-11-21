package graph

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unicode"
)

func TestGenerateSupportedCountsAndDeterminism(t *testing.T) {
	if DefaultSeed != 20261121 {
		t.Fatalf("DefaultSeed = %d, want 20261121", DefaultSeed)
	}
	for _, count := range []int{50, 150, 300, 500} {
		for _, density := range []Density{DensitySparse, DensityDense} {
			t.Run(countName(count, density), func(t *testing.T) {
				first, err := Generate(count, DefaultSeed, density)
				if err != nil {
					t.Fatal(err)
				}
				second, err := Generate(count, DefaultSeed, density)
				if err != nil {
					t.Fatal(err)
				}
				defaulted, err := GenerateDefault(count, density)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, defaulted) {
					t.Fatal("same count, seed, and density did not reproduce the scene")
				}

				if got := len(first.Nodes); got != count {
					t.Fatalf("nodes = %d, want %d", got, count)
				}
				if got, want := len(first.Edges), count*2; got != want {
					t.Fatalf("edges = %d, want %d", got, want)
				}

				perSource := make([]int, count)
				pairs := make(map[[2]int]struct{}, len(first.Edges))
				lastFrom := -1
				for i, node := range first.Nodes {
					if node.ID != i {
						t.Fatalf("node %d ID = %d", i, node.ID)
					}
				}
				for i, edge := range first.Edges {
					if edge.ID != i {
						t.Fatalf("edge %d ID = %d", i, edge.ID)
					}
					if edge.From < 0 || edge.From >= count || edge.To < 0 || edge.To >= count {
						t.Fatalf("edge %d has unknown node IDs %d -> %d", i, edge.From, edge.To)
					}
					if edge.From == edge.To {
						t.Fatalf("edge %d is a self edge", i)
					}
					if edge.From < lastFrom {
						t.Fatalf("edge %d From order %d is before %d", i, edge.From, lastFrom)
					}
					lastFrom = edge.From
					pair := [2]int{edge.From, edge.To}
					if _, exists := pairs[pair]; exists {
						t.Fatalf("repeated directed pair %d -> %d", edge.From, edge.To)
					}
					pairs[pair] = struct{}{}
					perSource[edge.From]++
				}
				for nodeID, edges := range perSource {
					if edges != 2 {
						t.Fatalf("node %d has %d edges, want 2", nodeID, edges)
					}
				}
			})
		}
	}
}

func TestGenerateExtentPortsAndDerivedBounds(t *testing.T) {
	for _, density := range []Density{DensitySparse, DensityDense} {
		t.Run(density.String(), func(t *testing.T) {
			scene, err := Generate(150, 77, density)
			if err != nil {
				t.Fatal(err)
			}
			expected := scene.Nodes[0].Bounds
			for _, node := range scene.Nodes {
				expected = expected.Union(node.Bounds)
			}
			for _, edge := range scene.Edges {
				if len(edge.Points) < 2 {
					t.Fatalf("edge %d has %d points", edge.ID, len(edge.Points))
				}
				from, to := scene.Nodes[edge.From], scene.Nodes[edge.To]
				if !isPortCenter(from.Bounds, edge.Points[0]) {
					t.Fatalf("edge %d starts at %+v, not a port of node %d", edge.ID, edge.Points[0], from.ID)
				}
				if !isPortCenter(to.Bounds, edge.Points[len(edge.Points)-1]) {
					t.Fatalf("edge %d ends at %+v, not a port of node %d", edge.ID, edge.Points[len(edge.Points)-1], to.ID)
				}
				edgeBounds := pointBounds(edge.Points)
				if edge.Bounds() != edgeBounds {
					t.Fatalf("edge %d bounds = %+v, want %+v", edge.ID, edge.Bounds(), edgeBounds)
				}
				expected = expected.Union(edgeBounds)
			}
			if scene.Bounds != expected {
				t.Fatalf("scene bounds = %+v, want %+v", scene.Bounds, expected)
			}
			for _, node := range scene.Nodes {
				if !containsRect(scene.Bounds, node.Bounds) {
					t.Fatalf("scene bounds omit node %d: %+v", node.ID, node.Bounds)
				}
			}
		})
	}
}

func TestDenseSparseAndCrossingRelationships(t *testing.T) {
	for _, count := range []int{50, 500} {
		dense, err := Generate(count, DefaultSeed, DensityDense)
		if err != nil {
			t.Fatal(err)
		}
		sparse, err := Generate(count, DefaultSeed, DensitySparse)
		if err != nil {
			t.Fatal(err)
		}
		if sparse.Bounds.Width() <= dense.Bounds.Width() || sparse.Bounds.Height() <= dense.Bounds.Height() {
			t.Fatalf("%d nodes: sparse extent %vx%v is not larger than dense %vx%v",
				count, sparse.Bounds.Width(), sparse.Bounds.Height(),
				dense.Bounds.Width(), dense.Bounds.Height())
		}
		if !hasCrossing(dense) {
			t.Fatalf("%d dense edges have no geometric crossing", count)
		}
		if !hasCrossing(sparse) {
			t.Fatalf("%d sparse edges have no geometric crossing", count)
		}
	}
}

func TestFixtureNamesFieldsAndDensityValidation(t *testing.T) {
	scene, err := Generate(50, 9, DensitySparse)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]struct{}, len(scene.Nodes))
	for _, node := range scene.Nodes {
		if _, exists := seen[node.Name]; exists {
			t.Fatalf("duplicate node name %q", node.Name)
		}
		seen[node.Name] = struct{}{}
		if !hasHan(node.Name) || !hasASCII(node.Name) || len(node.Name) < 30 {
			t.Fatalf("node %d name is not long bilingual content: %q", node.ID, node.Name)
		}
		if len(node.Fields) < 4 {
			t.Fatalf("node %d has %d fields", node.ID, len(node.Fields))
		}
		for _, field := range node.Fields {
			if !hasHan(field.Name) || !hasASCII(field.Name) || len(field.Name) < 24 {
				t.Fatalf("node %d field is not long bilingual content: %q", node.ID, field.Name)
			}
		}
	}

	if _, err := Generate(100, DefaultSeed, DensityDense); !errors.Is(err, ErrUnsupportedCount) {
		t.Fatalf("unsupported count error = %v", err)
	}
	if _, err := Generate(50, DefaultSeed, Density(99)); !errors.Is(err, ErrInvalidDensity) {
		t.Fatalf("invalid density error = %v", err)
	}
}

func countName(count int, density Density) string {
	return fmt.Sprintf("%03d-%s", count, density)
}

func isPortCenter(r Rect, p Point) bool {
	cx, cy := rectCenter(r)
	return p == (Point{X: r.Min.X, Y: cy}) ||
		p == (Point{X: r.Max.X, Y: cy}) ||
		p == (Point{X: cx, Y: r.Min.Y}) ||
		p == (Point{X: cx, Y: r.Max.Y})
}

func pointBounds(points []Point) Rect {
	bounds := Rect{Min: points[0], Max: points[0]}
	for _, p := range points[1:] {
		bounds.Min.X = min(bounds.Min.X, p.X)
		bounds.Min.Y = min(bounds.Min.Y, p.Y)
		bounds.Max.X = max(bounds.Max.X, p.X)
		bounds.Max.Y = max(bounds.Max.Y, p.Y)
	}
	return bounds
}

func containsRect(outer, inner Rect) bool {
	return inner.Min.X >= outer.Min.X && inner.Min.Y >= outer.Min.Y &&
		inner.Max.X <= outer.Max.X && inner.Max.Y <= outer.Max.Y
}

func hasCrossing(scene Scene) bool {
	for i := 0; i < len(scene.Edges); i++ {
		for j := i + 1; j < len(scene.Edges); j++ {
			a, b := scene.Edges[i], scene.Edges[j]
			if a.From == b.From || a.From == b.To || a.To == b.From || a.To == b.To {
				continue
			}
			for ai := 0; ai+1 < len(a.Points); ai++ {
				for bi := 0; bi+1 < len(b.Points); bi++ {
					if properCross(a.Points[ai], a.Points[ai+1], b.Points[bi], b.Points[bi+1]) {
						return true
					}
				}
			}
		}
	}
	return false
}

func properCross(a, b, c, d Point) bool {
	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)
	return o1*o2 < 0 && o3*o4 < 0
}

func orientation(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func hasASCII(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}
