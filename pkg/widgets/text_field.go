package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
)

// TextField is a styled text input built on the native text input connection.
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
	// Enabled controls whether the field accepts input.
	Enabled bool
	// Width of the text field (0 = expand to fill).
	Width float64
	// Height of the text field.
	Height float64
	// Padding inside the text field.
	Padding layout.EdgeInsets
	// BackgroundColor of the text field.
	BackgroundColor rendering.Color
	// BorderColor of the text field.
	BorderColor rendering.Color
	// FocusColor of the text field outline.
	FocusColor rendering.Color
	// BorderRadius for rounded corners.
	BorderRadius float64
	// Style for the text.
	Style rendering.TextStyle
}

func (t TextField) CreateElement() core.Element {
	return core.NewStatelessElement(t, nil)
}

func (t TextField) Key() any {
	return nil
}

func (t TextField) Build(ctx core.BuildContext) core.Widget {
	_, colors, textTheme := theme.UseTheme(ctx)

	labelStyle := textTheme.LabelLarge
	labelStyle.Color = colors.OnSurfaceVariant
	helperStyle := textTheme.BodySmall
	helperStyle.Color = colors.OnSurfaceVariant

	textStyle := t.Style
	if textStyle.FontSize == 0 {
		textStyle = textTheme.BodyLarge
	}
	if textStyle.Color == 0 {
		textStyle.Color = colors.OnSurface
	}

	backgroundColor := t.BackgroundColor
	if backgroundColor == 0 {
		backgroundColor = colors.Surface
	}
	borderColor := t.BorderColor
	if borderColor == 0 {
		borderColor = colors.Outline
	}
	focusColor := t.FocusColor
	if focusColor == 0 {
		focusColor = colors.Primary
	}
	if t.ErrorText != "" {
		borderColor = colors.Error
	}

	height := t.Height
	if height == 0 {
		height = 48
	}

	padding := t.Padding
	if padding == (layout.EdgeInsets{}) {
		padding = layout.EdgeInsetsSymmetric(12, 8)
	}

	children := make([]core.Widget, 0, 4)
	if t.Label != "" {
		children = append(children, Text{Content: t.Label, Style: labelStyle})
		children = append(children, VSpace(6))
	}

	children = append(children, NativeTextField{
		Controller:        t.Controller,
		Style:             textStyle,
		Placeholder:       t.Placeholder,
		KeyboardType:      t.KeyboardType,
		InputAction:       t.InputAction,
		Obscure:           t.Obscure,
		Autocorrect:       t.Autocorrect,
		OnChanged:         t.OnChanged,
		OnSubmitted:       t.OnSubmitted,
		OnEditingComplete: t.OnEditingComplete,
		Enabled:           t.Enabled,
		Width:             t.Width,
		Height:            height,
		Padding:           padding,
		BackgroundColor:   backgroundColor,
		BorderColor:       borderColor,
		FocusColor:        focusColor,
		BorderRadius:      t.BorderRadius,
	})

	if t.ErrorText != "" {
		errorStyle := helperStyle
		errorStyle.Color = colors.Error
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.ErrorText, Style: errorStyle})
	} else if t.HelperText != "" {
		children = append(children, VSpace(6))
		children = append(children, Text{Content: t.HelperText, Style: helperStyle})
	}

	return ColumnOf(MainAxisAlignmentStart, CrossAxisAlignmentStart, MainAxisSizeMin, children...)
}
