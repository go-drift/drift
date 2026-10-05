// Package plugin is the build-time integration for the Drift media
// plugin: video in the widget tree (with the platform's own controls) and
// audio players without a view.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/media/plugin
//	    config:
//	      android:          # optional
//	        dash: true      # DASH streams (adds the media3 DASH module)
//
// Android plays through media3 ExoPlayer, which adds several megabytes to
// the app, with HLS always supported and DASH when configured; iOS plays
// through AVPlayer (HLS natively, no DASH).
//
// The Go API is github.com/go-drift/drift/plugins/media/runtime.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android. Playback stops when the app goes to
// the background.
package plugin

import (
	"embed"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// media3 is the pinned media3 version. Bump it together with the Android
// sources.
const media3 = "1.2.1"

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.media"

// Config is the media plugin's drift.yaml config block.
type Config struct {
	Android *Android `yaml:"android,omitempty"`
}

// Android configures the Android player.
type Android struct {
	// DASH adds DASH stream support.
	DASH bool `yaml:"dash"`
}

type media struct{}

func (media) Name() string { return "media" }

func (media) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	ctx.IOS.Sources.AddFS("Media", iosSources, "ios")
	ctx.IOS.Plugin("DriftMediaPlugin")

	for _, module := range androidModules(cfg) {
		ctx.Android.AddGradleDependency("implementation", "androidx.media3:"+module+":"+media3)
	}
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftMediaPlugin")
	return nil
}

// androidModules are the media3 modules cfg needs.
func androidModules(cfg Config) []string {
	modules := []string{"media3-exoplayer", "media3-exoplayer-hls", "media3-ui"}
	if cfg.Android != nil && cfg.Android.DASH {
		modules = append(modules, "media3-exoplayer-dash")
	}
	return modules
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = media{}
