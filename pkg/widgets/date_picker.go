package widgets

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// InputDecoration provides styling configuration for input widgets like DatePicker and TimePicker.
type InputDecoration struct {
	// LabelText is shown above the input.
	LabelText string

	// HintText is shown when the input is empty.
	HintText string

	// HelperText is shown below the input.
	HelperText string

	// ErrorText replaces HelperText when validation fails.
	ErrorText string

	// PrefixIcon is shown at the start of the input.
	PrefixIcon core.Widget

	// SuffixIcon is shown at the end of the input (e.g., calendar icon).
	SuffixIcon core.Widget

	// ContentPadding is the padding inside the input field.
	ContentPadding layout.EdgeInsets

	// BorderRadius for the input field.
	BorderRadius float64

	// BorderColor when not focused.
	BorderColor graphics.Color

	// FocusedBorderColor when focused.
	FocusedBorderColor graphics.Color

	// BackgroundColor of the input field.
	BackgroundColor graphics.Color

	// LabelStyle for the label text.
	LabelStyle graphics.TextStyle

	// HintStyle for the hint text.
	HintStyle graphics.TextStyle

	// HelperStyle for the helper/error text.
	HelperStyle graphics.TextStyle
}

// DatePicker displays a date selection field that opens a native date picker modal.
type DatePicker struct {
	// Value is the current selected date (nil = no selection).
	Value *time.Time

	// OnChanged is called when the user selects a date.
	OnChanged func(time.Time)

	// Disabled disables interaction when true.
	Disabled bool

	// MinDate is the minimum selectable date (optional).
	MinDate *time.Time

	// MaxDate is the maximum selectable date (optional).
	MaxDate *time.Time

	// Format is the date format string (Go time format). Default: "Jan 2, 2006"
	Format string

	// Placeholder is shown when Value is nil.
	Placeholder string

	// Decoration provides styling (label, hint, border, icons, etc.).
	Decoration *InputDecoration

	// TextStyle for the value text.
	TextStyle graphics.TextStyle

	// Child overrides the default rendering for full customization.
	Child core.Widget
}

func (d DatePicker) CreateElement() core.Element {
	return core.NewStatefulElement(d, nil)
}

func (d DatePicker) Key() any {
	return nil
}

func (d DatePicker) CreateState() core.State {
	return &datePickerState{}
}

type datePickerState struct {
	element *core.StatefulElement
	picking bool
}

func (s *datePickerState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *datePickerState) InitState() {}

func (s *datePickerState) Dispose() {}

func (s *datePickerState) DidChangeDependencies() {}

func (s *datePickerState) DidUpdateWidget(oldWidget core.StatefulWidget) {}

func (s *datePickerState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *datePickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(DatePicker)
	themeData, _, textTheme := theme.UseTheme(ctx)

	// If custom child provided, wrap it with gesture detector
	if w.Child != nil {
		return GestureDetector{
			OnTap: func() {
				if !w.Disabled {
					s.showPicker()
				}
			},
			ChildWidget: w.Child,
		}
	}

	// Build default styled field
	return s.buildDefaultField(ctx, w, *themeData, textTheme)
}

func (s *datePickerState) buildDefaultField(ctx core.BuildContext, w DatePicker, themeData theme.ThemeData, textTheme theme.TextTheme) core.Widget {
	textFieldTheme := themeData.TextFieldThemeOf()

	// Apply defaults from decoration or theme
	decoration := w.Decoration
	if decoration == nil {
		decoration = &InputDecoration{}
	}

	borderRadius := decoration.BorderRadius
	if borderRadius == 0 {
		borderRadius = 8
	}

	borderColor := decoration.BorderColor
	if borderColor == 0 {
		borderColor = textFieldTheme.BorderColor
	}

	bgColor := decoration.BackgroundColor
	if bgColor == 0 {
		bgColor = textFieldTheme.BackgroundColor
	}

	contentPadding := decoration.ContentPadding
	if contentPadding == (layout.EdgeInsets{}) {
		contentPadding = layout.EdgeInsets{Left: 12, Top: 12, Right: 12, Bottom: 12}
	}

	// Text style
	textStyle := w.TextStyle
	if textStyle.FontSize == 0 {
		textStyle = textTheme.BodyLarge
	}
	if textStyle.Color == 0 {
		textStyle.Color = textFieldTheme.TextColor
	}

	// Hint style
	hintStyle := decoration.HintStyle
	if hintStyle.FontSize == 0 {
		hintStyle = textTheme.BodyLarge
	}
	if hintStyle.Color == 0 {
		hintStyle.Color = textFieldTheme.PlaceholderColor
	}

	// Label style
	labelStyle := decoration.LabelStyle
	if labelStyle.FontSize == 0 {
		labelStyle = textTheme.LabelMedium
	}
	if labelStyle.Color == 0 {
		labelStyle.Color = textFieldTheme.LabelColor
	}

	// Helper style
	helperStyle := decoration.HelperStyle
	if helperStyle.FontSize == 0 {
		helperStyle = textTheme.BodySmall
	}
	if helperStyle.Color == 0 {
		helperStyle.Color = textFieldTheme.PlaceholderColor
	}

	// Format the date value
	format := w.Format
	if format == "" {
		format = "Jan 2, 2006"
	}

	var displayText string
	var displayStyle graphics.TextStyle
	if w.Value != nil {
		displayText = w.Value.Format(format)
		displayStyle = textStyle
	} else {
		if w.Placeholder != "" {
			displayText = w.Placeholder
		} else if decoration.HintText != "" {
			displayText = decoration.HintText
		} else {
			displayText = "Select date"
		}
		displayStyle = hintStyle
	}

	// Build the content row
	var contentChildren []core.Widget

	// Prefix icon
	if decoration.PrefixIcon != nil {
		contentChildren = append(contentChildren, decoration.PrefixIcon)
		contentChildren = append(contentChildren, SizedBox{Width: 8})
	}

	// Text
	contentChildren = append(contentChildren, Text{
		Content: displayText,
		Style:   displayStyle,
	})

	// Suffix icon (default to calendar)
	if decoration.SuffixIcon != nil {
		contentChildren = append(contentChildren, SizedBox{Width: 8})
		contentChildren = append(contentChildren, decoration.SuffixIcon)
	}

	// Build the field
	var children []core.Widget

	// Label
	if decoration.LabelText != "" {
		children = append(children, Text{Content: decoration.LabelText, Style: labelStyle})
		children = append(children, SizedBox{Height: 6})
	}

	// Apply disabled styling
	opacity := 1.0
	if w.Disabled {
		opacity = 0.5
	}

	// Main input container
	inputContainer := Opacity{
		Opacity: opacity,
		ChildWidget: DecoratedBox{
			Color:        bgColor,
			BorderColor:  borderColor,
			BorderWidth:  1,
			BorderRadius: borderRadius,
			ChildWidget: Padding{
				Padding: contentPadding,
				ChildWidget: Row{
					CrossAxisAlignment: CrossAxisAlignmentCenter,
					MainAxisSize:       MainAxisSizeMin,
					ChildrenWidgets:    contentChildren,
				},
			},
		},
	}

	children = append(children, inputContainer)

	// Helper or error text
	if decoration.ErrorText != "" {
		errorStyle := helperStyle
		errorStyle.Color = textFieldTheme.ErrorColor
		children = append(children, SizedBox{Height: 6})
		children = append(children, Text{Content: decoration.ErrorText, Style: errorStyle})
	} else if decoration.HelperText != "" {
		children = append(children, SizedBox{Height: 6})
		children = append(children, Text{Content: decoration.HelperText, Style: helperStyle})
	}

	// Wrap with gesture detector
	return GestureDetector{
		OnTap: func() {
			if !w.Disabled {
				s.showPicker()
			}
		},
		ChildWidget: Semantics{
			Hint:   "Double tap to open date picker",
			Role:   semantics.SemanticsRoleButton,
			Flags:  semantics.SemanticsHasEnabledState | boolToFlag(!w.Disabled, semantics.SemanticsIsEnabled),
			OnTap:  func() { s.showPicker() },
			ChildWidget: Column{
				MainAxisSize:       MainAxisSizeMin,
				CrossAxisAlignment: CrossAxisAlignmentStart,
				ChildrenWidgets:    children,
			},
		},
	}
}

func (s *datePickerState) showPicker() {
	if s.picking {
		return
	}

	w := s.element.Widget().(DatePicker)

	// Determine initial date
	initialDate := time.Now()
	if w.Value != nil {
		initialDate = *w.Value
	}

	s.picking = true

	// Show the native picker
	go func() {
		selectedDate, err := platform.ShowDatePicker(platform.DatePickerConfig{
			InitialDate: initialDate,
			MinDate:     w.MinDate,
			MaxDate:     w.MaxDate,
		})

		// Dispatch callback to main thread for safe state updates
		platform.Dispatch(func() {
			s.picking = false

			if err != nil {
				// User cancelled or error - do nothing
				return
			}

			// Notify callback
			if w.OnChanged != nil {
				w.OnChanged(selectedDate)
			}
		})
	}()
}

// boolToFlag returns the flag if condition is true, otherwise 0.
func boolToFlag(condition bool, flag semantics.SemanticsFlag) semantics.SemanticsFlag {
	if condition {
		return flag
	}
	return 0
}
