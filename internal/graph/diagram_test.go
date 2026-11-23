package graph

import (
	"fmt"
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
		first.Bounds != (Rect{Min: Point{X: -100, Y: 0}, Max: Point{X: -20, Y: 80}}) {
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
		!reflect.DeepEqual(edge.Points, []Point{{X: -150, Y: 100}, {X: 0, Y: 0}, {X: 300, Y: -100}}) {
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

	wantBounds := Rect{Min: Point{X: -150, Y: -240}, Max: Point{X: 300, Y: 100}}
	if result.Scene.Bounds != wantBounds {
		t.Fatalf("bounds = %+v, want %+v", result.Scene.Bounds, wantBounds)
	}
}

func TestMapDiagramMirrorsPDMYCoordinates(t *testing.T) {
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("a", "A", "A"),
			mappingTable("b", "B", "B"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "mirror"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "a-symbol"}, TableID: "a", Rect: mappingRect(10, 100, 110, 160)},
			{Metadata: pdm.Metadata{ID: "b-symbol"}, TableID: "b", Rect: mappingRect(300, 20, 400, 80)},
		},
		ReferenceSymbols: []pdm.ReferenceSymbol{{
			Metadata:            pdm.Metadata{ID: "edge"},
			ReferenceID:         "reference",
			SourceSymbolID:      "a-symbol",
			DestinationSymbolID: "b-symbol",
			Rect:                mappingRect(10, 20, 400, 160),
			Points:              []pdm.Point{{X: 10, Y: 100}, {X: 10, Y: 160}, {X: 300, Y: 80}},
			ControlPoints:       []pdm.Point{{X: 200, Y: 100}},
		}},
	}

	result := MapDiagram(model, diagram)
	upper, lower := result.Scene.Nodes[0].Bounds, result.Scene.Nodes[1].Bounds
	if upper != (Rect{Min: Point{X: 10, Y: -160}, Max: Point{X: 110, Y: -100}}) ||
		lower != (Rect{Min: Point{X: 300, Y: -80}, Max: Point{X: 400, Y: -20}}) {
		t.Fatalf("projected upper/lower bounds = %+v / %+v", upper, lower)
	}
	if result.NodeSources[0].Rect == nil ||
		*result.NodeSources[0].Rect != *diagram.TableSymbols[0].Rect ||
		result.NodeSources[0].Rect == diagram.TableSymbols[0].Rect {
		t.Fatalf("raw node rectangle was not preserved: %+v", result.NodeSources[0].Rect)
	}
	if want := []Point{{X: 10, Y: -100}, {X: 10, Y: -160}, {X: 300, Y: -80}}; !reflect.DeepEqual(result.Scene.Edges[0].Points, want) {
		t.Fatalf("mirrored path = %+v, want %+v", result.Scene.Edges[0].Points, want)
	}
	if want := []pdm.Point{{X: 10, Y: 100}, {X: 10, Y: 160}, {X: 300, Y: 80}}; !reflect.DeepEqual(result.EdgeSources[0].Points, want) {
		t.Fatalf("raw path = %+v, want %+v", result.EdgeSources[0].Points, want)
	}

	index := NewSpatialIndex(result.Scene)
	camera := NewDiagramCamera(Rect{Max: Point{X: 500, Y: 300}})
	camera.FitBounds(result.Scene.Bounds, 0)
	nodeHit := HitTest(result.Scene, index, camera, camera.WorldToScreen(rectMiddle(upper)), 3)
	if nodeHit != (Hit{Kind: HitNode, Index: 0}) {
		t.Fatalf("mirrored node hit = %+v", nodeHit)
	}
	edgeHit := HitTest(result.Scene, index, camera, camera.WorldToScreen(Point{X: 155, Y: -120}), 3)
	if edgeHit != (Hit{Kind: HitEdge, Index: 0}) {
		t.Fatalf("mirrored edge hit = %+v", edgeHit)
	}
}

func TestMapAndBuildDiagramGraphAnnotationsAndBounds(t *testing.T) {
	black := &pdm.RGBColor{}
	white := &pdm.RGBColor{R: 255, G: 255, B: 255}
	model := &pdm.Model{
		Contents: pdm.Contents{Tables: []pdm.Table{
			mappingTable("table", "Table", "TABLE"),
		}},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "annotations"},
		TableSymbols: []pdm.TableSymbol{{
			Metadata: pdm.Metadata{ID: "table-symbol"},
			TableID:  "table",
			Rect:     mappingRect(0, 0, 100, 50),
		}},
		AnnotationSymbols: []pdm.AnnotationSymbol{
			{
				Metadata:  pdm.Metadata{ID: "visible"},
				Text:      "第一行\n第二行",
				Rect:      mappingRect(200, 100, 350, 260),
				Fill:      black,
				Border:    black,
				TextColor: white,
			},
			{
				Metadata: pdm.Metadata{ID: "blank"},
				Text:     " \n\t ",
				Rect:     mappingRect(400, 0, 500, 100),
			},
			{
				Metadata: pdm.Metadata{ID: "zero-width"},
				Text:     "zero",
				Rect:     mappingRect(600, 0, 600, 100),
			},
		},
	}

	mapped := MapDiagram(model, diagram)
	if len(mapped.Scene.Nodes) != 1 || mapped.Scene.Nodes[0].ID != 0 {
		t.Fatalf("node indexing changed: %+v", mapped.Scene.Nodes)
	}
	if len(mapped.Scene.Annotations) != 1 {
		t.Fatalf("annotations = %+v", mapped.Scene.Annotations)
	}
	annotation := mapped.Scene.Annotations[0]
	if annotation.Bounds != (Rect{Min: Point{X: 200, Y: -260}, Max: Point{X: 350, Y: -100}}) ||
		annotation.Text != "第一行\n第二行" ||
		annotation.Fill == nil || *annotation.Fill != (pdm.RGBColor{}) ||
		annotation.Border == nil || *annotation.Border != (pdm.RGBColor{}) ||
		annotation.TextColor == nil || *annotation.TextColor != *white {
		t.Fatalf("annotation = %+v", annotation)
	}
	wantBounds := Rect{Min: Point{X: 0, Y: -260}, Max: Point{X: 350, Y: 0}}
	if mapped.Scene.Bounds != wantBounds {
		t.Fatalf("mapped bounds = %+v, want %+v", mapped.Scene.Bounds, wantBounds)
	}

	built := BuildDiagramGraph(model, diagram)
	if len(built.Scene.Nodes) != 1 || built.Scene.Nodes[0].ID != 0 ||
		len(built.Scene.Annotations) != 1 {
		t.Fatalf("built scene indexes changed: %+v", built.Scene)
	}
	if built.Scene.Bounds != wantBounds {
		t.Fatalf("built bounds = %+v, want %+v", built.Scene.Bounds, wantBounds)
	}
}

func TestMapDiagramProjectsEachSymbolPresentation(t *testing.T) {
	table := mappingTable("table", "Table Name", "TABLE_CODE",
		pdm.Column{Metadata: pdm.Metadata{ID: "nonkey", Name: "First"}},
		pdm.Column{Metadata: pdm.Metadata{ID: "primary", Name: "Second", Code: "SECOND"}, DataType: pdm.OptionalText{Value: "int"}},
		pdm.Column{Metadata: pdm.Metadata{ID: "alternate", Name: "Third", Code: "THIRD"}},
	)
	table.PrimaryKeyID = "pk"
	table.Keys = []pdm.Key{
		{Metadata: pdm.Metadata{ID: "pk"}, ColumnIDs: []string{"primary"}},
		{Metadata: pdm.Metadata{ID: "ak"}, ColumnIDs: []string{"alternate"}},
	}
	model := &pdm.Model{Contents: pdm.Contents{Tables: []pdm.Table{table}}}
	yellow := &pdm.RGBColor{R: 255, G: 255}
	diagram := &pdm.Diagram{TableSymbols: []pdm.TableSymbol{
		{
			TableID: table.ID, Rect: mappingRect(0, 0, 200, 100),
			Appearance: pdm.TableAppearance{Fill: yellow, Display: &pdm.TableDisplay{
				ShowName: true, ShowColumns: true, ShowDataType: true, ShowKeyIndicator: true,
				ColumnFilter: pdm.ColumnFilterKey, ColumnLimit: 2,
			}},
		},
		{
			TableID: table.ID, Rect: mappingRect(300, 0, 500, 100),
			Appearance: pdm.TableAppearance{Display: &pdm.TableDisplay{ShowName: true}},
		},
	}}

	scene := MapDiagram(model, diagram).Scene
	first, second := scene.Nodes[0], scene.Nodes[1]
	if len(first.Fields) != 3 || len(second.Fields) != 3 || first.Name != "Table Name" {
		t.Fatal("presentation modified the underlying table data")
	}
	actual := first.Presentation
	if actual.Style.Fill != yellow || actual.Title != "Table Name" {
		t.Fatalf("first symbol appearance = %+v", actual)
	}
	want := []ColumnPresentation{
		{Name: "Second", DataType: "int", Indicators: "<pk>", PrimaryKey: true},
		{Name: "Third", Indicators: "<ak>"},
	}
	if !reflect.DeepEqual(actual.Columns, want) {
		t.Fatalf("filtered columns = %+v, want %+v", actual.Columns, want)
	}
	if second.Presentation.Title != "Table Name" || len(second.Presentation.Columns) != 0 {
		t.Fatalf("the second instance ignored its own display settings: %+v", second.Presentation)
	}
}

func TestMapDiagramPresentsAllRowsWithoutAppearanceAndKeepsUnresolvedStyle(t *testing.T) {
	columns := make([]pdm.Column, 21)
	for index := range columns {
		columns[index] = mappingColumn(
			fmt.Sprintf("col-%02d", index+1),
			fmt.Sprintf("Column %02d", index+1),
			fmt.Sprintf("COL_%02d", index+1),
		)
		columns[index].DataType = pdm.OptionalText{Value: "INTEGER", Present: true}
	}
	table := mappingTable("table", "All Rows", "ALL_ROWS", columns...)
	model := &pdm.Model{Contents: pdm.Contents{Tables: []pdm.Table{table}}}
	fill := &pdm.RGBColor{R: 12, G: 34, B: 56}
	diagram := &pdm.Diagram{TableSymbols: []pdm.TableSymbol{
		{
			Metadata: pdm.Metadata{ID: "resolved"},
			TableID:  "table",
			Rect:     mappingRect(0, 0, 300, 500),
		},
		{
			Metadata:   pdm.Metadata{ID: "unresolved-styled"},
			TableID:    "missing",
			Appearance: pdm.TableAppearance{Fill: fill},
			Rect:       mappingRect(400, 0, 700, 500),
		},
		{
			Metadata: pdm.Metadata{ID: "unresolved-plain"},
			TableID:  "missing",
			Rect:     mappingRect(800, 0, 1100, 500),
		},
	}}

	scene := MapDiagram(model, diagram).Scene
	resolved := scene.Nodes[0]
	if len(resolved.Fields) != 21 || resolved.Presentation == nil ||
		len(resolved.Presentation.Columns) != 21 ||
		resolved.Presentation.Title != "All Rows" ||
		resolved.Presentation.Columns[0].Name != "Column 01" ||
		resolved.Presentation.Columns[20].Name != "Column 21" {
		t.Fatalf("resolved presentation = %+v", resolved.Presentation)
	}
	if resolved.Presentation.Style != (pdm.TableAppearance{}) {
		t.Fatalf("resolved table with no saved appearance got style: %+v", resolved.Presentation.Style)
	}
	unresolvedStyled := scene.Nodes[1]
	if unresolvedStyled.Presentation == nil ||
		unresolvedStyled.Presentation.Style.Fill != fill ||
		len(unresolvedStyled.Presentation.Columns) != 0 {
		t.Fatalf("unresolved styled presentation = %+v", unresolvedStyled.Presentation)
	}
	unresolvedPlain := scene.Nodes[2]
	if unresolvedPlain.Presentation != nil {
		t.Fatalf("unresolved plain symbol gained a presentation: %+v", unresolvedPlain.Presentation)
	}
	fallback := MapDiagram(model, nil).Scene.Nodes[0]
	if fallback.Presentation == nil || len(fallback.Presentation.Columns) != 21 {
		t.Fatalf("fallback table lost its full presentation: %+v", fallback.Presentation)
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

func TestMapDiagramPopulatesCodeTypesAndKeyMarkers(t *testing.T) {
	parentTable := pdm.Table{
		Metadata: pdm.Metadata{ID: "parent", Name: "Parent Table", Code: "PARENT_CODE"},
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "parent-id", Name: "Parent ID", Code: "PARENT_ID"},
				DataType: pdm.OptionalText{Value: "INTEGER", Present: true}},
			{Metadata: pdm.Metadata{ID: "parent-name", Name: "Parent Name", Code: "PARENT_NAME"},
				DataType: pdm.OptionalText{Value: "VARCHAR(64)", Present: true}},
		},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "parent-pk"},
			ColumnIDs: []string{"parent-id"},
		}},
		PrimaryKeyID: "parent-pk",
	}
	childTable := pdm.Table{
		Metadata: pdm.Metadata{ID: "child", Name: "Child Table", Code: "CHILD_CODE"},
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "child-id", Name: "Child ID", Code: "CHILD_ID"},
				DataType: pdm.OptionalText{Value: "BIGINT", Present: true}},
			{Metadata: pdm.Metadata{ID: "child-parent-id", Name: "Parent Ref", Code: "PARENT_REF"},
				DataType: pdm.OptionalText{Value: "DECIMAL(10, 2)", Present: true}},
		},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "child-pk"},
			ColumnIDs: []string{"child-id"},
		}},
		PrimaryKeyID: "child-pk",
	}
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Packages: []pdm.Package{
				{Metadata: pdm.Metadata{ID: "package-parent"}, Contents: pdm.Contents{Tables: []pdm.Table{parentTable}}},
				{Metadata: pdm.Metadata{ID: "package-child"}, Contents: pdm.Contents{Tables: []pdm.Table{childTable}}},
			},
			References: []pdm.Reference{{
				Metadata:      pdm.Metadata{ID: "fk-child-parent"},
				OwnerID:       "missing-owner",
				ParentTableID: "parent",
				ChildTableID:  "child",
				Joins: []pdm.ReferenceJoin{{
					Metadata:       pdm.Metadata{ID: "join-one"},
					ParentColumnID: "parent-id",
					ChildColumnID:  "child-parent-id",
				}},
			}},
		},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "marker-diagram"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{
			{Metadata: pdm.Metadata{ID: "child-symbol"}, TableID: "child", Rect: mappingRect(0, 0, 100, 100)},
			{Metadata: pdm.Metadata{ID: "parent-symbol"}, TableID: "parent", Rect: mappingRect(200, 0, 300, 100)},
		},
	}

	result := MapDiagram(model, diagram)
	child := result.Scene.Nodes[0]
	parent := result.Scene.Nodes[1]
	if child.Code != "CHILD_CODE" || parent.Code != "PARENT_CODE" {
		t.Fatalf("node codes = %q/%q", child.Code, parent.Code)
	}
	if !reflect.DeepEqual(child.Fields, []Field{
		{Name: "Child ID", DataType: "BIGINT", PrimaryKey: true},
		{Name: "Parent Ref", DataType: "DECIMAL(10, 2)", ForeignKey: true},
	}) {
		t.Fatalf("child fields = %+v", child.Fields)
	}
	if !reflect.DeepEqual(parent.Fields, []Field{
		{Name: "Parent ID", DataType: "INTEGER", PrimaryKey: true},
		{Name: "Parent Name", DataType: "VARCHAR(64)"},
	}) {
		t.Fatalf("parent fields = %+v", parent.Fields)
	}
}

func TestMapDiagramDoesNotMarkColumnsFromOtherTables(t *testing.T) {
	tableA := pdm.Table{
		Metadata: pdm.Metadata{ID: "table-a", Code: "TABLE_A"},
		Columns: []pdm.Column{{
			Metadata: pdm.Metadata{ID: "column-a", Name: "Column A"},
			DataType: pdm.OptionalText{Value: "INTEGER", Present: true},
		}},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "key-a"},
			ColumnIDs: []string{"column-b"},
		}},
		PrimaryKeyID: "key-a",
	}
	tableB := pdm.Table{
		Metadata: pdm.Metadata{ID: "table-b", Code: "TABLE_B"},
		Columns: []pdm.Column{{
			Metadata: pdm.Metadata{ID: "column-b", Name: "Column B"},
			DataType: pdm.OptionalText{Value: "BIGINT", Present: true},
		}},
	}
	model := &pdm.Model{
		Metadata: pdm.Metadata{ID: "model"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{tableA, tableB},
			References: []pdm.Reference{{
				Metadata:      pdm.Metadata{ID: "bad-owner"},
				ChildTableID:  "table-a",
				ParentTableID: "table-b",
				Joins: []pdm.ReferenceJoin{{
					Metadata:       pdm.Metadata{ID: "bad-join"},
					ParentColumnID: "column-b",
					ChildColumnID:  "column-b",
				}},
			}},
		},
	}
	diagram := &pdm.Diagram{
		Metadata: pdm.Metadata{ID: "ownership"},
		OwnerID:  "model",
		TableSymbols: []pdm.TableSymbol{{
			Metadata: pdm.Metadata{ID: "a-symbol"},
			TableID:  "table-a",
			Rect:     mappingRect(0, 0, 10, 10),
		}},
	}

	result := MapDiagram(model, diagram)
	if len(result.Scene.Nodes) != 1 || len(result.Scene.Nodes[0].Fields) != 1 {
		t.Fatalf("ownership nodes = %+v", result.Scene.Nodes)
	}
	field := result.Scene.Nodes[0].Fields[0]
	if field.Name != "Column A" || field.PrimaryKey || field.ForeignKey {
		t.Fatalf("foreign column markers leaked across tables: %+v", field)
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
	if first.Scene.Nodes[0].Bounds != (Rect{Min: Point{X: -200, Y: -20}, Max: Point{X: -50, Y: 100}}) {
		t.Fatalf("valid geometry changed: %+v", first.Scene.Nodes[0].Bounds)
	}
	previous := first.Scene.Nodes[0].Bounds
	for offset, node := range first.Scene.Nodes[1:] {
		if node.Bounds.Min.X <= previous.Max.X || node.Bounds.Min.Y != -20 ||
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
