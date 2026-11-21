package appui

import (
	"errors"
	"strings"
	"testing"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	"pdmview/internal/sqlgen"

	"github.com/egoist/mygo/ui"
)

func TestTableCopyTextCompleteOrderedEscapedAndLocaleIndependent(t *testing.T) {
	if CopyTableKey != "pdmview.workspace.copy-table" ||
		CopyStatusKey != "pdmview.workspace.copy-status" {
		t.Fatal("stable copy keys changed")
	}
	detail := copyTestDetail()
	text := tableCopyText(detail)
	required := []string{
		`PDM Table`,
		`id: "table-id"`,
		`object_id: "object-id"`,
		`name: "客户 \"A\"\nB\\C"`,
		`code: "customer_table"`,
		`comment: "line one\nline two"`,
		`package_path:`,
		`  - id="package-id", name="Package \"One\"\n", code="pkg", resolved=true`,
		`columns:`,
		`  - position: 1`,
		`    id: "column-id"`,
		`    name: "编号\n一"`,
		`    code: "ID"`,
		`    comment: "第一行\n第二行"`,
		`    type: "VARCHAR(10)"`,
		`    length: ""`,
		`    precision: absent`,
		`    scale: "2"`,
		`    nullable: false`,
		`    default: "a\\b"`,
		`    identity: false`,
		`    pk: true`,
		`    fk: true`,
		`  - position: 2`,
		`    id: "column-note"`,
		`    name: "备注"`,
		`    code: "note"`,
		`    comment: absent`,
		`    type: absent`,
		`    length: absent`,
		`    precision: ""`,
		`    scale: absent`,
		`    nullable: absent`,
		`    default: ""`,
		`    identity: absent`,
		`    pk: false`,
		`    fk: false`,
		`primary_key: id="key-primary", name="Primary\nkey", code="PK", resolved=true`,
		`keys:`,
		`  - position: 1`,
		`    primary: true`,
		`    id: "key-primary"`,
		`    columns:`,
		`      - id="column-note", name="备注", code="note", resolved=true`,
		`      - id="column-id", name="编号\n一", code="ID", resolved=true`,
		`  - position: 2`,
		`    primary: false`,
		`    id: "key-secondary"`,
		`    comment: ""`,
		`    columns: []`,
		`indexes:`,
		`  - position: 1`,
		`    id: "index-first"`,
		`    comment: "index\ncomment"`,
		`    unique: false`,
		`    columns:`,
		`      - position: 1`,
		`        id: "index-column-id"`,
		`        comment: "sort\ncomment"`,
		`        column: id="column-id", name="编号\n一", code="ID", resolved=true`,
		`        sort: ""`,
		`  - position: 2`,
		`    id: "index-second"`,
		`    unique: absent`,
		`    columns:`,
		`      - position: 1`,
		`        column: id="column-note", name="备注", code="note", resolved=true`,
		`        sort: "DESC"`,
		`foreign_keys:`,
		`  - position: 1`,
		`    id: "foreign-key-id"`,
		`    comment: "foreign key\ncomment"`,
		`    parent_table: id="parent-table", name="Parent", code="PARENT", resolved=true`,
		`    child_table: id="table-id", name="客户 \"A\"\nB\\C", code="customer_table", resolved=true`,
		`    parent_key: id="parent-key", name="Parent key", code="PK_PARENT", resolved=true`,
		`    update_rule: "CASCADE"`,
		`    delete_rule: absent`,
		`    cardinality: ""`,
		`    joins:`,
		`      - position: 1`,
		`        id: "join-first"`,
		`        comment: "join\ncomment"`,
		`        child_column: id="column-note", name="备注", code="note", resolved=true`,
		`        parent_column: id="parent-note", name="Parent note", code="pnote", resolved=true`,
		`      - position: 2`,
		`        id: "join-second"`,
		`        child_column: id="column-id", name="编号\n一", code="ID", resolved=true`,
		`        parent_column: id="unresolved-parent", name="", code="", resolved=false`,
	}
	mustContainInOrder(t, text, required)
	if tableCopyText(detail) != text {
		t.Fatal("table copy is not deterministic")
	}
}

func TestTableCopyShowsOnlyForRealSelectedTableAndClearsStaleFeedback(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(detailDocument()))
	var written string
	failure := errors.New("clipboard unavailable")
	workspace.SetClipboardWriter(func(text string) error {
		written = text
		return failure
	})
	tester := ui.NewTester(View(state, workspace), 800, 600)

	if tester.HasText("Copy table info") {
		t.Fatal("copy action showed without a selected table")
	}
	workspace.LocateTable("customers")
	tester.Frame()
	button, ok := tester.AccessByRole(ui.RoleButton, "Copy table info")
	if !ok || button.Bounds.X < 0 || button.Bounds.Y < 0 {
		t.Fatalf("copy button = %+v, %v", button, ok)
	}
	if workspace.detailsScroll.MaxY <= 0 {
		t.Fatal("detail content is not scrollable")
	}
	workspace.detailsScroll.ScrollTo(0, workspace.detailsScroll.MaxY, ui.Animation{})
	tester.Frame()
	scrolledButton, ok := tester.AccessByRole(ui.RoleButton, "Copy table info")
	if !ok || scrolledButton.Bounds != button.Bounds {
		t.Fatalf("copy action moved with details scrolling: before %+v after %+v", button.Bounds, scrolledButton.Bounds)
	}
	if err := tester.Click("Copy table info"); err != nil {
		t.Fatal(err)
	}
	if written != tableCopyText(workspace.tableDetail) {
		t.Fatal("clipboard writer did not receive the complete table text")
	}
	if !tester.HasText("Copy failed") {
		t.Fatalf("copy failure feedback missing: %q", tester.Texts())
	}
	if alert, ok := tester.AccessByRole(ui.RoleAlert, "Copy failed"); !ok || alert.Bounds.Y < 0 {
		t.Fatalf("failure feedback is not accessible: %+v, %v", alert, ok)
	}

	workspace.LocateTable("package-child-table")
	tester.Frame()
	if tester.HasText("Copy failed") {
		t.Fatal("copy feedback survived a selection change")
	}
	if !tester.HasText("Copy table info") || !tester.HasText("Not on a diagram") {
		t.Fatalf("unplaced table lost its copy action: %q", tester.Texts())
	}
	workspace.SetClipboardWriter(func(text string) error {
		written = text
		return nil
	})
	if err := tester.Click("Copy table info"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("Copied") {
		t.Fatalf("copy success feedback missing: %q", tester.Texts())
	}
}

func TestTableCopyBytesStayLocaleInvariant(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(detailDocument()))
	var writes []string
	workspace.SetClipboardWriter(func(text string) error {
		writes = append(writes, text)
		return nil
	})
	tester := ui.NewTester(View(state, workspace), 800, 600)
	workspace.LocateTable("customers")
	tester.Frame()

	if err := tester.Click("Copy table info"); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || !tester.HasText("Copied") {
		t.Fatalf("English copy = %d writes, texts %q", len(writes), tester.Texts())
	}
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !tester.HasText("复制表信息") || !tester.HasText("已复制") {
		t.Fatalf("localized copy controls/status missing: %q", tester.Texts())
	}
	if err := tester.Click("复制表信息"); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 {
		t.Fatalf("localized copy writes = %d, want 2", len(writes))
	}
	if writes[0] != writes[1] {
		t.Fatal("localized UI changed the clipboard bytes")
	}
	if strings.Contains(writes[1], "复制") || strings.Contains(writes[1], "已") {
		t.Fatal("localized UI text leaked into the copied table")
	}
}

func TestCopyCreateTableUsesFrozenTableAndReportsBlocker(t *testing.T) {
	state := newState(t, "en-US")
	document := previewDocument()
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(document))
	var written string
	workspace.SetClipboardWriter(func(text string) error {
		written = text
		return nil
	})
	tester := ui.NewTester(View(state, workspace), 800, 600)
	workspace.LocateTable("customers")
	tester.Frame()

	info, ok := tester.AccessByRole(ui.RoleButton, "Copy CREATE TABLE")
	if !ok || info.Bounds.X < 0 || info.Bounds.Y < 0 {
		t.Fatalf("copy CREATE TABLE button = %+v, %v", info, ok)
	}
	table := workspace.detailIndex.tables["customers"]
	if table == nil {
		t.Fatal("frozen table is missing")
	}
	want := sqlgen.GenerateTable(*table)
	if want.HasErrors() {
		t.Fatalf("fixture CREATE TABLE errors: %+v", want.Diagnostics)
	}
	if err := tester.Click("Copy CREATE TABLE"); err != nil {
		t.Fatal(err)
	}
	if written != want.SQL || !tester.HasText("Copied") {
		t.Fatalf("copied CREATE TABLE = %q words=%q", written, tester.Texts())
	}
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	if !tester.HasText("复制建表 SQL") {
		t.Fatalf("localized CREATE copy missing: %q", tester.Texts())
	}

	blocked := previewDocument()
	blocked.Model.Contents.Tables[0].Columns[1].DataType =
		pdm.OptionalText{Value: "NUMBER", Present: true}
	workspace = NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(blocked))
	written = ""
	tester = ui.NewTester(View(state, workspace), 800, 600)
	workspace.LocateTable("customers")
	tester.Frame()
	if err := tester.Click("复制建表 SQL"); err != nil {
		t.Fatal(err)
	}
	if written != "" {
		t.Fatal("blocked CREATE TABLE was copied")
	}
	if !tester.HasText("建表 SQL 复制失败：字段与类型（unknown-column-type）") {
		t.Fatalf("blocked CREATE feedback missing: %q", tester.Texts())
	}
}

func copyTestDetail() TableDetail {
	yes, no := true, false
	text := func(value string) pdm.OptionalText {
		return pdm.OptionalText{Value: value, Present: true}
	}
	return TableDetail{
		Metadata: pdm.Metadata{
			ID:       "table-id",
			ObjectID: "object-id",
			Name:     "客户 \"A\"\nB\\C",
			Code:     "customer_table",
			Comment:  text("line one\nline two"),
		},
		PrimaryKeyID: "key-primary",
		PackagePath: []DetailRef{{
			ID: "package-id", Name: "Package \"One\"\n", Code: "pkg", Resolved: true,
		}},
		Columns: []ColumnDetail{
			{
				Column: pdm.Column{
					Metadata: pdm.Metadata{
						ID: "column-id", Name: "编号\n一", Code: "ID",
						Comment: text("第一行\n第二行"),
					},
					DataType:     text("VARCHAR(10)"),
					Length:       text(""),
					Scale:        text("2"),
					DefaultValue: text("a\\b"),
					Mandatory:    &yes,
					Identity:     &no,
				},
				Nullable: &no, PrimaryKey: true, ForeignKey: true,
			},
			{
				Column: pdm.Column{
					Metadata:     pdm.Metadata{ID: "column-note", Name: "备注", Code: "note"},
					Precision:    text(""),
					DefaultValue: text(""),
				},
			},
		},
		Keys: []KeyDetail{
			{
				Metadata: pdm.Metadata{ID: "key-primary", Name: "Primary\nkey", Code: "PK"},
				Primary:  true,
				Columns: []DetailRef{
					{ID: "column-note", Name: "备注", Code: "note", Resolved: true},
					{ID: "column-id", Name: "编号\n一", Code: "ID", Resolved: true},
				},
			},
			{
				Metadata: pdm.Metadata{ID: "key-secondary", Name: "Secondary", Code: "SK", Comment: text("")},
			},
		},
		Indexes: []IndexDetail{
			{
				Metadata: pdm.Metadata{ID: "index-first", Name: "First", Code: "IX_FIRST", Comment: text("index\ncomment")},
				Unique:   &no,
				Columns: []IndexColumnDetail{{
					Metadata:  pdm.Metadata{ID: "index-column-id", Comment: text("sort\ncomment")},
					Column:    DetailRef{ID: "column-id", Name: "编号\n一", Code: "ID", Resolved: true},
					SortOrder: text(""),
				}},
			},
			{
				Metadata: pdm.Metadata{ID: "index-second", Name: "Second", Code: "IX_SECOND"},
				Columns: []IndexColumnDetail{{
					Column:    DetailRef{ID: "column-note", Name: "备注", Code: "note", Resolved: true},
					SortOrder: text("DESC"),
				}},
			},
		},
		ForeignKeys: []ForeignKeyDetail{{
			Metadata:         pdm.Metadata{ID: "foreign-key-id", Name: "Customer order", Code: "FK_ORDER", Comment: text("foreign key\ncomment")},
			ParentTable:      DetailRef{ID: "parent-table", Name: "Parent", Code: "PARENT", Resolved: true},
			ChildTable:       DetailRef{ID: "table-id", Name: "客户 \"A\"\nB\\C", Code: "customer_table", Resolved: true},
			ParentKey:        DetailRef{ID: "parent-key", Name: "Parent key", Code: "PK_PARENT", Resolved: true},
			UpdateConstraint: text("CASCADE"),
			Cardinality:      text(""),
			Joins: []JoinDetail{
				{
					Metadata:     pdm.Metadata{ID: "join-first", Comment: text("join\ncomment")},
					ChildColumn:  DetailRef{ID: "column-note", Name: "备注", Code: "note", Resolved: true},
					ParentColumn: DetailRef{ID: "parent-note", Name: "Parent note", Code: "pnote", Resolved: true},
				},
				{
					Metadata:     pdm.Metadata{ID: "join-second"},
					ChildColumn:  DetailRef{ID: "column-id", Name: "编号\n一", Code: "ID", Resolved: true},
					ParentColumn: DetailRef{ID: "unresolved-parent"},
				},
			},
		}},
	}
}

func mustContainInOrder(t *testing.T, text string, required []string) {
	t.Helper()
	offset := 0
	for _, part := range required {
		index := strings.Index(text[offset:], part)
		if index < 0 {
			t.Fatalf("copy text is missing %q after byte %d:\n%s", part, offset, text)
		}
		offset += index + len(part)
	}
}
