# Plugins v1: plan

Handoff document for finishing the `feat/plugins` branch. Read the website guide [plugins.md](../website-docs/guides/plugins.md) for using and authoring plugins, and [plugins.md](plugins.md) for internals. **Phases 1 to 4 are done, and the fixes from the second review (2026-10-02) have landed; the branch is ready to merge once the new method-call transport has run on the iOS Simulator (see Second review).** Device runs not yet done are accepted on the strength of the emulator and Simulator runs (decision 2026-10-02, see Phase 4 decisions). Phase 5 is next. Open work is tracked in [Known follow-ups](#known-follow-ups), split into what must land before the first release that ships plugins and what can wait. Line numbers in the review findings are from commit `261864d` and may drift; many of those findings are now fixed (see Status).

## Context

**Branch state** (`feat/plugins`, 4 commits over `master` at `9a177df`):

| Commit | Content |
|--------|---------|
| `e1ceadc` | Plugin system: bridge, ops, mutators, registrants, `drift plugin sync/list`, demo plugin |
| `8219a67` | Splash plugin, overlay host, sticky event channel, `first_frame` event |
| `83cbe3f` | Gradle mutator helper refactor |
| `261864d` | SDK integration ops (SwiftPM sidecar, bundle resources, app-level iOS hooks + background-fetch coordinator, Gradle `apply plugin`, Android assets and app-module files), xtool main-bundle resources, splash enabled on xtool |

`go vet ./...` and `go test ./...` pass. **Nothing native has been verified on a device or compiled on macOS.**

**Environment:**
- Development happens on Linux (Fedora). iOS verification needs a Mac with Xcode 16+.
- xtool is the Linux iOS dev path only; shipping builds use xcodeproj on macOS.
- No production apps exist yet, so breaking changes to app-facing APIs are fine.

**Merge bar:** the branch merges when **two plugins work on real devices, iOS (xcodeproj) and Android**:
- `plugins/splash`
- a new `plugins/firebase`

A plugin system that cannot support Firebase is not worth merging. Firebase is the consumer that justifies each SDK op; ops it does not need get deleted.

**Decisions already made:**
- Push notifications move out of core into the Firebase plugin.
- Core features that add permissions, usage strings, entitlements, background modes or heavy dependencies migrate to plugins (see [Core feature migration](#core-feature-migration)).
- Ejected iOS projects missing wiring fail with an error naming the fix; the CLI does not auto-patch user Swift or pbxproj files.
- xtool has no asset catalog support. Workarounds are marked `FUTURE(xtool#219)`. Do not depend on xtool#219 landing.

## Status (2026-10-01)

**Phase 1 is complete** (`76a807e`..`bb740e5`, 15 commits, not yet pushed):

| Commit | Content |
|--------|---------|
| `76a807e` | Wire protocol moved to `pkg/plugin/protocol` |
| `e75c56d` | Every op validates at record time (into `ctx.Err()`, no panics) and at decode; path traversal closed |
| `9349cbe` | One recursive config check (`required` = key present, `default=`, `hex`, `asset`) shared by sync and build |
| `40a1928` | Conflicts keyed on targets (`Op.Targets()`), merge classes deleted; plugins can share a SwiftPM package with different products |
| `e395f20` | Bridge: `-mod=readonly` with a `go mod tidy` hint, atomic cache writes, no temp-dir leak, cache key covers go.mod/go.work/GOFLAGS/toolchain and local replaces |
| `fad4afb` | Keyword-safe bridge aliases, YAML anchors resolved, bare `plugins:`, `list --resolve` fixed |
| `7ce9a46` | Registrants in drift.yaml order |
| `f294d68` | Fix: xtool referenced the sidecar product by the wrong name (every xtool build failed) |
| `e921388` | Plugin objects with a lifecycle: iOS `DriftPlugin` (register, didFinishLaunching, attach/detach, URL/activity/remote-notification hooks), Android `DriftPlugin` (onRegister once per process, onPreActivityCreate, onAttach/onDetach with intent/result/permission listeners). `DriftPlugins` owns them; codegen only lists classes; built-in deep-link/notification handling lives in templates. Ops `ios.plugin`/`android.plugin` replace the four registrant ops |
| `d88465d` | Async method results (`DriftResult`); all plugin callbacks on the main thread; fail fast when Go calls a pending handler from the main thread; Android JNI error slot thread-local |
| `e594a06` | Watch-mode `Refresh` regenerates the managed project when the plugin op hash changes |
| `8fa8888` | Ejected projects: `.drift/plugins.lock.json`; removed plugins' owned files deleted (unless user-modified); in-file edits listed in a build error shown once; plugin values XML regenerated |
| `2d3887f` | One SwiftPM module per iOS plugin (`DriftPlugin_<name>`) plus `DriftPluginAPI` in the Drift-owned `Drift/Plugins` package; plugin classes public; host-only initializers behind `@_spi(DriftHost)` |
| `ee6fe00` | Fix: Simulator builds (no `addPresentedHandler` in the Simulator SDK; `first_frame` falls back to command-buffer completion there) |
| `bb740e5` | Fix: splash launch storyboard that ibtool compiles |

**Verified:** `go vet`/`go test` in all modules; `drift build android` of splash-demo (with and without `android_12`); `drift build xtool` of splash-demo and of a zero-plugin app (Linux, iOS 26.5 SDK); `drift build ios` of splash-demo on Xcode 26.5 (simulator). Firebase spike (branch `spike/firebase-ios`, Mac): firebase-ios-sdk resolves, and both loose sources and per-plugin targets compile and link, with and without explicit modules.

**Verified since:** `xcodebuild test` in `cmd/drift/internal/plugin/coordinator_test` passes (scheme `DriftPluginCoordinatorTests-Package`).

**Deviations from the original phase 1 plan:** removed plugins' in-file edits in ejected projects fail the build once instead of being probed until removed (a user may want to keep an entry); the watch-mode op hash lives on the in-memory `Workspace`. Android stays loose-source (no per-plugin Gradle modules); Firebase's `<service>` needs a manifest op in phase 3.

**New plugin-author rules:** `Plugin.Name()` is a lowercase identifier (it names the iOS module); iOS plugin sources are Swift only; the iOS plugin class is `public` with `public init()`; channels declare each method as `method` (replies by returning) or `asyncMethod` (replies through `DriftResult`; Go must call it off the UI thread).

**Phase 2 is done** (`7e0178c`..`561a0a5`), verified on an iPhone (xcodeproj) and the iOS Simulator, and on an API 36 emulator. Not yet on a real Android device.

| Commit | Content |
|--------|---------|
| `7e0178c` | Engine: `NeedsFrame` consumes the pending platform schedule. iOS skipped `StepFrame` while OnInit ran, so a frame request then left the flag set and the OnInit completion never woke the display link (black screen forever with a slow OnInit) |
| `7e917a6` | Engine: `first_frame` only for the first frame that composited the root (`HasRenderedContent`); Android emits after that `renderFrameSync` rather than waiting for a View draw |
| `a94eeb6` | `DriftViewBinding.overlayView` / `DriftActivityBinding.overlayView`: plugin overlays stay above platform views |
| `1ba5474` | `protocol.Color` / `driftplugin.ParseColor`: Drift hex is alpha last; `android.color.set` validates it and writes Android's alpha-first form |
| `fdaebc0` | Splash: `branding*`, `dark`, `android_12.branding` removed (never emitted; iOS has no appearance-aware asset ops) |
| `d5ddeb8` | Splash: `image_width`; storyboard and iOS overlay generated from the same values and pinned with constraints |
| `18b2f6c`, `3137839` | Splash iOS: dismissal is state (`reconcile()` on every input, no `applicationState` guard); `max_duration_ms` (default 10s) overrides Preserve but never a slow OnInit; `Preserve`/`Remove` return errors |
| `aa14136` | Coordinator harness instructions |
| `dfa591b` | Android toolchain: AGP 9.4.1 (built-in Kotlin), Gradle 9.8.0, compile/target SDK 36, **minSdk 31** (`templates.AndroidMinSDK`, also the NDK clang target) |
| `ecadc57` | `AddBitmap` writes to `drawable-nodpi` |
| `561a0a5` | Splash Android: the platform splash screen (`Drift.Splash` style over `LaunchTheme`), held with a pre-draw listener; `android.icon` / `android.icon_background_color`; overlay, `android_12` and `core-splashscreen` removed |

**Phase 2 decisions:** minSdk 31 (Vulkan-only renderer; an API 29 emulator crashes in its Vulkan driver; 31 brings the platform splash API). Android's splash is the platform icon splash, so it does not match iOS's `image_width` layout. A recreated Activity (dark mode, locale) ends the splash, since it gets no platform splash.

**Phase 3 is done** (`784571c`..`75971b7`); remaining device runs accepted in Phase 4 (see Verification matrix).

| Commit | Content |
|--------|---------|
| `784571c` | `platform.NewQueuedEventChannel`: events sent while nobody listens wait for the next subscriber; `ResetForTest` clears sticky slots and queues |
| `f480ce3` | `ctx.AppID()` (envelope `app_id`, required for build) |
| `3558506` | Entitlements: `info_plist.*` became `ios.plist.*` with a file (`info`/`entitlements`); `Runner.entitlements` at the project root on both iOS paths (`CODE_SIGN_ENTITLEMENTS`, xtool `entitlementsPath`); appending to a non-array plist value errors |
| `1454635` | `android.manifest.add_service`; manifest mutator takes `ManifestOps` |
| `329c431` | Deleted: `android.assets.add`; iOS `open`/`continueUserActivity` and `DeepLinkHandler.route`; Android `onPreActivityCreate` and activity/permission-result listeners; `DriftPluginCoordinator` and its harness (remote notifications are claimed, not merged) |
| `67d69ee` | iOS `willPresentNotification` / `didReceiveNotificationResponse`, offered by Drift's notification-center delegate before local handling |
| `fa68886` | Push left core (Go API, both iOS paths, Android handler, Gradle Firebase deps, `remote-notification` mode) |
| `27167c5` | `plugins/firebase` (Core + Messaging; Go API `runtime/messaging`: `Token()` state, `Messages()`/`Opens()` queued streams); `NewTestCtxFor` |
| `75971b7` | `examples/firebase-demo` and `tools/fcmsend` (FCM HTTP v1 sender, stdlib only) |

**Verified (Linux):** `go vet`/`go test` in every module; `drift build android` and `drift build xtool` of splash-demo; `drift build android` of firebase-demo with placeholder config (google-services 4.5.0 runs on AGP 9; manifest service, permission and dependency merge); **FCM on the API 36 emulator, real Firebase project** (`fir-demo-71e74`, sent with `fcmsend`): token delivered to Go; foreground notification and data messages reach `Messages()` with nothing shown; in the background the notification is shown and the data message reaches `Messages()` (`foreground=false`); tapping opens the app and reaches `Opens()`, both with the process alive and after it was killed (the tap that launched it is queued until the app listens); dark-mode Activity recreation replays no tap. **Mac (reported 2026-10-02):** `drift build ios` of splash-demo and firebase-demo; splash on the Simulator; firebase-demo on the Simulator with `xcrun simctl push` payloads carrying `gcm.message_id`: foreground message reaches `Messages()` with no banner, background banner tap and cold-start tap reach `Opens()`. **Not yet:** a real Android device; real iOS delivery through FCM (needs the APNs `.p8` uploaded; config only, no code).

**Phase 3 decisions:** local notifications and the notification permission stay core until a notifications plugin exists (Phase 5); core keeps the one `UNUserNotificationCenterDelegate` and offers notifications to plugins first. ~~Firebase on xtool is an error~~ (reversed in Phase 4: plugins must build on xtool; see Phase 4 decisions). Foreground FCM notifications are not shown on either platform; the app gets them in `Messages()`. Firebase Go API has no blocking calls. Android pins `firebase-messaging` without the BOM (one artifact).

**Phase 4 is done** (`c5d464d`..`4ddd72c`):

| Commit | Content |
|--------|---------|
| `c5d464d` | Core local-notification taps queued until the app listens (`drift/notifications/opened` is a queued channel); Android does not replay the launch tap on Activity recreation or a Recents relaunch |
| `740d718` | Android edge-to-edge under targetSdk 36 (a regression from `dfa591b`): always edge to edge (`enableEdgeToEdge`, appcompat 1.7.1); system UI style is state, reapplied to a recreated Activity; `StatusBarStyleDefault` follows the system theme. `SystemUIStyle` loses the no-op `TitleBarHidden`, `BackgroundColor`, `Transparent` |
| `9bdd240` | Android API checks below minSdk 31 removed, with the unreachable `ErrPlatformNotSupported`; `isMock` replaces `isFromMockProvider`; docs say Android 12 (API 31) |
| `05679cd` | `BuildCtx.Xtool` deleted (unused); stale API comments fixed; `Binding.Build` documented as the test entry point |
| `328f8ad` | Website guide `website-docs/guides/plugins.md` (using, authoring, op reference) with `TestOpReferenceListsEveryOp`; eject guide points to plugins and covers ejected plugin output |
| `c0c1580` | `docs/plugins.md` is internals only; `examples/plugins/demo` deleted |
| `a3053b8` | CI vets and tests `plugins/*` (separate modules) |
| `36c37c8` | Watch mode ignores the generated bridge (adding or removing a plugin rebuilt twice) |
| `3536c16` | Firebase builds on xtool (the iOS integration); push needs a paid team there, as with Xcode |
| `4ddd72c` | `examples/plugins/hello`: the smallest complete plugin, replacing the deleted demo; the guide quotes it, `TestGuideQuotesThisPlugin` keeps the quotes verbatim, CI runs it. Verified on the emulator (greeting reaches Go) and with `drift build xtool` |

**Verified (Linux, Phase 4):** `go vet`/`go test` in the root and plugin modules; Docusaurus build (no broken links from the new pages); edge-to-edge on the API 36 emulator (showcase: icons follow the app theme, and an explicit style survives dark-mode recreation); `drift build xtool` of splash-demo (shared Swift compiles); plugin removal leaves a compiling project: managed Android, watch mode on the emulator (one rebuild), ejected Android (owned files deleted, the theme edit listed once, compiles after undoing it), managed xtool.

**Phase 4 decisions:** follow-ups that land before merge are the ones this branch caused (edge-to-edge from targetSdk 36, dead API checks from minSdk 31) plus the one-line cold-start tap fix; the rest are post-merge (see Known follow-ups). `go test -race ./pkg/engine` fails the same way on `master`, so it is a separate fix. **Plugins never reject xtool**: it is how iOS apps are developed on Linux, so blocking it blocks development of any app using the plugin; what cannot work there is documented instead (release builds use Xcode). Firebase therefore builds on xtool (verified: firebase-ios-sdk compiles and links, the bundle carries `GoogleService-Info.plist` and the signature `aps-environment`); push needs a paid team, as with Xcode. This settles unresolved question 3. **Device runs:** the remaining real-device checks (Android phone, iPhone push through FCM with the APNs key, a final iPhone pass) are accepted without running them: the same code passed on the API 36 emulator and the iOS Simulator, and nothing in it differs on hardware. Upload the APNs `.p8` before relying on iOS push.

**Deviations from the Phase 3 plan:** no background-modes op (`append_array_item` does it); build half and native/runtime halves landed in one commit (the build half embeds the native sources); `core.Watchable` does not exist, so `Token()` returns a read-only `*core.Derived[string]`.

### Second review (2026-10-02)

A second three-part review (design, pipeline correctness, native runtime) checked the code rather than this Status table. It confirmed every structural finding from the first review is resolved, and found five pre-merge problems, fixed in `619ccce`..`c62b3cb`:

| Commit | Content |
|--------|---------|
| `619ccce` | `drift eject ios` wrote a pbxproj naming `Runner.entitlements` but not the file, so every freshly ejected iOS project failed to build. Eject and the managed scaffold now share `scaffold.WriteIOSProject`; a test checks every `CODE_SIGN_ENTITLEMENTS` file is written |
| `c03320e` | Plugin config passed through `map[string]any`, so unquoted values changed (`1.10` became `"1.1"`, `0123` became `"83"`). `protocol.ResolveYAML` resolves anchors and merge keys and keeps values as written; defaults are applied to the YAML; decoding into the typed config is strict |
| `aed86e2` | A second plugin asking for the same SwiftPM product was dropped as a duplicate, so its `import` failed. Products are tracked per plugin. `SetDict` keeps integers and floats distinct (`<integer>` vs `<real>`) |
| `c62b3cb` | Method-call transport. A cancelable context moved a native call onto another goroutine, hiding the UI thread from native, which then waited on the main thread while the main thread waited on Go (a frozen UI). Go now starts every call on the calling thread with a call ID; native replies once through `DriftPlatformReply`, in place or later from any thread; Go waits for the reply or `ctx`. Plugins declare each method as `method` (replies by returning) or `asyncMethod` (replies through `DriftResult`); an async method called from the UI thread fails with `ErrBlocksUIThread` before it runs; a dropped `DriftResult` fails with `ErrReplyDropped`; a second reply crashes on both platforms; unknown methods fail with `ErrMethodNotFound`. Native error codes match Go sentinels under `errors.Is` |

**Verified:** `go vet`/`go test` in the root and plugin modules; on the API 36 emulator with a scratch plugin: a UI-thread call with `context.WithTimeout` returns at once, `1.10` arrives as `1.10`, an async method from the UI thread gets `ErrBlocksUIThread` and from a goroutine replies, an unknown method gets `ErrMethodNotFound`, a dropped result gets `ErrReplyDropped` after GC. **Not yet:** the new transport on iOS was compiled and linked (`drift build xtool`) but not run. Run `examples/plugins/hello` and `examples/firebase-demo` on the Simulator before merge.

**Behaviour to know:** built-in channels still run synchronously on the calling thread, so `ErrBlocksUIThread` only protects plugin methods. Built-ins that wait for the user (permission requests, camera, pickers) still freeze the app if called from the UI thread; they get the guard by moving to plugins with `asyncMethod` in Phase 5. On Android a dropped result is detected only after garbage collection.

## Review findings driving this plan

These come from a three-part branch review (design, pipeline correctness, native runtime). Items marked *(device)* are code-level findings that need confirming on hardware.

**Splash does not work as built.**
1. **Overlay never installs.** *(device)* Plugins register before a window exists, so `driftRootView()` returns nil.
   - iOS: `SceneDelegate.swift:73` touches `PlatformChannelManager.shared` before the window is created at `:87`.
   - xtool: registration happens in `AppDelegate.didFinishLaunching`, before any scene connects.
   - Android: `init` runs in `onCreate` (`MainActivity.kt:34`), but `currentActivity` is only set in `onActivityResumed` (`PlatformChannel.kt:383`).
2. **Android hangs with `android_12` configured.** *(device)* `setKeepOnScreenCondition` (`Android12SplashController.kt:45`) suppresses draws. `first_frame` is emitted from `registerFrameCommitCallback` (`SkiaHostView.kt:147-160`), which only fires after a draw, so the splash never dismisses. `install` is not gated to API 31+.
3. **`first_frame` fires too early.** It fires on the blank background frame drawn while `App.OnInit` runs (`DriftMetalView.swift:278-297`, `engine.go:1158`), not on the first real content.
4. **iOS drops a dismiss.** A dismiss arriving while the app is not active is dropped with no retry (`DriftSplashPlugin.swift:91`).
5. **No safety timeout.** There is no timeout on `Preserve`, and `runtime/splash.go` discards `Invoke` errors.
6. **Android colour clash.** Splash writes `values/plugin_colors.xml` with `WriteXML`. That is the same file `Colors.Set` writes (`apply.go` colours step), so the last writer silently wins.
7. **8-digit hex colours differ by platform.** `#RRGGBBAA` is read as RGBA on iOS and ARGB on Android.
8. **Config fields that do nothing.** `branding`, `dark.branding` and `dark.android_12` are parsed but never emitted.

**Native lifecycle is underdesigned.**
- No attach/detach callback exists.
- Android calls `registerAll` on every Activity creation (dark-mode toggle, locale change). It clears `handlers` while other threads read them, and duplicates observers.
- Method handlers are synchronous only.
- Android has no new-intent, activity-result or permission-result hooks.
- iOS handlers run on the calling Go thread, which is undocumented.
- `registerAll` runs inside a `static let shared` initializer, so re-entrancy deadlocks.

**Conflict detection is keyed on op type, not target.**
- Two different op types hitting the same file or plist key never conflict (for example `SetString` vs `SetBool` on one key, `WriteXML` vs `Colors.Set`).
- `ClassIdempotent` and `ClassExclusive` run identical code (`conflict.go`).
- The `MergeClass` argument to `newBase` is ignored.

**Native code is injected as loose sources into the app target.**
- iOS: every plugin shares one Swift module with about 40 unprefixed template types, so name clashes are likely. Two plugins shipping the same file basename also fail.
- Android: there is no manifest merge, and no ops exist for `<service>`, `<receiver>` or `<provider>`.

**Stale output is not removed.**
- Watch-mode `Refresh` does not re-scaffold, and ejected projects are never cleaned.
- Only bundle resources and the SwiftPM sidecar are pruned. Swift/Kotlin sources, manifest entries, plist keys and Gradle lines remain after a plugin is removed.

**Bridge side effects.**
- `go build -mod=mod` edits the user's `go.mod`/`go.sum` (`bridge.go:106`).
- Local (replaced) plugins leak a temp dir with a full binary per build, which in watch mode means per save (`bridge.go:72-78`).
- The cached binary is written non-atomically.
- The cache key misses a locally replaced `github.com/go-drift/drift` and `go.mod`-only changes.
- Import aliases can be Go keywords (`map`, `go`, `chan`).
- YAML anchors in plugin config pass `sync` but break `build` (`manifest.go` `ConfigYAML`).
- An empty `plugins:` list is a hard error.

**Validation disagrees with itself.**
- Invalid input sometimes panics, sometimes becomes a deferred error, and sometimes is unchecked (`AddImageSet`, `WriteXML` relpath, `AddBitmap`, registrant symbols).
- The `default=`, `hex` and `asset` tags are parsed but never applied.
- `sync` checks that a required key is present, while `build` checks `IsZero`.
- Nested `required` fields are not enforced.

**Other.**
- `SetDict` integers become `<real>` after the JSON round trip.
- Registrant order is lexicographic by symbol, not `drift.yaml` order, so claim priority is arbitrary.
- `CheckEjectedIOS` does not check the `registerAll` call site or synchronized-folder project format. Ejected Android has no checks at all.
- Wire protocol types (`Envelope`, `Response`, `MarshalOp`, `NewOp`) are exported from the plugin-author package.
- No plugin docs exist in `website-docs/`.

## Plan

Phases are ordered so each one leaves the branch green. Phases 1 to 4 are the merge bar; phase 5 can happen before or after merge.

### Phase 1: core fixes (before building more on top) (done, see Status)

1. **Native lifecycle.**
   - Split registration into *register* (once per process: channels, handlers) and *attach/detach* (root view available; per Activity on Android, per scene on iOS). Add `onAttach(rootView)` / `onDetach` to the host protocols and remove `driftRootView()`.
   - Android: stop re-running `registerAll` per Activity. Keep handlers process-scoped and thread-safe.
   - iOS: stop registering inside the `static let shared` initializer.
   - Add async method replies (a completion/result object, like Flutter's `Result`). Firebase `getToken` needs this.
   - Document handler threading.
   - Reference: Flutter `FlutterPlugin` + `ActivityAware`.
2. **Target-keyed conflicts.**
   - Give ops an identity over what they touch (`file:<root>/<relpath>`, `plist:<key>`, `manifest:<element>/<name>`, `gradle:<coord>`), so cross-type collisions are detected.
   - Collapse merge classes to *exclusive-by-content* and *additive-set*. Delete the dead `MergeClass` parameter.
3. **Validation.**
   - Give each op `Validate() error`. Run it at record time into `ctx` errors (no panics) and again after decode on the CLI side.
   - Implement or delete the `default=`, `hex` and `asset` tags. Use one `required` check, applied recursively.
4. **Bridge hygiene.**
   - Build with `-mod=readonly` and print a `go mod tidy` hint on missing sums.
   - Remove temp dirs. Build into a temp name and rename into the cache.
   - Include `go.mod` and any path `replace` of `github.com/go-drift/drift` in the cache decision.
   - Make aliases keyword-safe. Resolve YAML aliases before marshalling config. Accept an empty `plugins:` list.
   - Order registrants by `drift.yaml` order.
5. **Stale output.**
   - Managed builds: `Refresh` re-runs a full `Prepare` when the op set hash changes.
   - Ejected builds: record applied ops in a manifest file in the platform dir and diff it to remove what a dropped plugin added. At minimum, error on removal.
6. **Native packaging decision** (spike, then decide).
   - Try generating one SwiftPM target per plugin inside the `Drift/Plugins` sidecar (plugin Swift in its own module, depending on a `DriftPluginAPI` target holding the host protocols).
   - Try one Gradle library module per plugin (gives manifest merge for services, receivers and providers, and scoped dependencies).
   - The Firebase spike (phase 3, step 1) is the test: can plugin Swift `import FirebaseCore` in the current loose-source model? If not, this redesign is required.
7. **Keep framework built-ins out of codegen.** `NotificationHandler` / `DeepLinkHandler` calls live in hand-written templates; codegen emits only plugin lists.
8. **Move wire types** (`Envelope`, `Response`, op marshalling) to `pkg/plugin/protocol`.

### Phase 2: splash works on devices (done, see Status)

Already done in phase 1: the overlay installs in `attach`/`onAttach` (findings 1); the colour file clash (finding 6) is fixed by writing `drift_splash_colors.xml`, and `WriteXML` now rejects Drift's own values files; hex is validated as `#RRGGBB`/`#RRGGBBAA` by the `hex` tag; splash no longer fades out twice. Still to do, then verify on devices:

- ~~Install the overlay in `onAttach`.~~ Done; confirm on device, including that iOS `detach` (from `DriftViewController.deinit`) and Android `onDetach` behave on scene disconnect / Activity recreation.
- Gate the Android 12 controller to API 31+. Make `first_frame` independent of the keep-on-screen draw suppression; for example, emit from the engine when the first non-empty layer tree is presented, then release keep-on-screen.
- Emit `first_frame` only after the root is mounted and composited (not during `OnInit`).
- Retry a pending dismiss on `didBecomeActive`.
- Add a configurable max-duration timeout. Surface `Invoke` errors (log or return).
- ~~Fix the colour file clash.~~ Done.
- Hex is defined and validated as `#RRGGBB` / `#RRGGBBAA` (RGBA); still convert it for Android (ARGB) and check iOS parsing matches.
- iOS: platform views added after attach sit above the splash overlay (both are subviews of the Drift view); keep the overlay on top.
- Emit branding and dark variants, or delete those config fields.
- Update `examples/splash-demo/drift.yaml` (its comment about xtool is stale).

### Phase 3: Firebase plugin, push moves out of core

1. ~~**Spike on a Mac first.**~~ Done (branch `spike/firebase-ios`): firebase-ios-sdk resolves and FirebaseCore/FirebaseMessaging import and link from a per-plugin target. Phase 1 item 6 shipped per-plugin SwiftPM targets.
2. **Create `plugins/firebase`.**
   - Build half: config for `GoogleService-Info.plist` and `google-services.json` paths.
   - Runtime half: Go API for init, token, and message/open events.
   - Core plus Messaging first.
3. **Move push out of core.**
   - Remove `firebase-bom`, `firebase-messaging-ktx`, the notification receiver and FCM code from the Android template.
   - Remove the `remote-notification` background mode and remote-notification handling from the iOS templates.
   - Remove the push parts of `pkg/platform/notifications.go` and `NotificationHandler`. Decide whether local notifications stay core until the notifications plugin exists (they need `POST_NOTIFICATIONS`).
4. **New ops Firebase needs** (verify each is really required):
   - iOS entitlements (`aps-environment`), with an entitlements file wired into both build paths.
   - iOS background modes.
   - Android `<service>` (`FirebaseMessagingService` subclass), unless per-plugin Gradle modules land (manifest merge).
5. **Delete ops from `261864d` that splash and Firebase do not use.** Review list:
   - `DriftPlugin` app hooks on iOS (the `ios.app_delegate_registrant` op is gone): keep only those Firebase uses.
   - `DriftPluginCoordinator` background-fetch merge.
   - `android.assets.add`.
   - `android.gradle.apply_plugin`: needed for google-services.
   - `android.app_module.add_file`: needed for `google-services.json`.

### Phase 4: docs and merge (code and docs done, see Status)

- ~~Add `website-docs/guides/plugins.md` covering using plugins, authoring plugins and the op reference.~~ Done.
- ~~Update `docs/plugins.md` to match the final design.~~ Done (internals only).
- ~~Walk through the device verification matrix below, then merge.~~ Matrix accepted (see Status); merge.

### Phase 5: core feature migration

**Rule:** a feature becomes a plugin if it adds permissions, usage strings, entitlements, background modes or heavy/third-party dependencies. Everything else stays core. This matches Flutter, where every "move" row below is a plugin.

**Why it matters:** today every Drift app declares all of the permissions below. Google Play rejects `ACCESS_BACKGROUND_LOCATION` without a declaration form, and App Store review rejects unused background modes.

| Feature | Move? | What it adds to every app today | Go API (`pkg/platform`) | Native (templates) |
|---------|-------|---------------------------------|-------------------------|--------------------|
| Push notifications | **Done in Phase 3** (Firebase plugin) | Firebase BOM + messaging, `POST_NOTIFICATIONS`, `DriftNotificationReceiver`, `remote-notification` background mode | `notifications.go`, `notification_permission.go` | `NotificationHandler` (iOS in `PlatformChannel.swift`, Android `NotificationHandler.kt`, `NotificationBridge.kt`, `DriftNotificationReceiver.kt`) |
| Local notifications | Yes (`notifications` plugin) | `POST_NOTIFICATIONS`, notification auth | `notifications.go` | same as above |
| Location | Yes, first after merge (Play policy) | `play-services-location`, fine/coarse/**background** location, `NSLocationAlwaysAndWhenInUse...` | `location.go` | `LocationHandler.swift/.kt` |
| Camera, photos, microphone | Yes (`camera`, `image_picker`) | `CAMERA`, `RECORD_AUDIO`, `READ_MEDIA_*`, legacy storage permissions, `FileProvider`, 4 iOS usage strings | `camera.go`, `photos.go`, `microphone.go` | `CameraHandler.swift/.kt` |
| Contacts | Yes | `READ_CONTACTS`, `NSContactsUsageDescription` | `contacts.go` | in `PermissionHandler` |
| Calendar | Yes | `READ_CALENDAR`, `NSCalendarsUsageDescription` | `calendar.go` | in `PermissionHandler` |
| Background tasks | Yes | `work-runtime`, `RECEIVE_BOOT_COMPLETED`, `WAKE_LOCK`, `FOREGROUND_SERVICE`, `fetch` + `processing` modes, `BGTaskSchedulerPermittedIdentifiers` | `background.go` | `BackgroundHandler.swift/.kt` |
| Secure storage + biometrics | Yes | `androidx.security:security-crypto` (alpha), `androidx.biometric`, `USE_BIOMETRIC`/`USE_FINGERPRINT`, `NSFaceIDUsageDescription` | `secure_storage.go` | `SecureStorageHandler.swift/.kt` |
| Video / audio players | Yes | `media3` exoplayer + hls + dash + ui (several MB) | `video_player_*.go`, `audio_player.go`, `playback_state.go` | `NativeVideoPlayer`, `NativeAudioPlayer`, `DriftMediaSession` |
| WebView | Optional, low priority | none (system WebView) | `webview_*.go` | platform view |
| Rendering, text input, accessibility, platform views, lifecycle, safe area, system UI, clipboard, haptics, share, URL launcher, deep links, preferences, storage, date/time pickers, switch, activity indicator | **Stay core** | no permissions or new deps | | |

**Migration notes:**
- `PermissionHandler` (iOS 456 lines, Android 242) knows every permission type. Split it so core keeps the request mechanism and each plugin registers its own permission kinds (`permissions.go`, `permission_interface.go`).
- Each migrated plugin moves its manifest permissions, usage strings, Gradle deps and background modes from the templates into its `Build`. Templates end up declaring only `INTERNET` and `ACCESS_NETWORK_STATE`.
- `android.manifest.add_service` exists; `<receiver>` and `<provider>` ops do not yet (local notifications need a receiver, camera needs the `FileProvider`). Background modes go through `ios.plist.append_array_item`.
- Built-ins that wait for the user (`Permission.Request`, `Camera.CapturePhoto`/`PickFromGallery`, `Storage.PickFile`/`PickDirectory`/`SaveFile`, the date and time pickers) use a request ID plus a result event channel (`pkg/platform/camera.go`, `permissions.go`) and block the UI thread if called from it. As plugins they become one `asyncMethod` each: delete the request-ID and event plumbing, and the UI-thread guard comes for free.
- Android plugins register from `MainActivity.onCreate` today. A migrated feature that runs without an Activity (background tasks, notification receivers, boot) needs process-level registration first (see Known follow-ups).
- Showcase (`showcase/`) uses many of these APIs; update it as each feature moves.

## Verification matrix (merge gate)

Cells give status. "Accepted" cells were not run on hardware; they rest on the emulator and Simulator runs (Phase 4 decisions).

| Check | iOS xcodeproj (Mac + device) | Android device | xtool (Linux, best effort) |
|-------|------------------------------|----------------|----------------------------|
| `go vet ./...`, `go test ./...` (root and `plugins/*`) | done (Linux, CI) | done (Linux, CI) | done |
| Splash shows, holds on `Preserve`, fades on `Remove`, auto-dismisses on real first frame | done on iPhone (Phase 2) | accepted (emulator done) | build only |
| Splash survives background/foreground during launch, dark-mode toggle | done on iPhone (Phase 2) | accepted (emulator done) | n/a |
| Firebase init, FCM/APNs token delivered to Go, foreground + background message, tap opens app | accepted (Simulator done with `simctl push`; needs the APNs `.p8`) | accepted (emulator done) | build done; device run optional (needs a paid team) |
| Removing a plugin from `drift.yaml` leaves a compiling project (managed, watch mode, ejected) | accepted (managed and ejected done on Android and xtool) | done on emulator: managed, watch, ejected | done: managed |
| Two plugins touching the same plist key / resource file report a conflict | unit test | unit test | |

## Known follow-ups

Not merge blocking. File references are from `c62b3cb`.

### Before the first release that ships plugins

**Plugin lifecycle (Android)**
- **Process-level registration.** Plugins register from `MainActivity.onCreate` (`MainActivity.kt`), so FCM data messages and token refreshes that start the process without an Activity are dropped (`DriftFirebaseMessagingService.kt`); the token is fetched again at the next start. Every future service, receiver or boot plugin hits the same wall. Register from an `Application` subclass or an `androidx.startup` initializer.
- **Fresh-launch detection is duplicated.** Plugins guess "first attach in this process" themselves (`DriftFirebasePlugin.kt`, `DriftSplashPlugin.kt`), while `MainActivity` uses `savedInstanceState` / `FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY`. When the process lives but `MainActivity` was finished (back from a task rooted by a deep link, "Don't keep activities", `finish()`), a later FCM tap cold-creates the Activity and `Opens()` never fires. `MainActivity` should compute it once and expose it on `DriftActivityBinding`.

**Splash (iOS)**
- **Timeout counts from process launch.** `DriftSplashPlugin.swift` arms the timer in `register` (`didFinishLaunching`). Firebase adds the `remote-notification` mode, so a silent push can launch the process in the background; if the user opens the app after `max_duration_ms`, Preserve is ignored and the splash hides at the first content frame. Start the timer on first attach.
- **A missed `first_frame` leaves the splash up forever** (both platforms). Dismissal requires `contentShown` and the timeout never overrides it; `first_frame` is one-shot and needs the content frame to pass every guard (`nextDrawable`, `renderSync`; `slotIndex >= 0` on Android). Re-arm on a later frame, or add a hard ceiling that ignores `contentShown`.

**iOS detach and leaks**
- **Detach never runs on scene disconnect.** `CADisplayLink(target: self)` (`DriftViewController.swift`) holds the view controller strongly until `viewDidDisappear`, `SceneDelegate` has no `sceneDidDisconnect`, and detach depends on `deinit`. Plugins stay attached to a dead controller and the controller, Metal view and display link leak, even when no plugin retains the binding (`DriftViewBinding` also holds the controller strongly). Detach explicitly in `sceneDidDisconnect` and give the display link a weak proxy target.

**Ejected projects (`.drift/plugins.lock.json`, `cmd/drift/internal/plugin/lock.go`)**
- **A plugin upgrade looks like a removal.** Lock keys include target content (for example the Gradle coordinate with its version), so a version bump fails the next ejected build once with "plugins removed ... left these edits" and leaves the old line next to the new one. Key the lock on the target without content.
- **The lock can delete a file the same build just wrote.** If a CLI upgrade renames an op type or changes its targets, the old key disappears while the hash still matches, so `SyncEjectedLock` deletes the path after `Apply` wrote it (reproduced with `app/google-services.json`; the next build recovers). Never delete a path a current op owns.
- **Plugins can overwrite user files.** `Apply` writes owned files unconditionally and `WriteXML` only reserves Drift's `plugin_*` values files, so a plugin can replace `drawable/launch_background.xml`, `values/styles.xml`, a user's image set or a Kotlin file in `com.drift.runner`; the lock then records Drift's hash and a later removal deletes it. In ejected mode, refuse to overwrite an existing file that is not in the previous lock and differs; reserve template resource paths and the `com.drift.runner` package.
- **The lock file is written non-atomically** (`lock.go`), so a crash mid-write fails every later build; paths read from it are not confined to the project.
- **Template-declared entries are blamed on plugins.** Firebase adds `POST_NOTIFICATIONS`, which the template already declares, so removing it from an ejected project asks the user to delete a permission core still needs.

**Distribution and versioning**
- **Plugin modules require `github.com/go-drift/drift v0.0.0` with `replace => ../..`** (`plugins/*/go.mod`), and `scripts/release.sh` tags only the root. Plugins declare no minimum drift version and nothing checks skew between a plugin's native code and the CLI's `DriftPluginAPI` templates (Unresolved question 4). Tag `plugins/<name>/vX`, require a real drift version, and have the bridge report its `pkg/plugin` version so the CLI fails on skew.

**Author experience**
- **No local-development workflow in the guide.** It does not explain the app `go.mod` `replace` needed to try an unpublished plugin, and `examples/plugins/hello` uses a repo-relative replace plus `v0.0.0`, so copying it outside the repo breaks.
- **No example app consumes `hello`**, so an author cannot run it without writing an app first.
- **Missing hooks, undocumented:** Android activity-result and permission-result listeners; iOS open-URL and user-activity hooks (deleted in `329c431`), so a deep-link or OAuth plugin can only be written for Android.

### Later

**Correctness, low severity**
- **`theme` vs `android:theme` do not conflict.** `SetActivityAttr.Targets()` uses the raw attribute name, while `setNSAttr` adds the `android:` prefix; normalise in `Targets()`.
- **Bitmap target key names the wrong directory.** It claims `drawable/<stem>` but writes `drawable-nodpi/`, missing a real clash and reporting a false one. `WriteXML` into `values*/` claims only the file, so duplicate resource names inside it surface only in Gradle.
- **Android string values are not escaped** (`mutate/xml.go`), so an apostrophe fails aapt.
- **`drift plugin sync` does not type-check scalars** (`n: notanint` for an int field passes sync, fails build); a null optional struct is reported as "must be a mapping".
- **Intent-filter dedup ignores the filter's own attributes** (`canonicalIntent`), so `autoVerify` is lost if an equivalent filter exists.
- **The same package listed twice in drift.yaml** gives a duplicate-import Go error instead of a clear message.
- **`walkEmbedFS` with root `"."`** flattens subdirectories.
- **Envelope `AppID` always comes from drift.yaml**, even for ejected projects whose id may differ (Firebase checks against it).
- **Old Drift-owned support files are never pruned from ejected projects** (`EnsureRunnerSupport`), so projects ejected before this branch keep `MethodHandler.kt`, `DriftOverlayHost.kt` and similar.
- **`first_frame` fires per view, not per process** (the guard lives on each `SkiaHostView` / `DriftRenderer`), so Activity recreation or scene reconnect emits it again, contradicting `frame_events.go`.

**Runtime semantics**
- **Sticky event replay** can arrive after a newer live event (`pkg/platform/channel.go` Listen).
- **The first subscriber to a queued channel drains it**, so an analytics listener on `Opens()` that subscribes first steals the launch tap from the router. `Subscription.Cancel` sets its flag before removing the subscription, so a dispatch in that window loses an event instead of queueing it; an app that never subscribes to `Messages()` gets a report for every message after 32.
- **iOS silent push completes immediately** (`DriftFirebasePlugin.swift` calls `completion(.newData)` at once), so Go cannot finish background work before suspension; no scene connects, so no app Go code runs.
- **Preserve after the splash timeout** returns success and has no effect, with no log.
- **Deep links replay on Android recreation.** `MainActivity.onCreate` passes the launch intent to `DeepLinkHandler` again after dark-mode recreation or a Recents relaunch (notification taps no longer do).
- **Android auto-grouped notifications**: tapping the system's group summary opens the app without FCM extras, so no `Opens()` event; tapping an individual notification works.

**xtool**
- **xtool may create two windows (unverified).** `SceneDelegate.swift` is copied into xtool (`scaffold/xtool.go`) and named in `xtool/Info.plist.tmpl` next to a SwiftUI `WindowGroup`; the `xtool/AppDelegate.swift` comment claims there is no SceneDelegate.
- **xtool launch storyboard.** It is uncompiled and sits in the SwiftPM resource bundle, while `UILaunchStoryboardName` looks in the main bundle.

**Platform and tooling**
- **System dark mode.** Neither platform reports the system appearance to Go, so apps cannot follow it.
- **Splash dark mode.** Needs appearance-aware image set and colour set ops on iOS (not on xtool before xtool#219), and a night colour on Android.
- **Ejected-project file hygiene.** The whole `Info.plist` is re-marshalled (comments stripped), the manifest re-indented, Gradle brace counting ignores strings and comments, and dependency dedup matches exact lines.
- **Bridge cache entries** under the cache root are never garbage-collected. The bridge log lands in `platform/<p>/logs/plugin-bridge.log` on ejected projects and is missing from eject's `.gitignore` suggestions.
- **The `drift/` channel prefix** is documented as reserved but not enforced. Android's `internal constructor` on host types protects nothing (plugin sources compile into the app module), and `@_spi(DriftHost)` is cosmetic on iOS.
- **Emulator Play services**: the `pixel_8` google_apis image warns its Play services (25.26) is older than firebase-messaging 25.1.3 asks for (26.12); FCM works anyway.
- **Engine test races.** `go test -race ./pkg/engine` fails in older `init_test.go` tests (also on `master`): leaked OnInit goroutines dispatch to the global `app` after `swapApp` restores it.
- **Stale comments and files:** `init/gitignore.tmpl` still mentions `Empty.swift` and `tools/drift-plugins/bridge`; `DriftPlugins.swift` says plugins can use `PlatformChannelManager`; `DriftPluginHost.swift` says the module depends only on Foundation and UIKit; `protocol/ops.go` mentions the deleted pre-activity hook; `ejected.go` has a stale hint.

## Unresolved questions

1. ~~Per-plugin SwiftPM targets / Gradle modules, or loose sources?~~ iOS: per-plugin SwiftPM targets (shipped). Android: loose sources for v1.
2. ~~Do local notifications stay core until a `notifications` plugin exists, or move with push?~~ Stay core until Phase 5 (see Phase 3 decisions).
3. ~~Keep the xcodeproj/xtool dual path, or declare some plugins xcodeproj-only?~~ Dual path; no plugin is xcodeproj-only (Phase 4 decisions).
4. Plugin compatibility metadata: should the bridge report its `pkg/plugin` version so the CLI can hard-fail on skew (today `APIVersion` stays `1`)?
