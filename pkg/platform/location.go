package platform

import "time"

var locationService = newLocationService()

// LocationUpdate represents a location update from the device.
type LocationUpdate struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Accuracy  float64
	Heading   float64
	Speed     float64
	Timestamp time.Time
	IsMocked  bool
}

// LocationOptions configures location update behavior.
type LocationOptions struct {
	HighAccuracy      bool
	DistanceFilter    float64
	IntervalMs        int64
	FastestIntervalMs int64
}

// GetCurrentLocation gets the device's current location.
func GetCurrentLocation(opts LocationOptions) (*LocationUpdate, error) {
	return locationService.getCurrentLocation(opts)
}

// StartLocationUpdates starts receiving continuous location updates.
func StartLocationUpdates(opts LocationOptions) error {
	return locationService.startUpdates(opts)
}

// StopLocationUpdates stops receiving location updates.
func StopLocationUpdates() error {
	return locationService.stopUpdates()
}

// LocationUpdates returns a channel that receives location updates.
func LocationUpdates() <-chan LocationUpdate {
	return locationService.updateChannel()
}

// IsLocationEnabled checks if location services are enabled on the device.
func IsLocationEnabled() (bool, error) {
	return locationService.isEnabled()
}

// GetLastKnownLocation returns the last known location without triggering a new request.
func GetLastKnownLocation() (*LocationUpdate, error) {
	return locationService.getLastKnown()
}

type locationServiceState struct {
	channel  *MethodChannel
	updates  *EventChannel
	updateCh chan LocationUpdate
}

func newLocationService() *locationServiceState {
	service := &locationServiceState{
		channel:  NewMethodChannel("drift/location"),
		updates:  NewEventChannel("drift/location/updates"),
		updateCh: make(chan LocationUpdate, 4),
	}

	service.updates.Listen(EventHandler{OnEvent: func(data any) {
		if update, ok := parseLocationUpdate(data); ok {
			service.updateCh <- update
		}
	}})

	return service
}

func (s *locationServiceState) getCurrentLocation(opts LocationOptions) (*LocationUpdate, error) {
	result, err := s.channel.Invoke("getCurrentLocation", map[string]any{
		"highAccuracy":      opts.HighAccuracy,
		"distanceFilter":    opts.DistanceFilter,
		"intervalMs":        opts.IntervalMs,
		"fastestIntervalMs": opts.FastestIntervalMs,
	})
	if err != nil {
		return nil, err
	}
	if update, ok := parseLocationUpdate(result); ok {
		return &update, nil
	}
	return nil, ErrInvalidArguments
}

func (s *locationServiceState) startUpdates(opts LocationOptions) error {
	_, err := s.channel.Invoke("startUpdates", map[string]any{
		"highAccuracy":      opts.HighAccuracy,
		"distanceFilter":    opts.DistanceFilter,
		"intervalMs":        opts.IntervalMs,
		"fastestIntervalMs": opts.FastestIntervalMs,
	})
	return err
}

func (s *locationServiceState) stopUpdates() error {
	_, err := s.channel.Invoke("stopUpdates", nil)
	return err
}

func (s *locationServiceState) isEnabled() (bool, error) {
	result, err := s.channel.Invoke("isEnabled", nil)
	if err != nil {
		return false, err
	}
	if m, ok := result.(map[string]any); ok {
		return parseBool(m["enabled"]), nil
	}
	return false, nil
}

func (s *locationServiceState) getLastKnown() (*LocationUpdate, error) {
	result, err := s.channel.Invoke("getLastKnown", nil)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	if update, ok := parseLocationUpdate(result); ok {
		return &update, nil
	}
	return nil, nil
}

func (s *locationServiceState) updateChannel() <-chan LocationUpdate {
	return s.updateCh
}

func parseLocationUpdate(data any) (LocationUpdate, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return LocationUpdate{}, false
	}
	return LocationUpdate{
		Latitude:  parseFloat64(m["latitude"]),
		Longitude: parseFloat64(m["longitude"]),
		Altitude:  parseFloat64(m["altitude"]),
		Accuracy:  parseFloat64(m["accuracy"]),
		Heading:   parseFloat64(m["heading"]),
		Speed:     parseFloat64(m["speed"]),
		Timestamp: parseTime(m["timestamp"]),
		IsMocked:  parseBool(m["isMocked"]),
	}, true
}

func parseFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	default:
		return 0
	}
}
