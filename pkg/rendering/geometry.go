package rendering

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
