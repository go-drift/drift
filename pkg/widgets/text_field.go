package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// TextField wraps [TextInput] and adds support for labels, helper text, and
// error display.
//
// # Styling Model
//
// TextField is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - BackgroundColor: 0 means transparent
//   - BorderColor: 0 means no border
//   - Height: 0 means zero height (invisible)
//   - Style.FontSize: 0 means no text rendered
//
// For theme-styled text fields, use [theme.TextFieldOf] which pre-fills visual
// properties from the current theme's [theme.TextFieldThemeData].
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.TextField{
//	    Controller:       controller,
//	    Label:            "Email",
//	    Placeholder:      "you@example.com",
//	    BackgroundColor:  graphics.ColorWhite,
//	    BorderColor:      graphics.RGB(200, 200, 200),
//	    FocusColor:       graphics.RGB(0, 122, 255),
//	    Height:           48,
//	    Padding:          layout.EdgeInsetsSymmetric(12, 10),
//	    BorderWidth:      1,
//	    Style:            graphics.TextStyle{FontSize: 16, Color: graphics.ColorBlack},
//	    PlaceholderColor: graphics.RGB(150, 150, 150),
//	}
//
// Themed (reads from current theme):
//
//	theme.TextFieldOf(ctx, controller).
//	    WithLabel("Email").
//	    WithPlaceholder("you@example.com")
//
// For form validation support, use [TextFormField] instead, which wraps TextField
// and integrates with [Form] for validation, save, and reset operations.
type TextField struct {
	core.StatelessBase

	// Controller manages the text content and selection.
	Controller *platform.TextEditingController
	// Label is shown above the field.
	Label string
	// Placeholder text shown when empty.
	Placeholder string
	// HelperText is shown below the field when no error.
	HelperText string
	// ErrorText is shown below the field when non-empty.
	ErrorText string
	// KeyboardType specifies the keyboard to show.
	KeyboardType platform.KeyboardType
	// InputAction specifies the keyboard action button.
	InputAction platform.TextInputAction
	// Obscure hides the text (for passwords).
	Obscure bool
	// Autocorrect enables auto-correction.
	Autocorrect bool
	// OnChanged is called when the text changes.
	OnChanged func(string)
	// OnSubmitted is called when the user submits.
	OnSubmitted func(string)
	// OnEditingComplete is called when editing is complete.
	OnEditingComplete func()
	// Disabled controls whether the field rejects input.
	Disabled bool
	// Width of the text field. Zero expands to fill available width.
	Width float64
	// Height of the text field. Zero means zero height (invisible).
	Height float64
	// Padding inside the text field. Zero means no padding.
	Padding layout.EdgeInsets
	// BackgroundColor of the text field. Zero means transparent.
	BackgroundColor graphics.Color
	// BorderColor of the text field. Zero means no border.
	BorderColor graphics.Color
	// FocusColor of the text field outline when focused. Zero means no focus highlight.
	FocusColor graphics.Color
	// BorderRadius for rounded corners. Zero means sharp corners.
	BorderRadius float64
	// BorderWidth for the border stroke. Zero means no border.
	BorderWidth float64
	// Style for the text content. Zero FontSize means no text rendered.
	Style graphics.TextStyle
	// PlaceholderColor for the placeholder text. Zero means transparent.
	PlaceholderColor graphics.Color
	// LabelStyle for the label text above the field.
	LabelStyle graphics.TextStyle
	// HelperStyle for helper/error text below the field.
	HelperStyle graphics.TextStyle
	// ErrorColor for error text and border when ErrorText is set. Zero means no error styling.
	ErrorColor graphics.Color

	// Input is an optional escape hatch for accessing TextInput fields not
	// exposed by TextField. TextField's own fields ALWAYS overwrite the
	// corresponding Input fields (even with zero values), so Input is only
	// useful for fields that TextField does not expose (e.g., future fields).
	// To set Controller, Placeholder, etc., use TextField's fields directly.
	Input *TextInput
}

// WithBackgroundColor returns a copy with the specified background color.
func (t TextField) WithBackgroundColor(c graphics.Color) TextField {
	t.BackgroundColor = c
	return t
}

// WithBorderColor returns a copy with the specified border color.
func (t TextField) WithBorderColor(c graphics.Color) TextField {
	t.BorderColor = c
	return t
}

// WithFocusColor returns a copy with the specified focus outline color.
func (t TextField) WithFocusColor(c graphics.Color) TextField {
	t.FocusColor = c
	return t
}

// WithPlaceholderColor returns a copy with the specified placeholder text color.
func (t TextField) WithPlaceholderColor(c graphics.Color) TextField {
	t.PlaceholderColor = c
	return t
}

// WithBorderRadius returns a copy with the specified corner radius.
func (t TextField) WithBorderRadius(radius float64) TextField {
	t.BorderRadius = radius
	return t
}

// WithHeight returns a copy with the specified height.
func (t TextField) WithHeight(height float64) TextField {
	t.Height = height
	return t
}

// WithPadding returns a copy with the specified internal padding.
func (t TextField) WithPadding(padding layout.EdgeInsets) TextField {
	t.Padding = padding
	return t
}

// WithLabel returns a copy with the specified label text.
func (t TextField) WithLabel(label string) TextField {
	t.Label = label
	return t
}

// WithPlaceholder returns a copy with the specified placeholder text.
func (t TextField) WithPlaceholder(placeholder string) TextField {
	t.Placeholder = placeholder
	return t
}

// WithHelperText returns a copy with the specified helper text.
func (t TextField) WithHelperText(helper string) TextField {
	t.HelperText = helper
	return t
}

// WithBorderWidth returns a copy with the specified border stroke width.
func (t TextField) WithBorderWidth(width float64) TextField {
	t.BorderWidth = width
	return t
}

func (t TextField) Build(ctx core.BuildContext) core.Widget {
	// Fully explicit: zero means zero. Callers (or theme.TextFieldOf) must
	// provide all visual properties.
	borderColor := t.BorderColor
	focusColor := t.FocusColor

	// When ErrorText is set, use error color for BOTH border and focus
	if t.ErrorText != "" && t.ErrorColor != 0 {
		borderColor = t.ErrorColor
		focusColor = t.ErrorColor
	}

	children := make([]core.Widget, 0, 4)
	if t.Label != "" {
		children = append(children, Text{Content: t.Label, Style: t.LabelStyle})
		children = append(children, VSpace(6))
	}

	// Build TextInput with all visual properties passed through directly.
	var input TextInput
	if t.Input != nil {
		input = *t.Input
	}

	input.Controller = t.Controller
	input.Placeholder = t.Placeholder
	input.KeyboardType = t.KeyboardType
	input.InputAction = t.InputAction
	input.Obscure = t.Obscure
	input.Autocorrect = t.Autocorrect
	input.OnChanged = t.OnChanged
	input.OnSubmitted = t.OnSubmitted
	input.OnEditingComplete = t.OnEditingComplete
	input.Disabled = t.Disabled
	input.Width = t.Width
	input.Height = t.Height
	input.Padding = t.Padding
	input.BackgroundColor = t.BackgroundColor
	input.BorderColor = borderColor
	input.FocusColor = focusColor
	input.BorderRadius = t.BorderRadius
	input.BorderWidth = t.BorderWidth
	input.Style = t.Style
	input.PlaceholderColor = t.PlaceholderColor

	children = append(children, input)

	if t.ErrorText != "" {
		errorStyle := t.HelperStyle
		if t.ErrorColor != 0 {
			errorStyle.Color = t.ErrorColor
		}
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.ErrorText, Style: errorStyle})
	} else if t.HelperText != "" {
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.HelperText, Style: t.HelperStyle})
	}

	return Column{
		MainAxisSize: MainAxisSizeMin,
		Children:     children,
	}
}
