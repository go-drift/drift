package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
)

// TextField is a Material Design styled text input that wraps [TextInput] and adds
// support for labels, helper text, and error display.
//
// TextField applies theme-based styling automatically, including colors, borders,
// border radius, padding, and typography from the current [theme.TextFieldTheme].
// When ErrorText is set, the border color changes to the theme's error color.
//
// For form validation support, use [TextFormField] instead, which wraps TextField
// and integrates with [Form] for validation, save, and reset operations.
//
// The Input field provides an escape hatch for accessing [TextInput] fields not
// directly exposed by TextField. However, TextField's own fields always take
// precedence over Input fields, so use TextField's fields for any shared options.
//
// Example:
//
//	controller := platform.NewTextEditingController("")
//	TextField{
//	    Controller:  controller,
//	    Label:       "Email",
//	    Placeholder: "you@example.com",
//	    HelperText:  "We'll never share your email",
//	    KeyboardType: platform.KeyboardTypeEmail,
//	}
type TextField struct {
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
	// Width of the text field (0 = expand to fill).
	Width float64
	// Height of the text field.
	Height float64
	// Padding inside the text field.
	Padding layout.EdgeInsets
	// BackgroundColor of the text field.
	BackgroundColor graphics.Color
	// BorderColor of the text field.
	BorderColor graphics.Color
	// FocusColor of the text field outline.
	FocusColor graphics.Color
	// BorderRadius for rounded corners.
	BorderRadius float64
	// Style for the text.
	Style graphics.TextStyle
	// PlaceholderColor for the placeholder text.
	PlaceholderColor graphics.Color

	// Input is an optional escape hatch for accessing TextInput fields not
	// exposed by TextField. TextField's own fields ALWAYS overwrite the
	// corresponding Input fields (even with zero values), so Input is only
	// useful for fields that TextField does not expose (e.g., future fields).
	// To set Controller, Placeholder, etc., use TextField's fields directly.
	Input *TextInput
}

func (t TextField) CreateElement() core.Element {
	return core.NewStatelessElement(t, nil)
}

func (t TextField) Key() any {
	return nil
}

func (t TextField) Build(ctx core.BuildContext) core.Widget {
	themeData, _, textTheme := theme.UseTheme(ctx)
	textFieldTheme := themeData.TextFieldThemeOf()

	labelStyle := textTheme.LabelLarge
	labelStyle.Color = textFieldTheme.LabelColor
	helperStyle := textTheme.BodySmall
	helperStyle.Color = textFieldTheme.LabelColor

	textStyle := t.Style
	if textStyle.FontSize == 0 {
		textStyle = textTheme.BodyLarge
	}
	if textStyle.Color == 0 {
		textStyle.Color = textFieldTheme.TextColor
	}

	backgroundColor := t.BackgroundColor
	if backgroundColor == 0 {
		backgroundColor = textFieldTheme.BackgroundColor
	}
	borderColor := t.BorderColor
	if borderColor == 0 {
		borderColor = textFieldTheme.BorderColor
	}
	focusColor := t.FocusColor
	if focusColor == 0 {
		focusColor = textFieldTheme.FocusColor
	}
	borderRadius := t.BorderRadius
	if borderRadius == 0 {
		borderRadius = textFieldTheme.BorderRadius
	}

	// When ErrorText is set, use error color for BOTH border and focus
	// This ensures error styling remains visible even when focused
	if t.ErrorText != "" {
		borderColor = textFieldTheme.ErrorColor
		focusColor = textFieldTheme.ErrorColor
	}

	height := t.Height
	if height == 0 {
		height = textFieldTheme.Height
	}

	padding := t.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = textFieldTheme.Padding
	}

	children := make([]core.Widget, 0, 4)
	if t.Label != "" {
		children = append(children, Text{Content: t.Label, Style: labelStyle})
		children = append(children, VSpace(6))
	}

	placeholderColor := t.PlaceholderColor
	if placeholderColor == 0 {
		placeholderColor = textFieldTheme.PlaceholderColor
	}

	// Build TextInput - either from Input escape hatch or from scratch.
	// When Input is provided, it supplies defaults for fields not exposed by TextField
	// (e.g., future fields like ReadOnly). TextField's own fields always take precedence.
	var input TextInput
	if t.Input != nil {
		// Copy Input as base for fields TextField doesn't directly expose
		input = *t.Input
	}

	// Always apply TextField's fields - they take precedence over Input.
	// This ensures predictable behavior: TextField fields are authoritative.
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
	input.Height = height
	input.Padding = padding
	input.BackgroundColor = backgroundColor
	input.BorderColor = borderColor
	input.FocusColor = focusColor
	input.BorderRadius = borderRadius
	input.Style = textStyle
	input.PlaceholderColor = placeholderColor

	children = append(children, input)

	if t.ErrorText != "" {
		errorStyle := helperStyle
		errorStyle.Color = textFieldTheme.ErrorColor
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.ErrorText, Style: errorStyle})
	} else if t.HelperText != "" {
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.HelperText, Style: helperStyle})
	}

	return ColumnOf(MainAxisAlignmentStart, CrossAxisAlignmentStart, MainAxisSizeMin, children...)
}
