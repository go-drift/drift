package platform

import (
	"fmt"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

var notificationService = newNotificationService()

// PermissionStatus represents notification permission state.
type PermissionStatus string

const (
	PermissionStatusGranted       PermissionStatus = "granted"
	PermissionStatusDenied        PermissionStatus = "denied"
	PermissionStatusProvisional   PermissionStatus = "provisional"
	PermissionStatusNotDetermined PermissionStatus = "not_determined"
	PermissionStatusUnknown       PermissionStatus = "unknown"
)

// PermissionOptions defines the permission capabilities to request.
type PermissionOptions struct {
	Alert       bool
	Sound       bool
	Badge       bool
	Provisional bool
}

// NotificationRequest describes a local notification schedule request.
type NotificationRequest struct {
	ID              string
	Title           string
	Body            string
	Data            map[string]any
	At              time.Time
	IntervalSeconds int64
	Repeats         bool
	ChannelID       string
	Sound           string
	Badge           *int
}

// NotificationSettings describes the current notification settings.
type NotificationSettings struct {
	Status        PermissionStatus
	AlertsEnabled bool
	SoundsEnabled bool
	BadgesEnabled bool
}

// NotificationEvent represents a received notification.
type NotificationEvent struct {
	ID           string
	Title        string
	Body         string
	Data         map[string]any
	Timestamp    time.Time
	IsForeground bool
	Source       string
}

// NotificationOpen represents a user opening a notification.
type NotificationOpen struct {
	ID        string
	Data      map[string]any
	Action    string
	Source    string
	Timestamp time.Time
}

// DeviceToken represents a device push token update.
type DeviceToken struct {
	Platform  string
	Token     string
	Timestamp time.Time
	IsRefresh bool
}

// NotificationError represents a notification-related error.
type NotificationError struct {
	Code     string
	Message  string
	Platform string
}

// RequestNotificationPermissions requests notification permissions.
func RequestNotificationPermissions(options PermissionOptions) (PermissionStatus, error) {
	return notificationService.requestPermissions(options)
}

// GetNotificationSettings returns the current notification settings.
func GetNotificationSettings() (NotificationSettings, error) {
	return notificationService.getSettings()
}

// ScheduleLocalNotification schedules a local notification.
func ScheduleLocalNotification(request NotificationRequest) error {
	return notificationService.scheduleLocal(request)
}

// CancelLocalNotification cancels a scheduled local notification by ID.
func CancelLocalNotification(id string) error {
	return notificationService.cancelLocal(id)
}

// CancelAllLocalNotifications cancels all scheduled local notifications.
func CancelAllLocalNotifications() error {
	return notificationService.cancelAllLocal()
}

// SetNotificationBadge sets the app badge count.
func SetNotificationBadge(count int) error {
	return notificationService.setBadge(count)
}

// Notifications streams notification deliveries.
func Notifications() <-chan NotificationEvent {
	return notificationService.receivedChannel()
}

// NotificationOpens streams notification open events.
func NotificationOpens() <-chan NotificationOpen {
	return notificationService.openChannel()
}

// DeviceTokens streams device push token updates.
func DeviceTokens() <-chan DeviceToken {
	return notificationService.tokenChannel()
}

// NotificationErrors streams notification errors.
func NotificationErrors() <-chan NotificationError {
	return notificationService.errorChannel()
}

// NotificationPermissionUpdates streams permission updates after a request.
func NotificationPermissionUpdates() <-chan PermissionStatus {
	return notificationService.permissionChannel()
}

type notificationServiceState struct {
	channel     *MethodChannel
	received    *EventChannel
	opened      *EventChannel
	permissions *EventChannel
	tokens      *EventChannel
	errors      *EventChannel

	receivedCh    chan NotificationEvent
	openedCh      chan NotificationOpen
	permissionsCh chan PermissionStatus
	tokensCh      chan DeviceToken
	errorsCh      chan NotificationError
}

func newNotificationService() *notificationServiceState {
	service := &notificationServiceState{
		channel:       NewMethodChannel("github.com/go-drift/drift/notifications"),
		received:      NewEventChannel("github.com/go-drift/drift/notifications/received"),
		opened:        NewEventChannel("github.com/go-drift/drift/notifications/opened"),
		permissions:   NewEventChannel("github.com/go-drift/drift/notifications/permission"),
		tokens:        NewEventChannel("github.com/go-drift/drift/notifications/token"),
		errors:        NewEventChannel("github.com/go-drift/drift/notifications/error"),
		receivedCh:    make(chan NotificationEvent, 4),
		openedCh:      make(chan NotificationOpen, 4),
		permissionsCh: make(chan PermissionStatus, 4),
		tokensCh:      make(chan DeviceToken, 4),
		errorsCh:      make(chan NotificationError, 4),
	}

	service.received.Listen(EventHandler{
		OnEvent: func(data any) {
			event, ok := parseNotificationEvent(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "notifications.parseEvent",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/notifications/received",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/notifications/received",
						DataType: "NotificationEvent",
						Got:      data,
					},
				})
				return
			}
			service.receivedCh <- event
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "notifications.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/notifications/received",
				Err:     err,
			})
		},
	})
	service.opened.Listen(EventHandler{
		OnEvent: func(data any) {
			event, ok := parseNotificationOpen(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "notifications.parseOpen",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/notifications/opened",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/notifications/opened",
						DataType: "NotificationOpen",
						Got:      data,
					},
				})
				return
			}
			service.openedCh <- event
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "notifications.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/notifications/opened",
				Err:     err,
			})
		},
	})
	service.permissions.Listen(EventHandler{
		OnEvent: func(data any) {
			status, ok := parsePermissionStatusEvent(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "notifications.parsePermission",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/notifications/permission",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/notifications/permission",
						DataType: "PermissionStatus",
						Got:      data,
					},
				})
				return
			}
			service.permissionsCh <- status
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "notifications.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/notifications/permission",
				Err:     err,
			})
		},
	})
	service.tokens.Listen(EventHandler{
		OnEvent: func(data any) {
			event, ok := parseDeviceToken(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "notifications.parseToken",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/notifications/token",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/notifications/token",
						DataType: "DeviceToken",
						Got:      data,
					},
				})
				return
			}
			service.tokensCh <- event
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "notifications.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/notifications/token",
				Err:     err,
			})
		},
	})
	service.errors.Listen(EventHandler{
		OnEvent: func(data any) {
			event, ok := parseNotificationError(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "notifications.parseError",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/notifications/error",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/notifications/error",
						DataType: "NotificationError",
						Got:      data,
					},
				})
				return
			}
			service.errorsCh <- event
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "notifications.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/notifications/error",
				Err:     err,
			})
		},
	})

	return service
}

func (s *notificationServiceState) requestPermissions(options PermissionOptions) (PermissionStatus, error) {
	result, err := s.channel.Invoke("requestPermissions", map[string]any{
		"alert":       options.Alert,
		"sound":       options.Sound,
		"badge":       options.Badge,
		"provisional": options.Provisional,
	})
	if err != nil {
		return PermissionStatusUnknown, err
	}
	status := parsePermissionStatus(result)
	return status, nil
}

func (s *notificationServiceState) getSettings() (NotificationSettings, error) {
	result, err := s.channel.Invoke("getSettings", nil)
	if err != nil {
		return NotificationSettings{Status: PermissionStatusUnknown}, err
	}
	settings := NotificationSettings{Status: PermissionStatusUnknown}
	if m, ok := result.(map[string]any); ok {
		settings.Status = PermissionStatus(parseString(m["status"]))
		settings.AlertsEnabled = parseBool(m["alertsEnabled"])
		settings.SoundsEnabled = parseBool(m["soundsEnabled"])
		settings.BadgesEnabled = parseBool(m["badgesEnabled"])
	}
	return settings, nil
}

func (s *notificationServiceState) scheduleLocal(request NotificationRequest) error {
	args := map[string]any{
		"id":              request.ID,
		"title":           request.Title,
		"body":            request.Body,
		"data":            request.Data,
		"intervalSeconds": request.IntervalSeconds,
		"repeats":         request.Repeats,
		"channelId":       request.ChannelID,
		"sound":           request.Sound,
	}
	if !request.At.IsZero() {
		args["at"] = request.At.UnixMilli()
	}
	if request.Badge != nil {
		args["badge"] = *request.Badge
	}
	_, err := s.channel.Invoke("schedule", args)
	return err
}

func (s *notificationServiceState) cancelLocal(id string) error {
	_, err := s.channel.Invoke("cancel", map[string]any{"id": id})
	return err
}

func (s *notificationServiceState) cancelAllLocal() error {
	_, err := s.channel.Invoke("cancelAll", nil)
	return err
}

func (s *notificationServiceState) setBadge(count int) error {
	_, err := s.channel.Invoke("setBadge", map[string]any{"count": count})
	return err
}

func (s *notificationServiceState) receivedChannel() <-chan NotificationEvent {
	return s.receivedCh
}

func (s *notificationServiceState) openChannel() <-chan NotificationOpen {
	return s.openedCh
}

func (s *notificationServiceState) tokenChannel() <-chan DeviceToken {
	return s.tokensCh
}

func (s *notificationServiceState) errorChannel() <-chan NotificationError {
	return s.errorsCh
}

func (s *notificationServiceState) permissionChannel() <-chan PermissionStatus {
	return s.permissionsCh
}

func parsePermissionStatus(result any) PermissionStatus {
	if m, ok := result.(map[string]any); ok {
		if status := parseString(m["status"]); status != "" {
			return PermissionStatus(status)
		}
	}
	return PermissionStatusUnknown
}

func parseNotificationEvent(data any) (NotificationEvent, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationEvent{}, false
	}
	return NotificationEvent{
		ID:           parseString(m["id"]),
		Title:        parseString(m["title"]),
		Body:         parseString(m["body"]),
		Data:         parseMap(m["data"]),
		Timestamp:    parseTime(m["timestamp"]),
		IsForeground: parseBool(m["isForeground"]),
		Source:       parseString(m["source"]),
	}, true
}

func parseNotificationOpen(data any) (NotificationOpen, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationOpen{}, false
	}
	return NotificationOpen{
		ID:        parseString(m["id"]),
		Action:    parseString(m["action"]),
		Source:    parseString(m["source"]),
		Data:      parseMap(m["data"]),
		Timestamp: parseTime(m["timestamp"]),
	}, true
}

func parsePermissionStatusEvent(data any) (PermissionStatus, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return PermissionStatusUnknown, false
	}
	status := PermissionStatus(parseString(m["status"]))
	if status == "" {
		return PermissionStatusUnknown, false
	}
	return status, true
}

func parseDeviceToken(data any) (DeviceToken, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return DeviceToken{}, false
	}
	return DeviceToken{
		Platform:  parseString(m["platform"]),
		Token:     parseString(m["token"]),
		Timestamp: parseTime(m["timestamp"]),
		IsRefresh: parseBool(m["isRefresh"]),
	}, true
}

func parseNotificationError(data any) (NotificationError, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return NotificationError{}, false
	}
	return NotificationError{
		Code:     parseString(m["code"]),
		Message:  parseString(m["message"]),
		Platform: parseString(m["platform"]),
	}, true
}

func parseString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func parseBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

func parseMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	if m, ok := value.(map[any]any); ok {
		converted := make(map[string]any, len(m))
		for key, val := range m {
			if keyString, ok := key.(string); ok {
				converted[keyString] = val
			}
		}
		return converted
	}
	return nil
}

func parseTime(value any) time.Time {
	var millis int64
	switch v := value.(type) {
	case int64:
		millis = v
	case int:
		millis = int64(v)
	case int32:
		millis = int64(v)
	case float64:
		millis = int64(v)
	case float32:
		millis = int64(v)
	case uint64:
		millis = int64(v)
	case uint32:
		millis = int64(v)
	case uint:
		millis = int64(v)
	case int16:
		millis = int64(v)
	case int8:
		millis = int64(v)
	case uint16:
		millis = int64(v)
	case uint8:
		millis = int64(v)
	default:
		return time.Time{}
	}
	return time.UnixMilli(millis)
}
