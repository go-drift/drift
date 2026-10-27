// Package plugin is the build-time integration for the Drift notifications
// plugin: local notifications the app schedules, and taps on them.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/notifications/plugin
//
// Showing notifications needs the user's permission, which Drift itself
// provides (platform.Notifications.Permission); this plugin declares
// Android's POST_NOTIFICATIONS for it, and SCHEDULE_EXACT_ALARM so the user
// can allow exact alarms. Push notifications come from the Firebase plugin;
// both can be used together.
//
// The Go API is github.com/go-drift/drift/plugins/notifications/runtime.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android. Android schedules with AlarmManager:
// exactly when the user allows exact alarms for the app, otherwise within
// Android's window for inexact alarms. Scheduled notifications do not
// survive a reboot on Android.
package plugin

import (
	"embed"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.notifications"

// Config is the notifications plugin's drift.yaml config block; it takes
// no settings.
type Config struct{}

type notifications struct{}

func (notifications) Name() string { return "notifications" }

func (notifications) Build(ctx *driftplugin.BuildCtx, _ Config) error {
	ctx.IOS.Sources.AddFS("Notifications", iosSources, "ios")
	ctx.IOS.Plugin("DriftNotificationsPlugin")

	ctx.Android.Manifest.AddPermission("android.permission.POST_NOTIFICATIONS")
	// The user grants it in Settings, Alarms & reminders, which lists only
	// apps that declare it. Not USE_EXACT_ALARM: Google Play allows that
	// only to alarm and calendar apps.
	ctx.Android.Manifest.AddPermission("android.permission.SCHEDULE_EXACT_ALARM")
	ctx.Android.Manifest.AddReceiver(`<receiver android:name="` + androidPackage + `.DriftNotificationReceiver" android:exported="false" />`)
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftNotificationsPlugin")
	return nil
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = notifications{}
