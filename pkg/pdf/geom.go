package pdf

import (
	"fmt"
	"math"
)

// Point represents a 2D coordinate in points.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BBox represents an axis-aligned bounding box defined by top-left (X0, Y0)
// and bottom-right (X1, Y1) in points, where X increases rightward and Y increases downward.
type BBox struct {
	X0 float64 `json:"x0"`
	Y0 float64 `json:"y0"`
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
}

// NewBBox creates a BBox ensuring X0 <= X1 and Y0 <= Y1.
func NewBBox(x0, y0, x1, y1 float64) BBox {
	minX, maxX := x0, x1
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := y0, y1
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	return BBox{X0: minX, Y0: minY, X1: maxX, Y1: maxY}
}

// Width returns the horizontal span of the bounding box.
func (b BBox) Width() float64 {
	return math.Max(0, b.X1-b.X0)
}

// Height returns the vertical span of the bounding box.
func (b BBox) Height() float64 {
	return math.Max(0, b.Y1-b.Y0)
}

// Area returns the surface area of the bounding box.
func (b BBox) Area() float64 {
	return b.Width() * b.Height()
}

// CenterX returns the horizontal center.
func (b BBox) CenterX() float64 {
	return (b.X0 + b.X1) / 2.0
}

// CenterY returns the vertical center.
func (b BBox) CenterY() float64 {
	return (b.Y0 + b.Y1) / 2.0
}

// Contains checks if point (x, y) is inside the box.
func (b BBox) Contains(p Point) bool {
	return p.X >= b.X0 && p.X <= b.X1 && p.Y >= b.Y0 && p.Y <= b.Y1
}

// Intersects returns true if this bounding box overlaps with another bounding box.
func (b BBox) Intersects(other BBox) bool {
	return !(b.X1 < other.X0 || b.X0 > other.X1 || b.Y1 < other.Y0 || b.Y0 > other.Y1)
}

// Intersection returns the overlapping region of two bounding boxes.
// If they do not intersect, it returns a zero-area BBox.
func (b BBox) Intersection(other BBox) BBox {
	if !b.Intersects(other) {
		return BBox{}
	}
	return NewBBox(
		math.Max(b.X0, other.X0),
		math.Max(b.Y0, other.Y0),
		math.Min(b.X1, other.X1),
		math.Min(b.Y1, other.Y1),
	)
}

// Union returns the smallest bounding box containing both bounding boxes.
func (b BBox) Union(other BBox) BBox {
	if b.Area() == 0 {
		return other
	}
	if other.Area() == 0 {
		return b
	}
	return NewBBox(
		math.Min(b.X0, other.X0),
		math.Min(b.Y0, other.Y0),
		math.Max(b.X1, other.X1),
		math.Max(b.Y1, other.Y1),
	)
}

// OverlapX returns the horizontal overlap width between two boxes.
func (b BBox) OverlapX(other BBox) float64 {
	overlap := math.Min(b.X1, other.X1) - math.Max(b.X0, other.X0)
	if overlap < 0 {
		return 0
	}
	return overlap
}

// OverlapY returns the vertical overlap height between two boxes.
func (b BBox) OverlapY(other BBox) float64 {
	overlap := math.Min(b.Y1, other.Y1) - math.Max(b.Y0, other.Y0)
	if overlap < 0 {
		return 0
	}
	return overlap
}

func (b BBox) String() string {
	return fmt.Sprintf("[%.2f, %.2f, %.2f, %.2f]", b.X0, b.Y0, b.X1, b.Y1)
}

// Line represents a 2D line segment.
type Line struct {
	P0    Point   `json:"p0"`
	P1    Point   `json:"p1"`
	Width float64 `json:"width"`
}

// IsHorizontal returns true if the line is approximately horizontal.
func (l Line) IsHorizontal(tolerance float64) bool {
	return math.Abs(l.P0.Y-l.P1.Y) <= tolerance && math.Abs(l.P0.X-l.P1.X) > tolerance
}

// IsVertical returns true if the line is approximately vertical.
func (l Line) IsVertical(tolerance float64) bool {
	return math.Abs(l.P0.X-l.P1.X) <= tolerance && math.Abs(l.P0.Y-l.P1.Y) > tolerance
}

// Length returns the Euclidean length of the line.
func (l Line) Length() float64 {
	dx := l.P1.X - l.P0.X
	dy := l.P1.Y - l.P0.Y
	return math.Hypot(dx, dy)
}

// BBox returns the bounding box for the line segment.
func (l Line) BBox() BBox {
	return NewBBox(l.P0.X, l.P0.Y, l.P1.X, l.P1.Y)
}
