package sqlgen

import (
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestGenerateTableIndexesDedupPrimaryAndPrefixes(t *testing.T) {
	table := indexTable()
	result := GenerateTable(table)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	wantIndexes := []string{
		"UNIQUE KEY `uk_ab` (`a`, `b`)",
		"UNIQUE KEY `ux_ab` (`a`, `b`) COMMENT 'tenant index'",
		"UNIQUE KEY `ux_ab_alt` (`a`, `b`)",
		"KEY `body_prefix` (`body`(20) DESC)",
		"KEY `bt_name` (`a` ASC) USING BTREE COMMENT 'btree name'",
	}
	for _, want := range wantIndexes {
		if !strings.Contains(result.SQL, "  "+want) {
			t.Fatalf("missing index %q:\n%s", want, result.SQL)
		}
	}
	for _, unwanted := range []string{"`pk_copy`"} {
		if strings.Contains(result.SQL, unwanted) {
			t.Fatalf("duplicate or primary index %q emitted:\n%s", unwanted, result.SQL)
		}
	}
	if strings.Count(result.SQL, "PRIMARY KEY") != 1 {
		t.Fatalf("PRIMARY error:\n%s", result.SQL)
	}
}

func TestSameNameCandidateKeyAndUniqueIndexDedup(t *testing.T) {
	table := pdm.Table{
		Metadata: pdm.Metadata{ID: "same", Code: "same"},
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "a", Code: "a"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}},
			{Metadata: pdm.Metadata{ID: "b", Code: "b"},
				DataType: pdm.OptionalText{Value: "INT", Present: true}},
		},
		Keys: []pdm.Key{{
			Metadata: pdm.Metadata{ID: "uk", Code: "uk"}, ColumnIDs: []string{"a", "b"},
		}},
		Indexes: []pdm.Index{{
			Metadata: pdm.Metadata{ID: "ux", Code: "uk"},
			Unique:   boolPointer(true),
			Columns:  []pdm.IndexColumn{{ColumnID: "a"}, {ColumnID: "b"}},
		}},
	}
	result := GenerateTable(table)
	if result.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	if strings.Count(result.SQL, "UNIQUE KEY `uk`") != 1 {
		t.Fatalf("same-name key/index was not deduplicated:\n%s", result.SQL)
	}
}

func TestGenerateTableIndexDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pdm.Table)
		code   string
	}{
		{name: "missing ID", mutate: func(table *pdm.Table) { table.Indexes[0].ID = "" }, code: DiagnosticMissingIndexID},
		{name: "duplicate ID", mutate: func(table *pdm.Table) {
			table.Indexes[1].ID = table.Indexes[0].ID
		}, code: DiagnosticDuplicateIndexID},
		{name: "missing name", mutate: func(table *pdm.Table) {
			table.Indexes[1].Code, table.Indexes[1].Name, table.Indexes[1].ID = "", "", ""
		}, code: DiagnosticInvalidIndexIdentifier},
		{name: "empty columns", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns = nil
		}, code: DiagnosticEmptyIndex},
		{name: "missing column", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns[0].ColumnID = "missing"
		}, code: DiagnosticMissingIndexColumn},
		{name: "duplicate column", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns = append(table.Indexes[1].Columns, table.Indexes[1].Columns[0])
		}, code: DiagnosticDuplicateIndexColumn},
		{name: "bad direction", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns[0].SortOrder = pdm.OptionalText{Value: "SIDEWAYS", Present: true}
		}, code: DiagnosticInvalidIndexDirection},
		{name: "unsupported type", mutate: func(table *pdm.Table) {
			table.Indexes[1].Raw = rawPDMElement("Index.Type", "HASH")
		}, code: DiagnosticUnsupportedIndexType},
		{name: "type conflict", mutate: func(table *pdm.Table) {
			table.Indexes[1].Raw = rawPDMElement("Index.Type", "BTREE", "IndexType", "HASH")
		}, code: DiagnosticUnsupportedIndexType},
		{name: "prefix on integer", mutate: func(table *pdm.Table) {
			table.Indexes[2].Columns[0].Raw = rawPDMElement("Length", "4")
		}, code: DiagnosticUnsupportedIndexPrefix},
		{name: "prefix too long", mutate: func(table *pdm.Table) {
			table.Indexes[3].Columns[0].ColumnID = "a"
			table.Indexes[3].Columns[0].Raw = rawPDMElement("Length", "21")
		}, code: DiagnosticIndexPrefixOutOfRange},
		{name: "text prefix required", mutate: func(table *pdm.Table) {
			table.Indexes[3].Columns[0].Raw = nil
		}, code: DiagnosticIndexPrefixRequired},
		{name: "bad prefix token", mutate: func(table *pdm.Table) {
			table.Indexes[3].Columns[0].Raw = rawPDMElement("Length", "20;DROP")
		}, code: DiagnosticInvalidIndexPrefix},
		{name: "duplicate incompatible name", mutate: func(table *pdm.Table) {
			table.Indexes[1].Code = table.Indexes[0].Code
			table.Indexes[1].Unique = boolPointer(false)
		}, code: DiagnosticDuplicateIndexIdentifier},
		{name: "json BTREE", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns = []pdm.IndexColumn{{ColumnID: "json_value"}}
		}, code: DiagnosticUnsupportedIndexPrefix},
		{name: "too many columns", mutate: func(table *pdm.Table) {
			table.Indexes[1].Columns = nil
			for offset := 0; offset < 17; offset++ {
				table.Indexes[1].Columns = append(table.Indexes[1].Columns,
					pdm.IndexColumn{ColumnID: "id"})
			}
		}, code: DiagnosticIndexColumnLimit},
		{name: "key too long", mutate: func(table *pdm.Table) {
			table.Indexes[3].Columns[0].Raw = rawPDMElement("Length", "4000")
		}, code: DiagnosticIndexKeyTooLong},
		{name: "unsupported physical option", mutate: func(table *pdm.Table) {
			table.Indexes[1].Metadata.Raw = rawPDMElement("Index.Algorithm", "INPLACE")
		}, code: DiagnosticUnsupportedIndexOption},
		{name: "unsupported column option", mutate: func(table *pdm.Table) {
			table.Indexes[0].Columns[0].Metadata.Raw = rawPDMElement("IndexColumn.OperatorClass", "text_ops")
		}, code: DiagnosticUnsupportedIndexOption},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := indexTable()
			tc.mutate(&table)
			result := GenerateTable(table)
			if result.SQL != "" || !hasCode(result.Diagnostics, tc.code) {
				t.Fatalf("result = %+v, want %s", result, tc.code)
			}
		})
	}
}

func TestKeyBTREELimits(t *testing.T) {
	tests := []struct {
		name  string
		table pdm.Table
		code  string
	}{
		{name: "text primary", table: keyLimitTable("TEXT"), code: DiagnosticIndexPrefixRequired},
		{name: "json primary", table: keyLimitTable("JSON"), code: DiagnosticUnsupportedIndexPrefix},
		{name: "long varchar primary", table: keyLimitTable("VARCHAR(4000)"), code: DiagnosticIndexKeyTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateTable(tc.table)
			if !hasCode(result.Diagnostics, tc.code) {
				t.Fatalf("diagnostics = %+v, want %s", result.Diagnostics, tc.code)
			}
		})
	}

	table := pdm.Table{Metadata: pdm.Metadata{ID: "wide", Code: "wide"}}
	var ids []string
	for offset := 0; offset < 17; offset++ {
		id := "c" + strconv.Itoa(offset)
		ids = append(ids, id)
		table.Columns = append(table.Columns, pdm.Column{
			Metadata: pdm.Metadata{ID: id, Code: id},
			DataType: pdm.OptionalText{Value: "INT", Present: true},
		})
	}
	table.Keys = []pdm.Key{{Metadata: pdm.Metadata{ID: "pk", Code: "pk"}, ColumnIDs: ids}}
	table.PrimaryKeyID = "pk"
	if result := GenerateTable(table); !hasCode(result.Diagnostics, DiagnosticIndexColumnLimit) {
		t.Fatalf("17-column key diagnostics = %+v", result.Diagnostics)
	}
}

func TestAutoIncrementUsesEmittedLeadingAscendingUnprefixedIndex(t *testing.T) {
	tests := []struct {
		name    string
		index   pdm.Index
		errCode string
	}{
		{
			name: "ascending",
			index: pdm.Index{
				Metadata: pdm.Metadata{ID: "ix", Code: "ix"},
				Columns:  []pdm.IndexColumn{{ColumnID: "id"}},
			},
		},
		{
			name: "descending",
			index: pdm.Index{
				Metadata: pdm.Metadata{ID: "ix", Code: "ix"},
				Columns: []pdm.IndexColumn{{
					ColumnID: "id", SortOrder: pdm.OptionalText{Value: "DESC", Present: true},
				}},
			},
			errCode: DiagnosticAutoIncrementNotKey,
		},
		{
			name: "prefixed",
			index: pdm.Index{
				Metadata: pdm.Metadata{ID: "ix", Code: "ix"},
				Columns: []pdm.IndexColumn{{
					ColumnID: "a", Metadata: pdm.Metadata{Raw: rawPDMElement("Length", "2")},
				}},
			},
			errCode: DiagnosticAutoIncrementNotKey,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := pdm.Table{
				Metadata: pdm.Metadata{ID: "table", Code: "t"},
				Columns: []pdm.Column{
					{Metadata: pdm.Metadata{ID: "id", Code: "id"}, DataType: pdm.OptionalText{Value: "INT", Present: true},
						Mandatory: boolPointer(true), Identity: boolPointer(true)},
					{Metadata: pdm.Metadata{ID: "a", Code: "a"}, DataType: pdm.OptionalText{Value: "VARCHAR", Present: true},
						Length: presentInt(10), Mandatory: boolPointer(true)},
				},
				Indexes: []pdm.Index{tc.index},
			}
			result := GenerateTable(table)
			if tc.errCode != "" {
				if !hasCode(result.Diagnostics, tc.errCode) {
					t.Fatalf("diagnostics = %+v, want %s", result.Diagnostics, tc.errCode)
				}
			} else {
				if result.HasErrors() {
					t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
				}
				if !strings.Contains(result.SQL, "KEY `ix` (`id`)") {
					t.Fatalf("eligible index not emitted:\n%s", result.SQL)
				}
			}
		})
	}
}

func indexTable() pdm.Table {
	return pdm.Table{
		Metadata: pdm.Metadata{ID: "table", ObjectID: "table-object", Code: "idx_table"},
		OwnerID:  "owner",
		Columns: []pdm.Column{
			{Metadata: pdm.Metadata{ID: "id", Code: "id"}, DataType: pdm.OptionalText{Value: "INT", Present: true}},
			{Metadata: pdm.Metadata{ID: "a", Code: "a"}, DataType: pdm.OptionalText{Value: "VARCHAR", Present: true},
				Length: presentInt(20)},
			{Metadata: pdm.Metadata{ID: "b", Code: "b"}, DataType: pdm.OptionalText{Value: "VARCHAR", Present: true},
				Length: presentInt(30)},
			{Metadata: pdm.Metadata{ID: "body", Code: "body"}, DataType: pdm.OptionalText{Value: "TEXT", Present: true}},
			{Metadata: pdm.Metadata{ID: "json_value", Code: "json_value"},
				DataType: pdm.OptionalText{Value: "JSON", Present: true}},
		},
		Keys: []pdm.Key{
			{Metadata: pdm.Metadata{ID: "pk", Code: "pk_copy"}, ColumnIDs: []string{"id"}},
			{Metadata: pdm.Metadata{ID: "uk_ab", Code: "uk_ab"}, ColumnIDs: []string{"a", "b"}},
		},
		PrimaryKeyID: "pk",
		Indexes: []pdm.Index{
			{
				Metadata: pdm.Metadata{ID: "ux_ab", Code: "ux_ab", Comment: pdm.OptionalText{
					Value: "tenant index", Present: true}},
				Unique:  boolPointer(true),
				Columns: []pdm.IndexColumn{{ColumnID: "a"}, {ColumnID: "b"}},
			},
			{
				Metadata: pdm.Metadata{ID: "ux_ab_alt", Code: "ux_ab_alt"},
				Unique:   boolPointer(true),
				Columns:  []pdm.IndexColumn{{ColumnID: "a"}, {ColumnID: "b"}},
			},
			{
				Metadata: pdm.Metadata{ID: "pk_copy", Code: "pk_copy"},
				Unique:   boolPointer(true),
				Columns:  []pdm.IndexColumn{{ColumnID: "id"}},
			},
			{
				Metadata: pdm.Metadata{ID: "body_prefix", Code: "body_prefix"},
				Columns: []pdm.IndexColumn{{
					ColumnID:  "body",
					Metadata:  pdm.Metadata{Raw: rawPDMElement("Length", "20")},
					SortOrder: pdm.OptionalText{Value: "DESC", Present: true},
				}},
			},
			{
				Metadata: pdm.Metadata{ID: "bt_name", Code: "bt_name", Comment: pdm.OptionalText{
					Value: "btree name", Present: true}, Raw: rawPDMElement("Index.Type", "BTREE")},
				Columns: []pdm.IndexColumn{
					{ColumnID: "a", SortOrder: pdm.OptionalText{Value: "ASC", Present: true}},
				},
			},
		},
	}
}

func keyLimitTable(dataType string) pdm.Table {
	return pdm.Table{
		Metadata: pdm.Metadata{ID: "limited", Code: "limited"},
		Columns: []pdm.Column{{
			Metadata: pdm.Metadata{ID: "value", Code: "value"},
			DataType: pdm.OptionalText{Value: dataType, Present: true},
		}},
		Keys: []pdm.Key{{
			Metadata: pdm.Metadata{ID: "pk", Code: "pk"}, ColumnIDs: []string{"value"},
		}},
		PrimaryKeyID: "pk",
	}
}

func rawPDMElement(properties ...string) *pdm.Element {
	element := &pdm.Element{Name: xml.Name{Local: "Raw"}}
	for offset := 0; offset+1 < len(properties); offset += 2 {
		element.Children = append(element.Children, &pdm.Element{
			Name: xml.Name{Space: pdm.NamespaceAttribute, Local: properties[offset]},
			Text: properties[offset+1],
		})
	}
	return element
}
