// Package plugin is the build-time integration for the Drift splash plugin.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/splash/plugin
//	    config:
//	      image: assets/splash.png      # PNG
//	      image_width: 200              # iOS points; height from the PNG's aspect
//	      background_color: "#1A2238"   # #RRGGBB or #RRGGBBAA (alpha last)
//	      fade_duration_ms: 200
//	      max_duration_ms: 10000        # after this, Preserve no longer holds it
//	      android:                      # optional
//	        icon: assets/splash_icon.png  # default: image
//	        icon_background_color: "#FFFFFF"
//
// The splash stays up until the app draws its first frame with content
// (after App.OnInit), and longer while the runtime package's Preserve holds
// it.
//
// iOS: the launch storyboard is replaced with the image centred at
// image_width on background_color, and a native overlay laid out the same
// way takes over from it until the app is ready.
//
// Android: the platform splash screen (Android 12+, Drift's minimum) shows
// the icon centred on background_color, sized and masked to a circle by
// the system, so keep the logo within the icon's centre two thirds. The
// plugin styles it through MainActivity's launch theme and holds it until
// the app is ready.
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

// emitAndroid configures the platform splash screen (Android 12+, Drift's
// minimum) through the launch theme: a Drift.Splash style over the
// scaffold's LaunchTheme, set on MainActivity. The native plugin holds it
// on screen until the app is ready.
func emitAndroid(ctx *driftplugin.BuildCtx, r resolvedConfig) {
	ctx.Android.Drawables.AddBitmap("drift_splash_icon", r.AndroidIcon)
	ctx.Android.Resources.Colors.Set("drift_splash_background", r.BackgroundColor.String())
	items := map[string]string{
		"android:windowSplashScreenBackground":   "@color/drift_splash_background",
		"android:windowSplashScreenAnimatedIcon": "@drawable/drift_splash_icon",
	}
	if c := r.AndroidIconBackground; c != nil {
		ctx.Android.Resources.Colors.Set("drift_splash_icon_background", c.String())
		items["android:windowSplashScreenIconBackgroundColor"] = "@color/drift_splash_icon_background"
	}
	ctx.Android.Resources.Styles.Set("Drift.Splash", "LaunchTheme", items)
	ctx.Android.Manifest.SetActivityTheme(".MainActivity", "@style/Drift.Splash")

	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Sources.AddFile(androidPackage, "SplashConfig.kt",
		[]byte(generateSplashConfigKotlin(r)))
	ctx.Android.Plugin(androidPackage + ".DriftSplashPlugin")
}

// Plugin is the binding the generated bridge picks up. The typed
// Plugin[Config] form is mandatory; the concrete-struct shorthand doesn't
// give generic inference enough information to recover the Config type.
var Plugin driftplugin.Plugin[Config] = splash{}
