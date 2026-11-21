package sqlgen

import (
	"encoding/xml"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"pdmview/internal/pdm"
)

func TestMapTypeObservedFamiliesAndDimensions(t *testing.T) {
	tests := []struct {
		name   string
		column pdm.Column
		want   MySQLType
		sql    string
	}{
		{
			name:   "char",
			column: withType("CHAR", length(10)),
			want:   MySQLType{Base: TypeChar, Length: 10},
			sql:    "CHAR(10)",
		},
		{
			name:   "varchar embedded length",
			column: withType("VARCHAR(255)", length(255)),
			want:   MySQLType{Base: TypeVarChar, Length: 255},
			sql:    "VARCHAR(255)",
		},
		{
			name:   "date",
			column: withType("DATE"),
			want:   MySQLType{Base: TypeDate},
			sql:    "DATE",
		},
		{
			name:   "datetime",
			column: withType("DATETIME"),
			want:   MySQLType{Base: TypeDateTime},
			sql:    "DATETIME",
		},
		{
			name:   "text",
			column: withType("TEXT"),
			want:   MySQLType{Base: TypeText},
			sql:    "TEXT",
		},
		{
			name:   "legacy int width ten",
			column: withType("INT(10)"),
			want:   MySQLType{Base: TypeInt, LegacyDisplayWidth: 10},
			sql:    "INT",
		},
		{
			name:   "legacy int width twenty",
			column: withType("int(20)"),
			want:   MySQLType{Base: TypeInt, LegacyDisplayWidth: 20},
			sql:    "INT",
		},
		{
			name:   "legacy int width from Length",
			column: withType("INT", length(10)),
			want:   MySQLType{Base: TypeInt, LegacyDisplayWidth: 10},
			sql:    "INT",
		},
		{
			name:   "legacy int twenty from Length",
			column: withType("INT", length(20)),
			want:   MySQLType{Base: TypeInt, LegacyDisplayWidth: 20},
			sql:    "INT",
		},
		{
			name:   "PowerDesigner decimal",
			column: withType("DECIMAL(20,2)", length(20), precision(2)),
			want:   MySQLType{Base: TypeDecimal, Precision: 20, Scale: 2},
			sql:    "DECIMAL(20,2)",
		},
		{
			name:   "embedded decimal without attributes",
			column: withType("DECIMAL(18,4)"),
			want:   MySQLType{Base: TypeDecimal, Precision: 18, Scale: 4},
			sql:    "DECIMAL(18,4)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MapType(tc.column)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) || got.SQL() != tc.sql {
				t.Fatalf("MapType = %+v SQL %q, want %+v SQL %q", got, got.SQL(), tc.want, tc.sql)
			}
		})
	}
}

func TestMapTypeAliasesAndCommonFamilies(t *testing.T) {
	tests := []struct {
		dataType string
		length   *int
		wantBase BaseType
		wantSQL  string
	}{
		{dataType: "INTEGER(11)", wantBase: TypeInt, wantSQL: "INT"},
		{dataType: "INT4", wantBase: TypeInt, wantSQL: "INT"},
		{dataType: "MIDDLEINT", wantBase: TypeMediumInt, wantSQL: "MEDIUMINT"},
		{dataType: "INT8 UNSIGNED", wantBase: TypeBigInt, wantSQL: "BIGINT UNSIGNED"},
		{dataType: "BOOL", wantBase: TypeTinyInt, wantSQL: "TINYINT"},
		{dataType: "DEC", length: intPtr(12), wantBase: TypeDecimal, wantSQL: "DECIMAL(12,0)"},
		{dataType: "NUMERIC(10,2)", wantBase: TypeDecimal, wantSQL: "DECIMAL(10,2)"},
		{dataType: "REAL", wantBase: TypeDouble, wantSQL: "DOUBLE"},
		{dataType: "DOUBLE PRECISION", wantBase: TypeDouble, wantSQL: "DOUBLE"},
		{dataType: "CHARACTER VARYING(32)", wantBase: TypeVarChar, wantSQL: "VARCHAR(32)"},
		{dataType: "BINARY(16)", wantBase: TypeBinary, wantSQL: "BINARY(16)"},
		{dataType: "VARBINARY(64)", wantBase: TypeVarBinary, wantSQL: "VARBINARY(64)"},
		{dataType: "TINYTEXT", wantBase: TypeTinyText, wantSQL: "TINYTEXT"},
		{dataType: "MEDIUMTEXT", wantBase: TypeMediumText, wantSQL: "MEDIUMTEXT"},
		{dataType: "LONGBLOB", wantBase: TypeLongBlob, wantSQL: "LONGBLOB"},
		{dataType: "TIME", wantBase: TypeTime, wantSQL: "TIME"},
		{dataType: "TIMESTAMP(6)", wantBase: TypeTimestamp, wantSQL: "TIMESTAMP(6)"},
		{dataType: "YEAR", wantBase: TypeYear, wantSQL: "YEAR"},
		{dataType: "JSON", wantBase: TypeJSON, wantSQL: "JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.dataType, func(t *testing.T) {
			column := withType(tc.dataType)
			if tc.length != nil {
				column.Length = presentInt(*tc.length)
			}
			got, err := MapType(column)
			if err != nil {
				t.Fatal(err)
			}
			if got.Base != tc.wantBase || got.SQL() != tc.wantSQL {
				t.Fatalf("MapType(%q) = %+v SQL %q, want %s SQL %q",
					tc.dataType, got, got.SQL(), tc.wantBase, tc.wantSQL)
			}
			if tc.dataType == "BOOL" && got.LegacyDisplayWidth != 1 {
				t.Fatalf("BOOL legacy display width = %d, want 1", got.LegacyDisplayWidth)
			}
		})
	}
}

func TestMapTypeDecimalReconciliation(t *testing.T) {
	tests := []struct {
		name   string
		column pdm.Column
		sql    string
		err    error
	}{
		{name: "MySQL default", column: withType("DECIMAL"), sql: "DECIMAL(10,0)"},
		{name: "embedded precision only", column: withType("DECIMAL(20)"), sql: "DECIMAL(20,0)"},
		{name: "Length and explicit Scale", column: withType("DECIMAL", length(20), scale(2)), sql: "DECIMAL(20,2)"},
		{name: "standalone Precision and Scale", column: withType("DECIMAL", precision(20), scale(2)), sql: "DECIMAL(20,2)"},
		{name: "PowerDesigner attributes only", column: withType("DECIMAL", length(20), precision(2)), sql: "DECIMAL(20,2)"},
		{name: "explicit Scale with duplicate PD precision", column: withType("DECIMAL", length(20), precision(2), scale(2)), sql: "DECIMAL(20,2)"},
		{name: "matching embedded and attributes", column: withType("DECIMAL(20,2)", length(20), precision(2)), sql: "DECIMAL(20,2)"},
		{name: "embedded and partial Length", column: withType("DECIMAL(20,2)", length(20)), sql: "DECIMAL(20,2)"},
		{name: "embedded and partial PD Precision", column: withType("DECIMAL(20,2)", precision(2)), sql: "DECIMAL(20,2)"},
		{name: "embedded and partial Scale", column: withType("DECIMAL(20,2)", scale(2)), sql: "DECIMAL(20,2)"},
		{name: "PD precision without length uses default width", column: withType("DECIMAL", precision(2)), sql: "DECIMAL(10,2)"},
		{name: "embedded conflict", column: withType("DECIMAL(20,2)", length(21)), err: ErrConflictingType},
		{name: "precision conflict", column: withType("DECIMAL", length(20), precision(2), scale(3)), err: ErrConflictingType},
		{name: "precision too large", column: withType("DECIMAL(66,2)"), err: ErrInvalidType},
		{name: "scale too large", column: withType("DECIMAL(10,31)"), err: ErrInvalidType},
		{name: "scale exceeds precision", column: withType("DECIMAL(5,6)"), err: ErrInvalidType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MapType(tc.column)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("MapType error = %v, want %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.SQL() != tc.sql {
				t.Fatalf("SQL = %q, want %q", got.SQL(), tc.sql)
			}
		})
	}
}

func TestMapTypeFloatAndBitCanonicalRules(t *testing.T) {
	tests := []struct {
		dataType string
		base     BaseType
		sql      string
		err      error
	}{
		{dataType: "FLOAT", base: TypeFloat, sql: "FLOAT"},
		{dataType: "FLOAT(0)", base: TypeFloat, sql: "FLOAT"},
		{dataType: "FLOAT(24)", base: TypeFloat, sql: "FLOAT"},
		{dataType: "FLOAT(25)", base: TypeDouble, sql: "DOUBLE"},
		{dataType: "FLOAT(53)", base: TypeDouble, sql: "DOUBLE"},
		{dataType: "FLOAT(54)", err: ErrInvalidType},
		{dataType: "DOUBLE", base: TypeDouble, sql: "DOUBLE"},
		{dataType: "DOUBLE(10)", err: ErrUnsupportedType},
		{dataType: "BIT", base: TypeBit, sql: "BIT(1)"},
		{dataType: "BIT(64)", base: TypeBit, sql: "BIT(64)"},
		{dataType: "BIT(0)", err: ErrInvalidType},
		{dataType: "BIT(65)", err: ErrInvalidType},
		{dataType: "BIT UNSIGNED", err: ErrUnsupportedType},
		{dataType: "BIT SIGNED", err: ErrUnsupportedType},
	}
	for _, tc := range tests {
		t.Run(tc.dataType, func(t *testing.T) {
			got, err := MapType(withType(tc.dataType))
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("MapType error = %v, want %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Base != tc.base || got.SQL() != tc.sql {
				t.Fatalf("MapType = %+v SQL %q, want %s SQL %q", got, got.SQL(), tc.base, tc.sql)
			}
		})
	}
}

func TestMapTypeUnsignedCharacterSetCollationAndConflicts(t *testing.T) {
	tests := []struct {
		name   string
		column pdm.Column
		want   MySQLType
		err    error
	}{
		{
			name:   "suffix unsigned across newline",
			column: withType("BIGINT\r\nUNSIGNED"),
			want:   MySQLType{Base: TypeBigInt, Unsigned: true},
		},
		{
			name:   "raw unsigned",
			column: withType("INT", raw("Column.Unsigned", "1")),
			want:   MySQLType{Base: TypeInt, Unsigned: true},
		},
		{
			name:   "suffix and raw unsigned agree",
			column: withType("INT UNSIGNED", raw("Unsigned", "1")),
			want:   MySQLType{Base: TypeInt, Unsigned: true},
		},
		{
			name:   "unsigned conflict",
			column: withType("INT UNSIGNED", raw("Unsigned", "0")),
			err:    ErrConflictingType,
		},
		{
			name:   "unsigned on varchar",
			column: withType("VARCHAR(20) UNSIGNED"),
			err:    ErrUnsupportedType,
		},
		{
			name:   "suffix charset and collation",
			column: withType("VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin"),
			want: MySQLType{
				Base: TypeVarChar, Length: 20,
				CharacterSet: "utf8mb4", Collation: "utf8mb4_bin",
			},
		},
		{
			name: "raw charset and collation",
			column: withType("TEXT",
				raw("CharacterSet", "utf8mb4"),
				raw("Collation", "utf8mb4_0900_ai_ci")),
			want: MySQLType{
				Base: TypeText, CharacterSet: "utf8mb4", Collation: "utf8mb4_0900_ai_ci",
			},
		},
		{
			name: "duplicate raw aliases agree",
			column: withType("CHAR(4)",
				raw("CharacterSet", "utf8mb4"),
				raw("Charset", "utf8mb4")),
			want: MySQLType{Base: TypeChar, Length: 4, CharacterSet: "utf8mb4"},
		},
		{
			name: "raw charset conflicts with suffix",
			column: withType("VARCHAR(20) CHARACTER SET utf8mb4",
				raw("CharacterSet", "latin1")),
			err: ErrConflictingType,
		},
		{
			name: "duplicate raw aliases conflict",
			column: withType("VARCHAR(20)",
				raw("CharacterSet", "utf8mb4"),
				raw("Charset", "latin1")),
			err: ErrConflictingType,
		},
		{
			name: "duplicate raw property conflicts",
			column: withType("VARCHAR(20)",
				raw("CharacterSet", "utf8mb4"),
				raw("CharacterSet", "latin1")),
			err: ErrConflictingType,
		},
		{
			name:   "charset on integer",
			column: withType("INT CHARACTER SET utf8mb4"),
			err:    ErrUnsupportedType,
		},
		{
			name:   "charset on binary",
			column: withType("BINARY(4) CHARACTER SET utf8mb4"),
			err:    ErrUnsupportedType,
		},
		{
			name:   "invalid charset token",
			column: withType("VARCHAR(20) CHARACTER SET utf8mb4;DROP"),
			err:    ErrInvalidType,
		},
		{
			name:   "invalid raw collation",
			column: withType("VARCHAR(20)", raw("Collation", "utf8mb4_bin`)")),
			err:    ErrInvalidType,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MapType(tc.column)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("MapType error = %v, want %v", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("MapType = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMapTypeErrorsBoundsAndNoRawPassthrough(t *testing.T) {
	tests := []struct {
		name   string
		column pdm.Column
		err    error
	}{
		{name: "absent type", column: pdm.Column{}, err: ErrInvalidType},
		{name: "blank type", column: withType("  "), err: ErrInvalidType},
		{name: "unterminated", column: withType("INT("), err: ErrInvalidType},
		{name: "two int args", column: withType("INT(10,2)"), err: ErrInvalidType},
		{name: "empty varchar", column: withType("VARCHAR()"), err: ErrInvalidType},
		{name: "negative varchar", column: withType("VARCHAR(-1)"), err: ErrInvalidType},
		{name: "char too long", column: withType("CHAR(256)"), err: ErrInvalidType},
		{name: "varchar too long", column: withType("VARCHAR(65536)"), err: ErrInvalidType},
		{name: "varchar requires length", column: withType("VARCHAR"), err: ErrInvalidType},
		{name: "datetime fsp too large", column: withType("DATETIME(7)"), err: ErrInvalidType},
		{name: "text dimensions", column: withType("TEXT(10)"), err: ErrUnsupportedType},
		{name: "binary character set changes physical type", column: withType("TEXT CHARACTER SET binary"), err: ErrUnsupportedType},
		{name: "year two", column: withType("YEAR(2)"), err: ErrUnsupportedType},
		{name: "zero fill", column: withType("INT ZEROFILL"), err: ErrUnsupportedType},
		{name: "repeated unsigned", column: withType("INT UNSIGNED UNSIGNED"), err: ErrConflictingType},
		{name: "enum", column: withType("ENUM('a','b')"), err: ErrUnsupportedType},
		{name: "set", column: withType("SET('a','b')"), err: ErrUnsupportedType},
		{name: "serial", column: withType("SERIAL"), err: ErrUnsupportedType},
		{name: "geometry", column: withType("POINT"), err: ErrUnsupportedType},
		{name: "foreign number", column: withType("NUMBER(10,2)"), err: ErrUnknownType},
		{name: "foreign nvarchar2", column: withType("NVARCHAR2(10)"), err: ErrUnknownType},
		{name: "foreign timestamptz", column: withType("TIMESTAMPTZ"), err: ErrUnknownType},
		{name: "foreign jsonb", column: withType("JSONB"), err: ErrUnknownType},
		{name: "injection semicolon", column: withType("INT; DROP TABLE x"), err: ErrInvalidType},
		{name: "injection modifier", column: withType("INT UNSIGNED OR 1=1"), err: ErrInvalidType},
		{name: "injection collation", column: withType("VARCHAR(10) COLLATE utf8mb4_bin; DROP"), err: ErrInvalidType},
		{name: "empty Length", column: withType("CHAR", presentText("Length", "")), err: ErrInvalidType},
		{name: "negative Length", column: withType("CHAR", presentText("Length", "-1")), err: ErrInvalidType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MapType(tc.column)
			if !errors.Is(err, tc.err) {
				t.Fatalf("MapType(%q) error = %v, want %v", tc.column.DataType.Value, err, tc.err)
			}
			if got != (MySQLType{}) {
				t.Fatalf("failed MapType returned %+v", got)
			}
		})
	}
}

func TestMySQLTypeSQLEmitsOnlyValidatedCanonicalFields(t *testing.T) {
	got, err := MapType(withType("INT(10) UNSIGNED"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SQL() != "INT UNSIGNED" || got.LegacyDisplayWidth != 10 {
		t.Fatalf("canonical int = %+v SQL %q", got, got.SQL())
	}
	typeValue := MySQLType{
		Base:         TypeVarChar,
		Length:       32,
		CharacterSet: "utf8mb4",
		Collation:    "utf8mb4_bin",
	}
	if typeValue.SQL() != "VARCHAR(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin" {
		t.Fatalf("canonical SQL = %q", typeValue.SQL())
	}
	if typeValue.Family() != FamilyCharacter {
		t.Fatalf("family = %v", typeValue.Family())
	}
}

type columnOption func(*pdm.Column)

func withType(dataType string, options ...columnOption) pdm.Column {
	column := pdm.Column{DataType: pdm.OptionalText{Value: dataType, Present: true}}
	for _, option := range options {
		option(&column)
	}
	return column
}

func length(value int) columnOption {
	return func(column *pdm.Column) { column.Length = presentInt(value) }
}

func precision(value int) columnOption {
	return func(column *pdm.Column) { column.Precision = presentInt(value) }
}

func scale(value int) columnOption {
	return func(column *pdm.Column) { column.Scale = presentInt(value) }
}

func raw(properties ...string) columnOption {
	return func(column *pdm.Column) {
		if column.Raw == nil {
			column.Raw = &pdm.Element{Name: xml.Name{Local: "Column"}}
		}
		for offset := 0; offset+1 < len(properties); offset += 2 {
			column.Raw.Children = append(column.Raw.Children, &pdm.Element{
				Name: xml.Name{Space: pdm.NamespaceAttribute, Local: properties[offset]},
				Text: properties[offset+1],
			})
		}
	}
}

func presentText(local, value string) columnOption {
	return func(column *pdm.Column) {
		switch local {
		case "Length":
			column.Length = pdm.OptionalText{Value: value, Present: true}
		case "Precision":
			column.Precision = pdm.OptionalText{Value: value, Present: true}
		case "Scale":
			column.Scale = pdm.OptionalText{Value: value, Present: true}
		}
	}
}

func presentInt(value int) pdm.OptionalText {
	return pdm.OptionalText{Value: strconv.Itoa(value), Present: true}
}

func intPtr(value int) *int {
	return &value
}
