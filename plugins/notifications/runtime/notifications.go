// Package runtime is the Go API of the Drift notifications plugin
// (github.com/go-drift/drift/plugins/notifications/plugin in drift.yaml):
// local notifications the app schedules, and taps on them.
//
//	import notifications "github.com/go-drift/drift/plugins/notifications/runtime"
//
//	go func() {
//	    err := notifications.Schedule(ctx, notifications.Request{
//	        ID:    "reminder",
//	        Title: "Stand up",
//	        At:    time.Now().Add(time.Hour),
//	        Data:  map[string]any{"route": "/reminders"},
//	    })
//	    ...
//	}()
//
//	notifications.Opens().Listen(platform.StreamHandler[notifications.Open]{OnEvent: func(o notifications.Open) {
//	    drift.Dispatch(func() { navigate(o.Data["route"]) })
//	}})
//
// Schedule and CurrentSettings wait for the platform, so call them from a
// goroutine; from the UI thread they fail with platform.ErrBlocksUIThread.
// Showing notifications needs the user's permission:
// platform.Notifications.Permission. Notifications are shown in the
// foreground too, and reported on Deliveries. Only this plugin's
// notifications reach Deliveries and Opens; push notifications come from
// the Firebase plugin.
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
// (plugin/ios/DriftNotificationsPlugin.swift, plugin/android/*.kt).
const (
	channelName     = "drift/notifications"
	receivedChannel = "drift/notifications/received"
	openedChannel   = "drift/notifications/opened"
)

// queueCapacity bounds the taps kept while the app has no Opens listener.
const queueCapacity = 32

// minRepeatInterval is the shortest repeat iOS allows.
const minRepeatInterval = 60 * time.Second

var (
	channel    = platform.NewMethodChannel(channelName)
	deliveries = platform.NewStream(receivedChannel, platform.NewEventChannel(receivedChannel), parseNotification)
	// Queued: the tap that launched the app arrives before the app listens.
	opens = platform.NewStream(openedChannel, platform.NewQueuedEventChannel(openedChannel, queueCapacity), parseOpen)
)

// Request describes a notification to schedule.
type Request struct {
	// ID identifies the notification; scheduling the same ID again
	// replaces it, and Cancel takes it.
	ID string
	// Title and Body are the notification's text.
	Title string
	Body  string
	// Data is delivered with the notification, to Deliveries and Opens.
	// Values must be JSON-like: strings, numbers, booleans, lists, maps.
	Data map[string]any
	// At is when to show the notification. Zero means after Interval, or
	// at once.
	At time.Time
	// Interval delays the first notification when At is zero, and is the
	// period when Repeats is set.
	Interval time.Duration
	// Repeats shows the notification every Interval, from now. It needs an
	// Interval of at least a minute, and cannot be combined with At.
	Repeats bool
	// ChannelID is the Android notification channel, created on first use
	// with the ID as its name. Empty uses the plugin's default channel.
	// Ignored on iOS.
	ChannelID string
	// Sound is an iOS sound file name in the app bundle; empty plays the
	// default sound. Android uses the channel's sound.
	Sound string
	// Badge, when set, is the app icon badge count to show with the
	// notification (iOS).
	Badge *int
}

// Settings are the user's notification settings for the app. The
// permission itself is platform.Notifications.Permission.
type Settings struct {
	AlertsEnabled bool
	SoundsEnabled bool
	BadgesEnabled bool
}

// Notification is a notification this plugin showed.
type Notification struct {
	ID    string
	Title string
	Body  string
	Data  map[string]any
	// Foreground reports whether the app was in the foreground.
	Foreground bool
}

// Open is the user tapping one of this plugin's notifications.
type Open struct {
	ID   string
	Data map[string]any
}

// Schedule schedules req, replacing any notification with the same ID. It
// waits for the platform to accept the request, so call it from a
// goroutine; from the UI thread it fails with [platform.ErrBlocksUIThread].
// A request the platform rejects fails with the platform's error.
func Schedule(ctx context.Context, req Request) error {
	if err := req.validate(); err != nil {
		return fmt.Errorf("notifications schedule: %w", err)
	}
	args := map[string]any{
		"id":              req.ID,
		"title":           req.Title,
		"body":            req.Body,
		"data":            req.Data,
		"intervalSeconds": int64(req.Interval / time.Second),
		"repeats":         req.Repeats,
		"channelId":       req.ChannelID,
		"sound":           req.Sound,
	}
	if !req.At.IsZero() {
		args["at"] = req.At.UnixMilli()
	}
	if req.Badge != nil {
		args["badge"] = *req.Badge
	}
	if _, err := channel.Invoke(ctx, "schedule", args); err != nil {
		return fmt.Errorf("notifications schedule: %w", err)
	}
	return nil
}

func (r Request) validate() error {
	switch {
	case r.ID == "":
		return errors.New("ID is empty")
	case r.Interval < 0:
		return fmt.Errorf("Interval %v is negative", r.Interval)
	case r.Interval%time.Second != 0:
		return fmt.Errorf("Interval %v is not whole seconds", r.Interval)
	case r.Repeats && r.Interval < minRepeatInterval:
		return fmt.Errorf("Repeats needs an Interval of at least %v, got %v", minRepeatInterval, r.Interval)
	case r.Repeats && !r.At.IsZero():
		return errors.New("Repeats starts now and cannot be combined with At")
	}
	return nil
}

// Cancel removes the notification with id, scheduled or shown. Any
// goroutine may call it.
func Cancel(id string) error {
	if _, err := channel.Invoke(context.Background(), "cancel", map[string]any{"id": id}); err != nil {
		return fmt.Errorf("notifications cancel: %w", err)
	}
	return nil
}

// CancelAll removes every notification this plugin scheduled or shows,
// leaving other plugins' (push notifications) alone. Any goroutine may
// call it.
func CancelAll() error {
	if _, err := channel.Invoke(context.Background(), "cancelAll", nil); err != nil {
		return fmt.Errorf("notifications cancel all: %w", err)
	}
	return nil
}

// SetBadge sets the app icon badge count (iOS; Android launchers show
// their own badges). Any goroutine may call it.
func SetBadge(count int) error {
	if _, err := channel.Invoke(context.Background(), "setBadge", map[string]any{"count": count}); err != nil {
		return fmt.Errorf("notifications set badge: %w", err)
	}
	return nil
}

// CurrentSettings returns the user's notification settings. It waits for
// the platform, so call it from a goroutine.
func CurrentSettings(ctx context.Context) (Settings, error) {
	result, err := channel.Invoke(ctx, "getSettings", nil)
	if err != nil {
		return Settings{}, fmt.Errorf("notifications settings: %w", err)
	}
	m, ok := result.(map[string]any)
	alerts, ok1 := m["alertsEnabled"].(bool)
	sounds, ok2 := m["soundsEnabled"].(bool)
	badges, ok3 := m["badgesEnabled"].(bool)
	if !ok || !ok1 || !ok2 || !ok3 {
		return Settings{}, &drifterrors.ParseError{Channel: channelName, DataType: "Settings", Got: result}
	}
	return Settings{AlertsEnabled: alerts, SoundsEnabled: sounds, BadgesEnabled: badges}, nil
}

// Deliveries streams this plugin's notifications as they are shown while
// the app runs.
func Deliveries() *platform.Stream[Notification] {
	return deliveries
}

// Opens streams taps on this plugin's notifications, including the one
// that launched the app. Taps while no listener is subscribed are queued
// for the next one, so subscribe once the app can act on them.
func Opens() *platform.Stream[Open] {
	return opens
}

func parseNotification(data any) (Notification, error) {
	m, ok := data.(map[string]any)
	id, _ := m["id"].(string)
	if !ok || id == "" {
		return Notification{}, &drifterrors.ParseError{Channel: receivedChannel, DataType: "Notification", Got: data}
	}
	n := Notification{ID: id}
	n.Title, _ = m["title"].(string)
	n.Body, _ = m["body"].(string)
	n.Data, _ = m["data"].(map[string]any)
	n.Foreground, _ = m["foreground"].(bool)
	return n, nil
}

func parseOpen(data any) (Open, error) {
	m, ok := data.(map[string]any)
	id, _ := m["id"].(string)
	if !ok || id == "" {
		return Open{}, &drifterrors.ParseError{Channel: openedChannel, DataType: "Open", Got: data}
	}
	o := Open{ID: id}
	o.Data, _ = m["data"].(map[string]any)
	return o, nil
}
