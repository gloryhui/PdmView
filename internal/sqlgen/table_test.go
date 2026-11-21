package sqlgen

import (
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestGenerateTableCompleteOrderedDDL(t *testing.T) {
	table := completeTable()
	result := GenerateTable(table)
	if result.HasErrors() {
		t.Fatalf("unexpected errors: %+v", result.Diagnostics)
	}
	want := "CREATE TABLE `Sales``Table` (\n" +
		"  `tenant` INT NOT NULL COMMENT '',\n" +
		"  `id` INT NOT NULL AUTO_INCREMENT,\n" +
		"  `display``name` VARCHAR(20) NULL DEFAULT 'O''Brien' COMMENT '名称 \"x\"',\n" +
		"  `status` CHAR(1) NULL DEFAULT '',\n" +
		"  `created` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),\n" +
		"  `note` TEXT NULL DEFAULT ('hello'),\n" +
		"  `payload` JSON NULL DEFAULT (JSON_ARRAY()),\n" +
		"  `amount` DECIMAL(20,2) NULL DEFAULT 12.30,\n" +
		"  PRIMARY KEY (`id`, `tenant`),\n" +
		"  UNIQUE KEY `ix-name` (`display``name`, `tenant`)\n" +
		") ENGINE=InnoDB COMMENT='客户表';"
	if result.SQL != want {
		t.Fatalf("SQL mismatch\nwant:\n%s\n\ngot:\n%s", want, result.SQL)
	}
	if len(result.Diagnostics) != 1 ||
		result.Diagnostics[0].Code != DiagnosticColumnIdentifierFallback ||
		result.Diagnostics[0].Severity != SeverityWarning {
		t.Fatalf("warnings = %+v", result.Diagnostics)
	}
}

func TestGenerateTableDefaults(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name     string
		column   pdm.Column
		nullable *bool
		contains string
		errCode  string
	}{
		{name: "absent", column: sqlTestColumn("INT"), nullable: &no},
		{name: "empty text", column: sqlTestColumn("VARCHAR(10)", sqlDefault("")), nullable: &no, contains: "DEFAULT ''"},
		{name: "unquoted text", column: sqlTestColumn("VARCHAR(10)", sqlDefault("hello")), nullable: &no, contains: "DEFAULT 'hello'"},
		{name: "quoted text", column: sqlTestColumn("VARCHAR(20)", sqlDefault(`'O''Brien\\path\n'`)), nullable: &no, contains: `DEFAULT 'O''Brien\\path\n'`},
		{name: "integer", column: sqlTestColumn("INT", sqlDefault("42")), nullable: &no, contains: "DEFAULT 42"},
		{name: "numeric bool", column: sqlTestColumn("TINYINT", sqlDefault("TRUE")), nullable: &no, contains: "DEFAULT 1"},
		{name: "nullable NULL", column: sqlTestColumn("INT", sqlDefault("NULL")), nullable: &no, contains: "DEFAULT NULL"},
		{name: "quoted NULL text", column: sqlTestColumn("VARCHAR(4)", sqlDefault("'NULL'")), nullable: &no, contains: "DEFAULT 'NULL'"},
		{name: "current date", column: sqlTestColumn("DATE", sqlDefault("CURRENT_DATE")), nullable: &no, contains: "DEFAULT (CURRENT_DATE)"},
		{name: "current time fsp", column: sqlTestColumn("TIME(3)", sqlDefault("CURRENT_TIME(3)")), nullable: &no, contains: "DEFAULT (CURRENT_TIME(3))"},
		{name: "current timestamp shorthand", column: sqlTestColumn("DATETIME(6)", sqlDefault("CURRENT_TIMESTAMP(6)")), nullable: &no, contains: "DEFAULT CURRENT_TIMESTAMP(6)"},
		{name: "now expression", column: sqlTestColumn("DATETIME(6)", sqlDefault("NOW(6)")), nullable: &no, contains: "DEFAULT (NOW(6))"},
		{name: "utc timestamp expression", column: sqlTestColumn("DATETIME(6)", sqlDefault("UTC_TIMESTAMP(6)")), nullable: &no, contains: "DEFAULT (UTC_TIMESTAMP(6))"},
		{name: "numeric char text", column: sqlTestColumn("CHAR(1)", sqlDefault("0")), nullable: &no, contains: "DEFAULT '0'"},
		{name: "numeric varchar text", column: sqlTestColumn("VARCHAR(2)", sqlDefault("1")), nullable: &no, contains: "DEFAULT '1'"},
		{name: "uuid expression", column: sqlTestColumn("VARCHAR(36)", sqlDefault("UUID()")), nullable: &no, contains: "DEFAULT (UUID())"},
		{name: "pi expression", column: sqlTestColumn("DOUBLE", sqlDefault("PI()")), nullable: &no, contains: "DEFAULT (PI())"},
		{name: "json array", column: sqlTestColumn("JSON", sqlDefault("JSON_ARRAY()")), nullable: &no, contains: "DEFAULT (JSON_ARRAY())"},
		{name: "text literal parentheses", column: sqlTestColumn("TEXT", sqlDefault("'hello'")), nullable: &no, contains: "DEFAULT ('hello')"},
		{name: "blob literal parentheses", column: sqlTestColumn("BLOB", sqlDefault("'bytes'")), nullable: &no, contains: "DEFAULT ('bytes')"},
		{name: "json literal parentheses", column: sqlTestColumn("JSON", sqlDefault("'{}'")), nullable: &no, contains: "DEFAULT ('{}')"},
		{name: "wrapped date", column: sqlTestColumn("DATE", sqlDefault("(CURRENT_DATE)")), nullable: &no, contains: "DEFAULT (CURRENT_DATE)"},
		{name: "null not nullable", column: sqlTestColumn("INT", sqlDefault("NULL")), nullable: &yes, errCode: DiagnosticDefaultTypeMismatch},
		{name: "number on text", column: sqlTestColumn("TEXT", sqlDefault("1")), nullable: &no, contains: "DEFAULT ('1')"},
		{name: "whitespace text stays unchanged", column: sqlTestColumn("VARCHAR(2)", sqlDefault("  ")), nullable: &no, contains: "DEFAULT '  '"},
		{name: "boolean spelling stays unchanged in text", column: sqlTestColumn("VARCHAR(4)", sqlDefault("true")), nullable: &no, contains: "DEFAULT 'true'"},
		{name: "wildcard escapes in literal", column: sqlTestColumn("VARCHAR(10)", sqlDefault(`'\%\_'`)), nullable: &no, contains: `DEFAULT '\\%\\_'`},
		{name: "oversize default", column: sqlTestColumn("VARCHAR(2)", sqlDefault("'中文本'")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "invalid literal JSON", column: sqlTestColumn("JSON", sqlDefault("'oops'")), nullable: &no, errCode: DiagnosticInvalidDefault},
		{name: "text on integer", column: sqlTestColumn("INT", sqlDefault("'1'")), nullable: &no, errCode: DiagnosticDefaultTypeMismatch},
		{name: "signed overflow", column: sqlTestColumn("TINYINT", sqlDefault("128")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "unsigned negative", column: sqlTestColumn("INT UNSIGNED", sqlDefault("-1")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "decimal integer overflow", column: sqlTestColumn("DECIMAL(5,2)", sqlDefault("1000")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "decimal scale overflow", column: sqlTestColumn("DECIMAL(5,2)", sqlDefault("1.234")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "decimal unsigned negative", column: sqlTestColumn("DECIMAL(5,2) UNSIGNED", sqlDefault("-1.00")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "double unsigned negative", column: sqlTestColumn("DOUBLE UNSIGNED", sqlDefault("-1.5")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "fsp mismatch", column: sqlTestColumn("DATETIME(6)", sqlDefault("CURRENT_TIMESTAMP")), nullable: &no, errCode: DiagnosticDefaultFractionalPrecision},
		{name: "arithmetic", column: sqlTestColumn("INT", sqlDefault("1+1")), nullable: &no, errCode: DiagnosticUnsupportedDefaultExpression},
		{name: "unknown function", column: sqlTestColumn("INT", sqlDefault("foo()")), nullable: &no, errCode: DiagnosticUnsupportedDefaultExpression},
		{name: "current date arithmetic", column: sqlTestColumn("DATE", sqlDefault("CURRENT_DATE + 1")), nullable: &no, errCode: DiagnosticUnsupportedDefaultExpression},
		{name: "unterminated text", column: sqlTestColumn("VARCHAR(20)", sqlDefault("'abc")), nullable: &no, errCode: DiagnosticInvalidDefault},
		{name: "unknown escape", column: sqlTestColumn("VARCHAR(20)", sqlDefault(`'a\q'`)), nullable: &no, errCode: DiagnosticInvalidDefault},
		{name: "multiword ambiguity", column: sqlTestColumn("VARCHAR(20)", sqlDefault("hello world")), nullable: &no, errCode: DiagnosticAmbiguousDefault},
		{name: "nested expression", column: sqlTestColumn("INT", sqlDefault("((1))")), nullable: &no, errCode: DiagnosticAmbiguousDefault},
		{name: "year out of range", column: sqlTestColumn("YEAR", sqlDefault("1800")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
		{name: "bit width", column: sqlTestColumn("BIT(4)", sqlDefault("16")), nullable: &no, errCode: DiagnosticDefaultOutOfRange},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.column.Mandatory = tc.nullable
			result := GenerateTable(sqlSingleColumnTable(tc.column))
			if tc.errCode != "" {
				if result.SQL != "" || !hasCode(result.Diagnostics, tc.errCode) {
					t.Fatalf("result = %+v, want error %s", result, tc.errCode)
				}
				return
			}
			if result.HasErrors() {
				t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
			}
			if tc.contains == "" {
				if strings.Contains(result.SQL, "DEFAULT") {
					t.Fatalf("absent default emitted SQL:\n%s", result.SQL)
				}
			} else if !strings.Contains(result.SQL, tc.contains) {
				t.Fatalf("SQL missing %q:\n%s", tc.contains, result.SQL)
			}
		})
	}
}

func TestGenerateTableStructuralDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*pdm.Table)
		code   string
	}{
		{name: "missing table ID", mutate: func(table *pdm.Table) { table.ID = "" }, code: DiagnosticMissingTableID},
		{name: "empty table", mutate: func(table *pdm.Table) { table.Columns = nil }, code: DiagnosticEmptyTable},
		{name: "missing column ID", mutate: func(table *pdm.Table) { table.Columns[0].ID = "" }, code: DiagnosticMissingColumnID},
		{name: "duplicate column ID", mutate: func(table *pdm.Table) {
			table.Columns[1].ID = table.Columns[0].ID
		}, code: DiagnosticDuplicateColumnID},
		{name: "duplicate column identifier", mutate: func(table *pdm.Table) {
			table.Columns[1].Code = table.Columns[0].Code
		}, code: DiagnosticDuplicateColumnIdentifier},
		{name: "duplicate column identifier case insensitive", mutate: func(table *pdm.Table) {
			table.Columns[1].Code = strings.ToUpper(table.Columns[0].Code)
		}, code: DiagnosticDuplicateColumnIdentifier},
		{name: "missing key ID", mutate: func(table *pdm.Table) { table.Keys[1].ID = "" }, code: DiagnosticMissingKeyID},
		{name: "duplicate key ID", mutate: func(table *pdm.Table) {
			table.Keys[1].ID = table.Keys[0].ID
		}, code: DiagnosticDuplicateKeyID},
		{name: "empty key", mutate: func(table *pdm.Table) { table.Keys[1].ColumnIDs = nil }, code: DiagnosticEmptyKey},
		{name: "missing key column", mutate: func(table *pdm.Table) {
			table.Keys[1].ColumnIDs = []string{"missing"}
		}, code: DiagnosticMissingKeyColumn},
		{name: "duplicate key column", mutate: func(table *pdm.Table) {
			table.Keys[1].ColumnIDs = []string{"display-name", "display-name"}
		}, code: DiagnosticDuplicateKeyColumn},
		{name: "missing primary key", mutate: func(table *pdm.Table) {
			table.PrimaryKeyID = "missing"
		}, code: DiagnosticMissingPrimaryKey},
		{name: "nullable primary key", mutate: func(table *pdm.Table) {
			makeNullable(&table.Columns[1])
		}, code: DiagnosticNullablePrimaryKey},
		{name: "unknown type", mutate: func(table *pdm.Table) {
			table.Columns[0].DataType = pdm.OptionalText{Value: "NUMBER", Present: true}
		}, code: DiagnosticUnknownColumnType},
		{name: "multiple auto increment", mutate: func(table *pdm.Table) {
			makeIdentity(&table.Columns[0])
			table.Keys[1].ColumnIDs = []string{"tenant", "display-name"}
		}, code: DiagnosticMultipleAutoIncrement},
		{name: "auto increment type", mutate: func(table *pdm.Table) {
			makeIdentity(&table.Columns[2])
			table.Keys[1].ColumnIDs = []string{"display-name", "tenant"}
			table.Columns[2].Mandatory = boolPointer(true)
		}, code: DiagnosticUnsupportedAutoIncrement},
		{name: "auto increment nullable", mutate: func(table *pdm.Table) {
			makeIdentity(&table.Columns[0])
			table.Keys[0].ColumnIDs = []string{"tenant", "id"}
			table.Columns[0].Mandatory = boolPointer(false)
		}, code: DiagnosticNullableAutoIncrement},
		{name: "auto increment default", mutate: func(table *pdm.Table) {
			table.Columns[1].DefaultValue = pdm.OptionalText{Value: "1", Present: true}
		}, code: DiagnosticAutoIncrementDefault},
		{name: "auto increment not key", mutate: func(table *pdm.Table) {
			makeIdentity(&table.Columns[0])
			table.PrimaryKeyID = ""
			table.Keys = nil
			table.Columns[0].Mandatory = boolPointer(true)
		}, code: DiagnosticAutoIncrementNotKey},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := completeTable()
			tc.mutate(&table)
			result := GenerateTable(table)
			if result.SQL != "" || !hasCode(result.Diagnostics, tc.code) {
				t.Fatalf("result = %+v, want %s", result, tc.code)
			}
		})
	}
}

func TestGenerateTableIdentifierRulesAndEscaping(t *testing.T) {
	if got := quoteIdentifier("a`b"); got != "`a``b`" {
		t.Fatalf("quoted identifier = %q", got)
	}
	if got := quoteSQLString("客户 'x'\\line\n\t\b\x00\x1a"); got != "'客户 ''x''\\\\line\\n\\t\\b\\0\\Z'" {
		t.Fatalf("quoted string = %q", got)
	}
	if err := validateIdentifier(strings.Repeat("中", 64)); err != nil {
		t.Fatalf("64 BMP characters rejected: %v", err)
	}
	for _, invalid := range []string{
		strings.Repeat("a", 65),
		"a\x00b",
		"name ",
		"😀",
	} {
		if err := validateIdentifier(invalid); err == nil {
			t.Errorf("identifier %q was accepted", invalid)
		}
	}
}

func TestGenerateTableWarningDoesNotBlockAndRawLocationFlows(t *testing.T) {
	table := completeTable()
	table.Code = ""
	table.Name = "Fallback Table"
	result := GenerateTable(table)
	if result.HasErrors() || !strings.Contains(result.SQL, "CREATE TABLE `Fallback Table`") ||
		!hasCode(result.Diagnostics, DiagnosticTableIdentifierFallback) {
		t.Fatalf("fallback result = %+v", result)
	}

	table = completeTable()
	table.Columns[0].DataType = pdm.OptionalText{Value: "UNKNOWN_TYPE", Present: true}
	table.Columns[0].Raw = &pdm.Element{Path: "/Model/Table/Column", Line: 17}
	result = GenerateTable(table)
	var found bool
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == DiagnosticUnknownColumnType {
			found = diagnostic.Path == "/Model/Table/Column" && diagnostic.Line == 17
		}
	}
	if !found {
		t.Fatalf("type diagnostic lost raw location: %+v", result.Diagnostics)
	}
}

func completeTable() pdm.Table {
	yes, no := true, false
	text := func(value string) pdm.OptionalText {
		return pdm.OptionalText{Value: value, Present: true}
	}
	return pdm.Table{
		Metadata: pdm.Metadata{
			ID: "table-id", ObjectID: "table-object", Code: "Sales`Table", Comment: text("客户表"),
		},
		Columns: []pdm.Column{
			{
				Metadata:  pdm.Metadata{ID: "tenant-id", Code: "tenant", Comment: text("")},
				DataType:  pdm.OptionalText{Value: "INT", Present: true},
				Mandatory: &yes,
			},
			{
				Metadata: pdm.Metadata{ID: "id", Code: "id"},
				DataType: pdm.OptionalText{Value: "INT", Present: true},
				Identity: &yes,
			},
			{
				Metadata:     pdm.Metadata{ID: "display-name", Code: "display`name", Comment: text(`名称 "x"`)},
				DataType:     pdm.OptionalText{Value: "VARCHAR(20)", Present: true},
				DefaultValue: text(`'O''Brien'`),
				Mandatory:    &no,
			},
			{
				Metadata:     pdm.Metadata{ID: "status", Name: "status"},
				DataType:     pdm.OptionalText{Value: "CHAR(1)", Present: true},
				DefaultValue: text(""),
				Mandatory:    &no,
			},
			{
				Metadata:     pdm.Metadata{ID: "created", Code: "created"},
				DataType:     pdm.OptionalText{Value: "DATETIME(6)", Present: true},
				DefaultValue: text("CURRENT_TIMESTAMP(6)"),
				Mandatory:    &yes,
			},
			{
				Metadata:     pdm.Metadata{ID: "note", Code: "note"},
				DataType:     pdm.OptionalText{Value: "TEXT", Present: true},
				DefaultValue: text("'hello'"),
				Mandatory:    &no,
			},
			{
				Metadata:     pdm.Metadata{ID: "payload", Code: "payload"},
				DataType:     pdm.OptionalText{Value: "JSON", Present: true},
				DefaultValue: text("JSON_ARRAY()"),
				Mandatory:    &no,
			},
			{
				Metadata:     pdm.Metadata{ID: "amount", Code: "amount"},
				DataType:     pdm.OptionalText{Value: "DECIMAL(20,2)", Present: true},
				DefaultValue: text("12.30"),
				Mandatory:    &no,
			},
		},
		Keys: []pdm.Key{
			{Metadata: pdm.Metadata{ID: "pk"}, ColumnIDs: []string{"id", "tenant-id"}},
			{Metadata: pdm.Metadata{ID: "ix-name"}, ColumnIDs: []string{"display-name", "tenant-id"}},
		},
		PrimaryKeyID: "pk",
	}
}

func sqlTestColumn(dataType string, options ...columnOption) pdm.Column {
	column := withType(dataType, options...)
	column.ID = "column-id"
	column.Code = "value"
	return column
}

func sqlDefault(value string) columnOption {
	return func(column *pdm.Column) {
		column.DefaultValue = pdm.OptionalText{Value: value, Present: true}
	}
}

func sqlSingleColumnTable(column pdm.Column) pdm.Table {
	return pdm.Table{
		Metadata: pdm.Metadata{ID: "table-id", Code: "t"},
		Columns:  []pdm.Column{column},
	}
}

func makeNullable(column *pdm.Column) {
	value := false
	column.Mandatory = &value
}

func makeIdentity(column *pdm.Column) {
	value := true
	column.Identity = &value
}

func boolPointer(value bool) *bool {
	return &value
}

func hasCode(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
