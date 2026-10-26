---
id: plugin-authoring
title: Authoring Plugins
sidebar_position: 10
---

# Authoring Plugins

The code on this page is [`examples/plugins/hello`](https://github.com/go-drift/drift/tree/master/examples/plugins/hello), the smallest complete plugin: it takes a greeting from `drift.yaml` and ships native code that returns it to Go. [`examples/plugins/hello-app`](https://github.com/go-drift/drift/tree/master/examples/plugins/hello-app) is an app using it. The splash and Firebase plugins are full references: splash for native UI and lifecycle, Firebase for an SDK with a SwiftPM package, a Gradle plugin, config files, entitlements and a manifest service.

## Your first plugin

This builds the hello plugin and an app that uses it, side by side in one directory, with the plugin unpublished. You need the `drift` CLI and, for the app, what [Getting Started](/docs/guides/getting-started) sets up.

**1. The plugin module.** Use your CLI's version (`drift version`) for Drift:

```bash
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/go-drift/drift@<version>
```

Write the files in [Module layout](#module-layout), taking their contents from the sections below: [the build half](#the-build-half) (`plugin/plugin.go`), the [native code](#native-code) (`plugin/ios/HelloPlugin.swift`, `plugin/android/HelloPlugin.kt`) and [the runtime half](#the-runtime-half) (`runtime/hello.go`). The tests are optional. Then:

```bash
go mod tidy
go test ./...
```

**2. The app.** Create it next to the plugin, and point it at the plugin's directory: an unpublished module needs a `replace`, and the `require` names it for Go (`v0.0.0` stands for "not released").

```bash
cd ..
drift init hello-app example.com/hello-app
cd hello-app
go mod edit -require=example.com/hello@v0.0.0 -replace=example.com/hello=../hello
```

In `drift.yaml`, replace `plugins: []` with the plugin and its config:

```yaml
plugins:
  - package: example.com/hello/plugin
    config:
      greeting: Hello from drift.yaml
```

Replace `main.go` with an app that shows the greeting:

```go
package main

import (
	"context"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"

	hello "example.com/hello/runtime"
)

func main() {
	drift.App{Root: app{}}.Run()
}

type app struct {
	core.StatefulBase
}

func (app) CreateState() core.State {
	return &appState{}
}

type appState struct {
	core.StateBase
	greeting string
}

func (s *appState) InitState() {
	// Greeting's native method replies in place, so the UI thread may
	// call it.
	greeting, err := hello.Greeting(context.Background())
	if err != nil {
		greeting = err.Error()
	}
	s.greeting = greeting
}

func (s *appState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	return widgets.Container{
		Color: colors.Background,
		Child: widgets.Centered(widgets.Text{
			Content: s.greeting,
			Style:   graphics.TextStyle{Color: colors.OnBackground, FontSize: 24},
		}),
	}
}
```

Generate the plugin bridge and record the plugin's dependencies in `go.mod` and `go.sum`:

```bash
drift plugin sync --tidy
```

**3. Run it.** The greeting from `drift.yaml` appears on screen:

```bash
drift run android
drift run ios          # macOS with Xcode
drift build xtool      # Linux
```

On Linux, iOS builds go through xtool; [xtool setup](/docs/guides/xtool-setup) covers installing it and running on a device.

Edit the plugin in `../hello` and run again: with the `replace`, every build uses the plugin's current source, Go and native alike.

**4. Publish it** when it is ready; see [Publishing a plugin](#publishing-a-plugin).

## Module layout

```
hello/
├── go.mod                  # module example.com/hello
├── plugin/                 # build half (drift.yaml names this package)
│   ├── plugin.go
│   ├── plugin_test.go
│   ├── ios/                # Swift, embedded and shipped by Build
│   │   └── HelloPlugin.swift
│   └── android/            # Kotlin, embedded and shipped by Build
│       └── HelloPlugin.kt
└── runtime/                # Go API for apps
    ├── hello.go
    └── hello_test.go
```

The `github.com/go-drift/drift` version the plugin's `go.mod` requires declares the plugin API its native code is written against: require the oldest Drift release whose API the plugin uses. A release CLI refuses a plugin requiring a newer Drift minor release than its own (patch releases do not change the plugin API), or one older than the oldest plugin API it still serves (see [Versions](/docs/guides/plugins#versions)). Version the plugin itself however you like.

The build half compiles into the bridge, the runtime half into the app. Keep them in separate packages so neither pulls in the other's dependencies.

## The build half

```go
package plugin

import (
	"embed"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// Config is the plugin's drift.yaml config block.
type Config struct {
	Greeting string `yaml:"greeting" drift:"required"`
}

type hello struct{}

func (hello) Name() string { return "hello" }

func (hello) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	// The native code reads the greeting as host.config["greeting"].
	ctx.NativeConfig(map[string]string{"greeting": cfg.Greeting})

	ctx.IOS.Sources.AddFS("Hello", iosSources, "ios")
	ctx.IOS.Plugin("HelloPlugin")

	ctx.Android.Sources.AddFS("com.example.hello", androidSources, "android")
	ctx.Android.Plugin("com.example.hello.HelloPlugin")
	return nil
}

// Plugin is the value the generated bridge binds: this exact name, typed
// as driftplugin.Plugin[Config].
var Plugin driftplugin.Plugin[Config] = hello{}
```

- `Name()` is a short lowercase identifier, unique among the app's plugins. It names the plugin's iOS Swift module (`DriftPlugin_hello`).
- `var Plugin` must have the typed form `driftplugin.Plugin[Config]`.
- `Build` only records ops on `ctx`; it must not write files. Drift validates the full op list from every plugin before touching the project.
- Record ops for every platform unconditionally; Drift applies the ones for the platform being built. `ctx.Platform()` (`"ios"`, `"xtool"` or `"android"`) is for the rare plugin whose ops must differ between them. Under `drift plugin sync` it is `"all"`: record for every platform the config enables, and check the config they need.
- Support xtool. It is how iOS apps are developed on Linux, so a plugin that rejects it blocks development of every app that uses it. If something cannot work there (for example, a capability xtool cannot sign), build anyway and document what is missing.

## Config

`Config` is decoded from the plugin's `config:` block with `yaml` tags. Unknown keys are rejected, so typos fail the build. `drift` tags add rules, checked recursively through nested structs, lists of structs and `yaml:",inline"` embeds, by both `drift plugin sync` and the build:

| Tag | Meaning |
|-----|---------|
| `required` | The key must be present (an explicit `false` or `0` counts); a string must not be empty or only whitespace |
| `default=<value>` | Used when the key is absent. Scalars only; not with `required` |
| `hex` | A `#RRGGBB` or `#RRGGBBAA` colour, alpha last |
| `asset` | A project-relative path to an existing file |

Combine them with commas: `drift:"required,asset"`. Optional sections are pointers (`Android *Android`), nil when absent. A malformed tag panics when the bridge starts, so it surfaces on the first build.

`ctx` helps with config that names files or the app:

- `ctx.ResolveAsset(path)` reads a project-relative file, such as an `asset` key's value.
- `ctx.AppID()` is `drift.yaml`'s `app.id` (the iOS bundle ID and Android application ID), for checking SDK config files that name the app.
- `driftplugin.ParseColor` parses a `hex` value into RGBA components, for platform files the plugin generates itself.

Further checks belong in `Build`: return an error naming the key, such as `fmt.Errorf("image_width must be positive, got %d", w)`. `drift plugin sync` runs `Build` too, so these fail it as well as the build.

## Native config

Pass what the native code needs from the config with `ctx.NativeConfig(v)`: `v` (a struct with `json` tags, or a map) must marshal to a JSON object, recorded once for both platforms. Native code reads it as `host.config`, a `[String: Any]` (iOS) or `Map<String, Any?>` (Android) of JSON-like values, empty when a plugin records none; numbers arrive as `NSNumber` on iOS and `Int`, `Long` or `Double` on Android. The hello plugin passes its greeting this way. Use it for every build-time choice the native code acts on, such as which permission kinds to register or a splash screen's timings, instead of generating sources or reading back `Info.plist` keys and resources.

What the platform itself requires stays where the platform reads it: a usage string in `Info.plist`, a permission in the manifest. Native code that checks for one (`Bundle.main.object(forInfoDictionaryKey:)` before using the camera) checks the platform's precondition, which another plugin may also satisfy, not the plugin's config.

## Recording ops

`ctx.IOS` and `ctx.Android` group the recorders; the [op reference](#op-reference) lists them all. Every op checks its input when recorded. Invalid input (a malformed plist key, a path escaping its directory, a bad Gradle coordinate) is not recorded; it is reported by `ctx.Err()`, which fails the build. The CLI checks again when it reads the ops.

Ops from different plugins are checked against each other by what they write, not by which recorder made them:

- Two ops writing the same location (a plist key, a resource, a file) must write the same content; they then count once.
- Ops adding to a set (manifest permissions, `UIBackgroundModes` entries, Gradle dependencies, SwiftPM products) merge.
- A location cannot be both written whole by one op and added to by another, such as `SetStringArray` and `AppendArrayItem` on one key.

A conflict fails the build and names both plugins.

## Native code

Native sources ship inside the build half: embed them and record them with `Sources.AddFS` (or `AddFile` for generated code), then name the plugin class.

**iOS.** Each plugin's Swift sources compile into their own module, `DriftPlugin_<name>`, inside the Drift-owned `Drift/Plugins` package, so plugins never clash on type or file names. Sources must be Swift. The plugin class conforms to `DriftPlugin`, and is `public` with a `public init()` because the app creates it from outside the module:

```swift
import DriftPluginAPI
import Foundation

public final class HelloPlugin: DriftPlugin {
    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel("example/hello") { channel in
            channel.method("greeting") { _ in
                host.config["greeting"] as? String
            }
        }
    }
}
```

SDKs come from SwiftPM with `ctx.IOS.AddPackageDependency`; the plugin's module depends on the products it asks for, so it can `import FirebaseCore` directly.

**Android.** Kotlin sources compile into the app module, under the package passed to `Sources.AddFS`. The plugin class implements `com.drift.runner.DriftPlugin` and has a no-argument constructor. Plugin code references only `com.drift.runner` types, never the app's package, so it compiles in every app:

```kotlin
package com.example.hello

import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class HelloPlugin : DriftPlugin {
    override fun onRegister(host: DriftPluginHost) {
        host.registerChannel("example/hello") {
            method("greeting") { _ -> host.config["greeting"] }
        }
    }
}
```

Android libraries come from `ctx.Android.AddGradleDependency`; components the system starts are declared with `ctx.Android.Manifest.AddService` (a `FirebaseMessagingService`, say) and `AddReceiver` (a `BroadcastReceiver`, such as an alarm's).

## Lifecycle

The app creates one instance of each plugin class per process, in `drift.yaml` order, and calls it on the main thread. Every method except registration has a default no-op implementation.

| | iOS (`DriftPlugin` protocol) | Android (`DriftPlugin` interface) |
|---|---|---|
| Once per process | `register(host:)`, then `didFinishLaunching(_:options:)` | `onRegister(host)`, when the process starts (`Application.onCreate`), before any Activity, service or receiver; a process may start without an Activity, for a push message or an alarm |
| View bound | `attach(DriftViewBinding)` / `detach()` around the Drift view controller, which detaches when its scene disconnects | `onAttach(DriftActivityBinding)` / `onDetach()` around each Activity: a configuration change Drift does not handle itself (dark mode, locale) destroys the Activity and creates another, as does reopening the app after its Activity finished, each with a fresh binding |
| Overlays | `binding.overlayView`: above Drift's content and platform views; touches pass through where it is empty | `binding.overlayView`: the window's decor view |
| App events | `openURL` (a URL the app is opened with, such as an OAuth redirect), `continueUserActivity` (universal links, Handoff, Spotlight; on xtool universal links only), `didRegisterForRemoteNotifications`, `didFailToRegisterForRemoteNotifications`, `didReceiveRemoteNotification`, `willPresentNotification`, `didReceiveNotificationResponse`; launch URLs and activities included | `binding.addIntentListener`: the intent the app is opened with (a notification tap, a deep link), including the one that launched it, offered once (a recreated Activity, or one relaunched from Recents, is not offered its old intent again) |

Register channels and observers once, in the registration method. Keep view state (overlays, view references) between attach and detach, and never keep a binding past detach.

App events are offered to plugins in `drift.yaml` order, before Drift's own handling: an unclaimed URL or intent reaches Drift's deep links. A hook that returns `true` (or presentation options) claims the event: later plugins and Drift's own handling do not see it. Claim only your own events, such as notifications from your push provider. A plugin that claims `didReceiveRemoteNotification` must call its completion handler exactly once.

## Host API

`DriftPluginHost` (passed to the registration method) has the same shape on both platforms:

| Method | Purpose |
|--------|---------|
| `registerChannel(name) { ... }` | Handle method calls from Go, declaring each method in the block, and optionally the event stream of the same name (`stream(onListen:onCancel:)`, see [The runtime half](#the-runtime-half)) |
| `sendEvent(channel, data)` | Send an event to Go |
| `sendEventError(channel, error)` / `sendEventDone(channel)` | Fail Go's stream with an error (a `DriftCallError`, or any error as `native_error`), which reaches its subscribers' `OnError`; or end it |
| `observeEvent(channel, handler)` | Receive events sent on a channel by any native module, such as the engine's `first_frame` on `drift/rendering/frame_events`; returns a subscription to cancel |
| `registerPermission(kind, permission)` | Provide a runtime permission kind (see [Providing permissions](#providing-permissions)) |
| `registerPlatformView(type, factory)` | Provide a native view type to embed in widgets (see [Platform views](#platform-views)) |
| `config` | The plugin's own native config (see [Native config](#native-config)) |
| `context` (Android only) | The application `Context` |

Each method declares how it replies:

- `method(name) { args in ... }` (Kotlin: `method(name) { args -> ... }`) replies by returning: the return value is the reply, and a thrown error fails the call. Use it whenever the answer is at hand.
- `asyncMethod(name) { args, result in ... }` replies later through its `DriftResult`, exactly once, with `success(value)` or `error(error)`, from any thread (for example, from an SDK callback). Use it for work that takes time: a network request, a permission prompt, a picker. A second reply is a programming error and fails loudly; a `DriftResult` released without a reply fails the call with `platform.ErrReplyDropped` (on Android, once the result is garbage-collected).

```swift
host.registerChannel("acme/push") { channel in
    channel.method("isRegistered") { _ in UIApplication.shared.isRegisteredForRemoteNotifications }
    channel.asyncMethod("token") { _, result in
        Messaging.messaging().token { token, error in
            if let error { result.error(error) } else { result.success(token) }
        }
    }
}
```

Calling a method that is not declared fails with `platform.ErrMethodNotFound`, on Drift's own channels too.

To fail a call with an error Go can match, throw a `DriftCallError(code, message)` (Swift and Kotlin alike), or pass it to `result.error`; an optional `details` argument carries JSON-like data. Go receives a `*platform.ChannelError` with that `Code`, `Message` and `Details`; the runtime half declares which of its sentinel errors each code means (see [The runtime half](#the-runtime-half)).

- Missing or malformed arguments: `DriftCallError.invalidArguments(message)` on iOS, an `IllegalArgumentException` (as `require` throws) on Android. Both match `platform.ErrInvalidArguments`.
- Any other error reaches Go as `native_error`, with the `NSError` domain and code (iOS) or the exception class (Android) in `Details`.

Values crossing the channel are JSON-like: nil, booleans, finite numbers, strings, lists and string-keyed maps. Send bytes as base64 strings (a Go `[]byte` argument already arrives as one). A reply or event outside that set, such as a NaN, an Infinity or a `Date`, never reaches Go as data: the call fails with `platform.ErrNotEncodable`, and an event arrives as that error on the stream's error handler, its message naming the call or channel and the offending part (`value.items[2] is NaN, not a finite number`). Native logs it too.

On Android, start an Activity for its result (a system picker, a camera app) with `launchForResult(activity, contract, input) { output -> ... }` from `com.drift.runner`, passing the binding's `FragmentActivity`. It calls back once: with the output, or with `null` if the Activity is destroyed first, so an `asyncMethod` waiting on it always replies.

Name channels `<vendor>/<feature>`, such as `acme/camera`; `drift/` is reserved for Drift and its first-party plugins. Registering a channel under `drift/`, or a name twice, traps at startup.

## Threading

Every plugin callback runs on the main thread: lifecycle methods, method handlers and event observers (observers asynchronously, in the order events were sent).

Go's UI thread (widget callbacks, `Build`, frame work) is the platform main thread, so it never waits on native: a `method` called from it runs in place, and an `asyncMethod` called from it fails at once with `platform.ErrBlocksUIThread`, every time, before the handler runs. Call an async method from a goroutine and publish its result, for example through a `Signal`:

```go
token := core.NewSignal("")

go func() {
	t, err := push.Token(ctx)
	if err != nil {
		log.Printf("push token: %v", err)
		return
	}
	token.Set(t)
}()
```

From a goroutine, both kinds of method run on the main thread and the call waits for the reply or the context, whichever comes first. A canceled context returns at once; the native work finishes in the background and its reply is discarded.

When a runtime API wraps an async method, say so in its doc comment. Prefer APIs that cannot block where they fit: state the native side pushes as events, and streams.

## Providing permissions

Drift owns the permission request mechanism; a plugin provides the kinds its feature needs. In `Build`, declare what the platform requires (Android manifest permissions, iOS usage strings); in native code, register the kind once with `host.registerPermission`; in the runtime half, expose it with `platform.NewPermission(kind)`. Apps then call `Status`, `Request` and `Listen` on it like any other `platform.Permission`.

```go
// Build
ctx.IOS.Info.SetString("NSContactsUsageDescription", cfg.IOSDescription)
ctx.Android.Manifest.AddPermission("android.permission.READ_CONTACTS")

// Runtime half
var Permission = platform.NewPermission("contacts")
```

On iOS a kind implements `DriftPermission`: `status()` returns the current `DriftPermissionStatus` and `request(options:completion:)` prompts, then calls the completion exactly once. iOS sends no callback when it shows no prompt (it already asked) or when the user keeps the current level; the kind must still complete, with the current status.

```swift
final class ContactsPermission: DriftPermission {
    func status() -> DriftPermissionStatus { /* CNContactStore.authorizationStatus */ }
    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        CNContactStore().requestAccess(for: .contacts) { [self] _, _ in completion(status()) }
    }
}

host.registerPermission("contacts", ContactsPermission())
```

On Android most kinds are runtime permissions; `DriftRuntimePermission` implements them. Listing several requests them together, and the kind reads `LIMITED` when only some are granted (approximate location):

```kotlin
host.registerPermission("contacts", DriftRuntimePermission(Manifest.permission.READ_CONTACTS))
```

Drift requests only when a prompt can change the status, reports each result on `Listen`, and fails calls on a kind nobody registered with `platform.ErrPermissionNotRegistered`. On Android, a kind whose manifest permission is missing fails with `platform.ErrPermissionNotDeclared`. `Request` waits for the user, so it fails with `platform.ErrBlocksUIThread` on the UI thread.

## Platform views

A plugin can embed native views (a map, a video, an ad) in Drift's widget tree. Drift positions, clips and shows them each frame; the plugin provides the view and its Go side.

Natively, register a view type with a factory. Drift calls the factory and the view on the main thread, in the order Go made the calls:

```swift
host.registerPlatformView("acme/map") { viewId, params, events in
    MapView(params: params, events: events) // conforms to DriftPlatformView
}
```

```kotlin
host.registerPlatformView("acme/map") { context, viewId, params, events ->
    MapView(context, params, events) // implements DriftPlatformView
}
```

A `DriftPlatformView` has its `view`, `handle(method, args)` for calls from Go, `dispose()`, and (Android) `supportsRegionMask`: true for TextureView-backed views, which Drift can clip by a path when Drift UI overlaps them. Go does not wait for view methods, so `handle` reports failures by throwing (they are logged) and results as events: `events.send(method, data)`.

A factory that cannot make its view throws (a `DriftCallError` to give Go a code). Go's `Create` waits for the factory and returns that error, and no view exists; the media plugin's `NewVideoPlayerController` returns it to the app.

A view outlives its host. When an Android Activity is recreated or replaced, or an iOS scene reconnects, Drift moves every view into the new Drift view, so a video keeps playing and a web view keeps its page; plugins do nothing. A view must not keep what it was hosted in: on Android its `context` wraps the hosting Activity and Drift points it at the next one (and at the Application in between), so create the view with it and never cast it to an `Activity` or keep the Activity; on iOS, reach the current view controller through `view.window`.

What a view built for its old host does not follow: on Android a `PopupWindow`, or a service a library fetched once from the context; on iOS a view controller it presented. Rebuild it in `onHostChanged()`, which Drift calls on both platforms after each move; the media plugin rebuilds its `PlayerView` there around the same player, so playback goes on. Android itself caches a few Activity services inside views it owns (`EditText`'s and `WebView`'s autofill manager), which keeps the Activity a view was created in alive until that view is disposed: one Activity at most, however often the host changes.

Drift creates and drops a throwaway WebView at launch so the first real one opens without a stall. A view whose framework is slow to load can do the same: observe the warm-up event (`host.observeEvent(driftPlatformViewWarmUpEvent)` on iOS, `DRIFT_PLATFORM_VIEW_WARM_UP_EVENT` on Android), which Drift posts once at launch unless the app called `engine.DisableViewWarmUp`.

In Go, implement `platform.PlatformView` (embed `platform.PlatformViewBase` for its ID and type), register a `platform.PlatformViewFactory` for the same type, and call the view with `GetPlatformViewRegistry().InvokeViewMethod`. Events arrive in the view's `HandleEvent(method, args)`; return `platform.ErrMethodNotFound` for one it does not know. A widget embeds the view by implementing `layout.PlatformViewOwner` and calling `ctx.EmbedPlatformView` while painting; [`plugins/media`](/docs/guides/plugin-catalog#media) is a complete example.

## The runtime half

The runtime half wraps platform channels in a Go API, using the same channel names as the native code:

```go
package runtime

import (
	"context"
	"fmt"

	"github.com/go-drift/drift/pkg/platform"
)

// channel matches the name the native halves register
// (plugin/ios/HelloPlugin.swift, plugin/android/HelloPlugin.kt).
var channel = platform.NewMethodChannel("example/hello")

// Greeting returns the greeting configured in drift.yaml. The native
// method replies in place, so any goroutine may call it, including widget
// callbacks.
func Greeting(ctx context.Context) (string, error) {
	res, err := channel.Invoke(ctx, "greeting", nil)
	if err != nil {
		return "", fmt.Errorf("hello greeting: %w", err)
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("hello greeting: got %T, want string", res)
	}
	return s, nil
}
```

Declare the codes the native side fails with once, on the channel, so callers can match them under `errors.Is` while keeping the native message:

```go
// ErrDenied is the native halves' permission_denied.
var ErrDenied = errors.New("acme camera: permission denied")

var errorCodes = platform.ErrorCodes{"permission_denied": ErrDenied}

var camera = platform.NewMethodChannel("acme/camera").WithErrors(errorCodes)
```

Every error `camera.Invoke` returns is then a `*platform.ChannelError` that matches its code's sentinel, so a runtime API wraps it with context (`fmt.Errorf("camera capture: %w", err)`) and documents the sentinels. Codes are per channel: two plugins may both use `permission_denied`. An event channel declares the codes its stream fails with the same way (`platform.NewEventChannel(name).WithErrors(errorCodes)`). Drift's own codes (`platform.ErrMethodNotFound`, `ErrInvalidArguments`, `ErrBlocksUIThread`, ...) match on every channel and cannot be redeclared.

For events from native code, pick the channel kind by what a late subscriber should see:

| Constructor | A subscriber that joins late gets | Use for |
|-------------|-----------------------------------|---------|
| `platform.NewEventChannel(name)` | Only later events | Live updates |
| `platform.NewStickyEventChannel(name)` | The most recent event, then later ones | One-shot signals, such as `first_frame` |
| `platform.NewQueuedEventChannel(name, capacity)` | Events sent while nobody listened, in order, once | Events that must each be handled, such as the tap that launched the app |

A native stream can run only while Go listens. Declare it on the channel of the event channel's name, and send its events on that name:

```swift
host.registerChannel("acme/compass") { channel in
    channel.method("calibrate") { _ in ... }
    channel.stream(onListen: { try startHeadingUpdates() }, onCancel: { stopHeadingUpdates() })
}
// in the heading callback:
host.sendEvent("acme/compass", data: ["heading": heading])
```

```kotlin
host.registerChannel("acme/compass") {
    method("calibrate") { _ -> ... }
    stream(onListen = { startHeadingUpdates() }, onCancel = { stopHeadingUpdates() })
}
```

```go
var methods = platform.NewMethodChannel("acme/compass")
var headings = platform.NewStream("acme/compass", platform.NewEventChannel("acme/compass"), parseHeading)
```

`onListen` runs when the Go event channel gains its first subscriber, `onCancel` when it loses its last, on the main thread and strictly alternating, including for subscriptions made during package init. An error `onListen` throws reaches the subscribers' `OnError`, and no `onCancel` follows. `sendEventDone` ends the stream and Go's subscriptions without `onCancel`. Channels without a declared stream send events whenever they have them, as before. Options the stream needs are a method of their own, as the location plugin's `SetUpdateOptions` is.

Wrap a channel in `platform.NewStream(name, channel, parse)` to give apps typed events. Apps subscribe with `Listen(platform.StreamHandler[T]{OnEvent: ..., OnError: ...})`: `OnError` receives the errors the native side sends (`sendEventError`), matching the channel's `ErrorCodes`, such as a permission revoked mid-stream; without one, they are reported through Drift's error handling. An event the parser rejects is malformed native data: it is reported, never delivered. Handlers run off the UI thread, so update widgets inside `drift.Dispatch`.

Events that reach the runtime half some other way (one platform view's, one player's) are republished with a `platform.NewEmitter[T](name)`: `Emit` and `Error` feed its `Stream()`.

Expose state the native side pushes as a `*core.Derived[T]` fed by a `core.Signal[T]` that the runtime half sets on the UI thread (inside `platform.Dispatch`), as the Firebase plugin does for the FCM token and the media plugin for playback state; widgets then rebuild on it with `core.UseListenable`.

## Testing

Test the build half by building it the way the bridge does: `Bind(...).Build` checks and defaults the config, then calls `Build`. From the hello plugin's `plugin_test.go`:

```go
// build runs the plugin the way the bridge does (config checked against
// the schema, defaulted, decoded, then Build), recording for every
// platform.
func build(t *testing.T, config string) ([]protocol.Op, error) {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/examples/plugins/hello/plugin", Plugin).
		Build(ctx, []byte(config)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops(), nil
}

func TestBuildShipsGreeting(t *testing.T) {
	ops, err := build(t, "greeting: hello")
	if err != nil {
		t.Fatal(err)
	}
	var ios, android string
	for _, op := range ops {
		f, ok := op.(*protocol.OpAddFile)
		if !ok {
			continue
		}
		content, err := protocol.DecodeContent(f.Content)
		if err != nil {
			t.Fatal(err)
		}
		switch f.Location {
		case protocol.FileIOSPluginConfig:
			ios = string(content)
		case protocol.FileAndroidPluginConfig:
			android = string(content)
		}
	}
	if want := `{"greeting":"hello"}`; ios != want || android != want {
		t.Errorf("native config = %s (iOS), %s (Android), want %s for both", ios, android, want)
	}
}
```

- `NewTestCtx()`, `NewTestCtxAt(root)` and `NewTestCtxFor(root, platform)` create contexts; the root is where `ResolveAsset` reads, so seed it with test files. The first two build for every platform (`Platform()` is `"all"`).
- Test contexts have `AppID() == driftplugin.TestAppID`.
- Recorded ops are the types in `github.com/go-drift/drift/pkg/plugin/protocol`, one per wire name in the [op reference](#op-reference). From v0.27.0 they are part of the plugin API, like `BuildCtx`: tests switch on them, as above, so a release changes them only when it changes the plugin API, in a new minor release (see [Versions](/docs/guides/plugins#versions)). Whole files (bundle resources, drawables, plugin configs) are all `*protocol.OpAddFile`, told apart by `Location`, with base64 `Content` (`protocol.DecodeContent`).

Test the runtime half against a fake native side from `pkg/platform` (the hello plugin's `runtime/hello_test.go` shows the pattern):

- `platform.SetNativeBridge(fake)` with a `platform.NativeBridge` implementation captures method calls and answers them through the `reply` callback, in place or later from another goroutine (as an async method would).
- `platform.HandleEvent(channel, data)` delivers an event as native code would; encode `data` with `platform.DefaultCodec.Encode`. `platform.HandleEventError(channel, err)` delivers a stream error.
- The fake also sees Go's stream requests: a `listen` or `cancel` call on the `drift/streams` channel, with the event channel's name in `channel`, when an event channel gains its first subscriber or loses its last. Answer them like any call; tests can assert on them.
- Call `platform.ResetForTest()` in `t.Cleanup` to clear subscriptions, sticky values and queues between tests.

## Publishing a plugin

A plugin is a Go module, published like any other: push it to a repository whose path matches the module path, and tag a version.

```bash
git tag v0.1.0
git push origin v0.1.0
```

A plugin in a subdirectory of its repository is tagged with that directory as a prefix, such as `hello/v0.1.0`. Its `go.mod` keeps requiring the oldest Drift whose plugin API it uses (see [Module layout](#module-layout)), and must have no `replace` of its own.

Apps then drop the `replace` and require the release:

```bash
go mod edit -dropreplace=example.com/hello
go get example.com/hello@v0.1.0
drift plugin sync --tidy
```

Until a plugin is published, an app can also use it from a branch: `go get example.com/hello@main` records a pseudo-version.

## Op reference

Each recorder records one op, shown here by its wire name; `file.add` ops also name their location. "Same location" ops must agree with any other op writing there; "set" ops merge with other members.

### iOS

| Recorder | Op | Writes | Conflicts |
|----------|----|--------|-----------|
| `IOS.Info.SetString(key, value)` / `SetBool(key, value)` / `SetStringArray(key, values)` / `SetDict(key, dict)` | `ios.plist.set` | `Info.plist` key, whole value | Same location |
| `IOS.Info.AppendArrayItem(key, value)` | `ios.plist.append_array_item` | One entry in an `Info.plist` array, alongside the template's and other plugins' | Set |
| `IOS.Entitlements.*` | the two `ios.plist.*` ops above | `Runner.entitlements`, such as `aps-environment` | As above |
| `IOS.Assets.AddImageSet(name, png)` | `ios.assets.add_image_set` | Image set in `Assets.xcassets` (a loose `<name>.png` on xtool); load with `UIImage(named:)` | Same location |
| `IOS.Storyboards.ReplaceLaunchScreen(xml)` | `ios.storyboards.replace_launch_screen` | `LaunchScreen.storyboard` | Same location |
| `IOS.Sources.AddFS(group, fs, root)` / `AddFile(group, rel, content)` | `ios.source.add` | Swift file in the plugin's module | File names unique per plugin |
| `IOS.Plugin(class)` | `ios.plugin` | The plugin class the app creates | Set |
| `IOS.AddPackageDependency(url, req, products)` | `ios.spm.add_package` | SwiftPM package and products for the plugin's module | One requirement per package; products are a set |
| `IOS.AddBundleResource(name, content)` | `file.add` at `ios.bundle` | File in the app bundle root, for SDKs that read `Bundle.main` | Same location |
| `NativeConfig(v)` (with Android's) | `file.add` at `ios.plugin_config` | The plugin's native config, `DriftPluginConfig-<name>.json` in the bundle root | Same location |

### Android

| Recorder | Op | Writes | Conflicts |
|----------|----|--------|-----------|
| `Android.Manifest.AddPermission(name)` / `AddPermissionMaxSDK(name, maxSDK)` | `android.manifest.add_permission` | `<uses-permission>`, optionally with `android:maxSdkVersion` | Set |
| `Android.Manifest.AddIntentFilter(activity, xml)` | `android.manifest.add_intent_filter` | `<intent-filter>` in an activity | Set |
| `Android.Manifest.SetActivityAttr(activity, attr, value)` / `SetActivityTheme(activity, theme)` | `android.manifest.set_activity_attr` | Activity attribute, such as `android:theme` | Same location |
| `Android.Manifest.AddMetaData(parent, name, value)` | `android.manifest.add_meta_data` | `<meta-data>` in `application` or `activity:<name>` | Same location |
| `Android.Manifest.AddService(xml)` / `AddReceiver(xml)` | `android.manifest.add_component` | `<service>` or `<receiver>` in `<application>`, named by its fully qualified class | Same location |
| `Android.Resources.Colors.Set(name, hex)` | `android.color.set` | `@color/<name>`, from a Drift hex colour | Same location |
| `Android.Resources.Strings.Set(name, value)` | `android.string.set` | `@string/<name>`, from literal text (Drift escapes it for Android) | Same location |
| `Android.Resources.Styles.Set(name, parent, items)` | `android.style.set` | `@style/<name>` | Same location |
| `Android.Drawables.AddBitmap(name, content)` | `file.add` at `android.drawable` | `res/drawable-nodpi/<name>` (not scaled by density) | Same location |
| `Android.Resources.WriteXML(relPath, xml)` | `file.add` at `android.resource` | Any resource file `res/<dir>/<file>.xml` except Drift's own values files | Same location |
| `Android.AddAppModuleFile(name, content)` | `file.add` at `android.app_module` | File next to `app/build.gradle`, such as `google-services.json` | Same location |
| `Android.AddGradleDependency(configuration, coord)` | `android.gradle.add_dependency` | Dependency with a pinned version | Set |
| `Android.ApplyGradlePlugin(id, version)` | `android.gradle.apply_plugin` | Applied Gradle plugin, declared at `version` (empty: already on the classpath) | One version per plugin id |
| `Android.Sources.AddFS(pkg, fs, root)` / `AddFile(pkg, rel, content)` | `android.source.add` | Kotlin file under the package | Same location |
| `Android.Plugin(class)` | `android.plugin` | The plugin class the app creates | Set |
| `NativeConfig(v)` (with iOS's) | `file.add` at `android.plugin_config` | The plugin's native config, the asset `drift/plugins/<name>.json` | Same location |

## Next Steps

- [Plugins](/docs/guides/plugins) - Using plugins in an app
- [Plugin Catalog](/docs/guides/plugin-catalog) - Drift's own plugins, as references
- [Platform Services](/docs/guides/platform) - Built-in native services
- [Testing](/docs/guides/testing) - Widget tests for the app itself
