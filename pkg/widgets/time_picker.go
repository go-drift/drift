package widgets

import (
	"context"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
)

// TimePicker displays a time selection field that opens a native time picker modal.
type TimePicker struct {
	core.StatefulBase

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

func (t TimePicker) CreateState() core.State {
	return &timePickerState{}
}

type timePickerState struct {
	core.StateBase
	picking bool
}

func (s *timePickerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(TimePicker)

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
	decoration := w.Decoration

	var displayText string
	var displayStyle graphics.TextStyle

	showPlaceholder := w.ShowPlaceholder && w.Hour == 0 && w.Minute == 0
	if showPlaceholder {
		displayText = pickerPlaceholder(w.Placeholder, decoration, "Select time")
		if decoration != nil {
			displayStyle = decoration.HintStyle
		}
	} else {
		displayText = formatTime(w.Hour, w.Minute, w.Format, w.Is24Hour)
		displayStyle = w.TextStyle
	}

	return buildPickerField(pickerFieldParams{
		displayText:  displayText,
		displayStyle: displayStyle,
		decoration:   decoration,
		disabled:     w.Disabled,
		hint:         "Double tap to open time picker",
		onTap:        s.showPicker,
	})
}

func (s *timePickerState) showPicker() {
	if s.picking {
		return
	}

	w := s.Element().Widget().(TimePicker)

	s.picking = true

	// Show the native picker
	go func() {
		hour, minute, err := platform.ShowTimePicker(context.Background(), platform.TimePickerConfig{
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
