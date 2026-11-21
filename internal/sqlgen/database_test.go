package sqlgen

import (
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestGenerateDatabaseExactFullScript(t *testing.T) {
	document := databaseFixture()
	plan, diagnostics := compileDatabase(document)
	if len(diagnostics) != 0 {
		t.Fatalf("plan diagnostics: %+v", diagnostics)
	}
	result := GenerateDatabase(document, DatabaseOptions{
		Name: "sales_db", CharacterSet: "UTF8MB4", Collation: "UTF8MB4_BIN",
	})
	if result.HasErrors() {
		t.Fatalf("generate diagnostics: %+v", result.Diagnostics)
	}
	statements := []string{
		"CREATE DATABASE IF NOT EXISTS `sales_db` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;",
		"USE `sales_db`;",
	}
	statements = append(statements, plan.tables...)
	for _, foreignKey := range plan.foreignKeys {
		statements = append(statements, foreignKey.AlterSQL())
	}
	want := strings.Join(statements, "\n\n") + "\n"
	if result.SQL != want {
		t.Fatalf("database SQL mismatch\nwant:\n%s\ngot:\n%s", want, result.SQL)
	}
	if !strings.HasSuffix(result.SQL, ";\n") || strings.Contains(result.SQL, "\n\n\n") {
		t.Fatalf("blank-line/trailing-newline contract broken")
	}
	for _, tableSQL := range plan.tables {
		if strings.Count(result.SQL, tableSQL) != 1 {
			t.Fatalf("table DDL not byte-identical exactly once:\n%s", tableSQL)
		}
	}
	alphaDirect := GenerateTable(document.Model.Contents.Packages[1].Contents.Tables[0]).SQL
	zetaDirect := GenerateTable(document.Model.Contents.Packages[0].Contents.Tables[0]).SQL
	if plan.tables[0] != alphaDirect || plan.tables[1] != zetaDirect {
		t.Fatal("database table DDL differs from single-table DDL")
	}
}

func TestGenerateDatabaseStableAcrossCallsAndCollectionOrder(t *testing.T) {
	first := databaseFixture()
	options := DatabaseOptions{Name: "stable_db", CharacterSet: "utf8mb4"}
	firstResult := GenerateDatabase(first, options)
	if firstResult.HasErrors() {
		t.Fatalf("first diagnostics: %+v", firstResult.Diagnostics)
	}
	if repeat := GenerateDatabase(first, options); repeat.SQL != firstResult.SQL {
		t.Fatal("repeated generation changed bytes")
	}

	second := databaseFixture()
	packages := second.Model.Contents.Packages
	packages[0], packages[1] = packages[1], packages[0]
	for offset := range packages {
		references := packages[offset].Contents.References
		for left, right := 0, len(references)-1; left < right; left, right = left+1, right-1 {
			references[left], references[right] = references[right], references[left]
		}
	}
	secondResult := GenerateDatabase(second, options)
	if secondResult.HasErrors() {
		t.Fatalf("second diagnostics: %+v", secondResult.Diagnostics)
	}
	if secondResult.SQL != firstResult.SQL {
		t.Fatalf("collection order changed database SQL\nfirst:\n%s\nsecond:\n%s",
			firstResult.SQL, secondResult.SQL)
	}
}

func TestGenerateDatabaseNameSelectionAndValidation(t *testing.T) {
	tests := []struct {
		name     string
		options  DatabaseOptions
		mutate   func(*pdm.Document)
		contains string
		warnCode string
		errCode  string
	}{
		{
			name: "explicit name wins", options: DatabaseOptions{Name: "explicit_db"},
			contains: "CREATE DATABASE IF NOT EXISTS `explicit_db`;",
		},
		{
			name: "model code", contains: "CREATE DATABASE IF NOT EXISTS `model`;",
		},
		{
			name: "model name fallback",
			mutate: func(document *pdm.Document) {
				document.Model.Code = ""
				document.Model.Name = "Display Model"
			},
			contains: "CREATE DATABASE IF NOT EXISTS `Display Model`;",
			warnCode: DiagnosticDatabaseNameFallback,
		},
		{
			name: "empty name",
			mutate: func(document *pdm.Document) {
				document.Model.Code = ""
				document.Model.Name = ""
			},
			errCode: DiagnosticDatabaseName,
		},
		{
			name: "system name", options: DatabaseOptions{Name: "information_schema"},
			errCode: DiagnosticDatabaseSystemName,
		},
		{
			name: "dot boundary", options: DatabaseOptions{Name: "sales.prod"},
			errCode: DiagnosticDatabaseNameBoundary,
		},
		{
			name: "invalid identifier", options: DatabaseOptions{Name: strings.Repeat("x", 65)},
			errCode: DiagnosticDatabaseName,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			document := databaseFixture()
			if tc.mutate != nil {
				tc.mutate(document)
			}
			result := GenerateDatabase(document, tc.options)
			if tc.errCode != "" {
				if result.SQL != "" || !hasCode(result.Diagnostics, tc.errCode) {
					t.Fatalf("result = %+v, want %s", result, tc.errCode)
				}
				return
			}
			if result.HasErrors() || !strings.Contains(result.SQL, tc.contains) {
				t.Fatalf("result = %+v, want %q", result, tc.contains)
			}
			if tc.warnCode != "" && !hasCode(result.Diagnostics, tc.warnCode) {
				t.Fatalf("warning = %+v, want %s", result.Diagnostics, tc.warnCode)
			}
		})
	}
}

func TestGenerateDatabaseCharacterOptions(t *testing.T) {
	tests := []struct {
		name      string
		options   DatabaseOptions
		firstLine string
		errCode   string
	}{
		{
			name: "charset only", options: DatabaseOptions{Name: "db", CharacterSet: "ascii"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET ascii;",
		},
		{
			name: "collation only", options: DatabaseOptions{Name: "db", Collation: "utf8mb4_bin"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` COLLATE utf8mb4_bin;",
		},
		{
			name: "both", options: DatabaseOptions{
				Name: "db", CharacterSet: "utf8mb4", Collation: "utf8mb4_0900_ai_ci"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;",
		},
		{
			name: "utf8mb3 historical prefix", options: DatabaseOptions{
				Name: "db", CharacterSet: "utf8mb3", Collation: "utf8_general_ci"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET utf8mb3 COLLATE utf8_general_ci;",
		},
		{
			name: "modern utf8mb3 prefix", options: DatabaseOptions{
				Name: "db", CharacterSet: "UTF8", Collation: "UTF8MB3_GENERAL_CI"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET utf8 COLLATE utf8mb3_general_ci;",
		},
		{
			name: "binary", options: DatabaseOptions{
				Name: "db", CharacterSet: "binary", Collation: "binary"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET binary COLLATE binary;",
		},
		{
			name: "obvious prefix accepted availability deferred",
			options: DatabaseOptions{
				Name: "db", CharacterSet: "utf8mb4", Collation: "utf8mb4_future_ci"},
			firstLine: "CREATE DATABASE IF NOT EXISTS `db` CHARACTER SET utf8mb4 COLLATE utf8mb4_future_ci;",
		},
		{
			name: "unknown charset", options: DatabaseOptions{Name: "db", CharacterSet: "notmysql"},
			errCode: DiagnosticDatabaseCharacterSet,
		},
		{
			name: "invalid charset token", options: DatabaseOptions{Name: "db", CharacterSet: "utf8mb4;DROP"},
			errCode: DiagnosticDatabaseCharacterSet,
		},
		{
			name: "non-ASCII charset token", options: DatabaseOptions{Name: "db", CharacterSet: "gbK"},
			errCode: DiagnosticDatabaseCharacterSet,
		},
		{
			name: "unknown collation namespace", options: DatabaseOptions{Name: "db", Collation: "oracle_bin"},
			errCode: DiagnosticDatabaseCollation,
		},
		{
			name: "invalid collation token", options: DatabaseOptions{Name: "db", Collation: "utf8mb4_bin;DROP"},
			errCode: DiagnosticDatabaseCollation,
		},
		{
			name: "overlong collation token", options: DatabaseOptions{Name: "db", Collation: "utf8mb4_" + strings.Repeat("a", 60)},
			errCode: DiagnosticDatabaseCollation,
		},
		{
			name: "obvious mismatch", options: DatabaseOptions{
				Name: "db", CharacterSet: "latin1", Collation: "utf8mb4_bin"},
			errCode: DiagnosticDatabaseOptionMismatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateDatabase(databaseFixture(), tc.options)
			if tc.errCode != "" {
				if result.SQL != "" || !hasCode(result.Diagnostics, tc.errCode) {
					t.Fatalf("result = %+v, want %s", result, tc.errCode)
				}
				return
			}
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
			}
			first := strings.SplitN(result.SQL, "\n", 2)[0]
			if first != tc.firstLine {
				t.Fatalf("first statement = %q, want %q", first, tc.firstLine)
			}
		})
	}
}

func TestGenerateDatabaseErrorsSuppressAllOutput(t *testing.T) {
	document := databaseFixture()
	document.Model.Contents.Packages[1].Contents.Tables[0].ID =
		document.Model.Contents.Packages[0].Contents.Tables[0].ID
	result := GenerateDatabase(document, DatabaseOptions{
		Name: "db", CharacterSet: "utf8mb4", Collation: "utf8mb4_bin",
	})
	if result.SQL != "" || !hasCode(result.Diagnostics, DiagnosticConflictingTableID) {
		t.Fatalf("partial output on plan error: %+v", result)
	}

	result = GenerateDatabase(databaseFixture(), DatabaseOptions{
		Name: "db", CharacterSet: "notmysql",
	})
	if result.SQL != "" || !hasCode(result.Diagnostics, DiagnosticDatabaseCharacterSet) {
		t.Fatalf("partial output on option error: %+v", result)
	}

	result = GenerateDatabase(nil, DatabaseOptions{Name: "db"})
	if result.SQL != "" || !hasCode(result.Diagnostics, DiagnosticMissingDocument) {
		t.Fatalf("nil document result: %+v", result)
	}
}
