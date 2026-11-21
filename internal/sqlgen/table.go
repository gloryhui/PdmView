package sqlgen

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"pdmview/internal/pdm"
)

// Severity classifies a table-generation diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

const (
	DiagnosticMissingTableID               = "missing-table-id"
	DiagnosticEmptyTable                   = "empty-table"
	DiagnosticTableIdentifierFallback      = "table-identifier-fallback"
	DiagnosticInvalidTableIdentifier       = "invalid-table-identifier"
	DiagnosticMissingColumnID              = "missing-column-id"
	DiagnosticDuplicateColumnID            = "duplicate-column-id"
	DiagnosticColumnIdentifierFallback     = "column-identifier-fallback"
	DiagnosticInvalidColumnIdentifier      = "invalid-column-identifier"
	DiagnosticDuplicateColumnIdentifier    = "duplicate-column-identifier"
	DiagnosticUnknownColumnType            = "unknown-column-type"
	DiagnosticInvalidColumnType            = "invalid-column-type"
	DiagnosticUnsupportedColumnType        = "unsupported-column-type"
	DiagnosticConflictingColumnType        = "conflicting-column-type"
	DiagnosticMissingKeyID                 = "missing-key-id"
	DiagnosticDuplicateKeyID               = "duplicate-key-id"
	DiagnosticEmptyKey                     = "empty-key"
	DiagnosticMissingKeyColumn             = "missing-key-column"
	DiagnosticDuplicateKeyColumn           = "duplicate-key-column"
	DiagnosticMissingPrimaryKey            = "missing-primary-key"
	DiagnosticNullablePrimaryKey           = "nullable-primary-key"
	DiagnosticMultipleAutoIncrement        = "multiple-auto-increment"
	DiagnosticUnsupportedAutoIncrement     = "unsupported-auto-increment"
	DiagnosticNullableAutoIncrement        = "nullable-auto-increment"
	DiagnosticAutoIncrementDefault         = "auto-increment-default"
	DiagnosticAutoIncrementNotKey          = "auto-increment-not-key"
	DiagnosticInvalidDefault               = "invalid-default"
	DiagnosticAmbiguousDefault             = "ambiguous-default"
	DiagnosticUnsupportedDefaultExpression = "unsupported-default-expression"
	DiagnosticDefaultTypeMismatch          = "default-type-mismatch"
	DiagnosticDefaultOutOfRange            = "default-out-of-range"
	DiagnosticDefaultFractionalPrecision   = "default-fractional-precision"
)

// Diagnostic is a stable, machine-readable table generation issue.
type Diagnostic struct {
	Code     string
	Severity Severity
	ObjectID string
	Message  string
	Path     string
	Line     int
}

// Result is one generated CREATE TABLE statement and its diagnostics.
type Result struct {
	SQL         string
	Diagnostics []Diagnostic
}

// HasErrors reports whether any diagnostic blocks SQL generation.
func (result Result) HasErrors() bool {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

// GenerateTable creates one deterministic MySQL 8.0 CREATE TABLE statement.
// It emits columns, nullability, defaults, AUTO_INCREMENT, the primary key,
// comments, and ENGINE=InnoDB. Indexes and foreign keys remain for later stages.
func GenerateTable(table pdm.Table) Result {
	compiled, diagnostics := compileTable(table)
	defaults := make([]string, len(compiled.columns))
	for offset, column := range compiled.columns {
		clause, diagnostic := compiled.defaultClause(column)
		defaults[offset] = clause
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
		}
	}
	result := Result{Diagnostics: diagnostics}
	if result.HasErrors() {
		return result
	}
	result.SQL = renderCreateTable(compiled, defaults)
	return result
}

type compiledTable struct {
	source           pdm.Table
	identifier       string
	columns          []compiledColumn
	columnByID       map[string]int
	keys             []compiledKey
	keyByID          map[string]int
	primary          int
	primaryPositions []int
}

type compiledColumn struct {
	source           pdm.Column
	position         int
	identifier       string
	typ              MySQLType
	hasType          bool
	explicitNullable *bool
	nullable         bool
	primary          bool
}

type compiledKey struct {
	source    pdm.Key
	position  int
	positions []int
}

func compileTable(table pdm.Table) (compiledTable, []Diagnostic) {
	compiled := compiledTable{
		source:     table,
		columnByID: make(map[string]int),
		keyByID:    make(map[string]int),
		primary:    -1,
	}
	var diagnostics []Diagnostic
	if table.ID == "" {
		diagnostics = append(diagnostics, tableDiagnostic(
			DiagnosticMissingTableID, table, "Table ID is required"))
	}
	identifier, fallback, err := sqlIdentifier(table.Code, table.Name)
	if err != nil {
		diagnostics = append(diagnostics, rawDiagnostic(
			DiagnosticInvalidTableIdentifier, SeverityError, table.ObjectID,
			"Table has no valid SQL identifier: "+err.Error(), table.Raw))
	} else {
		compiled.identifier = identifier
		if fallback {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticTableIdentifierFallback, SeverityWarning, table.ObjectID,
				"Table Code is absent; Name is used as the SQL identifier.", table.Raw))
		}
	}
	if len(table.Columns) == 0 {
		diagnostics = append(diagnostics, tableDiagnostic(
			DiagnosticEmptyTable, table, "Table has no columns"))
	}

	columnIdentifiers := make(map[string]int)
	for offset, column := range table.Columns {
		current := compiledColumn{
			source:           column,
			position:         offset,
			explicitNullable: columnNullable(column.Mandatory),
			nullable:         true,
		}
		if current.explicitNullable != nil {
			current.nullable = *current.explicitNullable
		}
		if column.ID == "" {
			diagnostics = append(diagnostics, columnDiagnostic(
				DiagnosticMissingColumnID, column, "Column ID is required"))
		} else if existing, ok := compiled.columnByID[column.ID]; ok {
			diagnostics = append(diagnostics, columnDiagnostic(
				DiagnosticDuplicateColumnID, column,
				fmt.Sprintf("Column ID duplicates column position %d.", existing+1)))
		} else {
			compiled.columnByID[column.ID] = offset
		}
		name, fallback, err := sqlIdentifier(column.Code, column.Name)
		if err != nil {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticInvalidColumnIdentifier, SeverityError, objectID(column.Metadata),
				"Column has no valid SQL identifier: "+err.Error(), column.Raw))
		} else {
			current.identifier = name
			if fallback {
				diagnostics = append(diagnostics, rawDiagnostic(
					DiagnosticColumnIdentifierFallback, SeverityWarning, objectID(column.Metadata),
					"Column Code is absent; Name is used as the SQL identifier.", column.Raw))
			}
			lookupName := strings.ToLower(name)
			if existing, ok := columnIdentifiers[lookupName]; ok {
				diagnostics = append(diagnostics, columnDiagnostic(
					DiagnosticDuplicateColumnIdentifier, column,
					fmt.Sprintf("Column identifier duplicates column position %d.", existing+1)))
			} else {
				columnIdentifiers[lookupName] = offset
			}
		}
		if typ, err := MapType(column); err != nil {
			diagnostics = append(diagnostics, typeDiagnostic(column, err))
		} else {
			current.typ = typ
			current.hasType = true
		}
		compiled.columns = append(compiled.columns, current)
	}

	for offset, key := range table.Keys {
		current := compiledKey{source: key, position: offset}
		if key.ID == "" {
			diagnostics = append(diagnostics, keyDiagnostic(
				DiagnosticMissingKeyID, key, "Key ID is required"))
		} else if existing, ok := compiled.keyByID[key.ID]; ok {
			diagnostics = append(diagnostics, keyDiagnostic(
				DiagnosticDuplicateKeyID, key,
				fmt.Sprintf("Key ID duplicates key position %d.", existing+1)))
		} else {
			compiled.keyByID[key.ID] = offset
		}
		if len(key.ColumnIDs) == 0 {
			diagnostics = append(diagnostics, keyDiagnostic(
				DiagnosticEmptyKey, key, "Key has no columns"))
		}
		seen := make(map[int]struct{})
		for _, columnID := range key.ColumnIDs {
			columnPosition, ok := compiled.columnByID[columnID]
			if !ok {
				diagnostics = append(diagnostics, keyDiagnostic(
					DiagnosticMissingKeyColumn, key,
					fmt.Sprintf("Key references missing column ID %q.", columnID)))
				continue
			}
			if _, duplicate := seen[columnPosition]; duplicate {
				diagnostics = append(diagnostics, keyDiagnostic(
					DiagnosticDuplicateKeyColumn, key,
					fmt.Sprintf("Key repeats column ID %q.", columnID)))
				continue
			}
			seen[columnPosition] = struct{}{}
			current.positions = append(current.positions, columnPosition)
		}
		compiled.keys = append(compiled.keys, current)
	}

	if table.PrimaryKeyID != "" {
		keyPosition, ok := compiled.keyByID[table.PrimaryKeyID]
		if !ok {
			diagnostics = append(diagnostics, tableDiagnostic(
				DiagnosticMissingPrimaryKey, table,
				fmt.Sprintf("Primary key ID %q is not present among table keys.", table.PrimaryKeyID)))
		} else {
			compiled.primary = keyPosition
			compiled.primaryPositions = append([]int(nil), compiled.keys[keyPosition].positions...)
			for _, columnPosition := range compiled.primaryPositions {
				column := &compiled.columns[columnPosition]
				column.primary = true
				if column.explicitNullable != nil && *column.explicitNullable {
					diagnostics = append(diagnostics, rawDiagnostic(
						DiagnosticNullablePrimaryKey, SeverityError, objectID(column.source.Metadata),
						"Primary-key column is explicitly nullable.", column.source.Raw))
				}
				column.nullable = false
			}
		}
	}

	compiled.validateAutoIncrement(&diagnostics)
	return compiled, diagnostics
}

func (compiled *compiledTable) validateAutoIncrement(diagnostics *[]Diagnostic) {
	var autoColumns []int
	for offset, column := range compiled.columns {
		if column.source.Identity != nil && *column.source.Identity {
			autoColumns = append(autoColumns, offset)
		}
	}
	if len(autoColumns) > 1 {
		*diagnostics = append(*diagnostics, tableDiagnostic(
			DiagnosticMultipleAutoIncrement, compiled.source,
			"Table has more than one AUTO_INCREMENT column."))
	}
	for _, offset := range autoColumns {
		column := &compiled.columns[offset]
		if !column.hasType || !supportsAutoIncrement(column.typ.Base) {
			*diagnostics = append(*diagnostics, columnDiagnostic(
				DiagnosticUnsupportedAutoIncrement, column.source,
				"AUTO_INCREMENT requires an integer column type."))
			continue
		}
		if column.nullable {
			*diagnostics = append(*diagnostics, columnDiagnostic(
				DiagnosticNullableAutoIncrement, column.source,
				"AUTO_INCREMENT column must be NOT NULL."))
		}
		if column.source.DefaultValue.Present {
			*diagnostics = append(*diagnostics, columnDiagnostic(
				DiagnosticAutoIncrementDefault, column.source,
				"AUTO_INCREMENT column must not have a DEFAULT."))
		}
		if !compiled.columnLeadsKey(column.source.ID) {
			*diagnostics = append(*diagnostics, columnDiagnostic(
				DiagnosticAutoIncrementNotKey, column.source,
				"AUTO_INCREMENT column must be the first column of a key or index."))
		}
	}
}

func (compiled *compiledTable) columnLeadsKey(columnID string) bool {
	if columnID == "" {
		return false
	}
	for _, key := range compiled.source.Keys {
		if len(key.ColumnIDs) > 0 && key.ColumnIDs[0] == columnID {
			return true
		}
	}
	for _, index := range compiled.source.Indexes {
		if len(index.Columns) > 0 && index.Columns[0].ColumnID == columnID {
			return true
		}
	}
	return false
}

func renderCreateTable(table compiledTable, defaults []string) string {
	var sql strings.Builder
	sql.WriteString("CREATE TABLE ")
	sql.WriteString(quoteIdentifier(table.identifier))
	sql.WriteString(" (\n")
	for offset, column := range table.columns {
		sql.WriteString("  ")
		sql.WriteString(quoteIdentifier(column.identifier))
		sql.WriteByte(' ')
		sql.WriteString(column.typ.SQL())
		if column.nullable {
			sql.WriteString(" NULL")
		} else {
			sql.WriteString(" NOT NULL")
		}
		if defaults[offset] != "" {
			sql.WriteByte(' ')
			sql.WriteString(defaults[offset])
		}
		if column.source.Identity != nil && *column.source.Identity {
			sql.WriteString(" AUTO_INCREMENT")
		}
		if column.source.Comment.Present {
			sql.WriteString(" COMMENT ")
			sql.WriteString(quoteSQLString(column.source.Comment.Value))
		}
		if offset != len(table.columns)-1 || table.primary >= 0 {
			sql.WriteByte(',')
		}
		sql.WriteByte('\n')
	}
	if table.primary >= 0 {
		sql.WriteString("  PRIMARY KEY (")
		for offset, columnPosition := range table.primaryPositions {
			if offset > 0 {
				sql.WriteString(", ")
			}
			sql.WriteString(quoteIdentifier(table.columns[columnPosition].identifier))
		}
		sql.WriteByte(')')
	}
	sql.WriteString("\n) ENGINE=InnoDB")
	if table.source.Comment.Present {
		sql.WriteString(" COMMENT=")
		sql.WriteString(quoteSQLString(table.source.Comment.Value))
	}
	sql.WriteByte(';')
	return sql.String()
}

func sqlIdentifier(code, name string) (string, bool, error) {
	fallback := strings.TrimSpace(code) == ""
	value := code
	if fallback {
		value = name
	}
	if err := validateIdentifier(value); err != nil {
		return "", fallback, err
	}
	return value, fallback, nil
}

func validateIdentifier(value string) error {
	if value == "" {
		return errors.New("identifier is empty")
	}
	if !utf8.ValidString(value) {
		return errors.New("identifier is not valid UTF-8")
	}
	runes := []rune(value)
	if len(runes) > 64 {
		return errors.New("identifier exceeds 64 characters")
	}
	for _, value := range runes {
		if value == 0 {
			return errors.New("identifier contains NUL")
		}
		if value > 0xffff {
			return errors.New("identifier contains a non-BMP character")
		}
	}
	if runes[len(runes)-1] == ' ' {
		return errors.New("identifier ends with a space")
	}
	return nil
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteSQLString(value string) string {
	var quoted strings.Builder
	quoted.WriteByte('\'')
	for _, current := range value {
		switch current {
		case 0:
			quoted.WriteString(`\0`)
		case '\b':
			quoted.WriteString(`\b`)
		case '\n':
			quoted.WriteString(`\n`)
		case '\r':
			quoted.WriteString(`\r`)
		case '\t':
			quoted.WriteString(`\t`)
		case 0x1a:
			quoted.WriteString(`\Z`)
		case '\\':
			quoted.WriteString(`\\`)
		case '\'':
			quoted.WriteString(`''`)
		default:
			quoted.WriteRune(current)
		}
	}
	quoted.WriteByte('\'')
	return quoted.String()
}

func columnNullable(mandatory *bool) *bool {
	if mandatory == nil {
		return nil
	}
	nullable := !*mandatory
	return &nullable
}

func supportsAutoIncrement(base BaseType) bool {
	switch base {
	case TypeTinyInt, TypeSmallInt, TypeMediumInt, TypeInt, TypeBigInt:
		return true
	default:
		return false
	}
}

func typeDiagnostic(column pdm.Column, err error) Diagnostic {
	code := DiagnosticInvalidColumnType
	switch {
	case errors.Is(err, ErrUnknownType):
		code = DiagnosticUnknownColumnType
	case errors.Is(err, ErrUnsupportedType):
		code = DiagnosticUnsupportedColumnType
	case errors.Is(err, ErrConflictingType):
		code = DiagnosticConflictingColumnType
	}
	return rawDiagnostic(code, SeverityError, objectID(column.Metadata), err.Error(), column.Raw)
}

func tableDiagnostic(code string, table pdm.Table, message string) Diagnostic {
	return rawDiagnostic(code, SeverityError, objectID(table.Metadata), message, table.Raw)
}

func columnDiagnostic(code string, column pdm.Column, message string) Diagnostic {
	return rawDiagnostic(code, SeverityError, objectID(column.Metadata), message, column.Raw)
}

func keyDiagnostic(code string, key pdm.Key, message string) Diagnostic {
	return rawDiagnostic(code, SeverityError, objectID(key.Metadata), message, key.Raw)
}

func rawDiagnostic(
	code string,
	severity Severity,
	objectID, message string,
	raw *pdm.Element,
) Diagnostic {
	diagnostic := Diagnostic{
		Code: code, Severity: severity, ObjectID: objectID, Message: message,
	}
	if raw != nil {
		diagnostic.Path, diagnostic.Line = raw.Path, raw.Line
	}
	return diagnostic
}

func objectID(metadata pdm.Metadata) string {
	if metadata.ObjectID != "" {
		return metadata.ObjectID
	}
	if metadata.ID != "" {
		return metadata.ID
	}
	return firstNonEmpty(metadata.Code, metadata.Name)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func integerBounds(base BaseType, unsigned bool) (*big.Int, *big.Int) {
	bits := map[BaseType]uint{
		TypeTinyInt: 8, TypeSmallInt: 16, TypeMediumInt: 24, TypeInt: 32, TypeBigInt: 64,
	}[base]
	if bits == 0 {
		return nil, nil
	}
	one := big.NewInt(1)
	if unsigned {
		max := new(big.Int).Sub(new(big.Int).Lsh(one, bits), one)
		return big.NewInt(0), max
	}
	half := bits - 1
	max := new(big.Int).Sub(new(big.Int).Lsh(one, half), one)
	min := new(big.Int).Neg(new(big.Int).Lsh(one, half))
	return min, max
}
