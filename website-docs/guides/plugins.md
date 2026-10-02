---
id: plugins
title: Plugins
sidebar_position: 8
---

# Plugins

Plugins add native capabilities to a Drift app: SDKs, permissions, native UI, and platform hooks. The app never edits the generated Xcode or Android Studio projects; Drift applies each plugin's changes on every build.

A plugin is a Go module with up to three parts:

| Part | Package | Runs | Purpose |
|------|---------|------|---------|
| Build half | `<module>/plugin` | At build time | Reads the plugin's config from `drift.yaml` and records the native project changes it needs (**ops**) |
| Native code | Swift and Kotlin embedded in the build half | In the app | The platform side: SDK calls, lifecycle hooks, overlays |
| Runtime half | `<module>/runtime` (any name) | In the app | The Go API the app calls; talks to the native code over platform channels |

First-party plugins live in the Drift repository:

- [`plugins/splash`](#splash): a native splash screen held until the first frame.
- [`plugins/firebase`](#firebase): Firebase Core and Cloud Messaging (push notifications).

## Using plugins

### Adding a plugin

Add the module to your app, then list the build half in `drift.yaml`:

```bash
go get github.com/go-drift/drift/plugins/splash
```

```yaml
# drift.yaml
plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
      background_color: "#1A2238"
```

The next `drift build` or `drift run` picks it up. Plugins run in `drift.yaml` order, which also decides which plugin gets first claim on shared events such as notification taps.

`config` is checked against the plugin's typed config: unknown keys, missing required keys, malformed colours and missing files fail the build with the offending key named.

### The plugin bridge

Drift runs build halves through a small generated program, `tools/drift-plugins/main.go`, built and cached by the CLI. Commit it: it imports every configured plugin, which keeps their modules in `go.mod` when you run `go mod tidy`.

Drift never edits `go.mod` or `go.sum` during a build. If an entry is missing, the build stops and tells you to run `go mod tidy` (or `go get` for a plugin that is not a dependency yet).

```bash
drift plugin sync          # regenerate the bridge, validate drift.yaml
drift plugin sync --tidy   # same, running go mod tidy first
drift plugin list          # configured plugins
drift plugin list --resolve  # with module versions and status
drift plugin list --json
```

`drift plugin sync` exits nonzero on invalid config, so CI can run it.

### Removing a plugin

Delete its entry from `drift.yaml`. Managed builds regenerate the native project from scratch, and `drift run --watch` regenerates it as soon as the plugin set changes, so nothing is left behind. With no plugins left, Drift also deletes `tools/drift-plugins/main.go`; run `go mod tidy` to drop the module.

### Ejected projects

Plugins work in [ejected](/docs/guides/eject) projects too, applied to your files on every build. Drift owns these files and rewrites them each build:

| iOS (`platform/ios`) | Android (`platform/android`) |
|----------------------|------------------------------|
| `Drift/Plugins/` (a local Swift package holding every plugin's Swift module) | `app/src/main/java/com/drift/runner/` (plugin host support and the generated registrant) |
| `Runner/DriftPlugins.swift`, `Runner/DriftPluginRegistrant.swift` | |

Plugins also edit shared files: `Info.plist`, `Runner.entitlements`, `AndroidManifest.xml`, `build.gradle`, resource values. Drift records what it applied in `.drift/plugins.lock.json` in each platform project; commit it.

When you remove a plugin, Drift deletes the files it owned (unless you changed them) and fails the next build once with a list of the edits it made inside shared files, such as an `Info.plist` key or a manifest permission. Drift cannot tell those apart from your own edits, so you remove them, or keep them if you still want them.

If an ejected project lacks the wiring plugins need (calls into `DriftPlugins`, the `Drift/Plugins` package reference), the build fails and names the file and call to restore. Compare with a freshly ejected project.

### Splash

Shows a native splash screen at launch and keeps it up until the app draws its first frame with content, after `App.OnInit` returns.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
      image_width: 200
      background_color: "#1A2238"
      fade_duration_ms: 200
      max_duration_ms: 10000
      android:
        icon: assets/splash_icon.png
        icon_background_color: "#FFFFFF"
```

| Key | Default | Meaning |
|-----|---------|---------|
| `image` | required | PNG centred on the background |
| `image_width` | `200` | iOS width in points; height follows the PNG's aspect ratio |
| `background_color` | `#FFFFFF` | `#RRGGBB` or `#RRGGBBAA` (alpha last) |
| `fade_duration_ms` | `200` | Fade-out length |
| `max_duration_ms` | `10000` | After this, `Preserve` no longer holds the splash |
| `android.icon` | `image` | Android icon, e.g. a padded variant |
| `android.icon_background_color` | none | Circle behind the Android icon |

On Android the splash is the system splash screen (Android 12+): the icon is centred on `background_color`, sized and masked to a circle by the system, so keep the logo inside the icon's centre two thirds.

To keep the splash up while the app loads something the first screen needs, call `Preserve` before the first frame and `Remove` when done:

```go
import splash "github.com/go-drift/drift/plugins/splash/runtime"

drift.App{
    Root: App(),
    OnInit: func(ctx context.Context) error {
        if err := splash.Preserve(); err != nil {
            return err
        }
        go func() {
            loadConfig()
            if err := splash.Remove(); err != nil {
                log.Printf("splash: %v", err)
            }
        }()
        return nil
    },
}.Run()
```

Calls are counted: the splash fades when every `Preserve` has a matching `Remove` and the first frame is drawn. After `max_duration_ms`, unmatched `Preserve` calls stop holding it, so a missed `Remove` cannot keep it up forever; a slow `App.OnInit` still does, since there is nothing to show yet.

Supported on iOS, xtool and Android. On xtool, whether the replacement launch storyboard renders is unverified; the native overlay that follows it works.

### Firebase

Firebase Core and Cloud Messaging: an FCM token, messages, and notification taps.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/firebase/plugin
    config:
      ios:
        config_file: firebase/GoogleService-Info.plist
      android:
        config_file: firebase/google-services.json
```

Register both apps in the Firebase console with `drift.yaml`'s `app.id` as the iOS bundle ID and Android package name, and download their config files. The build checks that each file names your `app.id`. Each platform's section is required to build for that platform. iOS also needs an APNs key uploaded to the Firebase project and a signing team; the plugin adds the push entitlement. [`examples/firebase-demo`](https://github.com/go-drift/drift/tree/master/examples/firebase-demo) walks through the setup and includes a test sender.

```go
import "github.com/go-drift/drift/plugins/firebase/runtime/messaging"

// Send the token to your server whenever it changes.
messaging.Token().AddListener(func() {
    go register(messaging.Token().Value())
})

// A message received while the app runs.
messaging.Messages().Listen(func(m messaging.Message) {
    drift.Dispatch(func() { showBanner(m) })
})

// A notification the user tapped, including the one that launched the app.
messaging.Opens().Listen(func(m messaging.Message) {
    drift.Dispatch(func() { navigate(m.Data["route"]) })
})
```

Nothing here blocks. Messages and taps that arrive before the app listens are queued for its first listener. Showing notifications needs the user's permission (`platform.Notifications.Permission.Request`).

| Situation | What happens |
|-----------|--------------|
| Notification message, app in the foreground | Reaches `Messages()` only; nothing is shown, on both platforms |
| Notification message, app in the background or not running | The system shows it; a tap opens the app and reaches `Opens()` |
| Data message, app running | Reaches `Messages()` with `Foreground` set accordingly |

Limitations:

- Push on iOS needs a paid Apple Developer team, on xtool as with Xcode. With a free team the app still runs, but Apple grants no push capability, so no token or message arrives.
- Android: data messages and token refreshes that arrive while no Activity has started in the process are dropped; the token is fetched again at the next launch.
- Android: when the system groups several notifications from the app, tapping the group summary opens the app without an `Opens()` event. Tapping an individual notification works.

## Authoring plugins

Start from [`examples/plugins/hello`](https://github.com/go-drift/drift/tree/master/examples/plugins/hello), the smallest complete plugin: it takes a greeting from `drift.yaml` and ships native code that returns it to Go. The code on this page is that plugin's. The splash and Firebase plugins are full references: splash for native UI and lifecycle, Firebase for an SDK with a SwiftPM package, a Gradle plugin, config files, entitlements and a manifest service.

### Module layout

```
hello/
├── go.mod                  # module github.com/go-drift/drift/examples/plugins/hello
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

The build half compiles into the bridge, the runtime half into the app. Keep them in separate packages so neither pulls in the other's dependencies.

### The build half

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
	// iOS: the greeting goes in Info.plist; HelloPlugin.swift reads it.
	ctx.IOS.Info.SetString("HelloGreeting", cfg.Greeting)
	ctx.IOS.Sources.AddFS("Hello", iosSources, "ios")
	ctx.IOS.Plugin("HelloPlugin")

	// Android: the greeting is a string resource; HelloPlugin.kt reads it.
	ctx.Android.Resources.Strings.Set("hello_greeting", cfg.Greeting)
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
- Record ops for every platform unconditionally; Drift applies the ones for the platform being built. `ctx.Platform()` (`"ios"`, `"xtool"` or `"android"`) is for the rare plugin whose ops must differ between them.
- Support xtool. It is how iOS apps are developed on Linux, so a plugin that rejects it blocks development of every app that uses it. If something cannot work there (for example, a capability xtool cannot sign), build anyway and document what is missing.

### Config

`Config` is decoded from the plugin's `config:` block with `yaml` tags. Unknown keys are rejected, so typos fail the build. `drift` tags add rules, checked recursively through nested structs, lists of structs and `yaml:",inline"` embeds, by both `drift plugin sync` and the build:

| Tag | Meaning |
|-----|---------|
| `required` | The key must be present (an explicit `false` or `""` counts) |
| `default=<value>` | Used when the key is absent. Scalars only; not with `required` |
| `hex` | A `#RRGGBB` or `#RRGGBBAA` colour, alpha last |
| `asset` | A project-relative path to an existing file |

Combine them with commas: `drift:"required,asset"`. Optional sections are pointers (`Android *Android`), nil when absent. A malformed tag panics when the bridge starts, so it surfaces on the first build.

`ctx` helps with config that names files or the app:

- `ctx.ResolveAsset(path)` reads a project-relative file, such as an `asset` key's value.
- `ctx.AppID()` is `drift.yaml`'s `app.id` (the iOS bundle ID and Android application ID), for checking SDK config files that name the app.
- `driftplugin.ParseColor` parses a `hex` value into RGBA components, for platform files the plugin generates itself.

Further checks belong in `Build`: return an error naming the key, such as `fmt.Errorf("image_width must be positive, got %d", w)`.

### Recording ops

`ctx.IOS` and `ctx.Android` group the recorders; the [op reference](#op-reference) lists them all. Every op checks its input when recorded. Invalid input (a malformed plist key, a path escaping its directory, a bad Gradle coordinate) is not recorded; it is reported by `ctx.Err()`, which fails the build. The CLI checks again when it reads the ops.

Ops from different plugins are checked against each other by what they write, not by which recorder made them:

- Two ops writing the same location (a plist key, a resource, a file) must write the same content; they then count once.
- Ops adding to a set (manifest permissions, `UIBackgroundModes` entries, Gradle dependencies, SwiftPM products) merge.
- A location cannot be both written whole by one op and added to by another, such as `SetStringArray` and `AppendArrayItem` on one key.

A conflict fails the build and names both plugins.

### Native code

Native sources ship inside the build half: embed them and record them with `Sources.AddFS` (or `AddFile` for generated code), then name the plugin class.

**iOS.** Each plugin's Swift sources compile into their own module, `DriftPlugin_<name>`, inside the Drift-owned `Drift/Plugins` package, so plugins never clash on type or file names. Sources must be Swift. The plugin class conforms to `DriftPlugin`, and is `public` with a `public init()` because the app creates it from outside the module:

```swift
import DriftPluginAPI
import Foundation

public final class HelloPlugin: DriftPlugin {
    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel("example/hello") { method, _, result in
            switch method {
            case "greeting":
                result.success(Bundle.main.object(forInfoDictionaryKey: "HelloGreeting") as? String)
            default:
                result.error(NSError(domain: "example.hello", code: 1, userInfo: [
                    NSLocalizedDescriptionKey: "unknown method \(method)",
                ]))
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
        host.registerChannel("example/hello") { method, _, result ->
            when (method) {
                "greeting" -> {
                    // Plugin code cannot see the app's R class; look the
                    // resource up by name.
                    val res = host.context.resources
                    val id = res.getIdentifier("hello_greeting", "string", host.context.packageName)
                    result.success(res.getString(id))
                }
                else -> result.error(IllegalArgumentException("unknown method $method"))
            }
        }
    }
}
```

Android libraries come from `ctx.Android.AddGradleDependency`; components the system starts, such as a `FirebaseMessagingService`, are declared with `ctx.Android.Manifest.AddService`.

### Lifecycle

The app creates one instance of each plugin class per process, in `drift.yaml` order, and calls it on the main thread. Every method except registration has a default no-op implementation.

| | iOS (`DriftPlugin` protocol) | Android (`DriftPlugin` interface) |
|---|---|---|
| Once per process | `register(host:)`, then `didFinishLaunching(_:options:)` | `onRegister(host)`, from the first Activity's `onCreate` |
| View bound | `attach(DriftViewBinding)` / `detach()` around the Drift view controller | `onAttach(DriftActivityBinding)` / `onDetach()` around each Activity (recreation detaches and attaches again) |
| Overlays | `binding.overlayView`: above Drift's content and platform views; touches pass through where it is empty | `binding.overlayView`: the window's decor view |
| App events | `didRegisterForRemoteNotifications`, `didFailToRegisterForRemoteNotifications`, `didReceiveRemoteNotification`, `willPresentNotification`, `didReceiveNotificationResponse` | `binding.addOnNewIntentListener`; the launch intent is `binding.activity.intent` |

Register channels and observers once, in the registration method. Keep view state (overlays, view references) between attach and detach, and never keep a binding past detach.

App events are offered to plugins in `drift.yaml` order. A hook that returns `true` (or presentation options) claims the event: later plugins and Drift's own handling do not see it. Claim only your own events, such as notifications from your push provider. A plugin that claims `didReceiveRemoteNotification` must call its completion handler exactly once.

### Host API

`DriftPluginHost` (passed to the registration method) has the same shape on both platforms:

| Method | Purpose |
|--------|---------|
| `registerChannel(name, handler)` | Handle method calls from Go |
| `sendEvent(channel, data)` | Send an event to Go |
| `sendEventError(channel, code, message)` / `sendEventDone(channel)` | Report a stream error or end |
| `observeEvent(channel, handler)` | Receive events sent on a channel by any native module, such as the engine's `first_frame` on `drift/rendering/frame_events`; returns a subscription to cancel |
| `context` (Android only) | The application `Context` |

A method handler replies through its `DriftResult` exactly once, with `success(value)` or `error(error)`, either before returning or later from any thread (for example, from an SDK callback). A second reply is a programming error and fails loudly. The Go caller waits until the reply arrives.

Values crossing the channel are JSON-like: nil, booleans, numbers, strings, lists and string-keyed maps.

Name channels `<vendor>/<feature>`, such as `acme/camera`; `drift/` is reserved for Drift and its first-party plugins. Registering a channel name twice traps at startup.

### Threading

Every plugin callback runs on the main thread: lifecycle methods, method handlers and event observers (observers asynchronously, in the order events were sent).

When Go calls a method from the main thread, inside a frame or a widget callback, the handler must reply before returning; otherwise the app traps rather than deadlocks. Go APIs that wait on slow native work (a network request, a permission prompt) must therefore be called from a goroutine. Prefer APIs that cannot block: state the native side pushes as events, and streams.

### The runtime half

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

// Greeting returns the greeting configured in drift.yaml. It waits for the
// native side: call it from a goroutine, not from a widget callback.
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

For events from native code, pick the channel kind by what a late subscriber should see:

| Constructor | A subscriber that joins late gets | Use for |
|-------------|-----------------------------------|---------|
| `platform.NewEventChannel(name)` | Only later events | Live updates |
| `platform.NewStickyEventChannel(name)` | The most recent event, then later ones | One-shot signals, such as `first_frame` |
| `platform.NewQueuedEventChannel(name, capacity)` | Events sent while nobody listened, in order, once | Events that must each be handled, such as the tap that launched the app |

Wrap a channel in `platform.NewStream(name, channel, parse)` to give apps typed events; parse errors are reported through Drift's error handling instead of reaching listeners. Expose state the native side pushes as a `*core.Derived[T]` fed by a `core.Signal[T]`, as the Firebase plugin does for the FCM token. Apps subscribe with `Listen`/`AddListener` and update widgets inside `drift.Dispatch`.

### Testing

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
	var plist, resource string
	for _, op := range ops {
		switch o := op.(type) {
		case *protocol.OpPlistSetString:
			if o.Key == "HelloGreeting" {
				plist = o.Value
			}
		case *protocol.OpAndroidStringSet:
			if o.Name == "hello_greeting" {
				resource = o.Value
			}
		}
	}
	if plist != "hello" || resource != "hello" {
		t.Errorf("Info.plist greeting = %q, string resource = %q, want hello for both", plist, resource)
	}
}
```

- `NewTestCtx()`, `NewTestCtxAt(root)` and `NewTestCtxFor(root, platform)` create contexts; the root is where `ResolveAsset` reads, so seed it with test files. The first two build for every platform (`Platform()` is `"all"`).
- Test contexts have `AppID() == driftplugin.TestAppID`.
- Recorded ops are the types in `github.com/go-drift/drift/pkg/plugin/protocol`.

Test the runtime half against a fake native side from `pkg/platform` (the hello plugin's `runtime/hello_test.go` shows the pattern):

- `platform.SetNativeBridge(fake)` with a `platform.NativeBridge` implementation captures method calls and returns canned replies.
- `platform.HandleEvent(channel, data)` delivers an event as native code would; encode `data` with `platform.DefaultCodec.Encode`.
- Call `platform.ResetForTest()` in `t.Cleanup` to clear subscriptions, sticky values and queues between tests.

## Op reference

Each recorder records one op type, shown here by its wire name. "Same location" ops must agree with any other op writing there; "set" ops merge with other members.

### iOS

| Recorder | Op | Writes | Conflicts |
|----------|----|--------|-----------|
| `IOS.Info.SetString(key, value)` | `ios.plist.set_string` | `Info.plist` key | Same location |
| `IOS.Info.SetBool(key, value)` | `ios.plist.set_bool` | `Info.plist` key | Same location |
| `IOS.Info.SetStringArray(key, values)` | `ios.plist.set_string_array` | `Info.plist` key, whole array | Same location |
| `IOS.Info.AppendArrayItem(key, value)` | `ios.plist.append_array_item` | One entry in an `Info.plist` array, alongside the template's and other plugins' | Set |
| `IOS.Info.SetDict(key, dict)` | `ios.plist.set_dict` | `Info.plist` key, whole dictionary | Same location |
| `IOS.Entitlements.*` | the five `ios.plist.*` ops above | `Runner.entitlements`, such as `aps-environment` | As above |
| `IOS.Assets.AddImageSet(name, png)` | `ios.assets.add_image_set` | Image set in `Assets.xcassets` (a loose `<name>.png` on xtool); load with `UIImage(named:)` | Same location |
| `IOS.Storyboards.ReplaceLaunchScreen(xml)` | `ios.storyboards.replace_launch_screen` | `LaunchScreen.storyboard` | Same location |
| `IOS.Sources.AddFS(group, fs, root)` / `AddFile(group, rel, content)` | `ios.source.add` | Swift file in the plugin's module | File names unique per plugin |
| `IOS.Plugin(class)` | `ios.plugin` | The plugin class the app creates | Set |
| `IOS.AddPackageDependency(url, req, products)` | `ios.spm.add_package` | SwiftPM package and products for the plugin's module | One requirement per package; products are a set |
| `IOS.AddBundleResource(name, content)` | `ios.bundle.add_resource` | File in the app bundle root, for SDKs that read `Bundle.main` | Same location |

### Android

| Recorder | Op | Writes | Conflicts |
|----------|----|--------|-----------|
| `Android.Manifest.AddPermission(name)` | `android.manifest.add_permission` | `<uses-permission>` | Set |
| `Android.Manifest.AddIntentFilter(activity, xml)` | `android.manifest.add_intent_filter` | `<intent-filter>` in an activity | Set |
| `Android.Manifest.SetActivityAttr(activity, attr, value)` / `SetActivityTheme(activity, theme)` | `android.manifest.set_activity_attr` | Activity attribute, such as `android:theme` | Same location |
| `Android.Manifest.AddMetaData(parent, name, value)` | `android.manifest.add_meta_data` | `<meta-data>` in `application` or `activity:<name>` | Same location |
| `Android.Manifest.AddService(xml)` | `android.manifest.add_service` | `<service>` in `<application>`, named by its fully qualified class | Same location |
| `Android.Resources.Colors.Set(name, hex)` | `android.color.set` | `@color/<name>`, from a Drift hex colour | Same location |
| `Android.Resources.Strings.Set(name, value)` | `android.string.set` | `@string/<name>` | Same location |
| `Android.Resources.Styles.Set(name, parent, items)` | `android.style.set` | `@style/<name>` | Same location |
| `Android.Drawables.AddBitmap(name, content)` | `android.drawable.write` | `res/drawable-nodpi/<name>` (not scaled by density) | Same location |
| `Android.Resources.WriteXML(relPath, xml)` | `android.resource.write_xml` | Any resource file `res/<dir>/<file>.xml` except Drift's own values files | Same location |
| `Android.AddAppModuleFile(name, content)` | `android.app_module.add_file` | File next to `app/build.gradle`, such as `google-services.json` | Same location |
| `Android.AddGradleDependency(configuration, coord)` | `android.gradle.add_dependency` | Dependency with a pinned version | Set |
| `Android.ApplyGradlePlugin(id, version)` | `android.gradle.apply_plugin` | Applied Gradle plugin, declared at `version` (empty: already on the classpath) | One version per plugin id |
| `Android.Sources.AddFS(pkg, fs, root)` / `AddFile(pkg, rel, content)` | `android.source.add` | Kotlin file under the package | Same location |
| `Android.Plugin(class)` | `android.plugin` | The plugin class the app creates | Set |

## Next Steps

- [Platform Services](/docs/guides/platform) - Built-in native services
- [Ejecting](/docs/guides/eject) - Owning the native projects
- [Testing](/docs/guides/testing) - Widget tests for the app itself
