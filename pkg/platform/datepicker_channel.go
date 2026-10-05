package platform

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrPickerCancelled is returned when the user cancels the picker.
var ErrPickerCancelled = errors.New("picker cancelled")

// DatePickerConfig contains configuration for the date picker modal.
type DatePickerConfig struct {
	// InitialDate is the initially selected date.
	InitialDate time.Time

	// MinDate is the minimum selectable date (optional).
	MinDate *time.Time

	// MaxDate is the maximum selectable date (optional).
	MaxDate *time.Time
}

var datePickerChannel = NewMethodChannel("drift/date_picker")

// ShowDatePicker shows the native date picker and returns the chosen date,
// at midnight in [time.Local], or [ErrPickerCancelled]. Dates cross the
// channel as calendar days (year, month, day in each time's own location),
// so no time zone shifts them. It waits for the user, so call it from a
// goroutine: from the UI thread it fails with [ErrBlocksUIThread].
func ShowDatePicker(ctx context.Context, config DatePickerConfig) (time.Time, error) {
	args := map[string]any{"initialDate": calendarDay(config.InitialDate)}
	if config.MinDate != nil {
		args["minDate"] = calendarDay(*config.MinDate)
	}
	if config.MaxDate != nil {
		args["maxDate"] = calendarDay(*config.MaxDate)
	}

	result, err := datePickerChannel.Invoke(ctx, "show", args)
	if err != nil {
		return time.Time{}, err
	}
	if result == nil {
		return time.Time{}, ErrPickerCancelled
	}
	m, _ := result.(map[string]any)
	year, yearOK := toInt(m["year"])
	month, monthOK := toInt(m["month"])
	day, dayOK := toInt(m["day"])
	if !yearOK || !monthOK || !dayOK {
		return time.Time{}, fmt.Errorf("date picker: want {year, month, day}, got %v", result)
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), nil
}

// calendarDay is t's date on the wire: month 1 to 12.
func calendarDay(t time.Time) map[string]any {
	year, month, day := t.Date()
	return map[string]any{"year": year, "month": int(month), "day": day}
}
