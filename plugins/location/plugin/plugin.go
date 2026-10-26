// Package plugin is the build-time integration for the Drift location
// plugin: the device's location, once or as a stream, and the location
// permissions.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/location/plugin
//	    config:
//	      when_in_use_description: Shows stores near you.
//	      always:                          # optional
//	        description: Records your runs while the app is in the background.
//	        notification_title: Recording your run          # optional
//	        notification_text: Your route is being recorded. # optional
//
// when_in_use_description is the text iOS shows when asking for location
// access (NSLocationWhenInUseUsageDescription); App Review rejects vague
// ones.
//
// The optional always section enables the Always permission and background
// updates (runtime.UpdateOptions.Background). It declares:
//
//   - iOS: NSLocationAlwaysAndWhenInUseUsageDescription (description, shown
//     when asking to upgrade to Always) and the location background mode
//     (UIBackgroundModes), which App Review expects the app to justify.
//   - Android: ACCESS_BACKGROUND_LOCATION, which Google Play accepts only
//     with a background location declaration, and a foreground service of
//     type location that keeps the app running while background updates
//     are on. Android requires that service to show an ongoing
//     notification: notification_title and notification_text. The plugin
//     also declares POST_NOTIFICATIONS: on Android 13+, the notification
//     appears in the notification drawer only once the app is granted
//     platform.Notifications.Permission.
//
// Leave always out unless the app needs location while in the background.
//
// The Go API is github.com/go-drift/drift/plugins/location/runtime.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android (Google Play services location).
package plugin

import (
	"embed"
	"fmt"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// playServicesLocation is the pinned Google Play services location
// library. Bump it together with the Android sources.
const playServicesLocation = "com.google.android.gms:play-services-location:21.1.0"

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.location"

// nativeConfig is what the native halves read as host.config
// (plugin/android/DriftLocationPlugin.kt).
type nativeConfig struct {
	// NotificationTitle and NotificationText are the ongoing notification
	// of Android's background service, set with the always: config.
	NotificationTitle string `json:"notificationTitle,omitempty"`
	NotificationText  string `json:"notificationText,omitempty"`
}

// Config is the location plugin's drift.yaml config block.
type Config struct {
	// WhenInUseDescription is why the app needs location while in use,
	// shown by iOS in the permission prompt.
	WhenInUseDescription string `yaml:"when_in_use_description" drift:"required"`
	// Always enables the Always permission and background updates.
	Always *Always `yaml:"always,omitempty"`
}

// Always configures the Always permission and background updates.
type Always struct {
	// Description is why the app needs location while in the background,
	// shown by iOS when asking to upgrade to Always.
	Description string `yaml:"description" drift:"required"`
	// NotificationTitle and NotificationText are the ongoing notification
	// Android shows while background updates run.
	NotificationTitle string `yaml:"notification_title" drift:"default=Using your location"`
	NotificationText  string `yaml:"notification_text" drift:"default=Location updates continue while the app is in the background."`
}

type location struct{}

func (location) Name() string { return "location" }

func (location) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	if a := cfg.Always; a != nil {
		// Defaulted, so not required: an explicit empty value is caught
		// here.
		for _, field := range []struct{ key, value string }{
			{"notification_title", a.NotificationTitle},
			{"notification_text", a.NotificationText},
		} {
			if strings.TrimSpace(field.value) == "" {
				return fmt.Errorf("location: always.%s must not be empty", field.key)
			}
		}
	}

	ctx.IOS.Info.SetString("NSLocationWhenInUseUsageDescription", cfg.WhenInUseDescription)
	if cfg.Always != nil {
		// The native plugin registers the Always permission, and allows
		// background updates, only when these are present.
		ctx.IOS.Info.SetString("NSLocationAlwaysAndWhenInUseUsageDescription", cfg.Always.Description)
		ctx.IOS.Info.AppendArrayItem("UIBackgroundModes", "location")
	}
	ctx.IOS.Sources.AddFS("Location", iosSources, "ios")
	ctx.IOS.Plugin("DriftLocationPlugin")

	ctx.Android.AddGradleDependency("implementation", playServicesLocation)
	ctx.Android.Manifest.AddPermission("android.permission.ACCESS_FINE_LOCATION")
	ctx.Android.Manifest.AddPermission("android.permission.ACCESS_COARSE_LOCATION")
	if cfg.Always != nil {
		// The native plugin registers the Always permission, and allows
		// background updates, only when ACCESS_BACKGROUND_LOCATION is
		// declared.
		ctx.Android.Manifest.AddPermission("android.permission.ACCESS_BACKGROUND_LOCATION")
		ctx.Android.Manifest.AddPermission("android.permission.FOREGROUND_SERVICE")
		ctx.Android.Manifest.AddPermission("android.permission.FOREGROUND_SERVICE_LOCATION")
		ctx.Android.Manifest.AddPermission("android.permission.POST_NOTIFICATIONS")
		ctx.Android.Manifest.AddService(`<service android:name="` + androidPackage + `.DriftLocationService" android:exported="false" android:foregroundServiceType="location" />`)
	}
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftLocationPlugin")

	var native nativeConfig
	if a := cfg.Always; a != nil {
		native = nativeConfig{NotificationTitle: a.NotificationTitle, NotificationText: a.NotificationText}
	}
	ctx.NativeConfig(native)
	return nil
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = location{}
