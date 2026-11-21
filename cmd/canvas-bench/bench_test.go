package main

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"pdmview/internal/graph"

	"github.com/egoist/mygo/ui"
)

func TestParseConfigDefaultsAndValidation(t *testing.T) {
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultConfig()
	if cfg != want {
		t.Fatalf("defaults = %+v, want %+v", cfg, want)
	}
	if cfg.Seed != 20261121 || cfg.Nodes != 500 || cfg.Density != graph.DensityDense {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	cfg, err = ParseConfig([]string{
		"-nodes", "50", "-density", "sparse", "-seed", "7",
		"-width", "800", "-height", "600",
		"-warmup", "500ms", "-sample", "3s", "-always-on-top",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Nodes != 50 || cfg.Density != graph.DensitySparse || cfg.Seed != 7 ||
		cfg.Width != 800 || cfg.Height != 600 ||
		cfg.Warmup != 500*time.Millisecond || cfg.Sample != 3*time.Second ||
		!cfg.AlwaysOnTop {
		t.Fatalf("parsed config = %+v", cfg)
	}

	for _, args := range [][]string{
		{"-nodes", "100"},
		{"-density", "mixed"},
		{"-width", "100"},
		{"-warmup", "-1s"},
		{"-sample", "0s"},
		{"unexpected"},
	} {
		if _, err := ParseConfig(args); err == nil {
			t.Fatalf("ParseConfig(%v) succeeded", args)
		}
	}
}

func TestProgressSnapshot(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.AlwaysOnTop = true
	bench := NewBench(cfg, scene, graph.NewSpatialIndex(scene), io.Discard, true, true)

	progress := bench.Snapshot()
	if !progress.StartedAt.IsZero() || progress.Frames != 0 || progress.Samples != 0 ||
		progress.WindowHidden || !progress.AlwaysOnTop {
		t.Fatalf("initial progress = %+v", progress)
	}

	bench.SetWindowHidden(true)
	bench.SetWindowHidden(true)
	progress = bench.Snapshot()
	if !progress.WindowHidden || progress.HiddenChanges != 1 {
		t.Fatalf("hidden progress = %+v", progress)
	}
	bench.SetWindowHidden(false)
	progress = bench.Snapshot()
	if progress.WindowHidden || progress.HiddenChanges != 2 {
		t.Fatalf("shown progress = %+v", progress)
	}

	var out bytes.Buffer
	PrintProgress(&out, progress)
	for _, want := range []string{
		"CANVAS_BENCH PROGRESS",
		"frames=0",
		"samples=0",
		"window_hidden=false",
		"hidden_changes=2",
		"always_on_top=true",
		"last_frame_age_ms=-1",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("progress line %q lacks %q", out.String(), want)
		}
	}
}

func TestMotionCycleFitAndTargetScales(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	viewport := simpleRect(1200, 800)
	camera := graph.NewCamera(viewport)
	fitCamera := graph.NewCamera(viewport)
	fitCamera.Fit(scene.Bounds, fitPaddingDIP)
	fitScale := fitCamera.Scale()

	if got := MotionAt(0, fitScale); !got.Fit || got.Scale != fitScale {
		t.Fatalf("cycle start = %+v, want fit at %v", got, fitScale)
	}
	if got := MotionAt(5*time.Second, fitScale); !got.Fit {
		t.Fatalf("cycle end = %+v, want fit", got)
	}
	if got := MotionAt(10*time.Second, fitScale); !got.Fit {
		t.Fatalf("next cycle start = %+v, want fit", got)
	}
	if got := MotionAt(450*time.Millisecond, fitScale); got.Fit || got.Scale != 1.05 {
		t.Fatalf("near key = %+v", got)
	}
	if got := MotionAt(1850*time.Millisecond, fitScale); got.Scale != 0.55 {
		t.Fatalf("medium key = %+v", got)
	}
	if got := MotionAt(2550*time.Millisecond, fitScale); got.Scale != 0.22 {
		t.Fatalf("far key = %+v", got)
	}
	if a, b := MotionAt(5450*time.Millisecond, fitScale), MotionAt(450*time.Millisecond, fitScale); a != b {
		t.Fatalf("cycle replay drifted: %+v != %+v", a, b)
	}

	applyMotion(camera, scene, ui.Rect{W: 1200, H: 800}, 0)
	if camera.Scale() != fitScale {
		t.Fatalf("fit application scale = %v, want %v", camera.Scale(), fitScale)
	}
	applyMotion(camera, scene, ui.Rect{W: 1200, H: 800}, 450*time.Millisecond)
	if !near(camera.Scale(), 1.05) {
		t.Fatalf("near application scale = %v, want 1.05", camera.Scale())
	}
	applyMotion(camera, scene, ui.Rect{W: 1200, H: 800}, 1850*time.Millisecond)
	if !near(camera.Scale(), 0.55) {
		t.Fatalf("medium application scale = %v, want 0.55", camera.Scale())
	}
	applyMotion(camera, scene, ui.Rect{W: 1200, H: 800}, 2550*time.Millisecond)
	if !near(camera.Scale(), 0.22) {
		t.Fatalf("far application scale = %v, want 0.22", camera.Scale())
	}
}

func TestBenchmarkSceneExercisesFormalRendering(t *testing.T) {
	cfg := DefaultConfig()
	scene, err := benchmarkScene(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(scene.Nodes) != cfg.Nodes || len(scene.Edges) != 2*cfg.Nodes {
		t.Fatalf("benchmark has %d nodes and %d edges", len(scene.Nodes), len(scene.Edges))
	}
	for _, node := range scene.Nodes {
		if node.Code == "" || !node.Fields[0].PrimaryKey || !node.Fields[1].ForeignKey {
			t.Fatalf("benchmark table %d lacks a code or key marker", node.ID)
		}
		for _, field := range node.Fields {
			if field.DataType == "" {
				t.Fatalf("benchmark table %d has an untyped field", node.ID)
			}
		}
	}
	for _, edge := range scene.Edges {
		if !edge.Directed {
			t.Fatalf("benchmark edge %d has no arrow direction", edge.ID)
		}
	}
}

func TestPercentileDuration(t *testing.T) {
	values := make([]time.Duration, 100)
	for i := range values {
		values[i] = time.Duration(i+1) * time.Millisecond
	}
	if got := percentileDuration(values, 0.95); got != 95*time.Millisecond {
		t.Fatalf("p95 = %v, want 95ms", got)
	}
	if got := percentileDuration(values, 0.50); got != 50*time.Millisecond {
		t.Fatalf("p50 = %v, want 50ms", got)
	}
	if got := percentileDuration(nil, 0.95); got != 0 {
		t.Fatalf("empty percentile = %v", got)
	}
}

func TestResultRequiresSufficientFrames(t *testing.T) {
	scene, err := graph.GenerateDefault(50, graph.DensityDense)
	if err != nil {
		t.Fatal(err)
	}
	bench := NewBench(DefaultConfig(), scene, graph.NewSpatialIndex(scene), io.Discard, true, true)
	if _, err := bench.Result(); err == nil {
		t.Fatal("empty benchmark returned a result without frame samples")
	}
}

func simpleRect(w, h float64) graph.Rect {
	return graph.Rect{Min: graph.Point{}, Max: graph.Point{X: w, Y: h}}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
