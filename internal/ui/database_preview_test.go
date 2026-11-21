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
