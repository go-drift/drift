package platform

import (
	"context"
	"errors"
	"maps"

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
// Initialized eagerly so that permissions constructed at package init all
// share the same channel state.
var (
	permissionsChannel = NewMethodChannel("drift/permissions")
	permissionChanges  = NewStream(
		"drift/permissions/changes",
		NewEventChannel("drift/permissions/changes"),
		parsePermissionChange,
	)
)

// NewPermission returns the permission of the given kind, such as
// "location". Native code registers each kind: Drift registers
// "notifications", and plugins register theirs (DriftPluginHost
// registerPermission on iOS and Android). Plugins call this from their
// runtime package and expose the result; apps use those values.
//
// Calls on a kind no native code registered fail with
// [ErrPermissionNotRegistered]: the plugin providing it is missing from
// drift.yaml, or not configured for that kind.
func NewPermission(kind string) Permission {
	return newPermission(kind)
}

// permission is the single concrete implementation of Permission.
// notificationPermission wraps it to add RequestWithOptions.
//
// Context usage: only Request takes ctx because it waits for the user.
// Status / IsGranted / IsDenied / ShouldShowRationale reply in place, so
// the high-level permission API does not expose ctx for them.
type permission struct {
	name string
}

func newPermission(name string) *permission {
	if name == "" {
		panic("platform: NewPermission with an empty kind")
	}
	return &permission{name: name}
}

// Status returns the current status of the permission.
func (p *permission) Status() (PermissionResult, error) {
	result, err := permissionsChannel.Invoke(context.Background(), "check", map[string]any{"permission": p.name})
	if err != nil {
		return PermissionResultUnknown, err
	}
	return parsePermissionStatus(p.name, result)
}

// Request prompts the user for the permission if needed and returns the
// resulting status, or fails when ctx is done.
func (p *permission) Request(ctx context.Context) (PermissionResult, error) {
	return p.request(ctx, nil)
}

// request asks native to request the permission, with kind-specific
// options, and returns the status native replies with. Native replies once
// the user has answered (or at once when no prompt is needed).
//
// Native shows one prompt per kind at a time: a request made while one is
// up waits there for that prompt's answer. Native serializes on the main
// thread rather than Go behind a lock, since a lock held across Invoke
// would block a UI-thread caller (which the prompt's reply needs) and
// ignore a waiting caller's ctx.
func (p *permission) request(ctx context.Context, options map[string]any) (PermissionResult, error) {
	args := map[string]any{"permission": p.name}
	maps.Copy(args, options)
	result, err := permissionsChannel.Invoke(ctx, "request", args)
	if err != nil {
		return PermissionResultUnknown, normalizePermissionCtxErr(err)
	}
	return parsePermissionStatus(p.name, result)
}

// IsGranted returns true if the permission is currently granted.
func (p *permission) IsGranted() bool {
	status, err := p.Status()
	if err != nil {
		return false
	}
	return status == PermissionGranted
}

// IsDenied returns true if the permission is denied or permanently denied.
func (p *permission) IsDenied() bool {
	status, err := p.Status()
	if err != nil {
		return false
	}
	return status == PermissionDenied || status == PermissionPermanentlyDenied
}

// ShouldShowRationale returns whether the app should show a rationale before
// requesting this permission. Android-specific; always returns (false, nil) on iOS.
// Native bridge errors are returned directly; a malformed payload returns (false, nil).
func (p *permission) ShouldShowRationale() (bool, error) {
	result, err := permissionsChannel.Invoke(context.Background(), "shouldShowRationale", map[string]any{"permission": p.name})
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

// newNotificationPerm constructs the notifications permission, a kind
// Drift itself registers natively.
func newNotificationPerm() *notificationPermission {
	return &notificationPermission{permission: newPermission("notifications")}
}

// RequestWithOptions prompts for notification permission with iOS-specific options.
// Zero values mean the capability is NOT requested. On Android, options are ignored.
func (n *notificationPermission) RequestWithOptions(ctx context.Context, opts NotificationPermissionOptions) (PermissionResult, error) {
	return n.request(ctx, map[string]any{
		"alert":       opts.Alert,
		"sound":       opts.Sound,
		"badge":       opts.Badge,
		"provisional": opts.Provisional,
	})
}

// normalizePermissionCtxErr maps raw context.Canceled / context.DeadlineExceeded
// (returned from ctx-aware Invoke calls) to the platform's stable sentinels.
// Other errors pass through unchanged, so callers can use
// `errors.Is(err, ErrCanceled)`.
func normalizePermissionCtxErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	return err
}

// permissionChange is the typed payload of the "drift/permissions/changes" event channel.
type permissionChange struct {
	Permission string
	Result     PermissionResult
}

// parsePermissionStatus extracts the status from a {"status": "..."} reply
// for kind. A malformed reply is a ParseError: native and Go disagree on the
// wire format. Unknown status strings pass through verbatim.
func parsePermissionStatus(kind string, result any) (PermissionResult, error) {
	if m, ok := result.(map[string]any); ok {
		if status := parseString(m["status"]); status != "" {
			return PermissionResult(status), nil
		}
	}
	return PermissionResultUnknown, &drifterrors.ParseError{
		Channel:  "drift/permissions",
		DataType: "status of " + kind,
		Got:      result,
	}
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
func OpenAppSettings() error {
	_, err := permissionsChannel.Invoke(context.Background(), "openSettings", nil)
	return err
}
