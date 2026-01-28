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

func buildGradientPayload(gradient *Gradient) (gradientPayload, bool) {
	if gradient == nil || !gradient.IsValid() {
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
		payload.start = gradient.Linear.Start
		payload.end = gradient.Linear.End
	case GradientTypeRadial:
		payload.center = gradient.Radial.Center
		payload.radius = gradient.Radial.Radius
	default:
		return gradientPayload{}, false
	}
	return payload, true
}
