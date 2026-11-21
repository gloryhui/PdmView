package sqlgen

import (
	"fmt"
	"sort"
	"strings"

	"pdmview/internal/pdm"
)

const (
	DiagnosticMissingDocument           = "missing-document"
	DiagnosticMultipleRootModels        = "multiple-root-models"
	DiagnosticScopeOverride             = "scope-override"
	DiagnosticQualifiedTableName        = "qualified-table-name"
	DiagnosticConflictingTableID        = "conflicting-table-id"
	DiagnosticConflictingTableName      = "conflicting-table-name"
	DiagnosticMissingForeignKeyID       = "missing-foreign-key-id"
	DiagnosticDuplicateForeignKeyID     = "duplicate-foreign-key-id"
	DiagnosticForeignKeyMissingEndpoint = "foreign-key-missing-endpoint"
	DiagnosticConflictingConstraintName = "conflicting-constraint-name"
)

type databasePlan struct {
	tables      []string
	foreignKeys []compiledForeignKey
}

type renderedTable struct {
	identifier string
	sql        string
}

func compileDatabase(document *pdm.Document) (databasePlan, []Diagnostic) {
	var diagnostics []Diagnostic
	if document == nil {
		return databasePlan{}, []Diagnostic{{
			Code: DiagnosticMissingDocument, Severity: SeverityError, Message: "A PDM document is required.",
		}}
	}
	diagnostics = append(diagnostics, parserDiagnostics(document.Diagnostics)...)
	if count := rootModelCount(document.Raw); count > 1 {
		message := fmt.Sprintf("Document has %d root Models.", count)
		var raw *pdm.Element
		if document.Raw != nil {
			raw = document.Raw.Model
		}
		diagnostics = append(diagnostics, rawDiagnostic(
			DiagnosticMultipleRootModels, SeverityError, objectID(document.Model.Metadata),
			message, raw))
	}

	var tables []pdm.Table
	var references []pdm.Reference
	collectDatabaseContents(
		document.Model.Contents, document.Model.ID,
		document.Model.Raw, "Model", &diagnostics,
		&tables, &references,
	)

	tableIDs := make(map[string]int)
	compiledByID := make(map[string]compiledTable)
	tableNames := make(map[string]int)
	var rendered []renderedTable
	for offset, table := range tables {
		compiled, sql, tableDiagnostics := compileAndRenderTable(table)
		diagnostics = append(diagnostics, tableDiagnostics...)
		if table.ID == "" {
			continue
		}
		if existing, ok := tableIDs[table.ID]; ok {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticConflictingTableID, SeverityError, objectID(table.Metadata),
				fmt.Sprintf("Table ID duplicates table position %d.", existing+1), table.Raw))
		} else {
			tableIDs[table.ID] = offset
			compiledByID[table.ID] = compiled
		}
		if compiled.identifier == "" {
			continue
		}
		folded := strings.ToLower(compiled.identifier)
		if existing, ok := tableNames[folded]; ok {
			diagnostics = append(diagnostics, rawDiagnostic(
				DiagnosticConflictingTableName, SeverityError, objectID(table.Metadata),
				fmt.Sprintf("SQL table name duplicates table position %d.", existing+1), table.Raw))
		} else {
			tableNames[folded] = offset
		}
		if sql != "" {
			rendered = append(rendered, renderedTable{identifier: compiled.identifier, sql: sql})
		}
	}
	sort.SliceStable(rendered, func(left, right int) bool {
		return stableNameLess(rendered[left].identifier, rendered[right].identifier)
	})
	plan := databasePlan{tables: make([]string, len(rendered))}
	for offset := range rendered {
		plan.tables[offset] = rendered[offset].sql
	}

	referenceIDs := make(map[string]int)
	constraintNames := make(map[string]int)
	for offset, reference := range references {
		if reference.ID == "" {
			diagnostics = append(diagnostics, referenceDiagnostic(
				DiagnosticMissingForeignKeyID, reference, "Foreign key ID is required."))
		} else if existing, ok := referenceIDs[reference.ID]; ok {
			diagnostics = append(diagnostics, referenceDiagnostic(
				DiagnosticDuplicateForeignKeyID, reference,
				fmt.Sprintf("Foreign key ID duplicates reference position %d.", existing+1)))
		} else {
			referenceIDs[reference.ID] = offset
		}
		child, childOK := compiledByID[reference.ChildTableID]
		parent, parentOK := compiledByID[reference.ParentTableID]
		if !childOK || !parentOK {
			diagnostics = append(diagnostics, referenceDiagnostic(
				DiagnosticForeignKeyMissingEndpoint, reference,
				"Foreign key child or parent table endpoint is missing."))
			continue
		}
		foreignKey, foreignDiagnostics := compileForeignKey(child, parent, reference)
		diagnostics = append(diagnostics, foreignDiagnostics...)
		if containsError(foreignDiagnostics) || foreignKey.identifier == "" {
			continue
		}
		folded := strings.ToLower(foreignKey.identifier)
		if existing, ok := constraintNames[folded]; ok {
			diagnostics = append(diagnostics, referenceDiagnostic(
				DiagnosticConflictingConstraintName, reference,
				fmt.Sprintf("Constraint name duplicates foreign key position %d.", existing+1)))
		} else {
			constraintNames[folded] = offset
		}
		plan.foreignKeys = append(plan.foreignKeys, foreignKey)
	}
	sort.SliceStable(plan.foreignKeys, func(left, right int) bool {
		return stableNameLess(plan.foreignKeys[left].identifier, plan.foreignKeys[right].identifier)
	})
	return plan, diagnostics
}

func collectDatabaseContents(
	contents pdm.Contents,
	ownerID string,
	raw *pdm.Element,
	rawKind string,
	diagnostics *[]Diagnostic,
	tables *[]pdm.Table,
	references *[]pdm.Reference,
) {
	if problem := scopeOverride(raw, rawKind); problem != "" {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticScopeOverride, SeverityError, ownerID, problem, raw))
	}
	for _, table := range contents.Tables {
		if problem := scopeOverride(table.Raw, "Table"); problem != "" {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticScopeOverride, SeverityError, objectID(table.Metadata), problem, table.Raw))
		}
		name := firstNonEmpty(table.Code, table.Name)
		if strings.Contains(name, ".") {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticQualifiedTableName, SeverityError, objectID(table.Metadata),
				"Qualified SQL table names are unsupported.", table.Raw))
		}
		*tables = append(*tables, table)
	}
	for _, reference := range contents.References {
		*references = append(*references, reference)
	}
	for _, pkg := range contents.Packages {
		collectDatabaseContents(
			pkg.Contents, pkg.ID, pkg.Raw, "Package", diagnostics, tables, references)
	}
}

func parserDiagnostics(diagnostics []pdm.Diagnostic) []Diagnostic {
	var converted []Diagnostic
	for _, source := range diagnostics {
		if diagramDiagnostic(source) {
			continue
		}
		if !sqlRelevantParserPath(source.Path) {
			continue
		}
		severity := SeverityError
		if source.Code == pdm.DiagnosticUnknownNode {
			severity = SeverityWarning
		}
		converted = append(converted, Diagnostic{
			Code:     "pdm-" + source.Code,
			Severity: severity,
			Message:  "PDM parser diagnostic: " + source.Code,
			Path:     source.Path,
			Line:     source.Line,
		})
	}
	return converted
}

func sqlRelevantParserPath(path string) bool {
	path = strings.ToLower(path)
	sawModel, relevant := false, false
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "object:model":
			sawModel = true
		case "collection:packages", "collection:tables", "collection:references":
			relevant = sawModel
		case "collection:dbms", "collection:defaultdiagram", "collection:shortcuts",
			"collection:sessionshortcuts", "collection:targetmodels":
			return false
		}
	}
	return relevant || strings.HasSuffix(path, "/object:model")
}

func diagramDiagnostic(diagnostic pdm.Diagnostic) bool {
	switch diagnostic.Code {
	case pdm.DiagnosticMissingGeometry, pdm.DiagnosticInvalidGeometry,
		pdm.DiagnosticWrongDiagramScope, pdm.DiagnosticReferenceMismatch:
		return true
	}
	path := strings.ToLower(diagnostic.Path)
	return strings.Contains(path, "physicaldiagram")
}

func rootModelCount(raw *pdm.RawDocument) int {
	if raw == nil || raw.Root == nil {
		return 0
	}
	count := 0
	for _, rootObject := range raw.Root.Children {
		if rootObject.Name.Space != pdm.NamespaceObject || rootObject.Name.Local != "RootObject" {
			continue
		}
		for _, children := range rootObject.Children {
			if children.Name.Space != pdm.NamespaceCollection || children.Name.Local != "Children" {
				continue
			}
			for _, child := range children.Children {
				if child.Name.Space == pdm.NamespaceObject && child.Name.Local == "Model" {
					count++
				}
			}
		}
	}
	return count
}

func scopeOverride(raw *pdm.Element, kind string) string {
	if raw == nil {
		return ""
	}
	scopeNames := map[string]struct{}{
		"database": {}, "databases": {}, "schema": {}, "schemas": {},
		"owner": {}, "owners": {}, "qualifier": {}, "qualifiers": {},
	}
	for _, attribute := range raw.Attr {
		if _, ok := scopeNames[strings.ToLower(attribute.Name.Local)]; ok {
			return fmt.Sprintf("%s declares a physical %s qualifier.", kind, attribute.Name.Local)
		}
	}
	for _, child := range raw.Children {
		lower := strings.ToLower(child.Name.Local)
		short := lower
		if dot := strings.LastIndexByte(short, '.'); dot >= 0 {
			short = short[dot+1:]
		}
		if _, ok := scopeNames[short]; ok {
			return fmt.Sprintf("%s declares a physical %s override.", kind, child.Name.Local)
		}
	}
	return ""
}

func stableNameLess(left, right string) bool {
	leftFold, rightFold := strings.ToLower(left), strings.ToLower(right)
	if leftFold == rightFold {
		return left < right
	}
	return leftFold < rightFold
}
