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
//	        description: Tracks your runs while the app is closed.
//
// when_in_use_description is the text iOS shows when asking for location
// access (NSLocationWhenInUseUsageDescription); App Review rejects vague
// ones. The optional always section enables the Always permission:
// Android's ACCESS_BACKGROUND_LOCATION, which Google Play accepts only with
// a background location declaration, and iOS's
// NSLocationAlwaysAndWhenInUseUsageDescription. Leave it out unless the app
// needs location while closed.
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

// Config is the location plugin's drift.yaml config block.
type Config struct {
	// WhenInUseDescription is why the app needs location while in use,
	// shown by iOS in the permission prompt.
	WhenInUseDescription string `yaml:"when_in_use_description" drift:"required"`
	// Always enables the Always (background) permission.
	Always *Always `yaml:"always,omitempty"`
}

// Always configures the Always (background) location permission.
type Always struct {
	// Description is why the app needs location while closed, shown by
	// iOS when asking to upgrade to Always.
	Description string `yaml:"description" drift:"required"`
}

type location struct{}

func (location) Name() string { return "location" }

func (location) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	if strings.TrimSpace(cfg.WhenInUseDescription) == "" {
		return fmt.Errorf("location: when_in_use_description must not be empty")
	}
	if cfg.Always != nil && strings.TrimSpace(cfg.Always.Description) == "" {
		return fmt.Errorf("location: always.description must not be empty")
	}

	ctx.IOS.Info.SetString("NSLocationWhenInUseUsageDescription", cfg.WhenInUseDescription)
	if cfg.Always != nil {
		// The native plugin registers the Always permission only when this
		// key is present.
		ctx.IOS.Info.SetString("NSLocationAlwaysAndWhenInUseUsageDescription", cfg.Always.Description)
	}
	ctx.IOS.Sources.AddFS("Location", iosSources, "ios")
	ctx.IOS.Plugin("DriftLocationPlugin")

	ctx.Android.AddGradleDependency("implementation", playServicesLocation)
	ctx.Android.Manifest.AddPermission("android.permission.ACCESS_FINE_LOCATION")
	ctx.Android.Manifest.AddPermission("android.permission.ACCESS_COARSE_LOCATION")
	if cfg.Always != nil {
		// The native plugin registers the Always permission only when this
		// permission is declared.
		ctx.Android.Manifest.AddPermission("android.permission.ACCESS_BACKGROUND_LOCATION")
	}
	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftLocationPlugin")
	return nil
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = location{}
