package widgets

// Overflow controls clipping behavior for [Container] and [DecoratedBox].
//
// This setting affects two things:
//   - Whether background gradients extend beyond widget bounds
//   - Whether children are clipped to widget bounds
//
// Shadows are always drawn behind the decoration and naturally overflow bounds
// regardless of this setting. Solid background colors never overflow.
//
// [OverflowClip] (the default) clips both gradients and children to the widget
// bounds. When BorderRadius > 0, content is clipped to the rounded shape.
// [OverflowVisible] allows gradients to extend beyond and does not clip children.
type Overflow int

const (
	// OverflowClip confines content strictly to widget bounds.
	// This is the default behavior.
	//
	// Both background gradients and children are clipped to the widget bounds.
	// When BorderRadius > 0, content is clipped to the rounded shape.
	//
	// This ensures that child content (such as images or colored bars at the
	// edges of a card) conforms to the parent's bounds and rounded corners.
	OverflowClip Overflow = iota

	// OverflowVisible allows content to extend beyond widget bounds.
	// Children are not clipped.
	//
	// This is useful for glow effects where a radial gradient's radius
	// exceeds the widget dimensions, or when children need to paint
	// outside the parent bounds (e.g., shadows on child widgets).
	OverflowVisible
)
