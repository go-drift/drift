# Plugins

Drift plugins extend an app with native capabilities (SDKs, permissions, native UI, platform hooks) without the app editing generated native projects. This document describes the system as built on the `feat/plugins` branch. The plan for finishing it is in [plugins-v1-plan.md](plugins-v1-plan.md).

## Model

A plugin is a Go module with two halves:

| Half | Package convention | Runs | Purpose |
|------|--------------------|------|---------|
| Build | `<module>/plugin` | At build time, inside a generated "bridge" binary | Reads typed config from `drift.yaml`, emits declarative **ops** describing native project changes |
| Runtime | `<module>/runtime` | Inside the app | Go API the app calls; talks to the plugin's native code over platform channels |

Native code (Swift/Kotlin) ships inside the build half as embedded files and is injected into the generated project by ops.

This combines Expo's prebuild model (declarative project changes applied to a managed native project) with Flutter's generated plugin registrant (one generated entry point that registers every plugin at startup).

## Configuring plugins

```yaml
# drift.yaml
plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
      background_color: "#FFFFFF"
```

The module must be in the app's `go.mod` (`go get`). `drift plugin sync` regenerates the bridge and validates config against the plugin's schema; `drift plugin list` shows configured plugins.

## Authoring a plugin (build half)

```go
type Config struct {
    Greeting string `yaml:"greeting" drift:"required"`
}

type demo struct{}

func (demo) Name() string { return "demo" }
func (demo) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
    ctx.IOS.Info.SetString("DriftDemoGreeting", cfg.Greeting)
    ctx.Android.Resources.Strings.Set("drift_demo_greeting", cfg.Greeting)
    return nil
}

// The bridge binds this exact variable name.
var Plugin driftplugin.Plugin[Config] = demo{}
```

- Config decodes with unknown keys rejected, so `drift.yaml` typos fail the build.
- `drift:"..."` tags declare config rules, checked recursively through nested structs, lists of structs and `yaml:",inline"` embeds by the same code on `drift plugin sync` and on build:
  - `required`: the key must be present (an explicit `false` or `""` counts).
  - `default=<literal>`: used when the key is absent. Scalars only; cannot combine with `required`.
  - `hex`: a `#RRGGBB` or `#RRGGBBAA` string.
  - `asset`: a canonical project-relative path to an existing file.
  - A malformed tag (unknown validator, unparseable default) panics when the bridge starts.
- `Build` must not touch disk; it only records ops on `ctx`.
- Every op validates its input when recorded. Invalid input is not recorded; it is reported by `ctx.Err()`, which fails the build. The CLI validates again when decoding the bridge response, so mutators only see valid ops.
- `driftplugin.NewTestCtx()` plus `ctx.Ops()` lets plugin authors unit test `Build`.
- Reference plugins: `examples/plugins/demo` (minimal), `plugins/splash` (full, with native code and runtime API).

## Build pipeline

`workspace.Prepare` (`cmd/drift/internal/workspace/workspace.go`) scaffolds the managed native project (wiped and regenerated every build under `~/.drift/build/...`), then `runPluginPipeline`:

1. **Resolve.** `CheckPluginDeps` runs `go list` for each configured package.
2. **Bridge.** `EnsureBridge` (`cmd/drift/internal/plugin/bridge.go`) writes `tools/drift-plugins/main.go` (build tag `drift_tool`, committed with the app so `go mod tidy` keeps plugin deps) and builds it. Binaries are cached by a key over CLI version, Go version, `go.sum`, module pins and source; locally replaced plugins bypass the cache.
3. **Run.** `RunBridge` sends a JSON envelope on stdin (`APIVersion`, platform, config per plugin) and reads a JSON response file of ops.
4. **Decode and validate.** `DecodeOps` parses and validates each op, then `Validate` (`conflict.go`) checks ops against each other by target and drops exact duplicates.
5. **Apply.** `Apply` (`apply.go`) files ops by platform and runs mutators (`mutate/`): plist, AndroidManifest XML, resource XML, Gradle, sources, assets, SwiftPM sidecar, `xtool.yml`.
6. **Registrant.** `EnsureRunnerSupport` writes host support files; `WriteRegistrant` generates `DriftPluginRegistrant.swift` / `.kt`.

With zero plugins, the bridge is skipped but the registrant, sidecar and resource directories are still reset so generated projects always compile.

Ejected projects (`platform/ios`, `platform/android`) run the same pipeline against user-owned files. `CheckEjectedIOS` (`ejected.go`) fails early when an ejected iOS project lacks wiring a plugin needs.

## Ops

Ops are typed structs in `pkg/plugin/protocol/ops.go`, recorded through scopes on `BuildCtx` (`pkg/plugin/buildctx.go`), and serialized as JSON with a `type` discriminator. Package `protocol` holds the whole CLI-to-bridge wire contract (envelope, response, ops, config schema); plugin authors import only `pkg/plugin`, plus `protocol` in tests that inspect `ctx.Ops()`.

| Area | Op types |
|------|----------|
| Info.plist | `info_plist.set_string`, `set_bool`, `set_string_array`, `append_array_item`, `set_dict` |
| iOS project | `ios.assets.add_image_set`, `ios.storyboards.replace_launch_screen`, `ios.source.add`, `ios.bundle.add_resource`, `ios.spm.add_package` |
| iOS plugin class | `ios.plugin` |
| AndroidManifest | `android.manifest.add_permission`, `add_intent_filter`, `set_activity_attr`, `add_meta_data` |
| Android resources | `android.color.set`, `android.string.set`, `android.style.set`, `android.drawable.write`, `android.resource.write_xml`, `android.assets.add`, `android.app_module.add_file` |
| Android build | `android.gradle.add_dependency`, `android.gradle.apply_plugin`, `android.source.add` |
| Android plugin class | `android.plugin` |

Each op declares the **targets** it writes (`Targets()`): a key naming a location (`plist:<key>`, `android-res:<type>/<name>`, `ios-bundle:<file>`, `spm:<package>`, ...), optionally a member of a set at that key (a permission, a registrant, a SwiftPM product), and a hash of what it writes there. Conflicts are keyed on targets, not op types, so two different op types writing one plist key or resource are caught:
- Ops owning the same key must write the same content (they then collapse).
- Ops adding the same member to a set must agree; different members merge (plugins sharing `firebase-ios-sdk` may each ask for their own products).
- A key cannot be both owned by one op and added to by another (`set_string_array` vs `append_array_item`).

Adding an op means touching: the struct and its methods (`Type`, `Targets`, `Validate`, `Platform`) plus the constructor table (`protocol/ops.go`), a recorder (`buildctx.go`), the fixture (`protocol/ops_test.go`), the bag and switch in `apply.go`, and a mutator. `TestOpsCoverAllConstructors` and `TestApplyKnowsEveryOpType` catch omissions.

## iOS specifics

- **Two build paths.** `ios` is xcodeproj on macOS (the shipping path). `xtool` is SwiftPM plus the xtool packer on Linux (dev only).
- **SwiftPM sidecar.** Both paths reference a generated local package at `Drift/Plugins` (`mutate/spm.go`). Its `DriftPlugins` target depends on every plugin-requested SwiftPM product.
- **Bundle resources.** Files land flat in the app bundle root: `Runner/PluginResources/` on xcodeproj (synchronized folder) and `PluginResources/` listed under `xtool.yml` `resources:` on xtool. SwiftPM target resources are avoided because they land in `Runner_Runner.bundle`, which `Bundle.main` cannot see.
- **Image sets on xtool.** xtool cannot compile asset catalogs, so image sets become loose `<Name>.png` files. `UIImage(named:)` resolves both forms. `FUTURE(xtool#219)` comments mark what changes if [xtool#219](https://github.com/xtool-org/xtool/pull/219) (asset catalog compiler, now in [AssetKit](https://github.com/xtool-org/AssetKit)) merges.
- **App-level hooks.** Plugins receive launch, URL, user activity and remote notification events through `DriftPlugin` methods. The app is scene-based, so URL and activity events arrive at `SceneDelegate` (xcodeproj) or SwiftUI modifiers (xtool) and go through `DeepLinkHandler.route`: the first plugin returning `true` claims a URL before Drift's deep-link channel sees it. `DriftPluginCoordinator.swift` merges background-fetch results from the plugins that accept a remote notification; its XCTest harness is `cmd/drift/internal/plugin/coordinator_test` (macOS only).

## Native plugin lifecycle

A plugin's native half is a class the build half names with `ctx.IOS.Plugin("MyPlugin")` / `ctx.Android.Plugin("com.example.MyPlugin")`. The generated `DriftPluginRegistrant` only lists these classes, in drift.yaml order; the hand-written `DriftPlugins` (iOS `templates/ios/DriftPlugins.swift`, Android `templates/android/runner/DriftPlugins.kt`) owns the instances and drives them on the main thread. Drift's own handling of the same events (deep links, notifications) stays in the app templates. Reference: Flutter's `FlutterPlugin` + `ActivityAware`.

| | iOS (`templates/ios/PluginAPI`, public) | Android (`templates/android/runner`, package `com.drift.runner`) |
|---|---|---|
| Plugin | `DriftPlugin` protocol | `DriftPlugin` interface |
| Once per process | `register(host:)` then `didFinishLaunching`, from `AppDelegate` | `onRegister(host)`, from the first `MainActivity.onCreate` |
| Per view / Activity | `attach(DriftViewBinding)` / `detach()` around `DriftViewController` | `onPreActivityCreate(activity)` before `super.onCreate`; `onAttach(DriftActivityBinding)` / `onDetach()` around each Activity |
| App events | `open(_:)`, `continueUserActivity(_:)` (return true to claim), remote-notification hooks | new-intent, activity-result and permission-result listeners on the binding (return true to claim) |
| Host | `DriftPluginHost`: `registerChannel`, `sendEvent`, `observeEvent` | same shape, `MethodHandler` |

Plugin Swift sources compile into the app module; Kotlin sources compile into the app module under the plugin's own package. Ejected projects are checked for the template calls that feed `DriftPlugins` (`ejected.go`); the plugin API and `DriftPlugins` itself are Drift-owned and rewritten by `EnsureRunnerSupport`.

## Runtime side

Runtime packages use `pkg/platform` method and event channels. The branch added a sticky `EventChannel` variant that replays the last event to late subscribers, and `engine.FrameEvents` emitting `first_frame`.
