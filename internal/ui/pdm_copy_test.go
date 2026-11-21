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
	workspace.PublishPDMCopyResult(request, "", nil)

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
	workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopySaved, nil)
	tester.Frame()
	if !tester.HasText("PDM copy saved") {
		t.Fatal("missing save confirmation")
	}
	workspace.CloseTableProperties()
	if workspace.PDMCopyCurrent(request) {
		t.Fatal("closed properties accepted a stale result")
	}
}

func TestPDMCopyAdoptsSavedModelForFurtherEdits(t *testing.T) {
	state := newState(t, "en-US")
	workspace := NewWorkspace()
	data := []byte(`<Model xmlns:a="attribute" xmlns:o="object" xmlns:c="collection">
<o:RootObject><c:Children><o:Model Id="model"><c:Tables>
<o:Table Id="one"><a:Name>First</a:Name></o:Table>
<o:Table Id="two"><a:Name>Second</a:Name></o:Table>
</c:Tables></o:Model></c:Children></o:RootObject></Model>`)
	setup := &PDMCopyRequest{}
	original, err := setup.Prepare("/workspace/original.pdm", data)
	if err != nil {
		t.Fatal(err)
	}
	original.WritableCopy = false
	workspace.PublishBoards(workspace.BeginOpen(), original)
	workspace.OpenTableProperties("one")
	workspace.LocateTable("one")
	camera := workspace.canvas.camera

	var request *PDMCopyRequest
	workspace.SetPDMCopyStarter(func(next *PDMCopyRequest) { request = next })
	tester := ui.NewTester(workspace.TablePropertiesView(state), 640, 480)
	tester.Frame()
	workspace.tableProperties.draft.General.Name = "First saved"
	if err := tester.Click("Save PDM Copy…"); err != nil {
		t.Fatal(err)
	}
	if request.WritableCopy() {
		t.Fatal("original is writable")
	}
	updated, err := request.Run()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := request.Prepare("/workspace/copy.pdm", updated)
	if err != nil {
		t.Fatal(err)
	}
	workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopySaved, &saved)
	if workspace.tableDetail.Name != "First saved" || workspace.canvas.camera != camera {
		t.Fatal("saved model did not refresh while preserving the camera")
	}
	workspace.OpenTableProperties("two")
	workspace.tableProperties.draft.General.Name = "Second saved"
	tester.Frame()
	if err := tester.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if !request.WritableCopy() || request.SourcePath() != saved.SourcePath {
		t.Fatal("second edit does not target the existing copy")
	}
	updated, err = request.Run()
	if err != nil {
		t.Fatal(err)
	}
	saved, err = request.Prepare(request.SourcePath(), updated)
	if err != nil {
		t.Fatal(err)
	}
	workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopySaved, &saved)
	for id, expected := range map[string]string{"one": "First saved", "two": "Second saved"} {
		if value, _ := workspace.document.Raw.ByID[id].Property("Name"); value != expected {
			t.Fatalf("cumulative save lost table %s", id)
		}
	}
}
