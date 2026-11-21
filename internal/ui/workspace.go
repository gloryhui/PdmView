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
}

func NewWorkspace() *Workspace {
	return &Workspace{
		NavigationPercent: 24,
		CanvasPercent:     72,
	}
}

func View(state *app.State, workspace *Workspace) func(*ui.Context) {
	if workspace == nil {
		workspace = NewWorkspace()
	}
	return func(c *ui.Context) {
		t := c.Theme()
		lang := string(state.Locale())
		ui.Column(c.Key(RootKey)).Fill().Background(t.Background).Children(func() {
			ui.SplitPercent(c.Key(NavigationSplitKey), &workspace.NavigationPercent,
				func() { sidePane(c, state, i18n.KeyNavigationHeading, t, lang) },
				func() {
					ui.SplitPercent(c.Key(DetailsSplitKey), &workspace.CanvasPercent,
						func() { canvasPane(c, state, t, lang) },
						func() { sidePane(c, state, i18n.KeyDetailsHeading, t, lang) },
					).Fill()
				},
			).Grow(1).FillWidth()
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			statusBar(c, state, t, lang)
		})
	}
}

func canvasPane(c *ui.Context, state *app.State, t *ui.Theme, lang string) {
	heading := state.Text(i18n.KeyCanvasHeading, nil)
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().Height(36).Padding(4, 10).Background(t.Surface).Children(func() {
			paneHeading(c, heading, t, lang)
		})
		ui.Box(c).Grow(1).FillWidth().Clip().
			Key(CanvasKey).
			Label(heading).
			Draw(func(p *ui.Painter, r ui.Rect) { drawGrid(p, r, t) })
	})
}

func sidePane(c *ui.Context, state *app.State, key i18n.Key, t *ui.Theme, lang string) {
	ui.Column(c).Fill().Background(t.Surface).Padding(12).Gap(8).Children(func() {
		paneHeading(c, state.Text(key, nil), t, lang)
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

func drawGrid(p *ui.Painter, r ui.Rect, t *ui.Theme) {
	p.Clip(r, 0, func() {
		p.Fill(r, t.Background, 0)
		line := t.Background.Mix(t.Border, 0.35)
		const step = 32
		for x := r.X; x <= r.X+r.W; x += step {
			p.Line(x, r.Y, x, r.Y+r.H, 0.5, line)
		}
		for y := r.Y; y <= r.Y+r.H; y += step {
			p.Line(r.X, y, r.X+r.W, y, 0.5, line)
		}
	})
}
