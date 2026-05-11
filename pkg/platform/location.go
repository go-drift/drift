package platform

import (
	"context"
	"fmt"
	"time"
)

// LocationUpdate represents a location update from the device.
type LocationUpdate struct {
	// Latitude is the latitude in degrees.
	Latitude float64
	// Longitude is the longitude in degrees.
	Longitude float64
	// Altitude is the altitude in meters.
	Altitude float64
	// Accuracy is the estimated horizontal accuracy in meters.
	Accuracy float64
	// Heading is the direction of travel in degrees.
	Heading float64
	// Speed is the speed in meters per second.
	Speed float64
	// Timestamp is when the reading was taken.
	Timestamp time.Time
	// IsMocked reports whether the reading came from a mock provider (Android).
	IsMocked bool
}

// LocationOptions configures location update behavior.
type LocationOptions struct {
	// HighAccuracy requests the highest available accuracy (may use more power).
	HighAccuracy bool
	// DistanceFilter is the minimum distance in meters between updates.
	DistanceFilter float64
	// IntervalMs is the desired update interval in milliseconds.
	IntervalMs int64
	// FastestIntervalMs is the fastest acceptable update interval in milliseconds (Android).
	FastestIntervalMs int64
}

// LocationService provides location and GPS services.
// Context parameters are currently unused and reserved for future cancellation support.
type LocationService struct {
	// Permission provides access to location permission levels.
	Permission struct {
		// WhenInUse permission for foreground location access.
		WhenInUse Permission
		// Always permission for background location access.
		// On iOS, WhenInUse must be granted before requesting Always.
		Always Permission
	}

	state   *locationServiceState
	updates *Stream[LocationUpdate]
}

// Location is the singleton location service.
var Location *LocationService

func init() {
	state := newLocationService()
	Location = &LocationService{
		state:   state,
		updates: NewStream("drift/location/updates", state.updates, parseLocationUpdateWithError),
	}
	Location.Permission.WhenInUse = newPerm("location")
	Location.Permission.Always = newPerm("location_always")
}

type locationServiceState struct {
	channel *MethodChannel
	updates *EventChannel
}

func newLocationService() *locationServiceState {
	return &locationServiceState{
		channel: NewMethodChannel("drift/location"),
		updates: NewEventChannel("drift/location/updates"),
	}
}

// GetCurrent returns the current device location.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (l *LocationService) GetCurrent(ctx context.Context, opts LocationOptions) (*LocationUpdate, error) {
	result, err := l.state.channel.Invoke("getCurrentLocation", map[string]any{
		"highAccuracy":      opts.HighAccuracy,
		"distanceFilter":    opts.DistanceFilter,
		"intervalMs":        opts.IntervalMs,
		"fastestIntervalMs": opts.FastestIntervalMs,
	})
	if err != nil {
		return nil, err
	}
	update, err := parseLocationUpdateWithError(result)
	if err != nil {
		return nil, ErrInvalidArguments
	}
	return &update, nil
}

// StartUpdates begins continuous location updates.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (l *LocationService) StartUpdates(ctx context.Context, opts LocationOptions) error {
	_, err := l.state.channel.Invoke("startUpdates", map[string]any{
		"highAccuracy":      opts.HighAccuracy,
		"distanceFilter":    opts.DistanceFilter,
		"intervalMs":        opts.IntervalMs,
		"fastestIntervalMs": opts.FastestIntervalMs,
	})
	return err
}

// StopUpdates stops location updates.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (l *LocationService) StopUpdates(ctx context.Context) error {
	_, err := l.state.channel.Invoke("stopUpdates", nil)
	return err
}

// Updates returns a stream of location updates.
func (l *LocationService) Updates() *Stream[LocationUpdate] {
	return l.updates
}

// IsEnabled checks if location services are enabled on the device.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (l *LocationService) IsEnabled(ctx context.Context) (bool, error) {
	result, err := l.state.channel.Invoke("isEnabled", nil)
	if err != nil {
		return false, err
	}
	if m, ok := result.(map[string]any); ok {
		return parseBool(m["enabled"]), nil
	}
	return false, nil
}

// LastKnown returns the last known location without triggering a new request.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (l *LocationService) LastKnown(ctx context.Context) (*LocationUpdate, error) {
	result, err := l.state.channel.Invoke("getLastKnown", nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	update, err := parseLocationUpdateWithError(result)
	if err != nil {
		return nil, nil
	}
	return &update, nil
}

func parseLocationUpdateWithError(data any) (LocationUpdate, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return LocationUpdate{}, fmt.Errorf("expected map, got %T", data)
	}
	lat, _ := toFloat64(m["latitude"])
	lon, _ := toFloat64(m["longitude"])
	alt, _ := toFloat64(m["altitude"])
	acc, _ := toFloat64(m["accuracy"])
	hdg, _ := toFloat64(m["heading"])
	spd, _ := toFloat64(m["speed"])
	return LocationUpdate{
		Latitude:  lat,
		Longitude: lon,
		Altitude:  alt,
		Accuracy:  acc,
		Heading:   hdg,
		Speed:     spd,
		Timestamp: parseTime(m["timestamp"]),
		IsMocked:  parseBool(m["isMocked"]),
	}, nil
}
