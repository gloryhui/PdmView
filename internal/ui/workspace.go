package appui

import (
	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

const (
	RootKey            = "pdmview.app.root"
	StatusKey          = "pdmview.app.status"
	CanvasKey          = "pdmview.workspace.canvas"
	NavigationSplitKey = "pdmview.workspace.navigation-split"
	DetailsSplitKey    = "pdmview.workspace.details-split"
)

type Workspace struct {
	NavigationPercent float32
	CanvasPercent     float32
	canvas            *canvasView
}

func NewWorkspace() *Workspace {
	return &Workspace{
		NavigationPercent: 24,
		CanvasPercent:     72,
		canvas:            newCanvasView(),
	}
}

func View(state *app.State, workspace *Workspace) func(*ui.Context) {
	if workspace == nil {
		workspace = NewWorkspace()
	}
	if workspace.canvas == nil {
		workspace.canvas = newCanvasView()
	}
	return func(c *ui.Context) {
		t := c.Theme()
		lang := string(state.Locale())
		ui.Column(c.Key(RootKey)).Fill().Background(t.Background).Children(func() {
			ui.SplitPercent(c.Key(NavigationSplitKey), &workspace.NavigationPercent,
				func() {
					sidePane(c, state, i18n.KeyNavigationHeading, t, lang, func() {
						ui.Text(c, state.Text(i18n.KeySampleDiagram, nil)).FontSize(12).TextColor(t.TextMuted).Lang(lang)
					})
				},
				func() {
					ui.SplitPercent(c.Key(DetailsSplitKey), &workspace.CanvasPercent,
						func() { workspace.canvas.view(c, state, t, lang) },
						func() {
							sidePane(c, state, i18n.KeyDetailsHeading, t, lang, func() {
								if node := workspace.canvas.selectedNode(); node != nil {
									ui.Text(c, node.Name).FillWidth().Wrap().FontSize(12).TextColor(t.Text)
								}
							})
						},
					).Fill()
				},
			).Grow(1).FillWidth()
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			statusBar(c, state, t, lang)
		})
	}
}

func sidePane(c *ui.Context, state *app.State, key i18n.Key, t *ui.Theme, lang string, content func()) {
	ui.Column(c).Fill().Background(t.Surface).Padding(12).Gap(8).Children(func() {
		paneHeading(c, state.Text(key, nil), t, lang)
		content()
	})
}

func paneHeading(c *ui.Context, text string, t *ui.Theme, lang string) {
	ui.Text(c, text).FontSize(13).Bold().TextColor(t.Text).Lang(lang).Role(ui.RoleHeading).Level(2)
}

func statusBar(c *ui.Context, state *app.State, t *ui.Theme, lang string) {
	ui.Row(c).FillWidth().Height(52).Padding(8, 12).Background(t.Surface).Children(func() {
		ui.Text(c, state.StatusText()).Key(StatusKey).
			FillWidth().Wrap().MaxLines(2).Ellipsis("…").
			FontSize(11).TextColor(t.TextMuted).Lang(lang)
	})
}
