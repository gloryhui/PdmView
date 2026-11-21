// Package graph holds toolkit-free graph geometry and synthetic fixtures for
// the native canvas. It does not depend on MyGo, PDM parsing, or SQL.
package graph

import (
	"errors"
	"fmt"
)

// DefaultSeed is the reproducible seed used by GenerateDefault.
const DefaultSeed uint64 = 20261121

var (
	ErrUnsupportedCount = errors.New("graph: synthetic count must be 50, 150, 300, or 500")
	ErrInvalidDensity   = errors.New("graph: density must be DensitySparse or DensityDense")
)

// Density chooses the fixture layout spacing.
type Density uint8

const (
	DensitySparse Density = iota
	DensityDense
)

func (d Density) String() string {
	switch d {
	case DensitySparse:
		return "sparse"
	case DensityDense:
		return "dense"
	default:
		return fmt.Sprintf("Density(%d)", d)
	}
}

// Point is a world-space coordinate.
type Point struct {
	X float64
	Y float64
}

// Rect is an axis-aligned world-space rectangle.
type Rect struct {
	Min Point
	Max Point
}

func (r Rect) Width() float64  { return r.Max.X - r.Min.X }
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Union returns the smallest rectangle containing r and other.
func (r Rect) Union(other Rect) Rect {
	return Rect{
		Min: Point{X: min(r.Min.X, other.Min.X), Y: min(r.Min.Y, other.Min.Y)},
		Max: Point{X: max(r.Max.X, other.Max.X), Y: max(r.Max.Y, other.Max.Y)},
	}
}

// Field is one displayable field of a synthetic node.
type Field struct {
	Name string
}

// Node is one synthetic table node. IDs are stable zero-based indexes in the
// deterministic row-major node order.
type Node struct {
	ID     int
	Bounds Rect
	Name   string
	Fields []Field
}

// Edge is one directed relationship. From and To are Node IDs. Points is the
// sampled display path, starting and ending at actual node port centers.
type Edge struct {
	ID     int
	From   int
	To     int
	Points []Point
}

// Bounds returns the axis-aligned bounds of the edge's sampled path.
func (e Edge) Bounds() Rect {
	if len(e.Points) == 0 {
		return Rect{}
	}
	bounds := Rect{Min: e.Points[0], Max: e.Points[0]}
	for _, p := range e.Points[1:] {
		bounds.Min.X = min(bounds.Min.X, p.X)
		bounds.Min.Y = min(bounds.Min.Y, p.Y)
		bounds.Max.X = max(bounds.Max.X, p.X)
		bounds.Max.Y = max(bounds.Max.Y, p.Y)
	}
	return bounds
}

// Scene is a deterministic synthetic graph fixture.
type Scene struct {
	Nodes  []Node
	Edges  []Edge
	Bounds Rect
}

// Generate creates a reproducible synthetic scene with exactly two edges per
// node. count must be 50, 150, 300, or 500. Node and edge IDs are stable
// zero-based values, nodes are row-major, and edges are ordered by From then
// generation slot.
func Generate(count int, seed uint64, density Density) (Scene, error) {
	if !supportedCount(count) {
		return Scene{}, fmt.Errorf("%w: %d", ErrUnsupportedCount, count)
	}
	if density != DensitySparse && density != DensityDense {
		return Scene{}, fmt.Errorf("%w: %v", ErrInvalidDensity, density)
	}

	const columns = 10
	gapX, gapY := 180.0, 120.0
	if density == DensityDense {
		gapX, gapY = 24.0, 20.0
	}

	scene := Scene{
		Nodes: make([]Node, 0, count),
		Edges: make([]Edge, 0, count*2),
	}
	for i := 0; i < count; i++ {
		col := i % columns
		row := i / columns
		x := float64(col) * (nodeWidth + gapX)
		y := float64(row) * (nodeHeight + gapY)
		node := Node{
			ID: i,
			Bounds: Rect{
				Min: Point{X: x, Y: y},
				Max: Point{X: x + nodeWidth, Y: y + nodeHeight},
			},
			Name:   fmt.Sprintf("%s %03d", nodeNames[i%len(nodeNames)], i+1),
			Fields: syntheticFields(i),
		}
		scene.Nodes = append(scene.Nodes, node)
	}

	rng := splitMix64{state: seed}
	forced := forcedTargets(count, columns)
	for from := 0; from < count; from++ {
		first, forcedSource := forced[from]
		if !forcedSource {
			first = (from + 1 + rng.intn(count-1)) % count
		}
		secondOffset := 1 + rng.intn(count-2)
		if secondOffset >= (first-from+count)%count {
			secondOffset++
		}
		for _, to := range [...]int{first, (from + secondOffset) % count} {
			points := routeEdge(scene.Nodes[from].Bounds, scene.Nodes[to].Bounds)
			edge := Edge{
				ID:     len(scene.Edges),
				From:   from,
				To:     to,
				Points: points,
			}
			scene.Edges = append(scene.Edges, edge)
		}
	}

	scene.Bounds = scene.Nodes[0].Bounds
	for _, node := range scene.Nodes[1:] {
		scene.Bounds = scene.Bounds.Union(node.Bounds)
	}
	for _, edge := range scene.Edges {
		scene.Bounds = scene.Bounds.Union(edge.Bounds())
	}
	return scene, nil
}

// GenerateDefault is Generate with DefaultSeed.
func GenerateDefault(count int, density Density) (Scene, error) {
	return Generate(count, DefaultSeed, density)
}

const (
	nodeWidth  = 236.0
	nodeHeight = 144.0
)

func supportedCount(count int) bool {
	switch count {
	case 50, 150, 300, 500:
		return true
	default:
		return false
	}
}

func forcedTargets(count, columns int) map[int]int {
	rows := (count + columns - 1) / columns
	return map[int]int{
		0:           count - 1,
		columns - 1: (rows - 1) * columns,
	}
}

func routeEdge(from, to Rect) []Point {
	start, startNormal, end, endNormal := ports(from, to)
	span := abs(end.X-start.X) + abs(end.Y-start.Y)
	reach := min(max(span*0.35, 48), 320)
	c1 := Point{X: start.X + startNormal.X*reach, Y: start.Y + startNormal.Y*reach}
	c2 := Point{X: end.X + endNormal.X*reach, Y: end.Y + endNormal.Y*reach}
	points := make([]Point, 5)
	for i := range points {
		t := float64(i) / float64(len(points)-1)
		points[i] = cubicPoint(start, c1, c2, end, t)
	}
	points[0], points[len(points)-1] = start, end
	return points
}

func ports(from, to Rect) (start, startNormal, end, endNormal Point) {
	fcx, fcy := rectCenter(from)
	tcx, tcy := rectCenter(to)
	switch {
	case abs(tcx-fcx) >= abs(tcy-fcy) && tcx >= fcx:
		return Point{X: from.Max.X, Y: fcy}, Point{X: 1}, Point{X: to.Min.X, Y: tcy}, Point{X: -1}
	case abs(tcx-fcx) >= abs(tcy-fcy):
		return Point{X: from.Min.X, Y: fcy}, Point{X: -1}, Point{X: to.Max.X, Y: tcy}, Point{X: 1}
	case tcy >= fcy:
		return Point{X: fcx, Y: from.Max.Y}, Point{Y: 1}, Point{X: tcx, Y: to.Min.Y}, Point{Y: -1}
	default:
		return Point{X: fcx, Y: from.Min.Y}, Point{Y: -1}, Point{X: tcx, Y: to.Max.Y}, Point{Y: 1}
	}
}

func rectCenter(r Rect) (float64, float64) {
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2
}

func cubicPoint(p0, p1, p2, p3 Point, t float64) Point {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return Point{
		X: a*p0.X + b*p1.X + c*p2.X + d*p3.X,
		Y: a*p0.Y + b*p1.Y + c*p2.Y + d*p3.Y,
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

type splitMix64 struct {
	state uint64
}

func (r *splitMix64) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *splitMix64) intn(n int) int {
	return int(r.next() % uint64(n))
}

var nodeNames = []string{
	"客户订单明细与履约记录 / Customer Order Fulfillment Detail",
	"仓储库存批次与调拨记录 / Warehouse Inventory Batch Transfer",
	"结算账单与优惠分摊明细 / Settlement Invoice Discount Allocation",
	"商品主数据与多语言描述 / Product Master and Localized Description",
	"运输线路与承运商轨迹 / Shipping Route and Carrier Tracking",
	"审批流程与审计留痕 / Approval Workflow Audit Trail",
	"会员权益与积分流水 / Membership Benefit and Point Ledger",
	"售后工单与退款处理 / After Sales Ticket and Refund Processing",
}

var fieldNames = []string{
	"主键标识 / Primary Identifier",
	"业务单号与外部引用 / Business Reference and External Identifier",
	"创建时间与最后更新时间 / Created At and Last Updated Timestamp",
	"状态、批次及区域 / Status, Batch, and Regional Scope",
	"扩展备注与双语说明 / Extended Notes and Bilingual Description",
}

func syntheticFields(index int) []Field {
	fields := make([]Field, len(fieldNames))
	for i, name := range fieldNames {
		fields[i] = Field{Name: fmt.Sprintf("%s %03d", name, index+1)}
	}
	return fields
}
