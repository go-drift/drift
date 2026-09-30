// Package plugin is the build-time integration for the Drift splash plugin.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/splash/plugin
//	    config:
//	      image: assets/splash.png
//	      background_color: "#1A2238"
//	      android_12:
//	        icon: assets/splash_icon.png
//	        icon_background_color: "#1A2238"
//
// At build time the plugin:
//   - Bundles the image as an iOS asset-catalogue image set and an Android
//     drawable bitmap.
//   - Replaces iOS LaunchScreen.storyboard with a generated layout matching
//     the runtime overlay so the system-to-runtime hand-off is seamless.
//   - Replaces the Android `@drawable/launch_background` referenced by the
//     scaffold's LaunchTheme; the theme itself is untouched, avoiding
//     resource-merge collisions on pre-API-31 devices.
//   - On `android_12:` configurations, writes a values-v31/styles.xml
//     LaunchTheme variant that opts into AndroidX SplashScreen, adds the
//     core-splashscreen Gradle dependency, and registers a pre-Activity
//     hook to call installSplashScreen() before super.onCreate.
//   - Ships native Swift / Kotlin runtime sources via embedded filesystems.
//   - Registers iOS and Android registrants so the runtime calls into the
//     plugin during `PlatformChannelManager` init.
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
// FUTURE(xtool#219): with asset-catalog support in xtool, a dark splash
// image set variant becomes possible on every iOS build path.
// https://github.com/xtool-org/xtool/pull/219
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

type splash struct{}

func (splash) Name() string { return "splash" }

func (s splash) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	r, err := resolve(cfg)
	if err != nil {
		return fmt.Errorf("splash plugin: %w", err)
	}

	if err := emitIOS(ctx, r); err != nil {
		return err
	}
	if err := emitAndroid(ctx, r); err != nil {
		return err
	}
	return nil
}

func emitIOS(ctx *driftplugin.BuildCtx, r resolvedConfig) error {
	img, err := ctx.ResolveAsset(r.Image)
	if err != nil {
		return fmt.Errorf("splash: read image %q: %w", r.Image, err)
	}
	ctx.IOS.Assets.AddImageSet("DriftSplash", img)
	ctx.IOS.Storyboards.ReplaceLaunchScreen(generateLaunchStoryboard(r))
	ctx.IOS.Info.SetString("UILaunchStoryboardName", "LaunchScreen")
	ctx.IOS.Sources.AddFS("Splash", iosSources, "ios")
	ctx.IOS.Sources.AddFile("Splash", "SplashConfig.swift", []byte(generateSplashConfigSwift(r)))
	ctx.IOS.Registrant("DriftSplashPlugin.register")
	return nil
}

func emitAndroid(ctx *driftplugin.BuildCtx, r resolvedConfig) error {
	img, err := ctx.ResolveAsset(r.Image)
	if err != nil {
		return fmt.Errorf("splash: read image %q: %w", r.Image, err)
	}
	ctx.Android.Drawables.AddBitmap("drift_splash", img)
	ctx.Android.Resources.WriteXML("drawable/launch_background.xml",
		generateLayerList("drift_splash_background", "drift_splash"))
	ctx.Android.Resources.WriteXML("values/plugin_colors.xml",
		generateValuesColors(r.BackgroundColor))

	if r.HasDark {
		darkImg, err := ctx.ResolveAsset(r.Dark.Image)
		if err != nil {
			return fmt.Errorf("splash: read dark image %q: %w", r.Dark.Image, err)
		}
		ctx.Android.Drawables.AddBitmap("drift_splash_dark", darkImg)
		ctx.Android.Resources.WriteXML("drawable-night/launch_background.xml",
			generateLayerList("drift_splash_background", "drift_splash_dark"))
		ctx.Android.Resources.WriteXML("values-night/plugin_colors.xml",
			generateValuesNightColors(r.Dark.BackgroundColor))
	}

	if r.HasAndroid12 {
		iconImg, err := ctx.ResolveAsset(r.Android12.Icon)
		if err != nil {
			return fmt.Errorf("splash: read android_12 icon %q: %w", r.Android12.Icon, err)
		}
		ctx.Android.Drawables.AddBitmap("drift_splash_icon", iconImg)
		ctx.Android.Resources.WriteXML("values-v31/styles.xml", generateV31Styles(r))
		ctx.Android.AddGradleDependency("implementation",
			"androidx.core:core-splashscreen:1.0.1")
		ctx.Android.PreActivityRegistrant("com.drift.plugin.splash.Android12SplashController.install")
	}

	ctx.Android.Sources.AddFS("com.drift.plugin.splash", androidSources, "android")
	ctx.Android.Sources.AddFile("com.drift.plugin.splash", "SplashConfig.kt",
		[]byte(generateSplashConfigKotlin(r)))
	ctx.Android.Registrant("com.drift.plugin.splash.DriftSplashPlugin.register")
	return nil
}

// Plugin is the binding the generated bridge picks up. The typed
// Plugin[Config] form is mandatory; the concrete-struct shorthand doesn't
// give generic inference enough information to recover the Config type.
var Plugin driftplugin.Plugin[Config] = splash{}
