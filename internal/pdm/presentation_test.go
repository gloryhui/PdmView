package pdm

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTableAppearanceDefaultsAndSymbolOverrides(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="parent"><col:Columns><obj:Column Id="parent-col"/></col:Columns></obj:Table>
          <obj:Table Id="child"><col:Columns><obj:Column Id="child-col"/></col:Columns></obj:Table>
        </col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="appearance">
            <attr:DisplayPreferences><![CDATA[
[DisplayPreferences\Symbol\TABL]
Brush color=255 240 220
Fill Color=Yes
Pen=1 0 0 128 192
Brush gradient mode=64
Brush gradient color=250 230 200
*Font color=10 20 30
DISPNAMEFont color=40 50 60
TablePkColumnsFont=Arial U
[DisplayPreferences\Object]
Table.DisplayName=1
Table.Columns=1
Table.Columns._Filter="All Columns" PDMCOLNALL
Table.Columns._Columns=Stereotype DataType KeyIndicator NullStatus
Table.Columns._Limit=5
Table.Stereotype=1
Table.Comment=1
TablColn=No
ColnMode=0
ColnMax=9
ColnDttp=No
ColnKey=No
ColnMand=No
TablStrn=No
TablCmmt=No
[DisplayPreferences\Symbol\VIEW]
Brush color=1 2 3
Pen=1 0 0 1 2 3
DISPNAMEFont color=1 2 3
]]></attr:DisplayPreferences>
            <col:Symbols>
              <obj:TableSymbol Id="parent-symbol">
                <col:Object><obj:Table Ref="parent"/></col:Object>
                <attr:Rect>((0,0),(100,100))</attr:Rect>
                <attr:FillColor>65535</attr:FillColor>
                <attr:LineColor>255</attr:LineColor>
                <attr:GradientEndColor>65280</attr:GradientEndColor>
                <attr:GradientFillMode>65</attr:GradientFillMode>
                <attr:FontList><![CDATA[
DISPNAME 255 Arial
Columns 65280 Arial
TableFkColumns 0 Arial
TablePkColumns 16711680 Arial N
]]></attr:FontList>
                <attr:SymbolContent><![CDATA[
Table.DisplayName 0
Table.Columns 1 3 "Primary Only" PDMCOLNPK Stereotype DataType
]]></attr:SymbolContent>
              </obj:TableSymbol>
              <obj:TableSymbol Id="child-symbol">
                <col:Object><obj:Table Ref="child"/></col:Object>
                <attr:Rect>((200,0),(300,100))</attr:Rect>
              </obj:TableSymbol>
              <obj:TableSymbol Id="bad-symbol">
                <col:Object><obj:Table Ref="child"/></col:Object>
                <attr:Rect>((400,0),(500,100))</attr:Rect>
                <attr:FillColor>-1</attr:FillColor>
                <attr:GradientEndColor>16777216</attr:GradientEndColor>
                <attr:GradientFillMode>bad</attr:GradientFillMode>
                <attr:SymbolContent>Table.Columns broken</attr:SymbolContent>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", document.Diagnostics)
	}
	diagram := diagramByID(t, document.Model.Contents, "appearance")
	if raw, ok := diagram.Raw.Property("DisplayPreferences"); !ok || !strings.Contains(raw, "Brush color=255 240 220") {
		t.Fatalf("DisplayPreferences raw text was not retained")
	}
	assertColor(t, "fill", diagram.TableAppearance.Fill, RGBColor{R: 255, G: 240, B: 220})
	assertColor(t, "border", diagram.TableAppearance.Border, RGBColor{R: 0, G: 128, B: 192})
	assertColor(t, "gradient end", diagram.TableAppearance.GradientEnd, RGBColor{R: 250, G: 230, B: 200})
	if diagram.TableAppearance.GradientMode != 64 || diagram.TableAppearance.Transparent {
		t.Fatalf("diagram appearance = %+v", diagram.TableAppearance)
	}
	if !diagram.TableAppearance.PrimaryUnderline {
		t.Fatal("default TablePkColumnsFont U was not parsed")
	}
	assertColor(t, "title color", diagram.TableAppearance.TitleColor, RGBColor{R: 40, G: 50, B: 60})
	assertColor(t, "column color", diagram.TableAppearance.ColumnColor, RGBColor{R: 10, G: 20, B: 30})
	assertColor(t, "primary color", diagram.TableAppearance.PrimaryColor, RGBColor{R: 10, G: 20, B: 30})
	assertColor(t, "foreign color", diagram.TableAppearance.ForeignColor, RGBColor{R: 10, G: 20, B: 30})
	wantDisplay := TableDisplay{
		ShowName:             true,
		ShowColumns:          true,
		ShowDataType:         true,
		ShowKeyIndicator:     true,
		ShowNullStatus:       true,
		ShowStereotype:       true,
		ShowColumnStereotype: true,
		ShowComment:          true,
		ColumnFilter:         ColumnFilterAll,
		ColumnLimit:          5,
	}
	if diagram.TableAppearance.Display == nil || *diagram.TableAppearance.Display != wantDisplay {
		t.Fatalf("diagram display = %+v, want %+v", diagram.TableAppearance.Display, wantDisplay)
	}

	parent := tableSymbolByID(t, diagram, "parent-symbol")
	assertColor(t, "symbol fill", parent.Appearance.Fill, RGBColor{R: 255, G: 255, B: 0})
	assertColor(t, "symbol border", parent.Appearance.Border, RGBColor{R: 255, G: 0, B: 0})
	assertColor(t, "symbol gradient end", parent.Appearance.GradientEnd, RGBColor{R: 0, G: 255, B: 0})
	if parent.Appearance.GradientMode != 65 {
		t.Fatalf("symbol gradient mode = %d", parent.Appearance.GradientMode)
	}
	if parent.Appearance.PrimaryUnderline {
		t.Fatal("symbol TablePkColumns N did not override default U")
	}
	assertColor(t, "symbol title", parent.Appearance.TitleColor, RGBColor{R: 255, G: 0, B: 0})
	assertColor(t, "symbol column", parent.Appearance.ColumnColor, RGBColor{R: 0, G: 255, B: 0})
	assertColor(t, "symbol primary", parent.Appearance.PrimaryColor, RGBColor{R: 0, G: 0, B: 255})
	assertColor(t, "symbol foreign", parent.Appearance.ForeignColor, RGBColor{})
	wantParentDisplay := wantDisplay
	wantParentDisplay.ShowName = false
	wantParentDisplay.ColumnLimit = 3
	wantParentDisplay.ColumnFilter = ColumnFilterPrimary
	wantParentDisplay.ShowKeyIndicator = false
	wantParentDisplay.ShowNullStatus = false
	if parent.Appearance.Display == nil || *parent.Appearance.Display != wantParentDisplay {
		t.Fatalf("parent display = %+v, want %+v", parent.Appearance.Display, wantParentDisplay)
	}

	child := tableSymbolByID(t, diagram, "child-symbol")
	if !reflect.DeepEqual(child.Appearance, diagram.TableAppearance) {
		t.Fatalf("symbol without overrides = %+v, want %+v", child.Appearance, diagram.TableAppearance)
	}
	if !child.Appearance.PrimaryUnderline {
		t.Fatal("symbol without overrides lost default underline")
	}
	bad := tableSymbolByID(t, diagram, "bad-symbol")
	if !reflect.DeepEqual(bad.Appearance, diagram.TableAppearance) {
		t.Fatalf("bad symbol content changed appearance: %+v", bad.Appearance)
	}
	if raw, ok := bad.Raw.Property("FillColor"); !ok || raw != "-1" {
		t.Fatalf("bad symbol fill raw property was not retained")
	}
	if raw, ok := bad.Raw.Property("GradientEndColor"); !ok || raw != "16777216" {
		t.Fatalf("bad symbol gradient raw property was not retained")
	}
}

func TestParsePrimaryUnderlineCommaFlags(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="underline">
            <attr:DisplayPreferences><![CDATA[
[DisplayPreferences\Symbol\TABL]
TablePkColumnsFont=Microsoft Sans Serif,8,U
[DisplayPreferences\Object]
Table.Columns=1
]]></attr:DisplayPreferences>
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
                <attr:FontList>TablePkColumns 0 新宋体,8,N</attr:FontList>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	diagram := diagramByID(t, document.Model.Contents, "underline")
	if !diagram.TableAppearance.PrimaryUnderline {
		t.Fatal("comma-separated TablePkColumnsFont U was not parsed")
	}
	symbol := tableSymbolByID(t, diagram, "symbol")
	if symbol.Appearance.PrimaryUnderline {
		t.Fatal("comma-separated symbol N did not override default U")
	}
}

func TestParseTableAppearanceOldKeysAndUnknownMode(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="old">
            <attr:DisplayPreferences><![CDATA[
[DisplayPreferences\Object]
TablColn=Yes
ColnMode=2
ColnMax=7
ColnDttp=Yes
ColnKey=No
ColnMand=Yes
TablStrn=No
TablCmmt=Yes
Table.Columns._Limit=-5
]]></attr:DisplayPreferences>
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	diagram := diagramByID(t, document.Model.Contents, "old")
	display := diagram.TableAppearance.Display
	if display == nil {
		t.Fatal("old display keys did not create a display")
	}
	if !display.ShowColumns || display.ColumnLimit != -5 ||
		!display.ShowDataType || display.ShowKeyIndicator || !display.ShowNullStatus ||
		display.ShowStereotype || !display.ShowComment {
		t.Fatalf("old display = %+v", display)
	}
	if display.ColumnFilter != "" {
		t.Fatalf("unknown ColnMode guessed a filter: %q", display.ColumnFilter)
	}
}

func TestParseTableAppearanceAllColumnsIgnoresColnMax(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="all">
            <attr:DisplayPreferences><![CDATA[
[DisplayPreferences\Object]
ColnMode=0
ColnMax=5
]]></attr:DisplayPreferences>
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	display := diagramByID(t, document.Model.Contents, "all").TableAppearance.Display
	if display == nil || !display.ShowName || !display.ShowColumns ||
		!display.ShowDataType || !display.ShowKeyIndicator ||
		display.ColumnFilter != ColumnFilterAll || display.ColumnLimit != 0 {
		t.Fatalf("all-columns display = %+v", display)
	}
}

func TestParseModernDisplayPartialDefaults(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="partial">
            <attr:DisplayPreferences><![CDATA[
[DisplayPreferences\Object]
Table.Columns._Filter=PDMCOLNPK
]]></attr:DisplayPreferences>
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
                <attr:SymbolContent>Table.DisplayName 0</attr:SymbolContent>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	display := tableSymbolByID(t, diagramByID(t, document.Model.Contents, "partial"), "symbol").Appearance.Display
	if display == nil || display.ShowName || !display.ShowColumns ||
		!display.ShowDataType || !display.ShowKeyIndicator ||
		display.ColumnFilter != ColumnFilterPrimary || display.ColumnLimit != 0 {
		t.Fatalf("partial display = %+v", display)
	}
}

func TestParseSymbolDisplayPartialDefaults(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="partial-symbol">
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
                <attr:SymbolContent>Table.DisplayName 0</attr:SymbolContent>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	display := tableSymbolByID(t, diagramByID(t, document.Model.Contents, "partial-symbol"), "symbol").Appearance.Display
	if display == nil || display.ShowName || !display.ShowColumns ||
		!display.ShowDataType || !display.ShowKeyIndicator ||
		display.ColumnFilter != ColumnFilterAll || display.ColumnLimit != 0 {
		t.Fatalf("partial symbol display = %+v", display)
	}
}

func TestParsePresentationAbsent(t *testing.T) {
	body := `
        <col:Tables><obj:Table Id="table"><col:Columns><obj:Column Id="col"/></col:Columns></obj:Table></col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="plain">
            <col:Symbols>
              <obj:TableSymbol Id="symbol">
                <col:Object><obj:Table Ref="table"/></col:Object>
                <attr:Rect>((0,0),(10,10))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	diagram := diagramByID(t, document.Model.Contents, "plain")
	if !reflect.DeepEqual(diagram.TableAppearance, TableAppearance{}) {
		t.Fatalf("empty diagram appearance = %+v", diagram.TableAppearance)
	}
	symbol := tableSymbolByID(t, diagram, "symbol")
	if !reflect.DeepEqual(symbol.Appearance, TableAppearance{}) {
		t.Fatalf("empty symbol appearance = %+v", symbol.Appearance)
	}
}

func TestParseStereotypePropertiesKnownAndRetained(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="table">
            <attr:Stereotype>TableStereotype</attr:Stereotype>
            <col:Columns>
              <obj:Column Id="col"><attr:Stereotype>ColumnStereotype</attr:Stereotype></obj:Column>
            </col:Columns>
          </obj:Table>
        </col:Tables>`

	raw, err := Decode(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := raw.ByID["table"].Property("Stereotype"); !ok || value != "TableStereotype" {
		t.Fatalf("raw table stereotype = %q, %v", value, ok)
	}
	if value, ok := raw.ByID["col"].Property("Stereotype"); !ok || value != "ColumnStereotype" {
		t.Fatalf("raw column stereotype = %q, %v", value, ok)
	}
	if hasDiagnostic(raw.Diagnostics, DiagnosticUnknownNode) {
		t.Fatalf("Stereotype was reported unknown: %+v", raw.Diagnostics)
	}
}

func TestParsePresentationColorsAndFilters(t *testing.T) {
	for _, test := range []struct {
		value string
		want  RGBColor
	}{
		{"0", RGBColor{}},
		{"255", RGBColor{R: 255}},
		{"65280", RGBColor{G: 255}},
		{"16711680", RGBColor{B: 255}},
		{"65535", RGBColor{R: 255, G: 255}},
	} {
		color, ok := parseColorRef(test.value)
		if !ok || *color != test.want {
			t.Fatalf("COLORREF %s = %+v, %v; want %+v", test.value, color, ok, test.want)
		}
	}
	for _, value := range []string{"-1", "16777216"} {
		if _, ok := parseColorRef(value); ok {
			t.Fatalf("invalid/system COLORREF %s was accepted", value)
		}
	}
	for _, test := range []struct {
		fields []string
		want   bool
	}{
		{[]string{"Arial", "U"}, true},
		{[]string{"Microsoft", "Sans", "Serif,8,U"}, true},
		{[]string{"新宋体,8,N"}, false},
	} {
		got, ok := trailingUnderline(test.fields)
		if !ok || got != test.want {
			t.Fatalf("underline flags %v = %v, %v; want %v", test.fields, got, ok, test.want)
		}
	}
	for _, test := range []struct {
		value string
		last  bool
		want  RGBColor
	}{
		{"255 240 220", false, RGBColor{R: 255, G: 240, B: 220}},
		{"255, 240, 220", false, RGBColor{R: 255, G: 240, B: 220}},
		{"1 0 0 128 192", true, RGBColor{R: 0, G: 128, B: 192}},
	} {
		color, ok := parseRGBFields(test.value, test.last)
		if !ok || *color != test.want {
			t.Fatalf("RGB %q = %+v, %v; want %+v", test.value, color, ok, test.want)
		}
	}
	for _, test := range []struct {
		value string
		want  string
	}{
		{`""PDMCOLNALL`, ColumnFilterAll},
		{`"All Columns" PDMCOLNALL`, ColumnFilterAll},
		{"PDMCOLNPK", ColumnFilterPrimary},
		{"PDMCOLNKEY", ColumnFilterKey},
		{`"All Columns" PDMCOLNPK`, ColumnFilterPrimary},
	} {
		got, ok := parseColumnFilter(test.value)
		if !ok || got != test.want {
			t.Fatalf("filter %q = %q, %v; want %q", test.value, got, ok, test.want)
		}
	}
	if _, ok := parseColumnFilter("unknown"); ok {
		t.Fatal("unknown filter was accepted")
	}
}

func assertColor(t *testing.T, name string, got *RGBColor, want RGBColor) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s color = %+v, want %+v", name, got, want)
	}
}
