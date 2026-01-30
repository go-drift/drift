package graphics

import "math"

// Alignment represents a position within a rectangle using a coordinate system
// where the center is (0, 0), with X and Y values typically ranging from -1 to 1.
//
// # Coordinate System
//
// The coordinate system maps positions within a bounding rectangle:
//
//	(-1,-1)     (0,-1)      (1,-1)
//	   +-----------+-----------+
//	   |  TopLeft  | TopCenter |  TopRight
//	   |           |           |
//	(-1,0)      (0,0)       (1,0)
//	   +-----------+-----------+
//	   |CenterLeft |  Center   |  CenterRight
//	   |           |           |
//	(-1,1)      (0,1)       (1,1)
//	   +-----------+-----------+
//	   |BottomLeft |BottomCenter| BottomRight
//
// # Extending Beyond Bounds
//
// Values outside the -1 to 1 range extend beyond the widget boundaries:
//
//	Alignment{-2, 0}  // One full width to the left of the widget
//	Alignment{1.5, 0} // Halfway past the right edge
//
// This is useful for creating gradient overflow effects like glows when
// combined with Overflow: OverflowVisible.
//
// # Usage with Gradients
//
// Alignment is used for gradient positioning, allowing gradients to be defined
// relative to widget dimensions rather than absolute pixel coordinates:
//
//	// Horizontal gradient from left to right edge
//	graphics.NewLinearGradient(
//	    graphics.AlignCenterLeft,
//	    graphics.AlignCenterRight,
//	    stops,
//	)
//
//	// Diagonal gradient from top-left to bottom-right
//	graphics.NewLinearGradient(
//	    graphics.AlignTopLeft,
//	    graphics.AlignBottomRight,
//	    stops,
//	)
//
//	// Radial gradient centered in widget
//	graphics.NewRadialGradient(
//	    graphics.AlignCenter,
//	    1.0, // radius = 100% of half the min dimension
//	    stops,
//	)
type Alignment struct {
	// X is the horizontal position, where -1 is the left edge, 0 is the center,
	// and 1 is the right edge. Values outside this range extend beyond the bounds.
	X float64
	// Y is the vertical position, where -1 is the top edge, 0 is the center,
	// and 1 is the bottom edge. Values outside this range extend beyond the bounds.
	Y float64
}

// Named alignment constants for common positions.
var (
	// AlignTopLeft positions at the top-left corner (-1, -1).
	AlignTopLeft = Alignment{-1, -1}
	// AlignTopCenter positions at the top center (0, -1).
	AlignTopCenter = Alignment{0, -1}
	// AlignTopRight positions at the top-right corner (1, -1).
	AlignTopRight = Alignment{1, -1}
	// AlignCenterLeft positions at the center-left edge (-1, 0).
	AlignCenterLeft = Alignment{-1, 0}
	// AlignCenter positions at the center (0, 0).
	AlignCenter = Alignment{0, 0}
	// AlignCenterRight positions at the center-right edge (1, 0).
	AlignCenterRight = Alignment{1, 0}
	// AlignBottomLeft positions at the bottom-left corner (-1, 1).
	AlignBottomLeft = Alignment{-1, 1}
	// AlignBottomCenter positions at the bottom center (0, 1).
	AlignBottomCenter = Alignment{0, 1}
	// AlignBottomRight positions at the bottom-right corner (1, 1).
	AlignBottomRight = Alignment{1, 1}
)

// Resolve converts the relative alignment to absolute pixel coordinates
// within the given bounding rectangle.
//
// The conversion uses center-relative coordinates:
//
//	X = centerX + alignment.X * (width / 2)
//	Y = centerY + alignment.Y * (height / 2)
//
// Example:
//
//	bounds := graphics.RectFromLTWH(0, 0, 200, 100)
//	pos := graphics.AlignCenterRight.Resolve(bounds)
//	// pos = Offset{X: 200, Y: 50}
func (a Alignment) Resolve(bounds Rect) Offset {
	cx := bounds.Left + bounds.Width()/2
	cy := bounds.Top + bounds.Height()/2
	return Offset{
		X: cx + a.X*bounds.Width()/2,
		Y: cy + a.Y*bounds.Height()/2,
	}
}

// ResolveRadius converts a relative radius value to absolute pixels.
// The radius is relative to half the minimum dimension of the bounds,
// so a radius of 1.0 will touch the nearest edge from the center.
//
// Example:
//
//	bounds := graphics.RectFromLTWH(0, 0, 200, 100)
//	r := graphics.ResolveRadius(1.0, bounds)
//	// r = 50 (half of the min dimension 100)
func ResolveRadius(relativeRadius float64, bounds Rect) float64 {
	minDim := math.Min(bounds.Width(), bounds.Height())
	return relativeRadius * minDim / 2
}
