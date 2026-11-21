package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const inspectHeader = `<?xml version="1.0" encoding="UTF-8"?>
<?PowerDesigner version="16.5.0.3982" signature="PDM_DATA_MODEL_XML"?>
`

func TestRunSuccessfulReportWithDiagnostics(t *testing.T) {
	path := writeInspectFixture(t, inspectFixture())
	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %q, stdout = %s", code, stderr.String(), stdout.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Fatal != nil {
		t.Fatalf("unexpected fatal report: %+v", report.Fatal)
	}
	if report.Format.WriterVersion != "16.5.0.3982" ||
		report.Format.XMLVersion != "1.0" ||
		report.Format.Encoding != "UTF-8" ||
		report.Format.Signature != "PDM_DATA_MODEL_XML" {
		t.Fatalf("format = %+v", report.Format)
	}
	wantSummary := SummaryReport{
		Packages:         2,
		Tables:           4,
		Columns:          5,
		Keys:             2,
		PrimaryKeys:      1,
		Indexes:          1,
		References:       1,
		Joins:            1,
		Diagrams:         2,
		TableSymbols:     4,
		ReferenceSymbols: 1,
		PathPoints:       3,
	}
	if report.Summary != wantSummary {
		t.Fatalf("summary = %+v, want %+v", report.Summary, wantSummary)
	}
	if report.RawDefinitions == 0 {
		t.Fatal("raw definition count is zero")
	}
	for _, code := range []string{
		"unknown-node",
		"duplicate-definition",
		"missing-reference",
		"external-reference",
	} {
		if !hasReportDiagnostic(report.Diagnostics, code) {
			t.Fatalf("diagnostic %q absent: %+v", code, report.Diagnostics)
		}
	}
	output := stdout.String()
	for _, forbidden := range []string{
		path,
		"should-not-leak",
		"also-secret",
		"duplicate-secret",
		"external secret",
		"模型",
		"urn:should-not-leak",
	} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("report leaked %q: %s", forbidden, output)
		}
	}
}

func TestRunDistinguishesFatalParseFailure(t *testing.T) {
	path := writeInspectFixture(t, inspectHeader+`<Model><obj:RootObject`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stderr = %q, stdout = %s", code, stderr.String(), stdout.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if report.Fatal == nil || report.Fatal.Code != "malformed-xml" {
		t.Fatalf("fatal report = %+v", report.Fatal)
	}
	if strings.Contains(stdout.String(), path) || strings.Contains(stdout.String(), "RootObject") {
		t.Fatalf("fatal report leaked source or path: %s", stdout.String())
	}
	if report.Diagnostics == nil {
		t.Fatal("diagnostics must be an array")
	}
}

func TestRunUsageAndRegularFileValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("usage exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("usage stderr = %q", stderr.String())
	}

	directory := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{directory}, &stdout, &stderr); code != 1 {
		t.Fatalf("directory exit = %d", code)
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Fatal == nil || report.Fatal.Code != "not-regular-file" {
		t.Fatalf("directory fatal = %+v", report.Fatal)
	}
}

func TestNumericWriterVersion(t *testing.T) {
	tests := map[string]string{
		"16.5.0.3982":  "16.5.0.3982",
		"16":           "16",
		"16.":          "",
		"16.5-preview": "",
		"release-16":   "",
		"":             "",
	}
	for input, want := range tests {
		if got := numericVersion(input); got != want {
			t.Fatalf("numericVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func inspectFixture() string {
	return inspectHeader + `<Model xmlns:obj="object" xmlns:col="collection" xmlns:attr="attribute">
  <obj:RootObject Id="root">
    <col:Children>
      <obj:Model Id="model">
        <attr:Name>模型 &amp; 数据</attr:Name>
        <attr:Comment></attr:Comment>
        <attr:UnknownProperty>should-not-leak</attr:UnknownProperty>
        <x:Unknown xmlns:x="urn:should-not-leak">also-secret</x:Unknown>
        <col:Tables>
          <obj:Table Id="parent">
            <col:Columns><obj:Column Id="parent-col-1"/><obj:Column Id="parent-col-2"/></col:Columns>
            <col:Keys><obj:Key Id="parent-key"><col:Key.Columns><obj:Column Ref="parent-col-1"/></col:Key.Columns></obj:Key></col:Keys>
            <col:PrimaryKey><obj:Key Ref="parent-key"/></col:PrimaryKey>
            <col:Indexes>
              <obj:Index Id="parent-index">
                <attr:Unique>1</attr:Unique>
                <col:IndexColumns><obj:IndexColumn Id="parent-index-column"><col:Column><obj:Column Ref="parent-col-1"/></col:Column></obj:IndexColumn></col:IndexColumns>
              </obj:Index>
            </col:Indexes>
          </obj:Table>
          <obj:Table Id="child">
            <col:Columns><obj:Column Id="child-col"/></col:Columns>
            <col:Keys><obj:Key Id="child-key"><col:Key.Columns><obj:Column Ref="child-col"/></col:Key.Columns></obj:Key></col:Keys>
          </obj:Table>
          <obj:Table Id="parent"><attr:Name>duplicate-secret</attr:Name></obj:Table>
          <obj:Shortcut Id="external-table"><attr:Name>external secret</attr:Name></obj:Shortcut>
        </col:Tables>
        <col:References>
          <obj:Reference Id="reference">
            <col:ParentTable><obj:Table Ref="parent"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="parent-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="join">
                <col:Object1><obj:Column Ref="parent-col-1"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
        </col:References>
        <col:PhysicalDiagrams>
          <obj:PhysicalDiagram Id="diagram">
            <col:Symbols>
              <obj:TableSymbol Id="parent-symbol"><col:Object><obj:Table Ref="parent"/></col:Object><attr:Rect>((0,0),(10,10))</attr:Rect></obj:TableSymbol>
              <obj:TableSymbol Id="child-symbol"><col:Object><obj:Table Ref="child"/></col:Object><attr:Rect>((20,20),(30,30))</attr:Rect></obj:TableSymbol>
              <obj:TableSymbol Id="missing-symbol"><col:Object><obj:Table Ref="missing-table"/></col:Object><attr:Rect>((40,40),(50,50))</attr:Rect></obj:TableSymbol>
              <obj:ReferenceSymbol Id="reference-symbol">
                <col:Object><obj:Reference Ref="reference"/></col:Object>
                <col:SourceSymbol><obj:TableSymbol Ref="child-symbol"/></col:SourceSymbol>
                <col:DestinationSymbol><obj:TableSymbol Ref="parent-symbol"/></col:DestinationSymbol>
                <attr:ListOfPoints>((0,0),(5,-5),(10,0))</attr:ListOfPoints>
              </obj:ReferenceSymbol>
            </col:Symbols>
          </obj:PhysicalDiagram>
        </col:PhysicalDiagrams>
        <col:DefaultDiagram><obj:PhysicalDiagram Ref="diagram"/></col:DefaultDiagram>
        <col:Packages>
          <obj:Package Id="package-a">
            <col:Tables><obj:Table Id="package-table-a"><col:Columns><obj:Column Id="package-column-a"/></col:Columns></obj:Table></col:Tables>
            <col:PhysicalDiagrams>
              <obj:PhysicalDiagram Id="package-diagram">
                <col:Symbols><obj:TableSymbol Id="package-symbol"><col:Object><obj:Table Ref="package-table-a"/></col:Object><attr:Rect>((0,0),(1,1))</attr:Rect></obj:TableSymbol></col:Symbols>
              </obj:PhysicalDiagram>
            </col:PhysicalDiagrams>
          </obj:Package>
          <obj:Package Id="package-b">
            <col:Tables><obj:Table Id="package-table-b"><col:Columns><obj:Column Id="package-column-b"/></col:Columns></obj:Table></col:Tables>
          </obj:Package>
        </col:Packages>
      </obj:Model>
    </col:Children>
  </obj:RootObject>
</Model>`
}

func writeInspectFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.pdm")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func hasReportDiagnostic(diagnostics []DiagnosticReport, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
