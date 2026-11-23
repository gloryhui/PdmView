package main

import (
	"os"
	"path/filepath"
	"testing"

	"pdmview/internal/app"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo/ui"
)

type fakeWindow struct {
	destroyed bool
}

func (w fakeWindow) IsDestroyed() bool {
	return w.destroyed
}

func TestWindowOptions(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	opts := windowOptions(state, appui.NewWorkspace())

	if opts.Title != appName {
		t.Fatalf("title = %q, want %q", opts.Title, appName)
	}
	if opts.Width != 1280 || opts.Height != 900 {
		t.Fatalf("initial size = %dx%d, want 1280x900", opts.Width, opts.Height)
	}
	if opts.MinWidth != minWidth || opts.MinHeight != minHeight {
		t.Fatalf("minimum size = %dx%d, want %dx%d", opts.MinWidth, opts.MinHeight, minWidth, minHeight)
	}
	if !opts.UseContentSize {
		t.Fatal("UseContentSize is false")
	}
	if opts.Content == nil {
		t.Fatal("WindowOptions.Content is nil")
	}
}

func TestTablePropertiesSizeUsesLiveParentAndClampsMinimum(t *testing.T) {
	tests := []struct {
		name                      string
		parentWidth, parentHeight int
		wantWidth, wantHeight     int
	}{
		{name: "three quarters", parentWidth: 1600, parentHeight: 1000, wantWidth: 1200, wantHeight: 750},
		{name: "minimum clamp", parentWidth: 800, parentHeight: 600, wantWidth: 640, wantHeight: 480},
		{name: "fallback", parentWidth: 0, parentHeight: 0, wantWidth: 960, wantHeight: 675},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height := tablePropertiesSize(test.parentWidth, test.parentHeight)
			if width != test.wantWidth || height != test.wantHeight {
				t.Fatalf("size = %dx%d, want %dx%d", width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

func TestMinimumViewShowsLocalizedWorkspace(t *testing.T) {
	state := app.New(app.NewStore(filepath.Join(t.TempDir(), "PdmView", "preferences.json")), "en-US")
	workspace := appui.NewWorkspace()
	tt := ui.NewTester(appui.View(state, workspace), 800, 600)

	if !tt.HasText("Interface language: English") ||
		!tt.HasText("Navigation") || !tt.HasText("Diagram canvas") || !tt.HasText("Details") {
		t.Fatalf("English minimum view missing; texts=%q", tt.Texts())
	}

	state.SelectLanguage("zh-CN")
	tt.Frame()
	if !tt.HasText("当前界面语言：简体中文。语言偏好已保存。") || !tt.HasText("导航") {
		t.Fatalf("Chinese status missing; texts=%q", tt.Texts())
	}
	if appui.RootKey == "" || appui.StatusKey == "" {
		t.Fatal("stable view keys are empty")
	}
}

func TestOptionalPositionalPDMPath(t *testing.T) {
	if _, ok := optionalPDMPath(nil); ok {
		t.Fatal("empty args selected a path")
	}
	if _, ok := optionalPDMPath([]string{"-style"}); ok {
		t.Fatal("flag selected as a path")
	}
	if _, ok := optionalPDMPath([]string{"one.pdm", "two.pdm"}); ok {
		t.Fatal("multiple args selected a path")
	}
	if got, ok := optionalPDMPath([]string{"one.pdm"}); !ok || got != "one.pdm" {
		t.Fatalf("positional path = %q, %v", got, ok)
	}
}

func TestApplyBoardResultGuardsStaleDestroyedAndFailure(t *testing.T) {
	workspace := appui.NewWorkspace()
	stale := workspace.BeginOpen()
	current := workspace.BeginOpen()
	set := appui.BuildBoardSet(nil)
	initialBoards, initialActive := workspace.BoardCount(), workspace.ActiveBoard()
	if applyBoardResult(fakeWindow{}, workspace, stale, set, nil) {
		t.Fatal("stale load result was published")
	}
	if workspace.BoardCount() != initialBoards || workspace.ActiveBoard() != initialActive {
		t.Fatal("stale result changed the active board set")
	}
	if applyBoardResult(fakeWindow{destroyed: true}, workspace, current, set, nil) {
		t.Fatal("destroyed-window result was published")
	}
	if !applyBoardResult(fakeWindow{}, workspace, current, set, nil) {
		t.Fatal("current load result was rejected")
	}
	before := workspace.ActiveBoard()
	workspace.ReportOpenError()
	if workspace.ActiveBoard() != before || workspace.BoardCount() != 1 {
		t.Fatal("open error replaced the active board")
	}
}

func TestLoadBoardSetRejectsDirectoryBeforeReading(t *testing.T) {
	if _, err := loadBoardSet(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as a PDM file")
	}
	if info, err := os.Stat(t.TempDir()); err != nil || !info.IsDir() {
		t.Fatal("temporary directory check failed")
	}
}

func TestWriteExportedSQLExactBytesSuffixAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	text := "CREATE TABLE `客户` (`id` INT);\n"
	path := filepath.Join(dir, "database.SQL")
	wrote, err := writeExportedSQL(path, text)
	if err != nil || !wrote {
		t.Fatalf("write = %v, %v", wrote, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != text {
		t.Fatalf("written bytes = %q, %v", got, err)
	}

	replacement := "SELECT 1;\n"
	if wrote, err := writeExportedSQL(path, replacement); err != nil || !wrote {
		t.Fatalf("overwrite = %v, %v", wrote, err)
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != replacement {
		t.Fatalf("overwritten bytes = %q, %v", got, err)
	}

	noSuffix := filepath.Join(dir, "database")
	before, _ := os.ReadFile(path)
	if wrote, err := writeExportedSQL(noSuffix, "SELECT 2;\n"); err == nil || wrote {
		t.Fatal("path without .sql was accepted")
	}
	if _, err := os.Stat(noSuffix); !os.IsNotExist(err) {
		t.Fatal("path without .sql was created")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid suffix changed another file")
	}
}

func TestWriteExportedSQLRejectsLinksDirectoriesAndPreservesOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.sql")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.sql")
	if err := os.Symlink(target, link); err == nil {
		if wrote, err := writeExportedSQL(link, "replace"); err == nil || wrote {
			t.Fatal("symlink was accepted")
		}
		if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
			t.Fatalf("symlink target changed: %q, %v", data, err)
		}
	}
	dirPath := filepath.Join(dir, "folder.sql")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if wrote, err := writeExportedSQL(dirPath, "replace"); err == nil || wrote {
		t.Fatal("directory was accepted")
	}

	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	lockedFile := filepath.Join(locked, "keep.sql")
	if err := os.WriteFile(lockedFile, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	if wrote, err := writeExportedSQL(lockedFile, "replacement"); err == nil || wrote {
		t.Fatal("write in a read-only directory succeeded")
	}
	if data, err := os.ReadFile(lockedFile); err != nil || string(data) != "original" {
		t.Fatalf("failed overwrite changed existing content: %q, %v", data, err)
	}
}

func TestWriteExportedSQLCancelAndValidation(t *testing.T) {
	if wrote, err := writeExportedSQL("", "SQL"); err != nil || wrote {
		t.Fatal("empty selected path was not a cancel")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.sql")
	if wrote, err := writeExportedSQL(path, ""); err == nil || wrote {
		t.Fatal("empty SQL was accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("empty SQL created a file")
	}
	relative := "relative.sql"
	if wrote, err := writeExportedSQL(relative, "SQL"); err == nil || wrote {
		t.Fatal("relative path was accepted")
	}
}

func TestWriteExportedSQLKeepsSelectedSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "already.sql")
	if wrote, err := writeExportedSQL(path, "SQL"); err != nil || !wrote {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".sql"); !os.IsNotExist(err) {
		t.Fatal("write appended a second extension")
	}
}
