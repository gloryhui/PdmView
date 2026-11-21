package sqlgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestGoldenDatabaseMatchesStaticFiles(t *testing.T) {
	document, options := goldenDatabaseFixture()
	result := GenerateDatabase(document, options)
	if result.HasErrors() || len(result.Diagnostics) != 0 {
		t.Fatalf("golden generation errors: %+v", result.Diagnostics)
	}
	wantDatabase := readGoldenFile(t, "golden_database.sql")
	if result.SQL != wantDatabase {
		t.Fatalf("database golden mismatch\nwant:\n%s\ngot:\n%s", wantDatabase, result.SQL)
	}
	if repeat := GenerateDatabase(document, options); repeat.SQL != wantDatabase {
		t.Fatal("repeated golden generation changed bytes")
	}

	reordered := goldenDatabaseFixtureDocument()
	packages := reordered.Model.Contents.Packages
	packages[0], packages[1] = packages[1], packages[0]
	for offset := range packages {
		refs := packages[offset].Contents.References
		for left, right := 0, len(refs)-1; left < right; left, right = left+1, right-1 {
			refs[left], refs[right] = refs[right], refs[left]
		}
	}
	if reorderedResult := GenerateDatabase(reordered, options); reorderedResult.SQL != wantDatabase {
		t.Fatal("collection order changed golden database bytes")
	}

	for _, golden := range []struct {
		name string
		file string
	}{
		{name: "alpha", file: "golden_table_alpha.sql"},
		{name: "zeta", file: "golden_table_zeta.sql"},
		{name: "Sales`Table", file: "golden_table_sales.sql"},
	} {
		table := findGoldenTable(t, document.Model.Contents, golden.name)
		single := GenerateTable(table)
		if single.HasErrors() {
			t.Fatalf("%s table errors: %+v", golden.name, single.Diagnostics)
		}
		wantTable := readGoldenTable(t, golden.file)
		if single.SQL != wantTable {
			t.Fatalf("%s table golden mismatch\nwant:\n%s\ngot:\n%s",
				golden.name, wantTable, single.SQL)
		}
		if count := strings.Count(result.SQL, wantTable); count != 1 {
			t.Fatalf("%s table appears %d times in database golden", golden.name, count)
		}
	}
}

func TestGoldenFixtureObjectIDsUnique(t *testing.T) {
	document, _ := goldenDatabaseFixture()
	seen := make(map[string]string)
	addID := func(kind, id string) {
		if id == "" {
			return
		}
		if previous, ok := seen[id]; ok {
			t.Fatalf("object ID %q used by %s and %s", id, previous, kind)
		}
		seen[id] = kind
	}
	addID("model", document.Model.ID)
	var collect func(pdm.Contents)
	collect = func(contents pdm.Contents) {
		for _, pkg := range contents.Packages {
			addID("package", pkg.ID)
			collect(pkg.Contents)
		}
		for _, table := range contents.Tables {
			addID("table", table.ID)
			for _, column := range table.Columns {
				addID("column", column.ID)
			}
			for _, key := range table.Keys {
				addID("key", key.ID)
			}
			for _, index := range table.Indexes {
				addID("index", index.ID)
				for _, column := range index.Columns {
					addID("index column", column.ID)
				}
			}
		}
		for _, reference := range contents.References {
			addID("reference", reference.ID)
			for _, join := range reference.Joins {
				addID("reference join", join.ID)
			}
		}
	}
	collect(document.Model.Contents)
}

func goldenDatabaseFixture() (*pdm.Document, DatabaseOptions) {
	return goldenDatabaseFixtureDocument(), DatabaseOptions{
		Name:         "pdmview_golden",
		CharacterSet: "utf8mb4",
		Collation:    "utf8mb4_bin",
	}
}

func goldenDatabaseFixtureDocument() *pdm.Document {
	document := databaseFixture()
	alpha := &document.Model.Contents.Packages[1].Contents.Tables[0]
	zeta := &document.Model.Contents.Packages[0].Contents.Tables[0]
	alpha.ID = "golden-table-alpha"
	zeta.ID = "golden-table-zeta"
	columnRemap := map[string]string{
		"alpha-id":        "golden-alpha-id",
		"alpha-zeta-id":   "golden-alpha-zeta-id",
		"alpha-parent-id": "golden-alpha-parent-id",
		"zeta-id":         "golden-zeta-id",
		"zeta-alpha-id":   "golden-zeta-alpha-id",
	}
	keyRemap := map[string]string{
		"alpha-pk": "golden-key-alpha-pk",
		"zeta-pk":  "golden-key-zeta-pk",
	}
	tableRemap := map[string]string{
		"alpha-id": "golden-table-alpha",
		"zeta-id":  "golden-table-zeta",
	}
	for offset := range alpha.Columns {
		alpha.Columns[offset].ID = remapGoldenID(alpha.Columns[offset].ID, columnRemap)
	}
	for offset := range zeta.Columns {
		zeta.Columns[offset].ID = remapGoldenID(zeta.Columns[offset].ID, columnRemap)
	}
	for offset := range alpha.Keys {
		alpha.Keys[offset].ID = remapGoldenID(alpha.Keys[offset].ID, keyRemap)
		alpha.Keys[offset].ColumnIDs = remapGoldenIDs(alpha.Keys[offset].ColumnIDs, columnRemap)
	}
	for offset := range zeta.Keys {
		zeta.Keys[offset].ID = remapGoldenID(zeta.Keys[offset].ID, keyRemap)
		zeta.Keys[offset].ColumnIDs = remapGoldenIDs(zeta.Keys[offset].ColumnIDs, columnRemap)
	}
	alpha.PrimaryKeyID = remapGoldenID(alpha.PrimaryKeyID, keyRemap)
	zeta.PrimaryKeyID = remapGoldenID(zeta.PrimaryKeyID, keyRemap)
	for packageOffset := range document.Model.Contents.Packages {
		references := document.Model.Contents.Packages[packageOffset].Contents.References
		for referenceOffset := range references {
			reference := &references[referenceOffset]
			reference.ID = "golden-" + reference.ID
			reference.ChildTableID = remapGoldenID(reference.ChildTableID, tableRemap)
			reference.ParentTableID = remapGoldenID(reference.ParentTableID, tableRemap)
			reference.ParentKeyID = remapGoldenID(reference.ParentKeyID, keyRemap)
			for joinOffset := range reference.Joins {
				reference.Joins[joinOffset].ChildColumnID =
					remapGoldenID(reference.Joins[joinOffset].ChildColumnID, columnRemap)
				reference.Joins[joinOffset].ParentColumnID =
					remapGoldenID(reference.Joins[joinOffset].ParentColumnID, columnRemap)
			}
		}
	}
	complete := completeTable()
	complete.ID = "golden-complete-table"
	complete.OwnerID = "p2"
	complete.Columns[3].Code = "status"
	complete.Columns[2].Comment = pdm.OptionalText{
		Value: "名称 \"x\"\\line\n下一行", Present: true,
	}
	complete.Columns[2].DefaultValue = pdm.OptionalText{
		Value: "'line\nnext\\\\path'", Present: true,
	}
	nullable := false
	complete.Columns = append(complete.Columns, pdm.Column{
		Metadata:     pdm.Metadata{ID: "golden-deleted-at", Code: "deleted_at"},
		DataType:     pdm.OptionalText{Value: "DATETIME(6)", Present: true},
		DefaultValue: pdm.OptionalText{Value: "NULL", Present: true},
		Mandatory:    &nullable,
	})
	completeRemap := make(map[string]string)
	for offset := range complete.Columns {
		old := complete.Columns[offset].ID
		next := "golden-complete-" + old
		complete.Columns[offset].ID = next
		completeRemap[old] = next
	}
	if len(complete.Keys) > 1 {
		complete.Keys[1].Code = "ix-name"
	}
	for offset := range complete.Keys {
		old := complete.Keys[offset].ID
		complete.Keys[offset].ID = "golden-complete-" + old
		complete.Keys[offset].ColumnIDs = remapGoldenIDs(complete.Keys[offset].ColumnIDs, completeRemap)
	}
	complete.PrimaryKeyID = "golden-complete-" + complete.PrimaryKeyID
	complete.Indexes = []pdm.Index{{
		Metadata: pdm.Metadata{
			ID: "golden-note-index", Code: "ix_note_desc",
			Comment: pdm.OptionalText{Value: "note suffix", Present: true},
		},
		Columns: []pdm.IndexColumn{{
			Metadata:  pdm.Metadata{ID: "golden-note-index-column", Raw: rawPDMElement("Length", "16")},
			ColumnID:  "golden-complete-note",
			SortOrder: pdm.OptionalText{Value: "DESC", Present: true},
		}},
	}}
	document.Model.Contents.Packages[1].Contents.Tables =
		append(document.Model.Contents.Packages[1].Contents.Tables, complete)
	return document
}

func remapGoldenIDs(ids []string, remap map[string]string) []string {
	for offset := range ids {
		ids[offset] = remapGoldenID(ids[offset], remap)
	}
	return ids
}

func remapGoldenID(id string, remap map[string]string) string {
	if next, ok := remap[id]; ok {
		return next
	}
	return id
}

func findGoldenTable(t *testing.T, contents pdm.Contents, code string) pdm.Table {
	t.Helper()
	for _, table := range contents.Tables {
		if table.Code == code {
			return table
		}
	}
	for _, pkg := range contents.Packages {
		if table := findGoldenTableOptional(pkg.Contents, code); table != nil {
			return *table
		}
	}
	t.Fatalf("golden table %q not found", code)
	return pdm.Table{}
}

func findGoldenTableOptional(contents pdm.Contents, code string) *pdm.Table {
	for offset := range contents.Tables {
		if contents.Tables[offset].Code == code {
			return &contents.Tables[offset]
		}
	}
	for _, pkg := range contents.Packages {
		if table := findGoldenTableOptional(pkg.Contents, code); table != nil {
			return table
		}
	}
	return nil
}

func readGoldenFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readGoldenTable(t *testing.T, name string) string {
	t.Helper()
	text := readGoldenFile(t, name)
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("%s must end with a file newline", name)
	}
	return strings.TrimSuffix(text, "\n")
}
