// Package plugin is the build-time integration for the Drift splash plugin.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/splash/plugin
//	    config:
//	      image: assets/splash.png   # PNG
//	      image_width: 200           # points / dp; height from the PNG's aspect
//	      background_color: "#1A2238" # #RRGGBB or #RRGGBBAA (alpha last)
//	      android_12:
//	        icon: assets/splash_icon.png
//	        icon_background_color: "#1A2238"
//
// At build time the plugin:
//   - Bundles the image as an iOS asset-catalogue image set and an Android
//     drawable bitmap.
//   - Replaces iOS LaunchScreen.storyboard with a generated layout matching
//     the runtime overlay (same colour, image centred at image_width) so the
//     launch-screen-to-overlay hand-off is seamless. On Android the overlay
//     draws the launch drawable itself.
//   - Replaces the Android `@drawable/launch_background` referenced by the
//     scaffold's LaunchTheme; the theme itself is untouched, avoiding
//     resource-merge collisions on pre-API-31 devices.
//   - On `android_12:` configurations, writes a values-v31/styles.xml
//     LaunchTheme variant that opts into AndroidX SplashScreen, adds the
//     core-splashscreen Gradle dependency, and ships the controller that
//     calls installSplashScreen() from the plugin's pre-Activity hook.
//   - Ships native Swift / Kotlin runtime sources via embedded filesystems
//     and names the DriftSplashPlugin class on each platform, which the app
//     registers at launch and attaches to the Drift view.
//
// # Platform support
//
// Supported: iOS (Xcode 16+), xtool, and Android.
//
// On xtool, which cannot compile asset catalogs, the Drift pipeline ships
// the DriftSplash image set as a loose DriftSplash.png in the app bundle
// root; UIImage(named:) resolves it the same way, so the native code is
// shared. Whether the replaced launch storyboard renders on xtool builds is
// unverified (xtool does not compile storyboards on Linux).
//
// FUTURE(xtool#219): with asset-catalog support in xtool, a dark-mode
// variant (image set and colour set appearances) becomes possible on every
// iOS build path. https://github.com/xtool-org/xtool/pull/219
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

// android12Sources ship only when android_12 is configured: they need the
// core-splashscreen dependency added alongside them.
//
//go:embed android12
var android12Sources embed.FS

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.splash"

type splash struct{}

func (splash) Name() string { return "splash" }

func (s splash) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	r, err := resolve(ctx, cfg)
	if err != nil {
		return fmt.Errorf("splash: %w", err)
	}
	emitIOS(ctx, r)
	emitAndroid(ctx, r)
	return nil
}

func emitIOS(ctx *driftplugin.BuildCtx, r resolvedConfig) {
	ctx.IOS.Assets.AddImageSet("DriftSplash", r.Image)
	ctx.IOS.Storyboards.ReplaceLaunchScreen(generateLaunchStoryboard(r))
	ctx.IOS.Sources.AddFS("Splash", iosSources, "ios")
	ctx.IOS.Sources.AddFile("Splash", "SplashConfig.swift", []byte(generateSplashConfigSwift(r)))
	ctx.IOS.Plugin("DriftSplashPlugin")
}

func emitAndroid(ctx *driftplugin.BuildCtx, r resolvedConfig) {
	ctx.Android.Drawables.AddBitmap("drift_splash", r.Image)
	ctx.Android.Resources.WriteXML("drawable/launch_background.xml", generateLayerList(r))
	ctx.Android.Resources.WriteXML("values/drift_splash_colors.xml",
		generateValuesColors(r.BackgroundColor))

	if r.Android12 != nil {
		ctx.Android.Drawables.AddBitmap("drift_splash_icon", r.Android12.Icon)
		ctx.Android.Resources.WriteXML("values-v31/styles.xml", generateV31Styles(*r.Android12))
		ctx.Android.AddGradleDependency("implementation",
			"androidx.core:core-splashscreen:1.0.1")
		ctx.Android.Sources.AddFS(androidPackage, android12Sources, "android12")
	}

	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Sources.AddFile(androidPackage, "SplashConfig.kt",
		[]byte(generateSplashConfigKotlin(r)))
	ctx.Android.Plugin(androidPackage + ".DriftSplashPlugin")
}

// Plugin is the binding the generated bridge picks up. The typed
// Plugin[Config] form is mandatory; the concrete-struct shorthand doesn't
// give generic inference enough information to recover the Config type.
var Plugin driftplugin.Plugin[Config] = splash{}
