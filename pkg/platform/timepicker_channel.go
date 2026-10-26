package platform

import (
	"context"
	"fmt"
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

var timePickerChannel = NewMethodChannel("drift/time_picker").WithErrors(ErrorCodes{
	"platform_unavailable": ErrPlatformUnavailable,
})

// ShowTimePicker shows the native time picker and returns the chosen hour
// and minute, or [ErrPickerCancelled]. It waits for the user, so call it
// from a goroutine: from the UI thread it fails with [ErrBlocksUIThread].
func ShowTimePicker(ctx context.Context, config TimePickerConfig) (hour, minute int, err error) {
	args := map[string]any{
		"hour":   config.Hour,
		"minute": config.Minute,
	}
	if config.Is24Hour != nil {
		args["is24Hour"] = *config.Is24Hour
	}

	result, err := timePickerChannel.Invoke(ctx, "show", args)
	if err != nil {
		return 0, 0, err
	}
	if result == nil {
		return 0, 0, ErrPickerCancelled
	}
	m, _ := result.(map[string]any)
	hour, hourOK := toInt(m["hour"])
	minute, minuteOK := toInt(m["minute"])
	if !hourOK || !minuteOK {
		return 0, 0, fmt.Errorf("time picker: want {hour, minute}, got %v", result)
	}
	return hour, minute, nil
}
