package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/semantics"
)

// TimePicker displays a time selection field that opens a native time picker modal.
type TimePicker struct {
	// Hour is the current selected hour (0-23).
	Hour int

	// Minute is the current selected minute (0-59).
	Minute int

	// OnChanged is called when the user selects a time.
	OnChanged func(hour, minute int)

	// Disabled disables interaction when true.
	Disabled bool

	// Is24Hour determines whether to use 24-hour format.
	// If nil, uses system default.
	Is24Hour *bool

	// Format is the time format string (Go time format). Default: "3:04 PM"
	// Set to "15:04" for 24-hour format.
	Format string

	// Placeholder is shown when no time is selected (Hour and Minute are both 0 and ShowPlaceholder is true).
	Placeholder string

	// ShowPlaceholder controls whether to show placeholder when hour/minute are 0.
	ShowPlaceholder bool

	// Decoration provides styling (label, hint, border, icons, etc.).
	Decoration *InputDecoration

	// TextStyle for the value text.
	TextStyle graphics.TextStyle

	// Child overrides the default rendering for full customization.
	Child core.Widget
}

func (t TimePicker) CreateElement() core.Element {
	return core.NewStatefulElement(t, nil)
}

func (t TimePicker) Key() any {
	return nil
}

func (t TimePicker) CreateState() core.State {
	return &timePickerState{}
}

type timePickerState struct {
	element *core.StatefulElement
	picking bool
}

func (s *timePickerState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *timePickerState) InitState() {}

func (s *timePickerState) Dispose() {}

func (s *timePickerState) DidChangeDependencies() {}

func (s *timePickerState) DidUpdateWidget(oldWidget core.StatefulWidget) {}

func (s *timePickerState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *timePickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(TimePicker)

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

func (s *timePickerState) buildDefaultField(ctx core.BuildContext, w TimePicker) core.Widget {
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

	// Format the time value
	var displayText string
	var displayStyle graphics.TextStyle

	showPlaceholder := w.ShowPlaceholder && w.Hour == 0 && w.Minute == 0
	if showPlaceholder {
		if w.Placeholder != "" {
			displayText = w.Placeholder
		} else if decoration.HintText != "" {
			displayText = decoration.HintText
		} else {
			displayText = "Select time"
		}
		displayStyle = hintStyle
	} else {
		displayText = formatTime(w.Hour, w.Minute, w.Format, w.Is24Hour)
		displayStyle = textStyle
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

	// Suffix icon
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
			Hint:  "Double tap to open time picker",
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

func (s *timePickerState) showPicker() {
	if s.picking {
		return
	}

	w := s.element.Widget().(TimePicker)

	s.picking = true

	// Show the native picker
	go func() {
		hour, minute, err := platform.ShowTimePicker(platform.TimePickerConfig{
			Hour:     w.Hour,
			Minute:   w.Minute,
			Is24Hour: w.Is24Hour,
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
				w.OnChanged(hour, minute)
			}
		})
	}()
}

// formatTime formats time for display.
func formatTime(hour, minute int, format string, is24Hour *bool) string {
	// If format is provided, use it
	if format != "" {
		// Simple replacement for common time format patterns
		h := hour
		ampm := "AM"
		if h >= 12 {
			ampm = "PM"
			if h > 12 {
				h -= 12
			}
		}
		if h == 0 {
			h = 12
		}

		// Handle common formats
		switch format {
		case "15:04":
			return formatHourMinute24(hour, minute)
		case "3:04 PM":
			return formatHourMinute12(hour, minute, ampm)
		case "03:04 PM":
			return formatHourMinute12Padded(hour, minute, ampm)
		}
	}

	// Default format based on is24Hour preference
	use24 := false
	if is24Hour != nil {
		use24 = *is24Hour
	}

	if use24 {
		return formatHourMinute24(hour, minute)
	}
	return formatHourMinute12AM(hour, minute)
}

func formatHourMinute24(hour, minute int) string {
	return padZero(hour) + ":" + padZero(minute)
}

func formatHourMinute12AM(hour, minute int) string {
	h := hour
	ampm := "AM"
	if h >= 12 {
		ampm = "PM"
		if h > 12 {
			h -= 12
		}
	}
	if h == 0 {
		h = 12
	}
	return intToString(h) + ":" + padZero(minute) + " " + ampm
}

func formatHourMinute12(hour, minute int, ampm string) string {
	h := hour
	if h >= 12 && h != 12 {
		h -= 12
	}
	if h == 0 {
		h = 12
	}
	return intToString(h) + ":" + padZero(minute) + " " + ampm
}

func formatHourMinute12Padded(hour, minute int, ampm string) string {
	h := hour
	if h >= 12 && h != 12 {
		h -= 12
	}
	if h == 0 {
		h = 12
	}
	return padZero(h) + ":" + padZero(minute) + " " + ampm
}

func padZero(n int) string {
	if n < 10 {
		return "0" + intToString(n)
	}
	return intToString(n)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	result := make([]byte, 0, 10)
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}
