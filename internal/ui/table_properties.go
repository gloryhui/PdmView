package appui

import (
	"strconv"

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
	TablePropertiesSaveKey        = "pdmview.table-properties.save"
	tablePropertiesPadding        = 12
)

func TablePropertiesTabKey(index int) string {
	return "pdmview.table-properties.tab." + strconv.Itoa(index)
}

func TablePropertiesColumnKey(id string) string {
	return "pdmview.table-properties.column." + id
}

func TablePropertiesColumnFieldKey(id, field string) string {
	return TablePropertiesColumnKey(id) + "." + field
}

func TablePropertiesIndexKey(id string) string {
	return "pdmview.table-properties.index." + id
}

func TablePropertiesIndexFieldKey(id, field string) string {
	return TablePropertiesIndexKey(id) + "." + field
}

func TablePropertiesIndexMemberKey(id string, ordinal int) string {
	return TablePropertiesIndexKey(id) + ".member." + strconv.Itoa(ordinal)
}

func TablePropertiesIndexMemberFieldKey(id string, ordinal int, field string) string {
	return TablePropertiesIndexMemberKey(id, ordinal) + "." + field
}

func TablePropertiesKeyKey(id string) string {
	return "pdmview.table-properties.key." + id
}

func TablePropertiesKeyFieldKey(id, field string) string {
	return TablePropertiesKeyKey(id) + "." + field
}

func TablePropertiesKeyMemberKey(id string, ordinal int) string {
	return TablePropertiesKeyKey(id) + ".member." + strconv.Itoa(ordinal)
}

func TablePropertiesKeyMemberFieldKey(id string, ordinal int, field string) string {
	return TablePropertiesKeyMemberKey(id, ordinal) + "." + field
}

type tablePropertiesState struct {
	saveStatus        i18n.Key
	open              bool
	tableID           string
	detail            TableDetail
	hasDetail         bool
	draft             tablePropertiesDraft
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
	copiedVersion     uint64
	copyVersion       uint64
}

type tablePropertiesDraft struct {
	General tableGeneralDraft
	Columns []tableColumnDraft
	Indexes []tableIndexDraft
	Keys    []tableKeyDraft
}

type tableGeneralDraft struct {
	Name           string
	Code           string
	Comment        string
	Package        string
	PackagePresent bool
}

type tableColumnDraft struct {
	ID           string
	Name         string
	Code         string
	Comment      string
	DataType     string
	Length       string
	Precision    string
	Scale        string
	DefaultValue string
	PrimaryKey   bool
	ForeignKey   bool
	Mandatory    ui.CheckState
	Identity     ui.CheckState
}

type tableIndexDraft struct {
	ID      string
	Name    string
	Code    string
	Unique  ui.CheckState
	Members []tableMemberDraft
}

type tableKeyDraft struct {
	ID      string
	Name    string
	Code    string
	Primary bool
	Members []tableMemberDraft
}

type tableMemberDraft struct {
	Name string
	Sort string
}

func newTablePropertiesDraft(detail TableDetail) tablePropertiesDraft {
	draft := tablePropertiesDraft{
		General: tableGeneralDraft{
			Name:    detail.Name,
			Code:    detail.Code,
			Comment: detail.Comment.Value,
			Package: packagePathText(detail.PackagePath),
		},
	}
	draft.General.PackagePresent = len(detail.PackagePath) > 0
	draft.Columns = make([]tableColumnDraft, len(detail.Columns))
	for offset, column := range detail.Columns {
		mandatory := ui.CheckMixed
		if column.Nullable != nil {
			if *column.Nullable {
				mandatory = ui.CheckOff
			} else {
				mandatory = ui.CheckOn
			}
		}
		identity := ui.CheckMixed
		if column.Identity != nil {
			identity = ui.CheckOff
			if *column.Identity {
				identity = ui.CheckOn
			}
		}
		draft.Columns[offset] = tableColumnDraft{
			ID:           column.ID,
			Name:         column.Name,
			Code:         column.Code,
			Comment:      column.Comment.Value,
			DataType:     column.DataType.Value,
			Length:       column.Length.Value,
			Precision:    column.Precision.Value,
			Scale:        column.Scale.Value,
			DefaultValue: column.DefaultValue.Value,
			PrimaryKey:   column.PrimaryKey,
			ForeignKey:   column.ForeignKey,
			Mandatory:    mandatory,
			Identity:     identity,
		}
	}
	draft.Indexes = make([]tableIndexDraft, len(detail.Indexes))
	for offset, index := range detail.Indexes {
		unique := ui.CheckMixed
		if index.Unique != nil {
			if *index.Unique {
				unique = ui.CheckOn
			} else {
				unique = ui.CheckOff
			}
		}
		draft.Indexes[offset] = tableIndexDraft{
			ID:     index.ID,
			Name:   index.Name,
			Code:   index.Code,
			Unique: unique,
		}
		for _, member := range index.Columns {
			draft.Indexes[offset].Members = append(draft.Indexes[offset].Members, tableMemberDraft{
				Name: firstNonEmpty(member.Column.Name, member.Column.Code, member.Column.ID),
				Sort: member.SortOrder.Value,
			})
		}
	}
	draft.Keys = make([]tableKeyDraft, len(detail.Keys))
	for offset, key := range detail.Keys {
		draft.Keys[offset] = tableKeyDraft{
			ID:      key.ID,
			Name:    key.Name,
			Code:    key.Code,
			Primary: key.Primary,
		}
		for _, member := range key.Columns {
			draft.Keys[offset].Members = append(draft.Keys[offset].Members, tableMemberDraft{
				Name: firstNonEmpty(member.Name, member.Code, member.ID),
			})
		}
	}
	return draft
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
	if w.pdmSaveRequest != nil {
		return false
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
	w.tableProperties.draft = newTablePropertiesDraft(detail)
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
		ui.Column(c.Key(TablePropertiesContentKey)).Fill().Padding(tablePropertiesPadding).Gap(8).Children(func() {
			ui.Row(c).FillWidth().Height(30).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, title).Key(TablePropertiesTitleKey).Grow(1).FontSize(16).Bold().
					TextColor(theme.Text).Lang(lang).Role(ui.RoleHeading).Level(2)
				if w.pdmSaveStarter != nil && w.sourcePath != "" {
					saveLabel := state.Text(i18n.KeyPDMSave, nil)
					ui.Button(c, saveLabel).Key(TablePropertiesSaveKey).Label(saveLabel).
						Lang(lang).Disabled(w.pdmSaveRequest != nil).
						OnClick(w.savePDM)
				}
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
					w.tablePropertiesGeneral(c, state, theme, lang)
				case 1:
					w.tablePropertiesColumns(c, state, theme, lang)
				case 2:
					w.tablePropertiesIndexes(c, state, theme, lang)
				case 3:
					w.tablePropertiesKeys(c, state, theme, lang)
				case 4:
					w.tablePropertiesPreview(c, state, theme, lang, detail)
				}
			})
			if status := w.tableProperties.saveStatus; status != "" {
				color, role := theme.TextMuted, ui.RoleStatus
				switch status {
				case i18n.KeyPDMInvalidEdit, i18n.KeyPDMSaveFailed, i18n.KeyPDMChanged:
					color, role = theme.Danger, ui.RoleAlert
				}
				ui.Text(c, state.Text(status, nil)).FillWidth().FontSize(12).
					TextColor(color).Lang(lang).Role(role)
			} else if w.pdmSaveStarter != nil && w.sourcePath != "" {
				ui.Text(c, state.Text(i18n.KeyPDMSaveScope, nil)).FillWidth().FontSize(12).
					TextColor(theme.TextMuted).Lang(lang)
			}
		})
	})
}

func (w *Workspace) tablePropertiesGeneral(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	draft := &w.tableProperties.draft.General
	ui.Scroll(c.Key(TablePropertiesGeneralScroll)).TrackScroll(&w.tableProperties.generalScroll).
		Grow(1).FillWidth().Children(func() {
		ui.Column(c.Key(TablePropertiesGeneralKey)).FillWidth().Gap(8).Padding(4).Children(func() {
			tablePropertiesLabeledInput(c, theme, lang,
				TablePropertiesGeneralKey+".name", state.Text(i18n.KeyTablePropertiesName, nil), &draft.Name)
			tablePropertiesLabeledInput(c, theme, lang,
				TablePropertiesGeneralKey+".code", state.Text(i18n.KeyDetailCode, nil), &draft.Code)
			tablePropertiesLabeledArea(c, theme, lang,
				TablePropertiesGeneralKey+".comment", state.Text(i18n.KeyDetailComment, nil), &draft.Comment)
			if draft.PackagePresent {
				tablePropertiesLabeledInput(c, theme, lang,
					TablePropertiesGeneralKey+".package", state.Text(i18n.KeyDetailPackage, nil), &draft.Package)
			}
		})
	})
}

func (w *Workspace) tablePropertiesColumns(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	const (
		minNameWidth    = float32(130)
		minCodeWidth    = float32(110)
		minCommentWidth = float32(160)
		minTypeWidth    = float32(90)
		lengthWidth     = float32(50)
		precisionWidth  = float32(60)
		scaleWidth      = float32(55)
		defaultWidth    = float32(115)
		keyWidth        = float32(45)
		mandatoryWidth  = float32(80)
		identityWidth   = float32(64)
		flagsWidth      = 2*keyWidth + mandatoryWidth + identityWidth
		minWidth        = minNameWidth + minCodeWidth + minCommentWidth + minTypeWidth +
			lengthWidth + precisionWidth + scaleWidth + defaultWidth + flagsWidth + 8*4 + 8
	)
	available, _ := c.Size()
	extra := max(0, available-2*tablePropertiesPadding-12-minWidth)
	nameWidth, codeWidth := minNameWidth+extra*0.25, minCodeWidth+extra*0.25
	commentWidth, typeWidth := minCommentWidth+extra*0.3, minTypeWidth+extra*0.2
	totalWidth := minWidth + extra
	primary := state.Text(i18n.KeyDetailPK, nil)
	foreign := state.Text(i18n.KeyDetailFK, nil)
	mandatory := state.Text(i18n.KeyTablePropertiesMandatory, nil)
	identity := state.Text(i18n.KeyDetailIdentity, nil)
	defaultLabel := state.Text(i18n.KeyDetailDefault, nil)
	header := func(label string, width float32) {
		ui.Text(c, label).Width(width).Shrink(0).SingleLine().FontSize(10).
			TextColor(theme.TextMuted).Lang(lang)
	}
	flagCell := func(width float32, build func()) {
		ui.Row(c).Width(width).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(build)
	}
	head := ui.Row(c).Width(totalWidth).Height(24).Clip().Padding(1, 4).Gap(4).
		Background(theme.Surface).Children(func() {
		header(state.Text(i18n.KeyTablePropertiesName, nil), nameWidth)
		header(state.Text(i18n.KeyDetailCode, nil), codeWidth)
		header(state.Text(i18n.KeyDetailComment, nil), commentWidth)
		header(state.Text(i18n.KeyDetailType, nil), typeWidth)
		header(state.Text(i18n.KeyDetailLength, nil), lengthWidth)
		header(state.Text(i18n.KeyDetailPrecision, nil), precisionWidth)
		header(state.Text(i18n.KeyDetailScale, nil), scaleWidth)
		header(defaultLabel, defaultWidth)
		ui.Row(c).Width(flagsWidth).Shrink(0).Children(func() {
			for _, flag := range []struct {
				label string
				width float32
			}{{primary, keyWidth}, {foreign, keyWidth}, {mandatory, mandatoryWidth}, {identity, identityWidth}} {
				flagCell(flag.width, func() {
					ui.Text(c, flag.label).SingleLine().FontSize(10).TextColor(theme.TextMuted).Lang(lang)
				})
			}
		})
	})
	w.tableProperties.columnsList.Key = func(row int) any { return w.tableProperties.draft.Columns[row].ID }
	list := ui.List(c.Key(TablePropertiesColumnsKey), &w.tableProperties.columnsList,
		len(w.tableProperties.draft.Columns), func(row int) {
			column := &w.tableProperties.draft.Columns[row]
			c.Key(TablePropertiesColumnKey(column.ID))
			ui.Row(c).Width(totalWidth).Shrink(0).Height(32).Padding(1, 4).Gap(4).Children(func() {
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "name"),
					state.Text(i18n.KeyTablePropertiesName, nil), nameWidth, &column.Name, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "code"),
					state.Text(i18n.KeyDetailCode, nil), codeWidth, &column.Code, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "comment"),
					state.Text(i18n.KeyDetailComment, nil), commentWidth, &column.Comment, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "type"),
					state.Text(i18n.KeyDetailType, nil), typeWidth, &column.DataType, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "length"),
					state.Text(i18n.KeyDetailLength, nil), lengthWidth, &column.Length, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "precision"),
					state.Text(i18n.KeyDetailPrecision, nil), precisionWidth, &column.Precision, lang)
				tablePropertiesInput(c, TablePropertiesColumnFieldKey(column.ID, "scale"),
					state.Text(i18n.KeyDetailScale, nil), scaleWidth, &column.Scale, lang)
				placeholder := "—"
				if w.tableProperties.detail.Columns[row].DefaultValue.Present {
					placeholder = `""`
				}
				c.Key(TablePropertiesColumnFieldKey(column.ID, "default"))
				ui.TextInput(c, &column.DefaultValue).Width(defaultWidth).Shrink(0).Label(defaultLabel).
					Placeholder(placeholder).Lang(lang)
				ui.Row(c).Width(flagsWidth).Shrink(0).Children(func() {
					flagCell(keyWidth, func() {
						ui.Checkbox(c.Key(TablePropertiesColumnFieldKey(column.ID, "pk")), &column.PrimaryKey, "").
							Label(primary).Lang(lang)
					})
					flagCell(keyWidth, func() {
						ui.Checkbox(c.Key(TablePropertiesColumnFieldKey(column.ID, "fk")), &column.ForeignKey, "").
							Label(foreign).Lang(lang)
					})
					flagCell(mandatoryWidth, func() {
						ui.CheckboxState(c.Key(TablePropertiesColumnFieldKey(column.ID, "mandatory")), &column.Mandatory, "").
							Label(mandatory).Lang(lang)
					})
					flagCell(identityWidth, func() {
						ui.CheckboxState(c.Key(TablePropertiesColumnFieldKey(column.ID, "identity")), &column.Identity, "").
							Label(identity).Lang(lang)
					})
				})
			})
		}).RowWidth(totalWidth).Grow(1).FillWidth()
	head.ScrollWith(list)
}

func detailID(id string, offset int) string {
	if id != "" {
		return id
	}
	return "index-" + strconv.Itoa(offset)
}

func (w *Workspace) tablePropertiesIndexes(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	w.tableProperties.indexesList.Key = func(row int) any { return w.tableProperties.draft.Indexes[row].ID }
	ui.List(c.Key(TablePropertiesIndexesKey), &w.tableProperties.indexesList,
		len(w.tableProperties.draft.Indexes), func(row int) {
			index := &w.tableProperties.draft.Indexes[row]
			id := detailID(index.ID, row)
			c.Key(TablePropertiesIndexKey(id))
			ui.Column(c).FillWidth().Padding(4, 6).Gap(4).Children(func() {
				tablePropertiesLabeledInput(c, theme, lang,
					TablePropertiesIndexFieldKey(id, "name"),
					state.Text(i18n.KeyTablePropertiesName, nil), &index.Name)
				tablePropertiesLabeledInput(c, theme, lang,
					TablePropertiesIndexFieldKey(id, "code"),
					state.Text(i18n.KeyDetailCode, nil), &index.Code)
				tablePropertiesCheckboxState(c, TablePropertiesIndexFieldKey(id, "unique"),
					state.Text(i18n.KeyDetailUnique, nil), &index.Unique, lang)
				for member := range index.Members {
					entry := &index.Members[member]
					ui.Row(c).FillWidth().Gap(6).Children(func() {
						ui.Text(c, strconv.Itoa(member+1)).Width(24).Shrink(0).
							FontSize(11).TextColor(theme.TextMuted).Lang(lang)
						tablePropertiesInput(c,
							TablePropertiesIndexMemberFieldKey(id, member, "name"),
							state.Text(i18n.KeyDetailColumns, nil), 0, &entry.Name, lang)
						tablePropertiesInput(c,
							TablePropertiesIndexMemberFieldKey(id, member, "sort"),
							state.Text(i18n.KeyDetailSort, nil), 90, &entry.Sort, lang)
					})
				}
			})
		}).Grow(1).FillWidth()
}

func (w *Workspace) tablePropertiesKeys(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	w.tableProperties.keysList.Key = func(row int) any { return w.tableProperties.draft.Keys[row].ID }
	ui.List(c.Key(TablePropertiesKeysKey), &w.tableProperties.keysList,
		len(w.tableProperties.draft.Keys), func(row int) {
			key := &w.tableProperties.draft.Keys[row]
			id := detailID(key.ID, row)
			c.Key(TablePropertiesKeyKey(id))
			ui.Column(c).FillWidth().Padding(4, 6).Gap(4).Children(func() {
				tablePropertiesLabeledInput(c, theme, lang,
					TablePropertiesKeyFieldKey(id, "name"),
					state.Text(i18n.KeyTablePropertiesName, nil), &key.Name)
				tablePropertiesLabeledInput(c, theme, lang,
					TablePropertiesKeyFieldKey(id, "code"),
					state.Text(i18n.KeyDetailCode, nil), &key.Code)
				tablePropertiesCheckbox(c, TablePropertiesKeyFieldKey(id, "primary"),
					state.Text(i18n.KeyDetailPrimary, nil), &key.Primary, lang)
				for member := range key.Members {
					entry := &key.Members[member]
					ui.Row(c).FillWidth().Gap(6).Children(func() {
						ui.Text(c, strconv.Itoa(member+1)).Width(24).Shrink(0).
							FontSize(11).TextColor(theme.TextMuted).Lang(lang)
						tablePropertiesInput(c,
							TablePropertiesKeyMemberFieldKey(id, member, "name"),
							state.Text(i18n.KeyDetailColumns, nil), 0, &entry.Name, lang)
					})
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
	preview := w.tableProperties.previewEditor
	ui.Column(c.Key(TablePropertiesPreviewKey)).FillWidth().Grow(1).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			label := state.Text(i18n.KeyPreviewCopySQL, nil)
			ui.Button(c, label).Key(TablePropertiesCopyKey).Label(label).Lang(lang).
				Disabled(preview == nil || preview.Buffer.Len() == 0).
				OnClick(w.copyTablePropertiesSQL)
			if status := w.tablePropertiesCopyStatus(state); status != "" {
				ui.Text(c, status).Key(TablePropertiesCopyStatusKey).Grow(1).
					FontSize(11).TextColor(theme.TextMuted).Lang(lang).Role(ui.RoleStatus)
			}
		})
		w.tablePropertiesDiagnostics(c, state, theme, lang, result)
		if preview != nil {
			preview.Label = state.Text(i18n.KeyPreviewSQL, nil)
			editor.View(c.Key(TablePropertiesSQLKey), preview).
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
	buffer := editor.NewBuffer(w.tableProperties.previewResult.SQL)
	buffer.SetLanguage(editor.LanguageFor("database.sql"))
	w.tableProperties.previewEditor = &editor.State{
		Buffer:          buffer,
		SoftWrap:        false,
		KeepScrollbars:  true,
		NoScrollPastEnd: true,
	}
}

func (w *Workspace) copyTablePropertiesSQL() {
	preview := w.tableProperties.previewEditor
	if preview == nil || preview.Buffer.Len() == 0 {
		return
	}
	text := preview.Buffer.String()
	version := preview.Buffer.Version()
	w.tableProperties.copyToken++
	token := w.tableProperties.copyToken
	writer := w.clipboardWriter
	feedback := copyFeedbackSucceeded
	if writer == nil || writer(text) != nil {
		feedback = copyFeedbackFailed
	}
	if token != w.tableProperties.copyToken || preview != w.tableProperties.previewEditor {
		return
	}
	w.tableProperties.copyFeedback = feedback
	w.tableProperties.copiedVersion = version
	w.notifyChanged()
}

func (w *Workspace) tablePropertiesCopyStatus(state *app.State) string {
	if preview := w.tableProperties.previewEditor; preview == nil || preview.Buffer.Version() != w.tableProperties.copiedVersion {
		return ""
	}
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

func tablePropertiesLabeledInput(
	c *ui.Context,
	theme *ui.Theme,
	lang string,
	key string,
	label string,
	value *string,
) {
	ui.Column(c).FillWidth().Gap(3).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(theme.TextMuted).Lang(lang)
		tablePropertiesInput(c, key, label, 0, value, lang)
	})
}

func tablePropertiesLabeledArea(
	c *ui.Context,
	theme *ui.Theme,
	lang string,
	key string,
	label string,
	value *string,
) {
	ui.Column(c).FillWidth().Gap(3).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(theme.TextMuted).Lang(lang)
		c.Key(key)
		ui.TextArea(c, value).Label(label).FillWidth().MinHeight(64).Lang(lang)
	})
}

func tablePropertiesInput(
	c *ui.Context,
	key, label string,
	width float32,
	value *string,
	lang string,
) {
	c.Key(key)
	input := ui.TextInput(c, value).Label(label).Lang(lang)
	if width > 0 {
		input.Width(width).Shrink(0)
	} else {
		input.FillWidth()
	}
}

func tablePropertiesCheckbox(
	c *ui.Context,
	key, label string,
	value *bool,
	lang string,
) {
	c.Key(key)
	ui.Checkbox(c, value, label).Label(label).Lang(lang)
}

func tablePropertiesCheckboxState(
	c *ui.Context,
	key, label string,
	value *ui.CheckState,
	lang string,
) {
	c.Key(key)
	ui.CheckboxState(c, value, label).Label(label).Lang(lang)
}
