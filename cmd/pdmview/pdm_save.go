package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo"
)

func startPDMSave(win *mygo.Window, workspace *appui.Workspace, request *appui.PDMSaveRequest) {
	if win == nil || win.IsDestroyed() {
		workspace.PublishPDMSaveResult(request, i18n.KeyPDMSaveFailed, nil)
		return
	}
	go func() {
		saved, status := savePDM(request, func() bool {
			var current bool
			mygo.RunOnMain(func() {
				current = !win.IsDestroyed() && workspace.PDMSaveCurrent(request)
			})
			return current
		})
		mygo.RunOnMain(func() {
			workspace.PublishPDMSaveResult(request, status, saved)
		})
	}()
}

func savePDM(request *appui.PDMSaveRequest, current func() bool) (*appui.BoardSet, i18n.Key) {
	data, err := request.Run()
	if errors.Is(err, pdm.ErrInvalidTextEdit) {
		return nil, i18n.KeyPDMInvalidEdit
	}
	if err != nil {
		return nil, i18n.KeyPDMSaveFailed
	}
	expected := request.SourceXML()
	var saved *appui.BoardSet
	if !bytes.Equal(expected, data) {
		set, err := request.Prepare(data)
		if err != nil {
			return nil, i18n.KeyPDMInvalidEdit
		}
		saved = &set
	}
	if !current() {
		return nil, ""
	}
	err = updatePDMFile(request.SourcePath(), expected, data)
	if errors.Is(err, errPDMChanged) {
		return nil, i18n.KeyPDMChanged
	}
	if err != nil {
		return nil, i18n.KeyPDMSaveFailed
	}
	return saved, i18n.KeyPDMSaved
}

var errPDMChanged = errors.New("pdmview: PDM file changed on disk")

func updatePDMFile(path string, expected, data []byte) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("pdmview: PDM path is not absolute")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errPDMChanged
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pdmview: PDM save requires a regular file")
	}
	changed := !bytes.Equal(expected, data)
	verify := func() error {
		access := os.O_RDONLY
		if changed {
			access = os.O_RDWR
		}
		current, err := os.OpenFile(path, access, 0)
		if errors.Is(err, os.ErrNotExist) {
			return errPDMChanged
		}
		if err != nil {
			return err
		}
		defer current.Close()
		dataInfo, err := current.Stat()
		if err != nil {
			return err
		}
		if !os.SameFile(info, dataInfo) || dataInfo.Size() != int64(len(expected)) {
			return errPDMChanged
		}
		expectedHash := sha256.Sum256(expected)
		hash := sha256.New()
		if _, err := io.Copy(hash, io.LimitReader(current, int64(len(expected))+1)); err != nil {
			return err
		}
		pathInfo, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return errPDMChanged
		}
		if err != nil {
			return err
		}
		if !pathInfo.Mode().IsRegular() || !os.SameFile(dataInfo, pathInfo) ||
			pathInfo.Size() != dataInfo.Size() || !pathInfo.ModTime().Equal(dataInfo.ModTime()) ||
			pathInfo.Mode().Perm() != info.Mode().Perm() ||
			!bytes.Equal(hash.Sum(nil), expectedHash[:]) {
			return errPDMChanged
		}
		return nil
	}
	if !changed {
		return verify()
	}
	if info.Mode().Perm()&0o222 == 0 {
		return os.ErrPermission
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".pdmview-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		_ = staged.Close()
		return err
	}
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
	if err := verify(); err != nil {
		return err
	}
	return os.Rename(staged.Name(), path)
}
