package pdm

import (
	"strings"
	"testing"
)

func TestParseDiagramsAcrossScopesAndNestedPackage(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="parent">
            <col:Columns><obj:Column Id="parent-col"/></col:Columns>
            <col:Keys>
              <obj:Key Id="parent-key">
                <col:Key.Columns><obj:Column Ref="parent-col"/></col:Key.Columns>
              </obj:Key>
            </col:Keys>
          </obj:Table>
          <obj:Table Id="child">
            <col:Columns><obj:Column Id="child-col"/></col:Columns>
            <col:Keys>
              <obj:Key Id="child-key">
                <col:Key.Columns><obj:Column Ref="child-col"/></col:Key.Columns>
              </obj:Key>
            </col:Keys>
          </obj:Table>
        </col:Tables>
        <col:References>
          <obj:Reference Id="logical-reference">
            <col:ParentTable><obj:Table Ref="parent"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="parent-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="logical-join">
                <col:Object1><obj:Column Ref="parent-col"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
        </col:References>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="diagram-one">
            <attr:Name>主画板</attr:Name>
            <col:Symbols>
              <obj:TableSymbol Id="parent-symbol-one">
                <col:Object><obj:Table Ref="parent"/></col:Object>
                <attr:Rect> (( -10, -20 ), ( 100, 200 )) </attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="child-symbol-one">
                <col:Object><obj:Table Ref="child"/></col:Object>
                <attr:Rect>((200,50),(300,150))</attr:Rect>
              </obj:TableSymbol>
              <obj:ReferenceSymbol Id="reference-symbol-one">
                <col:Object><obj:Reference Ref="logical-reference"/></col:Object>
                <col:SourceSymbol><obj:TableSymbol Ref="child-symbol-one"/></col:SourceSymbol>
                <col:DestinationSymbol><obj:TableSymbol Ref="parent-symbol-one"/></col:DestinationSymbol>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
                <attr:ListOfPoints> (( -5, 0 ), ( 0, -10 ), ( 20, 30 )) </attr:ListOfPoints>
                <attr:ControlPoints>((1,2),(3,4))</attr:ControlPoints>
                <attr:CornerStyle>rounded</attr:CornerStyle>
              </obj:ReferenceSymbol>
              <obj:RectangleSymbol Id="annotation-rectangle">
                <attr:Rect>((0,0),(5,5))</attr:Rect>
              </obj:RectangleSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
          <obj:PhysicalDiagram Id="diagram-two">
            <col:Symbols>
              <obj:TableSymbol Id="parent-symbol-two">
                <col:Object><obj:Table Ref="parent"/></col:Object>
                <attr:Rect>((50,60),(70,80))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>
        <col:DefaultDiagram><obj:PhysicalDiagram Ref="diagram-one"/></col:DefaultDiagram>
        <col:Packages>
          <obj:Package Id="nested-package">
            <col:Tables>
              <obj:Table Id="package-table">
                <col:Columns><obj:Column Id="package-column"/></col:Columns>
              </obj:Table>
            </col:Tables>
            <col:PhysicalDiagrams>
              <obj:PhysicalDiagram Id="package-diagram">
                <col:Symbols>
                  <obj:TableSymbol Id="package-table-symbol">
                    <col:Object><obj:Table Ref="package-table"/></col:Object>
                    <attr:Rect>((1,2),(3,4))</attr:Rect>
                  </obj:TableSymbol>
                </col:Symbols>
              </obj:PhysicalDiagram>
            </col:PhysicalDiagrams>
            <col:DefaultDiagram><obj:PhysicalDiagram Ref="package-diagram"/></col:DefaultDiagram>
          </obj:Package>
        </col:Packages>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", document.Diagnostics)
	}
	if len(document.Model.Contents.Diagrams) != 2 ||
		document.Model.Contents.DefaultDiagramID != "diagram-one" {
		t.Fatalf("root diagrams = %+v, default = %q",
			document.Model.Contents.Diagrams, document.Model.Contents.DefaultDiagramID)
	}

	one := diagramByID(t, document.Model.Contents, "diagram-one")
	two := diagramByID(t, document.Model.Contents, "diagram-two")
	if one.OwnerID != "model" || one.Name != "主画板" {
		t.Fatalf("first diagram = %+v", one)
	}
	if len(one.TableSymbols) != 2 || len(one.ReferenceSymbols) != 1 {
		t.Fatalf("first diagram symbols = %+v", one)
	}
	if first := one.TableSymbols[0]; first.TableID != "parent" ||
		first.Rect == nil || first.Rect.Min != (Point{X: -10, Y: -20}) ||
		first.Rect.Max != (Point{X: 100, Y: 200}) {
		t.Fatalf("first table symbol = %+v", first)
	}
	if second := two.TableSymbols[0]; second.TableID != "parent" ||
		second.ID == one.TableSymbols[0].ID ||
		second.Rect == nil || second.Rect.Min != (Point{X: 50, Y: 60}) {
		t.Fatalf("second diagram table symbol = %+v", second)
	}

	referenceSymbol := one.ReferenceSymbols[0]
	if referenceSymbol.ReferenceID != "logical-reference" ||
		referenceSymbol.SourceSymbolID != "child-symbol-one" ||
		referenceSymbol.DestinationSymbolID != "parent-symbol-one" {
		t.Fatalf("reference symbol endpoints = %+v", referenceSymbol)
	}
	wantPoints := []Point{{X: -5, Y: 0}, {X: 0, Y: -10}, {X: 20, Y: 30}}
	if !pointsEqual(referenceSymbol.Points, wantPoints) {
		t.Fatalf("path points = %+v", referenceSymbol.Points)
	}
	wantControl := []Point{{X: 1, Y: 2}, {X: 3, Y: 4}}
	if !pointsEqual(referenceSymbol.ControlPoints, wantControl) {
		t.Fatalf("control points = %+v", referenceSymbol.ControlPoints)
	}
	if value, ok := referenceSymbol.Raw.Property("CornerStyle"); !ok || value != "rounded" {
		t.Fatalf("corner style raw = %q, %v", value, ok)
	}
	if !hasRawObject(one.Raw, "RectangleSymbol") {
		t.Fatal("annotation RectangleSymbol raw node was not retained")
	}

	pkg := document.Model.Contents.Packages[0]
	if len(pkg.Contents.Diagrams) != 1 ||
		pkg.Contents.DefaultDiagramID != "package-diagram" ||
		pkg.Contents.Diagrams[0].OwnerID != "nested-package" {
		t.Fatalf("package diagrams = %+v", pkg.Contents)
	}
}

func TestParseRectangleAnnotationTextGeometryAndColors(t *testing.T) {
	body := `
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="annotation-diagram">
            <col:Symbols>
              <obj:RectangleSymbol Id="annotation">
                <attr:Text>第一行
第二行</attr:Text>
                <attr:Rect>((-20, 40), (120, -10))</attr:Rect>
                <attr:FillColor>0</attr:FillColor>
                <attr:LineColor>0</attr:LineColor>
                <attr:FontColor>16777215</attr:FontColor>
              </obj:RectangleSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	diagram := diagramByID(t, document.Model.Contents, "annotation-diagram")
	if len(diagram.AnnotationSymbols) != 1 {
		t.Fatalf("annotation symbols = %+v", diagram.AnnotationSymbols)
	}
	annotation := diagram.AnnotationSymbols[0]
	if annotation.ID != "annotation" ||
		annotation.Text != "第一行\n第二行" ||
		annotation.Rect == nil ||
		*annotation.Rect != (Rect{Min: Point{X: -20, Y: -10}, Max: Point{X: 120, Y: 40}}) {
		t.Fatalf("annotation = %+v", annotation)
	}
	assertAnnotationColor(t, "fill", annotation.Fill, RGBColor{})
	assertAnnotationColor(t, "border", annotation.Border, RGBColor{})
	assertAnnotationColor(t, "text", annotation.TextColor, RGBColor{R: 255, G: 255, B: 255})
}

func TestParsePointTuplesStrictGrammar(t *testing.T) {
	valid, ok := parsePointTuples(" (( -10, 20 ) , ( +30, -40 ), (0,0)) ", 2, 0)
	if !ok || !pointsEqual(valid, []Point{{X: -10, Y: 20}, {X: 30, Y: -40}, {X: 0, Y: 0}}) {
		t.Fatalf("valid points = %+v, %v", valid, ok)
	}
	if _, ok := parsePointTuples("((1,2),(3,4))", 2, 2); !ok {
		t.Fatal("rect cardinality rejected")
	}
	invalid := []string{
		"",
		"((1,2))",
		"((1,2),(3,4),(5,6))",
		"((1,2),(3,4)) trailing",
		"((1,2),(3,4),)",
		"((1 2),(3,4))",
		"((9223372036854775808,2),(3,4))",
		"((1,2)(3,4))",
	}
	for _, input := range invalid {
		if _, ok := parsePointTuples(input, 2, 2); ok {
			t.Fatalf("invalid tuple grammar accepted: %q", input)
		}
	}
}

func TestParseDiagramGeometryDiagnosticsRetainRaw(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="geometry-diagram">
            <col:Symbols>
              <obj:TableSymbol Id="missing-rect"><col:Object><obj:Table Ref="table"/></col:Object></obj:TableSymbol>
              <obj:TableSymbol Id="truncated-rect">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((1,2),(3))</attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="overflow-rect">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((1,2),(9223372036854775808,4))</attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="reverse-rect">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((3,4),(1,2))</attr:Rect>
              </obj:TableSymbol>
              <obj:ReferenceSymbol Id="bad-points">
                <attr:ListOfPoints>((1,2))</attr:ListOfPoints>
              </obj:ReferenceSymbol>
              <obj:ReferenceSymbol Id="bad-control">
                <attr:ListOfPoints>((1,2),(3,4))</attr:ListOfPoints>
                <attr:ControlPoints>((1,2),)</attr:ControlPoints>
              </obj:ReferenceSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticMissingGeometry) ||
		!hasDiagnostic(document.Diagnostics, DiagnosticInvalidGeometry) {
		t.Fatalf("geometry diagnostics absent: %+v", document.Diagnostics)
	}
	if diagnosticCount(document.Diagnostics, DiagnosticInvalidGeometry) < 5 {
		t.Fatalf("invalid geometry count = %+v", document.Diagnostics)
	}

	diagram := diagramByID(t, document.Model.Contents, "geometry-diagram")
	truncated := tableSymbolByID(t, diagram, "truncated-rect")
	if truncated.Rect != nil {
		t.Fatalf("invalid rect was populated: %+v", truncated.Rect)
	}
	if value, ok := truncated.Raw.Property("Rect"); !ok || value != "((1,2),(3))" {
		t.Fatalf("invalid rect raw text = %q, %v", value, ok)
	}
	if missing := tableSymbolByID(t, diagram, "missing-rect"); missing.Rect != nil {
		t.Fatalf("missing rect was fabricated: %+v", missing.Rect)
	}
	badControl := referenceSymbolByID(t, diagram, "bad-control")
	if len(badControl.Points) != 2 || badControl.ControlPoints != nil {
		t.Fatalf("bad control points = %+v", badControl)
	}
}

func TestParseDiagramScopeAndEndpointDiagnostics(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="parent">
            <col:Columns><obj:Column Id="parent-col"/></col:Columns>
            <col:Keys><obj:Key Id="parent-key"><col:Key.Columns><obj:Column Ref="parent-col"/></col:Key.Columns></obj:Key></col:Keys>
          </obj:Table>
          <obj:Table Id="child">
            <col:Columns><obj:Column Id="child-col"/></col:Columns>
            <col:Keys><obj:Key Id="child-key"><col:Key.Columns><obj:Column Ref="child-col"/></col:Key.Columns></obj:Key></col:Keys>
          </obj:Table>
        </col:Tables>
        <col:References>
          <obj:Reference Id="logical-reference">
            <col:ParentTable><obj:Table Ref="parent"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="parent-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="logical-join">
                <col:Object1><obj:Column Ref="parent-col"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
        </col:References>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="diagram-one">
            <col:Symbols>
              <obj:TableSymbol Id="parent-symbol">
                <col:Object><obj:Table Ref="parent"/></col:Object><attr:Rect>((0,0),(1,1))</attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="child-symbol">
                <col:Object><obj:Table Ref="child"/></col:Object><attr:Rect>((2,2),(3,3))</attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="external-symbol">
                <col:Object><obj:Shortcut Ref="external-table"/></col:Object><attr:Rect>((4,4),(5,5))</attr:Rect>
              </obj:TableSymbol>
              <obj:ReferenceSymbol Id="wrong-diagram-endpoint">
                <col:Object><obj:Reference Ref="logical-reference"/></col:Object>
                <col:SourceSymbol><obj:TableSymbol Ref="external-symbol"/></col:SourceSymbol>
                <col:DestinationSymbol><obj:TableSymbol Ref="symbol-in-other-diagram"/></col:DestinationSymbol>
                <attr:ListOfPoints>((0,0),(1,1))</attr:ListOfPoints>
              </obj:ReferenceSymbol>
              <obj:ReferenceSymbol Id="swapped-endpoints">
                <col:Object><obj:Reference Ref="logical-reference"/></col:Object>
                <col:SourceSymbol><obj:TableSymbol Ref="parent-symbol"/></col:SourceSymbol>
                <col:DestinationSymbol><obj:TableSymbol Ref="child-symbol"/></col:DestinationSymbol>
                <attr:ListOfPoints>((0,0),(1,1))</attr:ListOfPoints>
              </obj:ReferenceSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
          <obj:PhysicalDiagram Id="diagram-two">
            <col:Symbols>
              <obj:TableSymbol Id="symbol-in-other-diagram">
                <col:Object><obj:Table Ref="child"/></col:Object><attr:Rect>((0,0),(1,1))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>
        <col:DefaultDiagram><obj:PhysicalDiagram Ref="package-diagram"/></col:DefaultDiagram>
        <col:Packages>
          <obj:Package Id="package">
            <col:PhysicalDiagrams>
              <obj:PhysicalDiagram Id="package-diagram"><col:Symbols/></obj:PhysicalDiagram>
            </col:PhysicalDiagrams>
          </obj:Package>
        </col:Packages>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticWrongDiagramScope) {
		t.Fatalf("wrong diagram scope absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticReferenceMismatch) {
		t.Fatalf("reference mismatch absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticExternalReference) {
		t.Fatalf("external table warning absent: %+v", document.Diagnostics)
	}
	if document.Model.Contents.DefaultDiagramID != "package-diagram" {
		t.Fatalf("default diagram ID = %q", document.Model.Contents.DefaultDiagramID)
	}

	diagram := diagramByID(t, document.Model.Contents, "diagram-one")
	external := tableSymbolByID(t, diagram, "external-symbol")
	if external.TableID != "external-table" {
		t.Fatalf("external shortcut was dereferenced: %+v", external)
	}
}

func pointsEqual(got, want []Point) bool {
	if len(got) != len(want) {
		return false
	}
	for offset := range got {
		if got[offset] != want[offset] {
			return false
		}
	}
	return true
}

func diagramByID(t *testing.T, contents Contents, id string) Diagram {
	t.Helper()
	if diagram, ok := diagramLookup(contents, id); ok {
		return diagram
	}
	t.Fatalf("diagram %q not found", id)
	return Diagram{}
}

func diagramLookup(contents Contents, id string) (Diagram, bool) {
	for _, diagram := range contents.Diagrams {
		if diagram.ID == id {
			return diagram, true
		}
	}
	for _, pkg := range contents.Packages {
		if diagram, ok := diagramLookup(pkg.Contents, id); ok {
			return diagram, true
		}
	}
	return Diagram{}, false
}

func tableSymbolByID(t *testing.T, diagram Diagram, id string) TableSymbol {
	t.Helper()
	for _, symbol := range diagram.TableSymbols {
		if symbol.ID == id {
			return symbol
		}
	}
	t.Fatalf("table symbol %q not found", id)
	return TableSymbol{}
}

func referenceSymbolByID(t *testing.T, diagram Diagram, id string) ReferenceSymbol {
	t.Helper()
	for _, symbol := range diagram.ReferenceSymbols {
		if symbol.ID == id {
			return symbol
		}
	}
	t.Fatalf("reference symbol %q not found", id)
	return ReferenceSymbol{}
}

func hasRawObject(element *Element, local string) bool {
	if element == nil {
		return false
	}
	if element.Name.Space == NamespaceObject && element.Name.Local == local {
		return true
	}
	for _, child := range element.Children {
		if hasRawObject(child, local) {
			return true
		}
	}
	return false
}

func assertAnnotationColor(t *testing.T, name string, got *RGBColor, want RGBColor) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s color = %+v, want %+v", name, got, want)
	}
}
