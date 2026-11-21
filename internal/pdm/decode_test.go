package pdm

import (
	"errors"
	"strings"
	"testing"
)

const testHeader = `<?xml version="1.0" encoding="UTF-8"?>
<?PowerDesigner version="16.5.0.3982" signature="PDM_DATA_MODEL_XML"?>
`

func TestDecodePreservesNamespacesTextAndHelpers(t *testing.T) {
	var body strings.Builder
	body.WriteString(testHeader)
	body.WriteString(`<Model xmlns:obj="object" xmlns:col="collection" xmlns:attr="attribute">
  <obj:RootObject Id="root-id">
    <col:Children>
      <obj:Model Id="model-id">
        <attr:Name>  模型 &amp; 名称
</attr:Name>
        <attr:Comment></attr:Comment>
        <attr:Code attr:Flag="yes" Plain=" "/>
        <col:Tables>
          <obj:Table Id="t1"><attr:Code>orders</attr:Code></obj:Table>
        </col:Tables>
      </obj:Model>
    </col:Children>
  </obj:RootObject>
</Model>`)

	doc, err := Decode(strings.NewReader(body.String()))
	if err != nil {
		t.Fatal(err)
	}

	if doc.Format.PowerDesignerVersion != "16.5.0.3982" ||
		doc.Format.Signature != PDMSignature ||
		doc.Format.XMLVersion != "1.0" ||
		doc.Format.Encoding != "UTF-8" {
		t.Fatalf("format = %+v", doc.Format)
	}
	if doc.Root == nil || doc.Root.Name.Local != "Model" || doc.Root.Name.Space != "" {
		t.Fatalf("root = %+v", doc.Root)
	}
	if doc.Model == nil || doc.Model.Name.Space != NamespaceObject || doc.Model.Name.Local != "Model" {
		t.Fatalf("model = %+v", doc.Model)
	}
	if doc.Root.Path != "/Model" ||
		doc.Model.Path != "/Model/object:RootObject/collection:Children/object:Model" {
		t.Fatalf("paths root=%q model=%q", doc.Root.Path, doc.Model.Path)
	}

	if got := doc.Model.Child(NamespaceCollection, "Tables"); got == nil {
		t.Fatal("missing collection:Tables alias child")
	}
	if got, ok := doc.Model.Child(NamespaceCollection, "Tables").Child(NamespaceObject, "Table").Property("Code"); !ok || got != "orders" {
		t.Fatalf("table code = %q, %v", got, ok)
	}
	if got, ok := doc.Model.Property("Name"); !ok || got != "  模型 & 名称\n" {
		t.Fatalf("name text = %q, %v", got, ok)
	}
	if got, ok := doc.Model.Property("Comment"); !ok || got != "" {
		t.Fatalf("blank property = %q, %v", got, ok)
	}
	if _, ok := doc.Model.Property("Missing"); ok {
		t.Fatal("missing property reported present")
	}

	optional := doc.Model.Child(NamespaceAttribute, "Code")
	if optional == nil {
		t.Fatal("missing optional element")
	}
	if _, ok := optional.Attribute("Flag"); ok || len(optional.Attr) != 2 ||
		optional.Attr[0].Name.Space != NamespaceAttribute || optional.Attr[0].Value != "yes" {
		t.Fatalf("raw namespace attribute was not retained separately: %+v", optional.Attr)
	}
	if got, ok := optional.Attribute("Plain"); !ok || got != " " {
		t.Fatalf("blank attribute = %q, %v", got, ok)
	}
	if _, ok := optional.Attribute("Absent"); ok {
		t.Fatal("missing attribute reported present")
	}

	if doc.ByID["model-id"] != doc.Model {
		t.Fatal("model definition not indexed by Id")
	}
	if got := doc.ByID["t1"]; got == nil || got.Name.Local != "Table" {
		t.Fatalf("table definition = %+v", got)
	}
	if len(doc.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", doc.Diagnostics)
	}
}

func TestDecodeAcceptsUTF8BOM(t *testing.T) {
	input := "\xEF\xBB\xBF" + testDocument("")
	doc, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Root == nil || doc.Format.Encoding != "UTF-8" {
		t.Fatalf("BOM document root=%+v format=%+v", doc.Root, doc.Format)
	}
}

func TestDecodeDefinitionsFirstWinsAndTypedReferences(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="same"><attr:Name>first</attr:Name></obj:Table>
          <obj:Table Id="same"><attr:Name>second</attr:Name></obj:Table>
          <obj:Table Ref="missing"/>
          <obj:Column Ref="same"/>
        </col:Tables>
        <obj:Mystery Id="mystery">
          <attr:Name>private-value</attr:Name>
        </obj:Mystery>`
	doc, err := Decode(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}

	first := doc.ByID["same"]
	if first == nil {
		t.Fatal("duplicate definition was not indexed")
	}
	if got, ok := first.Property("Name"); !ok || got != "first" {
		t.Fatalf("first definition = %q, %v", got, ok)
	}

	wantCodes := map[string]bool{
		DiagnosticUnknownNode:           false,
		DiagnosticDuplicateDefinition:   false,
		DiagnosticMissingReference:      false,
		DiagnosticReferenceKindMismatch: false,
	}
	for _, diagnostic := range doc.Diagnostics {
		if _, ok := wantCodes[diagnostic.Code]; ok {
			wantCodes[diagnostic.Code] = true
		}
		if strings.Contains(diagnostic.Path, "private-value") || strings.Contains(diagnostic.Path, "first") {
			t.Fatalf("diagnostic leaked a source value: %+v", diagnostic)
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Fatalf("missing diagnostic %q in %+v", code, doc.Diagnostics)
		}
	}

	unknown := doc.Model.Child(NamespaceObject, "Mystery")
	if unknown == nil {
		t.Fatal("unknown element was not retained")
	}
	if got, ok := unknown.Property("Name"); !ok || got != "private-value" {
		t.Fatalf("unknown element data = %q, %v", got, ok)
	}
}

func TestDecodeVersionWarningAndForeignSignature(t *testing.T) {
	withoutVersion := `<?xml version="1.0" encoding="UTF-8"?>
<?PowerDesigner signature="PDM_DATA_MODEL_XML"?>
` + modelShell("")
	doc, err := Decode(strings.NewReader(withoutVersion))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(doc.Diagnostics, DiagnosticMissingVersion) {
		t.Fatalf("missing version warning: %+v", doc.Diagnostics)
	}

	foreign := strings.Replace(withoutVersion, PDMSignature, "SOMETHING_ELSE", 1)
	if _, err := Decode(strings.NewReader(foreign)); !errors.Is(err, ErrForeignSignature) {
		t.Fatalf("foreign signature error = %v", err)
	}
}

func TestDecodeRejectsMalformedRootsEncodingAndDirectives(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want error
	}{
		{name: "empty", in: "", want: ErrMalformedXML},
		{name: "wrong root", in: testHeader + "<Node/>", want: ErrInvalidStructure},
		{name: "missing root object", in: testHeader + "<Model/>", want: ErrInvalidStructure},
		{name: "missing children", in: testHeader + `<Model><obj:RootObject xmlns:obj="object"/></Model>`, want: ErrInvalidStructure},
		{name: "missing model", in: testHeader + `<Model><obj:RootObject xmlns:obj="object"><col:Children xmlns:col="collection"/></obj:RootObject></Model>`, want: ErrInvalidStructure},
		{name: "truncated", in: testDocument("")[:80], want: ErrMalformedXML},
		{name: "garbage before root", in: "garbage" + testDocument(""), want: ErrMalformedXML},
		{name: "garbage after root", in: testDocument("") + "garbage", want: ErrMalformedXML},
		{name: "multiple roots", in: testHeader + modelShell("") + modelShell(""), want: ErrMultipleRoots},
		{name: "doctype", in: testHeader + `<!DOCTYPE Model><Model/>`, want: ErrUnsupportedDirective},
		{name: "unsupported encoding", in: `<?xml version="1.0" encoding="ISO-8859-1"?><Model/>`, want: ErrUnsupportedEncoding},
		{name: "invalid UTF-8", in: testDocument("<attr:Name>\xFF</attr:Name>"), want: ErrMalformedXML},
		{name: "utf16 BOM", in: "\xFF\xFE<\x00M\x00", want: ErrUnsupportedEncoding},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(testCase.in)); !errors.Is(err, testCase.want) {
				t.Fatalf("Decode() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestDecodeLimits(t *testing.T) {
	if MaxDocumentSize != 256<<20 || MaxDepth != 128 {
		t.Fatalf("limits = %d bytes, %d depth", MaxDocumentSize, MaxDepth)
	}

	document := testDocument("")
	if _, err := decodeWithLimits(strings.NewReader(document), int64(len(document)), MaxDepth); err != nil {
		t.Fatalf("document at size limit rejected: %v", err)
	}
	if _, err := decodeWithLimits(strings.NewReader(document), int64(len(document)-1), MaxDepth); !errors.Is(err, ErrDocumentTooLarge) {
		t.Fatalf("oversize error = %v", err)
	}

	if _, err := decodeWithLimits(strings.NewReader(document), MaxDocumentSize, 4); err != nil {
		t.Fatalf("document at depth limit rejected: %v", err)
	}
	if _, err := decodeWithLimits(strings.NewReader(document), MaxDocumentSize, 3); !errors.Is(err, ErrDepthLimit) {
		t.Fatalf("depth error = %v", err)
	}
}

func testDocument(body string) string {
	return testHeader + modelShell(body)
}

func modelShell(body string) string {
	return `<Model xmlns:obj="object" xmlns:col="collection" xmlns:attr="attribute">` +
		`<obj:RootObject Id="root"><col:Children><obj:Model Id="model">` +
		body +
		`</obj:Model></col:Children></obj:RootObject></Model>`
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
