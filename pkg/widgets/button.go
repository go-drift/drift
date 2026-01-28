package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// Button is a tappable button widget with theme-aware styling and haptic feedback.
//
// Button uses colors from the current [theme.ButtonTheme] by default. Override
// individual properties using the struct fields or fluent With* methods.
//
// Example using struct literal:
//
//	Button{
//	    Label:    "Submit",
//	    OnTap:    handleSubmit,
//	    Color:    colors.Primary,
//	    Disabled: !isValid,
//	}
//
// Example using XxxOf helper:
//
//	ButtonOf("Submit", handleSubmit).
//	    WithColor(colors.Primary, colors.OnPrimary).
//	    WithPadding(layout.EdgeInsetsSymmetric(32, 16)).
//	    WithDisabled(!isValid)
//
// The button automatically provides:
//   - Visual feedback on press (opacity change)
//   - Haptic feedback on tap (configurable via Haptic field)
//   - Accessibility support (label announced by screen readers)
//   - Disabled state styling
type Button struct {
	// Label is the text displayed on the button.
	Label string
	// OnTap is called when the button is tapped.
	OnTap func()
	// Disabled disables the button when true.
	Disabled bool
	// Color is the background color. Defaults to primary if zero.
	Color graphics.Color
	// Gradient is the optional background gradient.
	Gradient *graphics.Gradient
	// TextColor is the label color. Defaults to onPrimary if zero.
	TextColor graphics.Color
	// FontSize is the label font size. Defaults to 16 if zero.
	FontSize float64
	// Padding is the button padding. Defaults to symmetric(24, 14) if zero.
	Padding layout.EdgeInsets
	// BorderRadius is the corner radius. Defaults to 8 if zero.
	BorderRadius float64
	// Haptic enables haptic feedback on tap. Defaults to true.
	Haptic bool
}

// ButtonOf creates a button with the given label and tap handler.
// Haptic feedback is enabled by default for better touch response.
//
// This is a convenience helper equivalent to:
//
//	Button{Label: label, OnTap: onTap, Haptic: true}
func ButtonOf(label string, onTap func()) Button {
	return Button{
		Label:  label,
		OnTap:  onTap,
		Haptic: true,
	}
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

func (b Button) CreateElement() core.Element {
	return core.NewStatelessElement(b, nil)
}

func (b Button) Key() any {
	return nil
}

func (b Button) Build(ctx core.BuildContext) core.Widget {
	// Get button theme for defaults
	buttonTheme := theme.ThemeOf(ctx).ButtonThemeOf()

	// Apply defaults from theme
	color := b.Color
	if color == 0 {
		color = buttonTheme.BackgroundColor
	}
	textColor := b.TextColor
	if textColor == 0 {
		textColor = buttonTheme.ForegroundColor
	}
	padding := b.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = buttonTheme.Padding
	}
	fontSize := b.FontSize
	if fontSize == 0 {
		fontSize = buttonTheme.FontSize
	}
	borderRadius := b.BorderRadius
	if borderRadius == 0 {
		borderRadius = buttonTheme.BorderRadius
	}

	// Handle disabled state
	if b.Disabled {
		color = buttonTheme.DisabledBackgroundColor
		textColor = buttonTheme.DisabledForegroundColor
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
		ChildWidget: Text{
			Content: b.Label,
			Style:   graphics.TextStyle{Color: textColor, FontSize: fontSize},
		},
	}

	var box core.Widget
	if b.Gradient != nil && !b.Disabled {
		box = DecoratedBox{
			Gradient:     b.Gradient,
			BorderRadius: borderRadius,
			Overflow:     OverflowClip,
			ChildWidget:  content,
		}
	} else {
		box = DecoratedBox{
			Color:        color,
			BorderRadius: borderRadius,
			ChildWidget:  content,
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
		ChildWidget: GestureDetector{
			OnTap:       onTap,
			ChildWidget: box,
		},
	}
}
