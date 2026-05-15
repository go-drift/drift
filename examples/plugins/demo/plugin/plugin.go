// Package plugin is a worked example of a Drift build-time plugin.
//
// It exists to show the authoring contract end to end: a typed Config with
// drift validation tags, a Build method that records ops on both platform
// surfaces, and a typed Plugin[Config] value the bridge can consume. The
// behaviour is deliberately trivial; treat this as the smallest thing that
// exercises every part of the API.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/examples/plugins/demo/plugin
//	    config:
//	      greeting: "Hello from drift"
//	      shout: true
package plugin

import (
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// Config is the typed shape of the drift.yaml `config:` block for this plugin.
// drift tags are read by the schema sub-command and double as documentation.
type Config struct {
	Greeting string `yaml:"greeting" drift:"required"`
	Shout    bool   `yaml:"shout"    drift:"default=false"`
}

type demo struct{}

func (demo) Name() string { return "demo" }

func (demo) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	message := cfg.Greeting
	if cfg.Shout {
		message = strings.ToUpper(message)
	}
	ctx.IOS.Info.SetString("DriftDemoGreeting", message)
	ctx.Android.Resources.Strings.Set("drift_demo_greeting", message)
	return nil
}

// Plugin is the binding consumed by the generated bridge in
// tools/drift-plugins/main.go. It must be a typed Plugin[Config] value so
// generic inference reaches the Config type parameter.
var Plugin driftplugin.Plugin[Config] = demo{}

// ---- Event patterns (commented worked examples) -----------------------------
//
// This plugin is intentionally build-time only. Plugins that need runtime
// behaviour ship embedded Swift/Kotlin sources alongside this file and
// wire channels through the runtime host. The three communication
// directions are sketched below; see plugins/splash/ for a complete
// worked example that exercises all of them.
//
//
// (1) Shipping native sources + registering an entry point. Sit Swift
// files under ios/ and Kotlin under android/ next to plugin.go, then in
// Build():
//
//     import "embed"
//
//     //go:embed ios
//     var iosSources embed.FS
//
//     //go:embed android
//     var androidSources embed.FS
//
//     func (demo) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
//         // ...existing build-time ops...
//         ctx.IOS.Sources.AddFS("Demo", iosSources, "ios")
//         ctx.IOS.Registrant("DriftDemoPlugin.register")
//         ctx.Android.Sources.AddFS("com.example.demo", androidSources, "android")
//         ctx.Android.Registrant("com.example.demo.DriftDemoPlugin.register")
//         return nil
//     }
//
//
// (2) Native entry point. The Drift runtime calls register(host) once at
// app startup with a DriftPluginHost. The host exposes three relevant
// methods: registerChannel (receive Go→native calls), sendEvent (emit
// native→Go events), and observeEvent (subscribe to events from other
// native modules). Kotlin sketch:
//
//     package com.example.demo
//
//     import com.drift.runner.DriftPluginHost
//
//     object DriftDemoPlugin {
//         @JvmStatic
//         fun register(host: DriftPluginHost) {
//             // Go → native: method call with response.
//             host.registerChannel("example/demo") { method, args ->
//                 when (method) {
//                     "ping" -> Pair("pong", null)
//                     else  -> Pair(null, IllegalArgumentException("unknown $method"))
//                 }
//             }
//
//             // Native → Go: emit an event from a button press, sensor
//             // callback, timer, lifecycle hook, etc.
//             // host.sendEvent("example/demo/events", mapOf("type" to "tick", "count" to 1))
//
//             // Native ← native: observe events emitted by another module
//             // (e.g. the engine's first_frame signal). The returned
//             // DriftSubscription's cancel() unsubscribes; retain the token
//             // if you ever need to tear down.
//             host.observeEvent("drift/rendering/frame_events") { data ->
//                 // react to first_frame, etc.
//             }
//         }
//     }
//
// The Swift twin lives in ios/ with the same shape: an enum or class with
// a static register(host:) entry, channel handlers as closures, sendEvent
// and observeEvent against the same DriftPluginHost protocol.
//
//
// (3) Go-side runtime API. Create a sibling `runtime` package that apps
// import. Wire MethodChannel and/or EventChannel with the same names the
// native code uses; expose a clean Go API:
//
//     // examples/plugins/demo/runtime/demo.go
//     package runtime
//
//     import (
//         "context"
//
//         "github.com/go-drift/drift/pkg/platform"
//     )
//
//     var (
//         channel = platform.NewMethodChannel("example/demo")
//         events  = platform.NewEventChannel("example/demo/events")
//     )
//
//     // Ping invokes the native handler and returns "pong".
//     func Ping(ctx context.Context) (string, error) {
//         res, err := channel.Invoke(ctx, "ping", nil)
//         if err != nil {
//             return "", err
//         }
//         return res.(string), nil
//     }
//
//     // OnTick subscribes to native-emitted tick events. The returned
//     // Subscription's Cancel() unsubscribes.
//     func OnTick(fn func(count int)) *platform.Subscription {
//         return events.Listen(platform.EventHandler{
//             OnEvent: func(data any) {
//                 m, _ := data.(map[string]any)
//                 if m["type"] != "tick" {
//                     return
//                 }
//                 if n, ok := m["count"].(float64); ok {
//                     fn(int(n))
//                 }
//             },
//         })
//     }
//
// Apps then import the runtime sub-package and call demoplugin.Ping(ctx)
// or demoplugin.OnTick(...).
//
//
// (4) Channel naming. Use a `<vendor>/<feature>` prefix (e.g.
// `acme/camera`); the `drift/*` namespace is reserved for first-party
// channels. Two plugins registering the same channel name panic at
// startup with both plugins implicated — pick something unique to your
// module.
