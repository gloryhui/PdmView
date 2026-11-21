package pdm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestIntegrationTableShortcutDeclarations(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="local-table"/>
          <obj:Shortcut Id="external-table-1"><attr:Name>External One</attr:Name></obj:Shortcut>
          <obj:Shortcut Id="external-table-2"><attr:Name>External Two</attr:Name></obj:Shortcut>
          <obj:Shortcut Id="external-table-3"><attr:Name>External Three</attr:Name></obj:Shortcut>
        </col:Tables>`
	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Model.Contents.Tables) != 1 ||
		document.Model.Contents.Tables[0].ID != "local-table" {
		t.Fatalf("shortcuts became local tables: %+v", document.Model.Contents.Tables)
	}
	if got := diagnosticCount(document.Diagnostics, DiagnosticExternalReference); got != 3 {
		t.Fatalf("external diagnostics = %d, %+v", got, document.Diagnostics)
	}
	if hasDiagnostic(document.Diagnostics, DiagnosticMalformedCollectionMember) {
		t.Fatalf("shortcut incorrectly reported malformed: %+v", document.Diagnostics)
	}
	for _, id := range []string{"external-table-1", "external-table-2", "external-table-3"} {
		shortcut := document.Raw.ByID[id]
		if shortcut == nil {
			t.Fatalf("raw shortcut %q not retained", id)
		}
		if _, ok := shortcut.Property("Name"); !ok {
			t.Fatalf("raw shortcut %q lost properties", id)
		}
	}
}

func TestIntegrationDiagnosticsScopesAndUnicode(t *testing.T) {
	body := `
        <attr:Name>模型 &amp; 数据</attr:Name>
        <attr:Comment></attr:Comment>
        <attr:UnknownProperty>secret-property-value</attr:UnknownProperty>
        <x:Unknown xmlns:x="urn:secret-namespace">secret-node-value</x:Unknown>
        <col:Tables>
          <obj:Table Id="duplicate"><attr:Name>first</attr:Name></obj:Table>
          <obj:Table Id="duplicate"><attr:Name>second</attr:Name></obj:Table>
          <obj:Table Id="unicode-table">
            <col:Columns>
              <obj:Column Id="unicode-column">
                <attr:DefaultValue>"特殊" &amp; data</attr:DefaultValue>
              </obj:Column>
            </col:Columns>
          </obj:Table>
        </col:Tables>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="root-diagram">
            <col:Symbols>
              <obj:TableSymbol Id="missing-symbol">
                <col:Object><obj:Table Ref="missing-table"/></col:Object>
                <attr:Rect>((0,0),(1,1))</attr:Rect>
              </obj:TableSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>
        <col:Packages>
          <obj:Package Id="package-one">
            <col:Tables><obj:Table Id="package-table-one"/></col:Tables>
            <col:PhysicalDiagrams><obj:PhysicalDiagram Id="package-diagram"><col:Symbols/></obj:PhysicalDiagram></col:PhysicalDiagrams>
          </obj:Package>
          <obj:Package Id="package-two">
            <col:Tables><obj:Table Id="package-table-two"/></col:Tables>
          </obj:Package>
        </col:Packages>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if document.Model.Name != "模型 & 数据" ||
		!document.Model.Comment.Present ||
		document.Model.Comment.Value != "" {
		t.Fatalf("model metadata = %+v", document.Model.Metadata)
	}
	if got := diagnosticCount(document.Diagnostics, DiagnosticUnknownNode); got < 2 {
		t.Fatalf("unknown diagnostics = %d, %+v", got, document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticMissingReference) {
		t.Fatalf("missing-reference absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticDuplicateDefinition) {
		t.Fatalf("duplicate-definition absent: %+v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		for _, forbidden := range []string{"secret-", "urn:secret", "模型 & 数据"} {
			if strings.Contains(diagnostic.Path, forbidden) {
				t.Fatalf("diagnostic leaked source data: %+v", diagnostic)
			}
		}
	}
	if len(document.Model.Contents.Packages) != 2 ||
		len(document.Model.Contents.Diagrams) != 1 ||
		len(document.Model.Contents.Packages[0].Contents.Diagrams) != 1 {
		t.Fatalf("scope summary = packages:%d root-diagrams:%d package-diagrams:%d",
			len(document.Model.Contents.Packages),
			len(document.Model.Contents.Diagrams),
			len(document.Model.Contents.Packages[0].Contents.Diagrams))
	}
	unicodeTable := schemaTestTable(t, document.Model.Contents, "unicode-table")
	if len(unicodeTable.Columns) != 1 ||
		unicodeTable.Columns[0].DefaultValue.Value != `"特殊" & data` ||
		!unicodeTable.Columns[0].DefaultValue.Present {
		t.Fatalf("unicode column = %+v", unicodeTable.Columns)
	}
}

func TestIntegrationTruncatedInput(t *testing.T) {
	input := testDocument(`<col:Tables><obj:Table Id="truncated"/>`)
	if _, err := Parse(strings.NewReader(input[:len(input)-20])); !errors.Is(err, ErrMalformedXML) {
		t.Fatalf("truncated parse error = %v", err)
	}
}

func TestLargeSyntheticDeterministicRegression(t *testing.T) {
	const (
		packageCount  = 2
		tablesPerPkg  = 55
		columnsPerTbl = 100
	)
	firstXML := generateLargePDM(packageCount, tablesPerPkg, columnsPerTbl)
	secondXML := generateLargePDM(packageCount, tablesPerPkg, columnsPerTbl)
	if firstXML != secondXML {
		t.Fatal("large synthetic generator is not deterministic")
	}

	document, err := Parse(strings.NewReader(firstXML))
	if err != nil {
		t.Fatal(err)
	}
	packages, tables, columns := countIntegrationContents(document.Model.Contents)
	if packages != packageCount || tables != packageCount*tablesPerPkg ||
		columns != packageCount*tablesPerPkg*columnsPerTbl {
		t.Fatalf("large summary = packages:%d tables:%d columns:%d", packages, tables, columns)
	}
	if len(document.Model.Contents.Diagrams) != 0 ||
		len(document.Model.Contents.Packages) != packageCount {
		t.Fatalf("large root diagrams/packages = %d/%d",
			len(document.Model.Contents.Diagrams), len(document.Model.Contents.Packages))
	}
	for _, pkg := range document.Model.Contents.Packages {
		if len(pkg.Contents.Diagrams) != 1 ||
			len(pkg.Contents.Diagrams[0].TableSymbols) != 1 {
			t.Fatalf("large package diagrams = %+v", pkg.Contents.Diagrams)
		}
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("large regression diagnostics: %+v", document.Diagnostics)
	}
}

func generateLargePDM(packageCount, tablesPerPackage, columnsPerTable int) string {
	var body strings.Builder
	body.WriteString(`<col:Packages>`)
	for pkg := 0; pkg < packageCount; pkg++ {
		fmt.Fprintf(&body, `<obj:Package Id="package-%d"><col:Tables>`, pkg)
		for table := 0; table < tablesPerPackage; table++ {
			fmt.Fprintf(&body, `<obj:Table Id="table-%d-%d"><col:Columns>`, pkg, table)
			for column := 0; column < columnsPerTable; column++ {
				fmt.Fprintf(
					&body,
					`<obj:Column Id="column-%d-%d-%d"><attr:DataType>VARCHAR</attr:DataType><attr:Length>64</attr:Length></obj:Column>`,
					pkg, table, column,
				)
			}
			body.WriteString(`</col:Columns></obj:Table>`)
		}
		body.WriteString(`</col:Tables>`)
		body.WriteString(`<col:PhysicalDiagrams><obj:PhysicalDiagram Id="diagram-`)
		fmt.Fprintf(&body, `%d"><col:Symbols>`, pkg)
		fmt.Fprintf(
			&body,
			`<obj:TableSymbol Id="symbol-%d"><col:Object><obj:Table Ref="table-%d-0"/></col:Object><attr:Rect>((0,0),(10,10))</attr:Rect></obj:TableSymbol>`,
			pkg, pkg,
		)
		body.WriteString(`</col:Symbols></obj:PhysicalDiagram></col:PhysicalDiagrams></obj:Package>`)
	}
	body.WriteString(`</col:Packages>`)
	return testDocument(body.String())
}

func countIntegrationContents(contents Contents) (packages, tables, columns int) {
	packages = len(contents.Packages)
	tables = len(contents.Tables)
	for _, table := range contents.Tables {
		columns += len(table.Columns)
	}
	for _, pkg := range contents.Packages {
		nestedPackages, nestedTables, nestedColumns := countIntegrationContents(pkg.Contents)
		packages += nestedPackages
		tables += nestedTables
		columns += nestedColumns
	}
	return packages, tables, columns
}
