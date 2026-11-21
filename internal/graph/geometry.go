package graph

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
