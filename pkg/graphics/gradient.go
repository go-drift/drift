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
type GradientStop struct {
	Position float64
	Color    Color
}

// LinearGradient defines a gradient between two points.
type LinearGradient struct {
	Start Offset
	End   Offset
	Stops []GradientStop
}

// RadialGradient defines a gradient from a center point.
type RadialGradient struct {
	Center Offset
	Radius float64
	Stops  []GradientStop
}

// Gradient describes a linear or radial gradient.
type Gradient struct {
	Type   GradientType
	Linear LinearGradient
	Radial RadialGradient
}

// NewLinearGradient constructs a linear gradient definition.
func NewLinearGradient(start, end Offset, stops []GradientStop) *Gradient {
	return &Gradient{
		Type: GradientTypeLinear,
		Linear: LinearGradient{
			Start: start,
			End:   end,
			Stops: cloneGradientStops(stops),
		},
	}
}

// NewRadialGradient constructs a radial gradient definition.
func NewRadialGradient(center Offset, radius float64, stops []GradientStop) *Gradient {
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
// gradient center with sides equal to twice the radius.
//
// For linear gradients, the natural bounds span from the start to end points.
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
		c, r := g.Radial.Center, g.Radial.Radius
		if r <= 0 {
			return widgetRect
		}
		gradientRect = RectFromLTWH(c.X-r, c.Y-r, r*2, r*2)
	case GradientTypeLinear:
		s, e := g.Linear.Start, g.Linear.End
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
