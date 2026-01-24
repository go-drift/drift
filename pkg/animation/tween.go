package animation

import (
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Tween interpolates between begin and end values.
type Tween[T any] struct {
	// Begin is the starting value of the interpolation.
	Begin T
	// End is the ending value of the interpolation.
	End T
	// Lerp is the interpolation function. It receives the begin value, end value,
	// and a progress value t in the range [0, 1].
	Lerp func(a, b T, t float64) T
}

// Evaluate returns the interpolated value at t (0.0 to 1.0).
func (tw *Tween[T]) Evaluate(t float64) T {
	if tw.Lerp == nil {
		return tw.End
	}
	return tw.Lerp(tw.Begin, tw.End, t)
}

// Transform returns the interpolated value using the controller's current value.
func (tw *Tween[T]) Transform(controller *AnimationController) T {
	return tw.Evaluate(controller.Value)
}

// LerpFloat64 linearly interpolates between two float64 values.
func LerpFloat64(a, b float64, t float64) float64 {
	return a + (b-a)*t
}

// LerpOffset linearly interpolates between two Offset values.
func LerpOffset(a, b rendering.Offset, t float64) rendering.Offset {
	return rendering.Offset{
		X: LerpFloat64(a.X, b.X, t),
		Y: LerpFloat64(a.Y, b.Y, t),
	}
}

// LerpColor linearly interpolates between two Color values.
func LerpColor(a, b rendering.Color, t float64) rendering.Color {
	aR := float64((a >> 16) & 0xFF)
	aG := float64((a >> 8) & 0xFF)
	aB := float64(a & 0xFF)
	aA := float64((a >> 24) & 0xFF)

	bR := float64((b >> 16) & 0xFF)
	bG := float64((b >> 8) & 0xFF)
	bB := float64(b & 0xFF)
	bA := float64((b >> 24) & 0xFF)

	r := uint8(LerpFloat64(aR, bR, t))
	g := uint8(LerpFloat64(aG, bG, t))
	b8 := uint8(LerpFloat64(aB, bB, t))
	alpha := uint8(LerpFloat64(aA, bA, t))

	return rendering.Color(uint32(alpha)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b8))
}

// TweenFloat64 creates a tween for float64 values.
func TweenFloat64(begin, end float64) *Tween[float64] {
	return &Tween[float64]{
		Begin: begin,
		End:   end,
		Lerp:  LerpFloat64,
	}
}

// TweenOffset creates a tween for Offset values.
func TweenOffset(begin, end rendering.Offset) *Tween[rendering.Offset] {
	return &Tween[rendering.Offset]{
		Begin: begin,
		End:   end,
		Lerp:  LerpOffset,
	}
}

// TweenColor creates a tween for Color values.
func TweenColor(begin, end rendering.Color) *Tween[rendering.Color] {
	return &Tween[rendering.Color]{
		Begin: begin,
		End:   end,
		Lerp:  LerpColor,
	}
}

// LerpEdgeInsets linearly interpolates between two EdgeInsets values.
func LerpEdgeInsets(a, b layout.EdgeInsets, t float64) layout.EdgeInsets {
	return layout.EdgeInsets{
		Left:   LerpFloat64(a.Left, b.Left, t),
		Top:    LerpFloat64(a.Top, b.Top, t),
		Right:  LerpFloat64(a.Right, b.Right, t),
		Bottom: LerpFloat64(a.Bottom, b.Bottom, t),
	}
}

// LerpRadius linearly interpolates between two Radius values.
func LerpRadius(a, b rendering.Radius, t float64) rendering.Radius {
	return rendering.Radius{
		X: LerpFloat64(a.X, b.X, t),
		Y: LerpFloat64(a.Y, b.Y, t),
	}
}

// LerpAlignment linearly interpolates between two Alignment values.
func LerpAlignment(a, b layout.Alignment, t float64) layout.Alignment {
	return layout.Alignment{
		X: LerpFloat64(a.X, b.X, t),
		Y: LerpFloat64(a.Y, b.Y, t),
	}
}

// TweenEdgeInsets creates a tween for EdgeInsets values.
func TweenEdgeInsets(begin, end layout.EdgeInsets) *Tween[layout.EdgeInsets] {
	return &Tween[layout.EdgeInsets]{
		Begin: begin,
		End:   end,
		Lerp:  LerpEdgeInsets,
	}
}

// TweenRadius creates a tween for Radius values.
func TweenRadius(begin, end rendering.Radius) *Tween[rendering.Radius] {
	return &Tween[rendering.Radius]{
		Begin: begin,
		End:   end,
		Lerp:  LerpRadius,
	}
}

// TweenAlignment creates a tween for Alignment values.
func TweenAlignment(begin, end layout.Alignment) *Tween[layout.Alignment] {
	return &Tween[layout.Alignment]{
		Begin: begin,
		End:   end,
		Lerp:  LerpAlignment,
	}
}
