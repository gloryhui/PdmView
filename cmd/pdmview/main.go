package main

import (
	"fmt"
	"os"

	"pdmview/internal/app"
	appui "pdmview/internal/ui"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

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
}
