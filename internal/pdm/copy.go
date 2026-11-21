package pdm

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrInvalidTextEdit = errors.New("pdm: cannot apply table text edit")

type TextProperty string

const (
	TextName    TextProperty = "Name"
	TextCode    TextProperty = "Code"
	TextComment TextProperty = "Comment"
)

type TextEdit struct {
	ObjectID string
	Property TextProperty
	Value    string
}

type textSplice struct {
	start int
	end   int
	value []byte
}

func PatchTableText(source []byte, tableID string, edits []TextEdit) ([]byte, error) {
	if len(edits) == 0 {
		return source, nil
	}
	if tableID == "" {
		return nil, ErrInvalidTextEdit
	}
	spans := make(map[*Element]sourceSpan)
	raw, err := decodeWithSpans(bytes.NewReader(source), MaxDocumentSize, MaxDepth, spans)
	if err != nil {
		return nil, err
	}
	for _, diagnostic := range raw.Diagnostics {
		if diagnostic.Code == DiagnosticDuplicateDefinition {
			return nil, ErrInvalidTextEdit
		}
	}
	table, siblings := findCopyTable(Project(raw).Model.Contents, tableID)
	if table == nil {
		return nil, ErrInvalidTextEdit
	}
	objects := map[string]*Element{table.ID: table.Raw}
	for _, column := range table.Columns {
		objects[column.ID] = column.Raw
	}

	var splices []textSplice
	codes := make(map[string]string)
	type target struct {
		id       string
		property TextProperty
	}
	seen := make(map[target]bool, len(edits))
	for _, edit := range edits {
		key := target{id: edit.ObjectID, property: edit.Property}
		object := objects[edit.ObjectID]
		if object == nil || seen[key] || !validTextEdit(edit) {
			return nil, ErrInvalidTextEdit
		}
		seen[key] = true
		property, err := copyTextProperty(object, edit.Property)
		if err != nil {
			return nil, err
		}
		if (property == nil && edit.Value == "") || (property != nil && property.Text == edit.Value) {
			continue
		}
		if edit.Property == TextCode {
			codes[edit.ObjectID] = edit.Value
		}
		splice, err := makeTextSplice(source, spans, object, property, edit)
		if err != nil {
			return nil, err
		}
		splices = append(splices, splice)
	}
	if err := validateCopyCodes(table, siblings, codes); err != nil {
		return nil, err
	}
	return spliceSource(source, splices)
}

func findCopyTable(contents Contents, tableID string) (*Table, []Table) {
	for index := range contents.Tables {
		if contents.Tables[index].ID == tableID {
			return &contents.Tables[index], contents.Tables
		}
	}
	for _, pkg := range contents.Packages {
		if table, siblings := findCopyTable(pkg.Contents, tableID); table != nil {
			return table, siblings
		}
	}
	return nil, nil
}

func validTextEdit(edit TextEdit) bool {
	switch edit.Property {
	case TextName, TextCode:
		if strings.TrimSpace(edit.Value) == "" {
			return false
		}
	case TextComment:
	default:
		return false
	}
	if edit.ObjectID == "" || !utf8.ValidString(edit.Value) {
		return false
	}
	for _, char := range edit.Value {
		if !(char == '\t' || char == '\n' || char == '\r' ||
			char >= 0x20 && char <= 0xD7FF || char >= 0xE000 && char <= 0xFFFD ||
			char >= 0x10000 && char <= 0x10FFFF) {
			return false
		}
	}
	return true
}

func copyTextProperty(object *Element, property TextProperty) (*Element, error) {
	var match *Element
	for _, child := range object.Children {
		if child.Name.Space == NamespaceAttribute && child.Name.Local == string(property) {
			if match != nil {
				return nil, ErrInvalidTextEdit
			}
			match = child
		}
	}
	return match, nil
}

func validateCopyCodes(table *Table, siblings []Table, codes map[string]string) error {
	equalCode := func(left, right string) bool {
		return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	if code, changed := codes[table.ID]; changed {
		for _, sibling := range siblings {
			if sibling.ID != table.ID && equalCode(code, sibling.Code) {
				return ErrInvalidTextEdit
			}
		}
	}
	columnCodes := make(map[string]string, len(table.Columns))
	for _, column := range table.Columns {
		code, changed := codes[column.ID]
		if !changed {
			code = column.Code
		}
		key := strings.ToLower(strings.TrimSpace(code))
		if otherID, exists := columnCodes[key]; exists {
			_, otherChanged := codes[otherID]
			if changed || otherChanged {
				return fmt.Errorf("%w: duplicate column code", ErrInvalidTextEdit)
			}
		} else {
			columnCodes[key] = column.ID
		}
	}
	return nil
}

func makeTextSplice(source []byte, spans map[*Element]sourceSpan, object, property *Element, edit TextEdit) (textSplice, error) {
	var text bytes.Buffer
	if err := xml.EscapeText(&text, []byte(edit.Value)); err != nil {
		return textSplice{}, err
	}
	if property != nil {
		span := spans[property]
		if span.hasMarkup || len(property.Children) != 0 {
			return textSplice{}, ErrInvalidTextEdit
		}
		if source[span.bodyStart-2] == '/' {
			name := source[span.start+1 : span.bodyStart-2]
			if boundary := bytes.IndexAny(name, " \t\r\n"); boundary >= 0 {
				name = name[:boundary]
			}
			return textSplice{span.bodyStart - 2, span.bodyStart, fmt.Appendf(nil, ">%s</%s>", text.Bytes(), name)}, nil
		}
		return textSplice{span.bodyStart, span.bodyEnd, text.Bytes()}, nil
	}

	span := spans[object]
	name, declaration := copyPropertyQName(spans, object, edit.Property)
	tag := fmt.Appendf(nil, "<%s%s>%s</%s>", name, declaration, text.Bytes(), name)
	if source[span.bodyStart-2] == '/' {
		return textSplice{}, ErrInvalidTextEdit
	}
	offset := span.bodyEnd
	for _, child := range object.Children {
		if child.Name.Space == NamespaceCollection {
			offset = spans[child].start
			break
		}
	}
	lineStart := bytes.LastIndexByte(source[:offset], '\n')
	if lineStart >= 0 && len(bytes.Trim(source[lineStart+1:offset], " \t")) == 0 {
		if lineStart > 0 && source[lineStart-1] == '\r' {
			lineStart--
		}
		tag = append(tag, source[lineStart:offset]...)
	}
	return textSplice{offset, offset, tag}, nil
}

func copyPropertyQName(spans map[*Element]sourceSpan, object *Element, property TextProperty) (string, string) {
	shadowed := make(map[string]bool)
	for node := object; node != nil; node = spans[node].parent {
		for _, attr := range node.Attr {
			if attr.Name.Space != "xmlns" || shadowed[attr.Name.Local] {
				continue
			}
			if attr.Value == NamespaceAttribute {
				return attr.Name.Local + ":" + string(property), ""
			}
			shadowed[attr.Name.Local] = true
		}
	}
	return "a:" + string(property), ` xmlns:a="attribute"`
}

func spliceSource(source []byte, splices []textSplice) ([]byte, error) {
	if len(splices) == 0 {
		return source, nil
	}
	sort.SliceStable(splices, func(left, right int) bool {
		return splices[left].start < splices[right].start
	})
	size := int64(len(source))
	for _, splice := range splices {
		size += int64(len(splice.value) - (splice.end - splice.start))
	}
	if size > MaxDocumentSize {
		return nil, ErrDocumentTooLarge
	}
	result := make([]byte, 0, int(size))
	offset := 0
	for _, splice := range splices {
		if splice.start < offset || splice.start > splice.end || splice.end > len(source) {
			return nil, ErrInvalidTextEdit
		}
		result = append(result, source[offset:splice.start]...)
		result = append(result, splice.value...)
		offset = splice.end
	}
	return append(result, source[offset:]...), nil
}
