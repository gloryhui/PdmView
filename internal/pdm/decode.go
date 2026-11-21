// Package pdm reads PowerDesigner PDM XML into a lossless raw tree.
package pdm

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

const (
	// MaxDocumentSize is the largest PDM document accepted by Decode.
	MaxDocumentSize int64 = 256 << 20
	// MaxDepth is the largest element nesting depth accepted by Decode.
	MaxDepth    = 128
	MaxElements = 1_000_000

	// PDMSignature is the writer signature required for PDM XML.
	PDMSignature = "PDM_DATA_MODEL_XML"

	// NamespaceObject is the PowerDesigner object namespace URI.
	NamespaceObject = "object"
	// NamespaceCollection is the PowerDesigner collection namespace URI.
	NamespaceCollection = "collection"
	// NamespaceAttribute is the PowerDesigner attribute namespace URI.
	NamespaceAttribute = "attribute"
)

const (
	DiagnosticMissingVersion        = "missing-version"
	DiagnosticUnknownNode           = "unknown-node"
	DiagnosticDuplicateDefinition   = "duplicate-definition"
	DiagnosticMissingReference      = "missing-reference"
	DiagnosticReferenceKindMismatch = "reference-kind-mismatch"
)

var (
	ErrMalformedXML         = errors.New("pdm: malformed XML")
	ErrUnsupportedEncoding  = errors.New("pdm: unsupported encoding")
	ErrUnsupportedDirective = errors.New("pdm: unsupported XML directive")
	ErrDocumentTooLarge     = errors.New("pdm: document exceeds size limit")
	ErrDepthLimit           = errors.New("pdm: document exceeds depth limit")
	ErrElementLimit         = errors.New("pdm: document exceeds element limit")
	ErrMultipleRoots        = errors.New("pdm: multiple root elements")
	ErrInvalidStructure     = errors.New("pdm: invalid PDM document structure")
	ErrForeignSignature     = errors.New("pdm: foreign XML signature")
	ErrReadFailure          = errors.New("pdm: document read failed")
)

// Format records the envelope information declared by the PDM document.
type Format struct {
	PowerDesignerVersion string
	Signature            string
	XMLVersion           string
	Encoding             string
}

// Diagnostic identifies a non-fatal issue without retaining an XML value.
type Diagnostic struct {
	Code string
	Path string
	Line int
}

// RawDocument is the lossless element tree plus document-level indexes.
type RawDocument struct {
	Root        *Element
	Model       *Element
	Format      Format
	ByID        map[string]*Element
	Diagnostics []Diagnostic
}

// Element preserves a decoded XML element's names, attributes, text, and order.
// Path uses normalized namespace labels: object, collection, and attribute.
type Element struct {
	Name     xml.Name
	Attr     []xml.Attr
	Text     string
	Children []*Element
	Line     int
	Path     string
}

// Child returns the first direct child in space with the given local name.
func (e *Element) Child(space, local string) *Element {
	if e == nil {
		return nil
	}
	for _, child := range e.Children {
		if child.Name.Space == space && child.Name.Local == local {
			return child
		}
	}
	return nil
}

// Attribute returns the first unqualified attribute with the given local name.
func (e *Element) Attribute(local string) (string, bool) {
	if e == nil {
		return "", false
	}
	for _, attr := range e.Attr {
		if attr.Name.Space == "" && attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return "", false
}

// Property returns the exact text of the first attribute-namespace child with
// the given local name. A present but blank property returns "", true.
func (e *Element) Property(local string) (string, bool) {
	child := e.Child(NamespaceAttribute, local)
	if child == nil {
		return "", false
	}
	return child.Text, true
}

// Decode reads a PowerDesigner PDM XML document using strict XML tokens.
func Decode(r io.Reader) (*RawDocument, error) {
	return decodeWithLimits(r, MaxDocumentSize, MaxDepth)
}

func decodeWithLimits(r io.Reader, maxBytes int64, maxDepth int) (*RawDocument, error) {
	if r == nil {
		return nil, ErrReadFailure
	}
	if maxBytes <= 0 || maxDepth <= 0 {
		return nil, ErrMalformedXML
	}

	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	buffered := bufio.NewReader(limited)
	hadBOM, err := stripUTF8BOM(buffered)
	if err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(buffered)
	decoder.Strict = true
	decoder.CharsetReader = func(string, io.Reader) (io.Reader, error) {
		return nil, ErrUnsupportedEncoding
	}

	doc := &RawDocument{
		Format: Format{XMLVersion: "1.0", Encoding: "UTF-8"},
		ByID:   make(map[string]*Element),
	}
	if hadBOM {
		doc.Format.Encoding = "UTF-8"
	}

	type openElement struct {
		node *Element
		text []byte
	}
	var stack []openElement
	var elementCount int
	var sawPowerDesigner bool
	var powerDesignerLine int

	for {
		line, _ := decoder.InputPos()
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			if limited.N <= 0 {
				return nil, tooLargeError(maxBytes)
			}
			if errors.Is(tokenErr, ErrUnsupportedEncoding) {
				return nil, ErrUnsupportedEncoding
			}
			if errors.Is(tokenErr, io.EOF) {
				break
			}
			return nil, malformedError(line)
		}
		if limited.N <= 0 {
			return nil, tooLargeError(maxBytes)
		}

		switch value := token.(type) {
		case xml.StartElement:
			elementCount++
			if elementCount > MaxElements {
				return nil, ErrElementLimit
			}
			if len(stack) == 0 && doc.Root != nil {
				return nil, ErrMultipleRoots
			}
			depth := len(stack) + 1
			if depth > maxDepth {
				return nil, fmt.Errorf("%w: max %d", ErrDepthLimit, maxDepth)
			}
			var parent *Element
			if len(stack) > 0 {
				parent = stack[len(stack)-1].node
			}
			element := &Element{
				Name: value.Name,
				Attr: append([]xml.Attr(nil), value.Attr...),
				Line: line,
				Path: elementPath(parent, value.Name),
			}
			if parent == nil {
				doc.Root = element
			} else {
				parent.Children = append(parent.Children, element)
			}
			if !isKnownElement(value.Name) {
				doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
					Code: DiagnosticUnknownNode,
					Path: element.Path,
					Line: element.Line,
				})
			}
			stack = append(stack, openElement{node: element})

		case xml.EndElement:
			if len(stack) == 0 {
				return nil, malformedError(line)
			}
			last := len(stack) - 1
			frame := &stack[last]
			frame.node.Text = string(frame.text)
			*frame = openElement{}
			stack = stack[:last]

		case xml.CharData:
			if len(stack) > 0 {
				frame := &stack[len(stack)-1]
				frame.text = append(frame.text, value...)
			} else if !isXMLSpace(value) {
				return nil, malformedError(line)
			}

		case xml.Comment:
			// Comments are not part of the text or object model.

		case xml.ProcInst:
			if len(stack) != 0 {
				continue
			}
			switch value.Target {
			case "xml":
				if doc.Root != nil {
					return nil, malformedError(line)
				}
				attrs, parseErr := parsePseudoAttributes(value.Inst)
				if parseErr != nil {
					return nil, malformedError(line)
				}
				if version := attrs["version"]; version != "" {
					if version != "1.0" {
						return nil, ErrUnsupportedEncoding
					}
					doc.Format.XMLVersion = version
				}
				if encoding := attrs["encoding"]; encoding != "" {
					if !strings.EqualFold(encoding, "UTF-8") {
						return nil, ErrUnsupportedEncoding
					}
					doc.Format.Encoding = encoding
				}

			case "PowerDesigner":
				attrs, parseErr := parsePseudoAttributes(value.Inst)
				if parseErr != nil {
					return nil, malformedError(line)
				}
				sawPowerDesigner = true
				powerDesignerLine = line
				doc.Format.PowerDesignerVersion = attrs["version"]
				doc.Format.Signature = attrs["signature"]
				if doc.Format.Signature != "" && doc.Format.Signature != PDMSignature {
					return nil, ErrForeignSignature
				}
			}

		case xml.Directive:
			return nil, ErrUnsupportedDirective
		}
	}

	if limited.N <= 0 {
		return nil, tooLargeError(maxBytes)
	}
	if doc.Root == nil {
		return nil, malformedError(1)
	}

	model, structureErr := findModel(doc.Root)
	if structureErr != nil {
		return nil, structureErr
	}
	doc.Model = model

	if !sawPowerDesigner {
		doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
			Code: DiagnosticMissingVersion,
			Path: doc.Root.Path,
			Line: doc.Root.Line,
		})
	} else if doc.Format.PowerDesignerVersion == "" {
		doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
			Code: DiagnosticMissingVersion,
			Path: doc.Root.Path,
			Line: powerDesignerLine,
		})
	}

	indexDefinitions(doc)
	checkReferences(doc)
	return doc, nil
}

func stripUTF8BOM(reader *bufio.Reader) (bool, error) {
	prefix, err := reader.Peek(3)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, ErrReadFailure
	}
	if len(prefix) >= 3 && bytes.Equal(prefix[:3], []byte{0xEF, 0xBB, 0xBF}) {
		if _, discardErr := reader.Discard(3); discardErr != nil {
			return false, ErrReadFailure
		}
		return true, nil
	}
	if len(prefix) >= 2 &&
		((prefix[0] == 0xFF && prefix[1] == 0xFE) || (prefix[0] == 0xFE && prefix[1] == 0xFF)) {
		return false, ErrUnsupportedEncoding
	}
	return false, nil
}

func findModel(root *Element) (*Element, error) {
	if root == nil || root.Name.Space != "" || root.Name.Local != "Model" {
		return nil, ErrInvalidStructure
	}
	rootObject := root.Child(NamespaceObject, "RootObject")
	if rootObject == nil {
		return nil, ErrInvalidStructure
	}
	children := rootObject.Child(NamespaceCollection, "Children")
	if children == nil {
		return nil, ErrInvalidStructure
	}
	model := children.Child(NamespaceObject, "Model")
	if model == nil {
		return nil, ErrInvalidStructure
	}
	return model, nil
}

func elementPath(parent *Element, name xml.Name) string {
	segment := name.Local
	if !isKnownElement(name) {
		segment = "?"
	}
	switch name.Space {
	case NamespaceObject:
		segment = "object:" + segment
	case NamespaceCollection:
		segment = "collection:" + segment
	case NamespaceAttribute:
		segment = "attribute:" + segment
	case "":
	default:
		segment = "unknown:?"
	}
	if parent == nil {
		return "/" + segment
	}
	return parent.Path + "/" + segment
}

func parsePseudoAttributes(inst []byte) (map[string]string, error) {
	text := string(inst)
	attrs := make(map[string]string)
	for offset := 0; offset < len(text); {
		for offset < len(text) && isXMLSpaceByte(text[offset]) {
			offset++
		}
		if offset == len(text) {
			break
		}
		nameStart := offset
		for offset < len(text) && !isXMLSpaceByte(text[offset]) && text[offset] != '=' {
			offset++
		}
		if nameStart == offset {
			return nil, ErrMalformedXML
		}
		name := text[nameStart:offset]
		for offset < len(text) && isXMLSpaceByte(text[offset]) {
			offset++
		}
		if offset == len(text) || text[offset] != '=' {
			return nil, ErrMalformedXML
		}
		offset++
		for offset < len(text) && isXMLSpaceByte(text[offset]) {
			offset++
		}
		if offset == len(text) || (text[offset] != '"' && text[offset] != '\'') {
			return nil, ErrMalformedXML
		}
		quote := text[offset]
		offset++
		valueStart := offset
		for offset < len(text) && text[offset] != quote {
			offset++
		}
		if offset == len(text) {
			return nil, ErrMalformedXML
		}
		attrs[name] = text[valueStart:offset]
		offset++
	}
	return attrs, nil
}

func indexDefinitions(doc *RawDocument) {
	walk(doc.Root, func(element *Element) {
		if element.Name.Space != NamespaceObject {
			return
		}
		id, ok := element.Attribute("Id")
		if !ok || id == "" {
			return
		}
		if _, exists := doc.ByID[id]; exists {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Code: DiagnosticDuplicateDefinition,
				Path: element.Path,
				Line: element.Line,
			})
			return
		}
		doc.ByID[id] = element
	})
}

func checkReferences(doc *RawDocument) {
	walk(doc.Root, func(element *Element) {
		if element.Name.Space != NamespaceObject {
			return
		}
		ref, ok := element.Attribute("Ref")
		if !ok {
			return
		}
		target, exists := doc.ByID[ref]
		if !exists {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Code: DiagnosticMissingReference,
				Path: element.Path,
				Line: element.Line,
			})
			return
		}
		if target.Name.Local != element.Name.Local {
			doc.Diagnostics = append(doc.Diagnostics, Diagnostic{
				Code: DiagnosticReferenceKindMismatch,
				Path: element.Path,
				Line: element.Line,
			})
		}
	})
}

func walk(element *Element, visit func(*Element)) {
	if element == nil {
		return
	}
	visit(element)
	for _, child := range element.Children {
		walk(child, visit)
	}
}

func isKnownElement(name xml.Name) bool {
	if name.Space == NamespaceAttribute {
		return slices.Contains(knownProperties, name.Local)
	}
	if name.Space == NamespaceObject {
		_, ok := knownObjectElements[name.Local]
		return ok
	}
	if name.Space == NamespaceCollection {
		_, ok := knownCollectionElements[name.Local]
		return ok
	}
	return name.Space == "" && name.Local == "Model"
}

var knownObjectElements = map[string]struct{}{
	"Column":          {},
	"DBMS":            {},
	"Group":           {},
	"Index":           {},
	"IndexColumn":     {},
	"Key":             {},
	"Model":           {},
	"Package":         {},
	"PhysicalDiagram": {},
	"RectangleSymbol": {},
	"Reference":       {},
	"ReferenceJoin":   {},
	"ReferenceSymbol": {},
	"RootObject":      {},
	"Shortcut":        {},
	"Table":           {},
	"TableSymbol":     {},
	"TargetModel":     {},
}

var knownCollectionElements = map[string]struct{}{
	"ChildTable":        {},
	"Children":          {},
	"Column":            {},
	"Columns":           {},
	"DBMS":              {},
	"DefaultDiagram":    {},
	"DefaultGroups":     {},
	"DestinationSymbol": {},
	"IndexColumns":      {},
	"Indexes":           {},
	"Joins":             {},
	"Key.Columns":       {},
	"Keys":              {},
	"LinkedObject":      {},
	"Object":            {},
	"Object1":           {},
	"Object2":           {},
	"Packages":          {},
	"ParentKey":         {},
	"ParentTable":       {},
	"PhysicalDiagrams":  {},
	"PrimaryKey":        {},
	"References":        {},
	"SessionShortcuts":  {},
	"SourceSymbol":      {},
	"Symbols":           {},
	"Tables":            {},
	"TargetModels":      {},
}

var knownProperties = strings.Fields(`
	ArrowStyle AutoAdjustToText BrushStyle Cardinality Code Column.Mandatory Comment
	ControlPoints CornerStyle CreationDate Creator DataType DefaultValue DeleteConstraint
	DisplayPreferences FillColor FontColor FontList FontName GradientEndColor GradientFillMode
	History IconMode Identity Length LineColor ListOfControlPoints ListOfPoints Mandatory
	ManuallyResized ModelOptionsText ModificationDate Modifier Name ObjectID PackageOptionsText
	PageMargins PageOrientation PaperSize PaperSource Precision Rect Scale ShadowColor SortOrder
	SubObjects SymbolContent TargetClassID TargetID TargetModelClassID TargetModelID
	TargetModelLastModificationDate TargetModelURL TargetStereotype Text TextStyle
	TotalSavingCurrency Unique UpdateConstraint
`)

func isXMLSpace(data []byte) bool {
	for _, value := range data {
		if !isXMLSpaceByte(value) {
			return false
		}
	}
	return true
}

func isXMLSpaceByte(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r'
}

func malformedError(line int) error {
	if line < 1 {
		line = 1
	}
	return fmt.Errorf("%w at line %d", ErrMalformedXML, line)
}

func tooLargeError(maxBytes int64) error {
	return fmt.Errorf("%w: max %d bytes", ErrDocumentTooLarge, maxBytes)
}
