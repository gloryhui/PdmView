package pdm

import "strconv"

const (
	DiagnosticMissingGeometry   = "missing-geometry"
	DiagnosticInvalidGeometry   = "invalid-geometry"
	DiagnosticWrongDiagramScope = "wrong-diagram-scope"
	DiagnosticReferenceMismatch = "reference-endpoint-mismatch"
)

// Point is one signed integer coordinate from PDM geometry text.
type Point struct {
	X int64
	Y int64
}

// Rect is an ordered pair of named corners, with Min inclusive of the input.
type Rect struct {
	Min Point
	Max Point
}

// Diagram contains the symbols drawn on one physical diagram.
type Diagram struct {
	Metadata
	OwnerID          string
	TableAppearance  TableAppearance
	TableSymbols     []TableSymbol
	ReferenceSymbols []ReferenceSymbol
}

// TableSymbol represents one table or external shortcut on a diagram.
type TableSymbol struct {
	Metadata
	TableID    string
	Appearance TableAppearance
	Rect       *Rect
}

// ReferenceSymbol represents a relationship line on one diagram.
type ReferenceSymbol struct {
	Metadata
	ReferenceID         string
	SourceSymbolID      string
	DestinationSymbolID string
	Rect                *Rect
	Points              []Point
	ControlPoints       []Point
}

type diagramState struct {
	raw      *RawDocument
	records  []diagramRecord
	defaults []defaultDiagramRecord
	scopes   map[string]map[string]struct{}
}

type diagramRecord struct {
	diagram        Diagram
	externalTables map[string]bool
}

type defaultDiagramRecord struct {
	ownerID string
	id      string
	element *Element
}

type tableSymbolInfo struct {
	tableID  string
	external bool
}

func newDiagramState(raw *RawDocument) *diagramState {
	return &diagramState{
		raw:    raw,
		scopes: make(map[string]map[string]struct{}),
	}
}

func (p *projector) projectDiagrams(collection *Element, ownerID string, diagnostics *[]Diagnostic) []Diagram {
	var diagrams []Diagram
	for _, member := range collection.Children {
		if !isObjectKind(member, "PhysicalDiagram") {
			p.malformedMember(member, diagnostics)
			continue
		}
		if !p.isDefinition(member) {
			continue
		}
		metadata := p.metadata(member, true, diagnostics)
		appearance := diagramAppearance(member)
		diagram := Diagram{
			Metadata:        metadata,
			OwnerID:         ownerID,
			TableAppearance: appearance,
		}
		externalTables := make(map[string]bool)

		for _, symbols := range member.Children {
			if symbols.Name.Space != NamespaceCollection || symbols.Name.Local != "Symbols" {
				continue
			}
			for _, symbol := range symbols.Children {
				if symbol.Name.Space != NamespaceObject {
					p.malformedMember(symbol, diagnostics)
					continue
				}
				switch symbol.Name.Local {
				case "TableSymbol":
					if !p.isDefinition(symbol) {
						continue
					}
					tableSymbol, external := p.projectTableSymbol(symbol, appearance, diagnostics)
					diagram.TableSymbols = append(diagram.TableSymbols, tableSymbol)
					if tableSymbol.ID != "" && external {
						externalTables[tableSymbol.ID] = true
					}

				case "ReferenceSymbol":
					if !p.isDefinition(symbol) {
						continue
					}
					diagram.ReferenceSymbols = append(
						diagram.ReferenceSymbols,
						p.projectReferenceSymbol(symbol, diagnostics),
					)

				case "RectangleSymbol":
					// Annotation symbols stay available through Diagram.Raw.
				}
			}
		}

		diagrams = append(diagrams, diagram)
		p.diagrams.addDiagram(diagram, externalTables)
	}
	return diagrams
}

func (p *projector) projectDefaultDiagram(collection *Element, ownerID string, diagnostics *[]Diagnostic) string {
	diagram := firstObjectChild(collection, "PhysicalDiagram")
	if diagram == nil {
		p.requiredValue(collection, diagnostics)
		return ""
	}
	id := p.refValue(diagram, diagnostics)
	p.diagrams.addDefault(ownerID, id, diagram)
	return id
}

func (p *projector) projectTableSymbol(symbol *Element, appearance TableAppearance, diagnostics *[]Diagnostic) (TableSymbol, bool) {
	result := TableSymbol{
		Metadata:   p.metadata(symbol, true, diagnostics),
		Appearance: symbolAppearance(appearance, symbol),
	}
	objectCollection := symbol.Child(NamespaceCollection, "Object")
	if objectCollection == nil {
		p.requiredValue(symbol, diagnostics)
		result.Rect = p.rectProperty(symbol, "Rect", true, diagnostics)
		return result, false
	}
	endpoint := firstObjectChild(objectCollection, "Table", "Shortcut")
	if endpoint == nil {
		p.requiredValue(objectCollection, diagnostics)
		result.Rect = p.rectProperty(symbol, "Rect", true, diagnostics)
		return result, false
	}
	external := endpoint.Name.Local == "Shortcut"
	if external {
		p.externalReference(endpoint, diagnostics)
	}
	result.TableID = p.refValue(endpoint, diagnostics)
	result.Rect = p.rectProperty(symbol, "Rect", true, diagnostics)
	return result, external
}

func (p *projector) projectReferenceSymbol(symbol *Element, diagnostics *[]Diagnostic) ReferenceSymbol {
	result := ReferenceSymbol{
		Metadata:      p.metadata(symbol, true, diagnostics),
		Rect:          p.rectProperty(symbol, "Rect", false, diagnostics),
		Points:        p.pointProperty(symbol, "ListOfPoints", true, 2, 0, diagnostics),
		ControlPoints: p.pointProperty(symbol, "ControlPoints", false, 1, 0, diagnostics),
	}

	objectCollection := symbol.Child(NamespaceCollection, "Object")
	if objectCollection == nil {
		p.requiredValue(symbol, diagnostics)
	} else {
		reference := firstObjectChild(objectCollection, "Reference")
		if reference == nil {
			p.requiredValue(objectCollection, diagnostics)
		} else {
			result.ReferenceID = p.refValue(reference, diagnostics)
		}
	}
	result.SourceSymbolID = p.symbolEndpoint(symbol, "SourceSymbol", diagnostics)
	result.DestinationSymbolID = p.symbolEndpoint(symbol, "DestinationSymbol", diagnostics)
	return result
}

func (p *projector) symbolEndpoint(symbol *Element, collectionLocal string, diagnostics *[]Diagnostic) string {
	collection := symbol.Child(NamespaceCollection, collectionLocal)
	if collection == nil {
		p.requiredValue(symbol, diagnostics)
		return ""
	}
	endpoint := firstObjectChild(collection, "TableSymbol")
	if endpoint == nil {
		p.requiredValue(collection, diagnostics)
		return ""
	}
	return p.refValue(endpoint, diagnostics)
}

func (p *projector) rectProperty(element *Element, local string, required bool, diagnostics *[]Diagnostic) *Rect {
	property := element.Child(NamespaceAttribute, local)
	if property == nil {
		if required {
			p.missingGeometry(element, diagnostics)
		}
		return nil
	}
	points, valid := parsePointTuples(property.Text, 2, 2)
	if !valid || !orderedRect(points[0], points[1]) {
		p.invalidGeometry(property, diagnostics)
		return nil
	}
	return &Rect{Min: points[0], Max: points[1]}
}

func (p *projector) pointProperty(
	element *Element,
	local string,
	required bool,
	minPoints int,
	maxPoints int,
	diagnostics *[]Diagnostic,
) []Point {
	property := element.Child(NamespaceAttribute, local)
	if property == nil {
		if required {
			p.missingGeometry(element, diagnostics)
		}
		return nil
	}
	points, valid := parsePointTuples(property.Text, minPoints, maxPoints)
	if !valid {
		p.invalidGeometry(property, diagnostics)
		return nil
	}
	return points
}

func (p *projector) missingGeometry(element *Element, diagnostics *[]Diagnostic) {
	p.diagnose(DiagnosticMissingGeometry, element, diagnostics)
}

func (p *projector) invalidGeometry(element *Element, diagnostics *[]Diagnostic) {
	p.diagnose(DiagnosticInvalidGeometry, element, diagnostics)
}

func orderedRect(min, max Point) bool {
	return min.X <= max.X && min.Y <= max.Y
}

func parsePointTuples(text string, minPoints, maxPoints int) ([]Point, bool) {
	parser := tupleParser{text: text}
	parser.skipSpace()
	if !parser.consume('(') {
		return nil, false
	}
	parser.skipSpace()

	var points []Point
	if parser.consume(')') {
		if minPoints > 0 {
			return nil, false
		}
	} else {
		for {
			if !parser.consume('(') {
				return nil, false
			}
			x, ok := parser.number()
			if !ok {
				return nil, false
			}
			parser.skipSpace()
			if !parser.consume(',') {
				return nil, false
			}
			y, ok := parser.number()
			if !ok {
				return nil, false
			}
			parser.skipSpace()
			if !parser.consume(')') {
				return nil, false
			}
			points = append(points, Point{X: x, Y: y})
			if maxPoints > 0 && len(points) > maxPoints {
				return nil, false
			}

			parser.skipSpace()
			if !parser.consume(',') {
				break
			}
			parser.skipSpace()
		}
		if !parser.consume(')') {
			return nil, false
		}
	}

	parser.skipSpace()
	if parser.pos != len(parser.text) {
		return nil, false
	}
	if len(points) < minPoints {
		return nil, false
	}
	return points, true
}

type tupleParser struct {
	text string
	pos  int
}

func (p *tupleParser) skipSpace() {
	for p.pos < len(p.text) && isXMLSpaceByte(p.text[p.pos]) {
		p.pos++
	}
}

func (p *tupleParser) consume(value byte) bool {
	if p.pos >= len(p.text) || p.text[p.pos] != value {
		return false
	}
	p.pos++
	return true
}

func (p *tupleParser) number() (int64, bool) {
	p.skipSpace()
	start := p.pos
	if p.pos < len(p.text) && (p.text[p.pos] == '-' || p.text[p.pos] == '+') {
		p.pos++
	}
	digits := p.pos
	for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
		p.pos++
	}
	if p.pos == digits {
		return 0, false
	}
	value, err := strconv.ParseInt(p.text[start:p.pos], 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func (s *diagramState) addDiagram(diagram Diagram, externalTables map[string]bool) {
	scope := s.scopes[diagram.OwnerID]
	if scope == nil {
		scope = make(map[string]struct{})
		s.scopes[diagram.OwnerID] = scope
	}
	if diagram.ID != "" {
		scope[diagram.ID] = struct{}{}
	}
	s.records = append(s.records, diagramRecord{
		diagram:        diagram,
		externalTables: externalTables,
	})
}

func (s *diagramState) addDefault(ownerID, id string, element *Element) {
	if id == "" {
		return
	}
	s.defaults = append(s.defaults, defaultDiagramRecord{
		ownerID: ownerID,
		id:      id,
		element: element,
	})
}

func (s *diagramState) validate(schema *schemaState, diagnostics *[]Diagnostic) {
	for _, defaultDiagram := range s.defaults {
		if _, belongs := s.scopes[defaultDiagram.ownerID][defaultDiagram.id]; belongs {
			continue
		}
		target := s.raw.ByID[defaultDiagram.id]
		if isObjectKind(target, "PhysicalDiagram") {
			s.wrongScope(defaultDiagram.element, diagnostics)
		}
	}

	for _, record := range s.records {
		symbols := make(map[string]tableSymbolInfo)
		for _, tableSymbol := range record.diagram.TableSymbols {
			if tableSymbol.ID == "" {
				continue
			}
			symbols[tableSymbol.ID] = tableSymbolInfo{
				tableID:  tableSymbol.TableID,
				external: record.externalTables[tableSymbol.ID],
			}
		}

		for _, referenceSymbol := range record.diagram.ReferenceSymbols {
			source, sourceFound := symbols[referenceSymbol.SourceSymbolID]
			scopeInvalid := false
			if !sourceFound && isObjectKind(s.raw.ByID[referenceSymbol.SourceSymbolID], "TableSymbol") {
				scopeInvalid = true
			}
			destination, destinationFound := symbols[referenceSymbol.DestinationSymbolID]
			if !destinationFound && isObjectKind(s.raw.ByID[referenceSymbol.DestinationSymbolID], "TableSymbol") {
				scopeInvalid = true
			}
			if scopeInvalid {
				s.wrongScope(referenceSymbol.Raw, diagnostics)
			}

			reference, referenceFound := schema.reference(referenceSymbol.ReferenceID)
			if !referenceFound {
				continue
			}
			mismatch := false
			if sourceFound && source.tableID != "" && !source.external &&
				reference.ChildTableID != "" && source.tableID != reference.ChildTableID {
				mismatch = true
			}
			if destinationFound && destination.tableID != "" && !destination.external &&
				reference.ParentTableID != "" && destination.tableID != reference.ParentTableID {
				mismatch = true
			}
			if mismatch {
				s.referenceMismatch(referenceSymbol.Raw, diagnostics)
			}
		}
	}
}

func (s *diagramState) wrongScope(element *Element, diagnostics *[]Diagnostic) {
	if element == nil {
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{
		Code: DiagnosticWrongDiagramScope,
		Path: element.Path,
		Line: element.Line,
	})
}

func (s *diagramState) referenceMismatch(element *Element, diagnostics *[]Diagnostic) {
	if element == nil {
		return
	}
	*diagnostics = append(*diagnostics, Diagnostic{
		Code: DiagnosticReferenceMismatch,
		Path: element.Path,
		Line: element.Line,
	})
}
