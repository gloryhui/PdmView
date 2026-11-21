package pdm

import (
	"errors"
	"strings"
	"testing"
)

func TestParseModelMetadataAndResolvedDBMS(t *testing.T) {
	body := `
        <attr:ObjectID>MODEL-OBJECT</attr:ObjectID>
        <attr:Name>Sales Model</attr:Name>
        <attr:Code>sales</attr:Code>
        <attr:Comment></attr:Comment>
        <col:DBMS>
          <obj:Shortcut Ref="db-object"/>
        </col:DBMS>
        <obj:Shortcut Id="db-object">
          <attr:ObjectID>DB-OBJECT</attr:ObjectID>
          <attr:Name>MySQL</attr:Name>
          <attr:Code>MYSQL</attr:Code>
        </obj:Shortcut>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}

	if document.Raw == nil || document.Model.Raw != document.Raw.Model {
		t.Fatal("model did not retain its raw element")
	}
	if document.Model.ID != "model" ||
		document.Model.ObjectID != "MODEL-OBJECT" ||
		document.Model.Name != "Sales Model" ||
		document.Model.Code != "sales" {
		t.Fatalf("model metadata = %+v", document.Model.Metadata)
	}
	if !document.Model.Comment.Present || document.Model.Comment.Value != "" {
		t.Fatalf("model comment = %+v", document.Model.Comment)
	}
	if document.Model.DBMS == nil ||
		document.Model.DBMS.ID != "db-object" ||
		document.Model.DBMS.Name != "MySQL" ||
		document.Model.DBMS.Code != "MYSQL" ||
		document.Model.DBMS.ObjectID != "DB-OBJECT" {
		t.Fatalf("model DBMS = %+v", document.Model.DBMS)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("valid DBMS shortcut emitted diagnostics: %v", document.Diagnostics)
	}
}

func TestProjectDBMSDirectFallback(t *testing.T) {
	body := `
        <col:DBMS>
          <obj:DBMS Id="direct-db">
            <attr:Name>Direct DB</attr:Name>
            <attr:Code>DIRECT</attr:Code>
          </obj:DBMS>
        </col:DBMS>`
	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if document.Model.DBMS == nil ||
		document.Model.DBMS.ID != "direct-db" ||
		document.Model.DBMS.Name != "Direct DB" {
		t.Fatalf("direct DBMS = %+v", document.Model.DBMS)
	}
}

func TestParseBlankVersusMissingComment(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="blank-comment"><attr:Comment></attr:Comment></obj:Table>
          <obj:Table Id="missing-comment"/>
        </col:Tables>`
	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Model.Contents.Tables) != 2 {
		t.Fatalf("tables = %+v", document.Model.Contents.Tables)
	}

	blank := document.Model.Contents.Tables[0]
	if !blank.Comment.Present || blank.Comment.Value != "" {
		t.Fatalf("blank comment = %+v", blank.Comment)
	}
	missing := document.Model.Contents.Tables[1]
	if missing.Comment.Present || missing.Comment.Value != "" {
		t.Fatalf("missing comment = %+v", missing.Comment)
	}
}

func TestParseNestedPackagesOwnershipOrderAndDuplicates(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="root-1"><attr:Name>root first</attr:Name></obj:Table>
          <obj:Table Ref="root-1"/>
          <obj:Table Id="root-2"/>
          <obj:Table Id="root-1"><attr:Name>duplicate</attr:Name></obj:Table>
        </col:Tables>
        <col:Packages>
          <obj:Package Id="package-a">
            <col:Tables>
              <obj:Table Id="a-1"/>
              <obj:Table Id="a-2"/>
            </col:Tables>
            <col:Packages>
              <obj:Package Id="package-a-child">
                <col:Tables><obj:Table Id="child-1"/></col:Tables>
              </obj:Package>
              <obj:Package Id="package-a-sibling"/>
            </col:Packages>
          </obj:Package>
          <obj:Package Id="package-b">
            <col:Tables><obj:Table Id="b-1"/></col:Tables>
          </obj:Package>
          <obj:Package Id="package-a">
            <col:Tables><obj:Table Id="duplicate-child"/></col:Tables>
          </obj:Package>
        </col:Packages>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}

	if got := tableIDs(document.Model.Contents.Tables); !equalStrings(got, []string{"root-1", "root-2"}) {
		t.Fatalf("root table order = %v", got)
	}
	if got, want := document.Model.Contents.Tables[0].Name, "root first"; got != want {
		t.Fatalf("first table metadata = %q, want %q", got, want)
	}
	for _, table := range document.Model.Contents.Tables {
		if table.OwnerID != "model" {
			t.Fatalf("root table %q owner = %q", table.ID, table.OwnerID)
		}
	}

	if got := packageIDs(document.Model.Contents.Packages); !equalStrings(got, []string{"package-a", "package-b"}) {
		t.Fatalf("root package order = %v", got)
	}
	pkgA := document.Model.Contents.Packages[0]
	pkgB := document.Model.Contents.Packages[1]
	if pkgA.ParentID != "model" || pkgB.ParentID != "model" {
		t.Fatalf("root parent IDs = %q, %q", pkgA.ParentID, pkgB.ParentID)
	}
	if got := tableIDs(pkgA.Contents.Tables); !equalStrings(got, []string{"a-1", "a-2"}) {
		t.Fatalf("package A table order = %v", got)
	}
	for _, table := range pkgA.Contents.Tables {
		if table.OwnerID != "package-a" {
			t.Fatalf("package A table %q owner = %q", table.ID, table.OwnerID)
		}
	}
	if got := packageIDs(pkgA.Contents.Packages); !equalStrings(got, []string{"package-a-child", "package-a-sibling"}) {
		t.Fatalf("package A child order = %v", got)
	}
	child := pkgA.Contents.Packages[0]
	if child.ParentID != "package-a" ||
		len(child.Contents.Tables) != 1 ||
		child.Contents.Tables[0].OwnerID != "package-a-child" {
		t.Fatalf("nested package = %+v", child)
	}
	if got := tableIDs(pkgB.Contents.Tables); !equalStrings(got, []string{"b-1"}) {
		t.Fatalf("package B tables = %v", got)
	}

	for _, table := range document.Model.Contents.Tables {
		if table.ID == "a-1" || table.ID == "child-1" || table.ID == "duplicate-child" {
			t.Fatalf("child table leaked into root contents: %+v", table)
		}
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticDuplicateDefinition) {
		t.Fatalf("duplicate diagnostic missing: %+v", document.Diagnostics)
	}
}

func TestParseDiagnosesMissingIDsAndMalformedCollectionMembers(t *testing.T) {
	body := `
        <col:Packages>
          <obj:Package><attr:Name>package-no-id</attr:Name></obj:Package>
          <obj:Table Id="wrong-package-member"><attr:Name>secret-package-value</attr:Name></obj:Table>
        </col:Packages>
        <col:Tables>
          <obj:Table><attr:Name>table-no-id</attr:Name></obj:Table>
          <obj:Package Id="wrong-table-member"/>
          <attr:Name>secret-table-value</attr:Name>
        </col:Tables>`

	raw, err := Decode(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	rawDiagnosticCount := len(raw.Diagnostics)
	document := Project(raw)

	if len(raw.Diagnostics) != rawDiagnosticCount {
		t.Fatal("Project mutated raw diagnostics")
	}
	if len(document.Model.Contents.Packages) != 1 || document.Model.Contents.Packages[0].ID != "" {
		t.Fatalf("package without ID = %+v", document.Model.Contents.Packages)
	}
	if len(document.Model.Contents.Tables) != 1 || document.Model.Contents.Tables[0].ID != "" {
		t.Fatalf("table without ID = %+v", document.Model.Contents.Tables)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticMissingID) {
		t.Fatalf("missing-id diagnostic absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticMalformedCollectionMember) {
		t.Fatalf("malformed-member diagnostic absent: %+v", document.Diagnostics)
	}
	for _, diagnostic := range document.Diagnostics {
		if strings.Contains(diagnostic.Path, "secret-") {
			t.Fatalf("diagnostic leaked a value: %+v", diagnostic)
		}
	}
}

func TestParsePropagatesInputErrors(t *testing.T) {
	document, err := Parse(strings.NewReader(`<Model>`))
	if document != nil {
		t.Fatalf("document returned with error: %+v", document)
	}
	if !errors.Is(err, ErrMalformedXML) {
		t.Fatalf("parse error = %v", err)
	}

	readErr := errors.New("synthetic read failure")
	document, err = Parse(modelErrorReader{err: readErr})
	if document != nil {
		t.Fatalf("document returned with read error: %+v", document)
	}
	if !errors.Is(err, readErr) {
		t.Fatalf("reader error = %v, want %v", err, readErr)
	}
}

type modelErrorReader struct {
	err error
}

func (r modelErrorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func tableIDs(tables []Table) []string {
	ids := make([]string, 0, len(tables))
	for _, table := range tables {
		ids = append(ids, table.ID)
	}
	return ids
}

func packageIDs(packages []Package) []string {
	ids := make([]string, 0, len(packages))
	for _, pkg := range packages {
		ids = append(ids, pkg.ID)
	}
	return ids
}

func equalStrings(got, want []string) bool {
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
