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

func pointsNear(a, b Point) bool {
	return near(a.X, b.X) && near(a.Y, b.Y)
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
