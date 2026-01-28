package widgets

// Overflow controls whether gradient decorations extend beyond widget bounds.
//
// This setting only affects gradient drawing in [Container] and [DecoratedBox].
// Shadows already overflow naturally and are not affected by this setting.
// Solid background colors never overflow regardless of this setting.
//
// For widgets with BorderRadius, [OverflowVisible] preserves rounded corners
// within bounds while allowing the gradient to extend beyond. [OverflowClip]
// clips the entire gradient to the widget shape (rounded if BorderRadius > 0).
type Overflow int

const (
	// OverflowVisible allows gradients to extend beyond widget bounds.
	// This is the default behavior and is useful for glow effects where a
	// radial gradient's radius exceeds the widget dimensions.
	//
	// When combined with BorderRadius > 0, the in-bounds area retains rounded
	// corners while the overflow area has squared corners.
	OverflowVisible Overflow = iota

	// OverflowClip confines gradients strictly to widget bounds.
	// Use this when the gradient should not visually extend beyond the widget,
	// or when performance is critical (avoids double-draw for rounded corners).
	//
	// When combined with BorderRadius > 0, clips to the rounded shape.
	OverflowClip
)
