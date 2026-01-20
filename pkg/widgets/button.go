package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
)

// Button is a tappable button widget with customizable appearance.
type Button struct {
	// Label is the text displayed on the button.
	Label string
	// OnTap is called when the button is tapped.
	OnTap func()
	// Color is the background color. Defaults to primary if zero.
	Color rendering.Color
	// Gradient is the optional background gradient.
	Gradient *rendering.Gradient
	// TextColor is the label color. Defaults to onPrimary if zero.
	TextColor rendering.Color
	// FontSize is the label font size. Defaults to 16 if zero.
	FontSize float64
	// Padding is the button padding. Defaults to symmetric(24, 14) if zero.
	Padding layout.EdgeInsets
	// Haptic enables haptic feedback on tap. Defaults to true.
	Haptic bool
}

// NewButton creates a button with the given label and tap handler.
// Uses sensible defaults for styling.
func NewButton(label string, onTap func()) Button {
	return Button{
		Label:  label,
		OnTap:  onTap,
		Haptic: true,
	}
}

// WithColor sets the background and text colors.
func (b Button) WithColor(bg, text rendering.Color) Button {
	b.Color = bg
	b.TextColor = text
	return b
}

// WithGradient sets the background gradient.
func (b Button) WithGradient(gradient *rendering.Gradient) Button {
	b.Gradient = gradient
	return b
}

// WithPadding sets the button padding.
func (b Button) WithPadding(padding layout.EdgeInsets) Button {
	b.Padding = padding
	return b
}

// WithFontSize sets the label font size.
func (b Button) WithFontSize(size float64) Button {
	b.FontSize = size
	return b
}

// WithHaptic enables or disables haptic feedback.
func (b Button) WithHaptic(enabled bool) Button {
	b.Haptic = enabled
	return b
}

func (b Button) CreateElement() core.Element {
	return core.NewStatelessElement(b, nil)
}

func (b Button) Key() any {
	return nil
}

func (b Button) Build(ctx core.BuildContext) core.Widget {
	// Apply defaults
	padding := b.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = layout.EdgeInsetsSymmetric(24, 14)
	}
	fontSize := b.FontSize
	if fontSize == 0 {
		fontSize = 16
	}

	onTap := b.OnTap
	if b.Haptic && onTap != nil {
		originalOnTap := onTap
		onTap = func() {
			platform.Haptics.LightImpact()
			originalOnTap()
		}
	}

	return GestureDetector{
		OnTap: onTap,
		ChildWidget: Container{
			Color:    b.Color,
			Gradient: b.Gradient,
			Padding:  padding,
			ChildWidget: Text{
				Content: b.Label,
				Style:   rendering.TextStyle{Color: b.TextColor, FontSize: fontSize},
			},
		},
	}
}
