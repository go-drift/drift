package platform

import "context"

// Permission provides access to a runtime permission for a platform feature.
// Use Status to check current state, Request to prompt the user, and Listen
// to observe changes.
//
// Context: only Request takes ctx because it owns a caller-visible Go-side
// wait that ctx can abandon (the call blocks on a result event from native
// and can be unblocked early on cancellation). Status, IsGranted, IsDenied,
// and ShouldShowRationale are simple native invocations, so the high-level
// permission API does not expose ctx for them.
type Permission interface {
	// Status returns the current permission status.
	Status() (PermissionResult, error)

	// Request prompts the user for permission and blocks until they respond
	// or ctx is canceled. If already in a terminal state, returns immediately
	// without showing a dialog.
	//
	// Cancellation: returns ErrCanceled or ErrTimeout (via the package's
	// stable error contract) on ctx cancellation; the underlying native
	// dialog continues to completion in the background.
	Request(ctx context.Context) (PermissionResult, error)

	// IsGranted returns true if permission is granted.
	// Best-effort convenience: returns false on any error. Use Status for
	// precise error handling when error details matter.
	IsGranted() bool

	// IsDenied returns true if permission is denied or permanently denied.
	// Best-effort convenience: returns false on any error. Use Status for
	// precise error handling when error details matter.
	IsDenied() bool

	// ShouldShowRationale returns whether to show a rationale before requesting.
	// Android-specific; always returns (false, nil) on iOS.
	ShouldShowRationale() (bool, error)

	// Listen subscribes to permission status changes.
	// Returns an unsubscribe function. Multiple listeners receive all events.
	Listen(handler func(PermissionResult)) (unsubscribe func())
}
