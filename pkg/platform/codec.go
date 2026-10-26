package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

	// ErrNotEncodable indicates native produced a reply or an event that
	// is not JSON-like (a NaN, an Infinity, or a type outside null,
	// booleans, numbers, strings, lists and string-keyed maps), a bug in
	// the native handler. A method call fails with it; an event stream
	// delivers it to OnError in place of the event. The message names the
	// call or channel and the offending part of the value.
	ErrNotEncodable = errors.New("native value is not JSON-like")

	// ErrPermissionNotRegistered indicates no native code registered the
	// permission kind: the plugin providing it is missing from drift.yaml,
	// or not configured for that kind.
	ErrPermissionNotRegistered = errors.New("permission kind not registered")

	// ErrPermissionNotDeclared indicates the Android manifest does not
	// declare a permission the kind needs, so Android would deny it without
	// asking. Add the plugin that declares it to drift.yaml.
	ErrPermissionNotDeclared = errors.New("permission not declared in the Android manifest")
)

// transportErrorCodes are the wire codes of the errors any channel can
// answer, which the transport and Drift's native hosts produce. Shared by
// both directions: a Go error crossing to native carries its code (see
// [AsChannelError]), and a native error with one of these codes matches its
// sentinel under [errors.Is]. A feature's own codes belong to its channel
// ([ErrorCodes]).
var transportErrorCodes = map[error]string{
	ErrChannelNotFound:  "channel_not_found",
	ErrMethodNotFound:   "method_not_found",
	ErrInvalidArguments: "invalid_arguments",
	ErrBlocksUIThread:   "blocks_ui_thread",
	ErrReplyDropped:     "reply_dropped",
	ErrNotEncodable:     "not_encodable",
}

// ErrorCodes maps the wire codes a channel's native side answers with to
// the sentinel errors they match. Declare a channel's codes once, with
// [MethodChannel.WithErrors] (and [EventChannel.WithErrors] for its
// stream): every error the channel returns or streams is then a
// [*ChannelError] that matches its code's sentinel under [errors.Is], with
// the native message kept.
//
//	var ErrDenied = errors.New("acme: permission denied")
//
//	var errorCodes = platform.ErrorCodes{"permission_denied": ErrDenied}
//
//	var channel = platform.NewMethodChannel("acme/camera").WithErrors(errorCodes)
//
// A code must not repeat a transport code (method_not_found and the like),
// and each sentinel must be comparable (as errors.New's are).
type ErrorCodes map[string]error

// NewError returns the error native would send with code and message, for
// Go code that receives codes some other way (in an event's payload, say).
func (c ErrorCodes) NewError(code, message string) *ChannelError {
	return &ChannelError{Code: code, Message: message, codes: c}
}

// check panics, naming channel, on a code that repeats a transport code or
// a sentinel that is nil or not comparable.
func (c ErrorCodes) check(channel string) {
	for code, sentinel := range c {
		for _, transport := range transportErrorCodes {
			if code == transport {
				panic(fmt.Sprintf("platform: channel %q declares the transport error code %q", channel, code))
			}
		}
		if sentinel == nil || !reflect.TypeOf(sentinel).Comparable() {
			panic(fmt.Sprintf("platform: channel %q maps error code %q to %#v; want a comparable error such as errors.New's", channel, code, sentinel))
		}
	}
}

// attach returns err with c as its codes when it is a [*ChannelError], so
// it matches c's sentinels; other errors are returned as they are.
func (c ErrorCodes) attach(err error) error {
	channelErr, ok := err.(*ChannelError)
	if !ok || len(c) == 0 {
		return err
	}
	withCodes := *channelErr
	withCodes.codes = c
	return &withCodes
}

// AsChannelError converts a Go error to the ChannelError sent to native:
// a ChannelError as is, a transport error with its code, anything else as
// "go_error".
func AsChannelError(err error) *ChannelError {
	if err == nil {
		return NewChannelError("go_error", "unknown error")
	}
	var channelErr *ChannelError
	if errors.As(err, &channelErr) {
		return channelErr
	}
	for sentinel, code := range transportErrorCodes {
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

	// codes are the channel's own codes (ErrorCodes), set by the channel
	// the error came from.
	codes ErrorCodes
}

func (e *ChannelError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

// Is reports whether e carries target's wire code: a transport code, such
// as "blocks_ui_thread" for [ErrBlocksUIThread], or one its channel
// declared ([ErrorCodes]).
func (e *ChannelError) Is(target error) bool {
	// Loops, not map lookups keyed by target: target may be of a
	// non-comparable type, which would panic as a map key. Comparing it
	// with a comparable sentinel is safe.
	for sentinel, code := range transportErrorCodes {
		if sentinel == target {
			return e.Code == code
		}
	}
	if sentinel, ok := e.codes[e.Code]; ok {
		return sentinel == target
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
