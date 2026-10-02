// Package plugin is the build-time integration for the Drift Firebase
// plugin: Firebase Core and Cloud Messaging (push notifications).
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/firebase/plugin
//	    config:
//	      ios:
//	        config_file: firebase/GoogleService-Info.plist
//	      android:
//	        config_file: firebase/google-services.json
//
// Download both files from the Firebase console (Project settings, Your
// apps), registered with drift.yaml's app.id as the iOS bundle ID and the
// Android package name. A platform's section is required to build for it.
// The files name the app, so keep them out of version control if the
// repository is public.
//
// iOS needs an APNs key uploaded to the Firebase project (Project settings,
// Cloud Messaging) and a signing team with the Push Notifications
// capability, which Xcode's automatic signing enables from the
// aps-environment entitlement this plugin adds.
//
// The Go API is github.com/go-drift/drift/plugins/firebase/runtime/messaging.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android. xtool, Drift's development path on
// Linux, gets the same integration as iOS; release builds use Xcode. Push
// needs a paid Apple Developer team on either path: with a free team the
// app still runs, but Apple grants no push capability, so no FCM token or
// message arrives.
package plugin

import (
	"embed"
	"fmt"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// Pinned SDK versions. Bump them together with the native sources.
const (
	firebaseIOSSDK         = "https://github.com/firebase/firebase-ios-sdk"
	firebaseIOSVersion     = "12.19.2"
	firebaseMessagingCoord = "com.google.firebase:firebase-messaging:25.1.3"
	googleServicesPlugin   = "com.google.gms.google-services"
	googleServicesVersion  = "4.5.0"
)

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.firebase"

type firebase struct{}

func (firebase) Name() string { return "firebase" }

// Build emits the native integration for the platform being built; xtool
// gets the iOS integration.
func (firebase) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	platform := ctx.Platform()
	wantIOS := platform == "ios" || platform == "xtool" || (platform == "all" && cfg.IOS != nil)
	wantAndroid := platform == "android" || (platform == "all" && cfg.Android != nil)
	if wantIOS {
		if cfg.IOS == nil {
			return fmt.Errorf("firebase: ios.config_file is required to build for iOS")
		}
		plist, err := resolveIOS(ctx, cfg.IOS.ConfigFile)
		if err != nil {
			return fmt.Errorf("firebase: %w", err)
		}
		emitIOS(ctx, plist)
	}
	if wantAndroid {
		if cfg.Android == nil {
			return fmt.Errorf("firebase: android.config_file is required to build for Android")
		}
		json, err := resolveAndroid(ctx, cfg.Android.ConfigFile)
		if err != nil {
			return fmt.Errorf("firebase: %w", err)
		}
		emitAndroid(ctx, json)
	}
	return nil
}

func emitIOS(ctx *driftplugin.BuildCtx, serviceInfo []byte) {
	ctx.IOS.AddPackageDependency(firebaseIOSSDK,
		driftplugin.SPMRequirement{Kind: driftplugin.SPMExact, Value: firebaseIOSVersion},
		[]string{"FirebaseCore", "FirebaseMessaging"})
	ctx.IOS.AddBundleResource("GoogleService-Info.plist", serviceInfo)
	// Drift forwards app delegate and notification events to the plugin
	// explicitly; Firebase's method swizzling would race Drift's delegates.
	ctx.IOS.Info.SetBool("FirebaseAppDelegateProxyEnabled", false)
	// Data messages (content-available) wake the app in the background.
	ctx.IOS.Info.AppendArrayItem("UIBackgroundModes", "remote-notification")
	// Archives signed for distribution get "production" from their profile.
	ctx.IOS.Entitlements.SetString("aps-environment", "development")
	ctx.IOS.Sources.AddFS("Firebase", iosSources, "ios")
	ctx.IOS.Plugin("DriftFirebasePlugin")
}

func emitAndroid(ctx *driftplugin.BuildCtx, googleServices []byte) {
	ctx.Android.AddGradleDependency("implementation", firebaseMessagingCoord)
	// The google-services Gradle plugin turns google-services.json into the
	// resources FirebaseApp initialises from.
	ctx.Android.ApplyGradlePlugin(googleServicesPlugin, googleServicesVersion)
	ctx.Android.AddAppModuleFile("google-services.json", googleServices)
	ctx.Android.Manifest.AddPermission("android.permission.POST_NOTIFICATIONS")
	ctx.Android.Manifest.AddService(`<service android:name="` + androidPackage + `.DriftFirebaseMessagingService" android:exported="false">
    <intent-filter>
        <action android:name="com.google.firebase.MESSAGING_EVENT" />
    </intent-filter>
</service>`)
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftFirebasePlugin")
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = firebase{}
