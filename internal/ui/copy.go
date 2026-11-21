package appui

import (
	"strconv"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"
)

const (
	CopyTableKey       = "pdmview.workspace.copy-table"
	CopyCreateTableKey = "pdmview.workspace.copy-create-table"
	CopyStatusKey      = "pdmview.workspace.copy-status"
)

type copyFeedback uint8

const (
	copyFeedbackNone copyFeedback = iota
	copyFeedbackSucceeded
	copyFeedbackFailed
)

func (w *Workspace) SetClipboardWriter(writer func(string) error) {
	w.clipboardWriter = writer
}

func (w *Workspace) copySelectedTable() {
	if w.selectedTableID == "" || !w.hasTableDetail ||
		w.tableDetail.ID != w.selectedTableID {
		return
	}
	w.copySelectedTableText(tableCopyText(w.tableDetail))
}

func (w *Workspace) copySelectedCreateTable() {
	if w.selectedTableID == "" || !w.hasTableDetail ||
		w.tableDetail.ID != w.selectedTableID || w.detailIndex == nil {
		w.setSelectedCopyFailure("")
		return
	}
	table, ok := w.detailIndex.tables[w.selectedTableID]
	if !ok {
		w.setSelectedCopyFailure("")
		return
	}
	result := sqlgen.GenerateTable(*table)
	if result.HasErrors() || result.SQL == "" {
		code := ""
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == sqlgen.SeverityError {
				code = diagnostic.Code
				break
			}
		}
		w.setSelectedCopyFailure(code)
		return
	}
	w.copySelectedTableText(result.SQL)
}

func (w *Workspace) copySelectedTableText(text string) {
	tableID := w.selectedTableID
	w.copyToken++
	token := w.copyToken
	writer := w.clipboardWriter
	if writer == nil || writer(text) != nil {
		if token == w.copyToken && w.selectedTableID == tableID {
			w.copyFeedback = copyFeedbackFailed
			w.copyFeedbackTable = tableID
			w.copyDiagnosticCode = ""
			w.notifyChanged()
		}
		return
	}
	if token != w.copyToken || w.selectedTableID != tableID {
		return
	}
	w.copyFeedback = copyFeedbackSucceeded
	w.copyFeedbackTable = tableID
	w.copyDiagnosticCode = ""
	w.notifyChanged()
}

func (w *Workspace) setSelectedCopyFailure(diagnosticCode string) {
	tableID := w.selectedTableID
	w.copyToken++
	w.copyFeedback = copyFeedbackFailed
	w.copyFeedbackTable = tableID
	w.copyDiagnosticCode = diagnosticCode
	w.notifyChanged()
}

func (w *Workspace) copyFeedbackText(state *app.State) (string, bool) {
	if w.copyFeedbackTable != w.selectedTableID {
		return "", false
	}
	switch w.copyFeedback {
	case copyFeedbackSucceeded:
		return state.Text(i18n.KeyCopySucceeded, nil), false
	case copyFeedbackFailed:
		if w.copyDiagnosticCode != "" {
			return state.Text(i18n.KeyCopyCreateTableFailed, map[string]string{
				"category": state.Text(previewDiagnosticCategory(w.copyDiagnosticCode), nil),
				"code":     w.copyDiagnosticCode,
			}), true
		}
		return state.Text(i18n.KeyCopyFailed, nil), true
	default:
		return "", false
	}
}

func (w *Workspace) clearCopyFeedback() {
	w.copyToken++
	w.copyFeedback = copyFeedbackNone
	w.copyFeedbackTable = ""
	w.copyDiagnosticCode = ""
}

func (w *Workspace) clearCopyFeedbackUnless(tableID string) {
	if w.copyFeedbackTable != tableID {
		w.clearCopyFeedback()
	}
}

func tableCopyText(detail TableDetail) string {
	var text strings.Builder
	text.WriteString("PDM Table\n")
	writeCopyMetadata(&text, "", detail.Metadata)
	writeCopyRefList(&text, "", "package_path", detail.PackagePath)
	writeCopyColumns(&text, detail.Columns)
	writeCopyPrimaryKey(&text, detail)
	writeCopyKeys(&text, detail.Keys)
	writeCopyIndexes(&text, detail.Indexes)
	writeCopyForeignKeys(&text, detail.ForeignKeys)
	return text.String()
}

func writeCopyMetadata(text *strings.Builder, indent string, metadata pdm.Metadata) {
	prefix := indent
	text.WriteString(prefix + "id: " + strconv.Quote(metadata.ID) + "\n")
	text.WriteString(prefix + "object_id: " + strconv.Quote(metadata.ObjectID) + "\n")
	text.WriteString(prefix + "name: " + strconv.Quote(metadata.Name) + "\n")
	text.WriteString(prefix + "code: " + strconv.Quote(metadata.Code) + "\n")
	text.WriteString(prefix + "comment: " + copyOptionalText(metadata.Comment) + "\n")
}

func writeCopyColumns(text *strings.Builder, columns []ColumnDetail) {
	if !beginCopyList(text, "", "columns", len(columns)) {
		return
	}
	for offset, column := range columns {
		text.WriteString("  - position: " + strconv.Itoa(offset+1) + "\n")
		writeCopyMetadata(text, "    ", column.Metadata)
		text.WriteString("    type: " + copyOptionalText(column.DataType) + "\n")
		text.WriteString("    length: " + copyOptionalText(column.Length) + "\n")
		text.WriteString("    precision: " + copyOptionalText(column.Precision) + "\n")
		text.WriteString("    scale: " + copyOptionalText(column.Scale) + "\n")
		text.WriteString("    nullable: " + copyOptionalBool(column.Nullable) + "\n")
		text.WriteString("    default: " + copyOptionalText(column.DefaultValue) + "\n")
		text.WriteString("    identity: " + copyOptionalBool(column.Identity) + "\n")
		text.WriteString("    pk: " + strconv.FormatBool(column.PrimaryKey) + "\n")
		text.WriteString("    fk: " + strconv.FormatBool(column.ForeignKey) + "\n")
	}
}

func writeCopyPrimaryKey(text *strings.Builder, detail TableDetail) {
	if detail.PrimaryKeyID == "" {
		text.WriteString("primary_key: absent\n")
		return
	}
	primary := DetailRef{ID: detail.PrimaryKeyID}
	for _, key := range detail.Keys {
		if key.ID == detail.PrimaryKeyID {
			primary = resolvedDetailRef(key.Metadata)
			break
		}
	}
	text.WriteString("primary_key: " + copyRefText(primary) + "\n")
}

func writeCopyKeys(text *strings.Builder, keys []KeyDetail) {
	if !beginCopyList(text, "", "keys", len(keys)) {
		return
	}
	for offset, key := range keys {
		text.WriteString("  - position: " + strconv.Itoa(offset+1) + "\n")
		text.WriteString("    primary: " + strconv.FormatBool(key.Primary) + "\n")
		writeCopyMetadata(text, "    ", key.Metadata)
		writeCopyRefList(text, "    ", "columns", key.Columns)
	}
}

func writeCopyIndexes(text *strings.Builder, indexes []IndexDetail) {
	if !beginCopyList(text, "", "indexes", len(indexes)) {
		return
	}
	for offset, index := range indexes {
		text.WriteString("  - position: " + strconv.Itoa(offset+1) + "\n")
		writeCopyMetadata(text, "    ", index.Metadata)
		text.WriteString("    unique: " + copyOptionalBool(index.Unique) + "\n")
		if !beginCopyList(text, "    ", "columns", len(index.Columns)) {
			continue
		}
		for columnOffset, column := range index.Columns {
			text.WriteString("      - position: " + strconv.Itoa(columnOffset+1) + "\n")
			writeCopyMetadata(text, "        ", column.Metadata)
			text.WriteString("        column: " + copyRefText(column.Column) + "\n")
			text.WriteString("        sort: " + copyOptionalText(column.SortOrder) + "\n")
		}
	}
}

func writeCopyForeignKeys(text *strings.Builder, foreignKeys []ForeignKeyDetail) {
	if !beginCopyList(text, "", "foreign_keys", len(foreignKeys)) {
		return
	}
	for offset, foreignKey := range foreignKeys {
		text.WriteString("  - position: " + strconv.Itoa(offset+1) + "\n")
		writeCopyMetadata(text, "    ", foreignKey.Metadata)
		text.WriteString("    parent_table: " + copyRefText(foreignKey.ParentTable) + "\n")
		text.WriteString("    child_table: " + copyRefText(foreignKey.ChildTable) + "\n")
		text.WriteString("    parent_key: " + copyRefText(foreignKey.ParentKey) + "\n")
		text.WriteString("    update_rule: " + copyOptionalText(foreignKey.UpdateConstraint) + "\n")
		text.WriteString("    delete_rule: " + copyOptionalText(foreignKey.DeleteConstraint) + "\n")
		text.WriteString("    cardinality: " + copyOptionalText(foreignKey.Cardinality) + "\n")
		if !beginCopyList(text, "    ", "joins", len(foreignKey.Joins)) {
			continue
		}
		for joinOffset, join := range foreignKey.Joins {
			text.WriteString("      - position: " + strconv.Itoa(joinOffset+1) + "\n")
			writeCopyMetadata(text, "        ", join.Metadata)
			text.WriteString("        child_column: " + copyRefText(join.ChildColumn) + "\n")
			text.WriteString("        parent_column: " + copyRefText(join.ParentColumn) + "\n")
		}
	}
}

func writeCopyRefList(text *strings.Builder, indent, label string, refs []DetailRef) {
	if !beginCopyList(text, indent, label, len(refs)) {
		return
	}
	for _, ref := range refs {
		text.WriteString(indent + "  - " + copyRefText(ref) + "\n")
	}
}

func beginCopyList(text *strings.Builder, indent, label string, count int) bool {
	text.WriteString(indent + label + ":")
	if count == 0 {
		text.WriteString(" []\n")
		return false
	}
	text.WriteString("\n")
	return true
}

func copyRefText(ref DetailRef) string {
	return "id=" + strconv.Quote(ref.ID) +
		", name=" + strconv.Quote(ref.Name) +
		", code=" + strconv.Quote(ref.Code) +
		", resolved=" + strconv.FormatBool(ref.Resolved)
}

func copyOptionalText(value pdm.OptionalText) string {
	if !value.Present {
		return "absent"
	}
	return strconv.Quote(value.Value)
}

func copyOptionalBool(value *bool) string {
	if value == nil {
		return "absent"
	}
	return strconv.FormatBool(*value)
}
