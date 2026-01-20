package platform

import "github.com/go-drift/drift/pkg/errors"

// Permission types for runtime permission requests.
type Permission string

const (
	PermissionCamera         Permission = "camera"
	PermissionMicrophone     Permission = "microphone"
	PermissionPhotos         Permission = "photos"
	PermissionLocation       Permission = "location"
	PermissionLocationAlways Permission = "location_always"
	PermissionStorage        Permission = "storage"
	PermissionContacts       Permission = "contacts"
	PermissionCalendar       Permission = "calendar"
	PermissionNotifications  Permission = "notifications"
)

// PermissionResult represents the result of a permission request.
type PermissionResult string

const (
	PermissionGranted           PermissionResult = "granted"
	PermissionDenied            PermissionResult = "denied"
	PermissionPermanentlyDenied PermissionResult = "permanently_denied"
	PermissionRestricted        PermissionResult = "restricted"
	PermissionLimited           PermissionResult = "limited"
	PermissionNotDetermined     PermissionResult = "not_determined"
	PermissionProvisional       PermissionResult = "provisional"
	PermissionResultUnknown     PermissionResult = "unknown"
)

// PermissionChange represents a permission status change event.
type PermissionChange struct {
	Permission Permission
	Result     PermissionResult
}

var permissionService = newPermissionService()

// CheckPermission checks the current status of a permission.
func CheckPermission(permission Permission) (PermissionResult, error) {
	return permissionService.check(permission)
}

// RequestPermission requests a single runtime permission.
func RequestPermission(permission Permission) (PermissionResult, error) {
	return permissionService.request(permission)
}

// RequestPermissions requests multiple runtime permissions.
func RequestPermissions(permissions []Permission) (map[Permission]PermissionResult, error) {
	return permissionService.requestMultiple(permissions)
}

// OpenAppSettings opens the app settings page where users can manage permissions.
func OpenAppSettings() error {
	return permissionService.openSettings()
}

// ShouldShowPermissionRationale returns whether the app should show a rationale
// for requesting a permission. This is Android-specific and always returns false on iOS.
func ShouldShowPermissionRationale(permission Permission) bool {
	return permissionService.shouldShowRationale(permission)
}

// PermissionChanges returns a channel that receives permission status changes.
func PermissionChanges() <-chan PermissionChange {
	return permissionService.changeChannel()
}

type permissionServiceState struct {
	channel  *MethodChannel
	changes  *EventChannel
	changeCh chan PermissionChange
}

func newPermissionService() *permissionServiceState {
	service := &permissionServiceState{
		channel:  NewMethodChannel("github.com/go-drift/drift/permissions"),
		changes:  NewEventChannel("github.com/go-drift/drift/permissions/changes"),
		changeCh: make(chan PermissionChange, 4),
	}

	service.changes.Listen(EventHandler{
		OnEvent: func(data any) {
			change, ok := parsePermissionChange(data)
			if !ok {
				errors.Report(&errors.DriftError{
					Op:      "permissions.parseChange",
					Kind:    errors.KindParsing,
					Channel: "github.com/go-drift/drift/permissions/changes",
					Err: &errors.ParseError{
						Channel:  "github.com/go-drift/drift/permissions/changes",
						DataType: "PermissionChange",
						Got:      data,
					},
				})
				return
			}
			service.changeCh <- change
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "permissions.streamError",
				Kind:    errors.KindPlatform,
				Channel: "github.com/go-drift/drift/permissions/changes",
				Err:     err,
			})
		},
	})

	return service
}

func (s *permissionServiceState) check(permission Permission) (PermissionResult, error) {
	result, err := s.channel.Invoke("check", map[string]any{
		"permission": string(permission),
	})
	if err != nil {
		return PermissionResultUnknown, err
	}
	return parsePermissionResult(result), nil
}

func (s *permissionServiceState) request(permission Permission) (PermissionResult, error) {
	result, err := s.channel.Invoke("request", map[string]any{
		"permission": string(permission),
	})
	if err != nil {
		return PermissionResultUnknown, err
	}
	return parsePermissionResult(result), nil
}

func (s *permissionServiceState) requestMultiple(permissions []Permission) (map[Permission]PermissionResult, error) {
	perms := make([]string, len(permissions))
	for i, p := range permissions {
		perms[i] = string(p)
	}

	result, err := s.channel.Invoke("requestMultiple", map[string]any{
		"permissions": perms,
	})
	if err != nil {
		return nil, err
	}

	results := make(map[Permission]PermissionResult)
	if m, ok := result.(map[string]any); ok {
		if resultsMap, ok := m["results"].(map[string]any); ok {
			for k, v := range resultsMap {
				if status, ok := v.(string); ok {
					results[Permission(k)] = PermissionResult(status)
				}
			}
		}
	}
	return results, nil
}

func (s *permissionServiceState) openSettings() error {
	_, err := s.channel.Invoke("openSettings", nil)
	return err
}

func (s *permissionServiceState) shouldShowRationale(permission Permission) bool {
	result, err := s.channel.Invoke("shouldShowRationale", map[string]any{
		"permission": string(permission),
	})
	if err != nil {
		return false
	}
	if m, ok := result.(map[string]any); ok {
		return parseBool(m["shouldShow"])
	}
	return false
}

func (s *permissionServiceState) changeChannel() <-chan PermissionChange {
	return s.changeCh
}

func parsePermissionResult(result any) PermissionResult {
	if m, ok := result.(map[string]any); ok {
		if status := parseString(m["status"]); status != "" {
			return PermissionResult(status)
		}
	}
	return PermissionResultUnknown
}

func parsePermissionChange(data any) (PermissionChange, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return PermissionChange{}, false
	}
	return PermissionChange{
		Permission: Permission(parseString(m["permission"])),
		Result:     PermissionResult(parseString(m["status"])),
	}, true
}
