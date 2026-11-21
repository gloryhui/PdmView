package sqlgen

import (
	"fmt"
	"strings"

	"pdmview/internal/pdm"
)

const (
	DiagnosticDatabaseName           = "database-name"
	DiagnosticDatabaseNameFallback   = "database-name-fallback"
	DiagnosticDatabaseNameBoundary   = "database-name-boundary"
	DiagnosticDatabaseSystemName     = "database-system-name"
	DiagnosticDatabaseCharacterSet   = "database-character-set"
	DiagnosticDatabaseCollation      = "database-collation"
	DiagnosticDatabaseOptionMismatch = "database-option-mismatch"
)

// DatabaseOptions selects a target database and optional MySQL text settings.
// An empty Name uses the model identifier. Empty text settings omit those clauses.
type DatabaseOptions struct {
	Name         string
	CharacterSet string
	Collation    string
}

// GenerateDatabase creates the complete deterministic database script.
func GenerateDatabase(document *pdm.Document, options DatabaseOptions) Result {
	plan, diagnostics := compileDatabase(document)
	name, nameDiagnostics := databaseName(document, options.Name)
	diagnostics = append(diagnostics, nameDiagnostics...)
	characterSet, collation, optionDiagnostics := databaseCharacterOptions(
		options.CharacterSet, options.Collation)
	diagnostics = append(diagnostics, optionDiagnostics...)
	if containsError(diagnostics) {
		return Result{Diagnostics: diagnostics}
	}

	statements := []string{
		databaseCreateStatement(name, characterSet, collation),
		"USE " + quoteIdentifier(name) + ";",
	}
	statements = append(statements, plan.tables...)
	for _, foreignKey := range plan.foreignKeys {
		statements = append(statements, foreignKey.AlterSQL())
	}
	return Result{
		SQL:         strings.Join(statements, "\n\n") + "\n",
		Diagnostics: diagnostics,
	}
}

func databaseName(document *pdm.Document, explicit string) (string, []Diagnostic) {
	if explicit != "" {
		return validateDatabaseName(explicit, nil)
	}
	if document == nil {
		return "", nil
	}
	if document.Model.Code != "" {
		return validateDatabaseName(document.Model.Code, document.Model.Raw)
	}
	if document.Model.Name != "" {
		diagnostics := []Diagnostic{rawDiagnostic(
			DiagnosticDatabaseNameFallback, SeverityWarning, objectID(document.Model.Metadata),
			"Model Code is absent; Name is used as the database name.", document.Model.Raw)}
		name, more := validateDatabaseName(document.Model.Name, document.Model.Raw)
		return name, append(diagnostics, more...)
	}
	return "", []Diagnostic{rawDiagnostic(
		DiagnosticDatabaseName, SeverityError, objectID(document.Model.Metadata),
		"Database name is empty.", document.Model.Raw)}
}

func validateDatabaseName(
	name string,
	raw *pdm.Element,
) (string, []Diagnostic) {
	if err := validateIdentifier(name); err != nil {
		return "", []Diagnostic{rawDiagnostic(
			DiagnosticDatabaseName, SeverityError, "", "Database name is invalid: "+err.Error(), raw)}
	}
	if strings.TrimSpace(name) == "" || strings.TrimRight(name, "\t\n\r\f\v ") != name {
		return "", []Diagnostic{rawDiagnostic(
			DiagnosticDatabaseName, SeverityError, "",
			"Database name must contain a non-space character and cannot end in ASCII whitespace.", raw)}
	}
	if strings.Contains(name, ".") {
		return "", []Diagnostic{rawDiagnostic(
			DiagnosticDatabaseNameBoundary, SeverityError, "",
			"Database name contains a qualifier boundary '.'", raw)}
	}
	switch strings.ToLower(name) {
	case "mysql", "information_schema", "performance_schema", "sys":
		return "", []Diagnostic{rawDiagnostic(
			DiagnosticDatabaseSystemName, SeverityError, "",
			"MySQL system database names are not allowed.", raw)}
	}
	return name, nil
}

func databaseCharacterOptions(
	characterSet, collation string,
) (string, string, []Diagnostic) {
	var diagnostics []Diagnostic
	if characterSet != "" && (len(characterSet) > 64 || !validIdentifier(characterSet)) {
		diagnostics = append(diagnostics, Diagnostic{
			Code: DiagnosticDatabaseCharacterSet, Severity: SeverityError,
			Message: "Character set must be an ASCII token.",
		})
		characterSet = ""
	}
	if collation != "" && (len(collation) > 64 || !validIdentifier(collation)) {
		diagnostics = append(diagnostics, Diagnostic{
			Code: DiagnosticDatabaseCollation, Severity: SeverityError,
			Message: "Collation must be an ASCII token.",
		})
		collation = ""
	}
	characterSet = strings.ToLower(characterSet)
	collation = strings.ToLower(collation)
	if characterSet != "" {
		if _, ok := mysqlCharacterSets[characterSet]; !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Code: DiagnosticDatabaseCharacterSet, Severity: SeverityError,
				Message: fmt.Sprintf("Character set %q is not a supported public MySQL 8.0 character set.", characterSet),
			})
		}
	}
	if collation != "" {
		collationSet := collationCharacterSet(collation)
		if collationSet == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Code: DiagnosticDatabaseCollation, Severity: SeverityError,
				Message: "Collation namespace is not recognized among public MySQL 8.0 character sets.",
			})
		} else if characterSet != "" && canonicalCharset(collationSet) != canonicalCharset(characterSet) {
			diagnostics = append(diagnostics, Diagnostic{
				Code: DiagnosticDatabaseOptionMismatch, Severity: SeverityError,
				Message: "Collation prefix does not match the selected character set.",
			})
		}
	}
	return characterSet, collation, diagnostics
}

func databaseCreateStatement(name, characterSet, collation string) string {
	var statement strings.Builder
	statement.WriteString("CREATE DATABASE IF NOT EXISTS ")
	statement.WriteString(quoteIdentifier(name))
	if characterSet != "" {
		statement.WriteString(" CHARACTER SET ")
		statement.WriteString(characterSet)
	}
	if collation != "" {
		statement.WriteString(" COLLATE ")
		statement.WriteString(collation)
	}
	statement.WriteByte(';')
	return statement.String()
}

func collationCharacterSet(collation string) string {
	if collation == "binary" {
		return collation
	}
	prefix, variant, ok := strings.Cut(collation, "_")
	if _, known := mysqlCharacterSets[prefix]; ok && known && variant != "" && prefix != "binary" {
		return prefix
	}
	return ""
}

func canonicalCharset(value string) string {
	if value == "utf8" {
		return "utf8mb3"
	}
	return value
}

var mysqlCharacterSets = map[string]struct{}{
	"armscii8": {}, "ascii": {}, "big5": {}, "binary": {},
	"cp1250": {}, "cp1251": {}, "cp1256": {}, "cp1257": {},
	"cp850": {}, "cp852": {}, "cp866": {}, "cp932": {},
	"dec8": {}, "eucjpms": {}, "euckr": {}, "gb18030": {},
	"gb2312": {}, "gbk": {}, "geostd8": {}, "greek": {},
	"hebrew": {}, "hp8": {}, "keybcs2": {}, "koi8r": {},
	"koi8u": {}, "latin1": {}, "latin2": {}, "latin5": {},
	"latin7": {}, "macce": {}, "macroman": {}, "sjis": {},
	"swe7": {}, "tis620": {}, "ucs2": {}, "ujis": {},
	"utf8": {}, "utf8mb3": {}, "utf8mb4": {},
	"utf16": {}, "utf16le": {}, "utf32": {},
}
