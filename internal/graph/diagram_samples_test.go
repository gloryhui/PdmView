//go:build pdm_layout_check

package graph

import (
	"encoding/xml"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"pdmview/internal/pdm"
)

type sampleContents struct {
	Tables   []sampleTable   `xml:"Tables>Table"`
	Diagrams []sampleDiagram `xml:"PhysicalDiagrams>PhysicalDiagram"`
	Packages []samplePackage `xml:"Packages>Package"`
}

type samplePackage struct {
	ID string `xml:"Id,attr"`
	sampleContents
}

type sampleTable struct {
	ID      string         `xml:"Id,attr"`
	Name    string         `xml:"Name"`
	Code    string         `xml:"Code"`
	Columns []sampleColumn `xml:"Columns>Column"`
}

type sampleColumn struct {
	Name string `xml:"Name"`
	Code string `xml:"Code"`
}

type sampleDiagram struct {
	ID     string                  `xml:"Id,attr"`
	Tables []sampleTableSymbol     `xml:"Symbols>TableSymbol"`
	Edges  []sampleReferenceSymbol `xml:"Symbols>ReferenceSymbol"`
}

type sampleTarget struct {
	Ref string `xml:"Ref,attr"`
}

type sampleTableSymbol struct {
	ID       string       `xml:"Id,attr"`
	Table    sampleTarget `xml:"Object>Table"`
	Shortcut sampleTarget `xml:"Object>Shortcut"`
	Rect     string       `xml:"Rect"`
}

type sampleReferenceSymbol struct {
	ID       string       `xml:"Id,attr"`
	Target   sampleTarget `xml:"Object>Reference"`
	Source   sampleTarget `xml:"SourceSymbol>TableSymbol"`
	Dest     sampleTarget `xml:"DestinationSymbol>TableSymbol"`
	Rect     string       `xml:"Rect"`
	Points   string       `xml:"ListOfPoints"`
	Controls string       `xml:"ControlPoints"`
}

type sampleLayoutCounts struct {
	diagrams, nodes, fields, edges, points, controls, shortcuts, undisplayedTables int
}

var samplePointPair = regexp.MustCompile(`\(\s*([+-]?[0-9]+)\s*,\s*([+-]?[0-9]+)\s*\)`)

func TestDiagramXMLSample(t *testing.T) {
	path := os.Getenv("PDM_LAYOUT_SAMPLE")
	if path == "" {
		t.Skip("set PDM_LAYOUT_SAMPLE to a local XML export")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("could not read local sample")
	}
	var source struct {
		Model sampleContents `xml:"RootObject>Children>Model"`
	}
	if err := xml.Unmarshal(raw, &source); err != nil {
		t.Fatal("could not independently decode source XML")
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal("could not open local sample for parser")
	}
	defer input.Close()
	document, err := pdm.Parse(input)
	if err != nil {
		t.Fatal("PDM parser rejected sample")
	}
	tables := make(map[string]sampleTable)
	indexSampleTables(tables, source.Model)
	counts := sampleLayoutCounts{}
	shownTables := make(map[string]bool)
	checkSampleContents(t, source.Model, &document.Model, document.Model.Contents, tables, shownTables, &counts)
	if counts.diagrams == 0 || counts.nodes == 0 {
		t.Fatal("expected at least one source diagram and table symbol")
	}
	for id := range tables {
		if !shownTables[id] {
			counts.undisplayedTables++
		}
	}
	t.Logf("XML parity: diagrams=%d table_symbols=%d displayed_fields=%d off_diagram_tables=%d shortcut_symbols=%d relationship_symbols=%d ordered_points=%d explicit_control_points=%d parser_diagnostics=%d",
		counts.diagrams, counts.nodes, counts.fields, counts.undisplayedTables, counts.shortcuts,
		counts.edges, counts.points, counts.controls, len(document.Diagnostics))
}

func indexSampleTables(dst map[string]sampleTable, contents sampleContents) {
	for _, table := range contents.Tables {
		dst[table.ID] = table
	}
	for _, pkg := range contents.Packages {
		indexSampleTables(dst, pkg.sampleContents)
	}
}

func checkSampleContents(t *testing.T, source sampleContents, model *pdm.Model, parsed pdm.Contents, tables map[string]sampleTable, shown map[string]bool, counts *sampleLayoutCounts) {
	t.Helper()
	if len(source.Diagrams) != len(parsed.Diagrams) || len(source.Packages) != len(parsed.Packages) {
		t.Fatal("diagram or package count differs from XML")
	}
	for index, want := range source.Diagrams {
		mapped := MapDiagram(model, &parsed.Diagrams[index])
		counts.diagrams++
		checkSampleDiagram(t, want, mapped, tables, shown, counts)
	}
	for index, pkg := range source.Packages {
		if pkg.ID != parsed.Packages[index].ID {
			t.Fatalf("package %d: ID differs from XML", index+1)
		}
		checkSampleContents(t, pkg.sampleContents, model, parsed.Packages[index].Contents, tables, shown, counts)
	}
}

func checkSampleDiagram(t *testing.T, source sampleDiagram, mapped DiagramGraph, tables map[string]sampleTable, shown map[string]bool, counts *sampleLayoutCounts) {
	t.Helper()
	if mapped.DiagramID != source.ID || mapped.AutoLayout ||
		len(mapped.Scene.Nodes) != len(source.Tables) || len(mapped.NodeSources) != len(source.Tables) ||
		len(mapped.Scene.Edges) != len(source.Edges) || len(mapped.EdgeSources) != len(source.Edges) {
		t.Fatalf("diagram %d: ID, layout mode, or symbol counts differ from XML", counts.diagrams)
	}
	offsets := make(map[string]int, len(source.Tables))
	details := os.Getenv("PDM_LAYOUT_DETAILS") == "1"
	for index, symbol := range source.Tables {
		offsets[symbol.ID] = index
		node, origin := mapped.Scene.Nodes[index], mapped.NodeSources[index]
		tableID := symbol.Table.Ref
		if symbol.Shortcut.Ref != "" {
			tableID = symbol.Shortcut.Ref
			counts.shortcuts++
		}
		rect := sampleRect(t, symbol.Rect)
		minimum, maximum := sampleWorldPoint(rect.Min), sampleWorldPoint(rect.Max)
		width, height := maximum.X-minimum.X, maximum.Y-minimum.Y
		if origin.SymbolID != symbol.ID || origin.TableID != tableID || node.ID != index ||
			!reflect.DeepEqual(origin.Rect, rect) || node.Bounds != (Rect{Min: minimum, Max: maximum}) ||
			node.Bounds.Width() != width || node.Bounds.Height() != height {
			t.Fatalf("diagram %d node %d: IDs, corners, position, or size differ from XML", counts.diagrams, index+1)
		}
		if table, ok := tables[tableID]; ok {
			title := table.Name
			if title == "" {
				title = table.Code
			}
			if node.Name != title || len(node.Fields) != len(table.Columns) {
				t.Fatalf("diagram %d node %d: title or field count differs from XML", counts.diagrams, index+1)
			}
			for columnIndex, column := range table.Columns {
				name := column.Name
				if name == "" {
					name = column.Code
				}
				if node.Fields[columnIndex].Name != name {
					t.Fatalf("diagram %d node %d column %d: field order or name differs from XML", counts.diagrams, index+1, columnIndex+1)
				}
			}
			shown[tableID] = true
		}
		counts.nodes++
		counts.fields += len(node.Fields)
		if details {
			t.Logf("diagram=%s symbol=%s table=%s x=%d y=%d width=%d height=%d fields=%d",
				source.ID, symbol.ID, tableID, rect.Min.X, rect.Min.Y, rect.Max.X-rect.Min.X, rect.Max.Y-rect.Min.Y, len(node.Fields))
		}
	}
	for index, symbol := range source.Edges {
		edge, origin := mapped.Scene.Edges[index], mapped.EdgeSources[index]
		points, controls := samplePoints(t, symbol.Points), samplePoints(t, symbol.Controls)
		from, fromOK := offsets[symbol.Source.Ref]
		to, toOK := offsets[symbol.Dest.Ref]
		if origin.SymbolID != symbol.ID || origin.ReferenceID != symbol.Target.Ref ||
			origin.SourceSymbolID != symbol.Source.Ref || origin.DestinationSymbolID != symbol.Dest.Ref ||
			!reflect.DeepEqual(origin.Rect, sampleRect(t, symbol.Rect)) ||
			!slices.Equal(origin.Points, points) || !slices.Equal(origin.ControlPoints, controls) ||
			edge.ID != index || !fromOK || edge.From != from || !toOK || edge.To != to || len(edge.Points) != len(points) {
			t.Fatalf("diagram %d edge %d: IDs, endpoints, or geometry differ from XML", counts.diagrams, index+1)
		}
		for pointIndex, point := range points {
			if edge.Points[pointIndex] != sampleWorldPoint(point) {
				t.Fatalf("diagram %d edge %d point %d: world path differs from XML", counts.diagrams, index+1, pointIndex+1)
			}
		}
		counts.edges++
		counts.points += len(points)
		counts.controls += len(controls)
		if details {
			t.Logf("diagram=%s symbol=%s reference=%s source=%s destination=%s path=%v controls=%v",
				source.ID, symbol.ID, symbol.Target.Ref, symbol.Source.Ref, symbol.Dest.Ref, points, controls)
		}
	}
}

func sampleRect(t *testing.T, text string) *pdm.Rect {
	t.Helper()
	points := samplePoints(t, text)
	if len(points) == 0 {
		return nil
	}
	if len(points) != 2 {
		t.Fatal("expected two XML rectangle corners")
	}
	return &pdm.Rect{Min: points[0], Max: points[1]}
}

func samplePoints(t *testing.T, text string) []pdm.Point {
	t.Helper()
	var points []pdm.Point
	for _, pair := range samplePointPair.FindAllStringSubmatch(text, -1) {
		x, xerr := strconv.ParseInt(pair[1], 10, 64)
		y, yerr := strconv.ParseInt(pair[2], 10, 64)
		if xerr != nil || yerr != nil {
			t.Fatal("sample coordinate is out of range")
		}
		points = append(points, pdm.Point{X: x, Y: y})
	}
	if text != "" && len(points) == 0 {
		t.Fatal("expected XML coordinate pairs")
	}
	return points
}

func sampleWorldPoint(point pdm.Point) Point {
	return Point{X: float64(point.X), Y: float64(point.Y)}
}
