package graph

import (
	"reflect"
	"testing"

	"pdmview/internal/pdm"
)

func TestMapDiagramPreservesOrderGeometryPathsAndBounds(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("table-a", "", "A_CODE",
				mappingColumn("col-a2", "", "A2"),
				mappingColumn("col-a1", "A1", "IGNORED"),
			),
			mappingTable("table-b", "Table B", "B",
				mappingColumn("col-b1", "B1", ""),
			),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram-one"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "s-b"}, TableID: "table-b", Rect: mappingRect(-100, -80, -20, 0)},
			{Metadata: pdm.Metadata{ID: "s-a"}, TableID: "table-a", Rect: mappingRect(10, 20, 110, 80)},
			{Metadata: pdm.Metadata{ID: "s-external"}, TableID: "external-table", Rect: mappingRect(200, 200, 250, 240)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{
			{
				Metadata:            pdm.Metadata{ID: "r-one"},
				ReferenceID:         "reference-one",
				SourceSymbolID:      "s-b",
				DestinationSymbolID: "s-a",
				Rect:                mappingRect(-120, -90, 310, 110),
				Points:              []pdm.Point{{X: -150, Y: -100}, {X: 0, Y: 0}, {X: 300, Y: 100}},
				ControlPoints:       []pdm.Point{{X: 1000, Y: 1000}},
			},
			{
				Metadata:            pdm.Metadata{ID: "r-two"},
				ReferenceID:         "reference-two",
				SourceSymbolID:      "missing-symbol",
				DestinationSymbolID: "s-external",
			},
		},
	}

	result := MapDiagram(model, diagram)
	if result.DiagramID != "diagram-one" || result.AutoLayout {
		t.Fatalf("diagram result = %+v", result)
	}
	if len(result.Scene.Nodes) != 3 || len(result.NodeSources) != 3 {
		t.Fatalf("nodes/sources = %d/%d", len(result.Scene.Nodes), len(result.NodeSources))
	}
	if len(result.Scene.Edges) != 2 || len(result.EdgeSources) != 2 {
		t.Fatalf("edges/sources = %d/%d", len(result.Scene.Edges), len(result.EdgeSources))
	}

	first := result.Scene.Nodes[0]
	if first.ID != 0 || first.Name != "Table B" || len(first.Fields) != 1 ||
		first.Fields[0].Name != "B1" ||
		first.Bounds != (Rect{Min: Point{X: -100, Y: -80}, Max: Point{X: -20, Y: 0}}) {
		t.Fatalf("first node = %+v", first)
	}
	second := result.Scene.Nodes[1]
	if second.ID != 1 || second.Name != "A_CODE" || len(second.Fields) != 2 ||
		second.Fields[0].Name != "A2" || second.Fields[1].Name != "A1" {
		t.Fatalf("ordered fallback fields = %+v", second)
	}
	external := result.Scene.Nodes[2]
	if external.ID != 2 || external.Name != "external-table" || external.Fields != nil {
		t.Fatalf("external node = %+v", external)
	}

	if result.NodeSources[0].SymbolID != "s-b" ||
		result.NodeSources[0].TableID != "table-b" ||
		result.NodeSources[0].Rect == nil ||
		*result.NodeSources[0].Rect != *mappingRect(-100, -80, -20, 0) {
		t.Fatalf("first node source = %+v", result.NodeSources[0])
	}
	if result.NodeSources[0].Rect == diagram.TableSymbols[0].Rect {
		t.Fatal("node source Rect aliases the PDM source")
	}

	edge := result.Scene.Edges[0]
	if edge.ID != 0 || edge.From != 0 || edge.To != 1 ||
		!reflect.DeepEqual(edge.Points, []Point{{X: -150, Y: -100}, {X: 0, Y: 0}, {X: 300, Y: 100}}) {
		t.Fatalf("first edge = %+v", edge)
	}
	if result.EdgeSources[0].SymbolID != "r-one" ||
		result.EdgeSources[0].SourceSymbolID != "s-b" ||
		result.EdgeSources[0].DestinationSymbolID != "s-a" ||
		!reflect.DeepEqual(result.EdgeSources[0].Points, diagram.ReferenceSymbols[0].Points) ||
		!reflect.DeepEqual(result.EdgeSources[0].ControlPoints, []pdm.Point{{X: 1000, Y: 1000}}) {
		t.Fatalf("first edge source = %+v", result.EdgeSources[0])
	}
	if result.Scene.Edges[1].From != -1 || result.Scene.Edges[1].To != 2 ||
		result.Scene.Edges[1].Points != nil {
		t.Fatalf("unresolved/empty edge = %+v", result.Scene.Edges[1])
	}

	wantBounds := Rect{Min: Point{X: -150, Y: -100}, Max: Point{X: 300, Y: 240}}
	if result.Scene.Bounds != wantBounds {
		t.Fatalf("bounds = %+v, want %+v", result.Scene.Bounds, wantBounds)
	}
}

func TestMapDiagramMultipleDiagramsAndSameTableSymbols(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("shared-table", "Shared", "SHARED", mappingColumn("shared-col", "Shared Column", "SHARED_COL")),
		}},
	}
	diagramA := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram-a"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "symbol-a"}, TableID: "shared-table", Rect: mappingRect(0, 0, 20, 10)},
			{Metadata: pdm.Metadata{ID: "symbol-a2"}, TableID: "shared-table", Rect: mappingRect(30, 0, 50, 10)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{{
			Metadata:            pdm.Metadata{ID: "loop"},
			ReferenceID:         "self-reference",
			SourceSymbolID:      "symbol-a2",
			DestinationSymbolID: "symbol-a",
			Points:              []pdm.Point{{X: 30, Y: 5}, {X: 20, Y: 5}},
		}},
	}
	diagramB := &pdm.Diagram{
		Metadata:     pdm.Metadata{ID: "diagram-b"},
		OwnerID:      "model",
		TableSymbols: []pdm.TableSymbol{{Metadata: pdm.Metadata{ID: "symbol-b"}, TableID: "shared-table", Rect: mappingRect(100, 200, 140, 260)}},
	}

	resultA := MapDiagram(model, diagramA)
	resultB := MapDiagram(model, diagramB)
	if resultA.DiagramID != "diagram-a" || resultB.DiagramID != "diagram-b" {
		t.Fatalf("diagram IDs = %q/%q", resultA.DiagramID, resultB.DiagramID)
	}
	if len(resultA.Scene.Nodes) != 2 || len(resultB.Scene.Nodes) != 1 ||
		resultA.Scene.Nodes[0].Name != resultB.Scene.Nodes[0].Name {
		t.Fatalf("same-table nodes = %+v / %+v", resultA.Scene.Nodes, resultB.Scene.Nodes)
	}
	if resultA.NodeSources[1].TableID != resultA.NodeSources[0].TableID ||
		resultA.NodeSources[1].SymbolID == resultA.NodeSources[0].SymbolID ||
		resultA.Scene.Nodes[1].Bounds == resultA.Scene.Nodes[0].Bounds {
		t.Fatalf("same-diagram symbols merged: %+v", resultA)
	}
	if len(resultA.Scene.Edges) != 1 || resultA.Scene.Edges[0].From != 1 || resultA.Scene.Edges[0].To != 0 {
		t.Fatalf("relationship endpoints did not use distinct table symbols: %+v", resultA.Scene.Edges)
	}
	if resultA.Scene.Nodes[0].Bounds == resultB.Scene.Nodes[0].Bounds {
		t.Fatal("same table's diagram symbols were merged")
	}
	if resultA.NodeSources[0].Rect == resultB.NodeSources[0].Rect {
		t.Fatal("different diagram symbols share copied geometry")
	}
}

func TestMapDiagramCrossPackageLookupOwnerFallbackAndNilDiagram(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{mappingTable("root-table", "Root", "ROOT")},
			Packages: []pdm.Package{
				{
					Metadata: pdm.Metadata{ID: "package-a"},
					Contents: pdm.Contents{
						Tables: []pdm.Table{mappingTable("package-a-table", "Package A", "PKG_A")},
						Packages: []pdm.Package{{
							Metadata: pdm.Metadata{ID: "package-a-child"},
							Contents: pdm.Contents{Tables: []pdm.Table{
								mappingTable("nested-table", "Nested", "NESTED"),
							}},
						}},
					},
				},
				{
					Metadata: pdm.Metadata{ID: "package-b"},
					Contents: pdm.Contents{Tables: []pdm.Table{
						mappingTable("package-b-table", "Package B", "PKG_B"),
					}},
				},
			},
		},
	}

	crossPackage := &pdm.Diagram{
		Metadata:     pdm.Metadata{ID: "cross-package"},
		OwnerID:      "package-a",
		TableSymbols: []pdm.TableSymbol{{Metadata: pdm.Metadata{ID: "b-symbol"}, TableID: "package-b-table", Rect: mappingRect(0, 0, 10, 10)}},
	}
	result := MapDiagram(model, crossPackage)
	if len(result.Scene.Nodes) != 1 || result.Scene.Nodes[0].Name != "Package B" {
		t.Fatalf("cross-package lookup = %+v", result.Scene.Nodes)
	}

	emptyPackageDiagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "empty-package"},
		OwnerID:  "package-a",
	}
	result = MapDiagram(model, emptyPackageDiagram)
	assertNodeTableIDs(t, result, []string{"package-a-table"})
	if !result.AutoLayout || result.NodeSources[0].SymbolID != "" || result.NodeSources[0].Rect != nil {
		t.Fatalf("owner fallback sources = %+v", result)
	}

	emptyModelDiagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "empty-model"},
		OwnerID:  "model",
	}
	result = MapDiagram(model, emptyModelDiagram)
	assertNodeTableIDs(t, result, []string{"root-table"})

	result = MapDiagram(model, nil)
	assertNodeTableIDs(t, result, []string{
		"root-table", "package-a-table", "nested-table", "package-b-table",
	})
	if result.DiagramID != "" || !result.AutoLayout {
		t.Fatalf("nil diagram result = %+v", result)
	}
}

func TestMapDiagramAutoLayoutDeterministicMixedAndFullFallback(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("valid", "Valid", "VALID"),
			mappingTable("missing", "Missing", "MISSING"),
			mappingTable("zero", "Zero", "ZERO"),
			mappingTable("reversed", "Reversed", "REVERSED"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "mixed"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "valid-symbol"}, TableID: "valid", Rect: mappingRect(-200, -100, -50, 20)},
			{Metadata: pdm.Metadata{ID: "missing-symbol"}, TableID: "missing", Rect: nil},
			{Metadata: pdm.Metadata{ID: "zero-symbol"}, TableID: "zero", Rect: mappingRect(5, 5, 5, 5)},
			{Metadata: pdm.Metadata{ID: "reversed-symbol"}, TableID: "reversed", Rect: mappingRect(20, 20, 10, 10)},
		},
	}
	first := MapDiagram(model, diagram)
	second := MapDiagram(model, diagram)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mixed auto layout is not deterministic")
	}
	if !first.AutoLayout {
		t.Fatal("mixed auto layout flag is false")
	}
	if first.Scene.Nodes[0].Bounds != (Rect{Min: Point{X: -200, Y: -100}, Max: Point{X: -50, Y: 20}}) {
		t.Fatalf("valid geometry changed: %+v", first.Scene.Nodes[0].Bounds)
	}
	previous := first.Scene.Nodes[0].Bounds
	for offset, node := range first.Scene.Nodes[1:] {
		if node.Bounds.Min.X <= previous.Max.X || node.Bounds.Min.Y != -100 ||
			node.Bounds.Width() != 236 || node.Bounds.Height() != 144 {
			t.Fatalf("auto node %d overlaps or has unexpected bounds: %+v", offset+1, node.Bounds)
		}
		previous = node.Bounds
	}
	if first.NodeSources[1].Rect != nil ||
		first.NodeSources[2].Rect == nil || first.NodeSources[2].Rect.Min != (pdm.Point{X: 5, Y: 5}) ||
		first.NodeSources[3].Rect == nil || first.NodeSources[3].Rect.Min != (pdm.Point{X: 20, Y: 20}) {
		t.Fatalf("invalid source rectangles not preserved: %+v", first.NodeSources)
	}

	full := MapDiagram(model, &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "fallback"},
		OwnerID:  "model",
	})
	if !full.AutoLayout || len(full.Scene.Nodes) != len(model.Contents.Tables) {
		t.Fatalf("full fallback = %+v", full)
	}
	if full.Scene.Nodes[0].Bounds.Min != (Point{}) ||
		full.Scene.Nodes[1].Bounds.Min != (Point{X: 300, Y: 0}) {
		t.Fatalf("full fallback grid = %+v", full.Scene.Nodes)
	}
}

func TestMapDiagramUnresolvedNodesEmptyInputAndDuplicateSymbolIDs(t *testing.T) {
	empty := MapDiagram(nil, nil)
	if empty.DiagramID != "" || len(empty.Scene.Nodes) != 0 || len(empty.Scene.Edges) != 0 ||
		len(empty.NodeSources) != 0 || len(empty.EdgeSources) != 0 || empty.AutoLayout ||
		!reflect.DeepEqual(empty.Scene.Bounds, Rect{}) {
		t.Fatalf("empty graph = %+v", empty)
	}

	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("one", "One", "ONE"),
			mappingTable("two", "Two", "TWO"),
			mappingTable("three", "Three", "THREE"),
			mappingTable("four", "Four", "FOUR"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "duplicates"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "duplicate"}, TableID: "one", Rect: mappingRect(0, 0, 1, 1)},
			{Metadata: pdm.Metadata{ID: "duplicate"}, TableID: "two", Rect: mappingRect(2, 0, 3, 1)},
			{Metadata: pdm.Metadata{ID: ""}, TableID: "three", Rect: mappingRect(4, 0, 5, 1)},
			{Metadata: pdm.Metadata{ID: "unique"}, TableID: "four", Rect: mappingRect(6, 0, 7, 1)},
			{Metadata: pdm.Metadata{ID: "external-symbol"}, TableID: "external-table", Rect: mappingRect(8, 0, 9, 1)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{
			{Metadata: pdm.Metadata{ID: "duplicate-source"}, SourceSymbolID: "duplicate", DestinationSymbolID: "unique"},
			{Metadata: pdm.Metadata{ID: "missing-source"}, SourceSymbolID: "not-present", DestinationSymbolID: "unique"},
			{Metadata: pdm.Metadata{ID: "empty-source"}, DestinationSymbolID: ""},
		},
	}
	result := MapDiagram(model, diagram)
	if len(result.Scene.Nodes) != 5 ||
		result.Scene.Nodes[4].Name != "external-table" ||
		len(result.Scene.Edges) != 3 {
		t.Fatalf("duplicate/unresolved nodes = %+v", result)
	}
	if result.Scene.Edges[0].From != -1 || result.Scene.Edges[0].To != 3 {
		t.Fatalf("duplicate symbol resolved to a wrong endpoint: %+v", result.Scene.Edges[0])
	}
	if result.Scene.Edges[1].From != -1 ||
		result.Scene.Edges[2].From != -1 || result.Scene.Edges[2].To != -1 {
		t.Fatalf("missing IDs selected endpoints: %+v", result.Scene.Edges)
	}
}

func TestMapDiagramSourceCopiesAndIntegerPrecision(t *testing.T) {
	const (
		largeMinX int64 = 9_007_199_254_740_993
		largeMaxX int64 = 9_007_199_254_740_997
		largeY    int64 = 9_007_199_254_740_999
	)
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("table", "Table", "TABLE"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "precision"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{{
			Metadata: pdm.Metadata{ID: "symbol"},
			TableID:  "table",
			Rect:     &pdm.Rect{Min: pdm.Point{X: largeMinX, Y: largeY}, Max: pdm.Point{X: largeMaxX, Y: largeY + 2}},
		}},
		ReferenceSymbols: []pdm.ReferenceSymbol{{
			Metadata:            pdm.Metadata{ID: "edge"},
			SourceSymbolID:      "symbol",
			DestinationSymbolID: "symbol",
			Rect:                &pdm.Rect{Min: pdm.Point{X: largeMinX, Y: largeY}, Max: pdm.Point{X: largeMaxX, Y: largeY + 2}},
			Points:              []pdm.Point{{X: largeMinX, Y: largeY}, {X: largeMaxX, Y: largeY + 2}},
			ControlPoints:       []pdm.Point{{X: largeMinX + 1, Y: largeY + 1}},
		}},
	}
	result := MapDiagram(model, diagram)
	if result.NodeSources[0].Rect == nil ||
		result.NodeSources[0].Rect.Min.X != largeMinX ||
		result.NodeSources[0].Rect.Max.X != largeMaxX {
		t.Fatalf("node integer precision lost: %+v", result.NodeSources[0].Rect)
	}
	if result.EdgeSources[0].Points[0].X != largeMinX ||
		result.EdgeSources[0].ControlPoints[0].X != largeMinX+1 {
		t.Fatalf("edge integer precision lost: %+v", result.EdgeSources[0])
	}

	result.NodeSources[0].Rect.Min.X = 1
	result.EdgeSources[0].Points[0].X = 2
	result.EdgeSources[0].ControlPoints[0].X = 3
	if diagram.TableSymbols[0].Rect.Min.X != largeMinX ||
		diagram.ReferenceSymbols[0].Points[0].X != largeMinX ||
		diagram.ReferenceSymbols[0].ControlPoints[0].X != largeMinX+1 {
		t.Fatal("result geometry aliases the PDM source")
	}
	if result.Scene.Nodes[0].Bounds.Min.X != float64(largeMinX) ||
		result.Scene.Edges[0].Points[0].X != float64(largeMinX) {
		t.Fatal("result source geometry aliases Scene geometry")
	}
	if reflect.DeepEqual(result.EdgeSources[0].Points, result.EdgeSources[0].ControlPoints) {
		t.Fatal("control points were concatenated with path points")
	}
	if result.Scene.Bounds.Max.X < float64(largeMaxX) {
		t.Fatalf("path bounds do not include extended geometry: %+v", result.Scene.Bounds)
	}
}

func mappingTable(id, name, code string, columns ...pdm.Column) pdm.Table {
	return pdm.Table{
		Metadata: pdm.Metadata{ID: id, Name: name, Code: code},
		Columns:  columns,
	}
}

func mappingColumn(id, name, code string) pdm.Column {
	return pdm.Column{Metadata: pdm.Metadata{ID: id, Name: name, Code: code}}
}

func mappingRect(minX, minY, maxX, maxY int64) *pdm.Rect {
	return &pdm.Rect{
		Min: pdm.Point{X: minX, Y: minY},
		Max: pdm.Point{X: maxX, Y: maxY},
	}
}

func assertNodeTableIDs(t *testing.T, result DiagramGraph, want []string) {
	t.Helper()
	got := make([]string, len(result.NodeSources))
	for offset, source := range result.NodeSources {
		got[offset] = source.TableID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("node table IDs = %v, want %v", got, want)
	}
}
