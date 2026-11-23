// Package renderer draws a graph.Scene into a MyGo native Canvas.
package renderer

import (
	"math"
	"sort"
	"strings"
	"unicode/utf16"

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
	farScale           = 0.36
	mediumEnterScale   = 0.44
	nearEnterScale     = 0.85
	nearExitScale      = 0.75
	gridTileDIP        = 64.0
	gridPixelMin       = 32
	gridPixelMax       = 256
	edgeStrokeWidth    = 1.5
	selectedLineWidth  = 2.5
	arrowLengthDIP     = 7.0
	arrowHalfWidthDIP  = 3.5
	arrowLineWidthDIP  = 1.4
	arrowMinSegmentDIP = 6.0
	hoverLineWidth     = 2.0
	titleTop           = 5.0
	codeTop            = 19.0
	fieldTopNoCode     = 23.0
	fieldTopWithCode   = 33.0
	fieldRowHeight     = 16.0
	maxFieldRows       = 5
	labelInset         = 8

	referenceNodeWidth = 236.0
	nodeVisualLimit    = 256
	prefillNodeLimit   = 64
	labelInternLimit   = 512
	maxPreviewRunes    = 256
)

// FrameStats describes one Draw call. CacheHits and CacheMisses count only
// the single reusable grid-tile lookup; retained title and field TextLayouts
// are not counted as cache hits or misses.
type FrameStats struct {
	VisibleNodes    int
	VisibleEdges    int
	UnpreparedNodes int
	Arrows          int
	FieldRows       int
	CacheHits       int
	CacheMisses     int
	LOD             LOD
}

type labelVariant struct {
	width  float32
	height float32
	layout *ui.TextLayout
}

type labelVisual struct {
	full       *ui.TextLayout
	fullWidth  float32
	fullHeight float32
	variants   []labelVariant
}

func (l labelVisual) pick(availableWidth, availableHeight float32) *ui.TextLayout {
	if l.full == nil || availableWidth <= 0 || availableHeight <= 0 {
		return nil
	}
	if l.fullWidth <= availableWidth && l.fullHeight <= availableHeight {
		return l.full
	}
	var best *ui.TextLayout
	bestWidth := float32(-1)
	for _, variant := range l.variants {
		if variant.layout != nil && variant.width <= availableWidth &&
			variant.height <= availableHeight && variant.width > bestWidth {
			best, bestWidth = variant.layout, variant.width
		}
	}
	return best
}

type nodeVisual struct {
	name        labelVisual
	code        labelVisual
	fieldLabels []labelVisual
	fieldTop    float32
	table       *tableVisual
}

type labelKey struct {
	text string
	font ui.Font
}

type labelCache struct {
	entries  map[labelKey]*labelVisual
	order    []labelKey
	ellipsis map[ui.Font]*ui.TextLayout
	limit    int
}

func newLabelCache(limit int) labelCache {
	return labelCache{
		entries:  make(map[labelKey]*labelVisual),
		ellipsis: make(map[ui.Font]*ui.TextLayout),
		limit:    limit,
	}
}

func (c *labelCache) label(text string, font ui.Font) *labelVisual {
	text = previewText(text)
	if text == "" {
		return nil
	}
	key := labelKey{text: text, font: font}
	if visual := c.entries[key]; visual != nil {
		return visual
	}
	if c.limit > 0 && len(c.order) >= c.limit {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	ellipsis := c.ellipsis[font]
	if ellipsis == nil {
		ellipsis = ui.ShapeText("…", font, 0)
		c.ellipsis[font] = ellipsis
	}
	visual := shapeLabelVariants(text, font, ellipsis)
	c.entries[key] = &visual
	c.order = append(c.order, key)
	return &visual
}

type gridKey struct {
	scaleBits uint32
	dark      bool
}

// Canvas is a retained scene renderer. Text layouts are cached per canvas for
// the lifetime of one board; replacing the canvas discards them. Node visuals
// use deterministic FIFO eviction, and identical text/font layouts are interned
// under a separate bounded cache. Prepare and draw on the same UI thread
// after handing off the canvas from its constructor.
type Canvas struct {
	scene          graph.Scene
	index          graph.SpatialQuery
	nodes          map[int]*nodeVisual
	nodeOrder      []int
	labels         labelCache
	annotations    []annotationVisual
	lodWidthFactor float64
	lod            LOD
	grid           *ui.Canvas
	gridKey        gridKey
	hasGrid        bool
	lastGridTiles  int

	nodeOffsets    []int
	edgeOffsets    []int
	prepareOffsets []int
	edgeScratch    [][2]float32
	arrowScratch   [][2]float32
}

// New prepares a bounded initial set of text layouts. A nil index uses a
// linear query baseline.
func New(scene graph.Scene, index graph.SpatialQuery) *Canvas {
	if index == nil {
		index = graph.NewLinearIndex(scene)
	}
	canvas := &Canvas{
		scene:          scene,
		index:          index,
		nodes:          make(map[int]*nodeVisual),
		labels:         newLabelCache(labelInternLimit),
		annotations:    prepareAnnotations(scene.Annotations),
		lodWidthFactor: sceneLODWidthFactor(scene.Nodes),
	}
	for offset := 0; offset < min(len(scene.Nodes), prefillNodeLimit); offset++ {
		canvas.PrepareNode(offset)
	}
	return canvas
}

// PrepareNode builds and retains one node's text outside Draw. It returns false
// for an invalid offset. When the cache is full, the oldest prepared node is
// discarded deterministically.
func (c *Canvas) PrepareNode(offset int) bool {
	if c == nil || offset < 0 || offset >= len(c.scene.Nodes) {
		return false
	}
	if c.nodes[offset] != nil {
		return true
	}
	if len(c.nodes) >= nodeVisualLimit {
		oldest := c.nodeOrder[0]
		c.nodeOrder = c.nodeOrder[1:]
		delete(c.nodes, oldest)
	}
	node := c.scene.Nodes[offset]
	if node.Presentation != nil {
		c.nodes[offset] = &nodeVisual{table: c.prepareTable(node)}
		c.nodeOrder = append(c.nodeOrder, offset)
		return true
	}
	titleFont := ui.Font{Size: 11.5, Weight: 650}
	codeFont := ui.Font{Size: 9.5, Weight: 500}
	fieldFont := ui.Font{Size: 9.5, Weight: 400}
	nameText, codeText := nodeDisplayText(node)
	name := c.labels.label(nameText, titleFont)
	code := c.labels.label(codeText, codeFont)
	fields := node.Fields
	if len(fields) > maxFieldRows {
		fields = fields[:maxFieldRows]
	}
	labels := make([]labelVisual, len(fields))
	for offset, field := range fields {
		if visual := c.labels.label(fieldDisplayText(field), fieldFont); visual != nil {
			labels[offset] = *visual
		}
	}
	visual := &nodeVisual{fieldLabels: labels, fieldTop: fieldTopNoCode}
	if name != nil {
		visual.name = *name
	}
	if code != nil {
		visual.code = *code
		visual.fieldTop = fieldTopWithCode
	}
	c.nodes[offset] = visual
	c.nodeOrder = append(c.nodeOrder, offset)
	return true
}

// PrepareVisible prepares up to the node-cache capacity of visible nodes and
// prioritizes the selected node. Existing offscreen visuals stay cached while
// capacity permits; only the oldest unpinned entries needed for missing desired
// nodes are evicted. It must be called outside Draw when a board exceeds the
// eager preparation window.
func (c *Canvas) PrepareVisible(camera *graph.Camera, selected graph.Hit) int {
	if c == nil || camera == nil {
		return 0
	}
	if c.nextLOD(camera.Scale()*c.lodWidthFactor) == Far {
		return 0
	}
	c.prepareOffsets = c.index.QueryNodes(camera.VisibleWorld(), c.prepareOffsets[:0])
	desired := make([]int, 0, nodeVisualLimit)
	desiredSet := make(map[int]bool)
	addDesired := func(offset int) {
		if offset >= 0 && offset < len(c.scene.Nodes) && !desiredSet[offset] &&
			len(desired) < nodeVisualLimit {
			desiredSet[offset] = true
			desired = append(desired, offset)
		}
	}
	if selected.Kind == graph.HitNode && selected.Index >= 0 && selected.Index < len(c.scene.Nodes) {
		addDesired(selected.Index)
	}
	for _, offset := range c.prepareOffsets {
		addDesired(offset)
	}

	missing := 0
	for _, offset := range desired {
		if c.nodes[offset] == nil {
			missing++
		}
	}
	evict := max(0, len(c.nodes)+missing-nodeVisualLimit)
	kept := c.nodeOrder[:0]
	for _, offset := range c.nodeOrder {
		if !desiredSet[offset] && evict > 0 {
			delete(c.nodes, offset)
			evict--
		} else {
			kept = append(kept, offset)
		}
	}
	c.nodeOrder = kept

	prepared := 0
	for _, offset := range desired {
		if c.nodes[offset] == nil && c.PrepareNode(offset) {
			prepared++
		}
	}
	return prepared
}

func (c *Canvas) nodeVisual(offset int) *nodeVisual {
	if c == nil {
		return nil
	}
	return c.nodes[offset]
}

func sceneLODWidthFactor(nodes []graph.Node) float64 {
	widths := make([]float64, 0, len(nodes))
	for _, node := range nodes {
		width := node.Bounds.Width()
		if finiteFloat(width) && width > 0 {
			widths = append(widths, width)
		}
	}
	if len(widths) == 0 {
		return 1
	}
	sort.Float64s(widths)
	median := widths[len(widths)/2]
	if len(widths)%2 == 0 {
		median = (widths[len(widths)/2-1] + median) / 2
	}
	if median <= 0 || !finiteFloat(median) {
		return 1
	}
	return median / referenceNodeWidth
}

func previewText(text string) string {
	if text == "" {
		return ""
	}
	count := 0
	for offset := range text {
		if count == maxPreviewRunes {
			return text[:offset] + "…"
		}
		count++
	}
	return text
}

func finiteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func shapeLabel(text string, font ui.Font, width float32, ellipsis *ui.TextLayout) *ui.TextLayout {
	layout := ui.ShapeText(text, font, 0)
	return shapeLabelLayout(layout, text, font, width, ellipsis)
}

func shapeLabelLayout(
	layout *ui.TextLayout,
	text string,
	font ui.Font,
	width float32,
	ellipsis *ui.TextLayout,
) *ui.TextLayout {
	if measured, _ := layout.Size(); measured <= width {
		return layout
	}
	ellipsisWidth, _ := ellipsis.Size()
	if ellipsisWidth > width {
		return nil
	}
	units := utf16.Encode([]rune(text))
	caret := layout.PositionAt(ui.Point{X: width - ellipsisWidth})
	for caret.Index > 0 {
		prefix := string(utf16.Decode(units[:caret.Index]))
		short := ui.ShapeText(prefix+"…", font, 0)
		if measured, _ := short.Size(); measured <= width {
			return short
		}
		previous := layout.MoveCaret(caret, -1)
		if previous.Index >= caret.Index {
			break
		}
		caret = previous
	}
	return ellipsis
}

var nodeLabelWidths = []float32{180, 112, 72}

func shapeLabelVariants(text string, font ui.Font, ellipsis *ui.TextLayout) labelVisual {
	if text == "" {
		return labelVisual{}
	}
	full := ui.ShapeText(text, font, 0)
	fullWidth, fullHeight := full.Size()
	visual := labelVisual{full: full, fullWidth: fullWidth, fullHeight: fullHeight}
	seen := map[*ui.TextLayout]bool{full: true}
	for _, width := range nodeLabelWidths {
		if layout := shapeLabelLayout(full, text, font, width, ellipsis); layout != nil {
			if seen[layout] {
				continue
			}
			seen[layout] = true
			measuredWidth, height := layout.Size()
			visual.variants = append(visual.variants, labelVariant{
				width:  measuredWidth,
				height: height,
				layout: layout,
			})
		}
	}
	return visual
}

func nodeDisplayText(node graph.Node) (string, string) {
	name, code := singleLineDisplay(node.Name), singleLineDisplay(node.Code)
	if strings.TrimSpace(name) == "" {
		name, code = code, ""
	}
	if strings.TrimSpace(code) == strings.TrimSpace(name) {
		code = ""
	}
	return name, code
}

func fieldDisplayText(field graph.Field) string {
	marker := ""
	switch {
	case field.PrimaryKey && field.ForeignKey:
		marker = "PK/FK"
	case field.PrimaryKey:
		marker = "PK"
	case field.ForeignKey:
		marker = "FK"
	}
	name := singleLineDisplay(field.Name)
	dataType := singleLineDisplay(field.DataType)
	text := name
	if dataType != "" && dataType != name {
		if text != "" {
			text += "  "
		}
		text += dataType
	}
	if marker != "" {
		if text == "" {
			return marker
		}
		return marker + " " + text
	}
	return text
}

func singleLineDisplay(text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	return strings.ReplaceAll(text, "\n", " ")
}

// Draw paints the visible scene inside area. camera's viewport is expected to
// describe the same content rectangle; its world transform determines the
// world-to-screen mapping.
func (c *Canvas) Draw(p *ui.Painter, area ui.Rect, camera *graph.Camera, selected graph.Hit) FrameStats {
	return c.DrawWithHover(p, area, camera, selected, graph.Hit{Kind: graph.HitNone, Index: -1})
}

// DrawWithHover paints the visible scene and distinguishes selected and hovered
// table nodes. Edges retain the existing selected-edge behavior.
func (c *Canvas) DrawWithHover(
	p *ui.Painter,
	area ui.Rect,
	camera *graph.Camera,
	selected graph.Hit,
	hovered graph.Hit,
) FrameStats {
	stats := FrameStats{}
	if p == nil || camera == nil || area.W <= 0 || area.H <= 0 {
		return stats
	}
	stats.LOD = c.updateLOD(camera.Scale() * c.lodWidthFactor)
	t := p.Theme()
	visible := camera.VisibleWorld()

	p.Clip(area, 0, func() {
		c.drawGrid(p, area, camera, t, &stats)
		edgePadding := edgeCullPaddingDIP(stats.LOD) / camera.Scale()
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
			if c.drawEdge(p, offset, selected, camera, stats.LOD, t) {
				stats.Arrows++
			}
		}
		for _, offset := range c.nodeOffsets {
			if offset < 0 || offset >= len(c.scene.Nodes) {
				continue
			}
			if stats.LOD != Far && c.nodes[offset] == nil {
				stats.UnpreparedNodes++
			}
			stats.FieldRows += c.drawNode(p, offset, selected, hovered, camera, t, stats.LOD)
		}
		c.drawAnnotations(p, camera, visible, t)
	})
	return stats
}

func (c *Canvas) updateLOD(scale float64) LOD {
	c.lod = c.nextLOD(scale)
	return c.lod
}

func (c *Canvas) nextLOD(scale float64) LOD {
	if c.lod == "" {
		switch {
		case scale > nearEnterScale:
			return Near
		case scale >= farScale:
			return Medium
		default:
			return Far
		}
	}
	switch c.lod {
	case Far:
		switch {
		case scale > nearEnterScale:
			return Near
		case scale > mediumEnterScale:
			return Medium
		}
	case Medium:
		switch {
		case scale < farScale:
			return Far
		case scale > nearEnterScale:
			return Near
		}
	case Near:
		switch {
		case scale < farScale:
			return Far
		case scale < nearExitScale:
			return Medium
		}
	}
	return c.lod
}

func (c *Canvas) drawGrid(p *ui.Painter, area ui.Rect, camera *graph.Camera, t *ui.Theme, stats *FrameStats) {
	key := makeGridKey(t, p.Scale())
	if !c.hasGrid || c.grid == nil || key != c.gridKey {
		pixels := gridPixels(p.Scale())
		c.grid = ui.NewCanvas(pixels, pixels)
		c.grid.Draw(p, func(cp *ui.Painter) {
			size := float32(pixels)
			bg := canvasBackground(t)
			ink := ui.RGB(0, 0, 0)
			if t.Dark {
				ink = ui.RGB(255, 255, 255)
			}
			line := bg.Mix(ink, 0.14)
			cp.Fill(ui.Rect{W: size, H: size}, bg, 0)
			cp.Fill(ui.Rect{W: size, H: 1}, line, 0)
			cp.Fill(ui.Rect{W: 1, H: size}, line, 0)
		})
		c.gridKey, c.hasGrid = key, true
		stats.CacheMisses++
	} else {
		stats.CacheHits++
	}

	// Doubling the world spacing as zoom decreases retains shared grid lines
	// while keeping the screen-space tile size between 64 and 128 DIPs.
	mantissa, _ := math.Frexp(camera.Scale())
	step := gridTileDIP * 2 * mantissa
	worldMin := camera.VisibleWorld().Min
	phaseX := math.Mod(worldMin.X*camera.Scale(), step)
	phaseY := math.Mod(worldMin.Y*camera.Scale(), step)
	if phaseX < 0 {
		phaseX += step
	}
	if phaseY < 0 {
		phaseY += step
	}
	c.lastGridTiles = 0
	for y := float64(area.Y) - phaseY; y < float64(area.Y+area.H); y += step {
		for x := float64(area.X) - phaseX; x < float64(area.X+area.W); x += step {
			p.Image(c.grid.Bitmap(), ui.Rect{
				X: float32(x), Y: float32(y),
				W: float32(step), H: float32(step),
			}, ui.FillBox)
			c.lastGridTiles++
		}
	}
}

func (c *Canvas) drawEdge(
	p *ui.Painter,
	offset int,
	selected graph.Hit,
	camera *graph.Camera,
	lod LOD,
	t *ui.Theme,
) bool {
	edge := c.scene.Edges[offset]
	points := edgeScreenPoints(c.edgeScratch[:0], camera, edge.Points)
	c.edgeScratch = points
	highlighted := edgeHighlighted(edge, offset, selected)
	width := edgeLineWidth(lod, highlighted)
	color := canvasOutline(t.Border, t)
	if highlighted {
		color = t.Accent
	}
	p.StrokePolyline(points, width, color)

	if lod == Far || !edge.Directed {
		return false
	}
	arrow := edgeArrow(c.arrowScratch[:0], points)
	c.arrowScratch = arrow
	if len(arrow) == 3 {
		p.StrokePolyline(arrow, min(width, float32(arrowLineWidthDIP)), color)
		return true
	}
	return false
}

func edgeScreenPoints(dst [][2]float32, camera *graph.Camera, points []graph.Point) [][2]float32 {
	if camera == nil {
		return dst
	}
	for _, point := range points {
		screen := camera.WorldToScreen(point)
		dst = append(dst, [2]float32{float32(screen.X), float32(screen.Y)})
	}
	return dst
}

func edgeHighlighted(edge graph.Edge, offset int, selected graph.Hit) bool {
	switch selected.Kind {
	case graph.HitEdge:
		return selected.Index == offset
	case graph.HitNode:
		return edge.From == selected.Index || edge.To == selected.Index
	default:
		return false
	}
}

func edgeLineWidth(lod LOD, highlighted bool) float32 {
	if highlighted {
		return selectedLineWidth
	}
	switch lod {
	case Far:
		return 1
	case Medium:
		return 1.25
	default:
		return edgeStrokeWidth
	}
}

func edgeCullPaddingDIP(lod LOD) float64 {
	padding := float64(max(selectedLineWidth, edgeLineWidth(lod, false))) / 2
	if lod != Far {
		padding = max(
			padding,
			math.Hypot(arrowLengthDIP, arrowHalfWidthDIP)+arrowLineWidthDIP/2,
		)
	}
	return padding
}

func edgeArrow(dst [][2]float32, points [][2]float32) [][2]float32 {
	if len(points) < 2 {
		return dst
	}
	end := points[len(points)-1]
	start := end
	for offset := len(points) - 2; offset >= 0; offset-- {
		if points[offset] != end {
			start = points[offset]
			break
		}
	}
	if start == end {
		return dst
	}
	dx, dy := float64(end[0]-start[0]), float64(end[1]-start[1])
	length := math.Hypot(dx, dy)
	if length < arrowMinSegmentDIP {
		return dst
	}
	ux, uy := float32(dx/length), float32(dy/length)
	nx, ny := -uy, ux
	arrowLength := min(float32(arrowLengthDIP), float32(length)*0.8)
	baseX, baseY := end[0]-ux*arrowLength, end[1]-uy*arrowLength
	return append(dst,
		[2]float32{baseX + nx*arrowHalfWidthDIP, baseY + ny*arrowHalfWidthDIP},
		end,
		[2]float32{baseX - nx*arrowHalfWidthDIP, baseY - ny*arrowHalfWidthDIP},
	)
}

func (c *Canvas) drawNode(
	p *ui.Painter,
	offset int,
	selected, hovered graph.Hit,
	camera *graph.Camera,
	t *ui.Theme,
	lod LOD,
) int {
	node := c.scene.Nodes[offset]
	rect := canvasRect(camera, node.Bounds)
	if node.Presentation != nil {
		return c.drawTable(p, rect, node, c.nodeVisual(offset), t, lod, camera,
			selected.Kind == graph.HitNode && selected.Index == offset,
			hovered.Kind == graph.HitNode && hovered.Index == offset,
		)
	}
	fill, border, width := nodeCardStyle(
		t,
		selected.Kind == graph.HitNode && selected.Index == offset,
		hovered.Kind == graph.HitNode && hovered.Index == offset,
	)
	drawCardShadow(p, rect, 5, t)
	p.Fill(rect, fill, 5)
	p.Stroke(rect, canvasOutline(border, t), 5, width)
	if lod == Far {
		return 0
	}

	visual := c.nodeVisual(offset)
	if visual == nil {
		return 0
	}
	availableWidth := max(0, rect.W-2*labelInset)
	availableHeight := max(0, rect.H-visual.fieldTop-2)
	rows := min(len(visual.fieldLabels), int(availableHeight/fieldRowHeight))
	drawnRows := 0
	if lod == Medium {
		rows = min(rows, 2)
	}
	if title := visual.name.pick(availableWidth, rect.H-titleTop-2); title != nil {
		p.TextLayout(title, rect.X+labelInset, rect.Y+titleTop, t.Text)
	}
	if code := visual.code.pick(availableWidth, rect.H-codeTop-2); code != nil {
		p.TextLayout(code, rect.X+labelInset, rect.Y+codeTop, t.TextMuted)
	}
	for i := 0; i < rows; i++ {
		label := visual.fieldLabels[i].pick(availableWidth, fieldRowHeight)
		if label == nil {
			continue
		}
		p.TextLayout(label, rect.X+labelInset, rect.Y+visual.fieldTop+float32(i)*fieldRowHeight, t.TextMuted)
		drawnRows++
	}
	return drawnRows
}

func nodeCardStyle(t *ui.Theme, selected, hovered bool) (fill, border ui.Color, width float32) {
	fill, border, width = t.Surface, t.Border, 1
	if hovered {
		fill = t.Surface.Mix(t.Accent, 0.08)
		border = t.Border.Mix(t.Accent, 0.70)
		width = hoverLineWidth
	}
	if selected {
		fill = t.Surface.Mix(t.Accent, 0.14)
		border = t.Accent
		width = selectedLineWidth
	}
	return fill, border, width
}

func gridPixels(scale float32) int {
	pixels := int(math.Round(float64(gridTileDIP * scale)))
	return min(max(pixels, gridPixelMin), gridPixelMax)
}

func makeGridKey(t *ui.Theme, scale float32) gridKey {
	return gridKey{
		scaleBits: math.Float32bits(scale),
		dark:      t.Dark,
	}
}

func canvasRect(camera *graph.Camera, bounds graph.Rect) ui.Rect {
	screenMin := camera.WorldToScreen(bounds.Min)
	screenMax := camera.WorldToScreen(bounds.Max)
	return ui.Rect{
		X: float32(screenMin.X), Y: float32(screenMin.Y),
		W: float32(screenMax.X - screenMin.X), H: float32(screenMax.Y - screenMin.Y),
	}
}
