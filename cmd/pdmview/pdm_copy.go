package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo"
)

func startPDMCopy(win *mygo.Window, workspace *appui.Workspace, state *app.State, request *appui.PDMCopyRequest) {
	if win == nil || win.IsDestroyed() || !saveDialogOpen.CompareAndSwap(false, true) {
		workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopyFailed)
		return
	}
	title := state.Text(i18n.KeyPDMCopySave, nil)
	go func() {
		status := savePDMCopy(win, workspace, request, title)
		saveDialogOpen.Store(false)
		mygo.RunOnMain(func() {
			workspace.PublishPDMCopyResult(request, status)
		})
	}()
}

func savePDMCopy(win *mygo.Window, workspace *appui.Workspace, request *appui.PDMCopyRequest, title string) i18n.Key {
	data, err := request.Run()
	if errors.Is(err, pdm.ErrInvalidTextEdit) {
		return i18n.KeyPDMCopyInvalidEdit
	}
	if err != nil {
		return i18n.KeyPDMCopyFailed
	}
	current := false
	checkCurrent := func() {
		mygo.RunOnMain(func() {
			current = !win.IsDestroyed() && workspace.PDMCopyCurrent(request)
		})
	}
	checkCurrent()
	if !current {
		return ""
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      win,
		Title:       title,
		DefaultPath: pdmCopyDefaultPath(request.SourcePath()),
		Filters:     []mygo.FileFilter{{Name: "PowerDesigner PDM", Extensions: []string{"pdm"}}},
	})
	if err != nil {
		return i18n.KeyPDMCopyFailed
	}
	checkCurrent()
	if path == "" || !current {
		return ""
	}
	err = writeNewPDMCopy(path, request.SourcePath(), data)
	switch {
	case errors.Is(err, os.ErrExist):
		return i18n.KeyPDMCopyExists
	case err != nil:
		return i18n.KeyPDMCopyFailed
	default:
		return i18n.KeyPDMCopySaved
	}
}

func pdmCopyDefaultPath(sourcePath string) string {
	base := filepath.Base(sourcePath)
	name := strings.TrimSuffix(base, filepath.Ext(base)) + "-copy"
	if runtime.GOOS != "darwin" {
		name += ".pdm"
	}
	return filepath.Join(filepath.Dir(sourcePath), name)
}

func writeNewPDMCopy(path, sourcePath string, data []byte) error {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".pdm") {
		return fmt.Errorf("pdmview: expected an absolute .pdm path")
	}
	path = filepath.Clean(path)
	if path == filepath.Clean(sourcePath) {
		return os.ErrExist
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".pdmview-copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	if _, err := staged.Write(data); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Sync(); err != nil {
		_ = staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	return os.Link(staged.Name(), path)
}
