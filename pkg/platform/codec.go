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
	// ErrChannelNotFound indicates the requested platform channel does not exist.
	ErrChannelNotFound = errors.New("platform channel not found")

	// ErrMethodNotFound indicates the method is not implemented on the native side.
	ErrMethodNotFound = errors.New("method not implemented")

	// ErrInvalidArguments indicates the arguments passed to the method were invalid.
	ErrInvalidArguments = errors.New("invalid arguments")

	// ErrPlatformUnavailable indicates the platform feature is not available
	// (e.g., hardware not present, OS version too old).
	ErrPlatformUnavailable = errors.New("platform feature unavailable")

	// ErrTimeout indicates the operation exceeded its deadline. For permission requests,
	// this means the user did not respond to the dialog within the timeout period.
	ErrTimeout = errors.New("operation timed out")

	// ErrCanceled indicates the operation was canceled via context cancellation.
	ErrCanceled = errors.New("operation was canceled")

	// ErrViewTypeNotFound indicates the platform view type is not registered.
	ErrViewTypeNotFound = errors.New("platform view type not registered")
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
