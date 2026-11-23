package appui

import (
	"fmt"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

const (
	RootKey            = "pdmview.app.root"
	StatusKey          = "pdmview.app.status"
	CanvasKey          = "pdmview.workspace.canvas"
	ModelTreeKey       = "pdmview.workspace.model-tree"
	DiagramListKey     = "pdmview.workspace.diagram-list"
	SelectedNodeKey    = "pdmview.workspace.canvas-selection"
	OpenPDMKey         = "pdmview.workspace.open-pdm"
	LoadStatusKey      = "pdmview.workspace.load-status"
	NavigationSplitKey = "pdmview.workspace.navigation-split"
	DetailsSplitKey    = "pdmview.workspace.details-split"
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
	document          *pdm.Document
	sourcePath        string
	sourceXML         []byte
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
	catalog           *modelCatalog
	outline           ui.OutlineState[string]
	navChosen         ui.Selection[string]
	navSelected       int
	selectedNavKey    string
	rowClickedKey     string
	openBefore        map[string]struct{}
	selectedTableID   string
	selectedTableName string
	detailIndex       *tableDetailIndex
	detailsScroll     ui.ScrollState
	detailsHidden     bool
	tableDetail       TableDetail
	hasTableDetail    bool
	tableProperties   tablePropertiesState

	tableOpen      func(string)
	tableClose     func()
	pdmSaveStarter func(*PDMSaveRequest)
	pdmSaveRequest *PDMSaveRequest

	unplacedTable        bool
	requestedTableID     string
	pendingTableID       string
	pendingTableToken    uint64
	pendingSmoothTable   bool
	requestedSmoothTable bool
	openHandler          func()
	boardSelector        func(int)
	searchStarter        func(SearchRequest)
	previewStarter       func(DatabasePreviewRequest)
	exportStarter        func(SQLExportRequest)
	preview              databasePreviewState
	searchIndex          *searchIndex
	searchQuery          string
	searchResults        []SearchResult
	searchFinished       bool
	searchToken          uint64
	searchCancel         func()
	searchSelected       int
	searchList           ui.ListState
	clipboardWriter      func(string) error
	copyFeedback         copyFeedback
	copyFeedbackTable    string
	copyDiagnosticCode   string
	copyToken            uint64
	onChanged            func()
}

func NewWorkspace() *Workspace {
	workspace := &Workspace{
		NavigationPercent: 25,
		CanvasPercent:     60,
		canvas:            newCanvasView(),
		activeBoard:       -1,
		pendingBoard:      -1,
		navSelected:       -1,
		openBefore:        make(map[string]struct{}),
	}
	workspace.outline.List.Selected = &workspace.navSelected
	workspace.outline.List.Selection = &workspace.navChosen
	workspace.canvas.onTableSelection = workspace.syncCanvasTableSelection
	workspace.canvas.onTableOpen = workspace.requestTableProperties
	return workspace
}

// SetOpenHandler installs the native local-file action shown by Open PDM.
func (w *Workspace) SetOpenHandler(handler func()) {
	w.openHandler = handler
}

// SetOnChanged installs a main-thread invalidation callback.
func (w *Workspace) SetOnChanged(changed func()) {
	w.onChanged = changed
}

// SetTablePropertiesOpener installs the main window bridge that shows the
// single table-properties window.
func (w *Workspace) SetTablePropertiesOpener(opener func(string)) {
	w.tableOpen = opener
}

// SetTablePropertiesCloser installs the main window bridge that closes the
// table-properties window.
func (w *Workspace) SetTablePropertiesCloser(closer func()) {
	w.tableClose = closer
}

// SetBoardSelector installs the off-UI board preparation action.
func (w *Workspace) SetBoardSelector(selector func(int)) {
	w.boardSelector = selector
}

// BeginOpen starts a load and returns its stale-result token.
func (w *Workspace) BeginOpen() uint64 {
	w.loadToken++
	w.pdmSaveRequest = nil
	w.cancelPendingSwitch()
	w.resetTableProperties()
	w.invalidateDatabasePreview()
	w.resetSearch()
	w.clearCopyFeedback()
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
	w.document = set.Document
	w.sourcePath, w.sourceXML = set.SourcePath, set.SourceXML
	w.boardToken++
	w.hasModel = true
	w.loading = false
	w.switching = false
	w.loadFailed = false
	w.activeBoard = set.Default
	w.activeFallback = set.Boards[set.Default].Fallback
	w.activeAutoLayout = set.Prepared.Graph.AutoLayout
	w.pendingBoard = -1
	w.catalog = set.Catalog
	w.detailIndex = set.Details
	w.searchIndex = set.Search
	w.resetSearch()
	w.clearCopyFeedback()
	w.outline = ui.OutlineState[string]{}
	w.outline.Open.Add(ModelRootKey)
	w.outline.List.Selected = &w.navSelected
	w.outline.List.Selection = &w.navChosen
	w.navChosen.Clear()
	w.navSelected = -1
	w.selectedNavKey = ""
	w.selectedTableID = ""
	w.selectedTableName = ""
	w.tableDetail = TableDetail{}
	w.hasTableDetail = false
	w.resetTableProperties()
	w.unplacedTable = false
	w.requestedTableID = ""
	w.pendingTableID = ""
	w.pendingTableToken = 0
	w.pendingSmoothTable = false
	w.canvas.activatePrepared(set.Prepared)
	w.invalidateDatabasePreview()
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
	if !w.hasModel || w.pdmSaveRequest != nil {
		return
	}
	if index < 0 || index >= len(w.boards) {
		return
	}
	if index == w.activeBoard {
		if w.switching {
			if w.pendingTableID != "" {
				w.clearTableSelection()
			}
			w.cancelPendingSwitch()
			w.notifyChanged()
		}
		return
	}
	w.cancelPendingSwitch()
	w.resetTableProperties()
	w.clearTableSelection()
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
	if !w.hasModel || w.pdmSaveRequest != nil || index < 0 || index >= len(w.boards) || index == w.activeBoard {
		return 0, false
	}
	w.boardToken++
	w.switching = true
	w.pendingBoard = index
	w.pendingTableID = ""
	w.pendingTableToken = 0
	w.pendingSmoothTable = false
	if w.requestedTableID != "" {
		w.pendingTableID = w.requestedTableID
		w.pendingTableToken = w.boardToken
		w.pendingSmoothTable = w.requestedSmoothTable
	}
	w.requestedTableID = ""
	w.requestedSmoothTable = false
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
	locateTableID := ""
	if w.pendingTableToken == token {
		locateTableID = w.pendingTableID
	}
	smoothTable := w.pendingSmoothTable && w.pendingTableToken == token
	w.pendingTableID = ""
	w.pendingTableToken = 0
	w.pendingSmoothTable = false
	w.resetTableProperties()
	w.canvas.activatePrepared(prepared)
	if locateTableID != "" {
		if offset, ok := w.canvas.tableNodeOffset(locateTableID); ok {
			if smoothTable {
				w.canvas.selectTableNodeSmooth(offset)
			} else {
				w.canvas.selectTableNode(offset)
			}
			w.unplacedTable = false
		} else {
			w.canvas.clearSelection()
			w.unplacedTable = true
		}
	}
	w.notifyChanged()
	return true
}

func (w *Workspace) ActiveBoard() int {
	return w.activeBoard
}

func (w *Workspace) BoardCount() int {
	return len(w.boards)
}

// LocateTable selects tableID in the current board or the first source board
// whose actual prepared graph contains it. Cross-board navigation uses the
// existing tokenized switch path.
func (w *Workspace) LocateTable(tableID string) bool {
	return w.locateTable(tableID, false)
}

// LocateSearchResult locates tableID and gives the camera a smooth,
// interruptible transition to the table.
func (w *Workspace) LocateSearchResult(tableID string) bool {
	return w.locateTable(tableID, true)
}

func (w *Workspace) locateTable(tableID string, smooth bool) bool {
	if tableID == "" || !w.hasModel || w.pdmSaveRequest != nil {
		return false
	}
	w.cancelPendingSwitch()
	w.clearCopyFeedbackUnless(tableID)
	w.selectedTableID = tableID
	w.selectedTableName = tableID
	if w.catalog != nil {
		w.selectedTableName = w.catalog.tableName(tableID)
	}
	w.syncSearchSelected()
	w.refreshSelectedDetail()
	w.unplacedTable = false
	w.revealTable(tableID)

	if offset, ok := w.canvas.tableNodeOffset(tableID); ok {
		if smooth {
			w.canvas.selectTableNodeSmooth(offset)
		} else {
			w.canvas.selectTableNode(offset)
		}
		w.notifyChanged()
		return true
	}
	for index, board := range w.boards {
		if index == w.activeBoard || !containsTableID(board.TableIDs, tableID) {
			continue
		}
		w.canvas.clearSelection()
		w.requestedTableID = tableID
		w.requestedSmoothTable = smooth
		if w.boardSelector != nil {
			w.boardSelector(index)
			w.requestedTableID = ""
			w.requestedSmoothTable = false
			return true
		}
		token, ok := w.BeginBoardSwitch(index)
		w.requestedTableID = ""
		w.requestedSmoothTable = false
		if !ok {
			return false
		}
		w.PublishPreparedBoard(token, PrepareBoard(board))
		return true
	}
	w.canvas.clearSelection()
	w.unplacedTable = true
	w.notifyChanged()
	return false
}

func (w *Workspace) syncCanvasTableSelection(tableID string) {
	w.cancelPendingSwitch()
	if tableID == "" {
		w.clearTableSelection()
		w.notifyChanged()
		return
	}
	w.clearCopyFeedbackUnless(tableID)
	w.selectedTableID = tableID
	w.selectedTableName = tableID
	if w.catalog != nil {
		w.selectedTableName = w.catalog.tableName(tableID)
	}
	w.syncSearchSelected()
	w.refreshSelectedDetail()
	w.unplacedTable = false
	w.revealTable(tableID)
	w.notifyChanged()
}

func (w *Workspace) revealTable(tableID string) {
	if w.catalog == nil {
		return
	}
	key := TableNavKey(tableID)
	if _, ok := w.catalog.nodes[key]; !ok {
		return
	}
	for parent := w.catalog.parent[key]; parent != "" && parent != ModelRootKey; parent = w.catalog.parent[parent] {
		w.outline.Open.Add(parent)
	}
	w.navChosen.Clear()
	w.navChosen.Add(key)
	w.selectedNavKey = key
}

func (w *Workspace) clearTableSelection() {
	w.clearCopyFeedback()
	w.selectedTableID = ""
	w.selectedTableName = ""
	w.searchSelected = -1
	w.tableDetail = TableDetail{}
	w.hasTableDetail = false
	w.unplacedTable = false
	w.navSelected = -1
	w.selectedNavKey = ""
	w.navChosen.Clear()
}

func (w *Workspace) reconcileOutlineSelection(openChanged bool) {
	if w.outline.List.Selection == nil {
		return
	}
	key := ""
	extra := false
	for chosen := range w.navChosen.All() {
		if key == "" {
			key = chosen
		} else {
			extra = true
		}
	}
	if extra {
		if w.navSelected >= 0 && w.navSelected < w.outline.Rows() {
			key = w.outline.Item(w.navSelected)
		}
		w.navChosen.Clear()
		if key != "" {
			w.navChosen.Add(key)
		}
	}
	clicked := w.rowClickedKey
	w.rowClickedKey = ""
	if clicked != "" {
		key = clicked
	} else if w.selectedNavKey != "" && !w.navChosen.Has(w.selectedNavKey) &&
		openChanged && key != "" && w.navIsAncestor(key, w.selectedNavKey) {
		w.navChosen.Clear()
		w.navChosen.Add(w.selectedNavKey)
		return
	}
	if key == w.selectedNavKey {
		return
	}
	if key == "" {
		w.selectedNavKey = ""
		w.cancelPendingSwitch()
		w.canvas.clearSelection()
		w.clearCopyFeedback()
		w.selectedTableID = ""
		w.selectedTableName = ""
		w.searchSelected = -1
		w.tableDetail = TableDetail{}
		w.hasTableDetail = false
		w.unplacedTable = false
		w.notifyChanged()
		return
	}
	if w.catalog == nil {
		return
	}
	w.selectedNavKey = key
	w.navChosen.Clear()
	w.navChosen.Add(key)
	node, ok := w.catalog.nodes[key]
	if !ok {
		return
	}
	if node.tableID != "" {
		w.LocateTable(node.tableID)
		return
	}
	w.cancelPendingSwitch()
	w.canvas.clearSelection()
	w.clearCopyFeedback()
	w.selectedTableID = ""
	w.selectedTableName = ""
	w.searchSelected = -1
	w.tableDetail = TableDetail{}
	w.hasTableDetail = false
	w.unplacedTable = false
	w.notifyChanged()
}

func (w *Workspace) refreshSelectedDetail() {
	if w.hasTableDetail && w.tableDetail.ID == w.selectedTableID {
		return
	}
	w.detailsScroll.ScrollTo(0, 0, ui.Animation{})
	w.tableDetail, w.hasTableDetail = w.detailIndex.resolve(w.selectedTableID)
	if w.hasTableDetail {
		w.selectedTableName = firstNonEmpty(w.tableDetail.Name, w.tableDetail.Code, w.tableDetail.ID)
	}
}

func (w *Workspace) captureOpen() {
	clear(w.openBefore)
	for key := range w.outline.Open.All() {
		w.openBefore[key] = struct{}{}
	}
}

func (w *Workspace) openChanged() bool {
	previous := len(w.openBefore)
	count := 0
	for key := range w.outline.Open.All() {
		count++
		delete(w.openBefore, key)
	}
	return count != previous || len(w.openBefore) != 0
}

func (w *Workspace) navIsAncestor(parent, child string) bool {
	if w.catalog == nil {
		return false
	}
	for key := w.catalog.parent[child]; key != ""; key = w.catalog.parent[key] {
		if key == parent {
			return true
		}
	}
	return false
}

func (w *Workspace) cancelPendingSwitch() {
	w.boardToken++
	w.switching = false
	w.pendingBoard = -1
	w.pendingTableID = ""
	w.pendingTableToken = 0
	w.pendingSmoothTable = false
	w.requestedTableID = ""
	w.requestedSmoothTable = false
}

func (w *Workspace) finishPendingOutlineSelection() {
	key := w.selectedNavKey
	if key == "" {
		w.navSelected = -1
		return
	}
	for offset := 0; offset < w.outline.Rows(); offset++ {
		if w.outline.Item(offset) == key {
			w.navSelected = offset
			return
		}
	}
}

func containsTableID(tables []string, tableID string) bool {
	for _, id := range tables {
		if id == tableID {
			return true
		}
	}
	return false
}

func (w *Workspace) notifyChanged() {
	if w.onChanged != nil {
		w.onChanged()
	}
}

func (w *Workspace) constrainPanes(total float32) {
	reserved := float32(canvasMinWidth)
	if !w.detailsHidden {
		reserved += detailsMinWidth + 1
	}
	if total < navigationMinWidth+reserved+1 {
		return
	}
	available := total - 1
	navMax := min(float32(navigationMaxWidth), max(float32(navigationMinWidth), available-reserved))
	navWidth := available * w.NavigationPercent / 100
	switch {
	case navWidth < navigationMinWidth-0.5:
		w.NavigationPercent = navigationMinWidth * 100 / available
	case navWidth > navMax+0.5:
		w.NavigationPercent = navMax * 100 / available
	}
	if w.detailsHidden {
		return
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
		t := c.Theme()
		lang := string(state.Locale())
		if workspace.preview.open {
			workspace.databasePreviewView(c, state, t, lang)
			return
		}
		width, _ := c.Size()
		workspace.constrainPanes(width)
		ui.Column(c.Key(RootKey)).Fill().Background(t.Background).Children(func() {
			ui.SplitPercent(c.Key(NavigationSplitKey), &workspace.NavigationPercent,
				func() {
					sidePane(c, state, i18n.KeyNavigationHeading, t, lang, navigationMinWidth, navigationMaxWidth, func() {
						workspace.diagramNavigation(c, state, t, lang)
					})
				},
				func() {
					canvas := func() {
						workspace.canvas.view(c, state, t, lang, func() {
							workspace.detailsToggle(c, state, lang)
						})
					}
					if workspace.detailsHidden {
						canvas()
						return
					}
					ui.SplitPercent(c.Key(DetailsSplitKey), &workspace.CanvasPercent,
						canvas,
						func() {
							workspace.detailsPane(c, state, t, lang)
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
		FillWidth().Disabled(w.loading || w.pdmSaveRequest != nil).OnClick(w.openHandler)
	ui.Button(c, state.Text(i18n.KeyDatabaseSQL, nil)).Key(DatabasePreviewOpenKey).
		FillWidth().Disabled(!w.hasModel || w.document == nil || w.loading).
		OnClick(w.openDatabasePreview)
	search := state.Text(i18n.KeySearch, nil)
	c.Key(SearchKey)
	field := ui.SearchField(c, &w.searchQuery).
		FillWidth().Placeholder(search).Label(search).
		Disabled(!w.hasModel || w.loading)
	if field.Changed() {
		w.requestSearch()
	}
	if len(w.boards) > 0 {
		paneHeading(c, state.Text(i18n.KeyDiagramsHeading, nil), t, lang)
		ui.Scroll(c.Key(DiagramListKey)).FillWidth().Height(96).KeepScrollbars(true).Children(func() {
			ui.Column(c).Role(ui.RoleTabList).Gap(4).Children(func() {
				for index, board := range w.boards {
					label := board.Name
					if label == "" {
						label = state.Text(i18n.KeyDiagramFallback, map[string]string{
							"index": fmt.Sprintf("%d", index),
						})
					}
					button := ui.Button(c, label).Key(DiagramButtonKey(index)).
						FillWidth().Role(ui.RoleTab).
						AccessSelected(index == w.activeBoard).
						OnClick(func() { w.ActivateBoard(index) })
					if index == w.activeBoard {
						button.Background(t.Accent).TextColor(t.AccentText).Bold()
					}
				}
			})
		})
	}
	if w.searchActive() {
		w.searchNavigation(c, state, t, lang)
	} else {
		w.modelNavigation(c, state, lang)
	}
	if status := w.statusText(state); status != "" {
		ui.Text(c, status).Key(LoadStatusKey).
			FillWidth().Wrap().MaxLines(3).Ellipsis("…").
			FontSize(11).TextColor(t.TextMuted).Lang(lang)
	}
}

func (w *Workspace) modelNavigation(c *ui.Context, state *app.State, lang string) {
	if w.catalog == nil {
		return
	}
	w.captureOpen()
	ui.Outline(c.Key(ModelTreeKey), &w.outline, []string{ModelRootKey}, w.catalog.children,
		func(key string) {
			node := w.catalog.nodes[key]
			label := node.label
			if key == ModelRootKey && label == "" {
				label = state.Text(i18n.KeyModelRoot, nil)
			}
			if ui.Text(c, label).SingleLine().Ellipsis("…").Lang(lang).RowClicked() {
				w.rowClickedKey = key
			}
		}).Grow(1).FillWidth()
	w.reconcileOutlineSelection(w.openChanged())
	w.finishPendingOutlineSelection()
}

func (w *Workspace) searchNavigation(c *ui.Context, state *app.State, t *ui.Theme, lang string) {
	w.searchList.Key = func(offset int) any { return w.searchResults[offset].TableID }
	w.searchList.Label = func(offset int) string { return w.searchResults[offset].Name }
	w.searchList.Selected = &w.searchSelected
	list := ui.List(c.Key(SearchResultsKey), &w.searchList, len(w.searchResults), func(offset int) {
		result := w.searchResults[offset]
		c.Key(SearchResultKey(result.TableID))
		ui.Row(c).FillWidth().Padding(4, 6).Label(result.Name).Role(ui.RoleOption).
			AccessSelected(offset == w.searchSelected).Children(func() {
			ui.Text(c, result.Name).SingleLine().Ellipsis("…").Lang(lang)
		})
	}).FillWidth().Grow(1).Label(state.Text(i18n.KeySearchResults, nil))
	if list.Changed() && w.searchSelected >= 0 && w.searchSelected < len(w.searchResults) {
		w.LocateSearchResult(w.searchResults[w.searchSelected].TableID)
	}
	if w.searchFinished && len(w.searchResults) == 0 {
		ui.Text(c, state.Text(i18n.KeySearchNoResults, nil)).
			FillWidth().FontSize(11).TextColor(t.TextMuted).
			Lang(lang).Role(ui.RoleStatus)
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
	lang string,
	minWidth, maxWidth float32,
	content func(),
) {
	ui.Column(c).Fill().Background(t.Surface).Padding(12).Gap(8).
		MinWidth(minWidth).MaxWidth(maxWidth).Clip().Children(func() {
		paneHeading(c, state.Text(key, nil), t, lang)
		content()
	})
}

func paneHeading(c *ui.Context, text string, t *ui.Theme, lang string) ui.Element {
	return ui.Text(c, text).FontSize(13).Bold().TextColor(t.Text).Lang(lang).Role(ui.RoleHeading).Level(2)
}

func statusBar(c *ui.Context, state *app.State, t *ui.Theme, lang string) {
	ui.Row(c).FillWidth().Height(52).Padding(8, 12).Background(t.Surface).Children(func() {
		ui.Text(c, state.StatusText()).Key(StatusKey).
			FillWidth().Wrap().MaxLines(2).Ellipsis("…").
			FontSize(11).TextColor(t.TextMuted).Lang(lang)
	})
}
