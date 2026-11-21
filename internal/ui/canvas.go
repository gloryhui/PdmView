package appui

import (
	"math"

	"pdmview/internal/app"
	"pdmview/internal/graph"
	"pdmview/internal/i18n"
	"pdmview/internal/renderer"

	"github.com/egoist/mygo/ui"
)

const (
	fitCanvasKey  = "pdmview.canvas.fit"
	showAllKey    = "pdmview.canvas.all"
	zoomInKey     = "pdmview.canvas.zoom-in"
	zoomOutKey    = "pdmview.canvas.zoom-out"
	ResetViewKey  = "pdmview.canvas.reset"
	canvasPadding = 24

	searchCameraMotionKey = "pdmview.canvas.search-motion"
)

type cameraTween struct {
	fromOrigin graph.Point
	fromScale  float64
	toOrigin   graph.Point
	toScale    float64
	started    bool
}

type canvasView struct {
	scene            graph.Scene
	index            *graph.SpatialIndex
	diagram          graph.DiagramGraph
	camera           *graph.Camera
	renderer         *renderer.Canvas
	selection        graph.Hit
	hovered          graph.Hit
	viewport         graph.Rect
	pointerDown      bool
	dragging         bool
	lastPointer      graph.Point
	demo             bool
	initialFit       bool
	prepareAfterFit  bool
	cameraTween      *cameraTween
	onTableSelection func(string)
	onTableOpen      func(string)
}

func newCanvasView() *canvasView {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		panic(err)
	}
	index := graph.NewSpatialIndex(scene)
	camera := graph.NewCamera(graph.Rect{})
	camera.Pan(graph.Point{X: -canvasPadding, Y: -canvasPadding})
	return &canvasView{
		scene:     scene,
		index:     index,
		camera:    camera,
		renderer:  renderer.New(scene, index),
		selection: graph.Hit{Kind: graph.HitNone, Index: -1},
		hovered:   graph.Hit{Kind: graph.HitNone, Index: -1},
		demo:      true,
	}
}

func (view *canvasView) activatePrepared(prepared PreparedBoard) {
	viewport := view.viewport
	view.scene = prepared.Graph.Scene
	view.index = prepared.Index
	view.diagram = prepared.Graph
	view.renderer = prepared.Renderer
	view.selection = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.demo = false
	view.viewport = viewport
	view.camera = graph.NewDiagramCamera(viewport)
	view.pointerDown = false
	view.dragging = false
	view.lastPointer = graph.Point{}
	view.cameraTween = nil
	view.initialFit = viewport.Width() <= 0 || viewport.Height() <= 0
	view.prepareAfterFit = false
	if !view.initialFit {
		view.camera.FitBounds(view.scene.Bounds, canvasPadding)
		view.prepareVisible()
	}
}

func (view *canvasView) view(c *ui.Context, state *app.State, theme *ui.Theme, lang string) {
	if view.prepareAfterFit {
		view.prepareVisible()
		view.prepareAfterFit = false
	}
	if view.initialFit {
		c.AnimationFrame()
	}
	heading := state.Text(i18n.KeyCanvasHeading, nil)
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).FillWidth().Height(28).Padding(4, 8).AlignItems(ui.Center).Background(theme.Surface).Children(func() {
			paneHeading(c, heading, theme, lang)
		})
		ui.Row(c).FillWidth().Height(38).Padding(4, 8).Gap(4).
			AlignItems(ui.Center).Background(theme.Surface).
			Children(func() {
				ui.Button(c, "−").Key(zoomOutKey).Label(state.Text(i18n.KeyZoomOut, nil)).OnClick(func() { view.zoomCentered(1 / 1.25) })
				ui.Button(c, "+").Key(zoomInKey).Label(state.Text(i18n.KeyZoomIn, nil)).OnClick(func() { view.zoomCentered(1.25) })
				ui.Button(c, state.Text(i18n.KeyFitShort, nil)).Key(fitCanvasKey).
					Label(state.Text(i18n.KeyFitSelection, nil)).
					Disabled(view.selection.Kind == graph.HitNone).OnClick(view.fitSelection)
				ui.Button(c, state.Text(i18n.KeyShowAllShort, nil)).Key(showAllKey).
					Label(state.Text(i18n.KeyShowAll, nil)).
					OnClick(view.showAll)
				ui.Button(c, state.Text(i18n.KeyResetShort, nil)).Key(ResetViewKey).
					Label(state.Text(i18n.KeyResetView, nil)).
					OnClick(view.resetView)
			})
		canvas := ui.Box(c).Grow(1).FillWidth().Clip().Key(CanvasKey).Label(heading).
			HandleInput(view.handleInput).Draw(view.draw)
		view.animateCamera(canvas)
		if !canvas.Hovered() && !view.pointerDown {
			view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
		}
	})
}

func (view *canvasView) draw(painter *ui.Painter, area ui.Rect) {
	view.viewport = graph.Rect{
		Min: graph.Point{X: float64(area.X), Y: float64(area.Y)},
		Max: graph.Point{X: float64(area.X) + float64(area.W), Y: float64(area.Y) + float64(area.H)},
	}
	view.camera.SetViewport(view.viewport)
	if view.initialFit && area.W > 0 && area.H > 0 {
		view.camera.FitBounds(view.scene.Bounds, canvasPadding)
		view.initialFit = false
		view.prepareAfterFit = true
	}
	view.renderer.DrawWithHover(painter, area, view.camera, view.selection, view.hovered)
}

func (view *canvasView) selectedNode() *graph.Node {
	if view.selection.Kind == graph.HitNode {
		return &view.scene.Nodes[view.selection.Index]
	}
	return nil
}

func (view *canvasView) tableNodeOffset(tableID string) (int, bool) {
	if tableID == "" || view.demo {
		return 0, false
	}
	for offset := range view.scene.Nodes {
		source, ok := view.diagram.NodeSource(offset)
		if ok && source.TableID == tableID {
			return offset, true
		}
	}
	return 0, false
}

func (view *canvasView) selectTableNode(offset int) {
	if offset < 0 || offset >= len(view.scene.Nodes) {
		return
	}
	view.cameraTween = nil
	view.selection = graph.Hit{Kind: graph.HitNode, Index: offset}
	view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.camera.FitBounds(view.scene.Nodes[offset].Bounds, canvasPadding)
	view.prepareVisible()
}

func (view *canvasView) selectTableNodeSmooth(offset int) {
	if offset < 0 || offset >= len(view.scene.Nodes) {
		return
	}
	view.selection = graph.Hit{Kind: graph.HitNode, Index: offset}
	view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.beginCameraTween(view.scene.Nodes[offset].Bounds)
	view.prepareVisible()
}

func (view *canvasView) clearSelection() {
	view.cameraTween = nil
	view.selection = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
}

func (view *canvasView) zoomCentered(factor float64) {
	view.cameraTween = nil
	view.camera.ZoomAt(graph.Point{X: view.viewport.Width() / 2, Y: view.viewport.Height() / 2}, factor)
	view.prepareVisible()
}

func (view *canvasView) fitSelection() {
	view.cameraTween = nil
	switch view.selection.Kind {
	case graph.HitNode:
		view.camera.FitBounds(view.scene.Nodes[view.selection.Index].Bounds, canvasPadding)
	case graph.HitEdge:
		view.camera.FitBounds(view.scene.Edges[view.selection.Index].Bounds(), canvasPadding)
	}
	view.prepareVisible()
}

func (view *canvasView) showAll() {
	view.cameraTween = nil
	view.camera.FitBounds(view.scene.Bounds, canvasPadding)
	view.prepareVisible()
}

func (view *canvasView) resetView() {
	view.cameraTween = nil
	view.camera.FitBounds(view.scene.Bounds, canvasPadding)
	view.clearSelection()
	view.pointerDown = false
	view.dragging = false
	view.lastPointer = graph.Point{}
	view.prepareVisible()
	if view.onTableSelection != nil {
		view.onTableSelection("")
	}
}

func (view *canvasView) prepareVisible() {
	if view.renderer != nil {
		view.renderer.PrepareVisible(view.camera, view.selection)
	}
}

func (view *canvasView) handleInput(event ui.InputEvent) bool {
	pointer := graph.Point{X: float64(event.X), Y: float64(event.Y)}
	switch event.Kind {
	case ui.InputPointerDown:
		if event.Button != 0 {
			return false
		}
		view.cameraTween = nil
		view.pointerDown, view.dragging, view.lastPointer = true, false, pointer
	case ui.InputPointerMove:
		if !view.pointerDown {
			view.hovered = view.hitTest(pointer)
			return true
		}
		delta := graph.Point{X: view.lastPointer.X - pointer.X, Y: view.lastPointer.Y - pointer.Y}
		if view.dragging || math.Hypot(delta.X, delta.Y) >= 4 {
			view.cameraTween = nil
			view.camera.Pan(delta)
			view.lastPointer, view.dragging = pointer, true
			view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
		}
	case ui.InputPointerUp:
		if !view.pointerDown || event.Button != 0 {
			return false
		}
		wasDragging := view.dragging
		if !event.Canceled && !wasDragging {
			hit := view.hitTest(pointer)
			view.selection = hit
			view.hovered = view.selection
			view.prepareVisible()
			view.reportTableSelection()
			if event.Clicks == 2 && view.onTableOpen != nil {
				if tableID := view.tableID(hit); tableID != "" {
					view.onTableOpen(tableID)
				}
			}
		}
		view.pointerDown, view.dragging = false, false
		if wasDragging {
			view.hovered = view.hitTest(pointer)
			view.prepareVisible()
		}
	case ui.InputScroll:
		view.cameraTween = nil
		if event.Mods&ui.Ctrl != 0 {
			view.camera.ZoomAt(pointer, math.Exp(-float64(event.DY)*0.009))
		} else {
			view.camera.Pan(graph.Point{X: float64(event.DX), Y: float64(event.DY)})
		}
		view.hovered = view.hitTest(pointer)
		view.prepareVisible()
	case ui.InputZoom:
		view.cameraTween = nil
		view.camera.ZoomAt(pointer, float64(event.Scale))
		view.hovered = view.hitTest(pointer)
		view.prepareVisible()
	default:
		return false
	}
	return true
}

func (view *canvasView) beginCameraTween(bounds graph.Rect) {
	if view.camera == nil {
		return
	}
	from := view.camera.VisibleWorld()
	target := graph.NewDiagramCamera(view.camera.ViewportBounds())
	target.FitBounds(bounds, canvasPadding)
	to := target.VisibleWorld()
	view.cameraTween = &cameraTween{
		fromOrigin: from.Min,
		fromScale:  view.camera.Scale(),
		toOrigin:   to.Min,
		toScale:    target.Scale(),
	}
}

func (view *canvasView) animateCamera(canvas ui.Element) {
	tween := view.cameraTween
	if tween == nil {
		return
	}
	if !tween.started {
		canvas.Jump(searchCameraMotionKey, 0)
		tween.started = true
	}
	progress := float64(canvas.AnimateTo(searchCameraMotionKey, 1, ui.Smooth))
	view.setCameraPose(
		graph.Point{
			X: tween.fromOrigin.X + (tween.toOrigin.X-tween.fromOrigin.X)*progress,
			Y: tween.fromOrigin.Y + (tween.toOrigin.Y-tween.fromOrigin.Y)*progress,
		},
		tween.fromScale+(tween.toScale-tween.fromScale)*progress,
	)
	view.prepareVisible()
	if progress == 1 {
		view.cameraTween = nil
	}
}

func (view *canvasView) setCameraPose(origin graph.Point, scale float64) {
	if view.camera == nil || scale <= 0 {
		return
	}
	if current := view.camera.Scale(); current > 0 && current != scale {
		view.camera.ZoomAt(
			graph.Point{X: view.viewport.Width() / 2, Y: view.viewport.Height() / 2},
			scale/current,
		)
	}
	visible := view.camera.VisibleWorld().Min
	view.camera.Pan(graph.Point{
		X: (origin.X - visible.X) * view.camera.Scale(),
		Y: (origin.Y - visible.Y) * view.camera.Scale(),
	})
}

func (view *canvasView) reportTableSelection() {
	if view.onTableSelection == nil {
		return
	}
	view.onTableSelection(view.tableID(view.selection))
}

func (view *canvasView) tableID(hit graph.Hit) string {
	if view.demo || hit.Kind != graph.HitNode {
		return ""
	}
	if source, ok := view.diagram.NodeSource(hit.Index); ok {
		return source.TableID
	}
	return ""
}

func (view *canvasView) hitTest(pointer graph.Point) graph.Hit {
	if view.demo {
		return graph.HitTest(view.scene, view.index, view.camera, pointer, 5)
	}
	screen := view.camera.ViewportToScreen(pointer)
	return view.diagram.HitTest(view.index, view.camera, screen, 5).Hit
}
