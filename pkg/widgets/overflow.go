package widgets

// Overflow controls whether gradient decorations extend beyond widget bounds.
//
// This setting only affects gradient drawing in [Container] and [DecoratedBox].
// Shadows already overflow naturally and are not affected by this setting.
// Solid background colors never overflow regardless of this setting.
//
// For widgets with BorderRadius, [OverflowClip] (the default) clips the gradient
// to the rounded shape. [OverflowVisible] allows the gradient to extend beyond
// while preserving rounded corners within bounds.
type Overflow int

const (
	// OverflowClip confines gradients strictly to widget bounds.
	// This is the default behavior and ensures gradients respect BorderRadius.
	//
	// When combined with BorderRadius > 0, clips to the rounded shape.
	OverflowClip Overflow = iota

	// OverflowVisible allows gradients to extend beyond widget bounds.
	// This is useful for glow effects where a radial gradient's radius
	// exceeds the widget dimensions.
	//
	// When combined with BorderRadius > 0, the in-bounds area retains rounded
	// corners while the overflow area has squared corners.
	OverflowVisible
)
