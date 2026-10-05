// Package runtime is the Go API of the Drift secure storage plugin
// (github.com/go-drift/drift/plugins/securestorage/plugin in drift.yaml):
// string values encrypted at rest, optionally behind biometric
// authentication.
//
//	import securestorage "github.com/go-drift/drift/plugins/securestorage/runtime"
//
//	go func() {
//	    err := securestorage.Set(ctx, "token", token, nil)
//	    ...
//	    token, found, err := securestorage.Get(ctx, "token", nil)
//	}()
//
// Every call waits for the platform (the keychain, a biometric prompt), so
// call them from a goroutine: from the UI thread they fail with
// platform.ErrBlocksUIThread.
//
// Biometric protection guards reading a value; writing and deleting one
// never prompt. On iOS it is enforced by the Keychain (the item's access
// control); on Android by the plugin, before it decrypts the value
// (BiometricPrompt without a CryptoObject), so it guards against another
// person using the unlocked device, not against code running as the app.
package runtime

import (
	"context"
	"errors"
	"fmt"

	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// channelName must match the literal in the plugin's native sources
// (plugin/ios/DriftSecureStoragePlugin.swift, plugin/android/*.kt).
const channelName = "drift/securestorage"

var channel = platform.NewMethodChannel(channelName)

var (
	// ErrAuthFailed indicates biometric authentication failed.
	ErrAuthFailed = errors.New("biometric authentication failed")
	// ErrAuthCanceled indicates the user canceled biometric authentication.
	ErrAuthCanceled = errors.New("biometric authentication canceled")
	// ErrBiometricsUnavailable indicates the device cannot do biometric
	// authentication (no hardware, or none enrolled).
	ErrBiometricsUnavailable = errors.New("biometric authentication unavailable")
	// ErrBiometricsNotConfigured indicates RequireBiometric without the
	// plugin's biometrics: config.
	ErrBiometricsNotConfigured = errors.New("biometric protection needs the securestorage plugin's biometrics: config")
	// ErrLocked indicates the value cannot be read while the device is
	// locked (its Accessibility).
	ErrLocked = errors.New("secure storage is locked until the device is unlocked")
	// ErrNoActivity indicates a biometric prompt with no screen to show it
	// over (Android).
	ErrNoActivity = errors.New("no screen to show the biometric prompt over")
)

// nativeErrors maps the native halves' error codes to the sentinels.
var nativeErrors = map[string]error{
	"auth_failed":               ErrAuthFailed,
	"auth_canceled":             ErrAuthCanceled,
	"biometrics_unavailable":    ErrBiometricsUnavailable,
	"biometrics_not_configured": ErrBiometricsNotConfigured,
	"locked":                    ErrLocked,
	"no_activity":               ErrNoActivity,
}

// Accessibility is when a value can be read (iOS; Android values are
// readable whenever the app runs).
type Accessibility string

const (
	// WhenUnlocked: only while the device is unlocked. The default.
	WhenUnlocked Accessibility = "when_unlocked"
	// AfterFirstUnlock: after the first unlock since boot, also while
	// locked (for background work).
	AfterFirstUnlock Accessibility = "after_first_unlock"
	// WhenUnlockedThisDeviceOnly is WhenUnlocked, excluded from backups.
	WhenUnlockedThisDeviceOnly Accessibility = "when_unlocked_this_device_only"
	// AfterFirstUnlockThisDeviceOnly is AfterFirstUnlock, excluded from
	// backups.
	AfterFirstUnlockThisDeviceOnly Accessibility = "after_first_unlock_this_device_only"
)

// Options configures a call. A nil *Options uses the defaults.
type Options struct {
	// Service namespaces the keys. Empty uses the app's default namespace.
	Service string
	// Accessibility is when Set's value can be read (iOS).
	Accessibility Accessibility
	// RequireBiometric makes Set's value readable only after biometric
	// authentication. Needs the plugin's biometrics: config.
	RequireBiometric bool
	// BiometricPrompt is the message Get shows when the value needs
	// biometric authentication.
	BiometricPrompt string
}

func (o *Options) args(base map[string]any) map[string]any {
	if o == nil {
		return base
	}
	if o.Service != "" {
		base["service"] = o.Service
	}
	if o.Accessibility != "" {
		base["accessibility"] = string(o.Accessibility)
	}
	if o.RequireBiometric {
		base["requireBiometric"] = true
	}
	if o.BiometricPrompt != "" {
		base["biometricPrompt"] = o.BiometricPrompt
	}
	return base
}

// Set stores value under key, replacing any value there.
func Set(ctx context.Context, key, value string, opts *Options) error {
	_, err := invoke(ctx, "set", opts.args(map[string]any{"key": key, "value": value}))
	return err
}

// Get returns the value under key, and whether there is one. A value
// stored with RequireBiometric prompts for biometric authentication first.
func Get(ctx context.Context, key string, opts *Options) (value string, found bool, err error) {
	result, err := invoke(ctx, "get", opts.args(map[string]any{"key": key}))
	if err != nil {
		return "", false, err
	}
	m, ok := result.(map[string]any)
	if !ok {
		return "", false, parseError("get", result)
	}
	if m["value"] == nil {
		return "", false, nil
	}
	value, ok = m["value"].(string)
	if !ok {
		return "", false, parseError("get", result)
	}
	return value, true, nil
}

// Delete removes the value under key, if any.
func Delete(ctx context.Context, key string, opts *Options) error {
	_, err := invoke(ctx, "delete", opts.args(map[string]any{"key": key}))
	return err
}

// Contains reports whether there is a value under key, without reading it
// (no prompt).
func Contains(ctx context.Context, key string, opts *Options) (bool, error) {
	result, err := invoke(ctx, "contains", opts.args(map[string]any{"key": key}))
	if err != nil {
		return false, err
	}
	m, _ := result.(map[string]any)
	exists, ok := m["exists"].(bool)
	if !ok {
		return false, parseError("contains", result)
	}
	return exists, nil
}

// Keys returns the keys that have values.
func Keys(ctx context.Context, opts *Options) ([]string, error) {
	result, err := invoke(ctx, "keys", opts.args(map[string]any{}))
	if err != nil {
		return nil, err
	}
	m, _ := result.(map[string]any)
	list, ok := m["keys"].([]any)
	if !ok {
		return nil, parseError("keys", result)
	}
	keys := make([]string, 0, len(list))
	for _, k := range list {
		s, ok := k.(string)
		if !ok {
			return nil, parseError("keys", result)
		}
		keys = append(keys, s)
	}
	return keys, nil
}

// DeleteAll removes every value.
func DeleteAll(ctx context.Context, opts *Options) error {
	_, err := invoke(ctx, "deleteAll", opts.args(map[string]any{}))
	return err
}

// Biometric is the kind of biometric authentication the device offers.
type Biometric string

const (
	// BiometricNone: none available, or none enrolled.
	BiometricNone Biometric = "none"
	// BiometricFaceID, BiometricTouchID and BiometricOpticID are iOS's.
	BiometricFaceID  Biometric = "face_id"
	BiometricTouchID Biometric = "touch_id"
	BiometricOpticID Biometric = "optic_id"
	// BiometricFingerprint and BiometricFace are Android's strong
	// biometrics.
	BiometricFingerprint Biometric = "fingerprint"
	BiometricFace        Biometric = "face"
)

// AvailableBiometric returns the biometric authentication the device
// offers, BiometricNone if it has none enrolled.
func AvailableBiometric(ctx context.Context) (Biometric, error) {
	result, err := invoke(ctx, "biometric", nil)
	if err != nil {
		return BiometricNone, err
	}
	m, _ := result.(map[string]any)
	kind, ok := m["type"].(string)
	if !ok {
		return BiometricNone, parseError("biometric", result)
	}
	return Biometric(kind), nil
}

func invoke(ctx context.Context, method string, args map[string]any) (any, error) {
	result, err := channel.Invoke(ctx, method, args)
	if err == nil {
		return result, nil
	}
	var channelErr *platform.ChannelError
	if errors.As(err, &channelErr) {
		if sentinel, ok := nativeErrors[channelErr.Code]; ok {
			return nil, fmt.Errorf("securestorage %s: %w", method, sentinel)
		}
	}
	return nil, fmt.Errorf("securestorage %s: %w", method, err)
}

func parseError(method string, got any) error {
	return &drifterrors.ParseError{Channel: channelName, DataType: method + " result", Got: got}
}
