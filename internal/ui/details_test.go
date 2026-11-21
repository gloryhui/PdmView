package appui

import (
	"testing"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestTableDetailPreservesColumnAndConstraintOrder(t *testing.T) {
	document := detailDocument()
	index := buildTableDetailIndex(&document.Model)
	detail, ok := index.resolve("customers")
	if !ok || detail.Code != "customer_record" || detail.Comment.Value != "客户档案" ||
		len(detail.PackagePath) != 1 || detail.PackagePath[0].ID != "package-a" {
		t.Fatalf("table detail = %+v, %v", detail, ok)
	}
	first, second, third := detail.Columns[0], detail.Columns[1], detail.Columns[2]
	if first.ID != "customer-id" || second.ID != "customer-region" || third.ID != "customer-note" ||
		first.Nullable == nil || *first.Nullable || second.Nullable == nil || !*second.Nullable ||
		third.Nullable != nil || first.Identity == nil || !*first.Identity ||
		second.Identity == nil || *second.Identity || third.Identity != nil ||
		first.DefaultValue.Present || !third.DefaultValue.Present || third.DefaultValue.Value != "" ||
		second.DataType.Value != "DECIMAL(12,2)" || second.Length.Value != "12" ||
		second.Precision.Value != "12" || second.Scale.Value != "2" ||
		!first.PrimaryKey || !second.PrimaryKey || third.PrimaryKey ||
		!first.ForeignKey || !second.ForeignKey || third.ForeignKey {
		t.Fatalf("column details = %+v", detail.Columns)
	}
	key := detail.Keys[0]
	if !key.Primary || key.Columns[0].ID != "customer-region" || key.Columns[1].ID != "customer-id" {
		t.Fatalf("primary key sequence = %+v", key)
	}
	order := detail.Indexes[0]
	if order.Unique == nil || !*order.Unique || order.Columns[0].Column.ID != "customer-id" ||
		order.Columns[0].SortOrder.Value != "DESC" || order.Columns[1].Column.ID != "customer-region" ||
		order.Columns[1].SortOrder.Value != "ASC" {
		t.Fatalf("index sequence = %+v", order)
	}
	foreign := detail.ForeignKeys[0]
	if foreign.ParentTable.Code != "order_record" || !foreign.ParentKey.Resolved ||
		foreign.Joins[0].ChildColumn.Code != "region" || foreign.Joins[0].ParentColumn.Code != "area" ||
		foreign.Joins[1].ChildColumn.Code != "id" || foreign.Joins[1].ParentColumn.Code != "order_id" ||
		foreign.Joins[2].ParentColumn.Resolved || foreign.Joins[2].ParentColumn.ID != "customer-id" {
		t.Fatalf("cross-package foreign-key sequence = %+v", foreign)
	}
}

func TestTableDetailsUseNativeScrollAndKeepDataOnLanguageSwitch(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	workspace.PublishBoards(workspace.BeginOpen(), BuildBoardSet(detailDocument()))
	tester := ui.NewTester(View(state, workspace), 800, 600)
	workspace.LocateTable("customers")
	tester.Frame()
	for _, label := range []string{"Columns", "Foreign keys", "客户档案", "customer_record", "DECIMAL(12,2)", `""`} {
		if !tester.HasText(label) {
			t.Errorf("missing English detail %q", label)
		}
	}
	state.SelectLanguage(i18n.ZHCN)
	tester.Frame()
	for _, label := range []string{"字段", "外键", "客户档案", "customer_record", "DECIMAL(12,2)", `""`} {
		if !tester.HasText(label) {
			t.Errorf("missing Chinese detail %q", label)
		}
	}
	if workspace.tableDetail.ID != "customers" || workspace.detailsScroll.MaxY <= 0 {
		t.Fatal("locale change lost selected table or detail scroll range")
	}
	workspace.LocateTable("package-child-table")
	tester.Frame()
	if workspace.tableDetail.ID != "package-child-table" || workspace.detailsScroll.Y != 0 {
		t.Fatal("selecting another table retained the previous detail view")
	}
}

func detailDocument() *pdm.Document {
	document := modelNavDocument()
	packageA := &document.Model.Contents.Packages[0]
	child := &packageA.Contents.Tables[0]
	parent := &document.Model.Contents.Packages[1].Contents.Tables[0]
	yes, no := true, false
	text := func(value string) pdm.OptionalText { return pdm.OptionalText{Value: value, Present: true} }
	child.Code, child.Comment = "customer_record", text("客户档案")
	child.Columns = []pdm.Column{
		{Metadata: pdm.Metadata{ID: "customer-id", Name: "编号", Code: "id"}, Mandatory: &yes, Identity: &yes},
		{Metadata: pdm.Metadata{ID: "customer-region", Name: "地区", Code: "region"},
			DataType: text("DECIMAL(12,2)"), Length: text("12"), Precision: text("12"), Scale: text("2"),
			Mandatory: &no, Identity: &no},
		{Metadata: pdm.Metadata{ID: "customer-note", Name: "备注", Code: "note"}, DefaultValue: text("")},
	}
	child.Keys = []pdm.Key{{
		Metadata:  pdm.Metadata{ID: "customer-pk", Name: "Customer key"},
		ColumnIDs: []string{"customer-region", "customer-id"},
	}}
	child.PrimaryKeyID = "customer-pk"
	child.Indexes = []pdm.Index{{
		Metadata: pdm.Metadata{ID: "customer-idx", Name: "Customer index"},
		Unique:   &yes,
		Columns: []pdm.IndexColumn{
			{ColumnID: "customer-id", SortOrder: text("DESC")},
			{ColumnID: "customer-region", SortOrder: text("ASC")},
		},
	}}
	parent.Code = "order_record"
	parent.Columns = []pdm.Column{
		{Metadata: pdm.Metadata{ID: "order-id", Name: "订单编号", Code: "order_id"}},
		{Metadata: pdm.Metadata{ID: "order-area", Name: "订单地区", Code: "area"}},
	}
	parent.Keys = []pdm.Key{{Metadata: pdm.Metadata{ID: "order-key"},
		ColumnIDs: []string{"order-area", "order-id"}}}
	document.Model.Contents.References = []pdm.Reference{{
		Metadata:      pdm.Metadata{ID: "customer-order-fk", Name: "Customer order"},
		ParentTableID: parent.ID,
		ChildTableID:  child.ID,
		ParentKeyID:   "order-key",
		Joins: []pdm.ReferenceJoin{
			{ParentColumnID: "order-area", ChildColumnID: "customer-region"},
			{ParentColumnID: "order-id", ChildColumnID: "customer-id"},
			{ParentColumnID: "customer-id"},
		},
	}}
	return document
}
