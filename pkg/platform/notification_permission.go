package platform

import "context"

// NotificationPermission extends Permission with notification-specific options.
type NotificationPermission interface {
	Permission

	// RequestWithOptions prompts for notification permission with iOS-specific options.
	// Uses the provided values verbatim - zero values mean that capability is NOT requested.
	// For default behavior (Alert, Sound, Badge all enabled), use Request() instead.
	// Options are ignored on Android.
	RequestWithOptions(ctx context.Context, opts NotificationPermissionOptions) (PermissionResult, error)
}

// NotificationPermissionOptions configures notification capabilities (iOS only).
// Zero values mean the capability is NOT requested. Use Request() for defaults.
type NotificationPermissionOptions struct {
	Alert       bool // Visible notifications (banners, alerts).
	Sound       bool // Notification sounds.
	Badge       bool // Badge count on app icon.
	Provisional bool // Quiet delivery to Notification Center (iOS 12+).
}
