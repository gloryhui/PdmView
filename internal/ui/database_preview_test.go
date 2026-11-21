package appui

import (
	"strings"
	"testing"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
)

func TestDatabasePreviewGenerationLocaleAndEditor(t *testing.T) {
	state := newState(t, "en-US")
	document := previewDocument()
	set := BuildBoardSet(document)
	if set.Document != document {
		t.Fatal("BoardSet did not retain the frozen PDM document")
	}
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), set)
	tester := ui.NewTester(View(state, workspace), 800, 600)

	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !workspace.preview.open || workspace.preview.loading || workspace.preview.editor == nil {
		t.Fatalf("preview state = %+v", workspace.preview)
	}
	wantSQL := sqlgen.GenerateDatabase(document, sqlgen.DatabaseOptions{}).SQL
	if workspace.preview.result.SQL != wantSQL ||
		workspace.preview.editor.Buffer.String() != wantSQL {
		t.Fatal("preview did not retain the generated Result.SQL bytes")
	}
	if language := workspace.preview.editor.Buffer.Language(); language == nil || language.Name != "SQL" {
		t.Fatalf("editor language = %+v", language)
	}
	if !workspace.preview.editor.ReadOnly || workspace.preview.editor.SoftWrap ||
		!workspace.preview.editor.NoScrollPastEnd {
		t.Fatalf("editor options = readonly:%v softWrap:%v noScrollPastEnd:%v",
			workspace.preview.editor.ReadOnly, workspace.preview.editor.SoftWrap,
			workspace.preview.editor.NoScrollPastEnd)
	}
	for _, text := range []string{
		"Database SQL", "Back", "Tables 2", "Columns 4", "Foreign keys 1",
	} {
		if !tester.HasText(text) {
			t.Fatalf("missing English preview text %q: %q", text, tester.Texts())
		}
	}
	if back, ok := tester.AccessByRole(ui.RoleButton, "Back"); !ok ||
		back.Bounds.X < 0 || back.Bounds.Y < 0 ||
		back.Bounds.X+back.Bounds.W > 801 || back.Bounds.Y+back.Bounds.H > 601 {
		t.Fatalf("back button outside minimum window: %+v, %v", back, ok)
	}

	editorState := workspace.preview.editor
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if workspace.preview.result.SQL != wantSQL ||
		workspace.preview.editor != editorState ||
		workspace.preview.editor.Buffer.String() != wantSQL {
		t.Fatal("language change regenerated or changed SQL bytes")
	}
	for _, text := range []string{"数据库 SQL", "返回", "表 2", "字段 4", "外键 1"} {
		if !tester.HasText(text) {
			t.Fatalf("missing Chinese preview text %q: %q", text, tester.Texts())
		}
	}
}

func TestDatabasePreviewLoadingStaleAndModelInvalidation(t *testing.T) {
	state := newState(t, "en-US")
	document := previewDocument()
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(document))
	var captured DatabasePreviewRequest
	workspace.SetDatabasePreviewStarter(func(request DatabasePreviewRequest) {
		captured = request
	})
	tester := ui.NewTester(View(state, workspace), 800, 600)
	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	if !workspace.preview.loading || !tester.HasText("Generating database SQL…") {
		t.Fatalf("loading preview = %+v, texts %q", workspace.preview, tester.Texts())
	}

	first := captured
	second, ok := workspace.BeginDatabasePreview()
	if !ok || first.token == second.token {
		t.Fatal("second preview request did not advance its token")
	}
	secondResult := second.Run()
	if workspace.PublishDatabasePreview(first, first.Run()) {
		t.Fatal("stale preview request was published")
	}
	if !workspace.PublishDatabasePreview(second, secondResult) {
		t.Fatal("current preview request was rejected")
	}

	third, ok := workspace.BeginDatabasePreview()
	if !ok {
		t.Fatal("third preview request failed")
	}
	loadToken := workspace.BeginOpen()
	workspace.PublishBoards(loadToken, BuildBoardSet(previewDocument()))
	if workspace.PublishDatabasePreview(third, third.Run()) {
		t.Fatal("preview survived a model load")
	}
	if workspace.preview.open || workspace.preview.editor != nil {
		t.Fatal("new model retained preview output")
	}

	fourth, ok := workspace.BeginDatabasePreview()
	if !ok {
		t.Fatal("fourth preview request failed")
	}
	workspace.CloseDatabasePreview()
	if workspace.PublishDatabasePreview(fourth, fourth.Run()) ||
		workspace.preview.open || workspace.preview.editor != nil {
		t.Fatal("closed preview accepted a result")
	}
}

func TestDatabasePreviewDiagnosticsSummaryAndLocation(t *testing.T) {
	state := newState(t, "en-US")
	document := previewDocument()
	path := "/Model/RootObject/Children/object:Model/collection:Tables/object:Shortcut"
	document.Diagnostics = []pdm.Diagnostic{{
		Code: pdm.DiagnosticExternalReference, Path: path, Line: 42,
	}}
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(document))
	tester := ui.NewTester(View(state, workspace), 800, 600)
	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if workspace.preview.result.SQL != "" || workspace.preview.editor != nil {
		t.Fatalf("failed preview kept SQL: %+v", workspace.preview.result)
	}
	for _, text := range []string{
		"Errors 1 · Warnings 0",
		"Source parsing · Error",
		"pdm-external-reference · line 42",
		path,
	} {
		if !tester.HasText(text) {
			t.Fatalf("missing diagnostic text %q: %q", text, tester.Texts())
		}
	}
	if count := countTextsContaining(tester.Texts(), "Errors 1 · Warnings 0"); count != 1 {
		t.Fatalf("summary shown %d times, want one", count)
	}

	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	for _, text := range []string{"1 个错误 · 0 个警告", "源解析 · 错误", "pdm-external-reference · 行 42"} {
		if !tester.HasText(text) {
			t.Fatalf("missing Chinese diagnostic text %q: %q", text, tester.Texts())
		}
	}
}

func TestDatabasePreviewCopyAndExportExactBytes(t *testing.T) {
	state := newState(t, "en-US")
	document := previewDocument()
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(document))
	var copied string
	workspace.SetClipboardWriter(func(text string) error {
		copied = text
		return nil
	})
	var exportRequest SQLExportRequest
	workspace.SetSQLExportStarter(func(request SQLExportRequest) {
		exportRequest = request
	})
	tester := ui.NewTester(View(state, workspace), 800, 600)
	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	wantSQL := workspace.preview.result.SQL
	info, ok := tester.AccessByRole(ui.RoleButton, "Copy SQL")
	if !ok || info.Disabled {
		t.Fatalf("Copy SQL = %+v, %v", info, ok)
	}
	if bounds := info.Bounds; bounds.X < 0 || bounds.Y < 0 ||
		bounds.X+bounds.W > 801 || bounds.Y+bounds.H > 601 {
		t.Fatalf("Copy SQL outside minimum window: %+v", bounds)
	}
	if exportInfo, ok := tester.AccessByRole(ui.RoleButton, "Export .sql"); !ok ||
		exportInfo.Bounds.X < 0 || exportInfo.Bounds.Y < 0 ||
		exportInfo.Bounds.X+exportInfo.Bounds.W > 801 ||
		exportInfo.Bounds.Y+exportInfo.Bounds.H > 601 {
		t.Fatalf("Export .sql outside minimum window: %+v, %v", exportInfo, ok)
	}
	if err := tester.Click("Copy SQL"); err != nil {
		t.Fatal(err)
	}
	if copied != wantSQL || !tester.HasText("SQL copied") {
		t.Fatalf("copied bytes/feedback = %d bytes, %q", len(copied), tester.Texts())
	}
	if err := tester.Click("Export .sql"); err != nil {
		t.Fatal(err)
	}
	if exportRequest.SQL() != wantSQL ||
		exportRequest.DefaultPath() != "preview_db.sql" ||
		exportRequest.Title() != "Export database SQL" ||
		exportRequest.FilterName() != "SQL file" ||
		!workspace.preview.exporting {
		t.Fatalf("export request = %+v", exportRequest)
	}
	if button, ok := tester.AccessByRole(ui.RoleButton, "Export .sql"); !ok || !button.Disabled {
		t.Fatalf("export button during save = %+v, %v", button, ok)
	}
	if !workspace.PublishSQLExportCanceled(exportRequest) || workspace.preview.exporting {
		t.Fatal("export cancel did not finish")
	}
	if tester.HasText("SQL exported") || tester.HasText("SQL export failed") {
		t.Fatalf("cancel left feedback: %q", tester.Texts())
	}

	request, ok := workspace.BeginSQLExport(state)
	if !ok {
		t.Fatal("second export did not start")
	}
	if !workspace.PublishSQLExportFailed(request) {
		t.Fatal("export failure was not published")
	}
	tester.Frame()
	if !tester.HasText("SQL export failed") {
		t.Fatalf("export failure feedback missing: %q", tester.Texts())
	}
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !tester.HasText("SQL 导出失败") {
		t.Fatalf("Chinese export failure missing: %q", tester.Texts())
	}
	if err := tester.Click("复制 SQL"); err != nil {
		t.Fatal(err)
	}
	if copied != wantSQL || !tester.HasText("SQL 已复制") {
		t.Fatalf("localized SQL copy = %d bytes, %q", len(copied), tester.Texts())
	}
	request, ok = workspace.BeginSQLExport(state)
	if !ok || !workspace.PublishSQLExportSucceeded(request) {
		t.Fatal("export success was not published")
	}
	tester.Frame()
	if !tester.HasText("SQL 已导出") {
		t.Fatalf("export success feedback missing: %q", tester.Texts())
	}
	if request.Title() != "导出数据库 SQL" || request.FilterName() != "SQL 文件" {
		t.Fatalf("localized save dialog labels = %q, %q", request.Title(), request.FilterName())
	}

	stale, ok := workspace.BeginSQLExport(state)
	if !ok {
		t.Fatal("stale export did not start")
	}
	token := workspace.BeginOpen()
	workspace.PublishBoards(token, BuildBoardSet(previewDocument()))
	if workspace.PublishSQLExportSucceeded(stale) || workspace.preview.open {
		t.Fatal("stale export survived a model change")
	}
}

func TestDatabasePreviewActionsBlockedAndWarningOnly(t *testing.T) {
	state := newState(t, "en-US")
	warningDocument := previewDocument()
	warningDocument.Model.Code = ""
	warningDocument.Model.Name = "Preview DB"
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(warningDocument))
	tester := ui.NewTester(View(state, workspace), 800, 600)
	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Errors 0 · Warnings 1") {
		t.Fatalf("warning summary missing: %q", tester.Texts())
	}
	for _, label := range []string{"Copy SQL", "Export .sql"} {
		if info, ok := tester.AccessByRole(ui.RoleButton, label); !ok || info.Disabled {
			t.Fatalf("warning-only action %q = %+v, %v", label, info, ok)
		}
	}

	blocked := previewDocument()
	blocked.Diagnostics = []pdm.Diagnostic{{
		Code: pdm.DiagnosticExternalReference,
		Path: "/Model/RootObject/Children/object:Model/collection:Tables/object:Shortcut",
		Line: 9,
	}}
	workspace = NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(blocked))
	tester = ui.NewTester(View(state, workspace), 800, 600)
	if err := tester.Click("Database SQL"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, label := range []string{"Copy SQL", "Export .sql"} {
		if info, ok := tester.AccessByRole(ui.RoleButton, label); !ok || !info.Disabled {
			t.Fatalf("blocked action %q = %+v, %v", label, info, ok)
		}
	}
}

func TestSuggestedSQLFilename(t *testing.T) {
	document := previewDocument()
	document.Model.Code = `sales/archive:2026`
	if got := suggestedSQLFilename(document); got != "sales_archive_2026.sql" {
		t.Fatalf("suggested filename = %q", got)
	}
	document.Model.Code = "sales.sql"
	if got := suggestedSQLFilename(document); got != "sales.sql" {
		t.Fatalf("existing .sql suffix changed: %q", got)
	}
	document.Model.Code = "NUL"
	if got := suggestedSQLFilename(document); got != "_NUL.sql" {
		t.Fatalf("reserved filename was not disambiguated: %q", got)
	}
}

func previewDocument() *pdm.Document {
	yes, no := true, false
	intColumn := func(id, code string, mandatory *bool) pdm.Column {
		return pdm.Column{
			Metadata:  pdm.Metadata{ID: id, Code: code},
			DataType:  pdm.OptionalText{Value: "INT", Present: true},
			Mandatory: mandatory,
		}
	}
	customers := pdm.Table{
		Metadata: pdm.Metadata{ID: "customers", Code: "customers"},
		OwnerID:  "model",
		Columns: []pdm.Column{
			intColumn("customer-id", "id", &yes),
			intColumn("customer-code", "code", &no),
		},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "customer-pk", Code: "customer_pk"},
			ColumnIDs: []string{"customer-id"},
		}},
		PrimaryKeyID: "customer-pk",
	}
	orders := pdm.Table{
		Metadata: pdm.Metadata{ID: "orders", Code: "orders"},
		OwnerID:  "model",
		Columns: []pdm.Column{
			intColumn("order-id", "id", &yes),
			intColumn("order-customer-id", "customer_id", &no),
		},
		Keys: []pdm.Key{{
			Metadata:  pdm.Metadata{ID: "order-pk", Code: "order_pk"},
			ColumnIDs: []string{"order-id"},
		}},
		PrimaryKeyID: "order-pk",
	}
	return &pdm.Document{Model: pdm.Model{
		Metadata: pdm.Metadata{ID: "model", Code: "preview_db"},
		Contents: pdm.Contents{
			Tables: []pdm.Table{customers, orders},
			References: []pdm.Reference{{
				Metadata: pdm.Metadata{ID: "order-customer", Code: "fk_order_customer"},
				OwnerID:  "model", ChildTableID: "orders", ParentTableID: "customers",
				ParentKeyID: "customer-pk",
				Joins: []pdm.ReferenceJoin{{
					ChildColumnID: "order-customer-id", ParentColumnID: "customer-id",
				}},
			}},
		},
	}}
}

func countTextsContaining(texts []string, want string) int {
	count := 0
	for _, text := range texts {
		if strings.Contains(text, want) {
			count++
		}
	}
	return count
}
