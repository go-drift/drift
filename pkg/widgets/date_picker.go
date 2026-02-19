package widgets

import (
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/semantics"
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

	// ErrorColor for error text. Zero means transparent (error text not visible).
	ErrorColor graphics.Color
}

// DatePicker displays a date selection field that opens a native date picker modal.
type DatePicker struct {
	core.StatefulBase

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

	// If custom child provided, wrap it with gesture detector
	if w.Child != nil {
		return GestureDetector{
			OnTap: func() {
				if !w.Disabled {
					s.showPicker()
				}
			},
			Child: w.Child,
		}
	}

	// Build default styled field
	return s.buildDefaultField(ctx, w)
}

func (s *datePickerState) buildDefaultField(ctx core.BuildContext, w DatePicker) core.Widget {
	// Apply defaults from decoration
	decoration := w.Decoration
	if decoration == nil {
		decoration = &InputDecoration{}
	}

	borderRadius := decoration.BorderRadius
	borderColor := decoration.BorderColor
	bgColor := decoration.BackgroundColor

	contentPadding := decoration.ContentPadding
	if contentPadding == (layout.EdgeInsets{}) {
		contentPadding = layout.EdgeInsets{Left: 12, Top: 12, Right: 12, Bottom: 12}
	}

	// Text style - use field values directly
	textStyle := w.TextStyle
	hintStyle := decoration.HintStyle
	labelStyle := decoration.LabelStyle
	helperStyle := decoration.HelperStyle

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
		Child: DecoratedBox{
			Color:        bgColor,
			BorderColor:  borderColor,
			BorderWidth:  1,
			BorderRadius: borderRadius,
			Child: Padding{
				Padding: contentPadding,
				Child: Row{
					CrossAxisAlignment: CrossAxisAlignmentCenter,
					MainAxisSize:       MainAxisSizeMin,
					Children:           contentChildren,
				},
			},
		},
	}

	children = append(children, inputContainer)

	// Helper or error text
	if decoration.ErrorText != "" {
		errorStyle := helperStyle
		errorStyle.Color = decoration.ErrorColor
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
		Child: Semantics{
			Hint:  "Double tap to open date picker",
			Role:  semantics.SemanticsRoleButton,
			Flags: semantics.SemanticsHasEnabledState | boolToFlag(!w.Disabled, semantics.SemanticsIsEnabled),
			OnTap: func() { s.showPicker() },
			Child: Column{
				MainAxisSize:       MainAxisSizeMin,
				CrossAxisAlignment: CrossAxisAlignmentStart,
				Children:           children,
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
