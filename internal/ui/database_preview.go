package appui

import (
	"fmt"
	"strings"
	"unicode"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	DatabasePreviewRootKey           = "pdmview.database-preview.root"
	DatabasePreviewOpenKey           = "pdmview.workspace.database-preview.open"
	DatabasePreviewBackKey           = "pdmview.workspace.database-preview.back"
	DatabasePreviewTabsKey           = "pdmview.database-preview.tabs"
	DatabasePreviewDiagnosticsTabKey = "pdmview.database-preview.tab.diagnostics"
	DatabasePreviewSQLTabKey         = "pdmview.database-preview.tab.sql"
	DatabasePreviewSQLKey            = "pdmview.workspace.database-preview.sql"
	DatabasePreviewDiagnosticsKey    = "pdmview.workspace.database-preview.diagnostics"
	DatabasePreviewCopySQLKey        = "pdmview.workspace.database-preview.copy-sql"
	DatabasePreviewExportSQLKey      = "pdmview.workspace.database-preview.export-sql"
)

type databasePreviewStats struct {
	tables      int
	columns     int
	foreignKeys int
}

type databasePreviewState struct {
	open              bool
	loading           bool
	token             uint64
	loadToken         uint64
	result            sqlgen.Result
	stats             databasePreviewStats
	editor            *editor.State
	diagnosticsScroll ui.ScrollState
	tab               int
	actionFeedback    previewActionFeedback
	actionToken       uint64
	exporting         bool
}

type previewActionFeedback uint8

const (
	previewActionNone previewActionFeedback = iota
	previewActionCopied
	previewActionCopyFailed
	previewActionExported
	previewActionExportFailed
)

// DatabasePreviewRequest is one background generation for a frozen document.
type DatabasePreviewRequest struct {
	workspace *Workspace
	document  *pdm.Document
	token     uint64
	loadToken uint64
}

// Run generates the database SQL away from the UI thread.
func (request DatabasePreviewRequest) Run() sqlgen.Result {
	return sqlgen.GenerateDatabase(request.document, sqlgen.DatabaseOptions{})
}

func (w *Workspace) SetDatabasePreviewStarter(starter func(DatabasePreviewRequest)) {
	w.previewStarter = starter
}

func (w *Workspace) SetSQLExportStarter(starter func(SQLExportRequest)) {
	w.exportStarter = starter
}

func (w *Workspace) BeginDatabasePreview() (DatabasePreviewRequest, bool) {
	if !w.hasModel || w.document == nil || w.loading {
		return DatabasePreviewRequest{}, false
	}
	w.preview.token++
	w.preview.open = true
	w.preview.loading = true
	w.preview.result = sqlgen.Result{}
	w.preview.editor = nil
	w.preview.actionFeedback = previewActionNone
	w.preview.actionToken++
	w.preview.exporting = false
	w.preview.tab = 0
	w.preview.stats = countDatabasePreview(w.document)
	w.preview.diagnosticsScroll.ScrollTo(0, 0, ui.Animation{})
	request := DatabasePreviewRequest{
		workspace: w,
		document:  w.document,
		token:     w.preview.token,
		loadToken: w.loadToken,
	}
	w.notifyChanged()
	return request, true
}

func (w *Workspace) PublishDatabasePreview(request DatabasePreviewRequest, result sqlgen.Result) bool {
	if request.workspace != w ||
		request.token != w.preview.token ||
		request.loadToken != w.loadToken ||
		!w.preview.open {
		return false
	}
	w.preview.loading = false
	w.preview.actionFeedback = previewActionNone
	w.preview.actionToken++
	w.preview.exporting = false
	w.preview.result = sqlgen.Result{
		SQL:         result.SQL,
		Diagnostics: append([]sqlgen.Diagnostic(nil), result.Diagnostics...),
	}
	if result.HasErrors() {
		w.preview.tab = 0
	} else {
		w.preview.tab = 1
	}
	w.preview.editor = nil
	if !result.HasErrors() && result.SQL != "" {
		buffer := editor.NewBuffer(result.SQL)
		buffer.SetLanguage(editor.LanguageFor("database.sql"))
		w.preview.editor = &editor.State{
			Buffer:          buffer,
			ReadOnly:        true,
			SoftWrap:        false,
			KeepScrollbars:  true,
			NoScrollPastEnd: true,
			Label:           "Full SQL",
		}
	}
	w.notifyChanged()
	return true
}

func (w *Workspace) CloseDatabasePreview() {
	w.invalidateDatabasePreview()
	w.notifyChanged()
}

func (w *Workspace) invalidateDatabasePreview() {
	w.preview.token++
	w.preview.open = false
	w.preview.loading = false
	w.preview.result = sqlgen.Result{}
	w.preview.stats = databasePreviewStats{}
	w.preview.editor = nil
	w.preview.actionFeedback = previewActionNone
	w.preview.actionToken++
	w.preview.exporting = false
	w.preview.tab = 0
	w.preview.diagnosticsScroll.ScrollTo(0, 0, ui.Animation{})
}

func (w *Workspace) openDatabasePreview() {
	request, ok := w.BeginDatabasePreview()
	if !ok {
		return
	}
	if w.previewStarter != nil {
		w.previewStarter(request)
		return
	}
	w.PublishDatabasePreview(request, request.Run())
}

// SQLExportRequest carries immutable SQL bytes and a UI-safe default filename.
type SQLExportRequest struct {
	workspace    *Workspace
	previewToken uint64
	loadToken    uint64
	actionToken  uint64
	sql          string
	defaultPath  string
	title        string
	filterName   string
}

func (request SQLExportRequest) SQL() string         { return request.sql }
func (request SQLExportRequest) DefaultPath() string { return request.defaultPath }
func (request SQLExportRequest) Title() string       { return request.title }
func (request SQLExportRequest) FilterName() string  { return request.filterName }

func (w *Workspace) BeginSQLExport(state *app.State) (SQLExportRequest, bool) {
	if !w.previewActionsAvailable() || w.preview.exporting {
		return SQLExportRequest{}, false
	}
	w.preview.actionToken++
	w.preview.exporting = true
	w.preview.actionFeedback = previewActionNone
	request := SQLExportRequest{
		workspace:    w,
		previewToken: w.preview.token,
		loadToken:    w.loadToken,
		actionToken:  w.preview.actionToken,
		sql:          w.preview.result.SQL,
		defaultPath:  suggestedSQLFilename(w.document),
		title:        state.Text(i18n.KeyPreviewSaveTitle, nil),
		filterName:   state.Text(i18n.KeyPreviewSaveFilter, nil),
	}
	w.notifyChanged()
	return request, true
}

func (w *Workspace) PublishSQLExportCanceled(request SQLExportRequest) bool {
	return w.finishSQLExport(request, previewActionNone)
}

func (w *Workspace) PublishSQLExportSucceeded(request SQLExportRequest) bool {
	return w.finishSQLExport(request, previewActionExported)
}

func (w *Workspace) PublishSQLExportFailed(request SQLExportRequest) bool {
	return w.finishSQLExport(request, previewActionExportFailed)
}

func (w *Workspace) SQLExportCurrent(request SQLExportRequest) bool {
	return request.workspace == w &&
		request.previewToken == w.preview.token &&
		request.loadToken == w.loadToken &&
		request.actionToken == w.preview.actionToken &&
		w.preview.open && w.preview.exporting &&
		request.sql == w.preview.result.SQL
}

func (w *Workspace) finishSQLExport(
	request SQLExportRequest,
	feedback previewActionFeedback,
) bool {
	if !w.SQLExportCurrent(request) {
		return false
	}
	w.preview.exporting = false
	w.preview.actionFeedback = feedback
	w.notifyChanged()
	return true
}

func (w *Workspace) copyPreviewSQL() {
	if !w.previewActionsAvailable() || w.preview.exporting {
		return
	}
	text := w.preview.result.SQL
	w.preview.actionToken++
	token := w.preview.actionToken
	previewToken, loadToken := w.preview.token, w.loadToken
	writer := w.clipboardWriter
	if writer == nil || writer(text) != nil {
		if token == w.preview.actionToken && previewToken == w.preview.token &&
			loadToken == w.loadToken && w.preview.open &&
			w.preview.result.SQL == text {
			w.preview.actionFeedback = previewActionCopyFailed
			w.notifyChanged()
		}
		return
	}
	if token != w.preview.actionToken || previewToken != w.preview.token ||
		loadToken != w.loadToken || !w.preview.open || w.preview.result.SQL != text {
		return
	}
	w.preview.actionFeedback = previewActionCopied
	w.notifyChanged()
}

func (w *Workspace) exportPreviewSQL(state *app.State) {
	request, ok := w.BeginSQLExport(state)
	if !ok {
		return
	}
	if w.exportStarter != nil {
		w.exportStarter(request)
		return
	}
	w.PublishSQLExportFailed(request)
}

func (w *Workspace) previewActionsAvailable() bool {
	return w.preview.open && !w.preview.loading &&
		!w.preview.result.HasErrors() && w.preview.result.SQL != ""
}

func (w *Workspace) databasePreviewView(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	title := state.Text(i18n.KeyDatabaseSQL, nil)
	status, statusColor, statusRole := w.databasePreviewStatus(state, theme)
	ui.Column(c.Key(DatabasePreviewRootKey)).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).FillWidth().Height(52).Padding(8, 12).Gap(10).
			AlignItems(ui.Center).Background(theme.Surface).
			BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
			ui.Button(c, state.Text(i18n.KeyPreviewBack, nil)).
				Key(DatabasePreviewBackKey).OnClick(w.CloseDatabasePreview)
			ui.Text(c, title).Grow(1).FontSize(15).Bold().TextColor(theme.Text).
				Lang(lang).Role(ui.RoleHeading).Level(1)
			if status != "" {
				ui.Text(c, status).FontSize(11).TextColor(statusColor).Lang(lang).Role(statusRole)
			}
		})
		ui.Column(c).Grow(1).FillWidth().Padding(12).Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(16).Wrap().Children(func() {
				ui.Text(c, state.Text(i18n.KeyPreviewTables, countParam(w.preview.stats.tables))).
					FontSize(11).TextColor(theme.TextMuted).Lang(lang)
				ui.Text(c, state.Text(i18n.KeyPreviewColumns, countParam(w.preview.stats.columns))).
					FontSize(11).TextColor(theme.TextMuted).Lang(lang)
				ui.Text(c, state.Text(i18n.KeyPreviewKeys, countParam(w.preview.stats.foreignKeys))).
					FontSize(11).TextColor(theme.TextMuted).Lang(lang)
			})
			w.databasePreviewTabs(c, state, theme, lang)
			ui.Column(c).Grow(1).FillWidth().MinHeight(0).
				Role(ui.RoleTabPanel).Children(func() {
				switch w.preview.tab {
				case 0:
					w.databasePreviewDiagnostics(c, state, theme, lang)
				case 1:
					w.databasePreviewSQL(c, state, theme, lang)
				}
			})
		})
	})
}

func (w *Workspace) databasePreviewTabs(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	labels := []string{
		state.Text(i18n.KeyPreviewDiagnostics, nil),
		state.Text(i18n.KeyPreviewSQLTab, nil),
	}
	tabs := ui.TabsBase(c.Key(DatabasePreviewTabsKey), &w.preview.tab, len(labels))
	tabs.List.Gap(4).Children(func() {
		for index, label := range labels {
			switch index {
			case 0:
				c.Key(DatabasePreviewDiagnosticsTabKey)
			case 1:
				c.Key(DatabasePreviewSQLTabKey)
			}
			tab := tabs.Tab(index).Padding(4, 8).Radius(4)
			if index == w.preview.tab {
				tab.Background(theme.Surface).TextColor(theme.Text).Bold()
			} else {
				tab.TextColor(theme.TextMuted)
			}
			tab.Children(func() {
				ui.Text(c, label).SingleLine().Lang(lang)
			})
		}
	})
}

func (w *Workspace) databasePreviewSQL(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Gap(8).Children(func() {
		w.databasePreviewActions(c, state, theme, lang)
		if w.preview.result.HasErrors() {
			ui.Text(c, state.Text(i18n.KeyPreviewSQLBlocked, nil)).
				FillWidth().FontSize(11).TextColor(theme.Danger).Lang(lang).Role(ui.RoleAlert)
		}
		if w.preview.editor != nil {
			w.preview.editor.Label = state.Text(i18n.KeyPreviewSQL, nil)
			editor.View(c.Key(DatabasePreviewSQLKey), w.preview.editor).Grow(1).FillWidth().MinHeight(0)
		}
	})
}

func (w *Workspace) databasePreviewActions(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	enabled := w.previewActionsAvailable() && !w.preview.exporting
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		copyLabel := state.Text(i18n.KeyPreviewCopySQL, nil)
		ui.Button(c, copyLabel).Key(DatabasePreviewCopySQLKey).
			Label(copyLabel).Lang(lang).Disabled(!enabled).OnClick(w.copyPreviewSQL)
		exportLabel := state.Text(i18n.KeyPreviewExportSQL, nil)
		ui.Button(c, exportLabel).Key(DatabasePreviewExportSQLKey).
			Label(exportLabel).Lang(lang).Disabled(!enabled).
			OnClick(func() { w.exportPreviewSQL(state) })
		if feedback, color, role := w.previewActionStatus(state, theme); feedback != "" {
			ui.Text(c, feedback).Grow(1).FontSize(11).TextColor(color).Lang(lang).Role(role)
		}
	})
}

func (w *Workspace) previewActionStatus(
	state *app.State,
	theme *ui.Theme,
) (string, ui.Color, ui.Role) {
	if w.preview.exporting {
		return state.Text(i18n.KeyPreviewExporting, nil), theme.TextMuted, ui.RoleStatus
	}
	switch w.preview.actionFeedback {
	case previewActionCopied:
		return state.Text(i18n.KeyPreviewSQLCopied, nil), theme.Success, ui.RoleStatus
	case previewActionCopyFailed:
		return state.Text(i18n.KeyPreviewSQLCopyFailed, nil), theme.Danger, ui.RoleAlert
	case previewActionExported:
		return state.Text(i18n.KeyPreviewSQLExported, nil), theme.Success, ui.RoleStatus
	case previewActionExportFailed:
		return state.Text(i18n.KeyPreviewSQLExportFailed, nil), theme.Danger, ui.RoleAlert
	default:
		return "", theme.TextMuted, ui.RoleStatus
	}
}

func (w *Workspace) databasePreviewStatus(
	state *app.State,
	theme *ui.Theme,
) (string, ui.Color, ui.Role) {
	if w.preview.loading {
		return state.Text(i18n.KeyPreviewLoading, nil), theme.TextMuted, ui.RoleStatus
	}
	errors, warnings := previewDiagnosticCounts(w.preview.result.Diagnostics)
	if errors+warnings == 0 {
		return "", theme.TextMuted, ui.RoleStatus
	}
	summary := state.Text(i18n.KeyPreviewSummary, map[string]string{
		"errors": fmt.Sprintf("%d", errors), "warnings": fmt.Sprintf("%d", warnings),
	})
	if errors > 0 {
		return summary, theme.Danger, ui.RoleAlert
	}
	return summary, theme.Warning, ui.RoleStatus
}

func (w *Workspace) databasePreviewDiagnostics(
	c *ui.Context,
	state *app.State,
	theme *ui.Theme,
	lang string,
) {
	diagnostics := w.preview.result.Diagnostics
	if len(diagnostics) == 0 {
		if !w.preview.loading {
			ui.Text(c, state.Text(i18n.KeyPreviewNoDiagnostics, nil)).
				FontSize(11).TextColor(theme.TextMuted).Lang(lang).Role(ui.RoleStatus)
		}
		return
	}
	ui.Scroll(c.Key(DatabasePreviewDiagnosticsKey)).TrackScroll(&w.preview.diagnosticsScroll).
		Grow(1).FillWidth().MinHeight(0).KeepScrollbars(true).Children(func() {
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			for _, diagnostic := range diagnostics {
				severity := state.Text(i18n.KeyPreviewWarning, nil)
				color := theme.Warning
				role := ui.RoleStatus
				if diagnostic.Severity == sqlgen.SeverityError {
					severity = state.Text(i18n.KeyPreviewError, nil)
					color, role = theme.Danger, ui.RoleAlert
				}
				category := state.Text(previewDiagnosticCategory(diagnostic.Code), nil)
				heading := state.Text(i18n.KeyPreviewDiagnosticHeading, map[string]string{
					"category": category, "severity": severity,
				})
				detail := diagnostic.Code
				if diagnostic.Line > 0 {
					detail += " · " + state.Text(i18n.KeyPreviewLine, map[string]string{
						"line": fmt.Sprintf("%d", diagnostic.Line),
					})
				}
				location := diagnostic.Path
				if location == "" {
					location = diagnostic.ObjectID
				}
				ui.Column(c).FillWidth().Gap(1).Children(func() {
					ui.Text(c, heading).FontSize(11).Bold().TextColor(color).
						Lang(lang).Role(role)
					ui.Text(c, detail).SingleLine().FontSize(10).
						Font("monospace").TextColor(theme.TextMuted).Lang(lang)
					if location != "" {
						ui.Text(c, location).Wrap().FontSize(10).
							Font("monospace").TextColor(theme.TextMuted).Selectable().Lang(lang)
					}
				})
			}
		})
	})
}

func previewDiagnosticCategory(code string) i18n.Key {
	code = strings.ToLower(code)
	switch {
	case strings.HasPrefix(code, "pdm-"):
		return i18n.KeyPreviewParserIssue
	case strings.HasPrefix(code, "database-"):
		return i18n.KeyPreviewDatabaseIssue
	case strings.Contains(code, "foreign-key"), strings.Contains(code, "constraint"):
		return i18n.KeyPreviewForeignKeyIssue
	case strings.Contains(code, "index"), strings.Contains(code, "key"),
		strings.Contains(code, "primary"), strings.Contains(code, "auto-increment"):
		return i18n.KeyPreviewIndexIssue
	case strings.Contains(code, "default"):
		return i18n.KeyPreviewDefaultIssue
	case strings.Contains(code, "column"), strings.Contains(code, "type"):
		return i18n.KeyPreviewColumnIssue
	default:
		return i18n.KeyPreviewSchemaIssue
	}
}

func countDatabasePreview(document *pdm.Document) databasePreviewStats {
	var stats databasePreviewStats
	if document != nil {
		countPreviewContents(document.Model.Contents, &stats)
	}
	return stats
}

func countPreviewContents(contents pdm.Contents, stats *databasePreviewStats) {
	for _, table := range contents.Tables {
		stats.tables++
		stats.columns += len(table.Columns)
	}
	stats.foreignKeys += len(contents.References)
	for _, pkg := range contents.Packages {
		countPreviewContents(pkg.Contents, stats)
	}
}

func countParam(value int) map[string]string {
	return map[string]string{"count": fmt.Sprintf("%d", value)}
}

func previewDiagnosticCounts(diagnostics []sqlgen.Diagnostic) (int, int) {
	errors, warnings := 0, 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == sqlgen.SeverityError {
			errors++
		} else {
			warnings++
		}
	}
	return errors, warnings
}

func suggestedSQLFilename(document *pdm.Document) string {
	name := "database"
	if document != nil {
		if document.Model.Code != "" {
			name = document.Model.Code
		} else if document.Model.Name != "" {
			name = document.Model.Name
		}
	}
	var sanitized strings.Builder
	for _, current := range name {
		if unicode.IsControl(current) || strings.ContainsRune(`/\:*?"<>|`, current) {
			sanitized.WriteByte('_')
		} else {
			sanitized.WriteRune(current)
		}
	}
	name = strings.Trim(sanitized.String(), " .")
	if name == "" {
		name = "database"
	}
	runes := []rune(name)
	if len(runes) > 96 {
		name = string(runes[:96])
	}
	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9", "COM¹", "COM²", "COM³", "LPT¹", "LPT²", "LPT³":
		name = "_" + name
	}
	if !strings.HasSuffix(strings.ToLower(name), ".sql") {
		name += ".sql"
	}
	return name
}
