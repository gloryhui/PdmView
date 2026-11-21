package appui

import (
	"strconv"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	TablePropertiesRootKey        = "pdmview.table-properties.root"
	TablePropertiesContentKey     = "pdmview.table-properties.content"
	TablePropertiesTitleKey       = "pdmview.table-properties.title"
	TablePropertiesTabsKey        = "pdmview.table-properties.tabs"
	TablePropertiesGeneralKey     = "pdmview.table-properties.general"
	TablePropertiesGeneralScroll  = "pdmview.table-properties.general.scroll"
	TablePropertiesColumnsKey     = "pdmview.table-properties.columns"
	TablePropertiesIndexesKey     = "pdmview.table-properties.indexes"
	TablePropertiesKeysKey        = "pdmview.table-properties.keys"
	TablePropertiesPreviewKey     = "pdmview.table-properties.preview"
	TablePropertiesSQLKey         = "pdmview.table-properties.preview.sql"
	TablePropertiesCopyKey        = "pdmview.table-properties.preview.copy"
	TablePropertiesCopyStatusKey  = "pdmview.table-properties.preview.status"
	TablePropertiesDiagnosticsKey = "pdmview.table-properties.preview.diagnostics"
	TablePropertiesCloseKey       = "pdmview.table-properties.close"
)

func TablePropertiesTabKey(index int) string {
	return "pdmview.table-properties.tab." + strconv.Itoa(index)
}

func TablePropertiesColumnKey(id string) string {
	return "pdmview.table-properties.column." + id
}

func TablePropertiesIndexKey(id string) string {
	return "pdmview.table-properties.index." + id
}

func TablePropertiesIndexMemberKey(id string, ordinal int) string {
	return TablePropertiesIndexKey(id) + ".member." + strconv.Itoa(ordinal)
}

func TablePropertiesKeyKey(id string) string {
	return "pdmview.table-properties.key." + id
}

func TablePropertiesKeyMemberKey(id string, ordinal int) string {
	return TablePropertiesKeyKey(id) + ".member." + strconv.Itoa(ordinal)
}

type tablePropertiesState struct {
	open              bool
	tableID           string
	detail            TableDetail
	hasDetail         bool
	tab               int
	generalScroll     ui.ScrollState
	columnsList       ui.ListState
	indexesList       ui.ListState
	keysList          ui.ListState
	previewResult     sqlgen.Result
	previewTableID    string
	previewLoaded     bool
	previewEditor     *editor.State
	diagnosticsScroll ui.ScrollState
	copyFeedback      copyFeedback
	copyToken         uint64
}

func (w *Workspace) requestTableProperties(tableID string) {
	if w.tableOpen != nil {
		w.tableOpen(tableID)
		return
	}
	w.OpenTableProperties(tableID)
}

func (w *Workspace) OpenTableProperties(tableID string) bool {
	if tableID == "" || w.detailIndex == nil {
		return false
	}
	if w.tableProperties.open && w.tableProperties.tableID == tableID {
		return true
	}
	detail, ok := w.detailIndex.resolve(tableID)
	if !ok {
		return false
	}
	if w.tableProperties.tableID != tableID {
		w.tableProperties = tablePropertiesState{tableID: tableID}
	}
	w.tableProperties.detail = detail
	w.tableProperties.hasDetail = true
	w.tableProperties.open = true
	w.notifyChanged()
	return true
}

func (w *Workspace) resetTableProperties() {
	wasOpen := w.tableProperties.open
	w.tableProperties = tablePropertiesState{}
	if wasOpen && w.tableClose != nil {
		w.tableClose()
	}
}

func (w *Workspace) CloseTableProperties() {
	if !w.tableProperties.open {
		return
	}
	w.tableProperties = tablePropertiesState{}
	if w.tableClose != nil {
		w.tableClose()
	}
	w.notifyChanged()
}

func (w *Workspace) TablePropertiesOpen() bool {
	return w.tableProperties.open
}

func (w *Workspace) TablePropertiesTitle(state *app.State) string {
	title := state.Text(i18n.KeyTablePropertiesTitle, nil)
	detail := w.tableProperties.detail
	name := firstNonEmpty(detail.Name, detail.Code, detail.ID)
	if name == "" || name == title {
		return title
	}
	return title + " — " + name
}

func (w *Workspace) TablePropertiesView(state *app.State) func(*ui.Context) {
	return func(c *ui.Context) {
		if !w.tableProperties.open || !w.tableProperties.hasDetail {
			return
		}
		if c.Shortcut(0, ui.KeyEscape) {
			w.CloseTableProperties()
			return
		}
		t := c.Theme()
		w.renderTableProperties(c, state, t, string(state.Locale()), w.tableProperties.detail)
	}
}

func (w *Workspace) renderTableProperties(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	title := firstNonEmpty(detail.Name, detail.Code, detail.ID, state.Text(i18n.KeyTablePropertiesTitle, nil))
	ui.Column(c.Key(TablePropertiesRootKey)).Fill().Children(func() {
		ui.Column(c.Key(TablePropertiesContentKey)).Fill().Gap(8).Children(func() {
			ui.Row(c).FillWidth().Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, title).Key(TablePropertiesTitleKey).Grow(1).FontSize(16).Bold().
					TextColor(theme.Text).Lang(lang).Role(ui.RoleHeading).Level(2)
				closeLabel := state.Text(i18n.KeyTablePropertiesClose, nil)
				ui.Button(c, closeLabel).Key(TablePropertiesCloseKey).Label(closeLabel).
					Lang(lang).OnClick(w.CloseTableProperties)
			})
			labels := []string{
				state.Text(i18n.KeyTablePropertiesGeneral, nil),
				state.Text(i18n.KeyDetailColumns, nil),
				state.Text(i18n.KeyDetailIndexes, nil),
				state.Text(i18n.KeyDetailKeys, nil),
				state.Text(i18n.KeyTablePropertiesPreview, nil),
			}
			tabs := ui.TabsBase(c.Key(TablePropertiesTabsKey), &w.tableProperties.tab, len(labels))
			tabs.List.Gap(4).Children(func() {
				for index, label := range labels {
					c.Key(TablePropertiesTabKey(index))
					tab := tabs.Tab(index).Padding(4, 8).Radius(4)
					if index == w.tableProperties.tab {
						tab.TextColor(theme.Text).Bold()
					} else {
						tab.TextColor(theme.TextMuted)
					}
					tab.Children(func() {
						ui.Text(c, label).SingleLine().Lang(lang)
					})
				}
			})
			ui.Column(c).Grow(1).FillWidth().Children(func() {
				switch w.tableProperties.tab {
				case 0:
					w.tablePropertiesGeneral(c, state, theme, lang, detail)
				case 1:
					w.tablePropertiesColumns(c, state, theme, lang, detail)
				case 2:
					w.tablePropertiesIndexes(c, state, theme, lang, detail)
				case 3:
					w.tablePropertiesKeys(c, state, theme, lang, detail)
				case 4:
					w.tablePropertiesPreview(c, state, theme, lang, detail)
				}
			})
		})
	})
}

func (w *Workspace) tablePropertiesGeneral(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	ui.Scroll(c.Key(TablePropertiesGeneralScroll)).TrackScroll(&w.tableProperties.generalScroll).
		Grow(1).FillWidth().Children(func() {
		ui.Column(c.Key(TablePropertiesGeneralKey)).FillWidth().Gap(8).Padding(4).Children(func() {
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesGeneralKey+".name",
				state.Text(i18n.KeyTablePropertiesName, nil), valueOrDash(detail.Name))
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesGeneralKey+".code",
				state.Text(i18n.KeyDetailCode, nil), valueOrDash(detail.Code))
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesGeneralKey+".comment",
				state.Text(i18n.KeyDetailComment, nil), optionalValue(detail.Comment))
			if path := packagePathText(detail.PackagePath); path != "" {
				tablePropertiesProperty(c, state, theme, lang, TablePropertiesGeneralKey+".package",
					state.Text(i18n.KeyDetailPackage, nil), path)
			}
		})
	})
}

func (w *Workspace) tablePropertiesColumns(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	const (
		nameWidth      = float32(140)
		codeWidth      = float32(100)
		commentWidth   = float32(150)
		typeWidth      = float32(100)
		lengthWidth    = float32(50)
		precisionWidth = float32(60)
		flagsWidth     = float32(80)
	)
	header := func(label string, width float32) {
		ui.Text(c, label).Width(width).Shrink(0).SingleLine().FontSize(10).
			TextColor(theme.TextMuted).Lang(lang)
	}
	ui.Row(c).FillWidth().Height(24).Padding(1, 4).Gap(4).Background(theme.Surface).Children(func() {
		header(state.Text(i18n.KeyTablePropertiesName, nil), nameWidth)
		header(state.Text(i18n.KeyDetailCode, nil), codeWidth)
		header(state.Text(i18n.KeyDetailComment, nil), commentWidth)
		header(state.Text(i18n.KeyDetailType, nil), typeWidth)
		header(state.Text(i18n.KeyDetailLength, nil), lengthWidth)
		header(state.Text(i18n.KeyDetailPrecision, nil), precisionWidth)
		header(state.Text(i18n.KeyTablePropertiesFlags, nil), flagsWidth)
	})
	w.tableProperties.columnsList.Key = func(row int) any { return detail.Columns[row].ID }
	ui.List(c.Key(TablePropertiesColumnsKey), &w.tableProperties.columnsList, len(detail.Columns), func(row int) {
		column := detail.Columns[row]
		c.Key(TablePropertiesColumnKey(column.ID))
		ui.Row(c).FillWidth().Height(26).Padding(1, 4).Gap(4).Children(func() {
			tablePropertiesCell(c, valueOrDash(column.Name), nameWidth, lang)
			tablePropertiesCell(c, valueOrDash(column.Code), codeWidth, lang)
			tablePropertiesCell(c, optionalValue(column.Comment), commentWidth, lang)
			tablePropertiesCell(c, optionalValue(column.DataType), typeWidth, lang)
			tablePropertiesCell(c, optionalValue(column.Length), lengthWidth, lang)
			tablePropertiesCell(c, optionalValue(column.Precision), precisionWidth, lang)
			tablePropertiesCell(c, tablePropertiesFlags(state, column), flagsWidth, lang)
		})
	}).Grow(1).FillWidth()
}

func (w *Workspace) tablePropertiesIndexes(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	w.tableProperties.indexesList.Key = func(row int) any { return detail.Indexes[row].ID }
	ui.List(c.Key(TablePropertiesIndexesKey), &w.tableProperties.indexesList, len(detail.Indexes), func(row int) {
		index := detail.Indexes[row]
		id := detailID(index.ID, row)
		c.Key(TablePropertiesIndexKey(id))
		ui.Column(c).FillWidth().Padding(4, 6).Gap(2).Children(func() {
			ui.Text(c, firstNonEmpty(index.Name, index.Code, index.ID, "—")).
				FontSize(12).Bold().TextColor(theme.Text).Lang(lang)
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesIndexKey(id)+".code",
				state.Text(i18n.KeyDetailCode, nil), valueOrDash(index.Code))
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesIndexKey(id)+".unique",
				state.Text(i18n.KeyDetailUnique, nil), boolValue(state, index.Unique))
			for member, column := range index.Columns {
				value := detailRefText(state, column.Column)
				if column.SortOrder.Present {
					value += " · " + state.Text(i18n.KeyDetailSort, nil) + ": " + optionalValue(column.SortOrder)
				}
				tablePropertiesProperty(c, state, theme, lang,
					TablePropertiesIndexMemberKey(id, member), strconv.Itoa(member+1), value)
			}
		})
	}).Grow(1).FillWidth()
}

func (w *Workspace) tablePropertiesKeys(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	w.tableProperties.keysList.Key = func(row int) any { return detail.Keys[row].ID }
	ui.List(c.Key(TablePropertiesKeysKey), &w.tableProperties.keysList, len(detail.Keys), func(row int) {
		key := detail.Keys[row]
		id := detailID(key.ID, row)
		c.Key(TablePropertiesKeyKey(id))
		ui.Column(c).FillWidth().Padding(4, 6).Gap(2).Children(func() {
			ui.Text(c, firstNonEmpty(key.Name, key.Code, key.ID, "—")).
				FontSize(12).Bold().TextColor(theme.Text).Lang(lang)
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesKeyKey(id)+".code",
				state.Text(i18n.KeyDetailCode, nil), valueOrDash(key.Code))
			tablePropertiesProperty(c, state, theme, lang, TablePropertiesKeyKey(id)+".primary",
				state.Text(i18n.KeyDetailPrimary, nil), boolValue(state, &key.Primary))
			for member, ref := range key.Columns {
				tablePropertiesProperty(c, state, theme, lang,
					TablePropertiesKeyMemberKey(id, member), strconv.Itoa(member+1), detailRefText(state, ref))
			}
		})
	}).Grow(1).FillWidth()
}

func (w *Workspace) tablePropertiesPreview(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	detail TableDetail,
) {
	w.ensureTablePropertiesPreview(detail)
	result := w.tableProperties.previewResult
	ui.Column(c.Key(TablePropertiesPreviewKey)).FillWidth().Grow(1).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			label := state.Text(i18n.KeyPreviewCopySQL, nil)
			ui.Button(c, label).Key(TablePropertiesCopyKey).Label(label).Lang(lang).
				Disabled(result.HasErrors() || result.SQL == "").
				OnClick(w.copyTablePropertiesSQL)
			if status := w.tablePropertiesCopyStatus(state); status != "" {
				ui.Text(c, status).Key(TablePropertiesCopyStatusKey).Grow(1).
					FontSize(11).TextColor(theme.TextMuted).Lang(lang).Role(ui.RoleStatus)
			}
		})
		w.tablePropertiesDiagnostics(c, state, theme, lang, result)
		if w.tableProperties.previewEditor != nil {
			w.tableProperties.previewEditor.Label = state.Text(i18n.KeyPreviewSQL, nil)
			editor.View(c.Key(TablePropertiesSQLKey), w.tableProperties.previewEditor).
				Grow(1).FillWidth()
		}
	})
}

func (w *Workspace) ensureTablePropertiesPreview(detail TableDetail) {
	if w.tableProperties.previewLoaded && w.tableProperties.previewTableID == detail.ID {
		return
	}
	w.tableProperties.previewLoaded = true
	w.tableProperties.previewTableID = detail.ID
	w.tableProperties.previewResult = sqlgen.Result{}
	w.tableProperties.previewEditor = nil
	if w.detailIndex == nil {
		return
	}
	table, ok := w.detailIndex.tables[detail.ID]
	if !ok {
		return
	}
	w.tableProperties.previewResult = sqlgen.GenerateTable(*table)
	if w.tableProperties.previewResult.HasErrors() || w.tableProperties.previewResult.SQL == "" {
		return
	}
	buffer := editor.NewBuffer(w.tableProperties.previewResult.SQL)
	buffer.SetLanguage(editor.LanguageFor("database.sql"))
	w.tableProperties.previewEditor = &editor.State{
		Buffer:          buffer,
		ReadOnly:        true,
		SoftWrap:        false,
		KeepScrollbars:  true,
		NoScrollPastEnd: true,
	}
}

func (w *Workspace) copyTablePropertiesSQL() {
	if w.tableProperties.previewEditor == nil ||
		w.tableProperties.previewResult.HasErrors() ||
		w.tableProperties.previewResult.SQL == "" {
		return
	}
	text := w.tableProperties.previewResult.SQL
	tableID := w.tableProperties.tableID
	w.tableProperties.copyToken++
	token := w.tableProperties.copyToken
	writer := w.clipboardWriter
	if writer == nil || writer(text) != nil {
		if token == w.tableProperties.copyToken && tableID == w.tableProperties.tableID {
			w.tableProperties.copyFeedback = copyFeedbackFailed
			w.notifyChanged()
		}
		return
	}
	if token != w.tableProperties.copyToken || tableID != w.tableProperties.tableID {
		return
	}
	w.tableProperties.copyFeedback = copyFeedbackSucceeded
	w.notifyChanged()
}

func (w *Workspace) tablePropertiesCopyStatus(state *app.State) string {
	switch w.tableProperties.copyFeedback {
	case copyFeedbackSucceeded:
		return state.Text(i18n.KeyPreviewSQLCopied, nil)
	case copyFeedbackFailed:
		return state.Text(i18n.KeyPreviewSQLCopyFailed, nil)
	default:
		return ""
	}
}

func (w *Workspace) tablePropertiesDiagnostics(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	result sqlgen.Result,
) {
	if len(result.Diagnostics) == 0 {
		return
	}
	ui.Text(c, state.Text(i18n.KeyPreviewDiagnostics, nil)).FontSize(12).Bold().
		TextColor(theme.Text).Lang(lang).Role(ui.RoleHeading).Level(3)
	ui.Scroll(c.Key(TablePropertiesDiagnosticsKey)).
		TrackScroll(&w.tableProperties.diagnosticsScroll).
		FillWidth().Height(90).KeepScrollbars(true).Children(func() {
		ui.Column(c).FillWidth().Gap(3).Children(func() {
			for _, diagnostic := range result.Diagnostics {
				severity := state.Text(i18n.KeyPreviewWarning, nil)
				color := theme.Warning
				role := ui.RoleStatus
				if diagnostic.Severity == sqlgen.SeverityError {
					severity = state.Text(i18n.KeyPreviewError, nil)
					color, role = theme.Danger, ui.RoleAlert
				}
				heading := state.Text(i18n.KeyPreviewDiagnosticHeading, map[string]string{
					"category": state.Text(previewDiagnosticCategory(diagnostic.Code), nil),
					"severity": severity,
				})
				ui.Text(c, heading+" · "+diagnostic.Code).FontSize(11).
					TextColor(color).Lang(lang).Role(role)
			}
		})
	})
}

func tablePropertiesProperty(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
	key string,
	label, value string,
) {
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, label).Width(90).Shrink(0).FontSize(11).
			TextColor(theme.TextMuted).Lang(lang)
		ui.Text(c, value).Key(key).FillWidth().Wrap().
			FontSize(11).TextColor(theme.Text).Lang(lang)
	})
}

func tablePropertiesCell(c *ui.Context, text string, width float32, lang string) {
	cell := ui.Text(c, text).Width(width).Shrink(0).SingleLine().Ellipsis("…").
		FontSize(11).Lang(lang)
	if text != "" && text != "—" {
		cell.Tooltip(text)
	}
}

func tablePropertiesFlags(state *app.State, column ColumnDetail) string {
	var flags []string
	if column.PrimaryKey {
		flags = append(flags, state.Text(i18n.KeyDetailPK, nil))
	}
	if column.ForeignKey {
		flags = append(flags, state.Text(i18n.KeyDetailFK, nil))
	}
	if column.Nullable != nil && !*column.Nullable {
		flags = append(flags, state.Text(i18n.KeyTablePropertiesMandatory, nil))
	}
	if len(flags) == 0 {
		return "—"
	}
	return strings.Join(flags, " ")
}
