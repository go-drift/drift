// Package runtime is the Go-side API for the Drift splash plugin.
//
// Without any call, the native splash stays up until the app draws its
// first frame with content: after App.OnInit returns and the root widget is
// built and composited. Preserve holds it past that until a matching
// Remove, for work the first screen should not show without:
//
//	import splash "github.com/go-drift/drift/plugins/splash/runtime"
//
//	drift.App{
//	    Root: App(),
//	    OnInit: func(ctx context.Context) error {
//	        if err := splash.Preserve(); err != nil {
//	            return err
//	        }
//	        go func() {
//	            loadConfig()
//	            if err := splash.Remove(); err != nil {
//	                log.Printf("splash: %v", err)
//	            }
//	        }()
//	        return nil
//	    },
//	}.Run()
//
// Preserve and Remove are counted on the native side. A Remove without a
// matching Preserve does nothing. The plugin's max_duration_ms after the
// app's window appears, Preserve no longer holds the splash, so a missed
// Remove cannot keep it up for good; a slow App.OnInit still does, since
// there is nothing to show, up to a ceiling of max(30 s, max_duration_ms).
package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-drift/drift/pkg/platform"
)

// channelName is the wire identifier the native splash plugin registers
// against. Must match the literal used in
// plugins/splash/plugin/ios/DriftSplashPlugin.swift and
// plugins/splash/plugin/android/DriftSplashPlugin.kt.
const channelName = "drift/splash"

// ErrAlreadyDismissed indicates Preserve came after the splash went:
// call it before the first frame.
var ErrAlreadyDismissed = errors.New("splash already dismissed")

var channel = platform.NewMethodChannel(channelName).WithErrors(platform.ErrorCodes{
	"already_dismissed": ErrAlreadyDismissed,
})

// Preserve keeps the splash up past the first frame until a matching
// Remove. Call it before the first frame: from App.OnInit or the root
// widget's InitState.
//
// It fails with ErrAlreadyDismissed if the splash has already gone, or if
// the native plugin is unreachable (the plugin is not in drift.yaml, or the
// call came from a package init before the app started).
func Preserve() error {
	if _, err := channel.Invoke(context.Background(), "preserve", nil); err != nil {
		return fmt.Errorf("splash preserve: %w", err)
	}
	return nil
}

// Remove releases one Preserve. When none remain and the app has drawn its
// first frame, the splash fades out.
//
// It fails if the native plugin is unreachable.
func Remove() error {
	if _, err := channel.Invoke(context.Background(), "remove", nil); err != nil {
		return fmt.Errorf("splash remove: %w", err)
	}
	return nil
}
