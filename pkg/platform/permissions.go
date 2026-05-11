package platform

import (
	"context"
	"sync"

	drifterrors "github.com/go-drift/drift/pkg/errors"
)

// PermissionResult represents the status of a permission.
type PermissionResult string

// Permission status constants.
const (
	// PermissionGranted indicates full access has been granted.
	PermissionGranted PermissionResult = "granted"

	// PermissionDenied indicates the user denied the permission. The app may request again.
	PermissionDenied PermissionResult = "denied"

	// PermissionPermanentlyDenied indicates the user denied with "don't ask again" (Android)
	// or denied twice (iOS). The app cannot request again; direct user to Settings.
	PermissionPermanentlyDenied PermissionResult = "permanently_denied"

	// PermissionRestricted indicates a system policy prevents granting (parental controls,
	// MDM, enterprise policy). The user cannot change this; no dialog will be shown.
	PermissionRestricted PermissionResult = "restricted"

	// PermissionLimited indicates partial access (iOS only). For Photos, this means the user
	// selected specific photos rather than granting full library access.
	PermissionLimited PermissionResult = "limited"

	// PermissionNotDetermined indicates the user has not yet been asked. Calling Request
	// will show the system permission dialog.
	PermissionNotDetermined PermissionResult = "not_determined"

	// PermissionProvisional indicates provisional notification permission (iOS only).
	// Notifications are delivered quietly to Notification Center without alerting the user.
	PermissionProvisional PermissionResult = "provisional"

	// PermissionResultUnknown indicates the status could not be determined.
	PermissionResultUnknown PermissionResult = "unknown"
)

// Shared platform channel and event stream used by every permission instance.
// Initialized eagerly so that service singletons constructed at package init
// (Camera, Location, Notifications, etc.) all share the same channel state.
var (
	permissionsChannel = NewMethodChannel("drift/permissions")
	permissionChanges  = NewStream(
		"drift/permissions/changes",
		NewEventChannel("drift/permissions/changes"),
		parsePermissionChange,
	)
)

// permission is the single concrete implementation of Permission used by every
// service that exposes one (camera, photos, contacts, microphone, calendar,
// location when-in-use, location_always). Notifications wraps it with
// notificationPermission to add RequestWithOptions.
//
// Context usage: ctx is honored on Request via [requestAndWait]. Status,
// IsGranted, IsDenied, and ShouldShowRationale accept ctx for API consistency
// but route through MethodChannel.Invoke, which does not currently support
// cancellation. The Permission interface documents this.
type permission struct {
	name      string
	requestMu sync.Mutex
}

// newPerm constructs a permission for the given native permission name
// (e.g. "camera", "location", "location_always", "notifications").
func newPerm(name string) *permission {
	return &permission{name: name}
}

// Status returns the current status of the permission.
func (p *permission) Status(ctx context.Context) (PermissionResult, error) {
	result, err := permissionsChannel.Invoke("check", map[string]any{"permission": p.name})
	if err != nil {
		return PermissionResultUnknown, err
	}
	return parsePermissionResult(result), nil
}

// Request prompts the user for the permission and blocks until they respond
// or the context is canceled.
func (p *permission) Request(ctx context.Context) (PermissionResult, error) {
	return requestAndWait(ctx, p, map[string]any{"permission": p.name})
}

// IsGranted returns true if the permission is currently granted.
func (p *permission) IsGranted(ctx context.Context) bool {
	status, err := p.Status(ctx)
	if err != nil {
		return false
	}
	return status == PermissionGranted
}

// IsDenied returns true if the permission is denied or permanently denied.
func (p *permission) IsDenied(ctx context.Context) bool {
	status, err := p.Status(ctx)
	if err != nil {
		return false
	}
	return status == PermissionDenied || status == PermissionPermanentlyDenied
}

// ShouldShowRationale returns whether the app should show a rationale before
// requesting this permission. Android-specific; always returns (false, nil) on iOS.
// Native bridge errors are returned directly; a malformed payload returns (false, nil).
func (p *permission) ShouldShowRationale(ctx context.Context) (bool, error) {
	result, err := permissionsChannel.Invoke("shouldShowRationale", map[string]any{"permission": p.name})
	if err != nil {
		return false, err
	}
	if m, ok := result.(map[string]any); ok {
		return parseBool(m["shouldShow"]), nil
	}
	return false, nil
}

// Listen subscribes to permission status changes for this permission.
// The handler is invoked only when the change event matches this permission's name.
func (p *permission) Listen(handler func(PermissionResult)) (unsubscribe func()) {
	return permissionChanges.Listen(func(change permissionChange) {
		if change.Permission == p.name {
			handler(change.Result)
		}
	})
}

// notificationPermission extends permission with iOS-specific request options.
type notificationPermission struct {
	*permission
}

// newNotificationPerm constructs the notifications permission.
func newNotificationPerm() *notificationPermission {
	return &notificationPermission{permission: newPerm("notifications")}
}

// RequestWithOptions prompts for notification permission with iOS-specific options.
// Zero values mean the capability is NOT requested. On Android, options are ignored.
func (n *notificationPermission) RequestWithOptions(ctx context.Context, opts NotificationPermissionOptions) (PermissionResult, error) {
	return requestAndWait(ctx, n.permission, map[string]any{
		"permission":  n.name,
		"alert":       opts.Alert,
		"sound":       opts.Sound,
		"badge":       opts.Badge,
		"provisional": opts.Provisional,
	})
}

// requestAndWait performs the request-and-wait dance shared by every
// permission Request implementation: serialize concurrent requests, fast-path
// when status is already terminal, subscribe to permission-change events
// filtered by name, invoke "request" on native, and resolve when the matching
// change arrives or ctx is canceled. On ctx cancellation, status is re-checked
// once in case the change event was missed.
func requestAndWait(ctx context.Context, p *permission, args map[string]any) (PermissionResult, error) {
	p.requestMu.Lock()
	defer p.requestMu.Unlock()

	currentStatus, err := p.Status(ctx)
	if err != nil {
		return PermissionResultUnknown, err
	}
	if isTerminalStatus(currentStatus) {
		return currentStatus, nil
	}

	resultChan := make(chan PermissionResult, 1)
	unsubscribe := permissionChanges.Listen(func(change permissionChange) {
		if change.Permission != p.name {
			return
		}
		select {
		case resultChan <- change.Result:
		default:
		}
	})
	defer unsubscribe()

	if _, err := permissionsChannel.Invoke("request", args); err != nil {
		return PermissionResultUnknown, err
	}

	select {
	case result := <-resultChan:
		return result, nil
	case <-ctx.Done():
		if finalStatus, err := p.Status(ctx); err == nil && isTerminalStatus(finalStatus) {
			return finalStatus, nil
		}
		if ctx.Err() == context.DeadlineExceeded {
			return PermissionResultUnknown, ErrTimeout
		}
		return PermissionResultUnknown, ErrCanceled
	}
}

// isTerminalStatus reports whether the status will not change as a result of
// showing a permission dialog.
func isTerminalStatus(status PermissionResult) bool {
	switch status {
	case PermissionGranted, PermissionPermanentlyDenied, PermissionRestricted,
		PermissionLimited, PermissionProvisional:
		return true
	default:
		return false
	}
}

// permissionChange is the typed payload of the "drift/permissions/changes" event channel.
type permissionChange struct {
	Permission string
	Result     PermissionResult
}

// parsePermissionResult extracts a PermissionResult from a {"status": "..."} map.
func parsePermissionResult(result any) PermissionResult {
	if m, ok := result.(map[string]any); ok {
		if status := parseString(m["status"]); status != "" {
			return PermissionResult(status)
		}
	}
	return PermissionResultUnknown
}

// parsePermissionChange parses a {"permission": "...", "status": "..."} map.
// Both fields must be non-empty; missing or malformed payloads return a typed
// ParseError so the surrounding Stream reports it. Unknown status strings are
// passed through verbatim to preserve forward compatibility with new native values.
func parsePermissionChange(data any) (permissionChange, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return permissionChange{}, &drifterrors.ParseError{
			Channel:  "drift/permissions/changes",
			DataType: "permissionChange",
			Got:      data,
		}
	}
	name := parseString(m["permission"])
	if name == "" {
		return permissionChange{}, &drifterrors.ParseError{
			Channel:  "drift/permissions/changes",
			DataType: "permissionChange.permission",
			Got:      data,
		}
	}
	status := parseString(m["status"])
	if status == "" {
		return permissionChange{}, &drifterrors.ParseError{
			Channel:  "drift/permissions/changes",
			DataType: "permissionChange.status",
			Got:      data,
		}
	}
	return permissionChange{
		Permission: name,
		Result:     PermissionResult(status),
	}, nil
}

// OpenAppSettings opens the system settings page for this app, where users can
// manage permissions manually. Use this when a permission is permanently denied
// and the app cannot request it again.
//
// On iOS, opens the Settings app to the app's settings page.
// On Android, opens the App Info screen in system settings.
//
// Cancellation: this call cannot be canceled today because MethodChannel.Invoke
// is synchronous. A ctx parameter will be added when the channel layer grows
// ctx support.
func OpenAppSettings() error {
	_, err := permissionsChannel.Invoke("openSettings", nil)
	return err
}
