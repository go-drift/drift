package graphics

import (
	"fmt"
	"math"
)

// GradientType describes the gradient variant.
type GradientType int

const (
	// GradientTypeNone indicates no gradient is applied.
	GradientTypeNone GradientType = iota
	// GradientTypeLinear indicates a linear gradient.
	GradientTypeLinear
	// GradientTypeRadial indicates a radial gradient.
	GradientTypeRadial
)

// String returns a human-readable representation of the gradient type.
func (t GradientType) String() string {
	switch t {
	case GradientTypeNone:
		return "none"
	case GradientTypeLinear:
		return "linear"
	case GradientTypeRadial:
		return "radial"
	default:
		return fmt.Sprintf("GradientType(%d)", int(t))
	}
}

// GradientStop defines a color stop within a gradient.
//
// Each stop specifies a color at a position along the gradient. Colors are
// interpolated between stops to create smooth transitions.
//
// Example:
//
//	[]graphics.GradientStop{
//	    {Position: 0.0, Color: graphics.RGB(255, 0, 0)},   // Red at start
//	    {Position: 0.5, Color: graphics.RGB(255, 255, 0)}, // Yellow at midpoint
//	    {Position: 1.0, Color: graphics.RGB(0, 255, 0)},   // Green at end
//	}
type GradientStop struct {
	// Position is where this color appears along the gradient, from 0.0 (start)
	// to 1.0 (end). For linear gradients, 0.0 is the Start point and 1.0 is
	// the End point. For radial gradients, 0.0 is the center and 1.0 is at
	// the radius edge.
	Position float64

	// Color is the color at this position.
	Color Color
}

// LinearGradient defines a gradient between two points using relative coordinates.
//
// Start and End use the Alignment coordinate system where (-1, -1) is the
// top-left corner, (0, 0) is the center, and (1, 1) is the bottom-right corner.
// This allows gradients to scale with widget dimensions.
//
// Common gradient directions:
//
//	Horizontal: AlignCenterLeft to AlignCenterRight
//	Vertical:   AlignTopCenter to AlignBottomCenter
//	Diagonal:   AlignTopLeft to AlignBottomRight
type LinearGradient struct {
	// Start is the starting point of the gradient in relative coordinates.
	Start Alignment
	// End is the ending point of the gradient in relative coordinates.
	End Alignment
	// Stops defines the color stops along the gradient.
	Stops []GradientStop
}

// RadialGradient defines a gradient radiating from a center point.
//
// Center uses the Alignment coordinate system where (-1, -1) is the
// top-left corner, (0, 0) is the center, and (1, 1) is the bottom-right corner.
//
// Radius is relative to half the minimum dimension of the bounding rectangle,
// so a radius of 1.0 will touch the nearest edge from the center.
type RadialGradient struct {
	// Center is the center point of the gradient in relative coordinates.
	Center Alignment
	// Radius is the relative radius (1.0 = half the min dimension).
	Radius float64
	// Stops defines the color stops along the gradient.
	Stops []GradientStop
}

// Gradient describes a linear or radial gradient.
//
// Use [NewLinearGradient] or [NewRadialGradient] to construct gradients.
type Gradient struct {
	// Type indicates whether this is a linear or radial gradient.
	Type GradientType
	// Linear contains the linear gradient configuration when Type is GradientTypeLinear.
	Linear LinearGradient
	// Radial contains the radial gradient configuration when Type is GradientTypeRadial.
	Radial RadialGradient
}

// NewLinearGradient constructs a linear gradient definition using relative coordinates.
//
// The start and end points use the Alignment coordinate system where
// (-1, -1) is the top-left corner, (0, 0) is the center, and (1, 1) is
// the bottom-right corner.
//
// Example:
//
//	// Horizontal gradient from left to right
//	graphics.NewLinearGradient(
//	    graphics.AlignCenterLeft,
//	    graphics.AlignCenterRight,
//	    []graphics.GradientStop{
//	        {Position: 0, Color: startColor},
//	        {Position: 1, Color: endColor},
//	    },
//	)
func NewLinearGradient(start, end Alignment, stops []GradientStop) *Gradient {
	return &Gradient{
		Type: GradientTypeLinear,
		Linear: LinearGradient{
			Start: start,
			End:   end,
			Stops: cloneGradientStops(stops),
		},
	}
}

// NewRadialGradient constructs a radial gradient definition using relative coordinates.
//
// The center point uses the Alignment coordinate system where
// (-1, -1) is the top-left corner, (0, 0) is the center, and (1, 1) is
// the bottom-right corner.
//
// The radius is relative to half the minimum dimension of the bounding rectangle,
// so a radius of 1.0 will touch the nearest edge from the center, and 2.0 will
// extend to the farthest corner of a square widget.
//
// Example:
//
//	// Radial gradient centered in widget, filling to edges
//	graphics.NewRadialGradient(
//	    graphics.AlignCenter,
//	    1.0, // touches nearest edge
//	    []graphics.GradientStop{
//	        {Position: 0, Color: centerColor},
//	        {Position: 1, Color: edgeColor},
//	    },
//	)
func NewRadialGradient(center Alignment, radius float64, stops []GradientStop) *Gradient {
	return &Gradient{
		Type: GradientTypeRadial,
		Radial: RadialGradient{
			Center: center,
			Radius: radius,
			Stops:  cloneGradientStops(stops),
		},
	}
}

// Stops returns the gradient stops for the configured type.
func (g *Gradient) Stops() []GradientStop {
	if g == nil {
		return nil
	}
	switch g.Type {
	case GradientTypeLinear:
		return g.Linear.Stops
	case GradientTypeRadial:
		return g.Radial.Stops
	default:
		return nil
	}
}

// IsValid reports whether the gradient has usable stops.
func (g *Gradient) IsValid() bool {
	if g == nil {
		return false
	}
	stops := g.Stops()
	if len(stops) < 2 {
		return false
	}
	if g.Type == GradientTypeRadial && g.Radial.Radius <= 0 {
		return false
	}
	for _, stop := range stops {
		if stop.Position < 0 || stop.Position > 1 {
			return false
		}
	}
	return g.Type == GradientTypeLinear || g.Type == GradientTypeRadial
}

func cloneGradientStops(stops []GradientStop) []GradientStop {
	if len(stops) == 0 {
		return nil
	}
	clone := make([]GradientStop, len(stops))
	copy(clone, stops)
	return clone
}

// Bounds returns the rectangle needed to fully render the gradient,
// expanded from widgetRect as needed. The result is the union of widgetRect
// and the gradient's natural bounds, ensuring it never shrinks widgetRect.
//
// For radial gradients, the natural bounds are a square centered on the
// resolved gradient center with sides equal to twice the resolved radius.
//
// For linear gradients, the natural bounds span from the resolved start
// to end points.
//
// This method is used by widgets with overflow visible to determine the
// drawing area for gradient overflow effects like glows.
func (g *Gradient) Bounds(widgetRect Rect) Rect {
	if g == nil || !g.IsValid() {
		return widgetRect
	}
	var gradientRect Rect
	switch g.Type {
	case GradientTypeRadial:
		c := g.Radial.Center.Resolve(widgetRect)
		r := ResolveRadius(g.Radial.Radius, widgetRect)
		if r <= 0 {
			return widgetRect
		}
		gradientRect = RectFromLTWH(c.X-r, c.Y-r, r*2, r*2)
	case GradientTypeLinear:
		s := g.Linear.Start.Resolve(widgetRect)
		e := g.Linear.End.Resolve(widgetRect)
		gradientRect = Rect{
			Left:   math.Min(s.X, e.X),
			Top:    math.Min(s.Y, e.Y),
			Right:  math.Max(s.X, e.X),
			Bottom: math.Max(s.Y, e.Y),
		}
	default:
		return widgetRect
	}
	return widgetRect.Union(gradientRect)
}
