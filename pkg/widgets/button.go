package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/semantics"
)

// Button is a tappable button widget with customizable styling and haptic feedback.
//
// # Styling Model
//
// Button is explicit by default — all visual properties (Color, TextColor, Padding,
// FontSize, BorderRadius) use their struct field values directly. A zero value
// means zero, not "use theme default." For example:
//
//   - Color: 0 means transparent background
//   - Padding: zero EdgeInsets means no padding
//   - BorderRadius: 0 means sharp corners
//
// For theme-styled buttons, use [theme.ButtonOf] which pre-fills visual properties
// from the current theme's [theme.ButtonThemeData].
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Button{
//	    Label:        "Submit",
//	    OnTap:        handleSubmit,
//	    Color:        graphics.RGB(33, 150, 243),
//	    TextColor:    graphics.ColorWhite,
//	    Padding:      layout.EdgeInsetsSymmetric(24, 14),
//	    BorderRadius: 8,
//	    Haptic:       true,
//	}
//
// Themed (reads from current theme):
//
//	theme.ButtonOf(ctx, "Submit", handleSubmit)
//	// Pre-filled with theme colors, padding, font size, border radius
//
// Themed with overrides:
//
//	theme.ButtonOf(ctx, "Submit", handleSubmit).
//	    WithBorderRadius(0).  // explicit zero for sharp corners
//	    WithPadding(layout.EdgeInsetsAll(20))
//
// # Automatic Features
//
// The button automatically provides:
//   - Visual feedback on press (opacity change)
//   - Haptic feedback on tap (when Haptic is true)
//   - Accessibility support (label announced by screen readers)
//   - Disabled state handling (when Disabled is true)
type Button struct {
	core.StatelessBase

	// Label is the text displayed on the button.
	Label string

	// OnTap is called when the button is tapped. Ignored when Disabled is true.
	OnTap func()

	// Disabled prevents interaction and applies disabled styling when true.
	Disabled bool

	// Color is the background color. Zero means transparent.
	Color graphics.Color

	// Gradient is an optional background gradient that replaces Color when set.
	// Ignored when Disabled is true.
	Gradient *graphics.Gradient

	// TextColor is the label text color. Zero means transparent (invisible text).
	TextColor graphics.Color

	// FontSize is the label font size in logical pixels. Zero means no text rendered.
	FontSize float64

	// Padding is the space between the button edge and label.
	// Zero means no padding.
	Padding layout.EdgeInsets

	// BorderRadius is the corner radius in logical pixels.
	// Zero means sharp corners.
	BorderRadius float64

	// Haptic enables haptic feedback on tap when true.
	Haptic bool

	// DisabledColor is the background color when disabled.
	// If zero, falls back to 0.5 opacity on the normal Color.
	DisabledColor graphics.Color

	// DisabledTextColor is the text color when disabled.
	// If zero, falls back to 0.5 opacity on the normal TextColor.
	DisabledTextColor graphics.Color
}

// WithColor returns a copy of the button with the specified background and text colors.
func (b Button) WithColor(bg, text graphics.Color) Button {
	b.Color = bg
	b.TextColor = text
	return b
}

// WithGradient returns a copy of the button with the specified background gradient.
func (b Button) WithGradient(gradient *graphics.Gradient) Button {
	b.Gradient = gradient
	return b
}

// WithPadding returns a copy of the button with the specified padding.
func (b Button) WithPadding(padding layout.EdgeInsets) Button {
	b.Padding = padding
	return b
}

// WithFontSize returns a copy of the button with the specified label font size.
func (b Button) WithFontSize(size float64) Button {
	b.FontSize = size
	return b
}

// WithHaptic returns a copy of the button with haptic feedback enabled or disabled.
func (b Button) WithHaptic(enabled bool) Button {
	b.Haptic = enabled
	return b
}

// WithDisabled returns a copy of the button with the specified disabled state.
func (b Button) WithDisabled(disabled bool) Button {
	b.Disabled = disabled
	return b
}

// WithBorderRadius returns a copy of the button with the specified corner radius.
func (b Button) WithBorderRadius(radius float64) Button {
	b.BorderRadius = radius
	return b
}

func (b Button) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero
	color := b.Color
	textColor := b.TextColor
	padding := b.Padding
	fontSize := b.FontSize
	borderRadius := b.BorderRadius

	// Disabled state handling:
	// - If DisabledColor/DisabledTextColor are set: use those colors directly
	// - If neither is set: wrap the entire button in an Opacity widget (0.5 alpha)
	//
	// When useOpacityFallback is true, we keep the original colors unchanged here
	// and the Opacity wrapper (applied later) handles the visual fade effect.
	useOpacityFallback := false
	if b.Disabled {
		if b.DisabledColor != 0 || b.DisabledTextColor != 0 {
			// At least one disabled color set — use explicit disabled styling.
			// Apply 50% alpha to any color without an explicit disabled variant.
			if b.DisabledColor != 0 {
				color = b.DisabledColor
			} else {
				color = color.WithAlpha(0.5)
			}
			if b.DisabledTextColor != 0 {
				textColor = b.DisabledTextColor
			} else {
				textColor = textColor.WithAlpha(0.5)
			}
		} else {
			// No disabled colors set — use opacity fallback on the entire widget.
			useOpacityFallback = true
		}
	}

	var onTap func()
	if !b.Disabled {
		onTap = b.OnTap
		if b.Haptic && onTap != nil {
			originalOnTap := onTap
			onTap = func() {
				platform.Haptics.LightImpact()
				originalOnTap()
			}
		}
	}

	content := Padding{
		Padding: padding,
		Child: Text{
			Content: b.Label,
			Style:   graphics.TextStyle{Color: textColor, FontSize: fontSize},
		},
	}

	var box core.Widget
	if b.Gradient != nil {
		// Use gradient for normal and disabled states. When disabled with opacity
		// fallback, the gradient is preserved and the opacity wrapper handles the fade.
		box = DecoratedBox{
			Gradient:     b.Gradient,
			BorderRadius: borderRadius,
			Overflow:     OverflowClip,
			Child:        content,
		}
	} else {
		box = DecoratedBox{
			Color:        color,
			BorderRadius: borderRadius,
			Child:        content,
		}
	}

	// Fall back to opacity if no disabled colors provided
	if useOpacityFallback {
		box = Opacity{
			Opacity: 0.5,
			Child:   box,
		}
	}

	// Build accessibility flags
	var flags semantics.SemanticsFlag = semantics.SemanticsIsButton | semantics.SemanticsHasEnabledState
	if !b.Disabled {
		flags = flags.Set(semantics.SemanticsIsEnabled)
	}

	var hint string
	if !b.Disabled && onTap != nil {
		hint = "Double tap to activate"
	}

	return Semantics{
		// Note: Don't set Label here - it comes from merged descendant Text widgets
		Hint:             hint,
		Role:             semantics.SemanticsRoleButton,
		Flags:            flags,
		Container:        true,
		MergeDescendants: true, // Merge text into button node so TalkBack highlights the button, not the text
		OnTap:            onTap,
		Child: GestureDetector{
			OnTap: onTap,
			Child: box,
		},
	}
}
