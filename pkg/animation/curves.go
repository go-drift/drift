package animation

// Easing curves transform linear animation progress into natural-feeling motion.
//
// Each curve is a function that takes a value t in [0, 1] and returns a
// transformed value. Set an [AnimationController]'s Curve field to apply easing.
//
// Standard curves: [LinearCurve], [Ease], [EaseIn], [EaseOut], [EaseInOut].
// Use [CubicBezier] to create custom curves matching CSS cubic-bezier().
//
// See ExampleCubicBezier for custom curve usage.

// LinearCurve returns linear progress (no easing).
func LinearCurve(t float64) float64 {
	return t
}

// IOSNavigationCurve approximates iOS navigation transition easing.
var IOSNavigationCurve = CubicBezier(0.32, 0.72, 0.0, 1.0)

// Ease is a standard cubic bezier curve for general-purpose easing.
// Equivalent to CSS ease.
var Ease = CubicBezier(0.25, 0.1, 0.25, 1.0)

// EaseIn starts slowly and accelerates. Use for elements exiting the screen.
// Equivalent to CSS ease-in.
var EaseIn = CubicBezier(0.4, 0.0, 1.0, 1.0)

// EaseOut starts quickly and decelerates. Use for elements entering the screen.
// Equivalent to CSS ease-out.
var EaseOut = CubicBezier(0.0, 0.0, 0.2, 1.0)

// EaseInOut starts and ends slowly with acceleration in the middle.
// Use for elements that stay on screen but change state.
// Equivalent to CSS ease-in-out.
var EaseInOut = CubicBezier(0.4, 0.0, 0.2, 1.0)

// CubicBezier returns a cubic-bezier easing function matching CSS cubic-bezier().
// The parameters define the two control points (x1,y1) and (x2,y2) of the curve.
// The curve starts at (0,0) and ends at (1,1).
func CubicBezier(x1, y1, x2, y2 float64) func(float64) float64 {
	return func(t float64) float64 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}

		u := t
		for i := 0; i < 5; i++ {
			x := sampleCurve(x1, x2, u) - t
			dx := sampleCurveDerivative(x1, x2, u)
			if dx == 0 {
				break
			}
			u -= x / dx
			if u <= 0 || u >= 1 {
				break
			}
		}

		return sampleCurve(y1, y2, clampUnit(u))
	}
}

func sampleCurve(a, b, t float64) float64 {
	inv := 1 - t
	return 3*inv*inv*t*a + 3*inv*t*t*b + t*t*t
}

func sampleCurveDerivative(a, b, t float64) float64 {
	inv := 1 - t
	return 3*inv*inv*a + 6*inv*t*(b-a) + 3*t*t*(1-b)
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
