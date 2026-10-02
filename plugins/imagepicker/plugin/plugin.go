// Package plugin is the build-time integration for the Drift image picker
// plugin: a photo from the camera, or images from the photo library.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/plugins/imagepicker/plugin
//	    config:
//	      camera:                     # optional: needed to take photos
//	        description: Takes a photo for your profile.
//
// Picking from the library needs no permission on either platform (the
// system pickers run outside the app). Taking a photo needs the camera
// section on iOS: its description is the text iOS shows when asking for
// camera access (NSCameraUsageDescription). Android captures through the
// system camera app, which needs no permission.
//
// The Go API is github.com/go-drift/drift/plugins/imagepicker/runtime.
//
// # Platform support
//
// iOS (Xcode 16+), xtool and Android. The iOS Simulator has no camera.
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

// androidPackage is the Kotlin package of the plugin's Android sources.
const androidPackage = "com.drift.plugin.imagepicker"

// Config is the image picker plugin's drift.yaml config block.
type Config struct {
	// Camera enables taking photos on iOS.
	Camera *Camera `yaml:"camera,omitempty"`
}

// Camera configures photo capture.
type Camera struct {
	// Description is why the app uses the camera, shown by iOS when asking
	// for camera access.
	Description string `yaml:"description" drift:"required"`
}

type imagePicker struct{}

func (imagePicker) Name() string { return "imagepicker" }

func (imagePicker) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	if cfg.Camera != nil {
		if strings.TrimSpace(cfg.Camera.Description) == "" {
			return fmt.Errorf("imagepicker: camera.description must not be empty")
		}
		// The native plugin captures only when this key is present.
		ctx.IOS.Info.SetString("NSCameraUsageDescription", cfg.Camera.Description)
	}
	ctx.IOS.Sources.AddFS("ImagePicker", iosSources, "ios")
	ctx.IOS.Plugin("DriftImagePickerPlugin")

	ctx.Android.Sources.AddFS(androidPackage, androidSources, "android")
	ctx.Android.Plugin(androidPackage + ".DriftImagePickerPlugin")
	return nil
}

// Plugin is the binding the generated bridge picks up.
var Plugin driftplugin.Plugin[Config] = imagePicker{}
