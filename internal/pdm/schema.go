package pdm

const (
	DiagnosticMissingRequiredValue = "missing-required-value"
	DiagnosticInvalidValue         = "invalid-value"
	DiagnosticConflictingValue     = "conflicting-value"
	DiagnosticWrongOwner           = "wrong-owner"
	DiagnosticExternalReference    = "external-reference"
)

// Column contains one ordered table column and its scalar properties.
type Column struct {
	Metadata
	DataType     OptionalText
	Length       OptionalText
	Precision    OptionalText
	Scale        OptionalText
	DefaultValue OptionalText
	Mandatory    *bool
	Identity     *bool
}

// Key contains an ordered list of column definition IDs.
type Key struct {
	Metadata
	ColumnIDs []string
}

// Index contains index metadata and its ordered index columns.
type Index struct {
	Metadata
	Unique  *bool
	Columns []IndexColumn
}

// IndexColumn references a table column and preserves its sort order text.
type IndexColumn struct {
	Metadata
	ColumnID  string
	SortOrder OptionalText
}

// Reference describes a foreign-key relationship between two table scopes.
type Reference struct {
	Metadata
	OwnerID          string
	ParentTableID    string
	ChildTableID     string
	ParentKeyID      string
	Joins            []ReferenceJoin
	UpdateConstraint OptionalText
	DeleteConstraint OptionalText
	Cardinality      OptionalText
}

// ReferenceJoin maps one parent column to one child column.
type ReferenceJoin struct {
	Metadata
	ParentColumnID string
	ChildColumnID  string
}

type schemaState struct {
	tables        map[string]struct{}
	columns       map[string]string
	keys          map[string]string
	referenceByID map[string]Reference
	keyList       []keyRecord
	primary       []primaryKeyRecord
	indexes       []indexRecord
	refs          []referenceRecord
}

type schemaRef struct {
	id      string
	element *Element
}

type keyRecord struct {
	tableID string
	columns []schemaRef
}

type indexRecord struct {
	tableID string
	columns []schemaRef
}

type primaryKeyRecord struct {
	tableID string
	key     schemaRef
}

type referenceRecord struct {
	reference         Reference
	parentKeyElement  *Element
	parentExternal    bool
	childExternal     bool
	parentKeyExternal bool
	joins             []joinRecord
}

type joinRecord struct {
	parent schemaRef
	child  schemaRef
}

func newSchemaState() *schemaState {
	return &schemaState{
		tables:        make(map[string]struct{}),
		columns:       make(map[string]string),
		keys:          make(map[string]string),
		referenceByID: make(map[string]Reference),
	}
}

func (p *projector) projectTable(element *Element, ownerID string, metadata Metadata, diagnostics *[]Diagnostic) Table {
	table := Table{
		Metadata: metadata,
		OwnerID:  ownerID,
		Columns:  p.projectColumns(element, metadata.ID, diagnostics),
		Keys:     p.projectKeys(element, metadata.ID, diagnostics),
		Indexes:  p.projectIndexes(element, metadata.ID, diagnostics),
	}
	table.PrimaryKeyID = p.projectPrimaryKey(element, metadata.ID, diagnostics)
	p.schema.addTable(metadata.ID)
	return table
}

func (p *projector) projectColumns(table *Element, tableID string, diagnostics *[]Diagnostic) []Column {
	var columns []Column
	for _, collection := range table.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "Columns" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "Column") {
				p.malformedMember(member, diagnostics)
				continue
			}
			if !p.isDefinition(member) {
				continue
			}
			metadata := p.metadata(member, true, diagnostics)
			column := Column{
				Metadata:     metadata,
				DataType:     optionalText(member, "DataType"),
				Length:       optionalText(member, "Length"),
				Precision:    optionalText(member, "Precision"),
				Scale:        optionalText(member, "Scale"),
				DefaultValue: optionalText(member, "DefaultValue"),
				Mandatory:    p.columnMandatory(member, diagnostics),
				Identity:     p.booleanChild(member, "Identity", diagnostics),
			}
			columns = append(columns, column)
			p.schema.addColumn(tableID, column.ID)
		}
	}
	return columns
}

func (p *projector) projectKeys(table *Element, tableID string, diagnostics *[]Diagnostic) []Key {
	var keys []Key
	for _, collection := range table.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "Keys" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "Key") {
				p.malformedMember(member, diagnostics)
				continue
			}
			if !p.isDefinition(member) {
				continue
			}
			metadata := p.metadata(member, true, diagnostics)
			columnIDs, refs := p.projectKeyColumns(member, diagnostics)
			keys = append(keys, Key{Metadata: metadata, ColumnIDs: columnIDs})
			p.schema.addKey(tableID, metadata.ID, refs)
		}
	}
	return keys
}

func (p *projector) projectKeyColumns(key *Element, diagnostics *[]Diagnostic) ([]string, []schemaRef) {
	var ids []string
	var refs []schemaRef
	for _, collection := range key.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "Key.Columns" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "Column") {
				p.malformedMember(member, diagnostics)
				continue
			}
			id, present := member.Attribute("Ref")
			if !present || id == "" {
				p.requiredValue(member, diagnostics)
			}
			if !present {
				continue
			}
			ids = append(ids, id)
			if id != "" {
				refs = append(refs, schemaRef{id: id, element: member})
			}
		}
	}
	if len(ids) == 0 {
		p.requiredValue(key, diagnostics)
	}
	return ids, refs
}

func (p *projector) projectPrimaryKey(table *Element, tableID string, diagnostics *[]Diagnostic) string {
	for _, collection := range table.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "PrimaryKey" {
			continue
		}
		key := firstObjectChild(collection, "Key")
		if key == nil {
			p.requiredValue(collection, diagnostics)
			return ""
		}
		id := p.refValue(key, diagnostics)
		p.schema.addPrimaryKey(tableID, id, key)
		return id
	}
	return ""
}

func (p *projector) projectIndexes(table *Element, tableID string, diagnostics *[]Diagnostic) []Index {
	var indexes []Index
	for _, collection := range table.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "Indexes" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "Index") {
				p.malformedMember(member, diagnostics)
				continue
			}
			if !p.isDefinition(member) {
				continue
			}
			metadata := p.metadata(member, true, diagnostics)
			columns, refs := p.projectIndexColumns(member, diagnostics)
			indexes = append(indexes, Index{
				Metadata: metadata,
				Unique:   p.booleanChild(member, "Unique", diagnostics),
				Columns:  columns,
			})
			p.schema.addIndex(tableID, refs)
		}
	}
	return indexes
}

func (p *projector) projectIndexColumns(index *Element, diagnostics *[]Diagnostic) ([]IndexColumn, []schemaRef) {
	var columns []IndexColumn
	var refs []schemaRef
	for _, collection := range index.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "IndexColumns" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "IndexColumn") {
				p.malformedMember(member, diagnostics)
				continue
			}
			if !p.isDefinition(member) {
				continue
			}
			metadata := p.metadata(member, true, diagnostics)
			indexColumn := IndexColumn{
				Metadata:  metadata,
				SortOrder: optionalText(member, "SortOrder"),
			}
			column := firstObjectColumn(member)
			if column == nil {
				p.requiredValue(member, diagnostics)
			} else {
				indexColumn.ColumnID = p.refValue(column, diagnostics)
				if indexColumn.ColumnID != "" {
					refs = append(refs, schemaRef{id: indexColumn.ColumnID, element: column})
				}
			}
			columns = append(columns, indexColumn)
		}
	}
	if len(columns) == 0 {
		p.requiredValue(index, diagnostics)
	}
	return columns, refs
}

func (p *projector) projectReferences(collection *Element, ownerID string, diagnostics *[]Diagnostic) []Reference {
	var references []Reference
	for _, member := range collection.Children {
		if !isObjectKind(member, "Reference") {
			p.malformedMember(member, diagnostics)
			continue
		}
		if !p.isDefinition(member) {
			continue
		}
		metadata := p.metadata(member, true, diagnostics)
		reference := Reference{
			Metadata:         metadata,
			OwnerID:          ownerID,
			UpdateConstraint: optionalText(member, "UpdateConstraint"),
			DeleteConstraint: optionalText(member, "DeleteConstraint"),
			Cardinality:      optionalText(member, "Cardinality"),
		}

		var parentExternal bool
		var childExternal bool
		reference.ParentTableID, parentExternal = p.projectTableEndpoint(member, "ParentTable", diagnostics)
		reference.ChildTableID, childExternal = p.projectTableEndpoint(member, "ChildTable", diagnostics)
		parentKeyElement, parentKeyExternal := p.projectParentKey(member, &reference.ParentKeyID, diagnostics)
		var joins []joinRecord
		reference.Joins, joins = p.projectReferenceJoins(member, diagnostics)
		references = append(references, reference)
		p.schema.addReference(reference, parentKeyElement, joins, parentExternal, childExternal, parentKeyExternal)
	}
	return references
}

func (p *projector) projectTableEndpoint(reference *Element, collectionLocal string, diagnostics *[]Diagnostic) (string, bool) {
	collection := reference.Child(NamespaceCollection, collectionLocal)
	if collection == nil {
		p.requiredValue(reference, diagnostics)
		return "", false
	}
	endpoint := firstObjectChild(collection, "Table", "Shortcut")
	if endpoint == nil {
		p.requiredValue(collection, diagnostics)
		return "", false
	}
	external := false
	if endpoint.Name.Local == "Shortcut" {
		external = true
		p.externalReference(endpoint, diagnostics)
	}
	return p.refValue(endpoint, diagnostics), external
}

func (p *projector) projectParentKey(reference *Element, target *string, diagnostics *[]Diagnostic) (*Element, bool) {
	collection := reference.Child(NamespaceCollection, "ParentKey")
	if collection == nil {
		p.requiredValue(reference, diagnostics)
		return nil, false
	}
	key := firstObjectChild(collection, "Key", "Shortcut")
	if key == nil {
		p.requiredValue(collection, diagnostics)
		return nil, false
	}
	external := false
	if key.Name.Local == "Shortcut" {
		external = true
		p.externalReference(key, diagnostics)
	}
	*target = p.refValue(key, diagnostics)
	return key, external
}

func (p *projector) projectReferenceJoins(reference *Element, diagnostics *[]Diagnostic) ([]ReferenceJoin, []joinRecord) {
	var joins []ReferenceJoin
	var records []joinRecord
	for _, collection := range reference.Children {
		if collection.Name.Space != NamespaceCollection || collection.Name.Local != "Joins" {
			continue
		}
		for _, member := range collection.Children {
			if !isObjectKind(member, "ReferenceJoin") {
				p.malformedMember(member, diagnostics)
				continue
			}
			if !p.isDefinition(member) {
				continue
			}
			metadata := p.metadata(member, true, diagnostics)
			join := ReferenceJoin{Metadata: metadata}
			parentElement := p.joinColumn(member, "Object1", &join.ParentColumnID, diagnostics)
			childElement := p.joinColumn(member, "Object2", &join.ChildColumnID, diagnostics)
			joins = append(joins, join)
			records = append(records, joinRecord{
				parent: schemaRef{id: join.ParentColumnID, element: parentElement},
				child:  schemaRef{id: join.ChildColumnID, element: childElement},
			})
		}
	}
	if len(joins) == 0 {
		p.requiredValue(reference, diagnostics)
	}
	return joins, records
}

func (p *projector) joinColumn(join *Element, collectionLocal string, target *string, diagnostics *[]Diagnostic) *Element {
	collection := join.Child(NamespaceCollection, collectionLocal)
	if collection == nil {
		p.requiredValue(join, diagnostics)
		return nil
	}
	column := firstObjectChild(collection, "Column")
	if column == nil {
		p.requiredValue(collection, diagnostics)
		return nil
	}
	*target = p.refValue(column, diagnostics)
	return column
}

func (p *projector) columnMandatory(column *Element, diagnostics *[]Diagnostic) *bool {
	modernElement := column.Child(NamespaceAttribute, "Column.Mandatory")
	legacyElement := column.Child(NamespaceAttribute, "Mandatory")
	modern, modernPresent, modernValid := p.booleanElement(modernElement, diagnostics)
	legacy, legacyPresent, legacyValid := p.booleanElement(legacyElement, diagnostics)

	if modernPresent && legacyPresent && modernValid && legacyValid && modern != legacy {
		p.diagnose(DiagnosticConflictingValue, modernElement, diagnostics)
		return nil
	}
	if modernValid {
		return boolPointer(modern)
	}
	if legacyValid {
		return boolPointer(legacy)
	}
	return nil
}

func (p *projector) booleanChild(element *Element, local string, diagnostics *[]Diagnostic) *bool {
	value, present, valid := p.booleanElement(element.Child(NamespaceAttribute, local), diagnostics)
	if !present || !valid {
		return nil
	}
	return boolPointer(value)
}

func (p *projector) booleanElement(element *Element, diagnostics *[]Diagnostic) (bool, bool, bool) {
	if element == nil {
		return false, false, false
	}
	switch element.Text {
	case "1":
		return true, true, true
	case "0":
		return false, true, true
	default:
		p.diagnose(DiagnosticInvalidValue, element, diagnostics)
		return false, true, false
	}
}

func (p *projector) refValue(element *Element, diagnostics *[]Diagnostic) string {
	if element == nil {
		return ""
	}
	id, present := element.Attribute("Ref")
	if !present || id == "" {
		p.requiredValue(element, diagnostics)
	}
	return id
}

func (p *projector) requiredValue(element *Element, diagnostics *[]Diagnostic) {
	p.diagnose(DiagnosticMissingRequiredValue, element, diagnostics)
}

func (p *projector) externalReference(element *Element, diagnostics *[]Diagnostic) {
	p.diagnose(DiagnosticExternalReference, element, diagnostics)
}

func (p *projector) diagnose(code string, element *Element, diagnostics *[]Diagnostic) {
	if element == nil {
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{
		Code: code,
		Path: element.Path,
		Line: element.Line,
	})
}

func boolPointer(value bool) *bool {
	return &value
}

func firstObjectChild(parent *Element, locals ...string) *Element {
	if parent == nil {
		return nil
	}
	for _, child := range parent.Children {
		if child.Name.Space != NamespaceObject {
			continue
		}
		for _, local := range locals {
			if child.Name.Local == local {
				return child
			}
		}
	}
	return nil
}

func firstObjectColumn(indexColumn *Element) *Element {
	collection := indexColumn.Child(NamespaceCollection, "Column")
	if collection == nil {
		return nil
	}
	return firstObjectChild(collection, "Column")
}

func (s *schemaState) addTable(id string) {
	if id != "" {
		s.tables[id] = struct{}{}
	}
}

func (s *schemaState) addColumn(tableID, id string) {
	if id == "" {
		return
	}
	if _, exists := s.columns[id]; !exists {
		s.columns[id] = tableID
	}
}

func (s *schemaState) addKey(tableID, id string, columns []schemaRef) {
	if id != "" {
		if _, exists := s.keys[id]; !exists {
			s.keys[id] = tableID
		}
	}
	s.keyList = append(s.keyList, keyRecord{tableID: tableID, columns: columns})
}

func (s *schemaState) addPrimaryKey(tableID, id string, element *Element) {
	if id == "" {
		return
	}
	s.primary = append(s.primary, primaryKeyRecord{
		tableID: tableID,
		key:     schemaRef{id: id, element: element},
	})
}

func (s *schemaState) addIndex(tableID string, columns []schemaRef) {
	s.indexes = append(s.indexes, indexRecord{tableID: tableID, columns: columns})
}

func (s *schemaState) addReference(
	reference Reference,
	parentKey *Element,
	joins []joinRecord,
	parentExternal bool,
	childExternal bool,
	parentKeyExternal bool,
) {
	if reference.ID != "" {
		if _, exists := s.referenceByID[reference.ID]; !exists {
			s.referenceByID[reference.ID] = reference
		}
	}
	s.refs = append(s.refs, referenceRecord{
		reference:         reference,
		parentKeyElement:  parentKey,
		parentExternal:    parentExternal,
		childExternal:     childExternal,
		parentKeyExternal: parentKeyExternal,
		joins:             joins,
	})
}

func (s *schemaState) reference(id string) (Reference, bool) {
	reference, exists := s.referenceByID[id]
	return reference, exists
}

func (s *schemaState) validate(diagnostics *[]Diagnostic) {
	for _, key := range s.keyList {
		if key.tableID == "" {
			continue
		}
		for _, column := range key.columns {
			if owner, found := s.columns[column.id]; found && owner != key.tableID {
				s.wrongOwner(column.element, diagnostics)
			}
		}
	}
	for _, primary := range s.primary {
		if primary.tableID == "" {
			continue
		}
		if owner, found := s.keys[primary.key.id]; found && owner != primary.tableID {
			s.wrongOwner(primary.key.element, diagnostics)
		}
	}
	for _, index := range s.indexes {
		if index.tableID == "" {
			continue
		}
		for _, column := range index.columns {
			if owner, found := s.columns[column.id]; found && owner != index.tableID {
				s.wrongOwner(column.element, diagnostics)
			}
		}
	}
	for _, record := range s.refs {
		_, foundParent := s.tables[record.reference.ParentTableID]
		_, foundChild := s.tables[record.reference.ChildTableID]
		localParent := foundParent && !record.parentExternal
		localChild := foundChild && !record.childExternal
		if localParent && !record.parentKeyExternal {
			if owner, found := s.keys[record.reference.ParentKeyID]; found &&
				owner != record.reference.ParentTableID {
				s.wrongOwner(record.parentKeyElement, diagnostics)
			}
		}
		for _, join := range record.joins {
			if localParent {
				if owner, found := s.columns[join.parent.id]; found &&
					owner != record.reference.ParentTableID {
					s.wrongOwner(join.parent.element, diagnostics)
				}
			}
			if localChild {
				if owner, found := s.columns[join.child.id]; found &&
					owner != record.reference.ChildTableID {
					s.wrongOwner(join.child.element, diagnostics)
				}
			}
		}
	}
}

func (s *schemaState) wrongOwner(element *Element, diagnostics *[]Diagnostic) {
	if element == nil {
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{
		Code: DiagnosticWrongOwner,
		Path: element.Path,
		Line: element.Line,
	})
}
