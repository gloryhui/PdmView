package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pdmview/internal/pdm"
)

func TestUpdatePDMFileWithNoEditsLeavesFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opened.pdm")
	data := []byte("original")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(path, data, data); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged file was rewritten")
	}
	if files, err := os.ReadDir(filepath.Dir(path)); err != nil || len(files) != 1 {
		t.Fatal("unexpected staging file")
	}
}

func TestUpdatePDMFileProtectsChangedFilesAndSymlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opened.pdm")
	original := []byte("old content")
	updated := []byte("new content")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(path, original, updated); err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(path, original, []byte("stale edit")); !errors.Is(err, errPDMChanged) {
		t.Fatalf("expected stale data refusal, got %v", err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.pdm")
	if err := os.Symlink(path, alias); err == nil {
		if err := updatePDMFile(alias, updated, nil); err == nil {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, updated) {
		t.Fatal("blocked write changed contents")
	}
	missing := filepath.Join(filepath.Dir(path), "missing.pdm")
	if err := updatePDMFile(missing, original, updated); !errors.Is(err, errPDMChanged) {
		t.Fatalf("expected missing file refusal, got %v", err)
	}
}

func TestPDMSaveLocalSample(t *testing.T) {
	source := os.Getenv("PDM_LAYOUT_SAMPLE")
	if source == "" {
		t.Skip("set PDM_LAYOUT_SAMPLE to a local XML export")
	}
	original, err := loadBoardSet(source)
	if err != nil {
		t.Fatal("could not open local sample")
	}
	before := original.Prepared.Graph
	if len(before.NodeSources) < 2 {
		t.Fatal("sample needs at least two saved table symbols")
	}
	tableID := before.NodeSources[0].TableID
	table := original.Document.Raw.ByID[tableID]
	columns := table.Child(pdm.NamespaceCollection, "Columns")
	if columns == nil || len(columns.Children) == 0 {
		t.Fatal("selected table is missing fields")
	}
	columnID, _ := columns.Children[0].Attribute("Id")
	edits := []pdm.TextEdit{
		{ObjectID: tableID, Property: pdm.TextName, Value: "表 & <副本>"},
		{ObjectID: tableID, Property: pdm.TextCode, Value: "PDMVIEW_LOCAL_COPY_TABLE"},
		{ObjectID: tableID, Property: pdm.TextComment, Value: "表注释 & <本地验证>"},
		{ObjectID: columnID, Property: pdm.TextName, Value: "字段 & <副本>"},
		{ObjectID: columnID, Property: pdm.TextCode, Value: "PDMVIEW_LOCAL_COPY_COLUMN"},
		{ObjectID: columnID, Property: pdm.TextComment, Value: "字段注释 & <本地验证>"},
	}

	workingPath := filepath.Join(t.TempDir(), "working.pdm")
	if err := os.WriteFile(workingPath, original.SourceXML, 0o600); err != nil {
		t.Fatal(err)
	}
	working, err := loadBoardSet(workingPath)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := pdm.PatchTableText(working.SourceXML, tableID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(working.SourcePath, working.SourceXML, unchanged); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(workingPath); err != nil || !bytes.Equal(data, original.SourceXML) {
		t.Fatal("unedited save differs from source bytes")
	}

	changed, err := pdm.PatchTableText(working.SourceXML, tableID, edits)
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(working.SourcePath, working.SourceXML, changed); err != nil {
		t.Fatal(err)
	}
	reopened, err := loadBoardSet(workingPath)
	if err != nil {
		t.Fatal("overwritten PDM did not reopen")
	}
	secondTableID := before.NodeSources[1].TableID
	secondEdit := pdm.TextEdit{ObjectID: secondTableID, Property: pdm.TextComment, Value: "另一张表 & <继续保存>"}
	secondData, err := pdm.PatchTableText(reopened.SourceXML, secondTableID, []pdm.TextEdit{secondEdit})
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePDMFile(reopened.SourcePath, reopened.SourceXML, secondData); err != nil {
		t.Fatal(err)
	}
	reopened, err = loadBoardSet(workingPath)
	if err != nil {
		t.Fatal("overwritten PDM did not reopen")
	}
	edits = append(edits, secondEdit)
	for _, edit := range edits {
		object := reopened.Document.Raw.ByID[edit.ObjectID]
		if value, present := object.Property(string(edit.Property)); !present || value != edit.Value {
			t.Fatalf("reopened %s differs", edit.Property)
		}
	}

	after := reopened.Prepared.Graph
	if !reflect.DeepEqual(reopened.Document.Diagnostics, original.Document.Diagnostics) ||
		!reflect.DeepEqual(reopened.Document.Raw.Format, original.Document.Raw.Format) ||
		len(reopened.Document.Raw.ByID) != len(original.Document.Raw.ByID) ||
		len(original.Boards) != len(reopened.Boards) || original.Default != reopened.Default ||
		before.DiagramID != after.DiagramID ||
		!reflect.DeepEqual(before.NodeSources, after.NodeSources) ||
		!reflect.DeepEqual(before.EdgeSources, after.EdgeSources) ||
		before.Scene.Bounds != after.Scene.Bounds || before.AutoLayout != after.AutoLayout {
		t.Fatal("model metadata or diagram geometry changed")
	}
	for index, oldNode := range before.Scene.Nodes {
		newNode := after.Scene.Nodes[index]
		if oldNode.Bounds != newNode.Bounds || len(oldNode.Fields) != len(newNode.Fields) {
			t.Fatalf("table symbol %d changed bounds or displayed field count", index)
		}
		if (oldNode.Presentation == nil) != (newNode.Presentation == nil) ||
			(oldNode.Presentation != nil && !reflect.DeepEqual(oldNode.Presentation.Style, newNode.Presentation.Style)) {
			t.Fatalf("table symbol %d changed style", index)
		}
	}
	if data, err := os.ReadFile(source); err != nil || !bytes.Equal(data, original.SourceXML) {
		t.Fatal("local source changed")
	}
	if entries, err := os.ReadDir(filepath.Dir(workingPath)); err != nil || len(entries) != 1 {
		t.Fatal("save left another file beside the opened PDM")
	}
	t.Logf("unedited save is byte-identical (%d bytes), %d edits across two tables reopened from one overwritten PDM, %d boards and %d table symbols preserved",
		len(original.SourceXML), len(edits), len(reopened.Boards), len(after.Scene.Nodes))
}
