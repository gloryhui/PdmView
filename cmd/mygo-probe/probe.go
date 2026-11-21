package main

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	nodeWidth  = 214
	nodeHeight = 126
	nodeStepX  = 260
	nodeStepY  = 176
	nodeOrigin = 80
	nodeCols   = 12
	gridTile   = 256
)

var (
	probeBackground = ui.Hex("#0b1220")
	probeGridBase   = ui.Hex("#0d1a30")
	probeGridLine   = ui.Hex("#173253")
	probeNodeFill   = ui.Hex("#f8fafc")
	probeNodeBorder = ui.Hex("#94a3b8")
	probeNodeText   = ui.Hex("#0f172a")
	probeNodeMuted  = ui.Hex("#475569")
	probeFieldFill  = ui.Hex("#e2e8f0")
	probeEdgeColor  = ui.Hex("#64748b")
	probeEdgeHot    = ui.Hex("#38bdf8")
	probeSelection  = ui.Hex("#0ea5e9")
)

type probeNode struct {
	index        int
	name         string
	rect         ui.Rect
	title        *ui.TextLayout
	subtitle     *ui.TextLayout
	hitPath      *ui.Path
	fieldLayouts [3]*ui.TextLayout
}

type probeEdge struct {
	from, to int
	path     *ui.Path
	points   [2][2]float32
	bounds   ui.Rect
}

type probeScene struct {
	nodes  []*probeNode
	edges  []*probeEdge
	bounds ui.Rect
}

type probeState struct {
	cfg   probeConfig
	scene *probeScene

	window *mygo.Window
	ctx    *ui.Context

	cameraX, cameraY float32
	zoom             float32
	canvas           ui.Rect
	autoFitted       bool
	selected         int

	dragActive bool
	dragMoved  bool
	dragStartX float32
	dragStartY float32
	dragLastX  float32
	dragLastY  float32

	status        string
	actionPending bool

	pickFiles      func(*mygo.Window) ([]string, error)
	clipboardProbe func() (string, error)

	grid          *ui.Canvas
	multiText     *ui.TextLayout
	fieldLabels   [3]*ui.TextLayout
	lastScale     float32
	lastGPU       bool
	frames        atomic.Int64
	lastDrawNanos atomic.Int64
	scaleBits     atomic.Uint32
	accelerated   atomic.Bool
	visibleNodes  atomic.Int64
	visibleEdges  atomic.Int64
	exitCode      atomic.Int32
	bench         benchRun
}

func newProbeState(cfg probeConfig, pickFiles func(*mygo.Window) ([]string, error), clipboardProbe func() (string, error)) *probeState {
	s := &probeState{
		cfg:            cfg,
		zoom:           1,
		selected:       -1,
		pickFiles:      pickFiles,
		clipboardProbe: clipboardProbe,
		status:         "Wheel/drag pans; click a table for Path.Contains hit testing. Native actions run only when clicked.",
	}
	s.scene = newProbeScene(cfg.nodes, cfg.bench)

	textFont := ui.Font{Size: 12.5, Weight: 500}
	s.multiText = ui.ShapeText("表 Table テーブル Таблица — pre-shaped outside Draw", textFont, 0)
	fieldFont := ui.Font{Size: 10.5, Weight: 400}
	for i, label := range []string{"id        BIGINT", "name      VARCHAR(120)", "amount    DECIMAL(18,2)"} {
		s.fieldLabels[i] = ui.ShapeText(label, fieldFont, 0)
	}
	s.scene.attachLayouts(s.fieldLabels)
	if cfg.bench {
		s.bench.doneCh = make(chan struct{})
		s.bench.warmup = cfg.benchWarmup
		s.bench.measure = cfg.benchMeasure
		s.bench.totalNodes = len(s.scene.nodes)
		s.bench.totalEdges = len(s.scene.edges)
	}
	return s
}

func (s *probeState) view(c *ui.Context) {
	s.ctx = c
	t := c.Theme()
	status := s.statusText()
	dpi := s.dpiLabel()

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().Height(58).Padding(10, 14).Gap(10).Background(t.Surface).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "PdmView P0-02 · MyGo native probe").FontSize(17).Bold()
				ui.Text(c, "No WebView · native UI only").FontSize(10.5).TextColor(t.TextMuted)
			})
			ui.Box(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d nodes · %d edges", len(s.scene.nodes), len(s.scene.edges))).
				FontSize(10.5).TextColor(t.TextMuted)
			ui.Text(c, dpi).FontSize(10.5).TextColor(t.TextMuted)
			ui.Button(c, "−").Key("zoom-out").OnClick(func() { s.zoomAtCenter(0.85) })
			ui.Button(c, "100%").Key("zoom-reset").OnClick(func() { s.resetView() })
			ui.Button(c, "+").Key("zoom-in").OnClick(func() { s.zoomAtCenter(1.18) })
			ui.Button(c, "File...").Key("pick-file").OnClick(func() { s.pickFile() })
			ui.Button(c, "Clipboard round-trip").Key("clipboard-round-trip").OnClick(func() { s.clipboardRoundTrip() })
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Box(c).Grow(1).FillWidth().Clip().
			Draw(s.drawCanvas).
			HandleInput(s.handleInput).
			Key("probe-canvas").
			Label("Probe canvas")
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c).FillWidth().Height(42).Padding(8, 14).Gap(12).Background(t.Surface).Children(func() {
			ui.Text(c, "Wheel pans · drag pans · click hits a table · −/+ zooms").FontSize(10.5).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			ui.Text(c, status).FontSize(10.5).TextColor(t.Text)
		})
	})
}

func newProbeScene(n int, benchmark bool) *probeScene {
	titleFont := ui.Font{Size: 13, Weight: 650}
	subtitleFont := ui.Font{Size: 10.5, Weight: 400}
	scene := &probeScene{}

	for i := 0; i < n; i++ {
		col := i % nodeCols
		row := i / nodeCols
		rect := ui.Rect{
			X: nodeOrigin + float32(col)*nodeStepX,
			Y: nodeOrigin + float32(row)*nodeStepY,
			W: nodeWidth,
			H: nodeHeight,
		}
		name := fmt.Sprintf("SYNTH_%03d", i+1)
		if n == 2 {
			name = []string{"DEMO_TABLE_A", "DEMO_TABLE_B"}[i]
		}
		node := &probeNode{
			index:    i,
			name:     name,
			rect:     rect,
			title:    ui.ShapeText(name, titleFont, 0),
			subtitle: ui.ShapeText(fmt.Sprintf("synthetic table #%03d", i+1), subtitleFont, 0),
			hitPath:  rectPath(rect),
		}
		scene.nodes = append(scene.nodes, node)
		scene.include(rect)
	}

	if benchmark {
		for i := 0; i < n; i++ {
			toA := (i*7 + 1) % n
			if toA == i {
				toA = (toA + 1) % n
			}
			toB := (i*13 + 5) % n
			if toB == i || toB == toA {
				toB = (toB + 1) % n
				if toB == i || toB == toA {
					toB = (toB + 1) % n
				}
			}
			scene.addEdge(i, toA, false)
			scene.addEdge(i, toB, false)
		}
	} else if n >= 2 {
		scene.addEdge(0, 1, true)
	}
	return scene
}

func (s *probeScene) include(r ui.Rect) {
	if s.bounds.W == 0 && s.bounds.H == 0 {
		s.bounds = r
		return
	}
	x0 := min(s.bounds.X, r.X)
	y0 := min(s.bounds.Y, r.Y)
	x1 := max(s.bounds.X+s.bounds.W, r.X+r.W)
	y1 := max(s.bounds.Y+s.bounds.H, r.Y+r.H)
	s.bounds = ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func (s *probeScene) addEdge(from, to int, dashed bool) {
	a, b := s.nodes[from], s.nodes[to]
	acx, acy := a.rect.X+a.rect.W/2, a.rect.Y+a.rect.H/2
	bcx, bcy := b.rect.X+b.rect.W/2, b.rect.Y+b.rect.H/2

	startX, endX := a.rect.X+a.rect.W, b.rect.X
	if bcx < acx {
		startX, endX = a.rect.X, b.rect.X+b.rect.W
	}
	startY, endY := acy, bcy
	points := [2][2]float32{{startX, startY}, {endX, endY}}
	bounds := ui.Rect{
		X: min(startX, endX), Y: min(startY, endY),
		W: max(startX, endX) - min(startX, endX), H: max(startY, endY) - min(startY, endY),
	}
	edge := &probeEdge{from: from, to: to, points: points, bounds: bounds}
	if !dashed {
		s.edges = append(s.edges, edge)
		return
	}
	bend := max(float32(52), float32(math.Abs(float64(endX-startX)))*0.45)
	var path ui.Path
	if bcx < acx {
		bend = -bend
	}
	path.MoveTo(startX, startY).
		CubeTo(startX+bend, startY, endX-bend, endY, endX, endY).
		Cap(ui.RoundCap).
		Join(ui.RoundJoin).
		Dash(0, 8, 6)
	edge.path, edge.bounds = &path, path.Bounds()
	s.edges = append(s.edges, edge)
}

func (s *probeScene) attachLayouts(fieldLayouts [3]*ui.TextLayout) {
	for _, node := range s.nodes {
		node.fieldLayouts = fieldLayouts
	}
}

func rectPath(r ui.Rect) *ui.Path {
	var p ui.Path
	p.MoveTo(r.X, r.Y).
		LineTo(r.X+r.W, r.Y).
		LineTo(r.X+r.W, r.Y+r.H).
		LineTo(r.X, r.Y+r.H).
		Close()
	return &p
}

func (s *probeState) drawCanvas(p *ui.Painter, r ui.Rect) {
	start := time.Now()
	s.canvas = r

	if s.cfg.bench && !s.autoFitted {
		s.fitAll(r)
		s.autoFitted = true
	}
	if s.cfg.bench {
		s.startBenchMonitor()
	}

	scale := p.Scale()
	accelerated := p.Accelerated()
	if scale != s.lastScale || accelerated != s.lastGPU {
		s.lastScale, s.lastGPU = scale, accelerated
		s.scaleBits.Store(math.Float32bits(scale))
		s.accelerated.Store(accelerated)
		s.requestFrame()
	}

	s.ensureGrid(p)
	visibleNodes, visibleEdges := s.drawScene(p, r)
	s.visibleNodes.Store(int64(visibleNodes))
	s.visibleEdges.Store(int64(visibleEdges))
	s.drawOverlay(p, r)

	frames := s.frames.Add(1)
	elapsed := time.Since(start)
	s.lastDrawNanos.Store(int64(elapsed))

	switch {
	case s.cfg.bench:
		if s.bench.observe(elapsed, visibleNodes, visibleEdges, scale, accelerated, time.Now()) {
			return
		}
		s.requestFrame()
	case s.cfg.smoke && frames < 3:
		s.requestFrame()
	}
}

func (s *probeState) ensureGrid(p *ui.Painter) {
	if s.grid != nil {
		return
	}
	s.grid = ui.NewCanvas(gridTile, gridTile)
	s.grid.Draw(p, func(cp *ui.Painter) {
		cp.Fill(ui.Rect{W: gridTile, H: gridTile}, probeGridBase, 0)
		for v := 0; v <= gridTile; v += 32 {
			cp.Line(float32(v), 0, float32(v), gridTile, 1, probeGridLine)
			cp.Line(0, float32(v), gridTile, float32(v), 1, probeGridLine)
		}
		cp.Stroke(ui.Rect{W: gridTile, H: gridTile}, probeGridLine, 0, 1)
	})
}

func (s *probeState) drawScene(p *ui.Painter, r ui.Rect) (visibleNodes, visibleEdges int) {
	p.Clip(r, 0, func() {
		p.Fill(r, probeBackground, 0)
		viewTransform := ui.Translation(-s.cameraX, -s.cameraY).
			Then(ui.Scaling(s.zoom, s.zoom)).
			Then(ui.Translation(r.X, r.Y))
		p.Transform(viewTransform, 0, 0, func() {
			view := p.Visible()
			s.drawGridTiles(p, view)

			for _, edge := range s.scene.edges {
				if !rectsIntersect(edge.bounds, view) {
					continue
				}
				visibleEdges++
				color := probeEdgeColor
				if s.selected == edge.from || s.selected == edge.to {
					color = probeEdgeHot
				}
				if edge.path != nil {
					p.StrokePath(edge.path, 1.5, color)
				} else {
					p.StrokePolyline(edge.points[:], 1.5, color)
				}
				end := edge.points[1]
				p.Fill(ui.Rect{
					X: end[0] - 3,
					Y: end[1] - 3,
					W: 6,
					H: 6,
				}, color, 3)
			}

			for _, node := range s.scene.nodes {
				if !rectsIntersect(node.rect, view) {
					continue
				}
				visibleNodes++
				s.drawNode(p, node)
			}
		})
	})
	return visibleNodes, visibleEdges
}

func (s *probeState) drawGridTiles(p *ui.Painter, view ui.Rect) {
	startX := float32(math.Floor(float64(view.X/gridTile)) * gridTile)
	startY := float32(math.Floor(float64(view.Y/gridTile)) * gridTile)
	endX := view.X + view.W + gridTile
	endY := view.Y + view.H + gridTile
	for y := startY; y < endY; y += gridTile {
		for x := startX; x < endX; x += gridTile {
			p.Image(s.grid.Bitmap(), ui.Rect{X: x, Y: y, W: gridTile, H: gridTile}, ui.FillBox)
		}
	}
}

func (s *probeState) drawNode(p *ui.Painter, node *probeNode) {
	r := node.rect
	p.Fill(r, probeNodeFill, 9)
	if s.selected == node.index {
		p.Stroke(r, probeSelection, 9, 2.5)
		p.Fill(ui.Rect{X: r.X + 10, Y: r.Y + 8, W: 34, H: 16}, probeSelection, 8)
	} else {
		p.Stroke(r, probeNodeBorder, 9, 1)
	}

	p.TextLayout(node.title, r.X+12, r.Y+10, probeNodeText)
	p.TextLayout(node.subtitle, r.X+12, r.Y+30, probeNodeMuted)
	for i := 0; i < len(node.fieldLayouts); i++ {
		y := r.Y + 52 + float32(i)*22
		p.Fill(ui.Rect{X: r.X + 12, Y: y + 12, W: r.W - 24, H: 1}, probeFieldFill, 0)
		p.TextLayout(node.fieldLayouts[i], r.X+12, y, probeNodeMuted)
	}
}

func (s *probeState) drawOverlay(p *ui.Painter, r ui.Rect) {
	label := s.multiText
	w, h := float32(330), float32(30)
	box := ui.Rect{X: r.X + 12, Y: r.Y + 12, W: w, H: h}
	p.Fill(box, ui.RGBA(10, 22, 40, 0.86), 7)
	p.Stroke(box, ui.Hex("#27496d"), 7, 1)
	p.TextLayout(label, box.X+10, box.Y+7, ui.Hex("#dbeafe"))
}

func (s *probeState) handleInput(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		s.dragActive = true
		s.dragMoved = false
		s.dragStartX, s.dragStartY = ev.X, ev.Y
		s.dragLastX, s.dragLastY = ev.X, ev.Y
		return true
	case ui.InputPointerMove:
		if !s.dragActive {
			return false
		}
		dx, dy := ev.X-s.dragLastX, ev.Y-s.dragLastY
		if !s.dragMoved {
			mx, my := ev.X-s.dragStartX, ev.Y-s.dragStartY
			s.dragMoved = mx*mx+my*my >= 9
		}
		if s.dragMoved {
			s.cameraX -= dx / s.zoom
			s.cameraY -= dy / s.zoom
			s.setStatus(fmt.Sprintf("Dragging canvas · camera %.1f, %.1f · zoom %.2f", s.cameraX, s.cameraY, s.zoom))
		}
		s.dragLastX, s.dragLastY = ev.X, ev.Y
		s.requestFrame()
		return true
	case ui.InputPointerUp:
		if !s.dragActive {
			return false
		}
		if ev.Canceled {
			s.setStatus("Native drag canceled; no table selection")
		} else if s.dragMoved {
			s.setStatus(fmt.Sprintf("Panned by native drag · camera %.1f, %.1f", s.cameraX, s.cameraY))
		} else {
			s.selectAt(ev.X, ev.Y)
		}
		s.dragActive = false
		s.requestFrame()
		return true
	case ui.InputScroll:
		s.cameraX += ev.DX / s.zoom
		s.cameraY += ev.DY / s.zoom
		s.setStatus(fmt.Sprintf("Native wheel pan · camera %.1f, %.1f", s.cameraX, s.cameraY))
		s.requestFrame()
		return true
	default:
		return false
	}
}

func (s *probeState) selectAt(x, y float32) {
	worldX := s.cameraX + x/s.zoom
	worldY := s.cameraY + y/s.zoom
	for i := len(s.scene.nodes) - 1; i >= 0; i-- {
		node := s.scene.nodes[i]
		if node.hitPath.Contains(worldX, worldY) {
			s.selected = i
			s.setStatus(fmt.Sprintf("Hit %s at %.1f, %.1f via Path.Contains", node.name, worldX, worldY))
			return
		}
	}
	s.selected = -1
	s.setStatus("Canvas click missed every table Path.Contains")
}

func (s *probeState) zoomAtCenter(factor float32) {
	oldZoom := s.zoom
	newZoom := max(0.08, min(4, s.zoom*factor))
	if newZoom == oldZoom || s.canvas.W <= 0 || s.canvas.H <= 0 {
		return
	}
	centerX := s.cameraX + s.canvas.W/(2*oldZoom)
	centerY := s.cameraY + s.canvas.H/(2*oldZoom)
	s.zoom = newZoom
	s.cameraX = centerX - s.canvas.W/(2*newZoom)
	s.cameraY = centerY - s.canvas.H/(2*newZoom)
	s.setStatus(fmt.Sprintf("Zoom %.0f%% · painter Transform + Scale verified", s.zoom*100))
	s.requestFrame()
}

func (s *probeState) resetView() {
	s.zoom = 1
	s.cameraX, s.cameraY = 0, 0
	s.selected = -1
	if s.cfg.bench {
		s.fitAll(s.canvas)
	}
	s.setStatus("View reset")
	s.requestFrame()
}

func (s *probeState) fitAll(r ui.Rect) {
	if r.W <= 0 || r.H <= 0 || s.scene.bounds.W <= 0 || s.scene.bounds.H <= 0 {
		return
	}
	const margin = 56
	z := min((r.W-2*margin)/s.scene.bounds.W, (r.H-2*margin)/s.scene.bounds.H)
	s.zoom = max(0.08, min(2, z))
	s.cameraX = s.scene.bounds.X - (r.W/s.zoom-s.scene.bounds.W)/2
	s.cameraY = s.scene.bounds.Y - (r.H/s.zoom-s.scene.bounds.H)/2
}

func (s *probeState) pickFile() {
	if s.pickFiles == nil {
		return
	}
	s.runAction(func() (string, error) {
		selected, err := s.pickFiles(s.window)
		count := len(selected)
		selected = nil
		if err != nil {
			return "Native file picker failed; selection details and paths are deliberately suppressed", err
		}
		if count == 0 {
			return "Native file picker canceled; no selection read or shown", nil
		}
		return fmt.Sprintf("Native file picker returned %d selection(s); path and contents were not read or shown", count), nil
	})
}

func (s *probeState) clipboardRoundTrip() {
	if s.clipboardProbe == nil {
		return
	}
	s.runAction(s.clipboardProbe)
}

func (s *probeState) runAction(action func() (string, error)) {
	if s.actionPending {
		return
	}
	s.actionPending = true
	finish := func(status string, err error) {
		if status == "" && err != nil {
			status = "Native action failed"
		}
		s.actionPending = false
		s.setStatus(status)
		s.requestFrame()
	}
	if s.window == nil {
		finish(action())
		return
	}
	go func() {
		status, err := action()
		mygo.RunOnMain(func() { finish(status, err) })
	}()
}

func (s *probeState) setStatus(status string) {
	s.status = status
}

func (s *probeState) statusText() string {
	return s.status
}

func (s *probeState) dpiLabel() string {
	scale := s.frameScale()
	accelerated := s.frameAccelerated()
	backend := "memory renderer"
	if accelerated {
		backend = "GPU accelerated"
	}
	if scale == 0 {
		return "DPI pending · " + backend
	}
	return fmt.Sprintf("DPI %.0f · scale %.2fx · %s", scale*96, scale, backend)
}

func (s *probeState) frameScale() float32 {
	bits := s.scaleBits.Load()
	if bits == 0 {
		return 0
	}
	return math.Float32frombits(bits)
}

func (s *probeState) frameAccelerated() bool {
	return s.accelerated.Load()
}

func (s *probeState) requestFrame() {
	if s.window != nil {
		s.window.Invalidate()
		return
	}
	if s.ctx != nil {
		s.ctx.Invalidate()
	}
}

func rectsIntersect(a, b ui.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

type benchRun struct {
	mu             sync.Mutex
	firstDraw      time.Time
	measureStart   time.Time
	lastFrame      time.Time
	drawTimes      []float64
	intervals      []float64
	visibleNodes   int
	visibleEdges   int
	scale          float32
	accelerated    bool
	totalNodes     int
	totalEdges     int
	warmup         time.Duration
	measure        time.Duration
	done           atomic.Bool
	doneCh         chan struct{}
	monitorStarted bool
}

func (b *benchRun) observe(draw time.Duration, visibleNodes, visibleEdges int, scale float32, accelerated bool, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done.Load() {
		return true
	}
	if b.firstDraw.IsZero() {
		b.firstDraw = now
		b.lastFrame = now
		return false
	}
	if now.Sub(b.firstDraw) < b.warmup {
		b.lastFrame = now
		return false
	}
	if b.measureStart.IsZero() {
		b.measureStart = now
		b.lastFrame = now
		b.visibleNodes, b.visibleEdges = visibleNodes, visibleEdges
		b.scale, b.accelerated = scale, accelerated
		return false
	}
	if now.Sub(b.measureStart) < b.measure {
		b.drawTimes = append(b.drawTimes, float64(draw)/float64(time.Millisecond))
		if !b.lastFrame.IsZero() {
			b.intervals = append(b.intervals, float64(now.Sub(b.lastFrame))/float64(time.Millisecond))
		}
		b.lastFrame = now
		b.visibleNodes, b.visibleEdges = visibleNodes, visibleEdges
		b.scale, b.accelerated = scale, accelerated
		return false
	}
	b.done.Store(true)
	if b.doneCh != nil {
		close(b.doneCh)
	}
	return true
}

func (s *probeState) startBenchMonitor() {
	if s.bench.monitorStarted || !s.cfg.bench {
		return
	}
	s.bench.monitorStarted = true
	go func() {
		<-s.bench.doneCh
		s.printBenchResult()
		mygo.App.Quit()
	}()
}

func (s *probeState) printBenchResult() {
	b := &s.bench
	b.mu.Lock()
	draws := append([]float64(nil), b.drawTimes...)
	intervals := append([]float64(nil), b.intervals...)
	totalNodes, totalEdges := b.totalNodes, b.totalEdges
	visibleNodes, visibleEdges := b.visibleNodes, b.visibleEdges
	scale, accelerated := b.scale, b.accelerated
	b.mu.Unlock()

	drawP95 := percentile(draws, 0.95)
	frameP95 := percentile(intervals, 0.95)
	seconds := s.cfg.benchMeasure.Seconds()
	fps := float64(len(draws)) / seconds
	fmt.Printf(
		"BENCH total_nodes=%d total_edges=%d visible_nodes=%d visible_edges=%d frames=%d duration_s=%.3f fps=%.2f callback_ms_p95=%.3f frame_interval_ms_p95=%.3f scale=%.2f accelerated=%t\n",
		totalNodes, totalEdges, visibleNodes, visibleEdges,
		len(draws), seconds, fps, drawP95, frameP95, scale, accelerated,
	)
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	idx = max(0, min(idx, len(sorted)-1))
	return sorted[idx]
}
