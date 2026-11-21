package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

var pdmChooserOpen atomic.Bool
var sqlSaveDialogOpen atomic.Bool

const (
	appName       = "PdmView"
	defaultWidth  = 1200
	defaultHeight = 800
	minWidth      = 800
	minHeight     = 600
)

func windowOptions(state *app.State, workspace *appui.Workspace) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:          appName,
		Width:          defaultWidth,
		Height:         defaultHeight,
		UseContentSize: true,
		MinWidth:       minWidth,
		MinHeight:      minHeight,
		Content:        ui.View(appui.View(state, workspace)),
	}
}

func main() {
	mygo.App.SetName(appName)
	mygo.App.WhenReady(showMainWindow)
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "pdmview:", err)
		os.Exit(1)
	}
}

func showMainWindow() {
	preferred := mygo.App.PreferredLanguages()
	systemTag := ""
	if len(preferred) > 0 {
		systemTag = preferred[0]
	}

	path, err := app.PreferencesPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pdmview: preferences:", err)
	}
	state := app.New(app.NewStore(path), systemTag)
	workspace := appui.NewWorkspace()
	workspace.SetClipboardWriter(func(text string) error {
		return mygo.Clipboard.Write(transfer.TextData(text))
	})

	var win *mygo.Window
	var refresh func()
	refresh = func() {
		mygo.App.SetMenu(state.Menu(refresh))
		if win != nil {
			win.Invalidate()
		}
	}
	refresh()
	win = mygo.NewWindow(windowOptions(state, workspace))
	workspace.SetOnChanged(func() {
		if win != nil && !win.IsDestroyed() {
			win.Invalidate()
		}
	})
	workspace.SetOpenHandler(func() {
		openPDMDialog(win, workspace, state)
	})
	workspace.SetBoardSelector(func(index int) {
		startBoardSwitch(win, workspace, index)
	})
	workspace.SetSearchStarter(func(request appui.SearchRequest) {
		startWorkspaceSearch(win, workspace, request)
	})
	workspace.SetDatabasePreviewStarter(func(request appui.DatabasePreviewRequest) {
		startDatabasePreview(win, workspace, request)
	})
	workspace.SetSQLExportStarter(func(request appui.SQLExportRequest) {
		startSQLExport(win, workspace, request)
	})
	if path, ok := optionalPDMPath(os.Args[1:]); ok {
		startBoardLoad(win, workspace, path)
	}
}

func optionalPDMPath(args []string) (string, bool) {
	if len(args) != 1 || args[0] == "" || args[0][0] == '-' {
		return "", false
	}
	return args[0], true
}

func openPDMDialog(win *mygo.Window, workspace *appui.Workspace, state *app.State) {
	if win == nil || win.IsDestroyed() {
		return
	}
	if !pdmChooserOpen.CompareAndSwap(false, true) {
		return
	}
	title := state.Text(i18n.KeyOpenPDM, nil)
	go func() {
		defer pdmChooserOpen.Store(false)
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  title,
			Filters: []mygo.FileFilter{{
				Name:       "PowerDesigner",
				Extensions: []string{"pdm"},
			}},
		})
		if err != nil {
			mygo.RunOnMain(func() {
				if win.IsDestroyed() {
					return
				}
				workspace.ReportOpenError()
			})
			return
		}
		if len(paths) == 0 {
			return
		}
		mygo.RunOnMain(func() {
			if win.IsDestroyed() {
				return
			}
			startBoardLoad(win, workspace, paths[0])
		})
	}()
}

func startBoardLoad(win *mygo.Window, workspace *appui.Workspace, path string) {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return
	}
	token := workspace.BeginOpen()
	go func() {
		set, err := loadBoardSet(path)
		mygo.RunOnMain(func() {
			applyBoardResult(win, workspace, token, set, err)
		})
	}()
}

func loadBoardSet(path string) (appui.BoardSet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return appui.BoardSet{}, err
	}
	if !info.Mode().IsRegular() {
		return appui.BoardSet{}, fmt.Errorf("pdmview: not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return appui.BoardSet{}, err
	}
	defer file.Close()
	document, err := pdm.Parse(file)
	if err != nil {
		return appui.BoardSet{}, err
	}
	return appui.BuildBoardSet(document), nil
}

func startBoardSwitch(win *mygo.Window, workspace *appui.Workspace, index int) {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return
	}
	board, ok := workspace.BoardDescriptor(index)
	if !ok {
		return
	}
	token, ok := workspace.BeginBoardSwitch(index)
	if !ok {
		return
	}
	go func() {
		prepared := appui.PrepareBoard(board)
		mygo.RunOnMain(func() {
			if win.IsDestroyed() {
				return
			}
			workspace.PublishPreparedBoard(token, prepared)
		})
	}()
}

func startWorkspaceSearch(win *mygo.Window, workspace *appui.Workspace, request appui.SearchRequest) {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return
	}
	go func() {
		results := request.Run()
		mygo.RunOnMain(func() {
			if win.IsDestroyed() {
				return
			}
			workspace.PublishSearchResults(request, results)
		})
	}()
}

func startDatabasePreview(
	win *mygo.Window,
	workspace *appui.Workspace,
	request appui.DatabasePreviewRequest,
) {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return
	}
	go func() {
		result := request.Run()
		mygo.RunOnMain(func() {
			if win.IsDestroyed() {
				return
			}
			workspace.PublishDatabasePreview(request, result)
		})
	}()
}

func startSQLExport(
	win *mygo.Window,
	workspace *appui.Workspace,
	request appui.SQLExportRequest,
) {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return
	}
	if !sqlSaveDialogOpen.CompareAndSwap(false, true) {
		workspace.PublishSQLExportFailed(request)
		return
	}
	go func() {
		defer sqlSaveDialogOpen.Store(false)
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       request.Title(),
			DefaultPath: sqlSaveDialogDefaultPath(request.DefaultPath()),
			Filters: []mygo.FileFilter{{
				Name:       request.FilterName(),
				Extensions: []string{"sql"},
			}},
		})
		current := false
		mygo.RunOnMain(func() {
			current = !win.IsDestroyed() && workspace.SQLExportCurrent(request)
		})
		if !current {
			return
		}
		if err != nil {
			mygo.RunOnMain(func() {
				workspace.PublishSQLExportFailed(request)
			})
			return
		}
		if path == "" {
			mygo.RunOnMain(func() {
				workspace.PublishSQLExportCanceled(request)
			})
			return
		}
		if _, err := writeExportedSQL(path, request.SQL()); err != nil {
			mygo.RunOnMain(func() {
				workspace.PublishSQLExportFailed(request)
			})
			return
		}
		mygo.RunOnMain(func() {
			if win.IsDestroyed() {
				return
			}
			workspace.PublishSQLExportSucceeded(request)
		})
	}()
}

func sqlSaveDialogDefaultPath(filename string) string {
	if runtime.GOOS == "darwin" && strings.EqualFold(filepath.Ext(filename), ".sql") {
		return filename[:len(filename)-len(".sql")]
	}
	return filename
}

func writeExportedSQL(path, text string) (bool, error) {
	if path == "" {
		return false, nil
	}
	if text == "" {
		return false, fmt.Errorf("pdmview: SQL is empty")
	}
	if !filepath.IsAbs(path) {
		return false, fmt.Errorf("pdmview: save path is not absolute")
	}
	if !strings.EqualFold(filepath.Ext(path), ".sql") {
		return false, fmt.Errorf("pdmview: save path must end in .sql")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("pdmview: save path must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".pdmview-sql-*")
	if err != nil {
		return false, err
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.WriteString(text); err != nil {
		_ = temp.Close()
		return false, err
	}
	if err := temp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return false, err
	}
	committed = true
	return true, nil
}

type windowLifecycle interface {
	IsDestroyed() bool
}

func applyBoardResult(
	win windowLifecycle,
	workspace *appui.Workspace,
	token uint64,
	set appui.BoardSet,
	loadErr error,
) bool {
	if win == nil || win.IsDestroyed() || workspace == nil {
		return false
	}
	if loadErr != nil {
		return workspace.FailOpen(token)
	}
	return workspace.PublishBoards(token, set)
}
