# Plugins: internals

How the plugin system is built. Using and authoring plugins, including the op reference, is in the website guide [website-docs/guides/plugins.md](../website-docs/guides/plugins.md); this document does not repeat it. The guide's code is [examples/plugins/hello](../examples/plugins/hello), the smallest complete plugin; `TestGuideQuotesThisPlugin` keeps the two identical. The plan for finishing the branch is in [plugins-v1-plan.md](plugins-v1-plan.md).

## Model

A plugin is a Go module with a build half (`<module>/plugin`, run at build time inside a generated bridge binary, records **ops**), native Swift/Kotlin embedded in the build half, and a runtime half (Go API over platform channels). This combines Expo's prebuild model (declarative changes applied to a managed native project) with Flutter's generated plugin registrant (one generated list of plugin classes the host drives).

Package layout:

| Package | Holds |
|---------|-------|
| `pkg/plugin` | Author API: `Plugin[T]`, `BuildCtx` and its recorder scopes, config schema from `drift` tags, test contexts, bridge `Main`/`Bind` |
| `pkg/plugin/protocol` | The CLI-to-bridge wire contract: envelope, response, op types (`ops.go`), validation (`validate.go`), config schema (`schema.go`), `Color` |
| `cmd/drift/internal/plugin` | CLI side: drift.yaml parsing (`manifest.go`), deps (`deps.go`), bridge build/run (`bridge.go`), conflicts (`conflict.go`), op kinds (`opkinds.go`), project paths (`project.go`), apply (`apply.go`), registrant and host support files (`registrant.go`), ejected checks (`ejected.go`) and lock (`lock.go`), `sync`/`list` (`sync.go`) |
| `cmd/drift/internal/plugin/mutate` | One mutator per file kind: plist, AndroidManifest and resource XML, Gradle, sources, bundle resources, the `Drift/Plugins` Swift package, `xtool.yml` |
| `cmd/drift/internal/templates/plugin-api/ios`, `templates/android/runner` | The native plugin API (`DriftPlugin`, `DriftPluginHost`, bindings, `DriftResult`) |

## Build pipeline

`workspace.Prepare` (`cmd/drift/internal/workspace/workspace.go`) scaffolds the managed project (regenerated every build under `~/.drift/build/...`), then:

1. **`resolvePluginOps`** runs every build half without touching the native project:
   1. `LoadFromDriftYAML` reads `plugins:` (YAML anchors resolved, an empty list allowed).
   2. `CheckPluginDeps` runs `go list` per package; a missing one fails with a `go get` hint, at the CLI's version for a first-party plugin under a release CLI and `@latest` otherwise.
   3. `EnsureBridge` writes `tools/drift-plugins/main.go` (build tag `drift_tool`, committed by the app so `go mod tidy` keeps plugin deps; keyword-safe import aliases) and builds it.
   4. `RunBridge` sends the envelope on stdin (`APIVersion`, `cmd`, platform, project root, build dir, `app_id`, config YAML per plugin) and reads ops from a response file.
   5. `protocol.DecodeOps` decodes and validates each op again; `Validate` (`conflict.go`) checks ops against each other by target and drops exact duplicates.
2. **`applyPluginOps`** writes them. Steps 2 to 4 plan into one `mutate.Changeset` and write nothing; step 5 commits it, so an op that fails anywhere leaves the project as it was:
   1. Ejected projects only: `CheckEjectedIOS` / `CheckEjectedAndroid` fail early if the project lacks the template calls plugins need.
   2. `Apply` files ops by platform (`opAppliesTo`: iOS ops apply to `ios` and `xtool`) and runs the mutators. A mutator reads through the changeset, so it sees what an earlier one planned for the same file.
   3. `EnsureRunnerSupport` plans the Drift-owned host files (`DriftPlugins`, plugin API), `WriteRegistrant` the generated `DriftPluginRegistrant` (plugin classes in drift.yaml order).
   4. Ejected projects only: `SyncEjectedLock` (see [Ejected projects](#ejected-projects)) plans its deletions and, last, the new lock.
   5. `Changeset.Commit` removes, then writes each file whose bytes changed in the order planned, so the lock lands after the output it records.

With zero plugins the bridge is skipped and `tools/drift-plugins/main.go` deleted, but the registrant, sidecar package and resource directories are still reset so generated projects always compile.

Watch mode: `Refresh` re-runs `resolvePluginOps` and, when the hash of the op list changed, regenerates the project.

## Bridge

- Built with `-mod=readonly`: the CLI never edits `go.mod`/`go.sum`; a missing entry fails with a `go mod tidy` hint (`drift plugin sync --tidy` runs it).
- A configured package the module graph lacks fails before the build (`CheckPluginDeps`, `MissingPluginError`): a third-party one with a `go get ...@latest` hint; a first-party one, being in the drift module, with a wrong-path or update-drift hint naming the CLI's version.
- Cache: binaries live under the cache root, keyed by a hash of everything that feeds the build (CLI and protocol versions, `bridgeTemplateVersion`, Go version, GOFLAGS, plugin module pins, bridge source, `go.mod`, `go.sum`, active `go.work`). Builds write a temp name and rename into place.
- When any source is local (a plugin in the main module, a directory `replace`, including a local `github.com/go-drift/drift`, or a multi-module `go.work`), the key cannot see edits, so the bridge is rebuilt every time into a fixed per-project path; the Go build cache keeps that cheap.
- The bridge refuses an envelope whose `APIVersion` differs, and the CLI refuses a response whose does. An op type the CLI does not know fails decoding.
- Every response carries `drift_version`: the `github.com/go-drift/drift` version the bridge was compiled with, from its build info (`(devel)` for a local checkout). See [Versions](#versions).
- `drift plugin sync` (`Sync`) checks config without a build: the `schema` command and `ValidateConfig`, then, if those pass, a `build` envelope per plugin with platform `"all"` and no build dir (`checkBuilds`), discarding the ops. A plugin's failed `Build` comes back as the response's `plugin_error` (`protocol.PluginError`), which sync reports as a diagnostic; `error` is for requests the bridge cannot serve.
- The bridge panics at start on an invalid `Name()` or a malformed `drift` tag (`Bind`) and on a duplicate plugin name or package (`Main`), so author errors surface on the first build.

## Versions

First-party plugins are packages of the root module (`plugins/<name>/plugin` and `/runtime`), so they release with Drift: a release tags only `vX.Y.Z` (the release workflow, after the Skia build).

The other modules in the repository (examples, showcase) are not released. Each keeps `replace github.com/go-drift/drift` pointing at the checkout, so `go mod tidy` works in each (it ignores `go.work`); there is no `go.work`. Their drift requirement is not bumped per release. `examples/plugins/hello`, the in-repo third-party-style plugin, requires the oldest drift whose plugin API it uses; `TestHelloRequiresAServedPluginAPI` keeps that at or above `PluginAPIFloor`.

A release CLI checks compatibility (`cmd/drift/internal/plugin/compat.go`); a development build (`cache.NormalizeVersion` empty) skips it:

- **Per plugin**, in `EnsureBridge` before building: the drift version the plugin module's `go.mod` requires (read from `go list`'s `Module.GoMod`) is the plugin API it was written against. The plugin API changes only in a minor release, so versions compare by major.minor: a major.minor newer than the CLI's, or older than `templates.PluginAPIFloor`'s, fails with a `VersionSkewError` naming every offending plugin. Plugins in the main module or in the drift module (first-party) are left to the next check.
- **Bridge**, in `RunBridge`: a `drift_version` whose major.minor differs from the CLI's fails, since its op set and the app's runtime may not match the templates.

Raise `PluginAPIFloor` in the release that breaks plugin native code (`plugin-api/ios`, `android/runner`, channel semantics).

`scripts/install-check.sh <version>` rehearses a release before tagging: HEAD served from a file `GOPROXY` (`scripts/fileproxy`) as the release and the next minor, the CLI installed from it, a fresh app with `plugins/splash` built per platform, and both skew errors against a copy of `examples/plugins/hello` served as the third-party module `example.com/hello`; `--real` repeats the install against the published tag.

## Ops and conflicts

Ops are typed structs in `protocol/ops.go` with `Type`, `Platform`, `Targets` and `Validate`, serialized as JSON with a `type` discriminator. Recorders (`pkg/plugin/buildctx.go`) validate at record time into `ctx.Err()`; the CLI validates again at decode, so mutators only see valid ops.

Each op declares the **targets** it writes: a key naming a location (`plist:<file>:<key>`, `android-res:<type>/<name>`, `ios-bundle:<file>`, `spm:<package>`, ...), optionally a member of a set at that key, and a hash of the content. `Validate` in `conflict.go`:

- Owners of one key must write the same content; they collapse to one.
- Members of a set merge; the same member twice must agree.
- A key cannot be both owned and added to (`set_string_array` vs `append_array_item`).

Some ops claim more than one key: an image set also claims its loose `<Name>.png` bundle name (xtool), a non-values resource XML file also claims the resource its file name defines, a Swift source also claims its basename within the plugin's module.

On the CLI side, each op type has one `opKind` entry (`opkinds.go`): the shared files it edits and what it changes there (the ejected lock's edit text and the ejected-project checks), the whole files it owns (written as they are by `Apply` and recorded by the lock), the exact line of a versioned Gradle op, and the mutator batch it feeds. Every project location lives on `project` (`project.go`), so `Apply`, the lock, the registrant and the ejected checks share one path for each file.

Adding an op means touching: the struct and its four methods plus the constructor table (`protocol/ops.go`), a recorder (`buildctx.go`), the fixture (`protocol/ops_test.go`), its `opKind` entry, a mutator if it writes a new kind of file, and the guide's op reference. `TestOpsCoverAllConstructors`, `TestEveryOpTypeHasAKind` and `TestOpReferenceListsEveryOp` catch omissions.

## iOS specifics

- **Two build paths.** `ios` is xcodeproj on macOS (the shipping path); `xtool` is SwiftPM plus the xtool packer on Linux (development only).
- **Plugin modules.** Both paths link the `DriftPlugins` product of the Drift-owned local package at `Drift/Plugins` (`mutate/spm.go`). It holds a `DriftPluginAPI` target (from `templates/plugin-api/ios`) and one `DriftPlugin_<name>` target per plugin, containing its sources and depending on `DriftPluginAPI` plus the SwiftPM products it requested. The generated registrant imports each module and creates `DriftPlugin_<name>.<Class>()`. The app imports `DriftPluginAPI` and constructs host-only types through `@_spi(DriftHost)`.
- **Bundle resources** land flat in the bundle root: `Runner/PluginResources/` on xcodeproj (synchronized folder) and `PluginResources/` listed under `xtool.yml` `resources:` on xtool. SwiftPM target resources are avoided because they land in `Runner_Runner.bundle`, which `Bundle.main` cannot see.
- **Image sets on xtool** become loose `<Name>.png` files, since xtool cannot compile asset catalogs. `FUTURE(xtool#219)` comments mark what changes if [xtool#219](https://github.com/xtool-org/xtool/pull/219) merges.
- **Entitlements.** `Runner.entitlements` at the project root on both paths (`CODE_SIGN_ENTITLEMENTS`, xtool `entitlementsPath`).
- **App-level hooks.** `AppDelegate.swift` (also on xtool) forwards launch and remote-notification events to `DriftPlugins`. Drift's `UNUserNotificationCenter` delegate (`NotificationHandler` in `PlatformChannel.swift`) offers foreground notifications and responses to plugins, which claim their own (local notifications by a userInfo marker, Firebase by `gcm.message_id`); an unclaimed notification is not shown in the foreground, as without a delegate. A remote notification nobody claims completes with `.noData`. Hooks exist only where a plugin needs them.

## Native host

`DriftPlugins` (iOS `templates/ios/DriftPlugins.swift`, Android `templates/android/runner/DriftPlugins.kt`) owns the plugin instances and drives the lifecycle described in the guide; it is Drift-owned and rewritten every build. Each plugin registers with its own `PluginHost`, which forwards to `PlatformChannelManager` (Drift's own host, whose `config` is empty) and adds:

- `config`: the plugin's `ctx.NativeConfig`, recorded as `ios.plugin_config` / `android.plugin_config` and shipped as `DriftPluginConfig-<name>.json` in the bundle root (`Runner/PluginResources`, xtool `PluginResources`) and the asset `drift/plugins/<name>.json` (Drift-owned `app/src/main/assets/drift/plugins`, pruned every build like the bundle resources), read at registration. Android decodes it with the runner's `JsonCodec` (`DriftJsonCodec.kt`), the host's own codec. A file that is not a JSON object crashes.
- The `drift/` channel reservation: the registrant lists each plugin as a `DriftPluginEntry(name, firstParty, plugin)`, `firstParty` for a package under `github.com/go-drift/drift/plugins/` (`firstParty` in `registrant.go`), and `registerChannel` traps on a `drift/` name from any other plugin.

Callers:

| | iOS | Android |
|---|---|---|
| Register (once) | `AppDelegate.didFinishLaunching` calls `DriftPlugins.shared.launch` | `DriftApplication.onCreate` (the manifest's `android:name`) calls `PlatformChannelManager.init` and `DriftPlugins.register`, so plugins exist in processes started for a service or receiver |
| Attach / detach | `DriftViewController.viewDidLoad`; `tearDown` (from `SceneDelegate.sceneDidDisconnect`, or `dismantleUIViewController` on xtool) detaches, as does `deinit`. The display link targets a weak proxy, so the controller is released | `MainActivity.onCreate` / `onDestroy`, with the window's decor view as overlay host; `MainActivity` decides once whether its intent is a new launch (not recreated, not from Recents); `DriftPlugins.attach` offers a new launch's intent to the plugins' intent listeners after every `onAttach`, and only an unclaimed one reaches `DeepLinkHandler`, as on iOS |
| Events | `AppDelegate`, `NotificationHandler`; URLs and user activities from `SceneDelegate` (xtool: `DriftApp`'s `onOpenURL` / `onContinueUserActivity`), offered to plugins before `DeepLinkHandler` | The launch intent (`attach`) and `MainActivity.onNewIntent`, offered to plugins before `DeepLinkHandler` |

Method calls cross as callbacks keyed by call ID. Go starts the call on the calling thread, never a pre-spawned goroutine (`pkg/platform/registry.go` `invokeNative`; the bridge template's `InvokeMethod`), so native sees whether the caller is the main thread. Native answers through `DriftPlatformReply(callID, ...)` exactly once, in place or later from any thread; Go waits for the reply or `ctx.Done()`, and a late reply is discarded.

- Every channel, Drift's and plugins' alike, declares each method's reply kind (`DriftChannel`); Drift's own register first, through the same `registerChannel`, so every handler runs on the main thread and UIKit and handler state are touched only there. `method` returns its reply: in place on the main thread, otherwise posted there. `asyncMethod` replies through `DriftResult`; called on the main thread it fails with `blocks_ui_thread` before running, since Go is blocking the main thread. So the main thread never waits on itself; a goroutine's call waits about one turn of the main queue, so Go code must not hold a lock the UI thread needs across a call (none in `pkg` does: the per-frame `drift/accessibility` calls and accessibility setup run on the UI thread).
- A `DriftResult` released without a reply answers `reply_dropped` (iOS `deinit`, Android `PhantomReference` reaper). A second reply crashes on both platforms.
- Native error codes match Go sentinels under `errors.Is` (`ChannelError.Is`, `pkg/platform/codec.go`): the transport codes (`transportErrorCodes`, which every channel can answer) and the channel's own `ErrorCodes`, attached by `MethodChannel.Invoke` and `EventChannel.dispatchError`. Drift's feature codes live on their channels too (`storage_busy` on `drift/storage`, `permission_*` on `drift/permissions`, `view_type_not_found` on `drift/platform_views`, `platform_unavailable` on each channel that answers it).
- Native encoding is total: each `JsonCodec.encode` returns the bytes or why the value is not JSON-like (NaN, Infinity, an unsupported type). Every reply passes through one encode point (iOS `handleMethodCall`'s `answer`, Android `replyNative`), so an unencodable reply answers `not_encodable` instead of hanging Go or raising an uncatchable exception; an unencodable event goes to `DriftPlatformHandleEventError` with the same code. iOS tells booleans from numbers by `CFBoolean`, since an `NSNumber` 0 or 1 also matches `as Bool`.

## Permissions

Drift owns the request mechanism; permission kinds come from plugins. `PermissionHandler` (iOS `templates/ios/PermissionHandler.swift`, Android `templates/android/java/PermissionHandler.kt`) holds the registry, fed by `DriftPluginHost.registerPermission(kind, permission)`, and registers the `drift/permissions` channel through the same `registerChannel` plugins use: `check`, `shouldShowRationale` and `openSettings` reply in place, and `request` is an async method whose reply is the resulting status (so the UI-thread guard applies). After a request it also sends `drift/permissions/changes` for `Permission.Listen`. `PermissionHandler` shows one prompt per kind at a time: a `request` arriving while that kind's prompt is up joins it and gets the same answer (its own options unused). The bookkeeping lives on the main thread, not behind a Go lock, so a waiting goroutine still honours its ctx and a UI-thread call is still refused at once. Go reaches a kind with `platform.NewPermission(kind)`.

- A kind nobody registered fails with `permission_not_registered` (`platform.ErrPermissionNotRegistered`); registering a kind twice crashes at startup.
- `request` is skipped when no prompt can change the status: on iOS when it is granted, permanently denied or restricted; on Android only when granted, since Android's "don't ask again" is inferred and the system answers at once when it will not prompt.
- iOS kinds implement `DriftPermission` (`templates/plugin-api/ios/DriftPermission.swift`) and must complete even when iOS sends no callback (no prompt shown, Always upgrade declined).
- Android kinds implement `DriftPermission` (`templates/android/runner/DriftPermission.kt`); most use `DriftRuntimePermission(manifestPermissions...)`, which requests through the Activity's `activityResultRegistry` (no request codes), infers "permanently denied" from the rationale flag having been raised (by a first denial) and lowered again (a dismissed dialog changes neither), reads LIMITED when only some of its permissions are granted, and fails with `permission_not_declared` (`platform.ErrPermissionNotDeclared`) when the merged manifest lacks one.
- Drift registers one kind itself: `notifications`, shared by every plugin that shows notifications. Those plugins declare `POST_NOTIFICATIONS`.

## Platform views

`PlatformViewHandler` (iOS `templates/ios/PlatformView.swift`, Android `templates/android/java/PlatformView.kt`) keeps a factory per view type, registered at launch through `DriftPluginHost.registerPlatformView`; Drift registers its own (text input, switch, activity indicator, web view) the same way. The `drift/platform_views` channel is registered like a plugin's (`registerChannel`). `create` is a `method`: it runs on the main thread (in place from the UI thread, otherwise Go waits), fails with `view_type_not_found` for an unknown type, and replies with the factory's outcome, so a throwing factory fails Go's `Create` and leaves no view. Every later operation (`invokeViewMethod`, `setVisible`, `setEnabled`, `dispose`) is a `method` too whose body posts the work to the main thread, after earlier work, and replies at once, as before: Go does not wait for view methods. Views and their dictionaries are main-thread only.

Views outlive their host. iOS: every view's interceptor exists from creation; `setHostView` (each `DriftViewController.viewDidLoad`) moves them into the new view below its overlay, and `removeHostView` (from `tearDown`) takes them out of a disconnected scene's view. Android: each view is created with its own `MutableContextWrapper`; `PlatformViewHandler.init` (each `MainActivity.onCreate`) points it at the new Activity and moves the interceptors into the new overlay layout, and `detachHost` (from `onDestroy`) takes them out and points the wrappers at the Application, so a destroyed Activity is not leaked. Moved views stay hidden until the next frame's geometry. A view created while no host exists waits for the next one. A view's context (`PlatformViewContext`) has its own `LayoutInflater`, cloned from the Application's: through the Activity's, inflated children (PlayerView's) took the Activity itself as their context, and that inflater holds the Activity as its private factory. Objects a view or library built from the old Activity (PlayerView's `PopupWindow`) are the view's to rebuild in `DriftPlatformView.onHostChanged` (both platforms), called after each move in and out. Framework caches remain (`View.mVisibilityChangeForAutofillHandler`'s `AutofillManager`, Chromium's `AwContents` autofill provider): each keeps the Activity its view was created in until the view is disposed, bounded at one per view. Measured on the API 36 emulator with `dumpsys meminfo` after `am dumpheap` and a GC-root path from the heap dump: video and text-input pages hold one Activity across eight recreations, and none once the page is left. A view's `handle` errors are logged. Events go to Go on the `drift/platform_views` event channel with `method` and `viewId`; the Go registry handles `onViewCreated` and routes everything else to the view's `HandleEvent`.

## Ejected projects

Ejected projects (`platform/ios`, `platform/android`) run the same pipeline against user-owned files.

- `ejected.go` checks the template calls that feed `DriftPlugins` and, on iOS, the `Drift/Plugins` package reference; the error names each missing call.
- `lock.go`: `ReadLock` reads `.drift/plugins.lock.json` from the previous build (committed by the user) before applying; paths in it must stay inside the project. After `Apply` plans, `SyncEjectedLock` compares it with the current ops by what the project holds, not by op identity, so ops renamed or retargeted by a CLI upgrade are neither removals nor additions:
  - a file the lock owned that no current op owns is deleted, unless the user changed it (then kept, with a warning); a template file a plugin replaced (`templateDefault`: the launch screen, which `Info.plist` names) is restored to the template's content instead;
  - an edit inside a shared file (plist keys, manifest entries, Gradle lines) whose key no current op has and which no current op makes is returned to `applyPluginOps`, which lists it in a warning after the commit for the user to undo, since Drift cannot tell it from user edits. It is not an error, so a build's outcome does not depend on whether the previous lock was committed.
- Keys exclude content, so a plugin upgrade is an update. A Gradle dependency is keyed by `Artifact()` (no version). The lock records the exact line each versioned Gradle op wrote (`line`); when the same plugin's op writes another line and the project still holds the old one verbatim, the mutator replaces it (`Lock.gradleReplacements`). Lines the user changed are left alone.
- `Apply` never overwrites a file Drift did not write (`checkOwnedWrites`, `ForeignFileError`): an owned file that exists with other content is written only if the previous lock records that content, or, for a template file a plugin replaces, it still holds the template's (`templates.DefaultLaunchScreen`, which the scaffold also writes). Managed builds pass no lock and start from a fresh scaffold, so a plugin colliding with a template file fails there too. Kotlin sources may not use `com.drift.runner`.
- Every project write is atomic (`mutate.WriteFileAtomic`: a temporary file renamed into place), so an interrupted build never leaves a torn user file or lock.
- Plugin values XML (`values/plugin_*.xml`) and the `Drift/Plugins` package are regenerated every build.

## Runtime side

Runtime packages use `pkg/platform` channels: `NewMethodChannel`, `NewEventChannel`, `NewStickyEventChannel` (last event replayed to each new subscriber; `engine.FrameEvents` uses it for `first_frame`), `NewQueuedEventChannel` (events kept until a subscriber takes them, once each), and `NewStream` for typed parsing. The guide covers when to use which.

Native stream lifecycle: an `EventChannel` (other than an `Emitter`'s) tells native when it gains its first subscriber and loses its last, as `listen` / `cancel` `method`s on Drift's `drift/streams` channel carrying the channel name and a per-channel generation (`tellNative`, `pkg/platform/channel.go`); no Go lock is held across the call, since a goroutine's call waits for the main thread. Native (`StreamLifecycle` in `PlatformChannel.swift` / `.kt`) records the request unless a newer generation arrived first (calls from different goroutines can arrive out of order), replies, and then, posted to the main thread so the first events follow `Listen`, runs the `onListen` / `onCancel` the channel of that name declared (`DriftChannel.stream`) when its running state differs from Go's: strictly alternating, `onCancel` only after an `onListen` that did not throw, a throw sent as the stream's error. A request for a channel not yet registered is applied when it registers; `sendEventDone` resets the state without `onCancel` (Go's subscriptions end with it, sending nothing). Subscriptions made before native connected (package init) fail with `ErrPlatformUnavailable` and are replayed by `HandleNativeConnected`, which the bridge calls from `DriftPlatformSetNativeHandler`.

A `Stream` delivers events and errors to each `StreamHandler`. Native stream errors (`sendEventError`) cross the C bridge as the same JSON payload a failed call replies with (`DriftPlatformHandleEventError(channel, errorJSON)`), so they carry code, message and details and match the event channel's `ErrorCodes`. `Emitter` feeds a `Stream` from Go through an unregistered `EventChannel`, which native never addresses.
