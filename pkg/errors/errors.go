// Package errors provides structured error handling for the Drift framework.
package errors

import (
	"fmt"
	"time"
)

// ErrorKind identifies the category of an error.
type ErrorKind int

const (
	// KindUnknown indicates an error of unknown type.
	KindUnknown ErrorKind = iota
	// KindPlatform indicates a platform channel or native bridge error.
	KindPlatform
	// KindParsing indicates an event parsing failure.
	KindParsing
	// KindInit indicates an initialization error.
	KindInit
	// KindRender indicates a rendering error.
	KindRender
	// KindPanic indicates a recovered panic.
	KindPanic
)

func (k ErrorKind) String() string {
	switch k {
	case KindPlatform:
		return "platform"
	case KindParsing:
		return "parsing"
	case KindInit:
		return "init"
	case KindRender:
		return "render"
	case KindPanic:
		return "panic"
	default:
		return "unknown"
	}
}

// DriftError represents a structured error in the Drift framework.
type DriftError struct {
	// Op is the operation that failed (e.g., "rendering.DefaultFontManager").
	Op string
	// Kind categorizes the error.
	Kind ErrorKind
	// Err is the underlying error.
	Err error
	// Channel is the platform channel name, if applicable.
	Channel string
	// StackTrace contains the call stack at the time of the error.
	StackTrace string
	// Timestamp is when the error occurred.
	Timestamp time.Time
}

func (e *DriftError) Error() string {
	if e.Channel != "" {
		return fmt.Sprintf("%s [%s] channel=%s: %v", e.Op, e.Kind, e.Channel, e.Err)
	}
	return fmt.Sprintf("%s [%s]: %v", e.Op, e.Kind, e.Err)
}

func (e *DriftError) Unwrap() error {
	return e.Err
}

// PanicError represents a recovered panic.
type PanicError struct {
	// Op is the operation that panicked (e.g., "engine.HandlePointer").
	Op string
	// Value is the value passed to panic().
	Value any
	// StackTrace contains the call stack at the time of the panic.
	StackTrace string
	// Timestamp is when the panic occurred.
	Timestamp time.Time
}

func (e *PanicError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("panic in %s: %v", e.Op, e.Value)
	}
	return fmt.Sprintf("panic: %v", e.Value)
}

// ParseError represents a failure to parse event data.
type ParseError struct {
	// Channel is the platform channel that received the event.
	Channel string
	// DataType is the expected type name.
	DataType string
	// Got is the actual data received.
	Got any
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse %s from channel %s: got %T", e.DataType, e.Channel, e.Got)
}

// ErrorHandler receives errors reported by the Drift framework.
type ErrorHandler interface {
	// HandleError is called when an error occurs.
	HandleError(err *DriftError)
	// HandlePanic is called when a panic is recovered.
	HandlePanic(err *PanicError)
}
