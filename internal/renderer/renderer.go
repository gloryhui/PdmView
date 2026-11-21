// Package renderer draws a graph.Scene into a MyGo native Canvas.
package renderer

import (
	"math"

	"pdmview/internal/graph"

	"github.com/egoist/mygo/ui"
)

type LOD string

const (
	Far    LOD = "far"
	Medium LOD = "medium"
	Near   LOD = "near"
)

const (
	farScale          = 0.36
	mediumEnterScale  = 0.44
	nearEnterScale    = 0.85
	nearExitScale     = 0.75
	gridTileDIP       = 64.0
	gridPixelMin      = 32
	gridPixelMax      = 256
	edgeStrokeWidth   = 1.5
	selectedLineWidth = 2.5
	fieldRowHeight    = 18.0
	maxFieldRows      = 5
)

// FrameStats describes one Draw call. CacheHits and CacheMisses count only
// the single reusable grid-tile lookup; retained title and field TextLayouts
// are not counted as cache hits or misses.
type FrameStats struct {
	VisibleNodes int
	VisibleEdges int
	FieldRows    int
	CacheHits    int
	CacheMisses  int
	LOD          LOD
}

type nodeVisual struct {
	title  *ui.TextLayout
	fields []*ui.TextLayout
}

type edgeVisual struct {
	points [][2]float32
}

type gridKey struct {
	scaleBits uint32
	dark      bool
	bg        uint32
	border    uint32
}

// Canvas is a retained scene renderer. It owns immutable-shaped text and
// reusable query/grid buffers; callers mutate only the Camera and selection.
type Canvas struct {
	scene         graph.Scene
	index         graph.SpatialQuery
	nodes         []nodeVisual
	edges         []edgeVisual
	lod           LOD
	grid          *ui.Canvas
	gridKey       gridKey
	hasGrid       bool
	lastGridTiles int

	nodeOffsets []int
	edgeOffsets []int
}

// New builds retained layouts and converted polyline data. index may be nil,
// in which case a linear query baseline is used.
func New(scene graph.Scene, index graph.SpatialQuery) *Canvas {
	if index == nil {
		index = graph.NewLinearIndex(scene)
	}
	canvas := &Canvas{
		scene: scene,
		index: index,
		nodes: make([]nodeVisual, len(scene.Nodes)),
		edges: make([]edgeVisual, len(scene.Edges)),
	}
	titleFont := ui.Font{Size: 11.5, Weight: 650}
	fieldFont := ui.Font{Size: 9.5, Weight: 400}
	for i, node := range scene.Nodes {
		title := ui.ShapeText(node.Name, titleFont, 0)
		fields := node.Fields
		if len(fields) > maxFieldRows {
			fields = fields[:maxFieldRows]
		}
		layouts := make([]*ui.TextLayout, len(fields))
		for j, field := range fields {
			layouts[j] = ui.ShapeText(field.Name, fieldFont, 0)
		}
		canvas.nodes[i] = nodeVisual{title: title, fields: layouts}
	}
	for i, edge := range scene.Edges {
		points := make([][2]float32, len(edge.Points))
		for j, point := range edge.Points {
			points[j] = [2]float32{float32(point.X), float32(point.Y)}
		}
		canvas.edges[i] = edgeVisual{points: points}
	}
	return canvas
}

// Draw paints the visible scene inside area. camera's viewport is expected to
// describe the same content rectangle; its world transform determines the
// world-to-screen mapping.
func (c *Canvas) Draw(p *ui.Painter, area ui.Rect, camera *graph.Camera, selected graph.Hit) FrameStats {
	stats := FrameStats{}
	if p == nil || camera == nil || area.W <= 0 || area.H <= 0 {
		return stats
	}
	stats.LOD = c.updateLOD(camera.Scale())
	t := p.Theme()
	visible := camera.VisibleWorld()
	screenOrigin := camera.ViewportToScreen(graph.Point{})

	p.Clip(area, 0, func() {
		p.Fill(area, t.Background, 0)
		c.drawGrid(p, area, t, &stats)
		transform := ui.Translation(float32(-visible.Min.X), float32(-visible.Min.Y)).
			Then(ui.Scaling(float32(camera.Scale()), float32(camera.Scale()))).
			Then(ui.Translation(float32(screenOrigin.X), float32(screenOrigin.Y)))
		p.Transform(transform, 0, 0, func() {
			edgePadding := selectedLineWidth / 2 / camera.Scale()
			edgeArea := graph.Rect{
				Min: graph.Point{X: visible.Min.X - edgePadding, Y: visible.Min.Y - edgePadding},
				Max: graph.Point{X: visible.Max.X + edgePadding, Y: visible.Max.Y + edgePadding},
			}
			c.edgeOffsets = c.index.QueryEdges(edgeArea, c.edgeOffsets[:0])
			c.nodeOffsets = c.index.QueryNodes(visible, c.nodeOffsets[:0])
			stats.VisibleEdges = len(c.edgeOffsets)
			stats.VisibleNodes = len(c.nodeOffsets)

			for _, offset := range c.edgeOffsets {
				if offset < 0 || offset >= len(c.scene.Edges) {
					continue
				}
				c.drawEdge(p, offset, selected, t)
			}
			for _, offset := range c.nodeOffsets {
				if offset < 0 || offset >= len(c.scene.Nodes) {
					continue
				}
				stats.FieldRows += c.drawNode(p, offset, selected, t, stats.LOD)
			}
		})
	})
	return stats
}

func (c *Canvas) updateLOD(scale float64) LOD {
	if c.lod == "" {
		switch {
		case scale > nearEnterScale:
			c.lod = Near
		case scale >= farScale:
			c.lod = Medium
		default:
			c.lod = Far
		}
		return c.lod
	}
	switch c.lod {
	case Far:
		switch {
		case scale > nearEnterScale:
			c.lod = Near
		case scale > mediumEnterScale:
			c.lod = Medium
		}
	case Medium:
		switch {
		case scale < farScale:
			c.lod = Far
		case scale > nearEnterScale:
			c.lod = Near
		}
	case Near:
		switch {
		case scale < farScale:
			c.lod = Far
		case scale < nearExitScale:
			c.lod = Medium
		}
	}
	return c.lod
}

func (c *Canvas) drawGrid(p *ui.Painter, area ui.Rect, t *ui.Theme, stats *FrameStats) {
	key := makeGridKey(t, p.Scale())
	if !c.hasGrid || c.grid == nil || key != c.gridKey {
		c.grid = ui.NewCanvas(gridPixels(p.Scale()), gridPixels(p.Scale()))
		c.grid.Draw(p, func(cp *ui.Painter) {
			size := float32(gridPixels(p.Scale()))
			cp.Fill(ui.Rect{W: size, H: size}, t.Background, 0)
			line := t.Background.Mix(t.Border, 0.28)
			cp.Line(0, 0, size, 0, 1, line)
			cp.Line(0, 0, 0, size, 1, line)
		})
		c.gridKey, c.hasGrid = key, true
		stats.CacheMisses++
	} else {
		stats.CacheHits++
	}

	c.lastGridTiles = 0
	for y := area.Y; y < area.Y+area.H; y += gridTileDIP {
		for x := area.X; x < area.X+area.W; x += gridTileDIP {
			p.Image(c.grid.Bitmap(), ui.Rect{
				X: float32(x), Y: float32(y),
				W: gridTileDIP, H: gridTileDIP,
			}, ui.FillBox)
			c.lastGridTiles++
		}
	}
}

func (c *Canvas) drawEdge(p *ui.Painter, offset int, selected graph.Hit, t *ui.Theme) {
	width, color := float32(edgeStrokeWidth), t.Border
	if selected.Kind == graph.HitEdge && selected.Index == offset {
		width, color = selectedLineWidth, t.Accent
	}
	p.StrokePolyline(c.edges[offset].points, width, color)
}

func (c *Canvas) drawNode(p *ui.Painter, offset int, selected graph.Hit, t *ui.Theme, lod LOD) int {
	node := c.scene.Nodes[offset]
	rect := ui.Rect{
		X: float32(node.Bounds.Min.X),
		Y: float32(node.Bounds.Min.Y),
		W: float32(node.Bounds.Width()),
		H: float32(node.Bounds.Height()),
	}
	border, width := t.Border, float32(1)
	if selected.Kind == graph.HitNode && selected.Index == offset {
		border, width = t.Accent, selectedLineWidth
	}
	p.Fill(rect, t.Surface, 5)
	p.Stroke(rect, border, 5, width)
	if lod == Far {
		return 0
	}

	rows := len(c.nodes[offset].fields)
	if lod == Medium {
		rows = min(rows, 2)
	}
	p.Clip(rect, 5, func() {
		p.TextLayout(c.nodes[offset].title, rect.X+8, rect.Y+8, t.Text)
		for i := 0; i < rows; i++ {
			p.TextLayout(c.nodes[offset].fields[i], rect.X+8, rect.Y+28+float32(i)*fieldRowHeight, t.TextMuted)
		}
	})
	return rows
}

func gridPixels(scale float32) int {
	pixels := int(math.Round(float64(gridTileDIP * scale)))
	return min(max(pixels, gridPixelMin), gridPixelMax)
}

func makeGridKey(t *ui.Theme, scale float32) gridKey {
	return gridKey{
		scaleBits: math.Float32bits(scale),
		dark:      t.Dark,
		bg:        colorBits(t.Background),
		border:    colorBits(t.Border),
	}
}

func colorBits(c ui.Color) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}
