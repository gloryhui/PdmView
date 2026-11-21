package sqlgen

import (
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestCompileForeignKeyOrderedActionsAndRendering(t *testing.T) {
	child, parent, reference := foreignKeyPair()
	compiledChild := mustCompileTable(t, child)
	compiledParent := mustCompileTable(t, parent)
	foreignKey, diagnostics := compileForeignKey(compiledChild, compiledParent, reference)
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	want := "CONSTRAINT `fk_property` FOREIGN KEY (`tenant_ref`, `id_ref`) " +
		"REFERENCES `parent` (`tenant`, `id`) ON DELETE CASCADE ON UPDATE SET NULL"
	if foreignKey.SQL() != want || foreignKey.InlineSQL() != want {
		t.Fatalf("constraint SQL = %q, want %q", foreignKey.SQL(), want)
	}
	wantAlter := "ALTER TABLE `child` ADD " + want + ";"
	if foreignKey.AlterSQL() != wantAlter {
		t.Fatalf("ALTER SQL = %q, want %q", foreignKey.AlterSQL(), wantAlter)
	}

	reference.Metadata.Raw = nil
	reference.Metadata.Name = "FK Display Name"
	foreignKey, diagnostics = compileForeignKey(compiledChild, compiledParent, reference)
	if len(diagnostics) != 0 || foreignKey.identifier != "fk_code" {
		t.Fatalf("Code priority result = %+v, diagnostics %+v", foreignKey, diagnostics)
	}
}

func TestCompileForeignKeySelfReference(t *testing.T) {
	yes, no := true, false
	table := pdm.Table{
		Metadata: pdm.Metadata{ID: "node", Code: "node"},
		OwnerID:  "scope",
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "id", Code: "id"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &yes},
			{Metadata: pdm.Metadata{ID: "parent_id", Code: "parent_id"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &no},
		},
		Keys:         []pdm.Key{{Metadata: pdm.Metadata{ID: "pk", Code: "pk"}, ColumnIDs: []string{"id"}}},
		PrimaryKeyID: "pk",
	}
	compiled := mustCompileTable(t, table)
	reference := pdm.Reference{
		Metadata:         pdm.Metadata{ID: "self-fk", Code: "fk_self"},
		OwnerID:          "scope",
		ParentTableID:    "node",
		ChildTableID:     "node",
		ParentKeyID:      "pk",
		DeleteConstraint: pdm.OptionalText{Value: "SET NULL", Present: true},
		Joins: []pdm.ReferenceJoin{{
			ParentColumnID: "id",
			ChildColumnID:  "parent_id",
		}},
	}
	foreignKey, diagnostics := compileForeignKey(compiled, compiled, reference)
	if len(diagnostics) != 0 {
		t.Fatalf("self-reference diagnostics: %+v", diagnostics)
	}
	want := "CONSTRAINT `fk_self` FOREIGN KEY (`parent_id`) REFERENCES `node` (`id`) ON DELETE SET NULL"
	if foreignKey.SQL() != want {
		t.Fatalf("self-reference SQL = %q", foreignKey.SQL())
	}
}

func TestCompileForeignKeySelfReferenceSameColumn(t *testing.T) {
	nullable := false
	table := pdm.Table{
		Metadata: pdm.Metadata{ID: "node", Code: "node"},
		OwnerID:  "scope",
		Columns: []pdm.Column{{
			Metadata:  pdm.Metadata{ID: "link_id", Code: "link_id"},
			DataType:  pdm.OptionalText{Value: "INT", Present: true},
			Mandatory: &nullable,
		}},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "uk_link", Code: "uk_link"},
			ColumnIDs: []string{"link_id"},
		}},
	}
	compiled := mustCompileTable(t, table)
	reference := pdm.Reference{
		Metadata: pdm.Metadata{ID: "self-fk", Code: "fk_self"},
		OwnerID:  "scope", ParentTableID: "node", ChildTableID: "node",
		ParentKeyID:      "uk_link",
		DeleteConstraint: pdm.OptionalText{Value: "SET NULL", Present: true},
		Joins:            []pdm.ReferenceJoin{{ParentColumnID: "link_id", ChildColumnID: "link_id"}},
	}
	_, diagnostics := compileForeignKey(compiled, compiled, reference)
	if !hasCode(diagnostics, DiagnosticForeignKeySelfColumn) {
		t.Fatalf("expected same-column self-reference diagnostic, got %+v", diagnostics)
	}
}

func TestCompileForeignKeyDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pdm.Table, *pdm.Table, *pdm.Reference)
		code   string
	}{
		{name: "table mismatch", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.ChildTableID = "other"
		}, code: DiagnosticForeignKeyTableMismatch},
		{name: "missing owner", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.OwnerID = ""
		}, code: DiagnosticForeignKeyOwnerMissing},
		{name: "missing name", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Code, ref.Name, ref.Raw = "", "", nil
		}, code: DiagnosticForeignKeyInvalidName},
		{name: "invalid name", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Code, ref.Name, ref.Raw = strings.Repeat("x", 65), "", nil
		}, code: DiagnosticForeignKeyInvalidName},
		{name: "name property conflict", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Raw = rawPDMElement("ConstraintName", "one", "Reference.ConstraintName", "two")
		}, code: DiagnosticForeignKeyInvalidName},
		{name: "empty joins", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Joins = nil
		}, code: DiagnosticForeignKeyEmpty},
		{name: "missing child column", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Joins[0].ChildColumnID = "missing"
		}, code: DiagnosticForeignKeyMissingColumn},
		{name: "missing parent column", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Joins[0].ParentColumnID = "missing"
		}, code: DiagnosticForeignKeyMissingColumn},
		{name: "duplicate child column", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.Joins[1].ChildColumnID = ref.Joins[0].ChildColumnID
		}, code: DiagnosticForeignKeyDuplicateColumn},
		{name: "type mismatch", mutate: func(child, _ *pdm.Table, _ *pdm.Reference) {
			child.Columns[1].DataType = pdm.OptionalText{Value: "BIGINT", Present: true}
		}, code: DiagnosticForeignKeyTypeMismatch},
		{name: "collation inheritance", mutate: func(child, parent *pdm.Table, _ *pdm.Reference) {
			child.Columns[0].DataType = pdm.OptionalText{
				Value: "VARCHAR(20) CHARACTER SET utf8mb4", Present: true}
			child.Columns[0].Length = pdm.OptionalText{}
			child.Columns[1].DataType = pdm.OptionalText{
				Value: "VARCHAR(20) CHARACTER SET utf8mb4", Present: true}
			child.Columns[1].Length = pdm.OptionalText{}
			parent.Columns[0].DataType = pdm.OptionalText{
				Value: "VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", Present: true}
			parent.Columns[0].Length = pdm.OptionalText{}
			parent.Columns[1].DataType = pdm.OptionalText{
				Value: "VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", Present: true}
			parent.Columns[1].Length = pdm.OptionalText{}
		}, code: DiagnosticForeignKeyTypeMismatch},
		{name: "missing parent key", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.ParentKeyID = "missing"
		}, code: DiagnosticForeignKeyMissingParentKey},
		{name: "unusable parent key", mutate: func(child, parent *pdm.Table, ref *pdm.Reference) {
			ref.ParentKeyID = ""
			parent.Keys[0].ColumnIDs = []string{"id", "tenant"}
		}, code: DiagnosticForeignKeyUnusableParentKey},
		{name: "set null on not null", mutate: func(child, _ *pdm.Table, _ *pdm.Reference) {
			child.Columns[0].Mandatory = boolPointer(true)
		}, code: DiagnosticForeignKeySetNull},
		{name: "numeric action code", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.DeleteConstraint = pdm.OptionalText{Value: "1", Present: true}
		}, code: DiagnosticForeignKeyAction},
		{name: "set default", mutate: func(_, _ *pdm.Table, ref *pdm.Reference) {
			ref.UpdateConstraint = pdm.OptionalText{Value: "SET DEFAULT", Present: true}
		}, code: DiagnosticForeignKeyAction},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			child, parent, reference := foreignKeyPair()
			tc.mutate(&child, &parent, &reference)
			compiledChild := mustCompileTableAllowErrors(child)
			compiledParent := mustCompileTableAllowErrors(parent)
			_, diagnostics := compileForeignKey(compiledChild, compiledParent, reference)
			if !hasCode(diagnostics, tc.code) {
				t.Fatalf("diagnostics = %+v, want %s", diagnostics, tc.code)
			}
		})
	}
}

func TestForeignKeyRejectsJSONTextAndBlob(t *testing.T) {
	for _, dataType := range []string{"JSON", "TEXT", "BLOB"} {
		t.Run(dataType, func(t *testing.T) {
			child, parent, reference := simpleForeignKeyPair(dataType)
			compiledChild := mustCompileTableAllowErrors(child)
			compiledParent := mustCompileTableAllowErrors(parent)
			_, diagnostics := compileForeignKey(compiledChild, compiledParent, reference)
			if !hasCode(diagnostics, DiagnosticForeignKeyTypeMismatch) {
				t.Fatalf("diagnostics = %+v, want type mismatch", diagnostics)
			}
		})
	}
}

func foreignKeyPair() (pdm.Table, pdm.Table, pdm.Reference) {
	yes, no := true, false
	parent := pdm.Table{
		Metadata: pdm.Metadata{ID: "parent", Code: "parent"},
		OwnerID:  "parent-scope",
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "tenant", Code: "tenant"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &yes},
			{Metadata: pdm.Metadata{ID: "id", Code: "id"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &yes},
		},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "pk", Code: "pk"},
			ColumnIDs: []string{"tenant", "id"},
		}},
		PrimaryKeyID: "pk",
	}
	child := pdm.Table{
		Metadata: pdm.Metadata{ID: "child", Code: "child"},
		OwnerID:  "child-scope",
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "tenant_ref", Code: "tenant_ref"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &no},
			{Metadata: pdm.Metadata{ID: "id_ref", Code: "id_ref"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}, Mandatory: &no},
		},
	}
	reference := pdm.Reference{
		Metadata: pdm.Metadata{
			ID: "fk", Code: "fk_code", Name: "FK Display",
			Raw: rawPDMElement("ConstraintName", "fk_property"),
		},
		OwnerID:          "child-scope",
		ParentTableID:    "parent",
		ChildTableID:     "child",
		ParentKeyID:      "pk",
		UpdateConstraint: pdm.OptionalText{Value: "SET NULL", Present: true},
		DeleteConstraint: pdm.OptionalText{Value: "CASCADE", Present: true},
		Joins: []pdm.ReferenceJoin{
			{ParentColumnID: "tenant", ChildColumnID: "tenant_ref"},
			{ParentColumnID: "id", ChildColumnID: "id_ref"},
		},
	}
	return child, parent, reference
}

func simpleForeignKeyPair(dataType string) (pdm.Table, pdm.Table, pdm.Reference) {
	yes := true
	table := func(id, owner string) pdm.Table {
		return pdm.Table{
			Metadata: pdm.Metadata{ID: id, Code: id},
			OwnerID:  owner,
			Columns: []pdm.Column{{
				Metadata:  pdm.Metadata{ID: id + "_value", Code: "value"},
				DataType:  pdm.OptionalText{Value: dataType, Present: true},
				Mandatory: &yes,
			}},
			Keys: []pdm.Key{{
				Metadata:  pdm.Metadata{ID: id + "_pk", Code: id + "_pk"},
				ColumnIDs: []string{id + "_value"},
			}},
			PrimaryKeyID: id + "_pk",
		}
	}
	child := table("child", "child-scope")
	parent := table("parent", "parent-scope")
	reference := pdm.Reference{
		Metadata:      pdm.Metadata{ID: "fk", Code: "fk_child_parent"},
		OwnerID:       "child-scope",
		ParentTableID: "parent",
		ChildTableID:  "child",
		ParentKeyID:   "parent_pk",
		Joins: []pdm.ReferenceJoin{{
			ParentColumnID: "parent_value",
			ChildColumnID:  "child_value",
		}},
	}
	return child, parent, reference
}

func mustCompileTable(t *testing.T, table pdm.Table) compiledTable {
	t.Helper()
	compiled, diagnostics := compileTable(table)
	if hasErrorDiagnostics(diagnostics) {
		t.Fatalf("compileTable diagnostics: %+v", diagnostics)
	}
	return compiled
}

func mustCompileTableAllowErrors(table pdm.Table) compiledTable {
	compiled, _ := compileTable(table)
	return compiled
}

func hasErrorDiagnostics(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}
