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

	// ErrBlocksUIThread indicates the UI thread called a native method that
	// replies asynchronously. The UI thread cannot wait for such a reply
	// (the reply may need the UI thread), so native refuses the call before
	// running it, every time. Call the method from a goroutine and publish
	// its result, for example through a Signal.
	ErrBlocksUIThread = errors.New("method replies asynchronously; call it from a goroutine, not the UI thread")

	// ErrReplyDropped indicates native discarded a method call's reply
	// without answering it, a bug in the native handler.
	ErrReplyDropped = errors.New("native handler dropped the call without replying")
)

// channelErrorCodes are the wire codes of the standard errors, shared by
// both directions: a Go error crossing to native carries its code (see
// [AsChannelError]), and a native error with a standard code matches its
// sentinel under [errors.Is].
var channelErrorCodes = map[error]string{
	ErrChannelNotFound:     "channel_not_found",
	ErrMethodNotFound:      "method_not_found",
	ErrInvalidArguments:    "invalid_arguments",
	ErrPlatformUnavailable: "platform_unavailable",
	ErrTimeout:             "timeout",
	ErrViewTypeNotFound:    "view_type_not_found",
	ErrBlocksUIThread:      "blocks_ui_thread",
	ErrReplyDropped:        "reply_dropped",
}

// AsChannelError converts a Go error to the ChannelError sent to native:
// a ChannelError as is, a standard error with its code, anything else as
// "go_error".
func AsChannelError(err error) *ChannelError {
	if err == nil {
		return NewChannelError("go_error", "unknown error")
	}
	var channelErr *ChannelError
	if errors.As(err, &channelErr) {
		return channelErr
	}
	for sentinel, code := range channelErrorCodes {
		if errors.Is(err, sentinel) {
			return NewChannelError(code, err.Error())
		}
	}
	return NewChannelError("go_error", err.Error())
}

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

// Is reports whether e carries target's wire code, so a native error such
// as "blocks_ui_thread" matches [ErrBlocksUIThread] under [errors.Is].
func (e *ChannelError) Is(target error) bool {
	// A loop, not a map lookup: target may be of a non-comparable type,
	// which would panic as a map key.
	for sentinel, code := range channelErrorCodes {
		if sentinel == target {
			return e.Code == code
		}
	}
	return false
}

// NewChannelError creates a new ChannelError with the given code and message.
func NewChannelError(code, message string) *ChannelError {
	return &ChannelError{Code: code, Message: message}
}

// NewChannelErrorWithDetails creates a new ChannelError with additional details.
func NewChannelErrorWithDetails(code, message string, details any) *ChannelError {
	return &ChannelError{Code: code, Message: message, Details: details}
}
