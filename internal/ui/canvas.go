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
)

type canvasView struct {
	scene           graph.Scene
	index           *graph.SpatialIndex
	diagram         graph.DiagramGraph
	camera          *graph.Camera
	renderer        *renderer.Canvas
	selection       graph.Hit
	hovered         graph.Hit
	viewport        graph.Rect
	pointerDown     bool
	dragging        bool
	lastPointer     graph.Point
	demo            bool
	initialFit      bool
	prepareAfterFit bool
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

func (view *canvasView) zoomCentered(factor float64) {
	view.camera.ZoomAt(graph.Point{X: view.viewport.Width() / 2, Y: view.viewport.Height() / 2}, factor)
	view.prepareVisible()
}

func (view *canvasView) fitSelection() {
	switch view.selection.Kind {
	case graph.HitNode:
		view.camera.FitBounds(view.scene.Nodes[view.selection.Index].Bounds, canvasPadding)
	case graph.HitEdge:
		view.camera.FitBounds(view.scene.Edges[view.selection.Index].Bounds(), canvasPadding)
	}
	view.prepareVisible()
}

func (view *canvasView) showAll() {
	view.camera.FitBounds(view.scene.Bounds, canvasPadding)
	view.prepareVisible()
}

func (view *canvasView) resetView() {
	view.camera.FitBounds(view.scene.Bounds, canvasPadding)
	view.selection = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.hovered = graph.Hit{Kind: graph.HitNone, Index: -1}
	view.pointerDown = false
	view.dragging = false
	view.lastPointer = graph.Point{}
	view.prepareVisible()
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
		view.pointerDown, view.dragging, view.lastPointer = true, false, pointer
	case ui.InputPointerMove:
		if !view.pointerDown {
			view.hovered = view.hitTest(pointer)
			return true
		}
		delta := graph.Point{X: view.lastPointer.X - pointer.X, Y: view.lastPointer.Y - pointer.Y}
		if view.dragging || math.Hypot(delta.X, delta.Y) >= 4 {
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
			view.selection = view.hitTest(pointer)
			view.hovered = view.selection
			view.prepareVisible()
		}
		view.pointerDown, view.dragging = false, false
		if wasDragging {
			view.hovered = view.hitTest(pointer)
			view.prepareVisible()
		}
	case ui.InputScroll:
		if event.Mods&ui.Ctrl != 0 {
			view.camera.ZoomAt(pointer, math.Exp(-float64(event.DY)*0.009))
		} else {
			view.camera.Pan(graph.Point{X: float64(event.DX), Y: float64(event.DY)})
		}
		view.hovered = view.hitTest(pointer)
		view.prepareVisible()
	case ui.InputZoom:
		view.camera.ZoomAt(pointer, float64(event.Scale))
		view.hovered = view.hitTest(pointer)
		view.prepareVisible()
	default:
		return false
	}
	return true
}

func (view *canvasView) hitTest(pointer graph.Point) graph.Hit {
	if view.demo {
		return graph.HitTest(view.scene, view.index, view.camera, pointer, 5)
	}
	screen := view.camera.ViewportToScreen(pointer)
	return view.diagram.HitTest(view.index, view.camera, screen, 5).Hit
}
