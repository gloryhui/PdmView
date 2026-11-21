package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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

func startPDMCopy(win *mygo.Window, workspace *appui.Workspace, state *app.State, store pdmCopyStore, request *appui.PDMCopyRequest) {
	if win == nil || win.IsDestroyed() || !saveDialogOpen.CompareAndSwap(false, true) {
		workspace.PublishPDMCopyResult(request, i18n.KeyPDMCopySaveFailed, nil)
		return
	}
	title := state.Text(i18n.KeyPDMCopySave, nil)
	go func() {
		saved, status := savePDMCopy(win, workspace, store, request, title)
		saveDialogOpen.Store(false)
		mygo.RunOnMain(func() {
			workspace.PublishPDMCopyResult(request, status, saved)
		})
	}()
}

func savePDMCopy(win *mygo.Window, workspace *appui.Workspace, store pdmCopyStore, request *appui.PDMCopyRequest, title string) (*appui.BoardSet, i18n.Key) {
	data, err := request.Run()
	if errors.Is(err, pdm.ErrInvalidTextEdit) {
		return nil, i18n.KeyPDMCopyInvalidEdit
	}
	if err != nil {
		return nil, i18n.KeyPDMCopyFailed
	}
	current := false
	checkCurrent := func() {
		mygo.RunOnMain(func() {
			current = !win.IsDestroyed() && workspace.PDMCopyCurrent(request)
		})
	}
	checkCurrent()
	if !current {
		return nil, ""
	}
	path := request.SourcePath()
	if !request.WritableCopy() {
		path, err = mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       title,
			DefaultPath: pdmCopyDefaultPath(path),
			Filters:     []mygo.FileFilter{{Name: "PowerDesigner PDM", Extensions: []string{"pdm"}}},
		})
		if err != nil {
			return nil, i18n.KeyPDMCopyFailed
		}
		if path == "" {
			return nil, ""
		}
		path, err = canonicalCopyPath(path)
		if err != nil {
			return nil, i18n.KeyPDMCopyFailed
		}
	}
	saved, err := request.Prepare(path, data)
	if err != nil {
		return nil, i18n.KeyPDMCopyInvalidEdit
	}
	checkCurrent()
	if !current {
		return nil, ""
	}
	if request.WritableCopy() {
		err = updatePDMCopy(path, request.SourceXML(), data)
	} else {
		err = writeNewPDMCopy(path, request.SourcePath(), data)
	}
	switch {
	case errors.Is(err, errPDMCopyChanged):
		return nil, i18n.KeyPDMCopyChanged
	case errors.Is(err, os.ErrExist):
		return nil, i18n.KeyPDMCopyExists
	case err != nil && request.WritableCopy():
		return nil, i18n.KeyPDMCopySaveFailed
	case err != nil:
		return nil, i18n.KeyPDMCopyFailed
	}
	if err := store.remember(path, data); err != nil {
		return &saved, i18n.KeyPDMCopyRememberFailed
	}
	return &saved, i18n.KeyPDMCopySaved
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
	return stagePDMCopy(path, data, 0o600, func(tempPath string) error {
		return os.Link(tempPath, path)
	})
}

var errPDMCopyChanged = errors.New("pdmview: PDM copy changed on disk")

func updatePDMCopy(path string, expected, data []byte) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errPDMCopyChanged
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errPDMCopyChanged
	}
	verify := func() error {
		current, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			return errPDMCopyChanged
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
			return errPDMCopyChanged
		}
		expectedHash := sha256.Sum256(expected)
		hash := sha256.New()
		if _, err := io.Copy(hash, io.LimitReader(current, int64(len(expected))+1)); err != nil {
			return err
		}
		pathInfo, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return errPDMCopyChanged
		}
		if err != nil {
			return err
		}
		if !pathInfo.Mode().IsRegular() || !os.SameFile(dataInfo, pathInfo) ||
			pathInfo.Size() != dataInfo.Size() || !pathInfo.ModTime().Equal(dataInfo.ModTime()) ||
			!bytes.Equal(hash.Sum(nil), expectedHash[:]) {
			return errPDMCopyChanged
		}
		return nil
	}
	if bytes.Equal(expected, data) {
		return verify()
	}
	if info.Mode().Perm()&0o222 == 0 {
		return os.ErrPermission
	}
	return stagePDMCopy(path, data, info.Mode().Perm(), func(tempPath string) error {
		if err := verify(); err != nil {
			return err
		}
		return os.Rename(tempPath, path)
	})
}

func stagePDMCopy(path string, data []byte, mode os.FileMode, publish func(string) error) error {
	staged, err := os.CreateTemp(filepath.Dir(path), ".pdmview-copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(staged.Name())
	if err := staged.Chmod(mode); err != nil {
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
	return publish(staged.Name())
}

type pdmCopyStore struct {
	dir string
}

func newPDMCopyStore(preferences string) pdmCopyStore {
	if preferences == "" {
		return pdmCopyStore{}
	}
	return pdmCopyStore{dir: filepath.Join(filepath.Dir(preferences), "editable-pdm-copies")}
}

func canonicalCopyPath(path string) (string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func (s pdmCopyStore) receipt(path string) string {
	key := sha256.Sum256([]byte(path))
	return filepath.Join(s.dir, fmt.Sprintf("%x", key))
}

func (s pdmCopyStore) remember(path string, data []byte) error {
	if s.dir == "" {
		return app.ErrPreferencesPath
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	checksum := sha256.Sum256(data)
	receipt := s.receipt(path)
	return stagePDMCopy(receipt, checksum[:], 0o600, func(tempPath string) error {
		return os.Rename(tempPath, receipt)
	})
}

func loadPDMSource(path string, store pdmCopyStore) (appui.BoardSet, error) {
	set, err := loadBoardSet(path)
	if err != nil || store.dir == "" {
		return set, err
	}
	canonical, err := canonicalCopyPath(set.SourcePath)
	if err != nil {
		return set, nil
	}
	info, err := os.Lstat(canonical)
	if err != nil || !info.Mode().IsRegular() {
		return set, nil
	}
	receipt, err := os.ReadFile(store.receipt(canonical))
	if err != nil || len(receipt) != sha256.Size {
		return set, nil
	}
	checksum := sha256.Sum256(set.SourceXML)
	if bytes.Equal(receipt, checksum[:]) {
		set.SourcePath, set.WritableCopy = canonical, true
	}
	return set, nil
}
