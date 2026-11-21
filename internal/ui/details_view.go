package appui

import (
	"strconv"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

const (
	DetailsToggleKey    = "pdmview.details.toggle"
	OpenPropertiesKey   = "pdmview.details.open-properties"
	DetailRelatedKey    = "pdmview.details.related"
	DetailReferencesKey = "pdmview.details.references-to"
	DetailReferencedKey = "pdmview.details.referenced-by"
)

func (w *Workspace) detailsToggle(c *ui.Context, state *app.State, lang string) {
	key := i18n.KeyDetailsHide
	if w.detailsHidden {
		key = i18n.KeyDetailsShow
	}
	label := state.Text(key, nil)
	ui.Button(c, label).Key(DetailsToggleKey).Height(24).Label(label).Lang(lang).Expanded(!w.detailsHidden).
		OnClick(func() {
			w.detailsHidden = !w.detailsHidden
			w.notifyChanged()
		})
}

func (w *Workspace) detailsPane(c *ui.Context, state *app.State, theme *ui.Theme, lang string) {
	ui.Column(c).Fill().Background(theme.Surface).Padding(12).Gap(10).
		MinWidth(detailsMinWidth).MaxWidth(detailsMaxWidth).Clip().Children(func() {
		paneHeading(c, state.Text(i18n.KeyDetailsHeading, nil), theme, lang)
		selected := w.hasTableDetail && w.tableDetail.ID == w.selectedTableID
		if selected {
			w.detailsActions(c, state, theme, lang, w.tableDetail.ID)
		}
		ui.Scroll(c.Key(DetailsBodyKey)).TrackScroll(&w.detailsScroll).Grow(1).FillWidth().Children(func() {
			if selected {
				detail := w.tableDetail
				ui.Column(c).FillWidth().Gap(12).Children(func() {
					renderTableSummary(c, state, theme, lang, detail, w.unplacedTable)
					if len(detail.ReferencesTo)+len(detail.ReferencedBy) != 0 {
						w.detailsRelations(c, state, theme, lang, detail)
					}
				})
			} else if w.selectedTableID != "" {
				renderTableName(c, theme, lang, w.selectedTableName)
			} else if node := w.canvas.selectedNode(); node != nil {
				renderTableName(c, theme, lang, node.Name)
			}
		})
	})
}

func renderTableName(c *ui.Context, theme *ui.Theme, lang, name string) {
	ui.Text(c, name).Key(DetailTableKey).FillWidth().Wrap().FontSize(14).
		Bold().TextColor(theme.Text).Lang(lang).Role(ui.RoleHeading).Level(3)
}

func renderTableSummary(c *ui.Context, state *app.State, theme *ui.Theme, lang string, detail TableDetail, unplaced bool) {
	name := firstNonEmpty(detail.Name, detail.Code, detail.ID)
	count := state.Text(i18n.KeyDetailColumnCount, map[string]string{"count": strconv.Itoa(len(detail.Columns))})
	for _, key := range detail.Keys {
		if key.Primary {
			count += " · " + state.Text(i18n.KeyDetailPrimaryCount, nil)
			break
		}
	}
	ui.Column(c).FillWidth().Gap(5).Children(func() {
		renderTableName(c, theme, lang, name)
		if detail.Code != "" && detail.Code != name {
			ui.Text(c, detail.Code).Key(DetailCodeKey).FillWidth().Wrap().Selectable().
				FontSize(12).TextColor(theme.Text).Lang(lang)
		}
		if strings.TrimSpace(detail.Comment.Value) != "" {
			ui.Text(c, detail.Comment.Value).Key(DetailCommentKey).FillWidth().Wrap().MaxLines(3).Ellipsis("…").
				FontSize(12).TextColor(theme.TextMuted).Lang(lang)
		}
		ui.Text(c, count).FillWidth().Wrap().FontSize(11).TextColor(theme.TextMuted).Lang(lang)
		if unplaced {
			ui.Text(c, state.Text(i18n.KeyNotOnDiagram, nil)).FillWidth().Wrap().
				FontSize(11).TextColor(theme.TextMuted).Lang(lang).Role(ui.RoleStatus)
		}
	})
}

func (w *Workspace) detailsActions(c *ui.Context, state *app.State, theme *ui.Theme, lang, tableID string) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, action := range []struct {
			key   string
			label i18n.Key
			run   func()
		}{
			{OpenPropertiesKey, i18n.KeyOpenTableProperties, func() { w.requestTableProperties(tableID) }},
			{CopyTableKey, i18n.KeyCopyTable, w.copySelectedTable},
			{CopyCreateTableKey, i18n.KeyCopyCreateTable, w.copySelectedCreateTable},
		} {
			label := state.Text(action.label, nil)
			ui.Button(c, label).Key(action.key).Label(label).Lang(lang).FillWidth().OnClick(action.run)
		}
		if status, failed := w.copyFeedbackText(state); status != "" {
			color, role := theme.Success, ui.RoleStatus
			if failed {
				color, role = theme.Danger, ui.RoleAlert
			}
			ui.Text(c, status).Key(CopyStatusKey).FillWidth().Wrap().
				FontSize(11).TextColor(color).Lang(lang).Role(role)
		}
	})
}

func (w *Workspace) detailsRelations(c *ui.Context, state *app.State, theme *ui.Theme, lang string, detail TableDetail) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, state.Text(i18n.KeyDetailRelatedTables, nil)).Key(DetailRelatedKey).FontSize(12).
			Bold().TextColor(theme.Text).Lang(lang).Role(ui.RoleHeading).Level(3)
		w.relatedTableGroup(c, state, theme, lang, DetailReferencesKey, i18n.KeyDetailReferencesTo, detail.ReferencesTo)
		w.relatedTableGroup(c, state, theme, lang, DetailReferencedKey, i18n.KeyDetailReferencedBy, detail.ReferencedBy)
	})
}

func (w *Workspace) relatedTableGroup(c *ui.Context, state *app.State, theme *ui.Theme, lang, key string, heading i18n.Key, refs []DetailRef) {
	if len(refs) == 0 {
		return
	}
	ui.Column(c.Key(key)).FillWidth().Gap(4).Children(func() {
		ui.Text(c, state.Text(heading, nil)).FillWidth().Wrap().FontSize(11).TextColor(theme.TextMuted).
			Lang(lang).Role(ui.RoleHeading).Level(4)
		for _, table := range refs {
			if !table.Resolved {
				label := state.Text(i18n.KeyDetailUnresolved, nil)
				if table.ID != "" {
					label = table.ID + " · " + label
				}
				ui.Text(c, label).FillWidth().Wrap().FontSize(11).TextColor(theme.TextMuted).Lang(lang)
				continue
			}
			name := firstNonEmpty(table.Name, table.Code, table.ID)
			ui.Button(c, name).Key(key + "." + table.ID).Label(name).Tooltip(name).Lang(lang).FillWidth().
				OnClick(func() { w.locateTable(table.ID, true) })
		}
	})
}
