package rendering

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
