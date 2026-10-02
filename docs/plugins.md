# Plugins: internals

How the plugin system is built. Using and authoring plugins, including the op reference, is in the website guide [website-docs/guides/plugins.md](../website-docs/guides/plugins.md); this document does not repeat it. The guide's code is [examples/plugins/hello](../examples/plugins/hello), the smallest complete plugin; `TestGuideQuotesThisPlugin` keeps the two identical. The plan for finishing the branch is in [plugins-v1-plan.md](plugins-v1-plan.md).

## Model

A plugin is a Go module with a build half (`<module>/plugin`, run at build time inside a generated bridge binary, records **ops**), native Swift/Kotlin embedded in the build half, and a runtime half (Go API over platform channels). This combines Expo's prebuild model (declarative changes applied to a managed native project) with Flutter's generated plugin registrant (one generated list of plugin classes the host drives).

Package layout:

| Package | Holds |
|---------|-------|
| `pkg/plugin` | Author API: `Plugin[T]`, `BuildCtx` and its recorder scopes, config schema from `drift` tags, test contexts, bridge `Main`/`Bind` |
| `pkg/plugin/protocol` | The CLI-to-bridge wire contract: envelope, response, op types (`ops.go`), validation (`validate.go`), config schema (`schema.go`), `Color` |
| `cmd/drift/internal/plugin` | CLI side: drift.yaml parsing (`manifest.go`), deps (`deps.go`), bridge build/run (`bridge.go`), conflicts (`conflict.go`), apply (`apply.go`), registrant and host support files (`registrant.go`), ejected checks (`ejected.go`) and lock (`lock.go`), `sync`/`list` (`sync.go`) |
| `cmd/drift/internal/plugin/mutate` | One mutator per file kind: plist, AndroidManifest and resource XML, Gradle, sources, bundle resources, the `Drift/Plugins` Swift package, `xtool.yml` |
| `cmd/drift/internal/templates/plugin-api/ios`, `templates/android/runner` | The native plugin API (`DriftPlugin`, `DriftPluginHost`, bindings, `DriftResult`) |

## Build pipeline

`workspace.Prepare` (`cmd/drift/internal/workspace/workspace.go`) scaffolds the managed project (regenerated every build under `~/.drift/build/...`), then:

1. **`resolvePluginOps`** runs every build half without touching the native project:
   1. `LoadFromDriftYAML` reads `plugins:` (YAML anchors resolved, an empty list allowed).
   2. `CheckPluginDeps` runs `go list` per package; a missing one fails with a `go get` hint.
   3. `EnsureBridge` writes `tools/drift-plugins/main.go` (build tag `drift_tool`, committed by the app so `go mod tidy` keeps plugin deps; keyword-safe import aliases) and builds it.
   4. `RunBridge` sends the envelope on stdin (`APIVersion`, `cmd`, platform, project root, build dir, `app_id`, config YAML per plugin) and reads ops from a response file.
   5. `protocol.DecodeOps` decodes and validates each op again; `Validate` (`conflict.go`) checks ops against each other by target and drops exact duplicates.
2. **`applyPluginOps`** writes them:
   1. Ejected projects only: `CheckEjectedIOS` / `CheckEjectedAndroid` fail early if the project lacks the template calls plugins need.
   2. `Apply` files ops by platform (`opAppliesTo`: iOS ops apply to `ios` and `xtool`) and runs the mutators.
   3. `EnsureRunnerSupport` writes the Drift-owned host files (`DriftPlugins`, plugin API), `WriteRegistrant` the generated `DriftPluginRegistrant` (plugin classes in drift.yaml order).
   4. Ejected projects only: `SyncEjectedLock` (see [Ejected projects](#ejected-projects)).

With zero plugins the bridge is skipped and `tools/drift-plugins/main.go` deleted, but the registrant, sidecar package and resource directories are still reset so generated projects always compile.

Watch mode: `Refresh` re-runs `resolvePluginOps` and, when the hash of the op list changed, regenerates the project.

## Bridge

- Built with `-mod=readonly`: the CLI never edits `go.mod`/`go.sum`; a missing entry fails with a `go mod tidy` hint (`drift plugin sync --tidy` runs it).
- Cache: binaries live under the cache root, keyed by a hash of everything that feeds the build (CLI and protocol versions, `bridgeTemplateVersion`, Go version, GOFLAGS, plugin module pins, bridge source, `go.mod`, `go.sum`, active `go.work`). Builds write a temp name and rename into place.
- When any source is local (a plugin in the main module, a directory `replace`, including a local `github.com/go-drift/drift`, or a multi-module `go.work`), the key cannot see edits, so the bridge is rebuilt every time into a fixed per-project path; the Go build cache keeps that cheap.
- The bridge refuses an envelope whose `APIVersion` differs, and the CLI refuses a response whose does. An op type the CLI does not know fails decoding.
- The bridge panics at start on an invalid `Name()` or a malformed `drift` tag (`Bind`) and on a duplicate plugin name or package (`Main`), so author errors surface on the first build.

## Ops and conflicts

Ops are typed structs in `protocol/ops.go` with `Type`, `Platform`, `Targets` and `Validate`, serialized as JSON with a `type` discriminator. Recorders (`pkg/plugin/buildctx.go`) validate at record time into `ctx.Err()`; the CLI validates again at decode, so mutators only see valid ops.

Each op declares the **targets** it writes: a key naming a location (`plist:<file>:<key>`, `android-res:<type>/<name>`, `ios-bundle:<file>`, `spm:<package>`, ...), optionally a member of a set at that key, and a hash of the content. `Validate` in `conflict.go`:

- Owners of one key must write the same content; they collapse to one.
- Members of a set merge; the same member twice must agree.
- A key cannot be both owned and added to (`set_string_array` vs `append_array_item`).

Some ops claim more than one key: an image set also claims its loose `<Name>.png` bundle name (xtool), a non-values resource XML file also claims the resource its file name defines, a Swift source also claims its basename within the plugin's module.

Adding an op means touching: the struct and its four methods plus the constructor table (`protocol/ops.go`), a recorder (`buildctx.go`), the fixture (`protocol/ops_test.go`), the bag and switch in `apply.go`, a mutator, the guide's op reference, and `describeEdit` (`lock.go`) if it edits a shared file. `TestOpsCoverAllConstructors`, `TestApplyKnowsEveryOpType` and `TestOpReferenceListsEveryOp` catch omissions.

## iOS specifics

- **Two build paths.** `ios` is xcodeproj on macOS (the shipping path); `xtool` is SwiftPM plus the xtool packer on Linux (development only).
- **Plugin modules.** Both paths link the `DriftPlugins` product of the Drift-owned local package at `Drift/Plugins` (`mutate/spm.go`). It holds a `DriftPluginAPI` target (from `templates/plugin-api/ios`) and one `DriftPlugin_<name>` target per plugin, containing its sources and depending on `DriftPluginAPI` plus the SwiftPM products it requested. The generated registrant imports each module and creates `DriftPlugin_<name>.<Class>()`. The app imports `DriftPluginAPI` and constructs host-only types through `@_spi(DriftHost)`.
- **Bundle resources** land flat in the bundle root: `Runner/PluginResources/` on xcodeproj (synchronized folder) and `PluginResources/` listed under `xtool.yml` `resources:` on xtool. SwiftPM target resources are avoided because they land in `Runner_Runner.bundle`, which `Bundle.main` cannot see.
- **Image sets on xtool** become loose `<Name>.png` files, since xtool cannot compile asset catalogs. `FUTURE(xtool#219)` comments mark what changes if [xtool#219](https://github.com/xtool-org/xtool/pull/219) merges.
- **Entitlements.** `Runner.entitlements` at the project root on both paths (`CODE_SIGN_ENTITLEMENTS`, xtool `entitlementsPath`).
- **App-level hooks.** `AppDelegate.swift` (also on xtool) forwards launch and remote-notification events to `DriftPlugins`. Drift's `UNUserNotificationCenter` delegate (`NotificationHandler` in `PlatformChannel.swift`) offers foreground notifications and responses to plugins before handling its own local notifications. A remote notification nobody claims completes with `.noData`. Hooks exist only where a plugin needs them.

## Native host

`DriftPlugins` (iOS `templates/ios/DriftPlugins.swift`, Android `templates/android/runner/DriftPlugins.kt`) owns the plugin instances and drives the lifecycle described in the guide; it is Drift-owned and rewritten every build. Callers:

| | iOS | Android |
|---|---|---|
| Register (once) | `AppDelegate.didFinishLaunching` calls `DriftPlugins.shared.launch` | `MainActivity.onCreate` calls `DriftPlugins.register` (no-op after the first) |
| Attach / detach | `DriftViewController` (`detach` from `deinit`) | `MainActivity.onCreate` / `onDestroy`, with the window's decor view as overlay host |
| Events | `AppDelegate`, `NotificationHandler` | `MainActivity.onNewIntent` |

Channel handlers run on the main thread. `DriftResult` lets a handler reply later; Go waits for the reply. When Go calls from the main thread and the handler has not replied by the time it returns, the host traps instead of deadlocking. Built-in channels keep synchronous handlers on the calling thread. The Android JNI error slot is thread-local.

## Ejected projects

Ejected projects (`platform/ios`, `platform/android`) run the same pipeline against user-owned files.

- `ejected.go` checks the template calls that feed `DriftPlugins` and, on iOS, the `Drift/Plugins` package reference; the error names each missing call.
- `lock.go`: after applying, `SyncEjectedLock` compares the ops with `.drift/plugins.lock.json` from the previous build (committed by the user). Files a removed plugin owned are deleted unless the user changed them; edits it made inside shared files (plist keys, manifest entries, Gradle lines) are listed once in a build error for the user to undo, since Drift cannot tell them from user edits.
- Plugin values XML (`values/plugin_*.xml`) and the `Drift/Plugins` package are regenerated every build.

## Runtime side

Runtime packages use `pkg/platform` channels: `NewMethodChannel`, `NewEventChannel`, `NewStickyEventChannel` (last event replayed to each new subscriber; `engine.FrameEvents` uses it for `first_frame`), `NewQueuedEventChannel` (events kept until a subscriber takes them, once each), and `NewStream` for typed parsing. The guide covers when to use which.
