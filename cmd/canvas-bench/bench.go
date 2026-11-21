package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"pdmview/internal/graph"
	"pdmview/internal/renderer"

	"github.com/egoist/mygo/ui"
)

const (
	MinFrameSamples = 30
	motionCycle     = 5 * time.Second
	fitPaddingDIP   = 48.0
)

type Config struct {
	Nodes   int
	Density graph.Density
	Seed    uint64
	Width   int
	Height  int
	Warmup  time.Duration
	Sample  time.Duration
}

func DefaultConfig() Config {
	return Config{
		Nodes:   500,
		Density: graph.DensityDense,
		Seed:    graph.DefaultSeed,
		Width:   1200,
		Height:  800,
		Warmup:  2 * time.Second,
		Sample:  10 * time.Second,
	}
}

func ParseConfig(args []string) (Config, error) {
	cfg := DefaultConfig()
	fs := flag.NewFlagSet("canvas-bench", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&cfg.Nodes, "nodes", cfg.Nodes, "node count: 50, 150, 300, or 500")
	density := "dense"
	fs.StringVar(&density, "density", density, "layout density: dense or sparse")
	fs.Uint64Var(&cfg.Seed, "seed", cfg.Seed, "deterministic graph seed")
	fs.IntVar(&cfg.Width, "width", cfg.Width, "content width in DIP")
	fs.IntVar(&cfg.Height, "height", cfg.Height, "content height in DIP")
	fs.DurationVar(&cfg.Warmup, "warmup", cfg.Warmup, "warmup duration")
	fs.DurationVar(&cfg.Sample, "sample", cfg.Sample, "measurement duration")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	switch strings.ToLower(strings.TrimSpace(density)) {
	case "dense":
		cfg.Density = graph.DensityDense
	case "sparse":
		cfg.Density = graph.DensitySparse
	default:
		return Config{}, fmt.Errorf("density must be dense or sparse")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.Nodes {
	case 50, 150, 300, 500:
	default:
		return fmt.Errorf("nodes must be 50, 150, 300, or 500")
	}
	if c.Density != graph.DensityDense && c.Density != graph.DensitySparse {
		return fmt.Errorf("density must be dense or sparse")
	}
	if c.Width < 320 || c.Height < 240 {
		return fmt.Errorf("content size must be at least 320x240 DIP")
	}
	if c.Warmup < 0 {
		return fmt.Errorf("warmup cannot be negative")
	}
	if c.Sample <= 0 {
		return fmt.Errorf("sample must be positive")
	}
	return nil
}

func benchmarkScene(cfg Config) (graph.Scene, error) {
	scene, err := graph.Generate(cfg.Nodes, cfg.Seed, cfg.Density)
	if err != nil {
		return graph.Scene{}, err
	}
	dataTypes := [...]string{"BIGINT", "BIGINT", "TIMESTAMP", "VARCHAR(30)", "TEXT"}
	for offset := range scene.Nodes {
		node := &scene.Nodes[offset]
		node.Code = fmt.Sprintf("bench_table_%03d", offset+1)
		for fieldOffset, dataType := range dataTypes {
			field := &node.Fields[fieldOffset]
			field.DataType = dataType
			field.PrimaryKey = fieldOffset == 0
			field.ForeignKey = fieldOffset == 1
		}
		node.Fields[0].Name = "id"
		node.Fields[1].Name = "parent_id"
	}
	for offset := range scene.Edges {
		scene.Edges[offset].Directed = true
	}
	return scene, nil
}

type motionSample struct {
	Fit   bool
	Scale float64
	Pan   graph.Point
}

type motionKey struct {
	at    time.Duration
	fit   bool
	scale float64
	pan   graph.Point
}

var motionKeys = []motionKey{
	{at: 0, fit: true},
	{at: 450 * time.Millisecond, scale: 1.05},
	{at: 1200 * time.Millisecond, scale: 1.05, pan: graph.Point{X: -180, Y: 90}},
	{at: 1850 * time.Millisecond, scale: 0.55, pan: graph.Point{X: 140, Y: -100}},
	{at: 2550 * time.Millisecond, scale: 0.22},
	{at: 3250 * time.Millisecond, scale: 0.55, pan: graph.Point{X: -120, Y: 80}},
	{at: 3950 * time.Millisecond, scale: 1.05, pan: graph.Point{X: 160, Y: -80}},
	{at: 4550 * time.Millisecond, scale: 1.05},
	{at: motionCycle, fit: true},
}

func MotionAt(elapsed time.Duration, fitScale float64) motionSample {
	t := elapsed % motionCycle
	if t < 0 {
		t += motionCycle
	}
	if t == 0 || t == motionCycle {
		return motionSample{Fit: true, Scale: fitScale}
	}
	for i := 1; i < len(motionKeys); i++ {
		next := motionKeys[i]
		if t > next.at {
			continue
		}
		prev := motionKeys[i-1]
		if t == next.at {
			return keySample(next, fitScale)
		}
		u := float64(t-prev.at) / float64(next.at-prev.at)
		u = u * u * (3 - 2*u)
		prevScale, nextScale := keyScale(prev, fitScale), keyScale(next, fitScale)
		return motionSample{
			Scale: prevScale + (nextScale-prevScale)*u,
			Pan: graph.Point{
				X: prev.pan.X + (next.pan.X-prev.pan.X)*u,
				Y: prev.pan.Y + (next.pan.Y-prev.pan.Y)*u,
			},
		}
	}
	return motionSample{Fit: true, Scale: fitScale}
}

func keySample(key motionKey, fitScale float64) motionSample {
	if key.fit {
		return motionSample{Fit: true, Scale: fitScale}
	}
	return motionSample{Scale: key.scale, Pan: key.pan}
}

func keyScale(key motionKey, fitScale float64) float64 {
	if key.fit {
		return fitScale
	}
	return key.scale
}

func applyMotion(camera *graph.Camera, scene graph.Scene, area ui.Rect, elapsed time.Duration) {
	camera.Fit(scene.Bounds, fitPaddingDIP)
	fitScale := camera.Scale()
	sample := MotionAt(elapsed, fitScale)
	if sample.Fit {
		return
	}
	anchor := graph.Point{X: float64(area.W) / 2, Y: float64(area.H) / 2}
	camera.ZoomAt(anchor, sample.Scale/camera.Scale())
	camera.Pan(sample.Pan)
}

type Bench struct {
	cfg       Config
	scene     graph.Scene
	renderer  *renderer.Canvas
	out       io.Writer
	native    bool
	pageNil   bool
	started   time.Time
	measureAt time.Time
	endAt     time.Time
	prevEnd   time.Time
	began     bool
	done      bool

	viewportW, viewportH     int
	unexpectedW, unexpectedH int
	gpu                      bool
	scale                    float32

	intervals   []time.Duration
	callbacks   []time.Duration
	nodesMin    int
	nodesMax    int
	nodesSum    int
	edgesMin    int
	edgesMax    int
	edgesSum    int
	lodCounts   map[renderer.LOD]int
	cacheHits   int
	cacheMisses int
	fieldRows   int
	arrows      int
	unprepared  int
}

func NewBench(cfg Config, scene graph.Scene, index graph.SpatialQuery, out io.Writer, native, pageNil bool) *Bench {
	canvas := renderer.New(scene, index)
	area := ui.Rect{W: float32(cfg.Width), H: float32(cfg.Height)}
	camera := graph.NewCamera(graph.Rect{Max: graph.Point{X: float64(cfg.Width), Y: float64(cfg.Height)}})
	for elapsed := time.Duration(0); elapsed < motionCycle; elapsed += 5 * time.Millisecond {
		applyMotion(camera, scene, area, elapsed)
		canvas.PrepareVisible(camera, graph.Hit{})
	}
	return &Bench{
		cfg:       cfg,
		scene:     scene,
		renderer:  canvas,
		out:       out,
		native:    native,
		pageNil:   pageNil,
		nodesMin:  math.MaxInt,
		edgesMin:  math.MaxInt,
		lodCounts: map[renderer.LOD]int{renderer.Far: 0, renderer.Medium: 0, renderer.Near: 0},
	}
}

// Frame draws and records one painter callback. It returns true once the
// measurement window has ended; callers print the END marker and quit.
func (b *Bench) Frame(p *ui.Painter, area ui.Rect, camera *graph.Camera) bool {
	if b.done {
		return true
	}
	start := time.Now()
	if b.started.IsZero() {
		b.started = start
		b.measureAt = start.Add(b.cfg.Warmup)
		b.endAt = b.measureAt.Add(b.cfg.Sample)
	}
	if !start.Before(b.endAt) {
		b.done = true
		return true
	}
	camera.SetViewport(graph.Rect{
		Min: graph.Point{X: float64(area.X), Y: float64(area.Y)},
		Max: graph.Point{X: float64(area.X + area.W), Y: float64(area.Y + area.H)},
	})
	b.viewportW, b.viewportH = int(math.Round(float64(area.W))), int(math.Round(float64(area.H)))
	b.gpu, b.scale = p.Accelerated(), p.Scale()
	sampling := !start.Before(b.measureAt)
	if sampling && (b.viewportW != b.cfg.Width || b.viewportH != b.cfg.Height) {
		b.unexpectedW, b.unexpectedH = b.viewportW, b.viewportH
	}
	if sampling && !b.began {
		b.began = true
		b.printBegin()
	}
	applyMotion(camera, b.scene, area, start.Sub(b.started))
	frame := b.renderer.Draw(p, area, camera, graph.Hit{Kind: graph.HitNone, Index: -1})
	end := time.Now()

	if sampling {
		if !b.prevEnd.IsZero() {
			b.intervals = append(b.intervals, end.Sub(b.prevEnd))
		}
		b.callbacks = append(b.callbacks, end.Sub(start))
		b.addFrameStats(frame)
		b.prevEnd = end
	}
	return false
}

func (b *Bench) addFrameStats(frame renderer.FrameStats) {
	b.nodesMin = min(b.nodesMin, frame.VisibleNodes)
	b.nodesMax = max(b.nodesMax, frame.VisibleNodes)
	b.nodesSum += frame.VisibleNodes
	b.edgesMin = min(b.edgesMin, frame.VisibleEdges)
	b.edgesMax = max(b.edgesMax, frame.VisibleEdges)
	b.edgesSum += frame.VisibleEdges
	b.lodCounts[frame.LOD]++
	b.cacheHits += frame.CacheHits
	b.cacheMisses += frame.CacheMisses
	b.fieldRows += frame.FieldRows
	b.arrows += frame.Arrows
	b.unprepared += frame.UnpreparedNodes
}

type Result struct {
	Frames          int
	FPS             float64
	IntervalP95     time.Duration
	CallbackP95     time.Duration
	CallbackMean    time.Duration
	NodesMin        int
	NodesMax        int
	NodesMean       float64
	EdgesMin        int
	EdgesMax        int
	EdgesMean       float64
	LODCounts       map[renderer.LOD]int
	CacheHits       int
	CacheMisses     int
	CacheHitPercent float64
	FieldRows       int
	Arrows          int
	UnpreparedNodes int
	ViewportW       int
	ViewportH       int
	Native          bool
	PageNil         bool
	GPU             bool
	HiDPI           bool
	Scale           float32
}

func (b *Bench) Result() (Result, error) {
	if !b.native || !b.pageNil {
		return Result{}, fmt.Errorf("expected a native MyGo window without a web page")
	}
	if b.unexpectedW != 0 || b.unexpectedH != 0 {
		return Result{}, fmt.Errorf("sampled viewport changed to %dx%d DIP, expected %dx%d",
			b.unexpectedW, b.unexpectedH, b.cfg.Width, b.cfg.Height)
	}
	frames := len(b.callbacks)
	if frames < MinFrameSamples {
		return Result{}, fmt.Errorf("insufficient frame samples: got %d, need at least %d", frames, MinFrameSamples)
	}
	if len(b.intervals) == 0 {
		return Result{}, fmt.Errorf("insufficient completed callback intervals")
	}
	var intervalSum, callbackSum time.Duration
	for _, d := range b.intervals {
		intervalSum += d
	}
	for _, d := range b.callbacks {
		callbackSum += d
	}
	meanInterval := intervalSum / time.Duration(len(b.intervals))
	meanCallback := callbackSum / time.Duration(frames)
	totalCache := b.cacheHits + b.cacheMisses
	cachePercent := 0.0
	if totalCache > 0 {
		cachePercent = float64(b.cacheHits) * 100 / float64(totalCache)
	}
	return Result{
		Frames:          frames,
		FPS:             float64(time.Second) / float64(meanInterval),
		IntervalP95:     percentileDuration(b.intervals, 0.95),
		CallbackP95:     percentileDuration(b.callbacks, 0.95),
		CallbackMean:    meanCallback,
		NodesMin:        b.nodesMin,
		NodesMax:        b.nodesMax,
		NodesMean:       float64(b.nodesSum) / float64(frames),
		EdgesMin:        b.edgesMin,
		EdgesMax:        b.edgesMax,
		EdgesMean:       float64(b.edgesSum) / float64(frames),
		LODCounts:       map[renderer.LOD]int{renderer.Far: b.lodCounts[renderer.Far], renderer.Medium: b.lodCounts[renderer.Medium], renderer.Near: b.lodCounts[renderer.Near]},
		CacheHits:       b.cacheHits,
		CacheMisses:     b.cacheMisses,
		CacheHitPercent: cachePercent,
		FieldRows:       b.fieldRows,
		Arrows:          b.arrows,
		UnpreparedNodes: b.unprepared,
		ViewportW:       b.viewportW,
		ViewportH:       b.viewportH,
		Native:          b.native,
		PageNil:         b.pageNil,
		GPU:             b.gpu,
		HiDPI:           b.scale > 1.01,
		Scale:           b.scale,
	}, nil
}

func percentileDuration(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	index = min(max(index, 0), len(sorted)-1)
	return sorted[index]
}

func (b *Bench) printBegin() {
	fmt.Fprintf(b.out,
		"CANVAS_BENCH BEGIN nodes=%d density=%s seed=%d warmup_ms=%d sample_ms=%d viewport_dip=%dx%d viewport_px=%dx%d native=%t page_nil=%t gpu=%t hidpi=%t scale=%.2f\n",
		b.cfg.Nodes, b.cfg.Density, b.cfg.Seed,
		b.cfg.Warmup.Milliseconds(), b.cfg.Sample.Milliseconds(),
		b.viewportW, b.viewportH,
		int(math.Round(float64(b.viewportW)*float64(b.scale))),
		int(math.Round(float64(b.viewportH)*float64(b.scale))),
		b.native, b.pageNil, b.gpu, b.scale > 1.01, b.scale,
	)
}

func PrintEnd(out io.Writer, result Result, err error) {
	errorField := ""
	if err != nil {
		errorField = fmt.Sprintf(" error=%q", err)
	}
	fmt.Fprintf(out,
		"CANVAS_BENCH END frames=%d fps=%.2f interval_p95_ms=%.3f callback_p95_ms=%.3f callback_mean_ms=%.3f nodes_min=%d nodes_max=%d nodes_mean=%.2f edges_min=%d edges_max=%d edges_mean=%.2f lod_far=%d lod_medium=%d lod_near=%d cache_hits=%d cache_misses=%d cache_hit_pct=%.1f field_rows=%d arrows=%d unprepared_nodes=%d viewport_dip=%dx%d native=%t page_nil=%t gpu=%t hidpi=%t scale=%.2f%s\n",
		result.Frames, result.FPS,
		durationMS(result.IntervalP95), durationMS(result.CallbackP95), durationMS(result.CallbackMean),
		result.NodesMin, result.NodesMax, result.NodesMean,
		result.EdgesMin, result.EdgesMax, result.EdgesMean,
		result.LODCounts[renderer.Far], result.LODCounts[renderer.Medium], result.LODCounts[renderer.Near],
		result.CacheHits, result.CacheMisses, result.CacheHitPercent,
		result.FieldRows, result.Arrows, result.UnpreparedNodes,
		result.ViewportW, result.ViewportH,
		result.Native, result.PageNil, result.GPU, result.HiDPI, result.Scale,
		errorField,
	)
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
