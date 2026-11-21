package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appName       = "PdmView"
	appBrand      = "PdmView"
	appRootKey    = "pdmview.app.root"
	defaultWidth  = 1200
	defaultHeight = 800
	minWidth      = 800
	minHeight     = 600
)

type appState struct {
	locale string
}

func newAppState() *appState {
	return &appState{}
}

func (s *appState) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c.Key(appRootKey)).Fill().Center().Background(t.Background).Children(func() {
		ui.Text(c, appBrand).FontSize(32).Bold().Lang(s.locale)
	})
}

func windowOptions(state *appState) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:          appName,
		Width:          defaultWidth,
		Height:         defaultHeight,
		UseContentSize: true,
		MinWidth:       minWidth,
		MinHeight:      minHeight,
		Content:        ui.View(state.view),
	}
}

func main() {
	state := newAppState()
	mygo.App.SetName(appName)
	mygo.App.WhenReady(func() {
		mygo.NewWindow(windowOptions(state))
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "pdmview:", err)
		os.Exit(1)
	}
}
