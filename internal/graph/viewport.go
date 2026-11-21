package graph

import "math"

const (
	// MinScale bounds the Phase 2 camera; MaxScale bounds every camera.
	MinScale = 0.02
	MaxScale = 8.0

	// diagramMinScale permits fitting uncalibrated PDM int64 coordinates.
	diagramMinScale = 1e-18
)

// Camera maps world units through a viewport-local DIP plane to a
// window-relative DIP rectangle. NewCamera starts at scale 1 with world
// (0,0) at the viewport-local origin, and the viewport's Min offsets local
// coordinates into window Screen coordinates.
type Camera struct {
	viewport Rect
	origin   Point
	scale    float64
	minScale float64
}

// NewCamera creates a camera for a window-content DIP rectangle. Non-finite
// coordinates and dimensions that collapse or run backwards are sanitized to
// a safe zero-sized viewport.
func NewCamera(viewport Rect) *Camera {
	c := &Camera{scale: 1, minScale: MinScale}
	c.SetViewport(viewport)
	return c
}

// NewDiagramCamera creates a camera for real PDM world coordinates. It keeps
// MaxScale but allows a much lower minimum so very wide integer-coordinate
// scenes can be fitted; this is still a DIP/world-unit ratio, not a physical
// unit calibration.
func NewDiagramCamera(viewport Rect) *Camera {
	c := &Camera{scale: 1, minScale: diagramMinScale}
	c.SetViewport(viewport)
	return c
}

// SetViewport changes the window-content DIP rectangle without changing the
// world point at the viewport-local origin or the scale.
func (c *Camera) SetViewport(viewport Rect) {
	c.viewport = orderedRect(viewport)
}

// Scale returns the current world-to-viewport DIP scale.
func (c *Camera) Scale() float64 {
	if c.scale == 0 {
		return 1
	}
	return c.scale
}

// WorldToViewport converts world units to viewport-local DIP.
func (c *Camera) WorldToViewport(p Point) Point {
	return Point{
		X: (p.X - c.origin.X) * c.Scale(),
		Y: (p.Y - c.origin.Y) * c.Scale(),
	}
}

// ViewportToWorld converts viewport-local DIP to world units.
func (c *Camera) ViewportToWorld(p Point) Point {
	return Point{
		X: c.origin.X + p.X/c.Scale(),
		Y: c.origin.Y + p.Y/c.Scale(),
	}
}

// ViewportToScreen converts viewport-local DIP to window-relative DIP.
func (c *Camera) ViewportToScreen(p Point) Point {
	return Point{X: p.X + c.viewport.Min.X, Y: p.Y + c.viewport.Min.Y}
}

// ScreenToViewport converts window-relative DIP to viewport-local DIP.
func (c *Camera) ScreenToViewport(p Point) Point {
	return Point{X: p.X - c.viewport.Min.X, Y: p.Y - c.viewport.Min.Y}
}

// WorldToScreen converts world DIP to window-relative DIP.
func (c *Camera) WorldToScreen(p Point) Point {
	return c.ViewportToScreen(c.WorldToViewport(p))
}

// ScreenToWorld converts window-relative DIP to world DIP.
func (c *Camera) ScreenToWorld(p Point) Point {
	return c.ViewportToWorld(c.ScreenToViewport(p))
}

// ViewportBounds returns the window-relative DIP rectangle of the viewport.
func (c *Camera) ViewportBounds() Rect {
	return c.viewport
}

// VisibleWorld returns the world rectangle shown by the viewport.
func (c *Camera) VisibleWorld() Rect {
	width, height := c.viewport.Width(), c.viewport.Height()
	return Rect{
		Min: c.origin,
		Max: Point{X: c.origin.X + width/c.Scale(), Y: c.origin.Y + height/c.Scale()},
	}
}

// Pan moves the camera by delta window DIP. Positive X/Y reveal world farther
// right/down; the visible world's origin moves by delta / Scale.
func (c *Camera) Pan(delta Point) {
	if !finite(delta.X) || !finite(delta.Y) {
		return
	}
	c.origin.X += delta.X / c.Scale()
	c.origin.Y += delta.Y / c.Scale()
}

// ZoomAt multiplies Scale by factor while keeping the world point under
// anchor fixed. anchor is in viewport-local DIP. Invalid factors are ignored;
// at a scale limit the anchor remains fixed using the clamped scale.
func (c *Camera) ZoomAt(anchor Point, factor float64) {
	if !finite(factor) || factor <= 0 || !finite(anchor.X) || !finite(anchor.Y) {
		return
	}
	worldAnchor := c.ViewportToWorld(anchor)
	next := c.clampScale(c.Scale() * factor)
	c.scale = next
	c.origin = Point{
		X: worldAnchor.X - anchor.X/next,
		Y: worldAnchor.Y - anchor.Y/next,
	}
}

// FitBounds centers bounds in the viewport and chooses the largest scale
// within the camera's limits that leaves padding window DIP on every edge
// where the bounds have extent. Padding that is invalid or consumes either
// viewport dimension is ignored.
// A zero-area point is centered at the current scale; one zero dimension fits
// through the other. A zero-sized viewport centers the bounds without
// changing scale.
func (c *Camera) FitBounds(bounds Rect, padding float64) {
	bounds = orderedRect(bounds)
	width, height := c.viewport.Width(), c.viewport.Height()
	if !finite(padding) || padding < 0 || padding >= min(width, height)/2 {
		padding = 0
	}
	boundsWidth, boundsHeight := bounds.Width(), bounds.Height()
	centerX, centerY := midpoint(bounds.Min.X, bounds.Max.X), midpoint(bounds.Min.Y, bounds.Max.Y)

	if width <= 0 || height <= 0 {
		c.origin = Point{X: centerX, Y: centerY}
		return
	}
	if boundsWidth <= 0 && boundsHeight <= 0 {
		c.origin = Point{X: centerX - width/(2*c.Scale()), Y: centerY - height/(2*c.Scale())}
		return
	}

	availableX, availableY := width-2*padding, height-2*padding
	scale := math.Inf(1)
	if boundsWidth > 0 && availableX > 0 {
		scale = min(scale, availableX/boundsWidth)
	}
	if boundsHeight > 0 && availableY > 0 {
		scale = min(scale, availableY/boundsHeight)
	}
	if !finite(scale) {
		scale = c.Scale()
	} else {
		scale = c.clampScale(scale)
	}
	c.scale = scale
	c.origin = Point{
		X: centerX - width/(2*scale),
		Y: centerY - height/(2*scale),
	}
}

// Fit forwards to FitBounds for Phase 2 compatibility.
func (c *Camera) Fit(bounds Rect, padding float64) {
	c.FitBounds(bounds, padding)
}

// ShowAll fits a scene's whole Bounds.
func (c *Camera) ShowAll(scene Scene, padding float64) {
	c.FitBounds(scene.Bounds, padding)
}

func orderedRect(r Rect) Rect {
	minX, minY := finiteOrZero(r.Min.X), finiteOrZero(r.Min.Y)
	maxX, maxY := finiteOrZero(r.Max.X), finiteOrZero(r.Max.Y)
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	return Rect{Min: Point{X: minX, Y: minY}, Max: Point{X: maxX, Y: maxY}}
}

func (c *Camera) clampScale(v float64) float64 {
	return min(max(v, c.minimumScale()), MaxScale)
}

func (c *Camera) minimumScale() float64 {
	if c.minScale > 0 {
		return c.minScale
	}
	return MinScale
}

func midpoint(a, b float64) float64 {
	if math.Signbit(a) == math.Signbit(b) {
		return a + (b-a)/2
	}
	return a/2 + b/2
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func finiteOrZero(v float64) float64 {
	if !finite(v) {
		return 0
	}
	return v
}
