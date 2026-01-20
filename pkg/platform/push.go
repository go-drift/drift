package platform

import (
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

var pushService = newPushService()

// PushToken represents a device push notification token.
type PushToken struct {
	Platform  string
	Token     string
	Timestamp time.Time
}

// PushError represents a push notification error.
type PushError struct {
	Code    string
	Message string
}

// RegisterForPush registers the device for push notifications.
func RegisterForPush() error {
	return pushService.register()
}

// GetPushToken returns the current push token if available.
func GetPushToken() (string, error) {
	return pushService.getToken()
}

// PushTokenUpdates returns a channel that receives token updates.
func PushTokenUpdates() <-chan PushToken {
	return pushService.tokenChannel()
}

// SubscribeToTopic subscribes to a push notification topic.
func SubscribeToTopic(topic string) error {
	return pushService.subscribeToTopic(topic)
}

// UnsubscribeFromTopic unsubscribes from a push notification topic.
func UnsubscribeFromTopic(topic string) error {
	return pushService.unsubscribeFromTopic(topic)
}

// DeletePushToken deletes the current push token.
func DeletePushToken() error {
	return pushService.deleteToken()
}

// PushErrors returns a channel that receives push-related errors.
func PushErrors() <-chan PushError {
	return pushService.errorChannel()
}

type pushServiceState struct {
	channel *MethodChannel
	tokens  *EventChannel
	errors  *EventChannel
	tokenCh chan PushToken
	errorCh chan PushError
}

func newPushService() *pushServiceState {
	service := &pushServiceState{
		channel: NewMethodChannel("github.com/go-drift/drift/push"),
		tokens:  NewEventChannel("github.com/go-drift/drift/push/token"),
		errors:  NewEventChannel("github.com/go-drift/drift/push/error"),
		tokenCh: make(chan PushToken, 4),
		errorCh: make(chan PushError, 4),
	}

	service.tokens.Listen(EventHandler{
		OnEvent: func(data any) {
			token, ok := parsePushToken(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "push.parseToken",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/push/token",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/push/token",
						DataType: "PushToken",
						Got:      data,
					},
				})
				return
			}
			service.tokenCh <- token
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "push.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/push/token",
				Err:     err,
			})
		},
	})

	service.errors.Listen(EventHandler{
		OnEvent: func(data any) {
			pushErr, ok := parsePushError(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "push.parseError",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/push/error",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/push/error",
						DataType: "PushError",
						Got:      data,
					},
				})
				return
			}
			service.errorCh <- pushErr
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "push.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/push/error",
				Err:     err,
			})
		},
	})

	return service
}

func (s *pushServiceState) register() error {
	_, err := s.channel.Invoke("register", nil)
	return err
}

func (s *pushServiceState) getToken() (string, error) {
	result, err := s.channel.Invoke("getToken", nil)
	if err != nil {
		return "", err
	}
	if m, ok := result.(map[string]any); ok {
		return parseString(m["token"]), nil
	}
	return "", nil
}

func (s *pushServiceState) subscribeToTopic(topic string) error {
	_, err := s.channel.Invoke("subscribeToTopic", map[string]any{
		"topic": topic,
	})
	return err
}

func (s *pushServiceState) unsubscribeFromTopic(topic string) error {
	_, err := s.channel.Invoke("unsubscribeFromTopic", map[string]any{
		"topic": topic,
	})
	return err
}

func (s *pushServiceState) deleteToken() error {
	_, err := s.channel.Invoke("deleteToken", nil)
	return err
}

func (s *pushServiceState) tokenChannel() <-chan PushToken {
	return s.tokenCh
}

func (s *pushServiceState) errorChannel() <-chan PushError {
	return s.errorCh
}

func parsePushToken(data any) (PushToken, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return PushToken{}, false
	}
	return PushToken{
		Platform:  parseString(m["platform"]),
		Token:     parseString(m["token"]),
		Timestamp: parseTime(m["timestamp"]),
	}, true
}

func parsePushError(data any) (PushError, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return PushError{}, false
	}
	return PushError{
		Code:    parseString(m["code"]),
		Message: parseString(m["message"]),
	}, true
}
