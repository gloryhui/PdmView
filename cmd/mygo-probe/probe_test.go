package main

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestDefaultSceneAndHeadlessRender(t *testing.T) {
	probe := newProbeState(probeConfig{nodes: 2}, nil, nil)
	if got, want := len(probe.scene.nodes), 2; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if got, want := len(probe.scene.edges), 1; got != want {
		t.Fatalf("edges = %d, want %d", got, want)
	}
	if probe.multiText == nil {
		t.Fatal("multilingual TextLayout was not pre-shaped")
	}
	if w, h := probe.multiText.Size(); w <= 0 || h <= 0 {
		t.Fatalf("multilingual TextLayout size = %vx%v", w, h)
	}
	for i, node := range probe.scene.nodes {
		if node.title == nil || node.subtitle == nil {
			t.Fatalf("node %d has an unshaped retained layout", i)
		}
		if !node.hitPath.Contains(node.rect.X+node.rect.W/2, node.rect.Y+node.rect.H/2) {
			t.Fatalf("node %d hit path misses its center", i)
		}
	}

	tt := ui.NewTester(probe.view, 1400, 900)
	if !tt.HasText("2 nodes · 1 edges") {
		t.Fatalf("summary text missing; texts=%q", tt.Texts())
	}
	if probe.grid == nil {
		t.Fatal("Canvas grid tile was not drawn")
	}
	if w, h := probe.grid.Size(); w != gridTile || h != gridTile {
		t.Fatalf("Canvas size = %dx%d", w, h)
	}
	if probe.frames.Load() == 0 || probe.visibleNodes.Load() != 2 || probe.visibleEdges.Load() != 1 {
		t.Fatalf("headless frame stats: frames=%d visible=%d/%d",
			probe.frames.Load(), probe.visibleNodes.Load(), probe.visibleEdges.Load())
	}

	img := tt.Image()
	nonTransparent := false
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			nonTransparent = true
			break
		}
	}
	if !nonTransparent {
		t.Fatal("headless render is completely transparent")
	}
}

func TestNativeWheelDragAndClickHit(t *testing.T) {
	probe := newProbeState(probeConfig{nodes: 2}, nil, nil)
	tt := ui.NewTester(probe.view, 1400, 900)
	canvas, ok := tt.Find("Probe canvas")
	if !ok {
		t.Fatalf("canvas label not found; texts=%q", tt.Texts())
	}

	node := probe.scene.nodes[0]
	x := canvas.X + (node.rect.X+node.rect.W/2-probe.cameraX)*probe.zoom
	y := canvas.Y + (node.rect.Y+node.rect.H/2-probe.cameraY)*probe.zoom
	assertNodeFill(t, tt.Image(), canvas, probe, node.rect.X+24, node.rect.Y+node.rect.H-10)
	tt.ClickAt(x, y)
	if probe.selected != 0 {
		t.Fatalf("click selected %d, want 0; status=%q", probe.selected, probe.statusText())
	}
	if got := probe.statusText(); !strings.Contains(got, "Path.Contains") || !strings.Contains(got, "DEMO_TABLE_A") {
		t.Fatalf("click status = %q", got)
	}

	probe.zoomAtCenter(1.18)
	tt.Frame()
	node = probe.scene.nodes[1]
	x = canvas.X + (node.rect.X+node.rect.W/2-probe.cameraX)*probe.zoom
	y = canvas.Y + (node.rect.Y+node.rect.H/2-probe.cameraY)*probe.zoom
	assertNodeFill(t, tt.Image(), canvas, probe, node.rect.X+24, node.rect.Y+node.rect.H-10)
	probe.selected = -1
	tt.ClickAt(x, y)
	if probe.selected != 1 {
		t.Fatalf("zoomed click selected %d, want 1; status=%q", probe.selected, probe.statusText())
	}

	beforeY := probe.cameraY
	tt.Wheel(canvas.X+canvas.W/2, canvas.Y+canvas.H/2, 0, 64)
	if probe.cameraY <= beforeY {
		t.Fatalf("wheel did not pan: cameraY %v -> %v", beforeY, probe.cameraY)
	}

	beforeX, beforeY := probe.cameraX, probe.cameraY
	startX, startY := canvas.X+20, canvas.Y+canvas.H-28
	tt.Press(startX, startY)
	tt.Move(startX+48, startY+32)
	tt.Release(startX+48, startY+32)
	if probe.cameraX >= beforeX || probe.cameraY >= beforeY {
		t.Fatalf("drag did not pan: camera %v,%v -> %v,%v", beforeX, beforeY, probe.cameraX, probe.cameraY)
	}
	if got := probe.statusText(); !strings.Contains(got, "native drag") {
		t.Fatalf("drag status = %q", got)
	}

	beforeSelected := probe.selected
	tt.Press(canvas.X+24, canvas.Y+36)
	tt.CancelPress()
	if probe.selected != beforeSelected {
		t.Fatalf("canceled press selected %d, want unchanged %d", probe.selected, beforeSelected)
	}
	if got := probe.statusText(); !strings.Contains(got, "canceled") {
		t.Fatalf("cancel status = %q", got)
	}
}

func assertNodeFill(t *testing.T, img interface {
	RGBAAt(x, y int) color.RGBA
}, canvas ui.Rect, probe *probeState, worldX, worldY float32) {
	t.Helper()
	x := int(canvas.X + (worldX-probe.cameraX)*probe.zoom)
	y := int(canvas.Y + (worldY-probe.cameraY)*probe.zoom)
	c := img.RGBAAt(x, y)
	if c.R < 220 || c.G < 220 || c.B < 220 || c.A < 250 {
		t.Fatalf("table fill at world %.1f,%.1f lands on window %d,%d with color %+v", worldX, worldY, x, y, c)
	}
}

func TestActionsOnlyRunWhenClicked(t *testing.T) {
	pickCalls, clipboardCalls := 0, 0
	pick := func(parent *mygo.Window) ([]string, error) {
		pickCalls++
		if parent != nil {
			t.Fatalf("headless picker parent = %p, want nil", parent)
		}
		return []string{"/private/should-not-appear.pdm"}, nil
	}
	clipboard := func() (string, error) {
		clipboardCalls++
		return "Clipboard Unicode plus scoped-format round-trip OK; prior clipboard restored", nil
	}
	probe := newProbeState(probeConfig{nodes: 2}, pick, clipboard)
	tt := ui.NewTester(probe.view, 1400, 900)
	if pickCalls != 0 || clipboardCalls != 0 {
		t.Fatalf("native actions ran without input: picker=%d clipboard=%d", pickCalls, clipboardCalls)
	}

	if err := tt.Click("File..."); err != nil {
		t.Fatal(err)
	}
	if pickCalls != 1 {
		t.Fatalf("picker calls = %d, want 1", pickCalls)
	}
	if got := probe.statusText(); !strings.Contains(got, "1 selection") ||
		strings.Contains(got, "should-not-appear") || strings.Contains(got, ".pdm") {
		t.Fatalf("picker status leaked a path or was wrong: %q", got)
	}

	if err := tt.Click("Clipboard round-trip"); err != nil {
		t.Fatal(err)
	}
	if clipboardCalls != 1 {
		t.Fatalf("clipboard calls = %d, want 1", clipboardCalls)
	}
	if got := probe.statusText(); !strings.Contains(got, "prior clipboard restored") {
		t.Fatalf("clipboard status = %q", got)
	}
}

func TestDPIUpdatesFromPainterScale(t *testing.T) {
	probe := newProbeState(probeConfig{nodes: 2}, nil, nil)
	tt := ui.NewTester(probe.view, 1400, 900)
	tt.SetScale(2)
	tt.Frame()
	if got := probe.frameScale(); got != 2 {
		t.Fatalf("Painter.Scale = %v, want 2", got)
	}
	if !tt.HasText("DPI 192") {
		t.Fatalf("DPI text missing; texts=%q", tt.Texts())
	}
}

func TestBenchmarkScenesAreFixedAndDeterministic(t *testing.T) {
	for _, n := range []int{100, 300, 500} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			probe := newProbeState(probeConfig{
				nodes:        n,
				bench:        true,
				benchWarmup:  time.Millisecond,
				benchMeasure: time.Millisecond,
			}, nil, nil)
			if got := len(probe.scene.nodes); got != n {
				t.Fatalf("nodes = %d, want %d", got, n)
			}
			if got, want := len(probe.scene.edges), 2*n; got != want {
				t.Fatalf("edges = %d, want %d", got, want)
			}
			if probe.bench.totalNodes != n || probe.bench.totalEdges != 2*n {
				t.Fatalf("benchmark totals = %d/%d", probe.bench.totalNodes, probe.bench.totalEdges)
			}
			if probe.scene.nodes[0].rect != (ui.Rect{X: nodeOrigin, Y: nodeOrigin, W: nodeWidth, H: nodeHeight}) {
				t.Fatalf("first node geometry changed: %+v", probe.scene.nodes[0].rect)
			}
			last := probe.scene.nodes[n-1].rect
			col, row := (n-1)%nodeCols, (n-1)/nodeCols
			wantLast := ui.Rect{
				X: nodeOrigin + float32(col)*nodeStepX,
				Y: nodeOrigin + float32(row)*nodeStepY,
				W: nodeWidth,
				H: nodeHeight,
			}
			if last != wantLast {
				t.Fatalf("last node geometry = %+v, want %+v", last, wantLast)
			}
		})
	}
}
