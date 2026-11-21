package main

import (
	"fmt"
	"os"

	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appName       = "PdmView"
	appRootKey    = "pdmview.app.root"
	appStatusKey  = "pdmview.app.status"
	defaultWidth  = 1200
	defaultHeight = 800
	minWidth      = 800
	minHeight     = 600
)

func view(state *app.State) func(*ui.Context) {
	return func(c *ui.Context) {
		t := c.Theme()
		lang := string(state.Locale())
		ui.Column(c.Key(appRootKey)).Fill().Center().Gap(12).Background(t.Background).Children(func() {
			ui.Text(c, state.Text(i18n.KeyAppBrand, nil)).FontSize(32).Bold().Lang(lang)
			ui.Text(c, state.StatusText()).Key(appStatusKey).FontSize(15).TextColor(t.TextMuted).Lang(lang)
		})
	}
}

func windowOptions(state *app.State) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:          appName,
		Width:          defaultWidth,
		Height:         defaultHeight,
		UseContentSize: true,
		MinWidth:       minWidth,
		MinHeight:      minHeight,
		Content:        ui.View(view(state)),
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

	var win *mygo.Window
	var refresh func()
	refresh = func() {
		mygo.App.SetMenu(state.Menu(refresh))
		if win != nil {
			win.Invalidate()
		}
	}
	refresh()
	win = mygo.NewWindow(windowOptions(state))
}
