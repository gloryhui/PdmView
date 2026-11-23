package graph

import "pdmview/internal/pdm"

// Field is one displayable field of a node.
type Field struct {
	Name       string
	DataType   string
	PrimaryKey bool
	ForeignKey bool
}

// Node is one table node. IDs are stable zero-based indexes in its Scene.
type Node struct {
	ID           int
	Bounds       Rect
	Name         string
	Code         string
	Fields       []Field
	Presentation *TablePresentation
}

type TablePresentation struct {
	Style      pdm.TableAppearance
	Title      string
	Stereotype string
	Comment    string
	Columns    []ColumnPresentation
}

type ColumnPresentation struct {
	Name       string
	DataType   string
	Indicators string
	PrimaryKey bool
	ForeignKey bool
}

// Edge is one directed relationship. From and To are Node IDs or -1 for an
// unresolved endpoint. Points holds the display path when available.
type Edge struct {
	ID       int
	From     int
	To       int
	Directed bool
	Points   []Point
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

// Annotation is one source rectangle annotation.
type Annotation struct {
	Bounds    Rect
	Text      string
	Fill      *pdm.RGBColor
	Border    *pdm.RGBColor
	TextColor *pdm.RGBColor
}

// Scene is a deterministic graph scene.
type Scene struct {
	Nodes       []Node
	Edges       []Edge
	Annotations []Annotation
	Bounds      Rect
}
