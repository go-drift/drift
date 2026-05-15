// Package runtime is the Go-side API for the Drift splash plugin. Apps
// import this package and call Preserve()/Remove() from a Drift lifecycle
// hook (typically App.OnInit, or StateBase.InitState) to hold the native
// splash past first-frame.
//
//	import splash "github.com/go-drift/drift/plugins/splash/runtime"
//
//	drift.App{
//	    Root: App(),
//	    OnInit: func(ctx context.Context) error {
//	        splash.Preserve()
//	        go func() {
//	            loadConfig()
//	            splash.Remove()
//	        }()
//	        return nil
//	    },
//	}.Run()
//
// Preserve and Remove are ref-counted on the native side. Calling Remove
// without a matching Preserve is end-to-end a no-op (native clamps the
// preserveCount at zero).
//
// Go's package `func init()` runs before Drift's native bridge is up; calls
// from there cannot reach the native handler and are silently dropped. Use
// App.OnInit or InitState instead — both run after the bridge is alive and
// before the first frame is composited, which is exactly the window the
// splash plugin gates.
package runtime

import (
	"context"

	"github.com/go-drift/drift/pkg/platform"
)

// channelName is the wire identifier the native splash plugin registers
// against. Must match the literal used in
// plugins/splash/plugin/ios/DriftSplashPlugin.swift and
// plugins/splash/plugin/android/DriftSplashPlugin.kt.
const channelName = "drift/splash"

var channel = platform.NewMethodChannel(channelName)

// Preserve blocks the splash from auto-dismissing on first frame. Pair
// with Remove when the app is ready for the splash to fade.
//
// Ref-counted: each Preserve must be matched by a Remove. Call from a
// Drift lifecycle hook (typically StateBase.InitState or App.OnInit); see
// the package doc for why Go's package init is out of scope.
func Preserve() {
	_, _ = channel.Invoke(context.Background(), "preserve", nil)
}

// Remove decrements the preserve count. Safe to call multiple times; the
// native side clamps the resulting count at zero, so a Remove without a
// matching Preserve is a no-op.
func Remove() {
	_, _ = channel.Invoke(context.Background(), "remove", nil)
}
