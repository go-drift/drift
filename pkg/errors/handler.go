package errors

import (
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	// DefaultHandler is the global error handler.
	// It defaults to LogHandler with verbose=false.
	DefaultHandler ErrorHandler = &LogHandler{}

	handlerMu sync.RWMutex
)

// SetHandler configures the global error handler.
// Pass nil to restore the default LogHandler.
func SetHandler(h ErrorHandler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	if h == nil {
		DefaultHandler = &LogHandler{}
	} else {
		DefaultHandler = h
	}
}

// getHandler returns the current error handler.
func getHandler() ErrorHandler {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return DefaultHandler
}

// Report sends an error to the global handler.
// If err.Timestamp is zero, it is set to the current time.
func Report(err *DriftError) {
	if err == nil {
		return
	}
	if err.Timestamp.IsZero() {
		err.Timestamp = time.Now()
	}
	if h := getHandler(); h != nil {
		h.HandleError(err)
	}
}

// ReportPanic sends a panic error to the global handler.
func ReportPanic(err *PanicError) {
	if err == nil {
		return
	}
	if h := getHandler(); h != nil {
		h.HandlePanic(err)
	}
}

// Recover is a helper for deferred panic recovery.
// Usage: defer errors.Recover("operation.name")
func Recover(op string) {
	if r := recover(); r != nil {
		ReportPanic(&PanicError{
			Op:         op,
			Value:      r,
			StackTrace: CaptureStack(),
			Timestamp:  time.Now(),
		})
	}
}

// RecoverWithCallback is like Recover but also calls the provided callback
// with the panic value after reporting it.
func RecoverWithCallback(op string, callback func(r any)) {
	if r := recover(); r != nil {
		ReportPanic(&PanicError{
			Op:         op,
			Value:      r,
			StackTrace: CaptureStack(),
			Timestamp:  time.Now(),
		})
		if callback != nil {
			callback(r)
		}
	}
}

// CaptureStack returns the current call stack as a string.
// It skips the first few frames to exclude the CaptureStack call itself.
func CaptureStack() string {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(3, pcs[:])
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder
	for {
		frame, more := frames.Next()
		sb.WriteString(frame.Function)
		sb.WriteString("\n\t")
		sb.WriteString(frame.File)
		sb.WriteString(":")
		sb.WriteString(itoa(frame.Line))
		sb.WriteString("\n")
		if !more {
			break
		}
	}
	return sb.String()
}

// itoa converts an integer to a string without allocating.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
