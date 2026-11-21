package appui

import (
	"reflect"
	"testing"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func TestPDMCopyCollectsOnlySupportedEdits(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	set := BuildBoardSet(tablePropertiesDocument())
	set.SourcePath = "/workspace/orders.pdm"
	workspace.PublishBoards(workspace.BeginOpen(), set)
	workspace.OpenTableProperties("orders")
	var request *PDMCopyRequest
	workspace.SetPDMCopyStarter(func(next *PDMCopyRequest) { request = next })
	tester := ui.NewTester(workspace.TablePropertiesView(state), 640, 480)
	tester.Frame()
	if err := tester.Click("Save PDM Copy…"); err != nil {
		t.Fatal(err)
	}
	if !workspace.PDMCopyCurrent(request) || len(request.edits) != 0 {
		t.Fatal("opening the table produced edits")
	}
	workspace.PublishPDMCopyResult(request, "")

	draft := &workspace.tableProperties.draft
	draft.General.Name = "Renamed table"
	draft.General.Package = "Different package"
	draft.Columns[0].Code = "NEW_CODE"
	draft.Columns[0].Comment = "New comment"
	draft.Columns[0].DataType = "text"
	draft.Columns[0].PrimaryKey = !draft.Columns[0].PrimaryKey
	draft.Indexes[0].Name = "Edited index"
	draft.Keys[0].Code = "EDITED_KEY"
	tester.Frame()
	if err := tester.Click("Save PDM Copy…"); err != nil {
		t.Fatal(err)
	}
	expected := []pdm.TextEdit{
		{ObjectID: "orders", Property: pdm.TextName, Value: "Renamed table"},
		{ObjectID: draft.Columns[0].ID, Property: pdm.TextCode, Value: "NEW_CODE"},
		{ObjectID: draft.Columns[0].ID, Property: pdm.TextComment, Value: "New comment"},
	}
	if !workspace.PDMCopyCurrent(request) || !reflect.DeepEqual(request.edits, expected) {
		t.Fatalf("unexpected copy edits: %#v", request.edits)
	}
	workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopySaved)
	tester.Frame()
	if !tester.HasText("PDM copy saved") {
		t.Fatal("missing save confirmation")
	}
	workspace.CloseTableProperties()
	if workspace.PDMCopyCurrent(request) {
		t.Fatal("closed properties accepted a stale result")
	}
}
