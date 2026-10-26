package platform

import "errors"

// Sentinel errors for platform operations.
var (
	// ErrDisposed is returned when a method is called on a controller that
	// has already been disposed, or whose underlying resource failed to
	// create. Check for this with [errors.Is] when you need to distinguish
	// a no-op from a successful operation.
	ErrDisposed = errors.New("platform: controller disposed")
)
