package errors

import (
	"fmt"
	"os"
)

// LogHandler is an ErrorHandler that logs errors to stderr.
type LogHandler struct {
	// Verbose enables detailed output including stack traces.
	Verbose bool
}

// HandleError logs a DriftError to stderr.
func (h *LogHandler) HandleError(err *DriftError) {
	if err == nil {
		return
	}
	if h.Verbose {
		fmt.Fprintf(os.Stderr, "[drift error] %s [%s]", err.Op, err.Kind)
		if err.Channel != "" {
			fmt.Fprintf(os.Stderr, " channel=%s", err.Channel)
		}
		fmt.Fprintf(os.Stderr, ": %v\n", err.Err)
		if err.StackTrace != "" {
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", err.StackTrace)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[drift error] %s: %v\n", err.Op, err.Err)
	}
}

// HandlePanic logs a PanicError to stderr.
func (h *LogHandler) HandlePanic(err *PanicError) {
	if err == nil {
		return
	}
	if err.Op != "" {
		fmt.Fprintf(os.Stderr, "[drift panic] %s: %v\n", err.Op, err.Value)
	} else {
		fmt.Fprintf(os.Stderr, "[drift panic] %v\n", err.Value)
	}
	if h.Verbose && err.StackTrace != "" {
		fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", err.StackTrace)
	}
}
