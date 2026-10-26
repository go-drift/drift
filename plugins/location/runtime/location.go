// Package runtime is the Go API of the Drift location plugin
// (github.com/go-drift/drift/plugins/location/plugin in drift.yaml).
//
//	import location "github.com/go-drift/drift/plugins/location/runtime"
//
//	go func() {
//	    status, err := location.Permission.WhenInUse.Request(ctx)
//	    if err != nil || status != platform.PermissionGranted {
//	        return
//	    }
//	    fix, err := location.Current(ctx, location.Options{HighAccuracy: true})
//	    ...
//	}()
//
// Permission requests and the methods taking a ctx wait for the platform,
// so call them from a goroutine: from the UI thread they fail with
// platform.ErrBlocksUIThread. Updates is a stream; Listen from anywhere.
//
// # Background updates
//
// A stream started with UpdateOptions.Background keeps delivering updates
// to Go while the app is in the background, for as long as the app's
// process lives. It needs the plugin's always: config and the Always
// permission, and must be started while the app is in the foreground
// (Android does not allow it from the background).
//
//   - iOS keeps the app running in the background while the stream is on,
//     showing the blue location indicator in the status bar. Updates stop
//     when the user force-quits the app (swipes it away in the app
//     switcher) or the system terminates it, and the app is not relaunched
//     for them.
//   - Android runs a foreground service with an ongoing notification (its
//     text comes from the always: config), which keeps the process alive
//     after the app leaves the screen and, on most devices, after the user
//     swipes it away from Recents. Updates stop when the process is killed
//     (force stop, the Task Manager's Stop, low memory) and do not restart.
//
// StopUpdates ends the stream, the service and the indicator.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// Wire channels. Must match the literals in the plugin's native sources
// (plugin/ios/DriftLocationPlugin.swift, plugin/android/DriftLocationPlugin.kt).
const (
	channelName    = "drift/location"
	updatesChannel = "drift/location/updates"
)

var (
	channel = platform.NewMethodChannel(channelName)
	updates = platform.NewStream(updatesChannel, platform.NewEventChannel(updatesChannel), parseUpdate)
)

var (
	// ErrPermissionDenied indicates the app has no location permission:
	// request Permission.WhenInUse first.
	ErrPermissionDenied = errors.New("location permission not granted")

	// ErrDisabled indicates the user turned location services off.
	ErrDisabled = errors.New("location services are turned off")

	// ErrUnavailable indicates no fix could be obtained (Android).
	ErrUnavailable = errors.New("no location fix available")

	// ErrBackgroundNotConfigured indicates background updates were
	// requested without the location plugin's always: config.
	ErrBackgroundNotConfigured = errors.New("background location updates need the location plugin's always: config")

	// ErrAlwaysDenied indicates background updates were requested without
	// the Always permission: request Permission.Always first.
	ErrAlwaysDenied = errors.New("background location updates need the Always location permission")

	// ErrBackgroundStart indicates background updates were started while
	// the app was in the background, which Android does not allow.
	ErrBackgroundStart = errors.New("background location updates must start while the app is in the foreground")
)

// nativeErrors maps the native halves' error codes to the sentinels.
var nativeErrors = map[string]error{
	"permission_denied":         ErrPermissionDenied,
	"location_disabled":         ErrDisabled,
	"location_unavailable":      ErrUnavailable,
	"background_not_configured": ErrBackgroundNotConfigured,
	"always_denied":             ErrAlwaysDenied,
	"background_start":          ErrBackgroundStart,
}

// Permissions are the location permission levels.
type Permissions struct {
	// WhenInUse is location access while the app is in use.
	WhenInUse platform.Permission
	// Always is location access at any time, including while the app is
	// in the background, as background updates (UpdateOptions.Background)
	// need. It needs the plugin's always: config; without it, its calls
	// fail with platform.ErrPermissionNotRegistered. On iOS, WhenInUse
	// must be granted first, and iOS asks once: after that, Always reads
	// platform.PermissionPermanentlyDenied until granted in Settings. On
	// Android, the user grants it in Settings.
	Always platform.Permission
}

// Permission holds the location permissions.
var Permission = Permissions{
	WhenInUse: platform.NewPermission("location"),
	Always:    alwaysPermission{platform.NewPermission("location_always")},
}

// Update is one location reading.
type Update struct {
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
	// IsMocked reports whether the reading came from a mock provider or
	// a simulator.
	IsMocked bool
}

// Options configures location requests.
type Options struct {
	// HighAccuracy requests the highest available accuracy (may use more power).
	HighAccuracy bool
	// DistanceFilter is the minimum distance in meters between updates.
	DistanceFilter float64
	// IntervalMs is the desired update interval in milliseconds (Android).
	IntervalMs int64
	// FastestIntervalMs is the fastest acceptable update interval in
	// milliseconds (Android).
	FastestIntervalMs int64
}

func (o Options) args() map[string]any {
	return map[string]any{
		"highAccuracy":      o.HighAccuracy,
		"distanceFilter":    o.DistanceFilter,
		"intervalMs":        o.IntervalMs,
		"fastestIntervalMs": o.FastestIntervalMs,
	}
}

// UpdateOptions configures a stream of updates (StartUpdates).
type UpdateOptions struct {
	Options
	// Background keeps updates coming while the app is in the background
	// (see the package documentation). It needs the plugin's always:
	// config (else ErrBackgroundNotConfigured) and the Always permission
	// (else ErrAlwaysDenied), and must be started while the app is in the
	// foreground (else ErrBackgroundStart, Android). On iOS it also keeps
	// updates from pausing automatically while the device is stationary.
	Background bool
}

func (o UpdateOptions) args() map[string]any {
	args := o.Options.args()
	args["background"] = o.Background
	return args
}

// Current returns a fresh location fix. It waits for the platform, so call
// it from a goroutine.
func Current(ctx context.Context, opts Options) (Update, error) {
	result, err := channel.Invoke(ctx, "getCurrentLocation", opts.args())
	if err != nil {
		return Update{}, wrap("current", err)
	}
	return parseUpdate(result)
}

// LastKnown returns the most recent cached fix without requesting a new
// one, or nil when there is none. It waits for the platform, so call it
// from a goroutine.
func LastKnown(ctx context.Context) (*Update, error) {
	result, err := channel.Invoke(ctx, "getLastKnown", nil)
	if err != nil {
		return nil, wrap("last known", err)
	}
	if result == nil {
		return nil, nil
	}
	update, err := parseUpdate(result)
	if err != nil {
		return nil, err
	}
	return &update, nil
}

// IsEnabled reports whether location services are turned on. It waits for
// the platform, so call it from a goroutine.
func IsEnabled(ctx context.Context) (bool, error) {
	result, err := channel.Invoke(ctx, "isEnabled", nil)
	if err != nil {
		return false, wrap("is enabled", err)
	}
	m, _ := result.(map[string]any)
	enabled, ok := m["enabled"].(bool)
	if !ok {
		return false, &drifterrors.ParseError{Channel: channelName, DataType: "isEnabled", Got: result}
	}
	return enabled, nil
}

// StartUpdates starts streaming location updates to Updates, replacing any
// running stream's options, including whether it runs in the background.
// A failed start leaves the running stream as it was. Any goroutine may
// call it.
func StartUpdates(opts UpdateOptions) error {
	if _, err := channel.Invoke(context.Background(), "startUpdates", opts.args()); err != nil {
		return wrap("start updates", err)
	}
	return nil
}

// StopUpdates stops the stream, background or not. Any goroutine may call
// it.
func StopUpdates() error {
	if _, err := channel.Invoke(context.Background(), "stopUpdates", nil); err != nil {
		return wrap("stop updates", err)
	}
	return nil
}

// Updates streams locations while StartUpdates is in effect.
func Updates() *platform.Stream[Update] {
	return updates
}

// wrap names op and maps native error codes to the sentinels.
func wrap(op string, err error) error {
	var channelErr *platform.ChannelError
	if errors.As(err, &channelErr) {
		if sentinel, ok := nativeErrors[channelErr.Code]; ok {
			return fmt.Errorf("location %s: %w", op, sentinel)
		}
	}
	return fmt.Errorf("location %s: %w", op, err)
}

// alwaysPermission names the missing config when Always is not registered.
type alwaysPermission struct {
	platform.Permission
}

func (p alwaysPermission) Status() (platform.PermissionResult, error) {
	status, err := p.Permission.Status()
	return status, alwaysErr(err)
}

func (p alwaysPermission) Request(ctx context.Context) (platform.PermissionResult, error) {
	status, err := p.Permission.Request(ctx)
	return status, alwaysErr(err)
}

func (p alwaysPermission) ShouldShowRationale() (bool, error) {
	show, err := p.Permission.ShouldShowRationale()
	return show, alwaysErr(err)
}

func alwaysErr(err error) error {
	if errors.Is(err, platform.ErrPermissionNotRegistered) {
		return fmt.Errorf("location Always permission needs the location plugin's always: config: %w", err)
	}
	return err
}

func parseUpdate(data any) (Update, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return Update{}, &drifterrors.ParseError{Channel: channelName, DataType: "Update", Got: data}
	}
	var u Update
	fields := []struct {
		key string
		dst *float64
	}{
		{"latitude", &u.Latitude},
		{"longitude", &u.Longitude},
		{"altitude", &u.Altitude},
		{"accuracy", &u.Accuracy},
		{"heading", &u.Heading},
		{"speed", &u.Speed},
	}
	for _, f := range fields {
		v, ok := m[f.key].(float64)
		if !ok {
			return Update{}, &drifterrors.ParseError{Channel: channelName, DataType: "Update." + f.key, Got: data}
		}
		*f.dst = v
	}
	millis, ok := m["timestamp"].(float64)
	if !ok {
		return Update{}, &drifterrors.ParseError{Channel: channelName, DataType: "Update.timestamp", Got: data}
	}
	u.Timestamp = time.UnixMilli(int64(millis))
	u.IsMocked, _ = m["isMocked"].(bool)
	return u, nil
}
