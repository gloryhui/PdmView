package graph

import (
	"reflect"
	"testing"

	"pdmview/internal/pdm"
)

func TestDiagramGraphSourceAccessorsTraceByOffset(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("table-one", "Table One", "TABLE_ONE"),
			mappingTable("table-two", "Table Two", "TABLE_TWO"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "diagram-trace"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "symbol-one"}, TableID: "table-one", Rect: mappingRect(-10, -20, 30, 40)},
			{Metadata: pdm.Metadata{ID: "symbol-two"}, TableID: "table-two", Rect: mappingRect(100, 200, 140, 260)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{
			{
				Metadata:            pdm.Metadata{ID: "reference-symbol-one"},
				ReferenceID:         "reference-one",
				SourceSymbolID:      "symbol-two",
				DestinationSymbolID: "symbol-one",
				Rect:                mappingRect(0, 0, 200, 200),
				Points:              []pdm.Point{{X: 100, Y: 200}, {X: 50, Y: 100}, {X: 30, Y: 40}},
				ControlPoints:       []pdm.Point{{X: 75, Y: 150}},
			},
			{
				Metadata:            pdm.Metadata{ID: "reference-symbol-two"},
				ReferenceID:         "reference-two",
				SourceSymbolID:      "missing-symbol",
				DestinationSymbolID: "symbol-two",
			},
		},
	}

	result := MapDiagram(model, diagram)
	if result.DiagramID != "diagram-trace" ||
		len(result.Scene.Nodes) != len(diagram.TableSymbols) ||
		len(result.Scene.Edges) != len(diagram.ReferenceSymbols) {
		t.Fatalf("trace graph = %+v", result)
	}

	for offset, symbol := range diagram.TableSymbols {
		source, ok := result.NodeSource(offset)
		if !ok {
			t.Fatalf("node source %d is not available", offset)
		}
		if result.Scene.Nodes[offset].ID != offset ||
			source.SymbolID != symbol.ID ||
			source.TableID != symbol.TableID ||
			source.Rect == nil ||
			*source.Rect != *symbol.Rect {
			t.Fatalf("node trace %d = node:%+v source:%+v symbol:%+v",
				offset, result.Scene.Nodes[offset], source, symbol)
		}
		if source.Rect != result.NodeSources[offset].Rect {
			t.Fatalf("node source %d is not a read view", offset)
		}
	}

	for offset, symbol := range diagram.ReferenceSymbols {
		source, ok := result.EdgeSource(offset)
		if !ok {
			t.Fatalf("edge source %d is not available", offset)
		}
		if result.Scene.Edges[offset].ID != offset ||
			source.SymbolID != symbol.ID ||
			source.ReferenceID != symbol.ReferenceID ||
			source.SourceSymbolID != symbol.SourceSymbolID ||
			source.DestinationSymbolID != symbol.DestinationSymbolID ||
			!reflect.DeepEqual(source.Points, symbol.Points) ||
			!reflect.DeepEqual(source.ControlPoints, symbol.ControlPoints) {
			t.Fatalf("edge trace %d = edge:%+v source:%+v symbol:%+v",
				offset, result.Scene.Edges[offset], source, symbol)
		}
		if offset == 0 {
			if source.Rect == nil || *source.Rect != *symbol.Rect {
				t.Fatalf("edge source rect = %+v", source.Rect)
			}
			if len(source.Points) != 0 && &source.Points[0] != &result.EdgeSources[offset].Points[0] {
				t.Fatalf("edge source %d is not a read view", offset)
			}
		}
	}

	if result.Scene.Edges[0].From != 1 || result.Scene.Edges[0].To != 0 ||
		result.Scene.Edges[1].From != -1 || result.Scene.Edges[1].To != 1 {
		t.Fatalf("scene edge endpoints = %+v", result.Scene.Edges)
	}
}

func TestDiagramGraphSourceAccessorsRejectInvalidAlignment(t *testing.T) {
	var empty DiagramGraph
	if source, ok := empty.NodeSource(0); ok || source != (DiagramNodeSource{}) {
		t.Fatalf("empty node source = %+v, %v", source, ok)
	}
	if source, ok := empty.EdgeSource(0); ok || !reflect.DeepEqual(source, DiagramEdgeSource{}) {
		t.Fatalf("empty edge source = %+v, %v", source, ok)
	}

	graph := DiagramGraph{
		Scene: Scene{
			Nodes: []Node{{ID: 0}},
			Edges: []Edge{{ID: 0}},
		},
		NodeSources: []DiagramNodeSource{{SymbolID: "node-source"}},
		EdgeSources: []DiagramEdgeSource{{SymbolID: "edge-source"}},
	}
	for _, offset := range []int{-1, -10, 1, 10} {
		if source, ok := graph.NodeSource(offset); ok || source != (DiagramNodeSource{}) {
			t.Fatalf("node source offset %d = %+v, %v", offset, source, ok)
		}
		if source, ok := graph.EdgeSource(offset); ok || !reflect.DeepEqual(source, DiagramEdgeSource{}) {
			t.Fatalf("edge source offset %d = %+v, %v", offset, source, ok)
		}
	}

	misaligned := DiagramGraph{
		Scene: Scene{
			Nodes: []Node{{ID: 0}},
			Edges: []Edge{{ID: 0}},
		},
	}
	if _, ok := misaligned.NodeSource(0); ok {
		t.Fatal("unaligned node source was returned")
	}
	if _, ok := misaligned.EdgeSource(0); ok {
		t.Fatal("unaligned edge source was returned")
	}
}
