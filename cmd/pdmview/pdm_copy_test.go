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

func TestWriteNewPDMCopyRefusesExistingDestinations(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.pdm")
	copyPath := filepath.Join(dir, "copy.pdm")
	alias := filepath.Join(dir, "alias.pdm")
	if err := os.WriteFile(source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, alias); err != nil {
		t.Fatal(err)
	}
	data := []byte("new")
	if err := writeNewPDMCopy(copyPath, source, data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, alias, copyPath} {
		if err := writeNewPDMCopy(path, source, []byte("overwrite")); !errors.Is(err, os.ErrExist) {
			t.Fatalf("expected refusal for %s, got %v", filepath.Base(path), err)
		}
	}
	if got, err := os.ReadFile(source); err != nil || string(got) != "original" {
		t.Fatal("source changed")
	}
	if got, err := os.ReadFile(copyPath); err != nil || !bytes.Equal(got, data) {
		t.Fatal("copy changed")
	}
	if files, err := os.ReadDir(dir); err != nil || len(files) != 3 {
		t.Fatal("unexpected staging file")
	}
}

func TestUpdatePDMCopyProtectsChangedFilesAndSymlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "copy.pdm")
	original := []byte("old content")
	updated := []byte("new content")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := updatePDMCopy(path, original, updated); err != nil {
		t.Fatal(err)
	}
	if err := updatePDMCopy(path, original, []byte("stale edit")); !errors.Is(err, errPDMCopyChanged) {
		t.Fatalf("expected stale data refusal, got %v", err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.pdm")
	if err := os.Symlink(path, alias); err == nil {
		if err := updatePDMCopy(alias, updated, nil); !errors.Is(err, errPDMCopyChanged) {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, updated) {
		t.Fatal("blocked write changed contents")
	}
}

func TestPDMCopyLocalSample(t *testing.T) {
	source := os.Getenv("PDM_LAYOUT_SAMPLE")
	if source == "" {
		t.Skip("set PDM_LAYOUT_SAMPLE to a local XML export")
	}
	original, err := loadBoardSet(source)
	if err != nil {
		t.Fatal("could not open local sample")
	}
	before := original.Prepared.Graph
	if len(before.NodeSources) == 0 {
		t.Fatal("sample is missing saved table symbols")
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

	dir := t.TempDir()
	unchanged, err := pdm.PatchTableText(original.SourceXML, tableID, nil)
	if err != nil {
		t.Fatal(err)
	}
	unchangedPath := filepath.Join(dir, "unchanged.pdm")
	if err := writeNewPDMCopy(unchangedPath, original.SourcePath, unchanged); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(unchangedPath); err != nil || !bytes.Equal(data, original.SourceXML) {
		t.Fatal("unedited copy differs from source bytes")
	}

	changed, err := pdm.PatchTableText(original.SourceXML, tableID, edits)
	if err != nil {
		t.Fatal(err)
	}
	changedPath := filepath.Join(dir, "changed.pdm")
	if err := writeNewPDMCopy(changedPath, original.SourcePath, changed); err != nil {
		t.Fatal(err)
	}
	changedPath, err = canonicalCopyPath(changedPath)
	if err != nil {
		t.Fatal(err)
	}
	store := newPDMCopyStore(filepath.Join(dir, "config", "preferences.json"))
	if err := store.remember(changedPath, changed); err != nil {
		t.Fatal(err)
	}
	originalAgain, err := loadPDMSource(source, store)
	if err != nil || originalAgain.WritableCopy {
		t.Fatal("original file unexpectedly became writable")
	}
	reopened, err := loadPDMSource(changedPath, store)
	if err != nil || !reopened.WritableCopy {
		t.Fatal("saved copy did not reopen for editing")
	}
	secondTableID := before.NodeSources[1].TableID
	secondEdit := pdm.TextEdit{ObjectID: secondTableID, Property: pdm.TextComment, Value: "另一张表 & <继续保存>"}
	secondData, err := pdm.PatchTableText(reopened.SourceXML, secondTableID, []pdm.TextEdit{secondEdit})
	if err != nil {
		t.Fatal(err)
	}
	if err := updatePDMCopy(changedPath, reopened.SourceXML, secondData); err != nil {
		t.Fatal(err)
	}
	external, err := loadPDMSource(changedPath, store)
	if err != nil || external.WritableCopy {
		t.Fatal("unrecorded on-disk contents unexpectedly became writable")
	}
	if err := store.remember(changedPath, secondData); err != nil {
		t.Fatal(err)
	}
	reopened, err = loadPDMSource(changedPath, store)
	if err != nil || !reopened.WritableCopy {
		t.Fatal("updated copy did not reopen for editing")
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
	t.Logf("byte-identical unedited copy (%d bytes), %d edits across two tables reopened from one updated copy, %d boards and %d table symbols preserved",
		len(original.SourceXML), len(edits), len(reopened.Boards), len(after.Scene.Nodes))
}
