package platform

import (
	"context"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

// NotificationRequest describes a local notification schedule request.
type NotificationRequest struct {
	// ID is the unique identifier for the notification.
	// Use the same ID to update or cancel a scheduled notification.
	ID string
	// Title is the notification title.
	Title string
	// Body is the notification body text.
	Body string
	// Data is an optional key/value payload delivered with the notification.
	Data map[string]any
	// At schedules the notification for a specific time.
	// If zero, the notification is scheduled immediately or based on IntervalSeconds.
	At time.Time
	// IntervalSeconds sets a repeat interval in seconds.
	// If > 0 and Repeats is true, the notification repeats at this interval.
	// If > 0 and At is zero, the first delivery is scheduled after IntervalSeconds.
	IntervalSeconds int64
	// Repeats indicates whether the notification should repeat.
	// Repeating is only honored when IntervalSeconds > 0.
	Repeats bool
	// ChannelID specifies the Android notification channel.
	// Ignored on iOS.
	ChannelID string
	// Sound sets the sound name. Use "default" for the platform default.
	// An empty string uses the platform default sound.
	Sound string
	// Badge sets the app icon badge count (iOS only).
	// Nil leaves the badge unchanged.
	Badge *int
}

// NotificationSettings describes the current notification settings.
type NotificationSettings struct {
	// Status is the current notification permission status.
	Status PermissionResult
	// AlertsEnabled reports whether visible alerts are enabled.
	AlertsEnabled bool
	// SoundsEnabled reports whether sounds are enabled.
	SoundsEnabled bool
	// BadgesEnabled reports whether badge updates are enabled.
	BadgesEnabled bool
}

// NotificationEvent represents a received notification.
type NotificationEvent struct {
	// ID is the notification identifier.
	ID string
	// Title is the notification title.
	Title string
	// Body is the notification body text.
	Body string
	// Data is the payload delivered with the notification.
	Data map[string]any
	// Timestamp is when the notification was received.
	Timestamp time.Time
	// IsForeground reports whether the notification was delivered while the app was in foreground.
	IsForeground bool
}

// NotificationOpen represents a user opening a notification.
type NotificationOpen struct {
	// ID is the notification identifier.
	ID string
	// Data is the payload delivered with the notification.
	Data map[string]any
	// Action is the action identifier (if supported by the platform).
	Action string
	// Timestamp is when the notification was opened.
	Timestamp time.Time
}

// NotificationsService provides local notification management. Push
// notifications come from a plugin (plugins/firebase).
type NotificationsService struct {
	// Permission for notification access. Implements NotificationPermission
	// for iOS-specific options.
	Permission NotificationPermission

	state      *notificationServiceState
	deliveries *Stream[NotificationEvent]
	opens      *Stream[NotificationOpen]
}

// Notifications is the singleton notifications service.
var Notifications *NotificationsService

func init() {
	state := newNotificationService()
	Notifications = &NotificationsService{
		Permission: newNotificationPerm(),
		state:      state,
		deliveries: NewStream("drift/notifications/received", state.received, parseNotificationEventWithError),
		opens:      NewStream("drift/notifications/opened", state.opened, parseNotificationOpenWithError),
	}
}

type notificationServiceState struct {
	channel  *MethodChannel
	received *EventChannel
	opened   *EventChannel
}

func newNotificationService() *notificationServiceState {
	return &notificationServiceState{
		channel:  NewMethodChannel("drift/notifications"),
		received: NewEventChannel("drift/notifications/received"),
		opened:   NewEventChannel("drift/notifications/opened"),
	}
}

// Settings returns current notification settings and permission status.
func (n *NotificationsService) Settings() (NotificationSettings, error) {
	result, err := n.state.channel.Invoke(context.Background(), "getSettings", nil)
	if err != nil {
		return NotificationSettings{Status: PermissionResultUnknown}, err
	}
	settings := NotificationSettings{Status: PermissionResultUnknown}
	if m, ok := result.(map[string]any); ok {
		settings.Status = PermissionResult(parseString(m["status"]))
		settings.AlertsEnabled = parseBool(m["alertsEnabled"])
		settings.SoundsEnabled = parseBool(m["soundsEnabled"])
		settings.BadgesEnabled = parseBool(m["badgesEnabled"])
	}
	return settings, nil
}

// Schedule schedules a local notification.
func (n *NotificationsService) Schedule(req NotificationRequest) error {
	args := map[string]any{
		"id":              req.ID,
		"title":           req.Title,
		"body":            req.Body,
		"data":            req.Data,
		"intervalSeconds": req.IntervalSeconds,
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
	_, err := n.state.channel.Invoke(context.Background(), "schedule", args)
	return err
}

// Cancel cancels a scheduled notification by ID.
func (n *NotificationsService) Cancel(id string) error {
	_, err := n.state.channel.Invoke(context.Background(), "cancel", map[string]any{"id": id})
	return err
}

// CancelAll cancels all scheduled notifications.
func (n *NotificationsService) CancelAll() error {
	_, err := n.state.channel.Invoke(context.Background(), "cancelAll", nil)
	return err
}

// SetBadge sets the app badge count.
func (n *NotificationsService) SetBadge(count int) error {
	_, err := n.state.channel.Invoke(context.Background(), "setBadge", map[string]any{"count": count})
	return err
}

// Deliveries returns a stream of delivered notifications.
func (n *NotificationsService) Deliveries() *Stream[NotificationEvent] {
	return n.deliveries
}

// Opens returns a stream of notification open events (user tapped notification).
func (n *NotificationsService) Opens() *Stream[NotificationOpen] {
	return n.opens
}

func parseNotificationEventWithError(data any) (NotificationEvent, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationEvent{}, &errors.ParseError{
			Channel:  "drift/notifications/received",
			DataType: "NotificationEvent",
			Got:      data,
		}
	}
	return NotificationEvent{
		ID:           parseString(m["id"]),
		Title:        parseString(m["title"]),
		Body:         parseString(m["body"]),
		Data:         parseMap(m["data"]),
		Timestamp:    parseTime(m["timestamp"]),
		IsForeground: parseBool(m["isForeground"]),
	}, nil
}

func parseNotificationOpenWithError(data any) (NotificationOpen, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationOpen{}, &errors.ParseError{
			Channel:  "drift/notifications/opened",
			DataType: "NotificationOpen",
			Got:      data,
		}
	}
	return NotificationOpen{
		ID:        parseString(m["id"]),
		Action:    parseString(m["action"]),
		Data:      parseMap(m["data"]),
		Timestamp: parseTime(m["timestamp"]),
	}, nil
}
