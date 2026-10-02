package platform

import "context"

// Permission provides access to a runtime permission for a platform feature.
// Use Status to check current state, Request to prompt the user, and Listen
// to observe changes. Plugins provide permissions (see [NewPermission]);
// Drift provides the notification permission ([Notifications]).
//
// Every method fails with [ErrPermissionNotRegistered] when no native code
// registered the kind, and on Android with [ErrPermissionNotDeclared] when
// the manifest lacks a permission the kind needs.
//
// Context: only Request takes ctx because it waits for the user. Status,
// IsGranted, IsDenied, and ShouldShowRationale reply in place, so the
// high-level permission API does not expose ctx for them.
type Permission interface {
	// Status returns the current permission status.
	Status() (PermissionResult, error)

	// Request prompts the user for permission if needed and returns the
	// resulting status once they respond. When no prompt can change the
	// status (granted, permanently denied, restricted), it returns at once
	// without showing a dialog.
	//
	// Request waits for the user, so call it from a goroutine: from the UI
	// thread it fails with [ErrBlocksUIThread] before any prompt shows.
	//
	// Cancellation: returns ErrCanceled or ErrTimeout on ctx cancellation;
	// the native dialog stays up until the user answers it.
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
