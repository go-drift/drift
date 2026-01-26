package platform

import (
	"errors"
)

// TimePickerConfig contains configuration for the time picker modal.
type TimePickerConfig struct {
	// Hour is the initial hour (0-23).
	Hour int

	// Minute is the initial minute (0-59).
	Minute int

	// Is24Hour determines whether to use 24-hour format.
	// If nil, uses system default.
	Is24Hour *bool
}

// TimePickerResult contains the selected time.
type TimePickerResult struct {
	Hour   int
	Minute int
}

var timePickerChannel = NewMethodChannel("drift/time_picker")

// ShowTimePicker shows the native time picker modal.
// Returns the selected hour and minute or error if cancelled.
func ShowTimePicker(config TimePickerConfig) (hour, minute int, err error) {
	args := map[string]any{
		"hour":   config.Hour,
		"minute": config.Minute,
	}

	if config.Is24Hour != nil {
		args["is24Hour"] = *config.Is24Hour
	}

	result, err := timePickerChannel.Invoke("show", args)
	if err != nil {
		return 0, 0, err
	}

	// nil result means cancelled
	if result == nil {
		return 0, 0, ErrPickerCancelled
	}

	// Parse result as map
	resultMap, ok := result.(map[string]any)
	if !ok {
		return 0, 0, errors.New("invalid time picker result: expected map")
	}

	hour, hourOk := toInt(resultMap["hour"])
	minute, minuteOk := toInt(resultMap["minute"])

	if !hourOk || !minuteOk {
		return 0, 0, errors.New("invalid time picker result: missing or invalid hour/minute")
	}

	return hour, minute, nil
}

func init() {
	// Register the channel handler (for any incoming calls from native)
	timePickerChannel.SetHandler(func(method string, args any) (any, error) {
		// Currently no methods are called from native to Go for time picker
		return nil, ErrMethodNotFound
	})
}
