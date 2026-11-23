package sqlgen

import (
	"fmt"
	"strconv"
	"strings"

	"pdmview/internal/pdm"
)

const (
	DiagnosticMissingIndexID           = "missing-index-id"
	DiagnosticDuplicateIndexID         = "duplicate-index-id"
	DiagnosticInvalidIndexIdentifier   = "invalid-index-identifier"
	DiagnosticEmptyIndex               = "empty-index"
	DiagnosticMissingIndexColumn       = "missing-index-column"
	DiagnosticDuplicateIndexColumn     = "duplicate-index-column"
	DiagnosticInvalidIndexDirection    = "invalid-index-direction"
	DiagnosticUnsupportedIndexType     = "unsupported-index-type"
	DiagnosticInvalidIndexPrefix       = "invalid-index-prefix"
	DiagnosticIndexPrefixRequired      = "index-prefix-required"
	DiagnosticUnsupportedIndexPrefix   = "unsupported-index-prefix"
	DiagnosticIndexPrefixOutOfRange    = "index-prefix-out-of-range"
	DiagnosticDuplicateIndexIdentifier = "duplicate-index-identifier"
	DiagnosticIndexColumnLimit         = "index-column-limit"
	DiagnosticIndexKeyTooLong          = "index-key-too-long"
	DiagnosticUnsupportedIndexOption   = "unsupported-index-option"
)

var (
	indexTypeProperties   = []string{"Index.Type", "IndexType", "Index.IndexType", "Index.Method", "Type"}
	indexPrefixProperties = []string{
		"Column.PrefixLength", "IndexColumn.PrefixLength", "PrefixLength", "IndexColumn.Prefix", "Length",
	}
	indexAllowedProperties = map[string]struct{}{
		"Name": {}, "Code": {}, "Comment": {}, "Unique": {},
		"Index.Type": {}, "IndexType": {}, "Index.IndexType": {}, "Index.Method": {}, "Type": {},
	}
	indexColumnAllowedProperties = map[string]struct{}{
		"Name": {}, "Code": {}, "Comment": {}, "SortOrder": {},
		"Column.PrefixLength": {}, "IndexColumn.PrefixLength": {},
		"PrefixLength": {}, "IndexColumn.Prefix": {}, "Length": {},
	}
)

type compiledIndex struct {
	sourceIndex    pdm.Index
	sourceKey      pdm.Key
	fromKey        bool
	identifier     string
	unique         bool
	method         string
	explicitMethod bool
	comment        pdm.OptionalText
	columns        []compiledIndexColumn
}

type compiledIndexColumn struct {
	position  int
	direction string
	prefix    int
}

func (column compiledIndexColumn) directionEffective() string {
	if column.direction == "" {
		return "ASC"
	}
	return column.direction
}

func (compiled *compiledTable) compileIndexes(diagnostics *[]Diagnostic) {
	var candidates []compiledIndex
	for offset, key := range compiled.keys {
		if offset == compiled.primary || len(key.positions) == 0 || key.identifier == "" {
			if offset != compiled.primary && len(key.positions) != 0 {
				compiled.validateKeyIndex(key, diagnostics)
			}
			continue
		}
		compiled.validateKeyIndex(key, diagnostics)
		candidate := compiledIndex{
			sourceKey:  key.source,
			fromKey:    true,
			identifier: key.identifier,
			unique:     true,
			method:     "BTREE",
		}
		for _, position := range key.positions {
			candidate.columns = append(candidate.columns, compiledIndexColumn{position: position})
		}
		candidates = append(candidates, candidate)
	}

	seenIDs := make(map[string]int)
	var explicit []compiledIndex
	for offset, index := range compiled.source.Indexes {
		current := compiled.compileIndex(index, offset, seenIDs, diagnostics)
		if current.identifier != "" && len(current.columns) > 0 {
			explicit = append(explicit, current)
		}
	}

	primarySignature, primaryName := "", ""
	if compiled.primary >= 0 && len(compiled.primaryPositions) > 0 {
		primary := compiledIndex{unique: true, method: "BTREE"}
		for _, position := range compiled.primaryPositions {
			primary.columns = append(primary.columns, compiledIndexColumn{position: position})
		}
		primarySignature = primary.signature()
		primaryName = compiled.keys[compiled.primary].identifier
	}
	physical := explicit[:0]
	for _, index := range explicit {
		isPrimaryName := strings.EqualFold(index.identifier, "PRIMARY")
		if primarySignature != "" && index.signature() == primarySignature &&
			(isPrimaryName || strings.EqualFold(index.identifier, primaryName)) && !index.comment.Present {
			continue
		}
		if isPrimaryName {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticInvalidIndexIdentifier, SeverityError, objectID(index.metadata()),
				"PRIMARY is reserved for the primary key.", index.raw()))
			continue
		}
		physical = append(physical, index)
	}
	explicitUnique := make(map[string]string)
	for _, index := range physical {
		if index.unique {
			explicitUnique[strings.ToLower(index.identifier)] = index.signature()
		}
	}
	var ordered []compiledIndex
	for _, candidate := range candidates {
		if explicitUnique[strings.ToLower(candidate.identifier)] == candidate.signature() {
			continue
		}
		ordered = append(ordered, candidate)
	}
	ordered = append(ordered, physical...)

	names := make(map[string]compiledIndex)
	for _, index := range ordered {
		normalized := strings.ToLower(index.identifier)
		if _, exists := names[normalized]; exists || normalized == "primary" {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticDuplicateIndexIdentifier, SeverityError, objectID(index.metadata()),
				fmt.Sprintf("Index identifier %q is duplicated or reserved.", index.identifier),
				index.raw()))
			continue
		}
		names[normalized] = index
		compiled.indexes = append(compiled.indexes, index)
	}
}

func (compiled *compiledTable) validateKeyIndex(key compiledKey, diagnostics *[]Diagnostic) {
	if len(key.positions) > 16 {
		*diagnostics = append(*diagnostics, keyDiagnostic(
			DiagnosticIndexColumnLimit, key.source, "BTREE key has more than 16 columns."))
	}
	columns := make([]compiledIndexColumn, 0, len(key.positions))
	for _, position := range key.positions {
		columns = append(columns, compiledIndexColumn{position: position})
		column := compiled.columns[position]
		if !column.hasType {
			continue
		}
		switch column.typ.Base {
		case TypeJSON:
			*diagnostics = append(*diagnostics, keyDiagnostic(
				DiagnosticUnsupportedIndexPrefix, key.source,
				"JSON columns cannot be used in a BTREE key."))
		case TypeTinyText, TypeText, TypeMediumText, TypeLongText,
			TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob:
			*diagnostics = append(*diagnostics, keyDiagnostic(
				DiagnosticIndexPrefixRequired, key.source,
				"TEXT/BLOB key columns require an explicit index prefix."))
		}
	}
	if problem := indexKeySizeProblem(*compiled, columns); problem != nil {
		*diagnostics = append(*diagnostics, keyDiagnostic(
			DiagnosticIndexKeyTooLong, key.source, problem.Error()))
	}
}

func (compiled *compiledTable) compileIndex(
	index pdm.Index,
	offset int,
	seenIDs map[string]int,
	diagnostics *[]Diagnostic,
) compiledIndex {
	current := compiledIndex{
		sourceIndex: index,
		identifier:  indexIdentifier(index.Metadata),
		unique:      index.Unique != nil && *index.Unique,
		comment:     index.Comment,
	}
	if index.ID == "" {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticMissingIndexID, SeverityError, objectID(index.Metadata),
			"Index ID is required.", index.Raw))
	} else if existing, ok := seenIDs[index.ID]; ok {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticDuplicateIndexID, SeverityError, objectID(index.Metadata),
			fmt.Sprintf("Index ID duplicates index position %d.", existing+1), index.Raw))
	} else {
		seenIDs[index.ID] = offset
	}
	if err := validateIdentifier(current.identifier); err != nil {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticInvalidIndexIdentifier, SeverityError, objectID(index.Metadata),
			"Index has no valid SQL identifier: "+err.Error(), index.Raw))
		current.identifier = ""
	}
	method, explicitMethod, problem := indexMethod(index.Raw)
	if problem != nil {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticUnsupportedIndexType, SeverityError, objectID(index.Metadata),
			problem.Error(), index.Raw))
	} else {
		current.method = method
		if explicitMethod {
			current.method = "BTREE"
			current.explicitMethod = true
		}
	}
	if len(index.Columns) == 0 {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticEmptyIndex, SeverityError, objectID(index.Metadata),
			"Index has no columns.", index.Raw))
	}
	if len(index.Columns) > 16 {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticIndexColumnLimit, SeverityError, objectID(index.Metadata),
			"BTREE index has more than 16 columns.", index.Raw))
	}
	reportUnsupportedIndexProperties(index.Raw, indexAllowedProperties, "Index", index.Metadata, diagnostics)
	seenColumns := make(map[int]struct{})
	for _, sourceColumn := range index.Columns {
		position, ok := compiled.columnByID[sourceColumn.ColumnID]
		if !ok {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticMissingIndexColumn, SeverityError, objectID(sourceColumn.Metadata),
				fmt.Sprintf("Index references missing column ID %q.", sourceColumn.ColumnID),
				sourceColumn.Raw))
			continue
		}
		if _, duplicate := seenColumns[position]; duplicate {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticDuplicateIndexColumn, SeverityError, objectID(sourceColumn.Metadata),
				fmt.Sprintf("Index repeats column ID %q.", sourceColumn.ColumnID),
				sourceColumn.Raw))
			continue
		}
		seenColumns[position] = struct{}{}
		direction, problem := indexDirection(sourceColumn.SortOrder)
		if problem != nil {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticInvalidIndexDirection, SeverityError, objectID(sourceColumn.Metadata),
				problem.Error(), sourceColumn.Raw))
		}
		prefix, problem := indexPrefix(sourceColumn.Raw)
		if problem != nil {
			*diagnostics = append(*diagnostics, rawDiagnostic(
				DiagnosticInvalidIndexPrefix, SeverityError, objectID(sourceColumn.Metadata),
				problem.Error(), sourceColumn.Raw))
		}
		if problem := validateIndexPrefix(compiled.columns[position], prefix); problem != nil {
			code := DiagnosticUnsupportedIndexPrefix
			switch problem.(type) {
			case *indexPrefixRequired:
				code = DiagnosticIndexPrefixRequired
			case *indexPrefixRange:
				code = DiagnosticIndexPrefixOutOfRange
			}
			*diagnostics = append(*diagnostics, rawDiagnostic(
				code, SeverityError, objectID(sourceColumn.Metadata), problem.Error(), sourceColumn.Raw))
		}
		reportUnsupportedIndexProperties(
			sourceColumn.Raw, indexColumnAllowedProperties, "IndexColumn", sourceColumn.Metadata, diagnostics)
		current.columns = append(current.columns, compiledIndexColumn{
			position: position, direction: direction, prefix: prefix,
		})
	}
	if problem := indexKeySizeProblem(*compiled, current.columns); problem != nil {
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticIndexKeyTooLong, SeverityError, objectID(index.Metadata),
			problem.Error(), index.Raw))
	}
	return current
}

func (index compiledIndex) metadata() pdm.Metadata {
	if index.fromKey {
		return index.sourceKey.Metadata
	}
	return index.sourceIndex.Metadata
}

func (index compiledIndex) raw() *pdm.Element {
	if index.fromKey {
		return index.sourceKey.Raw
	}
	return index.sourceIndex.Raw
}

func (index compiledIndex) signature() string {
	var signature strings.Builder
	if index.unique {
		signature.WriteString("U|")
	} else {
		signature.WriteString("N|")
	}
	signature.WriteString(index.method)
	for _, column := range index.columns {
		signature.WriteString("|")
		signature.WriteString(strconv.Itoa(column.position))
		signature.WriteByte(':')
		signature.WriteString(column.directionEffective())
		signature.WriteByte(':')
		signature.WriteString(strconv.Itoa(column.prefix))
	}
	return signature.String()
}

func renderIndex(
	table compiledTable,
	index compiledIndex,
	comments commentRenderMode,
) string {
	var sql strings.Builder
	if index.unique {
		sql.WriteString("UNIQUE KEY ")
	} else {
		sql.WriteString("KEY ")
	}
	sql.WriteString(quoteIdentifier(index.identifier))
	sql.WriteString(" (")
	for offset, column := range index.columns {
		if offset > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(quoteIdentifier(table.columns[column.position].identifier))
		if column.prefix > 0 {
			sql.WriteByte('(')
			sql.WriteString(strconv.Itoa(column.prefix))
			sql.WriteByte(')')
		}
		if column.direction != "" {
			sql.WriteByte(' ')
			sql.WriteString(column.direction)
		}
	}
	sql.WriteByte(')')
	if index.explicitMethod {
		sql.WriteString(" USING BTREE")
	}
	if comments.rendersObjectComments() && index.comment.Present {
		sql.WriteString(" COMMENT ")
		sql.WriteString(quoteSQLString(index.comment.Value))
	}
	return sql.String()
}

func indexIdentifier(metadata pdm.Metadata) string {
	return firstNonEmpty(metadata.Code, metadata.Name, metadata.ID)
}

func keyIdentifier(metadata pdm.Metadata) string {
	return firstNonEmpty(metadata.Code, metadata.Name, metadata.ID)
}

func indexMethod(raw *pdm.Element) (string, bool, error) {
	value, present, err := rawProperty(raw, indexTypeProperties...)
	if err != nil {
		return "", false, err
	}
	if !present {
		return "BTREE", false, nil
	}
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BTREE":
		return "BTREE", true, nil
	case "", "DEFAULT":
		return "BTREE", false, nil
	default:
		return "", false, fmt.Errorf("index type %q is unsupported; only BTREE/default is supported", value)
	}
}

func indexDirection(value pdm.OptionalText) (string, error) {
	if !value.Present {
		return "", nil
	}
	switch strings.ToUpper(strings.TrimSpace(value.Value)) {
	case "ASC":
		return "ASC", nil
	case "DESC":
		return "DESC", nil
	default:
		return "", fmt.Errorf("index sort order %q is not ASC or DESC", value.Value)
	}
}

func indexPrefix(raw *pdm.Element) (int, error) {
	value, present, err := rawProperty(raw, indexPrefixProperties...)
	if err != nil {
		return 0, err
	}
	if !present {
		return 0, nil
	}
	if !allDigits(value) {
		return 0, fmt.Errorf("index prefix %q is not an unsigned integer", value)
	}
	number, err := strconv.ParseUint(value, 10, 31)
	if err != nil || number == 0 {
		return 0, fmt.Errorf("index prefix must be between 1 and %d", int(^uint(0)>>1))
	}
	return int(number), nil
}

type indexPrefixRequired struct{}

func (*indexPrefixRequired) Error() string {
	return "TEXT/BLOB index columns require a prefix length"
}

type indexPrefixRange struct {
	max int
}

func (problem *indexPrefixRange) Error() string {
	return fmt.Sprintf("index prefix exceeds the column maximum %d", problem.max)
}

func validateIndexPrefix(column compiledColumn, prefix int) error {
	if !column.hasType {
		return nil
	}
	base := column.typ.Base
	if base == TypeJSON {
		return fmt.Errorf("JSON cannot be indexed with BTREE")
	}
	if prefix == 0 {
		switch base {
		case TypeTinyText, TypeText, TypeMediumText, TypeLongText,
			TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob:
			return &indexPrefixRequired{}
		default:
			return nil
		}
	}
	switch base {
	case TypeChar, TypeVarChar, TypeBinary, TypeVarBinary:
		if prefix > column.typ.Length {
			return &indexPrefixRange{max: column.typ.Length}
		}
		return nil
	case TypeTinyText, TypeTinyBlob:
		if prefix > 255 {
			return &indexPrefixRange{max: 255}
		}
		return nil
	case TypeText, TypeBlob:
		if prefix > 65535 {
			return &indexPrefixRange{max: 65535}
		}
		return nil
	case TypeMediumText, TypeMediumBlob:
		if prefix > 16777215 {
			return &indexPrefixRange{max: 16777215}
		}
		return nil
	case TypeLongText, TypeLongBlob:
		return nil
	default:
		return fmt.Errorf("index prefix is not supported for %s", base)
	}
}

func reportUnsupportedIndexProperties(
	raw *pdm.Element,
	allowed map[string]struct{},
	prefix string,
	metadata pdm.Metadata,
	diagnostics *[]Diagnostic,
) {
	if raw == nil {
		return
	}
	for _, child := range raw.Children {
		if child == nil || child.Name.Space != pdm.NamespaceAttribute {
			continue
		}
		if _, ok := allowed[child.Name.Local]; ok {
			continue
		}
		switch child.Name.Local {
		case "ObjectID", "CreationDate", "Creator", "ModificationDate", "Modifier":
			continue
		}
		*diagnostics = append(*diagnostics, rawDiagnostic(
			DiagnosticUnsupportedIndexOption, SeverityError, objectID(metadata),
			fmt.Sprintf("%s property %q is unsupported.", prefix, child.Name.Local), child))
	}
}

func indexKeySizeProblem(table compiledTable, columns []compiledIndexColumn) error {
	total := 0
	for _, indexColumn := range columns {
		column := table.columns[indexColumn.position]
		if !column.hasType {
			continue
		}
		units := indexLengthUnits(column.typ, indexColumn.prefix)
		bytesPerUnit := 1
		if column.typ.Family() == FamilyCharacter || column.typ.Family() == FamilyText {
			switch strings.ToLower(column.typ.CharacterSet) {
			case "utf8mb4":
				bytesPerUnit = 4
			case "utf8", "utf8mb3":
				bytesPerUnit = 3
			case "ucs2":
				bytesPerUnit = 2
			case "utf16", "utf16le", "utf32":
				bytesPerUnit = 4
			}
		}
		total += units * bytesPerUnit
		if total > 3072 {
			return fmt.Errorf("index requires at least %d bytes, exceeding the 3072-byte limit", total)
		}
	}
	return nil
}

func indexLengthUnits(typ MySQLType, prefix int) int {
	if prefix > 0 {
		return prefix
	}
	switch typ.Base {
	case TypeChar, TypeVarChar, TypeBinary, TypeVarBinary:
		return typ.Length
	default:
		return 0
	}
}
