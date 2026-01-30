package graphics

type gradientPayload struct {
	gradientType int32
	colors       []uint32
	positions    []float32
	start        Offset
	end          Offset
	center       Offset
	radius       float64
}

// buildGradientPayload converts a Gradient with relative Alignment coordinates
// to absolute pixel coordinates within the given bounds.
//
// The bounds parameter specifies the rectangle in which the gradient will be
// rendered. Alignment values are resolved to pixel coordinates relative to
// this bounds rectangle.
//
// Returns false if the gradient is invalid, bounds are completely empty (both
// dimensions zero), or the resolved radial gradient radius is zero or negative.
func buildGradientPayload(gradient *Gradient, bounds Rect) (gradientPayload, bool) {
	if gradient == nil || !gradient.IsValid() {
		return gradientPayload{}, false
	}
	// Guard against completely empty bounds (both dimensions zero)
	// Linear gradients can work with zero in one dimension (e.g., horizontal/vertical lines)
	if bounds.Width() <= 0 && bounds.Height() <= 0 {
		return gradientPayload{}, false
	}
	stops := gradient.Stops()
	colors := make([]uint32, len(stops))
	positions := make([]float32, len(stops))
	for i, stop := range stops {
		colors[i] = uint32(stop.Color)
		positions[i] = float32(stop.Position)
	}
	payload := gradientPayload{
		gradientType: int32(gradient.Type),
		colors:       colors,
		positions:    positions,
	}
	switch gradient.Type {
	case GradientTypeLinear:
		payload.start = gradient.Linear.Start.Resolve(bounds)
		payload.end = gradient.Linear.End.Resolve(bounds)
	case GradientTypeRadial:
		payload.center = gradient.Radial.Center.Resolve(bounds)
		payload.radius = ResolveRadius(gradient.Radial.Radius, bounds)
		// Guard against zero/negative resolved radius (radial needs non-zero min dimension)
		if payload.radius <= 0 {
			return gradientPayload{}, false
		}
	default:
		return gradientPayload{}, false
	}
	return payload, true
}
