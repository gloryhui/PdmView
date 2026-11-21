package appui

import (
	"fmt"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	DatabasePreviewRootKey        = "pdmview.database-preview.root"
	DatabasePreviewOpenKey        = "pdmview.workspace.database-preview.open"
	DatabasePreviewBackKey        = "pdmview.workspace.database-preview.back"
	DatabasePreviewSQLKey         = "pdmview.workspace.database-preview.sql"
	DatabasePreviewDiagnosticsKey = "pdmview.workspace.database-preview.diagnostics"
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
}

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

func (w *Workspace) BeginDatabasePreview() (DatabasePreviewRequest, bool) {
	if !w.hasModel || w.document == nil || w.loading {
		return DatabasePreviewRequest{}, false
	}
	w.preview.token++
	w.preview.open = true
	w.preview.loading = true
	w.preview.result = sqlgen.Result{}
	w.preview.editor = nil
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
	w.preview.result = sqlgen.Result{
		SQL:         result.SQL,
		Diagnostics: append([]sqlgen.Diagnostic(nil), result.Diagnostics...),
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
			w.databasePreviewDiagnostics(c, state, theme, lang)
			if w.preview.editor != nil {
				w.preview.editor.Label = state.Text(i18n.KeyPreviewSQL, nil)
				c.Key(DatabasePreviewSQLKey)
				editor.View(c, w.preview.editor).Grow(1).FillWidth()
			}
		})
	})
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
		return
	}
	ui.Text(c, state.Text(i18n.KeyPreviewDiagnostics, nil)).
		FontSize(12).Bold().TextColor(theme.Text).Lang(lang).
		Role(ui.RoleHeading).Level(2)
	ui.Scroll(c.Key(DatabasePreviewDiagnosticsKey)).TrackScroll(&w.preview.diagnosticsScroll).
		FillWidth().Height(112).KeepScrollbars(true).Children(func() {
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
						ui.Text(c, location).Wrap().MaxLines(2).FontSize(10).
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
