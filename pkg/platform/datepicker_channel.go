package platform

import (
	"errors"
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

// ShowDatePicker shows the native date picker modal.
// Returns the selected date or error if cancelled.
func ShowDatePicker(config DatePickerConfig) (time.Time, error) {
	args := map[string]any{
		"initialDate": config.InitialDate.Unix(),
	}

	if config.MinDate != nil {
		args["minDate"] = config.MinDate.Unix()
	}
	if config.MaxDate != nil {
		args["maxDate"] = config.MaxDate.Unix()
	}

	result, err := datePickerChannel.Invoke("show", args)
	if err != nil {
		return time.Time{}, err
	}

	// nil result means cancelled
	if result == nil {
		return time.Time{}, ErrPickerCancelled
	}

	// Parse result - native returns {"timestamp": int64}
	var timestamp int64
	var ok bool

	// Try to get from map first (expected format)
	if resultMap, isMap := result.(map[string]any); isMap {
		timestamp, ok = toInt64(resultMap["timestamp"])
	}

	// Fallback: try direct int64 (legacy format)
	if !ok {
		timestamp, ok = toInt64(result)
	}

	if !ok {
		return time.Time{}, errors.New("invalid date picker result")
	}

	return time.Unix(timestamp, 0), nil
}

func init() {
	// Register the channel handler (for any incoming calls from native)
	datePickerChannel.SetHandler(func(method string, args any) (any, error) {
		// Currently no methods are called from native to Go for date picker
		return nil, ErrMethodNotFound
	})
}
