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
	canvasPadding = 24
)

type canvasView struct {
	scene       graph.Scene
	index       graph.SpatialQuery
	camera      *graph.Camera
	renderer    *renderer.Canvas
	selection   graph.Hit
	viewport    graph.Rect
	pointerDown bool
	dragging    bool
	lastPointer graph.Point
}

func newCanvasView() *canvasView {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		panic(err)
	}
	index := graph.NewSpatialIndex(scene)
	camera := graph.NewCamera(graph.Rect{})
	camera.Pan(graph.Point{X: -canvasPadding, Y: -canvasPadding})
	return &canvasView{scene: scene, index: index, camera: camera, renderer: renderer.New(scene, index)}
}

func (view *canvasView) view(c *ui.Context, state *app.State, theme *ui.Theme, lang string) {
	heading := state.Text(i18n.KeyCanvasHeading, nil)
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).FillWidth().Height(38).Padding(4, 8).Gap(4).AlignItems(ui.Center).Background(theme.Surface).Children(func() {
			paneHeading(c, heading, theme, lang)
			ui.Box(c).Grow(1)
			ui.Button(c, "−").Key(zoomOutKey).Label(state.Text(i18n.KeyZoomOut, nil)).OnClick(func() { view.zoomCentered(1 / 1.25) })
			ui.Button(c, "+").Key(zoomInKey).Label(state.Text(i18n.KeyZoomIn, nil)).OnClick(func() { view.zoomCentered(1.25) })
			ui.Button(c, state.Text(i18n.KeyFitSelection, nil)).Key(fitCanvasKey).
				Disabled(view.selection.Kind == graph.HitNone).OnClick(view.fitSelection)
			ui.Button(c, state.Text(i18n.KeyShowAll, nil)).Key(showAllKey).
				OnClick(func() { view.camera.ShowAll(view.scene, canvasPadding) })
		})
		ui.Box(c).Grow(1).FillWidth().Clip().Key(CanvasKey).Label(heading).
			HandleInput(view.handleInput).Draw(view.draw)
	})
}

func (view *canvasView) draw(painter *ui.Painter, area ui.Rect) {
	view.viewport = graph.Rect{
		Min: graph.Point{X: float64(area.X), Y: float64(area.Y)},
		Max: graph.Point{X: float64(area.X) + float64(area.W), Y: float64(area.Y) + float64(area.H)},
	}
	view.camera.SetViewport(view.viewport)
	view.renderer.Draw(painter, area, view.camera, view.selection)
}

func (view *canvasView) selectedNode() *graph.Node {
	if view.selection.Kind == graph.HitNode {
		return &view.scene.Nodes[view.selection.Index]
	}
	return nil
}

func (view *canvasView) zoomCentered(factor float64) {
	view.camera.ZoomAt(graph.Point{X: view.viewport.Width() / 2, Y: view.viewport.Height() / 2}, factor)
}

func (view *canvasView) fitSelection() {
	switch view.selection.Kind {
	case graph.HitNode:
		view.camera.Fit(view.scene.Nodes[view.selection.Index].Bounds, canvasPadding)
	case graph.HitEdge:
		view.camera.Fit(view.scene.Edges[view.selection.Index].Bounds(), canvasPadding)
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
			return false
		}
		delta := graph.Point{X: view.lastPointer.X - pointer.X, Y: view.lastPointer.Y - pointer.Y}
		if view.dragging || math.Hypot(delta.X, delta.Y) >= 4 {
			view.camera.Pan(delta)
			view.lastPointer, view.dragging = pointer, true
		}
	case ui.InputPointerUp:
		if !view.pointerDown || event.Button != 0 {
			return false
		}
		if !event.Canceled && !view.dragging {
			view.selection = graph.HitTest(view.scene, view.index, view.camera, pointer, 5)
		}
		view.pointerDown, view.dragging = false, false
	case ui.InputScroll:
		if event.Mods&ui.Ctrl != 0 {
			view.camera.ZoomAt(pointer, math.Exp(-float64(event.DY)*0.009))
		} else {
			view.camera.Pan(graph.Point{X: float64(event.DX), Y: float64(event.DY)})
		}
	case ui.InputZoom:
		view.camera.ZoomAt(pointer, float64(event.Scale))
	default:
		return false
	}
	return true
}
