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
	// Source is "local" or "remote".
	Source string
}

// NotificationOpen represents a user opening a notification.
type NotificationOpen struct {
	// ID is the notification identifier.
	ID string
	// Data is the payload delivered with the notification.
	Data map[string]any
	// Action is the action identifier (if supported by the platform).
	Action string
	// Source is "local" or "remote".
	Source string
	// Timestamp is when the notification was opened.
	Timestamp time.Time
}

// DeviceToken represents a device push token update.
type DeviceToken struct {
	// Platform is the push provider platform (e.g., "ios", "android").
	Platform string
	// Token is the raw device push token.
	Token string
	// Timestamp is when this token was received.
	Timestamp time.Time
	// IsRefresh reports whether this is a refreshed token.
	IsRefresh bool
}

// NotificationError represents a notification-related error.
type NotificationError struct {
	// Code is a platform-specific error code.
	Code string
	// Message is the human-readable error description.
	Message string
	// Platform is the error source platform (e.g., "ios", "android").
	Platform string
}

// NotificationsService provides local and push notification management.
type NotificationsService struct {
	// Permission for notification access. Implements NotificationPermission
	// for iOS-specific options.
	Permission NotificationPermission

	state      *notificationServiceState
	deliveries *Stream[NotificationEvent]
	opens      *Stream[NotificationOpen]
	tokens     *Stream[DeviceToken]
	errors     *Stream[NotificationError]
}

// Notifications is the singleton notifications service.
var Notifications *NotificationsService

func init() {
	state := newNotificationService()
	Notifications = &NotificationsService{
		Permission: &notificationPermissionImpl{inner: newNotificationPermission()},
		state:      state,
		deliveries: NewStream("drift/notifications/received", state.received, parseNotificationEventWithError),
		opens:      NewStream("drift/notifications/opened", state.opened, parseNotificationOpenWithError),
		tokens:     NewStream("drift/notifications/token", state.tokens, parseDeviceTokenWithError),
		errors:     NewStream("drift/notifications/error", state.errors, parseNotificationErrorWithError),
	}
}

type notificationServiceState struct {
	channel  *MethodChannel
	received *EventChannel
	opened   *EventChannel
	tokens   *EventChannel
	errors   *EventChannel
}

func newNotificationService() *notificationServiceState {
	return &notificationServiceState{
		channel:  NewMethodChannel("drift/notifications"),
		received: NewEventChannel("drift/notifications/received"),
		opened:   NewEventChannel("drift/notifications/opened"),
		tokens:   NewEventChannel("drift/notifications/token"),
		errors:   NewEventChannel("drift/notifications/error"),
	}
}

// Settings returns current notification settings and permission status.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (n *NotificationsService) Settings(ctx context.Context) (NotificationSettings, error) {
	result, err := n.state.channel.Invoke("getSettings", nil)
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
// The ctx parameter is currently unused and reserved for future cancellation support.
func (n *NotificationsService) Schedule(ctx context.Context, req NotificationRequest) error {
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
	_, err := n.state.channel.Invoke("schedule", args)
	return err
}

// Cancel cancels a scheduled notification by ID.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (n *NotificationsService) Cancel(ctx context.Context, id string) error {
	_, err := n.state.channel.Invoke("cancel", map[string]any{"id": id})
	return err
}

// CancelAll cancels all scheduled notifications.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (n *NotificationsService) CancelAll(ctx context.Context) error {
	_, err := n.state.channel.Invoke("cancelAll", nil)
	return err
}

// SetBadge sets the app badge count.
// The ctx parameter is currently unused and reserved for future cancellation support.
func (n *NotificationsService) SetBadge(ctx context.Context, count int) error {
	_, err := n.state.channel.Invoke("setBadge", map[string]any{"count": count})
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

// Tokens returns a stream of device push token updates.
func (n *NotificationsService) Tokens() *Stream[DeviceToken] {
	return n.tokens
}

// Errors returns a stream of notification errors.
func (n *NotificationsService) Errors() *Stream[NotificationError] {
	return n.errors
}

// RegisterForPush registers the device for push notifications.
// On iOS, call Permission.Request() first. On Android, this triggers FCM token retrieval.
func (n *NotificationsService) RegisterForPush(ctx context.Context) error {
	_, err := n.state.channel.Invoke("registerForPush", nil)
	return err
}

// GetPushToken returns the current push token if available.
func (n *NotificationsService) GetPushToken(ctx context.Context) (string, error) {
	result, err := n.state.channel.Invoke("getPushToken", nil)
	if err != nil {
		return "", err
	}
	if m, ok := result.(map[string]any); ok {
		return parseString(m["token"]), nil
	}
	return "", nil
}

// SubscribeToTopic subscribes to a push notification topic.
func (n *NotificationsService) SubscribeToTopic(ctx context.Context, topic string) error {
	_, err := n.state.channel.Invoke("subscribeToTopic", map[string]any{
		"topic": topic,
	})
	return err
}

// UnsubscribeFromTopic unsubscribes from a push notification topic.
func (n *NotificationsService) UnsubscribeFromTopic(ctx context.Context, topic string) error {
	_, err := n.state.channel.Invoke("unsubscribeFromTopic", map[string]any{
		"topic": topic,
	})
	return err
}

// DeletePushToken deletes the current push token.
func (n *NotificationsService) DeletePushToken(ctx context.Context) error {
	_, err := n.state.channel.Invoke("deletePushToken", nil)
	return err
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
		Source:       parseString(m["source"]),
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
		Source:    parseString(m["source"]),
		Data:      parseMap(m["data"]),
		Timestamp: parseTime(m["timestamp"]),
	}, nil
}

func parseDeviceTokenWithError(data any) (DeviceToken, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return DeviceToken{}, &errors.ParseError{
			Channel:  "drift/notifications/token",
			DataType: "DeviceToken",
			Got:      data,
		}
	}
	return DeviceToken{
		Platform:  parseString(m["platform"]),
		Token:     parseString(m["token"]),
		Timestamp: parseTime(m["timestamp"]),
		IsRefresh: parseBool(m["isRefresh"]),
	}, nil
}

func parseNotificationErrorWithError(data any) (NotificationError, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationError{}, &errors.ParseError{
			Channel:  "drift/notifications/error",
			DataType: "NotificationError",
			Got:      data,
		}
	}
	return NotificationError{
		Code:     parseString(m["code"]),
		Message:  parseString(m["message"]),
		Platform: parseString(m["platform"]),
	}, nil
}
