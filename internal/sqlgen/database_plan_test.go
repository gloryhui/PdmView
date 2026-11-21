package sqlgen

import (
	"encoding/xml"
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestCompileDatabasePackagesCyclesSelfReferenceAndSorting(t *testing.T) {
	document := databaseFixture()
	plan, diagnostics := compileDatabase(document)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if len(plan.tables) != 2 {
		t.Fatalf("tables = %d, want 2", len(plan.tables))
	}
	apiTable := document.Model.Contents.Packages[1].Contents.Tables[0]
	zetaTable := document.Model.Contents.Packages[0].Contents.Tables[0]
	wantAlpha := GenerateTable(apiTable).SQL
	wantZeta := GenerateTable(zetaTable).SQL
	if plan.tables[0] != wantAlpha || plan.tables[1] != wantZeta {
		t.Fatalf("table order/SQL mismatch\n0:%s\n1:%s", plan.tables[0], plan.tables[1])
	}
	if len(plan.foreignKeys) != 3 {
		t.Fatalf("foreign keys = %d, want 3", len(plan.foreignKeys))
	}
	wantNames := []string{"fk_alpha_self", "fk_alpha_zeta", "fk_zeta_alpha"}
	for offset, want := range wantNames {
		if plan.foreignKeys[offset].identifier != want {
			t.Fatalf("foreign key %d = %q, want %q", offset, plan.foreignKeys[offset].identifier, want)
		}
	}
	if plan.foreignKeys[0].childTable != "alpha" || plan.foreignKeys[0].parentTable != "alpha" {
		t.Fatalf("self reference = %+v", plan.foreignKeys[0])
	}
}

func TestCompileDatabaseConflictsAndMissingDependencies(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pdm.Document)
		code   string
	}{
		{name: "conflicting table ID", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.Tables[0].ID =
				document.Model.Contents.Packages[0].Contents.Tables[0].ID
		}, code: DiagnosticConflictingTableID},
		{name: "case folded table name", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.Tables[0].Code = "ZETA"
		}, code: DiagnosticConflictingTableName},
		{name: "duplicate foreign key ID", mutate: func(document *pdm.Document) {
			references := &document.Model.Contents.Packages[1].Contents.References
			(*references)[1].ID = (*references)[0].ID
		}, code: DiagnosticDuplicateForeignKeyID},
		{name: "missing foreign key ID", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.References[0].ID = ""
		}, code: DiagnosticMissingForeignKeyID},
		{name: "duplicate constraint name", mutate: func(document *pdm.Document) {
			references := &document.Model.Contents.Packages[1].Contents.References
			(*references)[1].Code = (*references)[0].Code
		}, code: DiagnosticConflictingConstraintName},
		{name: "missing endpoint", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.References[0].ParentTableID = "missing"
		}, code: DiagnosticForeignKeyMissingEndpoint},
		{name: "missing dependent column", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.References[0].Joins[0].ParentColumnID = "missing"
		}, code: DiagnosticForeignKeyMissingColumn},
		{name: "missing dependent key", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[1].Contents.References[0].ParentKeyID = "missing"
		}, code: DiagnosticForeignKeyMissingParentKey},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			document := databaseFixture()
			tc.mutate(document)
			_, diagnostics := compileDatabase(document)
			if !hasCode(diagnostics, tc.code) {
				t.Fatalf("diagnostics = %+v, want %s", diagnostics, tc.code)
			}
		})
	}
}

func TestCompileDatabaseRejectsScopeOverridesQualifiedNamesAndMultipleRoots(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pdm.Document)
		code   string
	}{
		{name: "model database property", mutate: func(document *pdm.Document) {
			document.Model.Raw = rawPDMElement("Database", "sales")
		}, code: DiagnosticScopeOverride},
		{name: "package schema collection", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[0].Raw = &pdm.Element{
				Name: xml.Name{Local: "Package"},
				Children: []*pdm.Element{{
					Name: xml.Name{Space: pdm.NamespaceCollection, Local: "Schemas"},
				}},
			}
		}, code: DiagnosticScopeOverride},
		{name: "table owner attribute", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[0].Contents.Tables[0].Raw = rawPDMElement("Owner", "other")
		}, code: DiagnosticScopeOverride},
		{name: "qualified table name", mutate: func(document *pdm.Document) {
			document.Model.Contents.Packages[0].Contents.Tables[0].Code = "schema.zeta"
		}, code: DiagnosticQualifiedTableName},
		{name: "multiple root models", mutate: func(document *pdm.Document) {
			rootObject := &pdm.Element{Name: xml.Name{Space: pdm.NamespaceObject, Local: "RootObject"}}
			children := &pdm.Element{Name: xml.Name{Space: pdm.NamespaceCollection, Local: "Children"}}
			first := &pdm.Element{Name: xml.Name{Space: pdm.NamespaceObject, Local: "Model"}}
			second := &pdm.Element{Name: xml.Name{Space: pdm.NamespaceObject, Local: "Model"}}
			children.Children = []*pdm.Element{first, second}
			rootObject.Children = []*pdm.Element{children}
			document.Raw = &pdm.RawDocument{
				Root:  &pdm.Element{Name: xml.Name{Local: "Model"}, Children: []*pdm.Element{rootObject}},
				Model: first,
			}
		}, code: DiagnosticMultipleRootModels},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			document := databaseFixture()
			tc.mutate(document)
			_, diagnostics := compileDatabase(document)
			if !hasCode(diagnostics, tc.code) {
				t.Fatalf("diagnostics = %+v, want %s", diagnostics, tc.code)
			}
		})
	}
}

func TestCompileDatabaseSurfacesSQLParserDiagnosticsButNotDiagrams(t *testing.T) {
	document := databaseFixture()
	document.Diagnostics = []pdm.Diagnostic{
		{Code: pdm.DiagnosticDuplicateDefinition,
			Path: "/Model/RootObject/Children/object:Model/collection:Tables/object:Table", Line: 11},
		{Code: pdm.DiagnosticExternalReference,
			Path: "/Model/RootObject/Children/object:Model/collection:Tables/object:Shortcut", Line: 12},
		{Code: pdm.DiagnosticUnknownNode,
			Path: "/Model/RootObject/Children/object:Model/collection:Tables/object:Table/attribute:IndexType", Line: 12},
		{Code: pdm.DiagnosticMissingRequiredValue,
			Path: "/Model/RootObject/Children/object:Model/collection:DefaultDiagram/object:Key", Line: 12},
		{Code: pdm.DiagnosticInvalidValue,
			Path: "/Model/RootObject/Children/object:Model/collection:DBMS/object:DBMS", Line: 12},
		{Code: pdm.DiagnosticMissingGeometry, Path: "/Model/PhysicalDiagrams/Symbol", Line: 13},
		{Code: pdm.DiagnosticReferenceMismatch, Path: "/Model/PhysicalDiagrams/ReferenceSymbol", Line: 14},
		{Code: pdm.DiagnosticMissingVersion, Path: "", Line: 0},
	}
	_, diagnostics := compileDatabase(document)
	if !hasCode(diagnostics, "pdm-"+pdm.DiagnosticDuplicateDefinition) ||
		!hasCode(diagnostics, "pdm-"+pdm.DiagnosticExternalReference) {
		t.Fatalf("SQL parser diagnostics missing: %+v", diagnostics)
	}
	warnings := 0
	for _, diagnostic := range diagnostics {
		if strings.Contains(strings.ToLower(diagnostic.Path), "physicaldiagram") {
			t.Fatalf("diagram diagnostic leaked: %+v", diagnostic)
		}
		if diagnostic.Code == "pdm-"+pdm.DiagnosticUnknownNode && diagnostic.Severity == SeverityWarning {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("schema unknown-node warnings = %d, want 1", warnings)
	}
	if hasCode(diagnostics, "pdm-"+pdm.DiagnosticMissingGeometry) ||
		hasCode(diagnostics, "pdm-"+pdm.DiagnosticReferenceMismatch) ||
		hasCode(diagnostics, "pdm-"+pdm.DiagnosticMissingRequiredValue) ||
		hasCode(diagnostics, "pdm-"+pdm.DiagnosticInvalidValue) ||
		hasCode(diagnostics, "pdm-"+pdm.DiagnosticMissingVersion) {
		t.Fatalf("diagram diagnostics leaked: %+v", diagnostics)
	}
}

func databaseFixture() *pdm.Document {
	yes, no := true, false
	intColumn := func(id, code string, mandatory *bool) pdm.Column {
		return pdm.Column{
			Metadata:  pdm.Metadata{ID: id, Code: code},
			DataType:  pdm.OptionalText{Value: "INT", Present: true},
			Mandatory: mandatory,
		}
	}
	pk := func(tableID string) []pdm.Key {
		return []pdm.Key{{
			Metadata:  pdm.Metadata{ID: tableID + "-pk", Code: tableID + "_pk"},
			ColumnIDs: []string{tableID + "-id"},
		}}
	}
	zeta := pdm.Table{
		Metadata: pdm.Metadata{ID: "zeta-id", Code: "zeta"},
		OwnerID:  "p1",
		Columns: []pdm.Column{
			intColumn("zeta-id", "id", &yes),
			intColumn("zeta-alpha-id", "alpha_id", &no),
		},
		Keys:         pk("zeta"),
		PrimaryKeyID: "zeta-pk",
	}
	alpha := pdm.Table{
		Metadata: pdm.Metadata{ID: "alpha-id", Code: "alpha"},
		OwnerID:  "p2",
		Columns: []pdm.Column{
			intColumn("alpha-id", "id", &yes),
			intColumn("alpha-zeta-id", "zeta_id", &no),
			intColumn("alpha-parent-id", "parent_id", &no),
		},
		Keys:         pk("alpha"),
		PrimaryKeyID: "alpha-pk",
	}
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model", Code: "model"},
		Contents: pdm.Contents{Packages: []pdm.Package{
			{
				Metadata: pdm.Metadata{ID: "p1", Code: "p1"},
				Contents: pdm.Contents{
					Tables: []pdm.Table{zeta},
					References: []pdm.Reference{{
						Metadata: pdm.Metadata{ID: "fk-zeta-alpha", Code: "fk_zeta_alpha"},
						OwnerID:  "p1", ChildTableID: "zeta-id", ParentTableID: "alpha-id",
						ParentKeyID: "alpha-pk",
						Joins: []pdm.ReferenceJoin{{
							ChildColumnID: "zeta-alpha-id", ParentColumnID: "alpha-id",
						}},
					}},
				},
			},
			{
				Metadata: pdm.Metadata{ID: "p2", Code: "p2"},
				Contents: pdm.Contents{
					Tables: []pdm.Table{alpha},
					References: []pdm.Reference{
						{
							Metadata: pdm.Metadata{ID: "fk-alpha-zeta", Code: "fk_alpha_zeta"},
							OwnerID:  "p2", ChildTableID: "alpha-id", ParentTableID: "zeta-id",
							ParentKeyID: "zeta-pk",
							Joins: []pdm.ReferenceJoin{{
								ChildColumnID: "alpha-zeta-id", ParentColumnID: "zeta-id",
							}},
						},
						{
							Metadata: pdm.Metadata{ID: "fk-alpha-self", Code: "fk_alpha_self"},
							OwnerID:  "p2", ChildTableID: "alpha-id", ParentTableID: "alpha-id",
							ParentKeyID: "alpha-pk",
							Joins: []pdm.ReferenceJoin{{
								ChildColumnID: "alpha-parent-id", ParentColumnID: "alpha-id",
							}},
						},
					},
				},
			},
		}},
	}}
}
