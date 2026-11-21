package graph

import (
	"math"
	"testing"
)

func TestCameraTransformsWithOffsetAndNegativeCoordinates(t *testing.T) {
	cam := NewCamera(Rect{Min: Point{X: -25, Y: -40}, Max: Point{X: 175, Y: 60}})
	if got := cam.Scale(); got != 1 {
		t.Fatalf("initial scale = %v", got)
	}

	world := Point{X: 12.5, Y: -7.5}
	viewport := cam.WorldToViewport(world)
	if want := (Point{X: 12.5, Y: -7.5}); !pointsNear(viewport, want) {
		t.Fatalf("WorldToViewport = %+v, want %+v", viewport, want)
	}
	if got := cam.ViewportToWorld(viewport); !pointsNear(got, world) {
		t.Fatalf("ViewportToWorld = %+v, want %+v", got, world)
	}
	screen := cam.ViewportToScreen(viewport)
	if want := (Point{X: -12.5, Y: -47.5}); !pointsNear(screen, want) {
		t.Fatalf("ViewportToScreen = %+v, want %+v", screen, want)
	}
	if got := cam.ScreenToViewport(screen); !pointsNear(got, viewport) {
		t.Fatalf("ScreenToViewport = %+v, want %+v", got, viewport)
	}
	if got := cam.WorldToScreen(world); !pointsNear(got, screen) {
		t.Fatalf("WorldToScreen = %+v, want %+v", got, screen)
	}
	if got := cam.ScreenToWorld(screen); !pointsNear(got, world) {
		t.Fatalf("ScreenToWorld = %+v, want %+v", got, world)
	}

	cam.Pan(Point{X: 40, Y: -20})
	if got := cam.WorldToScreen(Point{X: 40, Y: -20}); !pointsNear(got, cam.viewport.Min) {
		t.Fatalf("panned origin screen = %+v, want viewport Min %+v", got, cam.viewport.Min)
	}
	negative := Point{X: -80.25, Y: -125.5}
	if got := cam.ScreenToWorld(cam.WorldToScreen(negative)); !pointsNear(got, negative) {
		t.Fatalf("negative world round trip = %+v, want %+v", got, negative)
	}
}

func TestCameraPanUsesWindowDIPAndVisibleWorld(t *testing.T) {
	cam := NewCamera(Rect{Min: Point{X: 100, Y: 50}, Max: Point{X: 500, Y: 350}})
	cam.ZoomAt(Point{X: 50, Y: 25}, 2)
	before := cam.VisibleWorld()
	cam.Pan(Point{X: 20, Y: -10})
	after := cam.VisibleWorld()
	if !pointsNear(after.Min, Point{X: before.Min.X + 10, Y: before.Min.Y - 5}) {
		t.Fatalf("visible origin after pan = %+v, want half the window delta from %+v", after.Min, before.Min)
	}
	if got := after.Width(); !near(got, 200) {
		t.Fatalf("visible width = %v, want 200", got)
	}
	if got := after.Height(); !near(got, 150) {
		t.Fatalf("visible height = %v, want 150", got)
	}

	unchanged := cam.VisibleWorld()
	cam.Pan(Point{X: math.NaN(), Y: 10})
	if got := cam.VisibleWorld(); got != unchanged {
		t.Fatalf("invalid pan changed visible world: %+v -> %+v", unchanged, got)
	}
}

func TestCameraZoomAnchorAndClamp(t *testing.T) {
	cam := NewCamera(Rect{Min: Point{X: 100, Y: 50}, Max: Point{X: 500, Y: 350}})
	anchor := Point{X: 37, Y: 83}
	worldAnchor := cam.ViewportToWorld(anchor)

	cam.ZoomAt(anchor, 2)
	if got := cam.Scale(); !near(got, 2) {
		t.Fatalf("scale = %v, want 2", got)
	}
	if got := cam.ViewportToWorld(anchor); !pointsNear(got, worldAnchor) {
		t.Fatalf("anchor moved at 2x: %+v -> %+v", worldAnchor, got)
	}

	cam.ZoomAt(anchor, 1000)
	if got := cam.Scale(); got != MaxScale {
		t.Fatalf("clamped max scale = %v, want %v", got, MaxScale)
	}
	if got := cam.ViewportToWorld(anchor); !pointsNear(got, worldAnchor) {
		t.Fatalf("anchor moved at max clamp: %+v -> %+v", worldAnchor, got)
	}

	cam.ZoomAt(anchor, 0.000001)
	if got := cam.Scale(); got != MinScale {
		t.Fatalf("clamped min scale = %v, want %v", got, MinScale)
	}
	if got := cam.ViewportToWorld(anchor); !pointsNear(got, worldAnchor) {
		t.Fatalf("anchor moved at min clamp: %+v -> %+v", worldAnchor, got)
	}

	scale, visible := cam.Scale(), cam.VisibleWorld()
	for _, factor := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		cam.ZoomAt(anchor, factor)
		if cam.Scale() != scale || cam.VisibleWorld() != visible {
			t.Fatalf("invalid factor %v changed camera", factor)
		}
	}
}

func TestCameraFitCentersWithPadding(t *testing.T) {
	cam := NewCamera(Rect{Min: Point{X: 100, Y: 50}, Max: Point{X: 900, Y: 650}})
	bounds := Rect{Min: Point{X: -200, Y: -100}, Max: Point{X: 400, Y: 300}}
	cam.Fit(bounds, 50)

	wantScale := 700.0 / 600.0
	if got := cam.Scale(); !near(got, wantScale) {
		t.Fatalf("fit scale = %v, want %v", got, wantScale)
	}
	center := Point{X: (bounds.Min.X + bounds.Max.X) / 2, Y: (bounds.Min.Y + bounds.Max.Y) / 2}
	if got := cam.WorldToScreen(center); !pointsNear(got, Point{X: 500, Y: 350}) {
		t.Fatalf("fitted center screen = %+v, want viewport center", got)
	}
	minScreen, maxScreen := cam.WorldToScreen(bounds.Min), cam.WorldToScreen(bounds.Max)
	if minScreen.X < 149.999 || maxScreen.X > 850.001 {
		t.Fatalf("horizontal padding not honored: %+v .. %+v", minScreen, maxScreen)
	}
	if minScreen.Y < 49.999 || maxScreen.Y > 650.001 {
		t.Fatalf("vertical bounds outside viewport: %+v .. %+v", minScreen, maxScreen)
	}
	cam.Fit(bounds, 320)
	if got, want := cam.Scale(), 800.0/600.0; !near(got, want) {
		t.Fatalf("fit with excessive padding = %v, want %v", got, want)
	}
}

func TestCameraFitDegenerateAndZeroViewport(t *testing.T) {
	cam := NewCamera(Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 200, Y: 100}})
	cam.ZoomAt(Point{}, 3)
	scale := cam.Scale()
	point := Rect{Min: Point{X: 10, Y: 20}, Max: Point{X: 10, Y: 20}}
	cam.Fit(point, -20)
	if cam.Scale() != scale {
		t.Fatalf("point fit changed scale to %v, want %v", cam.Scale(), scale)
	}
	if got := cam.WorldToScreen(Point{X: 10, Y: 20}); !pointsNear(got, Point{X: 100, Y: 50}) {
		t.Fatalf("point fit center = %+v, want 100,50", got)
	}

	cam.Fit(Rect{Min: Point{X: 0, Y: 0}, Max: Point{X: 0, Y: 50}}, 0)
	if got := cam.Scale(); !near(got, 2) {
		t.Fatalf("one-axis fit scale = %v, want 2", got)
	}
	if got := cam.WorldToScreen(Point{X: 0, Y: 25}); !pointsNear(got, Point{X: 100, Y: 50}) {
		t.Fatalf("one-axis fit center = %+v", got)
	}

	zero := NewCamera(Rect{})
	zero.Fit(Rect{Min: Point{X: -50, Y: -60}, Max: Point{X: -10, Y: -20}}, 0)
	if zero.Scale() != 1 || !pointsNear(zero.WorldToViewport(Point{X: -30, Y: -40}), Point{}) {
		t.Fatalf("zero viewport fit: scale=%v center=%+v", zero.Scale(), zero.WorldToViewport(Point{X: -30, Y: -40}))
	}
}

func TestCameraShowAllFits500NodeScene(t *testing.T) {
	scene, err := GenerateDefault(500, DensitySparse)
	if err != nil {
		t.Fatal(err)
	}
	cam := NewCamera(Rect{Min: Point{X: 30, Y: 40}, Max: Point{X: 830, Y: 640}})
	cam.ShowAll(scene, 40)
	if cam.Scale() < MinScale || cam.Scale() > MaxScale {
		t.Fatalf("show-all scale %v outside limits", cam.Scale())
	}
	minScreen, maxScreen := cam.WorldToScreen(scene.Bounds.Min), cam.WorldToScreen(scene.Bounds.Max)
	if minScreen.X < 29.999 || minScreen.Y < 39.999 || maxScreen.X > 830.001 || maxScreen.Y > 640.001 {
		t.Fatalf("500-node scene does not fit: %+v .. %+v", minScreen, maxScreen)
	}

	direct := NewCamera(Rect{Min: Point{X: 30, Y: 40}, Max: Point{X: 830, Y: 640}})
	direct.Fit(scene.Bounds, 40)
	if direct.Scale() != cam.Scale() || direct.VisibleWorld() != cam.VisibleWorld() {
		t.Fatalf("ShowAll differs from Fit: scale %v/%v visible %+v/%+v",
			cam.Scale(), direct.Scale(), cam.VisibleWorld(), direct.VisibleWorld())
	}
}

func TestCameraSafeInvalidViewport(t *testing.T) {
	cam := NewCamera(Rect{
		Min: Point{X: math.NaN(), Y: 10},
		Max: Point{X: -5, Y: math.Inf(1)},
	})
	if got := cam.ViewportToScreen(Point{X: 3, Y: 4}); !pointsNear(got, Point{X: 3, Y: 14}) {
		t.Fatalf("safe viewport screen transform = %+v", got)
	}
	cam.SetViewport(Rect{Min: Point{X: -10, Y: -20}, Max: Point{X: -30, Y: -40}})
	if got := cam.ViewportToScreen(Point{X: 2, Y: 3}); !pointsNear(got, Point{X: -8, Y: -17}) {
		t.Fatalf("collapsed viewport offset = %+v", got)
	}
}

func TestDiagramCameraViewportAndCoordinateRoundTrip(t *testing.T) {
	viewport := Rect{Min: Point{X: -30, Y: 40}, Max: Point{X: 170, Y: 140}}
	cam := NewDiagramCamera(viewport)
	if cam.Scale() != 1 {
		t.Fatalf("initial diagram scale = %v", cam.Scale())
	}
	if got := cam.ViewportBounds(); got != viewport {
		t.Fatalf("view bounds = %+v, want %+v", got, viewport)
	}

	for _, factor := range []float64{0.25, 4, 1e-6} {
		anchor := Point{X: 37, Y: 83}
		worldAnchor := cam.ViewportToWorld(anchor)
		cam.ZoomAt(anchor, factor)
		if got := cam.ViewportToWorld(anchor); !pointsNearScaled(got, worldAnchor, 1e-9) {
			t.Fatalf("zoom %v moved anchor: %+v -> %+v", factor, worldAnchor, got)
		}
		for _, world := range []Point{
			{X: -9_000_000_000, Y: 4_000_000_000},
			{X: 12.5, Y: -7.5},
			{X: 8_000_000_000_000, Y: -3_000_000_000_000},
		} {
			screen := cam.WorldToScreen(world)
			if got := cam.ScreenToWorld(screen); !pointsNearScaled(got, world, 1e-9) {
				t.Fatalf("screen/world round trip by %v = %+v -> %+v", factor, world, got)
			}
			viewportPoint := cam.ScreenToViewport(screen)
			if got := cam.ViewportToScreen(viewportPoint); !pointsNear(got, screen) {
				t.Fatalf("viewport/screen round trip = %+v -> %+v", screen, got)
			}
		}
	}

	cam.SetViewport(Rect{Min: Point{X: 500, Y: 600}, Max: Point{X: 700, Y: 800}})
	if got := cam.ViewportBounds(); got != (Rect{Min: Point{X: 500, Y: 600}, Max: Point{X: 700, Y: 800}}) {
		t.Fatalf("updated view bounds = %+v", got)
	}
}

func TestDiagramCameraFitBoundsWideSpanPaddingAndLimits(t *testing.T) {
	viewport := Rect{Min: Point{X: 100, Y: 50}, Max: Point{X: 900, Y: 650}}
	diagram := NewDiagramCamera(viewport)
	bounds := Rect{Min: Point{X: -4e20, Y: -4e20}, Max: Point{X: 4e20, Y: 4e20}}
	diagram.FitBounds(bounds, 40)
	if got := diagram.Scale(); got != diagramMinScale {
		t.Fatalf("wide diagram scale = %v, want %v", got, diagramMinScale)
	}
	center := Point{X: midpoint(bounds.Min.X, bounds.Max.X), Y: midpoint(bounds.Min.Y, bounds.Max.Y)}
	if got := diagram.WorldToScreen(center); !pointsNear(got, Point{X: 500, Y: 350}) {
		t.Fatalf("wide diagram center = %+v", got)
	}

	fitBounds := Rect{Min: Point{X: -4e17, Y: -2e17}, Max: Point{X: 4e17, Y: 2e17}}
	diagram.FitBounds(fitBounds, 40)
	if got, want := diagram.Scale(), 720.0/8e17; !nearRelative(got, want, 1e-12) {
		t.Fatalf("padded wide scale = %v, want %v", got, want)
	}
	minScreen := diagram.WorldToScreen(fitBounds.Min)
	maxScreen := diagram.WorldToScreen(fitBounds.Max)
	if minScreen.X < 139.999 || maxScreen.X > 860.001 ||
		minScreen.Y < 89.999 || maxScreen.Y > 610.001 {
		t.Fatalf("wide padding not honored: %+v .. %+v", minScreen, maxScreen)
	}

	legacy := NewCamera(viewport)
	legacy.FitBounds(bounds, 40)
	if got := legacy.Scale(); got != MinScale {
		t.Fatalf("legacy wide-fit scale = %v, want %v", got, MinScale)
	}

	anchor := Point{X: 13, Y: 29}
	worldAnchor := diagram.ViewportToWorld(anchor)
	diagram.ZoomAt(anchor, 1e-30)
	if diagram.Scale() != diagramMinScale {
		t.Fatalf("diagram min zoom = %v", diagram.Scale())
	}
	if got := diagram.ViewportToWorld(anchor); !pointsNearScaled(got, worldAnchor, 1e-12) {
		t.Fatalf("diagram min zoom moved anchor: %+v -> %+v", worldAnchor, got)
	}
	diagram.ZoomAt(anchor, 1e30)
	if diagram.Scale() != MaxScale {
		t.Fatalf("diagram max zoom = %v", diagram.Scale())
	}
	if got := diagram.ViewportToWorld(anchor); !pointsNearScaled(got, worldAnchor, 1e-12) {
		t.Fatalf("diagram max zoom moved anchor: %+v -> %+v", worldAnchor, got)
	}

	var zero Camera
	zero.SetViewport(viewport)
	zeroAnchorWorld := zero.ViewportToWorld(anchor)
	zero.ZoomAt(anchor, 1e-30)
	if zero.Scale() != MinScale {
		t.Fatalf("zero camera min scale = %v, want %v", zero.Scale(), MinScale)
	}
	if got := zero.ViewportToWorld(anchor); !pointsNear(got, zeroAnchorWorld) {
		t.Fatalf("zero camera moved anchor: %+v -> %+v", zeroAnchorWorld, got)
	}
}

func TestFitBoundsSafeMidpointAndShowAll(t *testing.T) {
	if got := midpoint(1e308, 1.2e308); math.IsInf(got, 0) || !nearRelative(got, 1.1e308, 1e-12) {
		t.Fatalf("same-sign midpoint = %+v", got)
	}
	if got := midpoint(-1e308, 1e308); got != 0 {
		t.Fatalf("opposite-sign midpoint = %v", got)
	}

	cam := NewDiagramCamera(Rect{Min: Point{X: 10, Y: 20}, Max: Point{X: 110, Y: 120}})
	cam.FitBounds(Rect{Min: Point{X: 1e308, Y: 1e308}, Max: Point{X: 1.2e308, Y: 1.2e308}}, 0)
	if math.IsInf(cam.origin.X, 0) || math.IsInf(cam.origin.Y, 0) ||
		math.IsNaN(cam.origin.X) || math.IsNaN(cam.origin.Y) {
		t.Fatalf("fit midpoint overflowed origin: %+v", cam.origin)
	}

	scene := Scene{Bounds: Rect{Min: Point{X: -4e20, Y: -4e20}, Max: Point{X: 4e20, Y: 4e20}}}
	show := NewDiagramCamera(cam.ViewportBounds())
	show.ShowAll(scene, 0)
	direct := NewDiagramCamera(cam.ViewportBounds())
	direct.FitBounds(scene.Bounds, 0)
	if show.Scale() != direct.Scale() || show.VisibleWorld() != direct.VisibleWorld() {
		t.Fatalf("ShowAll differs from FitBounds: %+v / %+v", show, direct)
	}
}

func pointsNear(a, b Point) bool {
	return near(a.X, b.X) && near(a.Y, b.Y)
}

func pointsNearScaled(a, b Point, tolerance float64) bool {
	scale := max(1, max(math.Abs(a.X), max(math.Abs(a.Y), max(math.Abs(b.X), math.Abs(b.Y)))))
	return math.Abs(a.X-b.X) <= tolerance*scale && math.Abs(a.Y-b.Y) <= tolerance*scale
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

func nearRelative(a, b, tolerance float64) bool {
	scale := max(math.Abs(a), math.Abs(b))
	if scale == 0 {
		return true
	}
	return math.Abs(a-b) <= tolerance*scale
}
