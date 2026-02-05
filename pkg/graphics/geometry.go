package graphics

import "math"

// epsilon is the tolerance for floating-point comparisons.
const epsilon = 0.0001

// Offset represents a 2D point or vector in pixel coordinates.
type Offset struct {
	X float64
	Y float64
}

// Size represents width and height dimensions in pixels.
type Size struct {
	Width  float64
	Height float64
}

// Rect represents a rectangle using left, top, right, bottom coordinates.
type Rect struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// RectFromLTWH constructs a Rect from left, top, width, height values.
func RectFromLTWH(left, top, width, height float64) Rect {
	return Rect{
		Left:   left,
		Top:    top,
		Right:  left + width,
		Bottom: top + height,
	}
}

// Width returns the width of the rectangle.
func (r Rect) Width() float64 {
	return r.Right - r.Left
}

// Height returns the height of the rectangle.
func (r Rect) Height() float64 {
	return r.Bottom - r.Top
}

// Size returns the size of the rectangle.
func (r Rect) Size() Size {
	return Size{Width: r.Width(), Height: r.Height()}
}

// Center returns the center point of the rectangle.
func (r Rect) Center() Offset {
	return Offset{
		X: (r.Left + r.Right) * 0.5,
		Y: (r.Top + r.Bottom) * 0.5,
	}
}

// Radius represents corner radii for rounded rectangles.
type Radius struct {
	X float64
	Y float64
}

// CircularRadius creates a circular radius with equal X/Y values.
func CircularRadius(value float64) Radius {
	return Radius{X: value, Y: value}
}

// RRect represents a rounded rectangle with per-corner radii.
type RRect struct {
	Rect        Rect
	TopLeft     Radius
	TopRight    Radius
	BottomRight Radius
	BottomLeft  Radius
}

// RRectFromRectAndRadius creates a rounded rectangle with uniform corner radii.
func RRectFromRectAndRadius(rect Rect, radius Radius) RRect {
	return RRect{
		Rect:        rect,
		TopLeft:     radius,
		TopRight:    radius,
		BottomRight: radius,
		BottomLeft:  radius,
	}
}

// UniformRadius returns a single radius value if all corners match, or 0 if not.
func (r RRect) UniformRadius() float64 {
	v := r.TopLeft.X
	if !floatEqual(r.TopLeft.Y, v) ||
		!floatEqual(r.TopRight.X, v) ||
		!floatEqual(r.TopRight.Y, v) ||
		!floatEqual(r.BottomRight.X, v) ||
		!floatEqual(r.BottomRight.Y, v) ||
		!floatEqual(r.BottomLeft.X, v) ||
		!floatEqual(r.BottomLeft.Y, v) {
		return 0
	}
	return v
}

// floatEqual returns true if two float64 values are approximately equal.
func floatEqual(a, b float64) bool {
	return math.Abs(a-b) <= epsilon
}

// Intersect returns the intersection of two rectangles.
// Returns empty rect if they don't overlap.
func (r Rect) Intersect(other Rect) Rect {
	left := math.Max(r.Left, other.Left)
	top := math.Max(r.Top, other.Top)
	right := math.Min(r.Right, other.Right)
	bottom := math.Min(r.Bottom, other.Bottom)
	if left >= right || top >= bottom {
		return Rect{} // Empty
	}
	return Rect{Left: left, Top: top, Right: right, Bottom: bottom}
}

// IsEmpty returns true if the rectangle has zero or negative area.
func (r Rect) IsEmpty() bool {
	return r.Right <= r.Left || r.Bottom <= r.Top
}

// Translate returns a new rect offset by (dx, dy).
func (r Rect) Translate(dx, dy float64) Rect {
	return Rect{
		Left:   r.Left + dx,
		Top:    r.Top + dy,
		Right:  r.Right + dx,
		Bottom: r.Bottom + dy,
	}
}

// Intersects returns true if the two rectangles overlap.
// Returns false if either rectangle is empty.
func (r Rect) Intersects(other Rect) bool {
	return r.Left < other.Right && other.Left < r.Right &&
		r.Top < other.Bottom && other.Top < r.Bottom
}

// Union returns the smallest rect containing both r and other.
func (r Rect) Union(other Rect) Rect {
	return Rect{
		Left:   math.Min(r.Left, other.Left),
		Top:    math.Min(r.Top, other.Top),
		Right:  math.Max(r.Right, other.Right),
		Bottom: math.Max(r.Bottom, other.Bottom),
	}
}
