package platform

import "context"

// Permission provides access to a runtime permission for a platform feature.
// Use Status to check current state, Request to prompt the user, and Listen
// to observe changes.
//
// Context usage: ctx is honored on Request for cancellation and timeout.
// The non-blocking methods (Status, IsGranted, IsDenied, ShouldShowRationale)
// accept ctx for API consistency but route through MethodChannel.Invoke,
// which does not currently support cancellation.
type Permission interface {
	// Status returns the current permission status.
	Status(ctx context.Context) (PermissionResult, error)

	// Request prompts the user for permission and blocks until they respond
	// or ctx is canceled. If already in a terminal state, returns immediately
	// without showing a dialog.
	Request(ctx context.Context) (PermissionResult, error)

	// IsGranted returns true if permission is granted.
	// Best-effort convenience: returns false on any error. Use Status for
	// precise error handling when error details matter.
	IsGranted(ctx context.Context) bool

	// IsDenied returns true if permission is denied or permanently denied.
	// Best-effort convenience: returns false on any error. Use Status for
	// precise error handling when error details matter.
	IsDenied(ctx context.Context) bool

	// ShouldShowRationale returns whether to show a rationale before requesting.
	// Android-specific; always returns (false, nil) on iOS.
	ShouldShowRationale(ctx context.Context) (bool, error)

	// Listen subscribes to permission status changes.
	// Returns an unsubscribe function. Multiple listeners receive all events.
	Listen(handler func(PermissionResult)) (unsubscribe func())
}
