package platform

import "maps"

import "fmt"

// SecureStorage provides secure key-value storage using platform-native encryption.
// On iOS, this uses the Keychain with optional LocalAuthentication.
// On Android, this uses EncryptedSharedPreferences with optional BiometricPrompt.
//
// Security notes:
//   - All data is encrypted at rest using platform-native encryption (Keychain/EncryptedSharedPreferences)
//   - On Android API < 23, secure storage is not available and operations return ErrPlatformNotSupported
//   - Biometric protection (RequireBiometric option) provides app-level authentication:
//   - On iOS: Hardware-backed via Keychain SecAccessControl with .biometryCurrentSet
//   - On Android: App-level UI gate only (BiometricPrompt without CryptoObject).
//     The data is still encrypted, but the biometric check is enforced by the app,
//     not cryptographically tied to key unlocking. This is a common pattern but
//     provides weaker security guarantees than iOS Keychain biometric protection.
var SecureStorage = &SecureStorageService{
	channel: NewMethodChannel("drift/secure_storage"),
	events:  NewEventChannel("drift/secure_storage/events"),
}

// SecureStorageService manages secure storage operations.
type SecureStorageService struct {
	channel *MethodChannel
	events  *EventChannel
}

// KeychainAccessibility determines when a keychain item is accessible (iOS-specific).
type KeychainAccessibility string

const (
	// AccessibleWhenUnlocked makes the item accessible only when the device is unlocked.
	AccessibleWhenUnlocked KeychainAccessibility = "when_unlocked"

	// AccessibleAfterFirstUnlock makes the item accessible after first unlock until reboot.
	AccessibleAfterFirstUnlock KeychainAccessibility = "after_first_unlock"

	// AccessibleWhenUnlockedThisDeviceOnly is like WhenUnlocked but not included in backups.
	AccessibleWhenUnlockedThisDeviceOnly KeychainAccessibility = "when_unlocked_this_device_only"

	// AccessibleAfterFirstUnlockThisDeviceOnly is like AfterFirstUnlock but not included in backups.
	AccessibleAfterFirstUnlockThisDeviceOnly KeychainAccessibility = "after_first_unlock_this_device_only"
)

// BiometricType represents the type of biometric authentication available.
type BiometricType string

const (
	// BiometricTypeNone indicates no biometric authentication is available.
	BiometricTypeNone BiometricType = "none"

	// BiometricTypeTouchID indicates Touch ID is available (iOS).
	BiometricTypeTouchID BiometricType = "touch_id"

	// BiometricTypeFaceID indicates Face ID is available (iOS).
	BiometricTypeFaceID BiometricType = "face_id"

	// BiometricTypeFingerprint indicates fingerprint authentication is available (Android).
	BiometricTypeFingerprint BiometricType = "fingerprint"

	// BiometricTypeFace indicates face authentication is available (Android).
	BiometricTypeFace BiometricType = "face"
)

// SecureStorageError represents errors from secure storage operations.
type SecureStorageError struct {
	Code    string
	Message string
}

func (e *SecureStorageError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error codes for secure storage operations.
const (
	SecureStorageErrorItemNotFound          = "item_not_found"
	SecureStorageErrorAuthFailed            = "auth_failed"
	SecureStorageErrorAuthCancelled         = "auth_cancelled"
	SecureStorageErrorBiometricNotAvailable = "biometric_not_available"
	SecureStorageErrorBiometricNotEnrolled  = "biometric_not_enrolled"
	SecureStorageErrorAuthPending           = "auth_pending"
	SecureStorageErrorPlatformNotSupported  = "platform_not_supported"
)

// ErrAuthPending is returned when an operation requires biometric authentication
// and the result will be delivered asynchronously via the event channel.
// Callers should listen on SecureStorage.Listen() to receive the result.
var ErrAuthPending = &SecureStorageError{
	Code:    SecureStorageErrorAuthPending,
	Message: "Biometric authentication pending - listen for result on event channel",
}

// ErrPlatformNotSupported is returned when secure storage is not available on the platform.
// On Android, this occurs on API < 23 (pre-Marshmallow).
var ErrPlatformNotSupported = &SecureStorageError{
	Code:    SecureStorageErrorPlatformNotSupported,
	Message: "Secure storage requires Android 6.0 (API 23) or higher",
}

// SecureStorageOptions configures secure storage operations.
type SecureStorageOptions struct {
	// KeychainAccessibility determines when the keychain item is accessible (iOS only).
	// Defaults to AccessibleWhenUnlocked.
	KeychainAccessibility KeychainAccessibility

	// RequireBiometric requires biometric authentication (Face ID/Touch ID/Fingerprint)
	// to access the stored value.
	RequireBiometric bool

	// BiometricPrompt is the message shown to the user during biometric authentication.
	BiometricPrompt string

	// Service is the namespace for storage. Defaults to the app's bundle identifier.
	Service string
}

func (o *SecureStorageOptions) toArgs() map[string]any {
	if o == nil {
		return nil
	}
	args := make(map[string]any)
	if o.KeychainAccessibility != "" {
		args["accessibility"] = string(o.KeychainAccessibility)
	}
	if o.RequireBiometric {
		args["requireBiometric"] = true
	}
	if o.BiometricPrompt != "" {
		args["biometricPrompt"] = o.BiometricPrompt
	}
	if o.Service != "" {
		args["service"] = o.Service
	}
	return args
}

// Set stores a value securely.
// If RequireBiometric is true, this may return ErrAuthPending indicating
// that biometric authentication is in progress. Listen on Listen() for the result.
func (s *SecureStorageService) Set(key, value string, opts *SecureStorageOptions) error {
	args := map[string]any{
		"key":   key,
		"value": value,
	}
	if opts != nil {
		maps.Copy(args, opts.toArgs())
	}

	result, err := s.channel.Invoke("set", args)
	if err != nil {
		return s.wrapError(err)
	}

	// Check for structured error or pending status
	if err := s.checkResultError(result); err != nil {
		return err
	}
	if m, ok := result.(map[string]any); ok {
		if pending, ok := m["pending"].(bool); ok && pending {
			return ErrAuthPending
		}
	}

	return nil
}

// Get retrieves a securely stored value.
// Returns empty string and nil error if the key doesn't exist.
// If the value is protected by biometrics (on Android), this may return ErrAuthPending
// indicating that biometric authentication is in progress. Listen on Listen() for the result.
func (s *SecureStorageService) Get(key string, opts *SecureStorageOptions) (string, error) {
	args := map[string]any{
		"key": key,
	}
	if opts != nil {
		if opts.BiometricPrompt != "" {
			args["biometricPrompt"] = opts.BiometricPrompt
		}
		if opts.Service != "" {
			args["service"] = opts.Service
		}
	}

	result, err := s.channel.Invoke("get", args)
	if err != nil {
		return "", s.wrapError(err)
	}

	if result == nil {
		return "", nil
	}

	// Check for structured error
	if err := s.checkResultError(result); err != nil {
		return "", err
	}

	if m, ok := result.(map[string]any); ok {
		// Check if biometric auth is pending
		if pending, ok := m["pending"].(bool); ok && pending {
			return "", ErrAuthPending
		}

		if value, ok := m["value"].(string); ok {
			return value, nil
		}
		// value is null - key doesn't exist
		if m["value"] == nil {
			return "", nil
		}
	}

	return "", nil
}

// Delete removes a securely stored value.
// On Android, deleting a biometric-protected key may return ErrAuthPending.
func (s *SecureStorageService) Delete(key string, opts *SecureStorageOptions) error {
	args := map[string]any{
		"key": key,
	}
	if opts != nil && opts.Service != "" {
		args["service"] = opts.Service
	}

	result, err := s.channel.Invoke("delete", args)
	if err != nil {
		return s.wrapError(err)
	}

	// Check for structured error or pending status
	if err := s.checkResultError(result); err != nil {
		return err
	}
	if m, ok := result.(map[string]any); ok {
		if pending, ok := m["pending"].(bool); ok && pending {
			return ErrAuthPending
		}
	}

	return nil
}

// Contains checks if a key exists in secure storage.
func (s *SecureStorageService) Contains(key string, opts *SecureStorageOptions) (bool, error) {
	args := map[string]any{
		"key": key,
	}
	if opts != nil && opts.Service != "" {
		args["service"] = opts.Service
	}

	result, err := s.channel.Invoke("contains", args)
	if err != nil {
		return false, s.wrapError(err)
	}

	// Check for structured error
	if err := s.checkResultError(result); err != nil {
		return false, err
	}

	if m, ok := result.(map[string]any); ok {
		if exists, ok := m["exists"].(bool); ok {
			return exists, nil
		}
	}

	return false, nil
}

// GetAllKeys returns all keys stored in secure storage.
func (s *SecureStorageService) GetAllKeys(opts *SecureStorageOptions) ([]string, error) {
	var args map[string]any
	if opts != nil && opts.Service != "" {
		args = map[string]any{
			"service": opts.Service,
		}
	}

	result, err := s.channel.Invoke("getAllKeys", args)
	if err != nil {
		return nil, s.wrapError(err)
	}

	// Check for structured error
	if err := s.checkResultError(result); err != nil {
		return nil, err
	}

	if m, ok := result.(map[string]any); ok {
		if keys, ok := m["keys"].([]any); ok {
			strKeys := make([]string, 0, len(keys))
			for _, k := range keys {
				if str, ok := k.(string); ok {
					strKeys = append(strKeys, str)
				}
			}
			return strKeys, nil
		}
	}

	return []string{}, nil
}

// DeleteAll removes all values from secure storage.
func (s *SecureStorageService) DeleteAll(opts *SecureStorageOptions) error {
	var args map[string]any
	if opts != nil && opts.Service != "" {
		args = map[string]any{
			"service": opts.Service,
		}
	}

	result, err := s.channel.Invoke("deleteAll", args)
	if err != nil {
		return s.wrapError(err)
	}

	// Check for structured error
	if err := s.checkResultError(result); err != nil {
		return err
	}

	return nil
}

// IsBiometricAvailable checks if biometric authentication is available on the device.
func (s *SecureStorageService) IsBiometricAvailable() (bool, error) {
	result, err := s.channel.Invoke("isBiometricAvailable", nil)
	if err != nil {
		return false, s.wrapError(err)
	}

	if m, ok := result.(map[string]any); ok {
		if available, ok := m["available"].(bool); ok {
			return available, nil
		}
	}

	return false, nil
}

// GetBiometricType returns the type of biometric authentication available on the device.
func (s *SecureStorageService) GetBiometricType() (BiometricType, error) {
	result, err := s.channel.Invoke("getBiometricType", nil)
	if err != nil {
		return BiometricTypeNone, s.wrapError(err)
	}

	if m, ok := result.(map[string]any); ok {
		if typeStr, ok := m["type"].(string); ok {
			return BiometricType(typeStr), nil
		}
	}

	return BiometricTypeNone, nil
}

// SecureStorageEvent represents an async result from a secure storage operation.
type SecureStorageEvent struct {
	Type    string // "auth_result"
	Success bool
	Key     string
	Value   string // Only for get operations
	Error   string // Error code if failed
}

// Listen returns a channel for receiving async authentication results.
// This is useful when biometric authentication is required and runs asynchronously.
func (s *SecureStorageService) Listen() <-chan SecureStorageEvent {
	ch := make(chan SecureStorageEvent, 4)

	s.events.Listen(EventHandler{
		OnEvent: func(data any) {
			if m, ok := data.(map[string]any); ok {
				evt := SecureStorageEvent{
					Success: m["success"] == true,
				}
				if typeStr, ok := m["type"].(string); ok {
					evt.Type = typeStr
				}
				if key, ok := m["key"].(string); ok {
					evt.Key = key
				}
				if value, ok := m["value"].(string); ok {
					evt.Value = value
				}
				if errStr, ok := m["error"].(string); ok {
					evt.Error = errStr
				}
				ch <- evt
			}
		},
	})

	return ch
}

// checkResultError extracts structured error from result map.
// Returns nil if no error field is present.
func (s *SecureStorageService) checkResultError(result any) error {
	if m, ok := result.(map[string]any); ok {
		if errCode, ok := m["error"].(string); ok {
			if errCode == SecureStorageErrorPlatformNotSupported {
				return ErrPlatformNotSupported
			}
			return &SecureStorageError{Code: errCode, Message: "Platform error: " + errCode}
		}
	}
	return nil
}

func (s *SecureStorageService) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already a SecureStorageError
	if _, ok := err.(*SecureStorageError); ok {
		return err
	}

	// Check if it's a ChannelError with a known code
	if ce, ok := err.(*ChannelError); ok {
		switch ce.Code {
		case SecureStorageErrorItemNotFound,
			SecureStorageErrorAuthFailed,
			SecureStorageErrorAuthCancelled,
			SecureStorageErrorBiometricNotAvailable,
			SecureStorageErrorBiometricNotEnrolled,
			SecureStorageErrorAuthPending,
			SecureStorageErrorPlatformNotSupported:
			return &SecureStorageError{
				Code:    ce.Code,
				Message: ce.Message,
			}
		}
	}

	return err
}
