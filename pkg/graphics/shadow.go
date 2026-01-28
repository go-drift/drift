package graphics

import "fmt"

// BlurStyle controls how the blur mask is generated.
type BlurStyle int

const (
	// BlurStyleNormal blurs inside and outside the shape.
	BlurStyleNormal BlurStyle = iota
	// BlurStyleSolid keeps the shape solid inside, blurs outside.
	BlurStyleSolid
	// BlurStyleOuter draws nothing inside, blurs outside only.
	BlurStyleOuter
	// BlurStyleInner blurs inside the shape only, nothing outside.
	BlurStyleInner
)

// String returns a human-readable representation of the blur style.
func (s BlurStyle) String() string {
	switch s {
	case BlurStyleNormal:
		return "normal"
	case BlurStyleSolid:
		return "solid"
	case BlurStyleOuter:
		return "outer"
	case BlurStyleInner:
		return "inner"
	default:
		return fmt.Sprintf("BlurStyle(%d)", int(s))
	}
}

// BoxShadow defines a shadow to draw behind a shape.
type BoxShadow struct {
	Color      Color
	Offset     Offset
	BlurRadius float64 // sigma = blurRadius * 0.5
	Spread     float64
	BlurStyle  BlurStyle
}

// Sigma returns the blur sigma for Skia's mask filter.
// Returns 0 if BlurRadius is negative.
func (s BoxShadow) Sigma() float64 {
	if s.BlurRadius <= 0 {
		return 0
	}
	return s.BlurRadius * 0.5
}

// NewBoxShadow creates a simple drop shadow with the given color and blur radius.
// Offset defaults to (0, 2) for a subtle downward shadow.
func NewBoxShadow(color Color, blurRadius float64) *BoxShadow {
	return &BoxShadow{
		Color:      color,
		Offset:     Offset{X: 0, Y: 2},
		BlurRadius: blurRadius,
	}
}

// BoxShadowElevation returns a Material-style elevation shadow.
// Level should be 1-5, where higher levels have larger blur and offset.
func BoxShadowElevation(level int, color Color) *BoxShadow {
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	// Material Design elevation values (approximate)
	offsets := []float64{1, 2, 4, 6, 8}
	blurs := []float64{3, 6, 10, 14, 18}
	spreads := []float64{0, 0, 1, 2, 3}

	return &BoxShadow{
		Color:      color,
		Offset:     Offset{X: 0, Y: offsets[level-1]},
		BlurRadius: blurs[level-1],
		Spread:     spreads[level-1],
		BlurStyle:  BlurStyleNormal,
	}
}

// TextShadow defines a shadow to draw behind text.
type TextShadow struct {
	Color      Color
	Offset     Offset
	BlurRadius float64 // sigma = blurRadius * 0.5, 0 = hard shadow
}

// Sigma returns the blur sigma for Skia's mask filter.
// Returns 0 if BlurRadius is zero or negative.
func (s TextShadow) Sigma() float64 {
	if s.BlurRadius <= 0 {
		return 0
	}
	return s.BlurRadius * 0.5
}

// NewTextShadow creates a simple text shadow with the given color and blur radius.
// Offset defaults to (0, 2) for a subtle downward shadow.
func NewTextShadow(color Color, blurRadius float64) *TextShadow {
	return &TextShadow{
		Color:      color,
		Offset:     Offset{X: 0, Y: 2},
		BlurRadius: blurRadius,
	}
}
