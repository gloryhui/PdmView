package sqlgen

import (
	"fmt"
	"strings"

	"pdmview/internal/pdm"
)

const (
	DiagnosticForeignKeyTableMismatch     = "foreign-key-table-mismatch"
	DiagnosticForeignKeyOwnerMissing      = "foreign-key-owner-missing"
	DiagnosticForeignKeyInvalidName       = "foreign-key-invalid-name"
	DiagnosticForeignKeyEmpty             = "foreign-key-empty"
	DiagnosticForeignKeyMissingColumn     = "foreign-key-missing-column"
	DiagnosticForeignKeyDuplicateColumn   = "foreign-key-duplicate-column"
	DiagnosticForeignKeySelfColumn        = "foreign-key-self-column"
	DiagnosticForeignKeyTypeMismatch      = "foreign-key-type-mismatch"
	DiagnosticForeignKeyMissingParentKey  = "foreign-key-missing-parent-key"
	DiagnosticForeignKeyUnusableParentKey = "foreign-key-unusable-parent-key"
	DiagnosticForeignKeyAction            = "foreign-key-action"
	DiagnosticForeignKeySetNull           = "foreign-key-set-null"
)

var constraintNameProperties = []string{"ConstraintName", "Reference.ConstraintName"}

// compiledForeignKey is validated independently of WHERE it is emitted, so
// later stages can inline it or add it with ALTER TABLE deterministically.
type compiledForeignKey struct {
	identifier    string
	childTable    string
	childColumns  []string
	parentTable   string
	parentColumns []string
	updateAction  string
	deleteAction  string
}

func compileForeignKey(
	child, parent compiledTable,
	reference pdm.Reference,
) (compiledForeignKey, []Diagnostic) {
	var diagnostics []Diagnostic
	result := compiledForeignKey{
		childTable:  child.identifier,
		parentTable: parent.identifier,
	}
	if reference.ChildTableID == "" || reference.ChildTableID != child.source.ID ||
		reference.ParentTableID == "" || reference.ParentTableID != parent.source.ID {
		diagnostics = append(diagnostics, referenceDiagnostic(
			DiagnosticForeignKeyTableMismatch, reference,
			"Foreign key endpoints do not match the compiled child and parent tables."))
	}
	if reference.OwnerID == "" {
		diagnostics = append(diagnostics, referenceDiagnostic(
			DiagnosticForeignKeyOwnerMissing, reference,
			"Foreign key owner ID is missing."))
	}
	name, problem := foreignKeyName(reference)
	if problem != "" {
		diagnostics = append(diagnostics, referenceDiagnostic(
			DiagnosticForeignKeyInvalidName, reference, problem))
	} else {
		result.identifier = name
	}
	if len(reference.Joins) == 0 {
		diagnostics = append(diagnostics, referenceDiagnostic(
			DiagnosticForeignKeyEmpty, reference, "Foreign key has no column joins."))
	}
	seenChild := make(map[int]struct{})
	seenParent := make(map[int]struct{})
	for _, join := range reference.Joins {
		childPosition, childOK := child.columnByID[join.ChildColumnID]
		parentPosition, parentOK := parent.columnByID[join.ParentColumnID]
		if !childOK || !parentOK {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticForeignKeyMissingColumn, SeverityError, objectID(join.Metadata),
				"Foreign-key join references a missing child or parent column.", join.Raw))
			continue
		}
		if _, duplicate := seenChild[childPosition]; duplicate {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticForeignKeyDuplicateColumn, SeverityError, objectID(join.Metadata),
				"Foreign-key join repeats a child column.", join.Raw))
			continue
		}
		if _, duplicate := seenParent[parentPosition]; duplicate {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticForeignKeyDuplicateColumn, SeverityError, objectID(join.Metadata),
				"Foreign-key join repeats a parent column.", join.Raw))
			continue
		}
		seenChild[childPosition] = struct{}{}
		seenParent[parentPosition] = struct{}{}
		if child.source.ID == parent.source.ID && childPosition == parentPosition {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticForeignKeySelfColumn, SeverityError, objectID(join.Metadata),
				"A foreign key cannot reference the same column in its own table.", join.Raw))
		}
		if compatible, reason := foreignKeyTypesCompatible(
			child.columns[childPosition].typ, parent.columns[parentPosition].typ); !compatible {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticForeignKeyTypeMismatch, SeverityError, objectID(join.Metadata),
				reason, join.Raw))
		}
		result.childColumns = append(result.childColumns, child.columns[childPosition].identifier)
		result.parentColumns = append(result.parentColumns, parent.columns[parentPosition].identifier)
	}
	if len(result.childColumns) == len(reference.Joins) && len(result.parentColumns) == len(reference.Joins) {
		positions := make([]int, 0, len(result.parentColumns))
		for _, join := range reference.Joins {
			positions = append(positions, parent.columnByID[join.ParentColumnID])
		}
		if problem := parentKeyProblem(parent, positions, reference.ParentKeyID); problem != "" {
			code := DiagnosticForeignKeyUnusableParentKey
			if reference.ParentKeyID != "" {
				if _, ok := parent.keyByID[reference.ParentKeyID]; !ok {
					code = DiagnosticForeignKeyMissingParentKey
				}
			}
			diagnostics = append(diagnostics, referenceDiagnostic(code, reference, problem))
		}
	}
	result.updateAction, problem = foreignKeyAction(
		reference.UpdateConstraint, child, seenChild)
	if problem != "" {
		code := DiagnosticForeignKeyAction
		if strings.Contains(problem, "SET NULL") {
			code = DiagnosticForeignKeySetNull
		}
		diagnostics = append(diagnostics, referenceDiagnostic(code, reference, problem))
	}
	result.deleteAction, problem = foreignKeyAction(
		reference.DeleteConstraint, child, seenChild)
	if problem != "" {
		code := DiagnosticForeignKeyAction
		if strings.Contains(problem, "SET NULL") {
			code = DiagnosticForeignKeySetNull
		}
		diagnostics = append(diagnostics, referenceDiagnostic(code, reference, problem))
	}
	return result, diagnostics
}

// SQL returns the deterministic FOREIGN KEY constraint clause.
func (foreignKey compiledForeignKey) SQL() string {
	var sql strings.Builder
	sql.WriteString("CONSTRAINT ")
	sql.WriteString(quoteIdentifier(foreignKey.identifier))
	sql.WriteString(" FOREIGN KEY (")
	for offset, column := range foreignKey.childColumns {
		if offset > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(quoteIdentifier(column))
	}
	sql.WriteString(") REFERENCES ")
	sql.WriteString(quoteIdentifier(foreignKey.parentTable))
	sql.WriteString(" (")
	for offset, column := range foreignKey.parentColumns {
		if offset > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(quoteIdentifier(column))
	}
	sql.WriteByte(')')
	if foreignKey.deleteAction != "" {
		sql.WriteString(" ON DELETE ")
		sql.WriteString(foreignKey.deleteAction)
	}
	if foreignKey.updateAction != "" {
		sql.WriteString(" ON UPDATE ")
		sql.WriteString(foreignKey.updateAction)
	}
	return sql.String()
}

// InlineSQL is the clause for use inside CREATE TABLE.
func (foreignKey compiledForeignKey) InlineSQL() string {
	return foreignKey.SQL()
}

// AlterSQL is the statement form for the later ALTER TABLE dependency stage.
func (foreignKey compiledForeignKey) AlterSQL() string {
	return "ALTER TABLE " + quoteIdentifier(foreignKey.childTable) +
		" ADD " + foreignKey.SQL() + ";"
}

func foreignKeyName(reference pdm.Reference) (string, string) {
	property, present, err := rawProperty(reference.Raw, constraintNameProperties...)
	if err != nil {
		return "", err.Error()
	}
	value := property
	if !present {
		value = firstNonEmpty(reference.Code, reference.Name)
	}
	if value == "" {
		return "", "Foreign key has no constraint name."
	}
	if err := validateIdentifier(value); err != nil {
		return "", "Foreign key constraint name is invalid: " + err.Error()
	}
	return value, ""
}

func foreignKeyTypesCompatible(child, parent MySQLType) (bool, string) {
	if child.Base != parent.Base {
		if !(isCharacterBase(child.Base) && isCharacterBase(parent.Base)) &&
			!(isBinaryBase(child.Base) && isBinaryBase(parent.Base)) {
			return false, "Child and parent foreign-key base types differ."
		}
	}
	switch {
	case isIntegerBase(child.Base):
		if child.Unsigned != parent.Unsigned {
			return false, "Child and parent integer signedness differs."
		}
		return true, ""
	case child.Base == TypeDecimal:
		if child.Unsigned != parent.Unsigned ||
			child.Precision != parent.Precision || child.Scale != parent.Scale {
			return false, "Child and parent DECIMAL definitions differ."
		}
		return true, ""
	case child.Base == TypeBit:
		if child.Length != parent.Length {
			return false, "Child and parent BIT widths differ."
		}
		return true, ""
	case isCharacterBase(child.Base):
		if !isCharacterBase(parent.Base) {
			return false, "Child and parent character families differ."
		}
		if child.CharacterSet == "" || parent.CharacterSet == "" {
			if child.CharacterSet != parent.CharacterSet {
				return false, "Character set inheritance prevents proving foreign-key compatibility."
			}
		} else if child.CharacterSet != parent.CharacterSet {
			return false, "Child and parent character sets differ."
		}
		if child.Collation == "" || parent.Collation == "" {
			if child.Collation != parent.Collation {
				return false, "Collation inheritance prevents proving foreign-key compatibility."
			}
		} else if child.Collation != parent.Collation {
			return false, "Child and parent collations differ."
		}
		return true, ""
	case isBinaryBase(child.Base):
		if !isBinaryBase(parent.Base) {
			return false, "Child and parent binary families differ."
		}
		return true, ""
	case child.Family() == FamilyText || child.Family() == FamilyBlob || child.Family() == FamilyJSON:
		return false, "TEXT, BLOB, and JSON columns cannot be foreign-key columns."
	default:
		if child.Base != parent.Base {
			return false, "Child and parent foreign-key types differ."
		}
		return true, ""
	}
}

func parentKeyProblem(parent compiledTable, positions []int, keyID string) string {
	if len(positions) == 0 {
		return "Foreign key has no parent columns."
	}
	if keyID != "" {
		keyOffset, ok := parent.keyByID[keyID]
		if !ok {
			return fmt.Sprintf("Parent key ID %q is missing.", keyID)
		}
		key := parent.keys[keyOffset]
		if !samePositions(key.positions, positions) {
			return "Parent key columns do not match the foreign-key join order."
		}
	}
	if parent.primary >= 0 && samePositions(parent.primaryPositions, positions) {
		return ""
	}
	for _, index := range parent.indexes {
		if !index.unique || len(index.columns) != len(positions) {
			continue
		}
		matches := true
		for offset, column := range index.columns {
			if column.position != positions[offset] || column.prefix != 0 {
				matches = false
				break
			}
		}
		if matches {
			return ""
		}
	}
	return "Parent columns are not covered by a non-prefix unique key."
}

func samePositions(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for offset := range left {
		if left[offset] != right[offset] {
			return false
		}
	}
	return true
}

func foreignKeyAction(
	value pdm.OptionalText,
	child compiledTable,
	childPositions map[int]struct{},
) (string, string) {
	if !value.Present {
		return "", ""
	}
	action := strings.ToUpper(strings.TrimSpace(value.Value))
	switch action {
	case "CASCADE", "RESTRICT", "NO ACTION":
		return action, ""
	case "SET NULL":
		for position := range childPositions {
			if !child.columns[position].nullable {
				return "", "SET NULL requires nullable child columns."
			}
		}
		return action, ""
	case "SET DEFAULT":
		return "", "SET DEFAULT is unsupported by MySQL InnoDB."
	default:
		return "", fmt.Sprintf("Foreign-key action %q is unsupported.", value.Value)
	}
}

func referenceDiagnostic(code string, reference pdm.Reference, message string) Diagnostic {
	return rawDiagnostic(code, SeverityError, objectID(reference.Metadata), message, reference.Raw)
}

func isIntegerBase(base BaseType) bool {
	switch base {
	case TypeTinyInt, TypeSmallInt, TypeMediumInt, TypeInt, TypeBigInt:
		return true
	default:
		return false
	}
}

func isCharacterBase(base BaseType) bool {
	return base == TypeChar || base == TypeVarChar
}

func isBinaryBase(base BaseType) bool {
	return base == TypeBinary || base == TypeVarBinary
}
