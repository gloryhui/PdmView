package appui

import (
	"fmt"

	"pdmview/internal/app"
	"pdmview/internal/i18n"

	"github.com/egoist/mygo/ui"
)

const (
	RootKey            = "pdmview.app.root"
	StatusKey          = "pdmview.app.status"
	CanvasKey          = "pdmview.workspace.canvas"
	SelectedNodeKey    = "pdmview.workspace.canvas-selection"
	OpenPDMKey         = "pdmview.workspace.open-pdm"
	LoadStatusKey      = "pdmview.workspace.load-status"
	NavigationSplitKey = "pdmview.workspace.navigation-split"
	DetailsSplitKey    = "pdmview.workspace.details-split"
	NavigationBodyKey  = "pdmview.workspace.navigation-body"
	DetailsBodyKey     = "pdmview.workspace.details-body"
	navigationMinWidth = 200
	navigationMaxWidth = 360
	detailsMinWidth    = 220
	detailsMaxWidth    = 420
	canvasMinWidth     = 360
)

// DiagramButtonKey returns the stable key for a zero-based board button.
func DiagramButtonKey(index int) string {
	return fmt.Sprintf("pdmview.workspace.diagram.%d", index)
}

type Workspace struct {
	NavigationPercent float32
	CanvasPercent     float32
	canvas            *canvasView
	boards            []DiagramBoard
	activeBoard       int
	loadToken         uint64
	boardToken        uint64
	hasModel          bool
	loading           bool
	switching         bool
	loadFailed        bool
	activeFallback    bool
	activeAutoLayout  bool
	pendingBoard      int
	openHandler       func()
	boardSelector     func(int)
	onChanged         func()
}

func NewWorkspace() *Workspace {
	return &Workspace{
		NavigationPercent: 25,
		CanvasPercent:     60,
		canvas:            newCanvasView(),
		pendingBoard:      -1,
	}
}

// SetOpenHandler installs the native local-file action shown by Open PDM.
func (w *Workspace) SetOpenHandler(handler func()) {
	w.openHandler = handler
}

// SetOnChanged installs a main-thread invalidation callback.
func (w *Workspace) SetOnChanged(changed func()) {
	w.onChanged = changed
}

// SetBoardSelector installs the off-UI board preparation action.
func (w *Workspace) SetBoardSelector(selector func(int)) {
	w.boardSelector = selector
}

// BeginOpen starts a load and returns its stale-result token.
func (w *Workspace) BeginOpen() uint64 {
	w.loadToken++
	w.boardToken++
	w.loading = true
	w.switching = false
	w.loadFailed = false
	w.pendingBoard = -1
	w.notifyChanged()
	return w.loadToken
}

// PublishBoards adopts a completed load only when token is current.
func (w *Workspace) PublishBoards(token uint64, set BoardSet) bool {
	if token != w.loadToken || len(set.Boards) == 0 {
		return false
	}
	if set.Default < 0 || set.Default >= len(set.Boards) {
		set.Default = 0
	}
	w.boards = append([]DiagramBoard(nil), set.Boards...)
	w.boardToken++
	w.hasModel = true
	w.loading = false
	w.switching = false
	w.loadFailed = false
	w.activeBoard = set.Default
	w.activeFallback = set.Boards[set.Default].Fallback
	w.activeAutoLayout = set.Prepared.Graph.AutoLayout
	w.pendingBoard = -1
	w.canvas.activatePrepared(set.Prepared)
	w.notifyChanged()
	return true
}

// FailOpen reports a load failure only when token is current.
func (w *Workspace) FailOpen(token uint64) bool {
	if token != w.loadToken {
		return false
	}
	w.loading = false
	w.switching = false
	w.loadFailed = true
	w.notifyChanged()
	return true
}

// ReportOpenError reports a dialog/open error without changing the active board.
func (w *Workspace) ReportOpenError() {
	w.loading = false
	w.switching = false
	w.loadFailed = true
	w.notifyChanged()
}

// ActivateBoard switches by zero-based catalog index.
func (w *Workspace) ActivateBoard(index int) {
	if !w.hasModel {
		return
	}
	if index < 0 || index >= len(w.boards) {
		return
	}
	if index == w.activeBoard {
		if w.switching {
			w.boardToken++
			w.switching = false
			w.pendingBoard = -1
			w.notifyChanged()
		}
		return
	}
	if w.boardSelector != nil {
		w.boardSelector(index)
	}
}

// BoardDescriptor returns one catalog source for off-UI preparation.
func (w *Workspace) BoardDescriptor(index int) (DiagramBoard, bool) {
	if !w.hasModel || index < 0 || index >= len(w.boards) {
		return DiagramBoard{}, false
	}
	return w.boards[index], true
}

// BeginBoardSwitch marks a board preparation request and returns its token.
func (w *Workspace) BeginBoardSwitch(index int) (uint64, bool) {
	if !w.hasModel || index < 0 || index >= len(w.boards) || index == w.activeBoard {
		return 0, false
	}
	w.boardToken++
	w.switching = true
	w.pendingBoard = index
	w.notifyChanged()
	return w.boardToken, true
}

// PublishPreparedBoard adopts a prepared board only when its token is current.
func (w *Workspace) PublishPreparedBoard(token uint64, prepared PreparedBoard) bool {
	if token != w.boardToken || !w.switching ||
		w.pendingBoard < 0 || w.pendingBoard >= len(w.boards) {
		return false
	}
	index := w.pendingBoard
	w.activeBoard = index
	w.activeFallback = prepared.Board.Fallback
	w.activeAutoLayout = prepared.Graph.AutoLayout
	w.switching = false
	w.pendingBoard = -1
	w.canvas.activatePrepared(prepared)
	w.notifyChanged()
	return true
}

func (w *Workspace) ActiveBoard() int {
	return w.activeBoard
}

func (w *Workspace) BoardCount() int {
	if !w.hasModel {
		return 1
	}
	return len(w.boards)
}

func (w *Workspace) notifyChanged() {
	if w.onChanged != nil {
		w.onChanged()
	}
}

func (w *Workspace) constrainPanes(total float32) {
	if total < navigationMinWidth+detailsMinWidth+canvasMinWidth+2 {
		return
	}
	available := total - 1
	navMax := min(float32(navigationMaxWidth), max(float32(navigationMinWidth), available-detailsMinWidth-canvasMinWidth-1))
	navWidth := available * w.NavigationPercent / 100
	switch {
	case navWidth < navigationMinWidth-0.5:
		w.NavigationPercent = navigationMinWidth * 100 / available
	case navWidth > navMax+0.5:
		w.NavigationPercent = navMax * 100 / available
	}
	innerAvailable := available*(1-w.NavigationPercent/100) - 1
	details := innerAvailable * (1 - w.CanvasPercent/100)
	detailsMax := min(float32(detailsMaxWidth), max(float32(detailsMinWidth), innerAvailable-canvasMinWidth))
	switch {
	case details < detailsMinWidth-0.5:
		w.CanvasPercent = 100 - detailsMinWidth*100/innerAvailable
	case details > detailsMax+0.5:
		w.CanvasPercent = 100 - detailsMax*100/innerAvailable
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
		width, _ := c.Size()
		workspace.constrainPanes(width)
		t := c.Theme()
		lang := string(state.Locale())
		ui.Column(c.Key(RootKey)).Fill().Background(t.Background).Children(func() {
			ui.SplitPercent(c.Key(NavigationSplitKey), &workspace.NavigationPercent,
				func() {
					sidePane(c, state, i18n.KeyNavigationHeading, t, lang, NavigationBodyKey, navigationMinWidth, navigationMaxWidth, func() {
						workspace.diagramNavigation(c, state, t, lang)
					})
				},
				func() {
					ui.SplitPercent(c.Key(DetailsSplitKey), &workspace.CanvasPercent,
						func() { workspace.canvas.view(c, state, t, lang) },
						func() {
							sidePane(c, state, i18n.KeyDetailsHeading, t, lang, DetailsBodyKey, detailsMinWidth, detailsMaxWidth, func() {
								if node := workspace.canvas.selectedNode(); node != nil {
									ui.Text(c, node.Name).Key(SelectedNodeKey).
										FillWidth().Wrap().FontSize(12).TextColor(t.Text).Lang(lang)
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

func (w *Workspace) diagramNavigation(c *ui.Context, state *app.State, t *ui.Theme, lang string) {
	ui.Button(c, state.Text(i18n.KeyOpenPDM, nil)).Key(OpenPDMKey).
		FillWidth().Disabled(w.loading).OnClick(w.openHandler)
	paneHeading(c, state.Text(i18n.KeyDiagramsHeading, nil), t, lang)
	ui.Column(c).FillWidth().Gap(4).Role(ui.RoleTabList).Children(func() {
		for index := 0; index < w.BoardCount(); index++ {
			label := state.Text(i18n.KeySampleDiagram, nil)
			if w.hasModel {
				board := w.boards[index]
				label = board.Name
				if label == "" {
					label = state.Text(i18n.KeyDiagramFallback, map[string]string{
						"index": fmt.Sprintf("%d", index),
					})
				}
			}
			index := index
			button := ui.Button(c, label).Key(DiagramButtonKey(index)).
				FillWidth().Role(ui.RoleTab).
				AccessSelected(index == w.activeBoard).
				OnClick(func() { w.ActivateBoard(index) })
			if index == w.activeBoard {
				button.Background(t.Accent).TextColor(t.AccentText).Bold()
			}
		}
	})
	if status := w.statusText(state); status != "" {
		ui.Text(c, status).Key(LoadStatusKey).
			FillWidth().Wrap().MaxLines(3).Ellipsis("…").
			FontSize(11).TextColor(t.TextMuted).Lang(lang)
	}
}

func (w *Workspace) statusText(state *app.State) string {
	switch {
	case w.loading:
		return state.Text(i18n.KeyLoadingPDM, nil)
	case w.switching:
		return state.Text(i18n.KeyPreparingDiagram, nil)
	case w.loadFailed:
		return state.Text(i18n.KeyLoadPDMFailed, nil)
	case w.hasModel && w.activeFallback:
		return state.Text(i18n.KeyNoDiagramFallback, nil)
	case w.hasModel && w.activeAutoLayout:
		return state.Text(i18n.KeyAutoLayoutStatus, nil)
	default:
		return ""
	}
}

func sidePane(
	c *ui.Context,
	state *app.State,
	key i18n.Key,
	t *ui.Theme,
	lang, bodyKey string,
	minWidth, maxWidth float32,
	content func(),
) {
	ui.Column(c).Fill().Background(t.Surface).Padding(12).Gap(8).
		MinWidth(minWidth).MaxWidth(maxWidth).Clip().Children(func() {
		paneHeading(c, state.Text(key, nil), t, lang)
		ui.Scroll(c.Key(bodyKey)).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(content)
		})
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
