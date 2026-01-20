// Package platform provides platform channel communication between Go and native code.
// It enables Go code to call native APIs (clipboard, haptics, etc.) and receive
// events from the native platform (lifecycle changes, sensor data, etc.).
package platform

import (
	"encoding/json"
	"errors"
)

// MessageCodec encodes and decodes messages for platform channel communication.
type MessageCodec interface {
	// Encode converts a Go value to bytes for transmission to native code.
	Encode(value any) ([]byte, error)

	// Decode converts bytes received from native code to a Go value.
	Decode(data []byte) (any, error)
}

// JsonCodec implements MessageCodec using JSON encoding.
// JSON prioritizes interoperability and minimal native dependencies.
type JsonCodec struct{}

// Encode serializes the value to JSON bytes.
func (c JsonCodec) Encode(value any) ([]byte, error) {
	return json.Marshal(value)
}

// Decode deserializes JSON bytes to a Go value.
func (c JsonCodec) Decode(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DecodeInto deserializes JSON bytes into a specific type.
func (c JsonCodec) DecodeInto(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// DefaultCodec is the codec used by platform channels.
var DefaultCodec MessageCodec = JsonCodec{}

// Standard errors for platform channel operations.
var (
	ErrChannelNotFound     = errors.New("platform channel not found")
	ErrMethodNotFound      = errors.New("method not implemented")
	ErrInvalidArguments    = errors.New("invalid arguments")
	ErrPlatformUnavailable = errors.New("platform feature unavailable")
	ErrTimeout             = errors.New("operation timed out")
	ErrViewTypeNotFound    = errors.New("platform view type not registered")
)

// ChannelError represents an error returned from native code.
type ChannelError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *ChannelError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

// NewChannelError creates a new ChannelError with the given code and message.
func NewChannelError(code, message string) *ChannelError {
	return &ChannelError{Code: code, Message: message}
}

// NewChannelErrorWithDetails creates a new ChannelError with additional details.
func NewChannelErrorWithDetails(code, message string, details any) *ChannelError {
	return &ChannelError{Code: code, Message: message, Details: details}
}
