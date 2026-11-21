package pdm

import "io"

const (
	DiagnosticMissingID                 = "missing-id"
	DiagnosticMalformedCollectionMember = "malformed-collection-member"
)

// Document is the typed model projection together with its lossless source.
type Document struct {
	Raw         *RawDocument
	Model       Model
	Diagnostics []Diagnostic
}

// Metadata contains the common fields shared by PDM definitions.
type Metadata struct {
	ID       string
	ObjectID string
	Name     string
	Code     string
	Comment  OptionalText
	Raw      *Element
}

// OptionalText distinguishes an absent property from a present blank one.
type OptionalText struct {
	Value   string
	Present bool
}

// Model contains model metadata plus package and table definitions.
type Model struct {
	Metadata
	DBMS     *Metadata
	Contents Contents
}

// Package is a package definition and its directly owned contents.
type Package struct {
	Metadata
	ParentID string
	Contents Contents
}

// Table is a table definition with its nearest owning model or package ID.
type Table struct {
	Metadata
	OwnerID      string
	Columns      []Column
	Keys         []Key
	PrimaryKeyID string
	Indexes      []Index
}

// Contents holds directly owned package and table definitions.
type Contents struct {
	Packages         []Package
	Tables           []Table
	References       []Reference
	Diagrams         []Diagram
	DefaultDiagramID string
}

// Parse decodes a PDM document and projects its model, packages, and tables.
func Parse(r io.Reader) (*Document, error) {
	tracked := &parseTrackedReader{reader: r}
	raw, err := Decode(tracked)
	if err != nil {
		if tracked.err != nil {
			return nil, tracked.err
		}
		return nil, err
	}
	return Project(raw), nil
}

type parseTrackedReader struct {
	reader io.Reader
	err    error
}

func (r *parseTrackedReader) Read(data []byte) (int, error) {
	if r.reader == nil {
		r.err = ErrReadFailure
		return 0, r.err
	}
	count, err := r.reader.Read(data)
	if err != nil && err != io.EOF && r.err == nil {
		r.err = err
	}
	return count, err
}

// Project maps the raw element tree into the typed P3-04 document model.
func Project(raw *RawDocument) *Document {
	if raw == nil {
		return nil
	}
	document := &Document{
		Raw:         raw,
		Diagnostics: append([]Diagnostic(nil), raw.Diagnostics...),
	}
	if raw.Model == nil {
		return document
	}

	projector := &projector{
		raw:      raw,
		schema:   newSchemaState(),
		diagrams: newDiagramState(raw),
	}
	document.Model.Metadata = projector.metadata(raw.Model, true, &document.Diagnostics)
	document.Model.DBMS = projector.projectDBMS(raw.Model)
	document.Model.Contents = projector.projectContents(raw.Model, document.Model.ID, &document.Diagnostics)
	projector.schema.validate(&document.Diagnostics)
	projector.diagrams.validate(projector.schema, &document.Diagnostics)
	return document
}

type projector struct {
	raw      *RawDocument
	schema   *schemaState
	diagrams *diagramState
}

func (p *projector) metadata(element *Element, requireID bool, diagnostics *[]Diagnostic) Metadata {
	metadata := metadataFromElement(element)
	if requireID && metadata.ID == "" {
		*diagnostics = append(*diagnostics, Diagnostic{
			Code: DiagnosticMissingID,
			Path: element.Path,
			Line: element.Line,
		})
	}
	return metadata
}

func metadataFromElement(element *Element) Metadata {
	if element == nil {
		return Metadata{}
	}
	metadata := Metadata{Raw: element}
	metadata.ID, _ = element.Attribute("Id")
	metadata.ObjectID, _ = element.Property("ObjectID")
	metadata.Name, _ = element.Property("Name")
	metadata.Code, _ = element.Property("Code")
	metadata.Comment = optionalText(element, "Comment")
	return metadata
}

func optionalText(element *Element, local string) OptionalText {
	value, present := element.Property(local)
	return OptionalText{Value: value, Present: present}
}

func (p *projector) projectDBMS(model *Element) *Metadata {
	collection := model.Child(NamespaceCollection, "DBMS")
	if collection == nil {
		return nil
	}

	var shortcut *Element
	var direct *Element
	for _, child := range collection.Children {
		if child.Name.Space != NamespaceObject {
			continue
		}
		switch child.Name.Local {
		case "Shortcut":
			if shortcut == nil {
				shortcut = child
			}
		case "DBMS":
			if direct == nil {
				direct = child
			}
		}
	}

	if shortcut != nil {
		if ref, ok := shortcut.Attribute("Ref"); ok && ref != "" {
			if target := p.raw.ByID[ref]; isObjectKind(target, "Shortcut") {
				metadata := metadataFromElement(target)
				return &metadata
			}
		}
	}
	if direct != nil {
		metadata := metadataFromElement(direct)
		return &metadata
	}
	if shortcut != nil {
		metadata := metadataFromElement(shortcut)
		return &metadata
	}
	return nil
}

func (p *projector) projectContents(parent *Element, ownerID string, diagnostics *[]Diagnostic) Contents {
	var contents Contents
	for _, collection := range parent.Children {
		if collection.Name.Space != NamespaceCollection {
			continue
		}
		switch collection.Name.Local {
		case "Packages":
			for _, member := range collection.Children {
				if !isObjectKind(member, "Package") {
					p.malformedMember(member, diagnostics)
					continue
				}
				if !p.isDefinition(member) {
					continue
				}
				metadata := p.metadata(member, true, diagnostics)
				contents.Packages = append(contents.Packages, Package{
					Metadata: metadata,
					ParentID: ownerID,
					Contents: p.projectContents(member, metadata.ID, diagnostics),
				})
			}

		case "Tables":
			for _, member := range collection.Children {
				if !isObjectKind(member, "Table") {
					p.malformedMember(member, diagnostics)
					continue
				}
				if !p.isDefinition(member) {
					continue
				}
				metadata := p.metadata(member, true, diagnostics)
				contents.Tables = append(contents.Tables, p.projectTable(member, ownerID, metadata, diagnostics))
			}

		case "References":
			contents.References = append(contents.References, p.projectReferences(collection, ownerID, diagnostics)...)

		case "PhysicalDiagrams":
			contents.Diagrams = append(contents.Diagrams, p.projectDiagrams(collection, ownerID, diagnostics)...)

		case "DefaultDiagram":
			id := p.projectDefaultDiagram(collection, ownerID, diagnostics)
			if contents.DefaultDiagramID == "" {
				contents.DefaultDiagramID = id
			}
		}
	}
	return contents
}

func (p *projector) malformedMember(element *Element, diagnostics *[]Diagnostic) {
	if element == nil {
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{
		Code: DiagnosticMalformedCollectionMember,
		Path: element.Path,
		Line: element.Line,
	})
}

func (p *projector) isDefinition(element *Element) bool {
	if isReference(element) {
		return false
	}
	id, _ := element.Attribute("Id")
	return id == "" || p.raw.ByID[id] == element
}

func isReference(element *Element) bool {
	if element == nil {
		return false
	}
	_, present := element.Attribute("Ref")
	return present
}

func isObjectKind(element *Element, local string) bool {
	return element != nil && element.Name.Space == NamespaceObject && element.Name.Local == local
}
