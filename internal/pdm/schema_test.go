package pdm

import (
	"strings"
	"testing"
)

func TestParseSchemaColumnsKeysIndexesAndCrossPackageReference(t *testing.T) {
	body := `
        <col:Packages>
          <obj:Package Id="package-parent">
            <col:Tables>
              <obj:Table Id="parent-table">
                <attr:Name>父表</attr:Name>
                <col:Columns>
                  <obj:Column Id="parent-col-1">
                    <attr:Name>编号</attr:Name>
                    <attr:Code>id</attr:Code>
                    <attr:DataType>INT</attr:DataType>
                    <attr:Length>11</attr:Length>
                    <attr:Precision>10</attr:Precision>
                    <attr:Scale>0</attr:Scale>
                    <attr:Column.Mandatory>1</attr:Column.Mandatory>
                    <attr:Identity>1</attr:Identity>
                  </obj:Column>
                  <obj:Column Id="parent-col-2">
                    <attr:Name>名称</attr:Name>
                    <attr:DataType>VARCHAR</attr:DataType>
                    <attr:Length>64</attr:Length>
                    <attr:DefaultValue></attr:DefaultValue>
                    <attr:Mandatory>0</attr:Mandatory>
                    <attr:Identity>0</attr:Identity>
                  </obj:Column>
                  <obj:Column Id="parent-col-3">
                    <attr:DataType>DECIMAL</attr:DataType>
                    <attr:Length>12</attr:Length>
                    <attr:Precision>4</attr:Precision>
                    <attr:Scale>2</attr:Scale>
                  </obj:Column>
                </col:Columns>
                <col:Keys>
                  <obj:Key Id="parent-key">
                    <attr:Name>PK_PARENT</attr:Name>
                    <col:Key.Columns>
                      <obj:Column Ref="parent-col-1"/>
                      <obj:Column Ref="parent-col-2"/>
                    </col:Key.Columns>
                  </obj:Key>
                </col:Keys>
                <col:PrimaryKey><obj:Key Ref="parent-key"/></col:PrimaryKey>
                <col:Indexes>
                  <obj:Index Id="parent-index">
                    <attr:Name>IDX_PARENT</attr:Name>
                    <attr:Unique>1</attr:Unique>
                    <col:LinkedObject><obj:Key Ref="parent-key"/></col:LinkedObject>
                    <col:IndexColumns>
                      <obj:IndexColumn Id="index-col-2">
                        <attr:SortOrder>DESC</attr:SortOrder>
                        <col:Column><obj:Column Ref="parent-col-2"/></col:Column>
                      </obj:IndexColumn>
                      <obj:IndexColumn Id="index-col-1">
                        <col:Column><obj:Column Ref="parent-col-1"/></col:Column>
                      </obj:IndexColumn>
                    </col:IndexColumns>
                  </obj:Index>
                </col:Indexes>
              </obj:Table>
            </col:Tables>
          </obj:Package>
          <obj:Package Id="package-child">
            <col:Tables>
              <obj:Table Id="child-table">
                <col:Columns>
                  <obj:Column Id="child-col-1"><attr:DataType>INT</attr:DataType></obj:Column>
                  <obj:Column Id="child-col-2"><attr:DataType>VARCHAR</attr:DataType></obj:Column>
                </col:Columns>
                <col:Keys>
                  <obj:Key Id="child-key">
                    <col:Key.Columns><obj:Column Ref="child-col-1"/></col:Key.Columns>
                  </obj:Key>
                </col:Keys>
              </obj:Table>
            </col:Tables>
          </obj:Package>
        </col:Packages>
        <col:References>
          <obj:Reference Id="reference-parent-child">
            <attr:Name>FK_CHILD_PARENT</attr:Name>
            <attr:UpdateConstraint>RESTRICT</attr:UpdateConstraint>
            <attr:DeleteConstraint>CASCADE</attr:DeleteConstraint>
            <attr:Cardinality>0,n</attr:Cardinality>
            <col:ParentTable><obj:Table Ref="parent-table"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child-table"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="parent-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="join-second">
                <col:Object1><obj:Column Ref="parent-col-2"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col-2"/></col:Object2>
              </obj:ReferenceJoin>
              <obj:ReferenceJoin Id="join-first">
                <col:Object1><obj:Column Ref="parent-col-1"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col-1"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
        </col:References>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", document.Diagnostics)
	}

	parent := schemaTestTable(t, document.Model.Contents, "parent-table")
	if parent.Name != "父表" || len(parent.Columns) != 3 {
		t.Fatalf("parent table = %+v", parent)
	}
	first := parent.Columns[0]
	if first.Name != "编号" || first.Code != "id" ||
		first.DataType != (OptionalText{Value: "INT", Present: true}) ||
		first.Length != (OptionalText{Value: "11", Present: true}) ||
		first.Precision != (OptionalText{Value: "10", Present: true}) ||
		first.Scale != (OptionalText{Value: "0", Present: true}) {
		t.Fatalf("first column = %+v", first)
	}
	if first.Mandatory == nil || !*first.Mandatory ||
		first.Identity == nil || !*first.Identity {
		t.Fatalf("first flags = mandatory:%v identity:%v", first.Mandatory, first.Identity)
	}
	if first.DefaultValue.Present {
		t.Fatalf("absent default marked present: %+v", first.DefaultValue)
	}
	second := parent.Columns[1]
	if !second.DefaultValue.Present || second.DefaultValue.Value != "" {
		t.Fatalf("blank default = %+v", second.DefaultValue)
	}
	if second.Mandatory == nil || *second.Mandatory ||
		second.Identity == nil || *second.Identity {
		t.Fatalf("second flags = mandatory:%v identity:%v", second.Mandatory, second.Identity)
	}
	third := parent.Columns[2]
	if third.DataType != (OptionalText{Value: "DECIMAL", Present: true}) ||
		third.Length != (OptionalText{Value: "12", Present: true}) ||
		third.Precision != (OptionalText{Value: "4", Present: true}) ||
		third.Scale != (OptionalText{Value: "2", Present: true}) {
		t.Fatalf("third column = %+v", third)
	}
	if third.Mandatory != nil || third.Identity != nil || third.DefaultValue.Present {
		t.Fatalf("absent optional properties were fabricated: %+v", third)
	}

	if len(parent.Keys) != 1 ||
		!equalStrings(parent.Keys[0].ColumnIDs, []string{"parent-col-1", "parent-col-2"}) {
		t.Fatalf("composite key = %+v", parent.Keys)
	}
	if parent.PrimaryKeyID != "parent-key" {
		t.Fatalf("primary key = %q", parent.PrimaryKeyID)
	}
	if len(parent.Indexes) != 1 || parent.Indexes[0].Unique == nil || !*parent.Indexes[0].Unique {
		t.Fatalf("index = %+v", parent.Indexes)
	}
	index := parent.Indexes[0]
	if len(index.Columns) != 2 ||
		index.Columns[0].ColumnID != "parent-col-2" ||
		index.Columns[0].SortOrder.Value != "DESC" || !index.Columns[0].SortOrder.Present ||
		index.Columns[1].ColumnID != "parent-col-1" ||
		index.Columns[1].SortOrder.Present {
		t.Fatalf("index columns = %+v", index.Columns)
	}
	if index.Raw == nil || index.Raw.Child(NamespaceCollection, "LinkedObject") == nil {
		t.Fatal("index LinkedObject was not preserved in raw metadata")
	}

	if len(document.Model.Contents.References) != 1 {
		t.Fatalf("references = %+v", document.Model.Contents.References)
	}
	reference := document.Model.Contents.References[0]
	if reference.OwnerID != "model" ||
		reference.ParentTableID != "parent-table" ||
		reference.ChildTableID != "child-table" ||
		reference.ParentKeyID != "parent-key" {
		t.Fatalf("reference endpoints = %+v", reference)
	}
	if reference.UpdateConstraint.Value != "RESTRICT" ||
		reference.DeleteConstraint.Value != "CASCADE" ||
		reference.Cardinality.Value != "0,n" {
		t.Fatalf("reference constraints = %+v", reference)
	}
	if len(reference.Joins) != 2 ||
		reference.Joins[0].ParentColumnID != "parent-col-2" ||
		reference.Joins[0].ChildColumnID != "child-col-2" ||
		reference.Joins[1].ParentColumnID != "parent-col-1" ||
		reference.Joins[1].ChildColumnID != "child-col-1" {
		t.Fatalf("reference join order = %+v", reference.Joins)
	}
}

func TestParseMandatoryVariantsAndInvalidFlags(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="flags-table">
            <col:Columns>
              <obj:Column Id="same">
                <attr:Column.Mandatory>1</attr:Column.Mandatory>
                <attr:Mandatory>1</attr:Mandatory>
              </obj:Column>
              <obj:Column Id="conflict">
                <attr:Column.Mandatory>1</attr:Column.Mandatory>
                <attr:Mandatory>0</attr:Mandatory>
              </obj:Column>
              <obj:Column Id="invalid">
                <attr:Mandatory>yes</attr:Mandatory>
                <attr:Identity>2</attr:Identity>
              </obj:Column>
              <obj:Column Id="legacy">
                <attr:Mandatory>1</attr:Mandatory>
              </obj:Column>
            </col:Columns>
          </obj:Table>
        </col:Tables>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	table := schemaTestTable(t, document.Model.Contents, "flags-table")
	if table.Columns[0].Mandatory == nil || !*table.Columns[0].Mandatory {
		t.Fatalf("matching mandatory aliases = %v", table.Columns[0].Mandatory)
	}
	if table.Columns[1].Mandatory != nil {
		t.Fatalf("conflicting mandatory fabricated a value: %v", *table.Columns[1].Mandatory)
	}
	if table.Columns[2].Mandatory != nil || table.Columns[2].Identity != nil {
		t.Fatalf("invalid flags fabricated values: %+v", table.Columns[2])
	}
	if table.Columns[3].Mandatory == nil || !*table.Columns[3].Mandatory {
		t.Fatalf("legacy mandatory = %v", table.Columns[3].Mandatory)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticConflictingValue) {
		t.Fatalf("conflict diagnostic absent: %+v", document.Diagnostics)
	}
	invalidCount := diagnosticCount(document.Diagnostics, DiagnosticInvalidValue)
	if invalidCount != 2 {
		t.Fatalf("invalid value diagnostics = %d, %+v", invalidCount, document.Diagnostics)
	}
}

func TestParseSchemaOwnershipMissingValuesAndExternalParent(t *testing.T) {
	body := `
        <col:Tables>
          <obj:Table Id="parent">
            <col:Columns>
              <obj:Column Id="parent-col"><attr:DataType>INT</attr:DataType></obj:Column>
            </col:Columns>
            <col:Keys>
              <obj:Key Id="parent-key">
                <col:Key.Columns><obj:Column Ref="parent-col"/></col:Key.Columns>
              </obj:Key>
              <obj:Key Id="wrong-key">
                <col:Key.Columns><obj:Column Ref="child-col"/></col:Key.Columns>
              </obj:Key>
              <obj:Key Id="empty-key"/>
            </col:Keys>
            <col:PrimaryKey><obj:Key Ref="child-key"/></col:PrimaryKey>
            <col:Indexes>
              <obj:Index Id="wrong-index">
                <attr:Unique>yes</attr:Unique>
                <col:IndexColumns>
                  <obj:IndexColumn Id="wrong-index-col">
                    <col:Column><obj:Column Ref="child-col"/></col:Column>
                  </obj:IndexColumn>
                  <obj:IndexColumn Id="empty-index-col"/>
                </col:IndexColumns>
              </obj:Index>
            </col:Indexes>
          </obj:Table>
          <obj:Table Id="child">
            <col:Columns>
              <obj:Column Id="child-col"><attr:DataType>INT</attr:DataType></obj:Column>
            </col:Columns>
            <col:Keys>
              <obj:Key Id="child-key">
                <col:Key.Columns><obj:Column Ref="child-col"/></col:Key.Columns>
              </obj:Key>
            </col:Keys>
          </obj:Table>
        </col:Tables>
        <col:References>
          <obj:Reference Id="bad-reference">
            <col:ParentTable><obj:Table Ref="parent"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="child-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="bad-join">
                <col:Object1><obj:Column Ref="child-col"/></col:Object1>
                <col:Object2><obj:Column Ref="parent-col"/></col:Object2>
              </obj:ReferenceJoin>
              <obj:ReferenceJoin Id="missing-join-end">
                <col:Object1><obj:Column Ref="parent-col"/></col:Object1>
                <col:Object2><obj:Column Ref="ghost-col"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
          <obj:Reference Id="external-reference">
            <col:ParentTable><obj:Shortcut Ref="external-parent"/></col:ParentTable>
            <col:ChildTable><obj:Table Ref="child"/></col:ChildTable>
            <col:ParentKey><obj:Key Ref="child-key"/></col:ParentKey>
            <col:Joins>
              <obj:ReferenceJoin Id="external-join">
                <col:Object1><obj:Column Ref="parent-col"/></col:Object1>
                <col:Object2><obj:Column Ref="child-col"/></col:Object2>
              </obj:ReferenceJoin>
            </col:Joins>
          </obj:Reference>
        </col:References>`

	document, err := Parse(strings.NewReader(testDocument(body)))
	if err != nil {
		t.Fatal(err)
	}
	if diagnosticCount(document.Diagnostics, DiagnosticWrongOwner) < 6 {
		t.Fatalf("wrong-owner diagnostics = %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticMissingRequiredValue) {
		t.Fatalf("missing-required-value absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticInvalidValue) {
		t.Fatalf("invalid-value absent: %+v", document.Diagnostics)
	}
	if !hasDiagnostic(document.Diagnostics, DiagnosticExternalReference) {
		t.Fatalf("external diagnostic absent: %+v", document.Diagnostics)
	}
	if got := diagnosticCount(document.Diagnostics, DiagnosticMissingReference); got != 2 {
		t.Fatalf("raw missing-reference diagnostics = %d, %+v", got, document.Diagnostics)
	}

	references := document.Model.Contents.References
	if len(references) != 2 || references[1].ParentTableID != "external-parent" {
		t.Fatalf("external parent was dereferenced or lost: %+v", references)
	}
}

func schemaTestTable(t *testing.T, contents Contents, id string) Table {
	t.Helper()
	for _, table := range contents.Tables {
		if table.ID == id {
			return table
		}
	}
	for _, pkg := range contents.Packages {
		if table, ok := schemaTestLookupTable(pkg.Contents, id); ok {
			return table
		}
	}
	t.Fatalf("table %q not found", id)
	return Table{}
}

func schemaTestLookupTable(contents Contents, id string) (Table, bool) {
	for _, table := range contents.Tables {
		if table.ID == id {
			return table, true
		}
	}
	for _, pkg := range contents.Packages {
		if table, ok := schemaTestLookupTable(pkg.Contents, id); ok {
			return table, true
		}
	}
	return Table{}, false
}

func diagnosticCount(diagnostics []Diagnostic, code string) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			count++
		}
	}
	return count
}
