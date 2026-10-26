// Package plugin is the build-time integration for the Drift secure
// storage plugin: string values encrypted at rest (iOS Keychain, Android
// EncryptedSharedPreferences), optionally behind biometric authentication.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/securestorage/plugin
//	    config:
//	      biometrics:                    # optional: RequireBiometric
//	        face_id_description: Unlocks your saved passwords.
//
// Without the biometrics section, values marked RequireBiometric fail with
// ErrBiometricsNotConfigured and the app carries no biometric code:
// face_id_description is the text iOS shows when asking to use Face ID
// (NSFaceIDUsageDescription), and on Android the section adds
// androidx.biometric, which declares USE_BIOMETRIC.
//
// The Go API is github.com/go-drift/drift/plugins/securestorage/runtime.
//
// # Backup and restore
//
// On Android, values last as long as the install that wrote them: Auto
// Backup and device transfer copy the encrypted files but not the Android
// Keystore key that decrypts them (it never leaves the device), and
// uninstalling deletes the key. A store restored without its key opens empty (the plugin resets it and logs a warning), so keep
// what it holds recoverable, such as a token the user gets back by signing
// in again. On iOS, Keychain items move with an encrypted backup or device
// transfer unless their Accessibility is a ThisDeviceOnly one.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android.
package plugin

import (
	"embed"
	"fmt"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

// Android sources: the plugin, plus one of two Biometrics.kt, the
// BiometricPrompt gate or a stub that refuses, so apps without biometrics
// carry no biometric library.
//
//go:embed android android-biometric android-nobiometric
var androidSources embed.FS

// Pinned Android libraries. Bump them together with the Android sources.
const (
	securityCrypto = "androidx.security:security-crypto:1.1.0-alpha06"
	biometric      = "androidx.biometric:biometric:1.1.0"
)

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.securestorage"

// Config is the secure storage plugin's drift.yaml config block.
type Config struct {
	// Biometrics enables values that need biometric authentication.
	Biometrics *Biometrics `yaml:"biometrics,omitempty"`
}

// Biometrics configures biometric authentication.
type Biometrics struct {
	// FaceIDDescription is why the app uses Face ID, shown by iOS when
	// asking for it.
	FaceIDDescription string `yaml:"face_id_description" drift:"required"`
}

type secureStorage struct{}

func (secureStorage) Name() string { return "securestorage" }

func (secureStorage) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	if cfg.Biometrics != nil && strings.TrimSpace(cfg.Biometrics.FaceIDDescription) == "" {
		return fmt.Errorf("securestorage: biometrics.face_id_description must not be empty")
	}

	if cfg.Biometrics != nil {
		// The native plugin allows RequireBiometric only when this key is
		// present.
		ctx.IOS.Info.SetString("NSFaceIDUsageDescription", cfg.Biometrics.FaceIDDescription)
	}
	ctx.IOS.Sources.AddFS("SecureStorage", iosSources, "ios")
	ctx.IOS.Plugin("DriftSecureStoragePlugin")

	ctx.Android.AddGradleDependency("implementation", securityCrypto)
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	if cfg.Biometrics != nil {
		ctx.Android.AddGradleDependency("implementation", biometric)
		ctx.Android.Sources.AddFS(androidPackage, androidSources, "android-biometric")
	} else {
		ctx.Android.Sources.AddFS(androidPackage, androidSources, "android-nobiometric")
	}
	ctx.Android.Plugin(androidPackage + ".DriftSecureStoragePlugin")
	return nil
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = secureStorage{}
