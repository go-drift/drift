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
	// KindBuild indicates a build-time widget error.
	KindBuild
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
	case KindBuild:
		return "build"
	default:
		return "unknown"
	}
}

// DriftError represents a structured error in the Drift framework.
type DriftError struct {
	// Op is the operation that failed (e.g., "graphics.DefaultFontManager").
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

// LayoutIssue is a panic value used for constraint violations and other
// developer-facing layout mistakes. Recovery code detects this type and
// presents a calmer "layout issue" screen instead of a crash error.
type LayoutIssue struct {
	Message string
}

func (e LayoutIssue) Error() string { return e.Message }

// BoundaryError represents a failure caught by an ErrorBoundary or the global
// panic recovery in the engine. This is a unified error type that covers all phases.
//
// Possible Phase values:
//   - "build": panic during widget Build()
//   - "layout": panic during RenderObject layout
//   - "paint": panic during RenderObject paint
//   - "hittest": panic during hit testing
//   - "frame": panic during frame processing (dispatch callbacks, animations, etc.)
//   - "pointer": panic during pointer/gesture event handling
type BoundaryError struct {
	// Phase is the phase where the error occurred.
	Phase string
	// Widget is the type name of the widget that failed (for build errors).
	Widget string
	// RenderObject is the type name of the render object that failed (for layout/paint/hittest errors).
	RenderObject string
	// IsLayoutIssue indicates this is a developer layout mistake, not a crash.
	IsLayoutIssue bool
	// Recovered is the panic value (nil for regular errors).
	Recovered any
	// Err is the underlying error (nil for panics).
	Err error
	// StackTrace contains the call stack at the time of the error.
	StackTrace string
	// Timestamp is when the error occurred.
	Timestamp time.Time
}

func (e *BoundaryError) Error() string {
	if e.IsLayoutIssue {
		if li, ok := e.Recovered.(LayoutIssue); ok {
			return li.Message
		}
	}
	typeName := e.Widget
	if typeName == "" {
		typeName = e.RenderObject
	}
	if e.Recovered != nil {
		if typeName != "" {
			return fmt.Sprintf("panic in %s (%s): %v", typeName, e.Phase, e.Recovered)
		}
		// No type info - just show the panic message (it should be self-explanatory)
		return fmt.Sprintf("%v", e.Recovered)
	}
	if e.Err != nil {
		if typeName != "" {
			return fmt.Sprintf("error in %s (%s): %v", typeName, e.Phase, e.Err)
		}
		return fmt.Sprintf("%v", e.Err)
	}
	if typeName != "" {
		return fmt.Sprintf("unknown error in %s (%s)", typeName, e.Phase)
	}
	return "unknown error"
}

func (e *BoundaryError) Unwrap() error {
	return e.Err
}

// ErrorHandler receives errors reported by the Drift framework.
type ErrorHandler interface {
	// HandleError is called when an error occurs.
	HandleError(err *DriftError)
	// HandlePanic is called when a panic is recovered.
	HandlePanic(err *PanicError)
	// HandleBoundaryError is called when an ErrorBoundary catches an error
	// or a panic is caught by global recovery.
	HandleBoundaryError(err *BoundaryError)
}
