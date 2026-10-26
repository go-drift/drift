# Plugins v1: plan

Handoff document for finishing the `feat/plugins` branch. Read the website guide [plugins.md](../website-docs/guides/plugins.md) for using and authoring plugins, and [plugins.md](plugins.md) for internals. **Phases 1 to 4 are done, and the fixes from the second review (2026-10-02) have landed; the new method-call transport has run on the iOS Simulator (2026-10-21, see Second review), so the branch is ready to merge.** Device runs not yet done are accepted on the strength of the emulator and Simulator runs (decision 2026-10-02, see Phase 4 decisions). Phase 5 is done on `feat/core-plugins` and verified on the iOS Simulator (see Phase 5 status). Next is [Phase 6](#phase-6-release-readiness) (release readiness), done before merging on `feat/plugins-release` (Phase 6 done, see [6.1 status](#61-status), [6.2 status](#62-status), [6.3 status](#63-status), [6.4 status](#64-status)); then the [Release sequence](#release-sequence) rebases, merges and tags. Non-blocking work is in [Known follow-ups](#known-follow-ups). Line numbers in the review findings are from commit `261864d` and may drift; many of those findings are now fixed (see Status).

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

**Phase 1 is complete** (`ae5ab9e`..`36d696a`, 15 commits, not yet pushed):

| Commit | Content |
|--------|---------|
| `ae5ab9e` | Wire protocol moved to `pkg/plugin/protocol` |
| `7b3edc0` | Every op validates at record time (into `ctx.Err()`, no panics) and at decode; path traversal closed |
| `fff39ad` | One recursive config check (`required` = key present, `default=`, `hex`, `asset`) shared by sync and build |
| `92ea447` | Conflicts keyed on targets (`Op.Targets()`), merge classes deleted; plugins can share a SwiftPM package with different products |
| `d00971e` | Bridge: `-mod=readonly` with a `go mod tidy` hint, atomic cache writes, no temp-dir leak, cache key covers go.mod/go.work/GOFLAGS/toolchain and local replaces |
| `70f36c5` | Keyword-safe bridge aliases, YAML anchors resolved, bare `plugins:`, `list --resolve` fixed |
| `b2fdef6` | Registrants in drift.yaml order |
| `0cbf61f` | Fix: xtool referenced the sidecar product by the wrong name (every xtool build failed) |
| `82c221c` | Plugin objects with a lifecycle: iOS `DriftPlugin` (register, didFinishLaunching, attach/detach, URL/activity/remote-notification hooks), Android `DriftPlugin` (onRegister once per process, onPreActivityCreate, onAttach/onDetach with intent/result/permission listeners). `DriftPlugins` owns them; codegen only lists classes; built-in deep-link/notification handling lives in templates. Ops `ios.plugin`/`android.plugin` replace the four registrant ops |
| `20eade9` | Async method results (`DriftResult`); all plugin callbacks on the main thread; fail fast when Go calls a pending handler from the main thread; Android JNI error slot thread-local |
| `1e2f84f` | Watch-mode `Refresh` regenerates the managed project when the plugin op hash changes |
| `241c56b` | Ejected projects: `.drift/plugins.lock.json`; removed plugins' owned files deleted (unless user-modified); in-file edits listed in a build error shown once; plugin values XML regenerated |
| `7e7c790` | One SwiftPM module per iOS plugin (`DriftPlugin_<name>`) plus `DriftPluginAPI` in the Drift-owned `Drift/Plugins` package; plugin classes public; host-only initializers behind `@_spi(DriftHost)` |
| `e4a93c8` | Fix: Simulator builds (no `addPresentedHandler` in the Simulator SDK; `first_frame` falls back to command-buffer completion there) |
| `36d696a` | Fix: splash launch storyboard that ibtool compiles |

**Verified:** `go vet`/`go test` in all modules; `drift build android` of splash-demo (with and without `android_12`); `drift build xtool` of splash-demo and of a zero-plugin app (Linux, iOS 26.5 SDK); `drift build ios` of splash-demo on Xcode 26.5 (simulator). Firebase spike (branch `spike/firebase-ios`, Mac): firebase-ios-sdk resolves, and both loose sources and per-plugin targets compile and link, with and without explicit modules.

**Verified since:** `xcodebuild test` in `cmd/drift/internal/plugin/coordinator_test` passes (scheme `DriftPluginCoordinatorTests-Package`).

**Deviations from the original phase 1 plan:** removed plugins' in-file edits in ejected projects fail the build once instead of being probed until removed (a user may want to keep an entry); the watch-mode op hash lives on the in-memory `Workspace`. Android stays loose-source (no per-plugin Gradle modules); Firebase's `<service>` needs a manifest op in phase 3.

**New plugin-author rules:** `Plugin.Name()` is a lowercase identifier (it names the iOS module); iOS plugin sources are Swift only; the iOS plugin class is `public` with `public init()`; channels declare each method as `method` (replies by returning) or `asyncMethod` (replies through `DriftResult`; Go must call it off the UI thread).

**Phase 2 is done** (`d76daa8`..`9687f69`), verified on an iPhone (xcodeproj) and the iOS Simulator, and on an API 36 emulator. Not yet on a real Android device.

| Commit | Content |
|--------|---------|
| `d76daa8` | Engine: `NeedsFrame` consumes the pending platform schedule. iOS skipped `StepFrame` while OnInit ran, so a frame request then left the flag set and the OnInit completion never woke the display link (black screen forever with a slow OnInit) |
| `cd62969` | Engine: `first_frame` only for the first frame that composited the root (`HasRenderedContent`); Android emits after that `renderFrameSync` rather than waiting for a View draw |
| `03de901` | `DriftViewBinding.overlayView` / `DriftActivityBinding.overlayView`: plugin overlays stay above platform views |
| `50518b7` | `protocol.Color` / `driftplugin.ParseColor`: Drift hex is alpha last; `android.color.set` validates it and writes Android's alpha-first form |
| `bedab2f` | Splash: `branding*`, `dark`, `android_12.branding` removed (never emitted; iOS has no appearance-aware asset ops) |
| `6108212` | Splash: `image_width`; storyboard and iOS overlay generated from the same values and pinned with constraints |
| `d182e57`, `a58e3a3` | Splash iOS: dismissal is state (`reconcile()` on every input, no `applicationState` guard); `max_duration_ms` (default 10s) overrides Preserve but never a slow OnInit; `Preserve`/`Remove` return errors |
| `0b2a469` | Coordinator harness instructions |
| `d64d123` | Android toolchain: AGP 9.4.1 (built-in Kotlin), Gradle 9.8.0, compile/target SDK 36, **minSdk 31** (`templates.AndroidMinSDK`, also the NDK clang target) |
| `86571ba` | `AddBitmap` writes to `drawable-nodpi` |
| `9687f69` | Splash Android: the platform splash screen (`Drift.Splash` style over `LaunchTheme`), held with a pre-draw listener; `android.icon` / `android.icon_background_color`; overlay, `android_12` and `core-splashscreen` removed |

**Phase 2 decisions:** minSdk 31 (Vulkan-only renderer; an API 29 emulator crashes in its Vulkan driver; 31 brings the platform splash API). Android's splash is the platform icon splash, so it does not match iOS's `image_width` layout. A recreated Activity (dark mode, locale) ends the splash, since it gets no platform splash.

**Phase 3 is done** (`49e86ec`..`bf30b1a`); remaining device runs accepted in Phase 4 (see Verification matrix).

| Commit | Content |
|--------|---------|
| `49e86ec` | `platform.NewQueuedEventChannel`: events sent while nobody listens wait for the next subscriber; `ResetForTest` clears sticky slots and queues |
| `68fc9af` | `ctx.AppID()` (envelope `app_id`, required for build) |
| `1fe8e75` | Entitlements: `info_plist.*` became `ios.plist.*` with a file (`info`/`entitlements`); `Runner.entitlements` at the project root on both iOS paths (`CODE_SIGN_ENTITLEMENTS`, xtool `entitlementsPath`); appending to a non-array plist value errors |
| `df92fff` | `android.manifest.add_service`; manifest mutator takes `ManifestOps` |
| `bc57176` | Deleted: `android.assets.add`; iOS `open`/`continueUserActivity` and `DeepLinkHandler.route`; Android `onPreActivityCreate` and activity/permission-result listeners; `DriftPluginCoordinator` and its harness (remote notifications are claimed, not merged) |
| `d7f68d0` | iOS `willPresentNotification` / `didReceiveNotificationResponse`, offered by Drift's notification-center delegate before local handling |
| `86be4db` | Push left core (Go API, both iOS paths, Android handler, Gradle Firebase deps, `remote-notification` mode) |
| `e2bde43` | `plugins/firebase` (Core + Messaging; Go API `runtime/messaging`: `Token()` state, `Messages()`/`Opens()` queued streams); `NewTestCtxFor` |
| `bf30b1a` | `examples/firebase-demo` and `tools/fcmsend` (FCM HTTP v1 sender, stdlib only) |

**Verified (Linux):** `go vet`/`go test` in every module; `drift build android` and `drift build xtool` of splash-demo; `drift build android` of firebase-demo with placeholder config (google-services 4.5.0 runs on AGP 9; manifest service, permission and dependency merge); **FCM on the API 36 emulator, real Firebase project** (`fir-demo-71e74`, sent with `fcmsend`): token delivered to Go; foreground notification and data messages reach `Messages()` with nothing shown; in the background the notification is shown and the data message reaches `Messages()` (`foreground=false`); tapping opens the app and reaches `Opens()`, both with the process alive and after it was killed (the tap that launched it is queued until the app listens); dark-mode Activity recreation replays no tap. **Mac (reported 2026-10-02):** `drift build ios` of splash-demo and firebase-demo; splash on the Simulator; firebase-demo on the Simulator with `xcrun simctl push` payloads carrying `gcm.message_id`: foreground message reaches `Messages()` with no banner, background banner tap and cold-start tap reach `Opens()`. **Not yet:** a real Android device; real iOS delivery through FCM (needs the APNs `.p8` uploaded; config only, no code).

**Phase 3 decisions:** local notifications and the notification permission stay core until a notifications plugin exists (Phase 5); core keeps the one `UNUserNotificationCenterDelegate` and offers notifications to plugins first. ~~Firebase on xtool is an error~~ (reversed in Phase 4: plugins must build on xtool; see Phase 4 decisions). Foreground FCM notifications are not shown on either platform; the app gets them in `Messages()`. Firebase Go API has no blocking calls. Android pins `firebase-messaging` without the BOM (one artifact).

**Phase 4 is done** (`9dd00be`..`d0d41f4`):

| Commit | Content |
|--------|---------|
| `9dd00be` | Core local-notification taps queued until the app listens (`drift/notifications/opened` is a queued channel); Android does not replay the launch tap on Activity recreation or a Recents relaunch |
| `82baa87` | Android edge-to-edge under targetSdk 36 (a regression from `d64d123`): always edge to edge (`enableEdgeToEdge`, appcompat 1.7.1); system UI style is state, reapplied to a recreated Activity; `StatusBarStyleDefault` follows the system theme. `SystemUIStyle` loses the no-op `TitleBarHidden`, `BackgroundColor`, `Transparent` |
| `f701e52` | Android API checks below minSdk 31 removed, with the unreachable `ErrPlatformNotSupported`; `isMock` replaces `isFromMockProvider`; docs say Android 12 (API 31) |
| `1094d47` | `BuildCtx.Xtool` deleted (unused); stale API comments fixed; `Binding.Build` documented as the test entry point |
| `2db7e01` | Website guide `website-docs/guides/plugins.md` (using, authoring, op reference) with `TestOpReferenceListsEveryOp`; eject guide points to plugins and covers ejected plugin output |
| `ff38eee` | `docs/plugins.md` is internals only; `examples/plugins/demo` deleted |
| `f6bf22b` | CI vets and tests `plugins/*` (separate modules) |
| `2aa1627` | Watch mode ignores the generated bridge (adding or removing a plugin rebuilt twice) |
| `9e08f10` | Firebase builds on xtool (the iOS integration); push needs a paid team there, as with Xcode |
| `d0d41f4` | `examples/plugins/hello`: the smallest complete plugin, replacing the deleted demo; the guide quotes it, `TestGuideQuotesThisPlugin` keeps the quotes verbatim, CI runs it. Verified on the emulator (greeting reaches Go) and with `drift build xtool` |

**Verified (Linux, Phase 4):** `go vet`/`go test` in the root and plugin modules; Docusaurus build (no broken links from the new pages); edge-to-edge on the API 36 emulator (showcase: icons follow the app theme, and an explicit style survives dark-mode recreation); `drift build xtool` of splash-demo (shared Swift compiles); plugin removal leaves a compiling project: managed Android, watch mode on the emulator (one rebuild), ejected Android (owned files deleted, the theme edit listed once, compiles after undoing it), managed xtool.

**Phase 4 decisions:** follow-ups that land before merge are the ones this branch caused (edge-to-edge from targetSdk 36, dead API checks from minSdk 31) plus the one-line cold-start tap fix; the rest are post-merge (see Known follow-ups). `go test -race ./pkg/engine` fails the same way on `master`, so it is a separate fix. **Plugins never reject xtool**: it is how iOS apps are developed on Linux, so blocking it blocks development of any app using the plugin; what cannot work there is documented instead (release builds use Xcode). Firebase therefore builds on xtool (verified: firebase-ios-sdk compiles and links, the bundle carries `GoogleService-Info.plist` and the signature `aps-environment`); push needs a paid team, as with Xcode. This settles unresolved question 3. **Device runs:** the remaining real-device checks (Android phone, iPhone push through FCM with the APNs key, a final iPhone pass) are accepted without running them: the same code passed on the API 36 emulator and the iOS Simulator, and nothing in it differs on hardware. Upload the APNs `.p8` before relying on iOS push.

**Deviations from the Phase 3 plan:** no background-modes op (`append_array_item` does it); build half and native/runtime halves landed in one commit (the build half embeds the native sources); `core.Watchable` does not exist, so `Token()` returns a read-only `*core.Derived[string]`.

### Second review (2026-10-02)

A second three-part review (design, pipeline correctness, native runtime) checked the code rather than this Status table. It confirmed every structural finding from the first review is resolved, and found five pre-merge problems, fixed in `6d2eefd`..`03e9be8`:

| Commit | Content |
|--------|---------|
| `6d2eefd` | `drift eject ios` wrote a pbxproj naming `Runner.entitlements` but not the file, so every freshly ejected iOS project failed to build. Eject and the managed scaffold now share `scaffold.WriteIOSProject`; a test checks every `CODE_SIGN_ENTITLEMENTS` file is written |
| `5f44252` | Plugin config passed through `map[string]any`, so unquoted values changed (`1.10` became `"1.1"`, `0123` became `"83"`). `protocol.ResolveYAML` resolves anchors and merge keys and keeps values as written; defaults are applied to the YAML; decoding into the typed config is strict |
| `010d459` | A second plugin asking for the same SwiftPM product was dropped as a duplicate, so its `import` failed. Products are tracked per plugin. `SetDict` keeps integers and floats distinct (`<integer>` vs `<real>`) |
| `03e9be8` | Method-call transport. A cancelable context moved a native call onto another goroutine, hiding the UI thread from native, which then waited on the main thread while the main thread waited on Go (a frozen UI). Go now starts every call on the calling thread with a call ID; native replies once through `DriftPlatformReply`, in place or later from any thread; Go waits for the reply or `ctx`. Plugins declare each method as `method` (replies by returning) or `asyncMethod` (replies through `DriftResult`); an async method called from the UI thread fails with `ErrBlocksUIThread` before it runs; a dropped `DriftResult` fails with `ErrReplyDropped`; a second reply crashes on both platforms; unknown methods fail with `ErrMethodNotFound`. Native error codes match Go sentinels under `errors.Is` |

**Verified:** `go vet`/`go test` in the root and plugin modules; on the API 36 emulator with a scratch plugin: a UI-thread call with `context.WithTimeout` returns at once, `1.10` arrives as `1.10`, an async method from the UI thread gets `ErrBlocksUIThread` and from a goroutine replies, an unknown method gets `ErrMethodNotFound`, a dropped result gets `ErrReplyDropped` after GC. **iOS Simulator** (2026-10-21, Xcode 26.5, iPhone 17 / iOS 26.5, `drift run ios`):
- `hello` from `feat/plugins` (`12583ac`), in a scratch app outside the repo (none consumes it, see 6.4): the configured greeting returns from the UI thread at once with a `context.WithTimeout` and with `context.Background()` (no freeze), and from a goroutine; an unknown method on its channel matches `ErrMethodNotFound`.
- `firebase-demo` from `feat/plugins` (`12583ac`) and again from `feat/core-plugins` (`097354d`, where the notification delegate is offer-only): a real FCM token reaches Go; permission is granted; `simctl push` payloads carrying `gcm.message_id` reach `Messages()` in the foreground with no banner, and a background banner tap and a cold-start tap reach `Opens()`.
- Not testable on the Simulator: real delivery through FCM (Firebase answers "Invalid APNs credential" until the APNs `.p8` is uploaded), and data-only messages (the Simulator turns a `content-available` push into a background fetch). The emulator covers both over real FCM (Phase 3).

**Behaviour to know:** built-in channels still run synchronously on the calling thread, so `ErrBlocksUIThread` only protects plugin methods. Built-ins that wait for the user (permission requests, camera, pickers) still freeze the app if called from the UI thread; they get the guard by moving to plugins with `asyncMethod` in Phase 5. (Done in Phase 5: every waiting call, core or plugin, is an `asyncMethod`.) On Android a dropped result is detected only after garbage collection.

### Phase 5 status

Branch `feat/core-plugins`, from `feat/plugins` (`12583ac`; rebase onto `master` once that merges). Decisions (2026-10-02): background tasks are deleted (a stub that never ran Go; a real plugin needs a headless Go start and follows Phase 5); Contacts, Calendar, Microphone and Photos become one `permissions` plugin; video and audio become one `media` plugin; WebView, file storage and the FileProvider stay core.

| Commit | Content |
|--------|---------|
| `14c8715` | Permission registry: Drift keeps the request mechanism, plugins register kinds (`DriftPluginHost.registerPermission`, `platform.NewPermission`). `Request` replies with the status (async, so the UI-thread guard applies); `ErrPermissionNotRegistered` / `ErrPermissionNotDeclared`; Android requests through `activityResultRegistry`, reads LIMITED for partial grants; `DriftActivityBinding.activity` is a `FragmentActivity` |
| `7dc8baf` | `plugins/location`: usage strings from config, `always:` opt-in for background location; `Current`/`LastKnown`/`IsEnabled` async; sentinels for denied, disabled, unavailable. Core location removed (Go API, handlers, kinds, 3 permissions, `uses-feature` gps, `play-services-location`, 2 usage strings). `DriftCallError` is public for plugins |
| `43d63f4` | Background tasks deleted: the Go API, both handlers, `work-runtime` (whose manifest brought `RECEIVE_BOOT_COMPLETED`, `WAKE_LOCK`, `FOREGROUND_SERVICE`), those three template permissions, the `fetch`/`processing` modes and `BGTaskSchedulerPermittedIdentifiers`. See the background-tasks follow-up |
| `7e71c9d` | `android.manifest.add_permission` takes an optional `max_sdk` (`AddPermissionMaxSDK`), written as `android:maxSdkVersion`; plugins disagreeing on it conflict |
| `3ee2221` | `plugins/permissions`: contacts, calendar, microphone and photos, each enabled by a config section (generated `PermissionsConfig` lists the kinds to register). Core loses the four Go services and kinds, `RECORD_AUDIO`, `READ_MEDIA_*`, both external-storage permissions, `READ_CONTACTS`, `READ_CALENDAR`, and five usage strings (`NSPhotoLibraryAddUsageDescription` was used by nothing). Calendar gains the iOS 17 `NSCalendarsFullAccessUsageDescription` |
| `3562a6d` | `android.manifest.add_receiver` (`Manifest.AddReceiver`), sharing validation and the mutator with `add_service` |
| `648f1c2` | Android process-level registration: `DriftApplication` (manifest `android:name`) registers channels and plugins in `Application.onCreate`, so a process started for a service or receiver has them; a second `init`/`register` crashes. `MainActivity` decides `isNewLaunch` once (`DriftActivityBinding.isNewLaunch`); Firebase uses it, and FCM data messages and token refreshes without an Activity reach Go. Launch deep links no longer replay on recreation. Ejected checks cover the Application. Verified on the emulator: a data message sent to a killed app starts the process for the FCM broadcast (no Activity, no crash) and reaches `Messages()` when the app opens; a tap with the process alive and the Activity finished reaches `Opens()`; dark-mode recreation keeps one registration |
| `8abb0ef` | Fix (from `14c8715`): a permission dialog dismissed with Back read as permanently denied on Android, and `Request` then never prompted again. "Don't ask again" is now inferred from the rationale flag going up (first denial) and down again, and Android requests whenever not granted, letting the system decide. Verified on the emulator: dismiss, deny, deny, request reads `not_determined`, `denied`, `permanently_denied`, `permanently_denied` with no prompt |
| `ede6d0d` | `plugins/notifications`: local notifications leave core (Go API, the `drift/notifications` channels, Android handler and receiver, `POST_NOTIFICATIONS`). Core keeps the notification-center delegate, now offer-only (an unclaimed notification is not shown in the foreground), and the notification permission (`platform.Notifications` holds only `Permission`). The plugin claims its notifications by a userInfo marker (iOS) or intent extras (Android); `Schedule` takes `Interval time.Duration` and rejects combinations the platforms differ on (`Repeats` with `At`, or under a minute); shown in the foreground on both platforms; `CancelAll` leaves push alone; exact alarms only when allowed; `Settings` is `CurrentSettings(ctx)` without a duplicate status. Verified on the emulator: foreground delivery and tap, an alarm firing in a killed process (started for the receiver), and the cold-start tap queued until the app listens |
| `8b95173` | Fix: the generated `tools/drift-plugins/main.go` listed imports in drift.yaml order, so apps committing it failed `gofmt -l`; the source is now formatted (registration keeps drift.yaml order) |
| `9f72a69` | `plugins/imagepicker`: `Capture` / `Pick` async, cancel returns nothing rather than an error, multi-select on both platforms. Android uses the system photo picker and `TakePicture` through `activityResultRegistry` (no permission; asks for `CAMERA` only if another plugin declares it), writing through Drift's FileProvider, which stays core for Share. iOS needs `camera:` (the usage string) to capture. Core loses the camera service and kind, both handlers, `CAMERA`, the camera `uses-feature`s, `NSCameraUsageDescription` and the camera branch of `onActivityResult`. Verified on the emulator: pick, capture, cancel |
| `8bae363` | `plugins/securestorage`: every call async (keychain work off the main thread; the Android-only `pending` / `ErrAuthPending` / events / leaking `Listen()` are gone); `Get` returns `(value, found, err)`; only reading prompts on both platforms (Android prompted on set and delete, and orphaned data when a delete was canceled); `biometrics:` is the opt-in on both platforms (iOS usage string; on Android the plugin compiles either the BiometricPrompt gate with `androidx.biometric` or a stub that refuses); `optic_id` reaches Go. Core loses the service, both handlers, `security-crypto`, `biometric`, `USE_BIOMETRIC`/`USE_FINGERPRINT`, `NSFaceIDUsageDescription` and `-framework LocalAuthentication`. **The template manifest now declares only `INTERNET` and `ACCESS_NETWORK_STATE`.** Verified on the emulator: save/get, biometric save without enrollment fails as unavailable, after enrolling a fingerprint the prompt guards the read, cancel reads canceled |
| `4a146fd` | Platform views, Go side: `PlatformView.HandleEvent(method, args)`; each view parses its own events and the registry routes by `viewId` (only `onViewCreated` / `onViewDisposed` stay in the registry), replacing two closed switches and nine registry handlers; `PlatformViewBase` (was `basePlatformView`) is exported for plugin views. Verified on the emulator: text input, native switch, WebView page events |
| `b051d5a` | Platform views, native side: `DriftPlatformView` and `DriftPluginHost.registerPlatformView` on both platforms; one factory map replaces the create switches and per-type method allow-lists, and Drift's views register into it. iOS created views on the calling Go thread (UIKit off main) and read its view dictionary from it; now creation and every view operation run on main in call order, as Android already did. Verified on the emulator: text input, switch, WebView events and methods; `drift build xtool` |
| `42f32f4` | `plugins/media`: video (a plugin platform view, `drift/media/video`) and audio players; Go API `media.VideoPlayerController`, `media.VideoPlayer`, `media.AudioPlayerController`. HLS always, DASH opt-in (`android.dash`). Each audio controller listens for its own events and stops on `Dispose` (no process-wide registry). Core loses the Go API and widget, both platforms' players, media session and error mapping, the `drift/audio_player` channel and the four media3 dependencies; core warm-up keeps WebView and text input and posts a native-only warm-up event that plugins observe (media warms its player there). A `VideoPlayer` given a new controller now applies `HideControls` to it. Verified on the emulator: HLS video plays, seeks, hides controls; audio plays, seeks, stops, releases on leaving the page and plays again on return; the warm-up builds and releases an ExoPlayer; `drift build xtool` |
| `5d650d5` | Core waiting built-ins reply async: the storage pickers, file I/O and the date and time pickers are `asyncMethod`s on core `registerChannel` (UI-thread guard; `ctx` everywhere; no request IDs, result event channel, 300 s native timeouts or `MainActivity.onActivityResult`). `PickFile` returns the files, `PickDirectory` / `SaveFile` the path, empty on cancel; `storage_busy` maps to `ErrStorageBusy`. Fixes found on the way: `ReadFile` returned nothing on both platforms (bytes did not survive the JSON codec; contents now cross as base64), and dates came back a day early east of UTC on Android (Go's `time.Local` is UTC there; dates now cross as calendar days). Android's activity-result plumbing is one runner helper, `launchForResult`, used by permissions, storage and the image picker. Verified on the emulator: save, pick and read back (61 bytes), directory cancel, date and time pick, reopen at the picked date, cancel; `drift build xtool` |
| (this commit) | Template guard (`cmd/drift/internal/templates/guard_test.go`): the rendered manifest declares only `INTERNET` and `ACCESS_NETWORK_STATE` and no `<permission>`, `<uses-feature>`, `<receiver>` or `<service>`; neither Info.plist has `NS*UsageDescription`, `UIBackgroundModes` or `BGTaskSchedulerPermittedIdentifiers`; `Runner.entitlements` is empty; the app depends only on appcompat. Each guard was checked to fail on a planted violation. Restores platform.md's Clipboard, Haptics, App Lifecycle and System UI sections, deleted by mistake in `14c8715` (two guides linked to them). Verified: a zero-plugin `drift init` app builds on Android and xtool and runs on the emulator; its merged manifest holds `INTERNET`, `ACCESS_NETWORK_STATE` and library-internal entries only (androidx's DUMP-protected `ProfileInstallReceiver`, `DYNAMIC_RECEIVER_NOT_EXPORTED_PERMISSION`); the Docusaurus build has no broken links and no broken guide anchors |

**Verified:** step 1 on the API 36 emulator (approximate reads `limited`, upgrade to precise, two denials read `permanently_denied`, notification grant) and `drift build xtool`; step 2 on the emulator (denied before grant, fix, update stream with `geo fix`, Always through Settings), merged APK manifest, `drift build xtool` (Info.plist carries only the plugin's strings).

**iOS Simulator** (2026-10-21, `097354d`, Xcode 26.5, iPhone 17 / iOS 26.5, `drift run ios`): every Phase 5 feature.
- Location: the when-in-use prompt carries the plugin's string, `granted`, current location, the update stream follows `simctl location set`, Always upgrades.
- Permissions: contacts denied reads `permanently_denied`, calendar (iOS 17 full-access prompt) and microphone `granted`, photos "Limit Access" reads `limited`.
- Notifications: permission, foreground banner and `Deliveries()`, banner tap reaches `Opens()`, and a tap on a notification delivered to a killed app is queued until the page listens.
- Image picker: gallery pick (3000x2002); the camera cancels cleanly. The Simulator's camera previews but cannot capture.
- Secure storage: save and get, `face_id` reported, a biometric save without enrollment fails as unavailable. The keychain does not enforce `.biometryCurrentSet` on the Simulator (no prompt, item readable after unenrolling), so the prompt needs a device run.
- Platform views: WebView loads and reports pages, native switch both ways, HLS video plays, seeks and hides controls; audio plays, seeks, stops and plays again after re-entering the page.
- Storage: save (share sheet, 61 bytes), pick and read back 61 bytes, folder picker cancel.
- Date and time pickers: picked day round-trips exactly, reopen preselects it, tapping outside cancels, picked time returns.

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

### Phase 5: core feature migration (done, see Phase 5 status)

**Rule:** a feature becomes a plugin if it adds permissions, usage strings, entitlements, background modes or heavy/third-party dependencies. Everything else stays core. This matches Flutter, where every feature that moved below is a plugin.

**Why it mattered:** before Phase 5 every Drift app declared all of the permissions below. Google Play rejects `ACCESS_BACKGROUND_LOCATION` without a declaration form, and App Store review rejects unused background modes.

**Outcome** (corrected against the code; the original table listed `NotificationBridge.kt`, already deleted, and missed the local-notification receiver, the shared FileProvider and several dead declarations):

| Feature | Outcome | What left every app |
|---------|---------|---------------------|
| Push notifications | `plugins/firebase` (Phase 3) | Firebase BOM + messaging, `remote-notification` background mode |
| Local notifications | `plugins/notifications` | `POST_NOTIFICATIONS`, `DriftNotificationReceiver` and AlarmManager. Core keeps the notification-center delegate (offer-only, shared with Firebase) and the notification permission kind |
| Location | `plugins/location` | `play-services-location`, fine/coarse/background location, the gps `uses-feature`, 2 usage strings |
| Camera and gallery | `plugins/imagepicker` | `CAMERA`, the camera `uses-feature`s, `NSCameraUsageDescription`. The FileProvider stays core (Share uses it) |
| Contacts, calendar, microphone, photos | `plugins/permissions` (permission-only) | `READ_CONTACTS`, `READ_CALENDAR`, `RECORD_AUDIO`, `READ_MEDIA_*`, external-storage permissions, 5 usage strings (`NSPhotoLibraryAddUsageDescription`, `READ_MEDIA_VIDEO/AUDIO` and `WRITE_EXTERNAL_STORAGE` were used by nothing) |
| Background tasks | Deleted (a stub that never ran Go; plugin is a follow-up) | `work-runtime` and its boot/wake-lock/foreground-service permissions, `fetch`/`processing`, `BGTaskSchedulerPermittedIdentifiers` |
| Secure storage + biometrics | `plugins/securestorage` | `security-crypto`, `biometric`, `USE_BIOMETRIC`/`USE_FINGERPRINT`, `NSFaceIDUsageDescription`, `-framework LocalAuthentication` |
| Video / audio players | `plugins/media` (on the platform-view plugin API) | media3 exoplayer + hls + dash + ui |
| WebView | Stays core (no permission or dependency) | |
| Rendering, text input, accessibility, platform views, lifecycle, safe area, system UI, clipboard, haptics, share, URL launcher, deep links, preferences, storage, date/time pickers, switch, activity indicator | Stay core | Storage and the date/time pickers now reply async |

What the migration built in core: the permission registry (plugins register kinds), the platform-view plugin API, Android process-level registration (`DriftApplication`) with `isNewLaunch`, the `add_receiver` op and `add_permission` `max_sdk`, and `launchForResult` for Android activity results. Waiting calls are `asyncMethod`s throughout, so the UI-thread guard covers them and the request-ID / result-event plumbing is gone. The template guard test keeps the templates at `INTERNET` and `ACCESS_NETWORK_STATE`.

### Release sequence

**Decision (2026-10-21):** Phase 6 lands before anything merges, so `master` never holds a plugin system that cannot ship (releases are cut from `master`; `scripts/release.sh` tags the root). `master` has moved since (`27d66f9`, a CLI fix), so the stack is rebased once, at the end.

1. ~~On the Mac: run `examples/plugins/hello` and `examples/firebase-demo` on the iOS Simulator, and the Phase 5 plugins.~~ Done 2026-10-21 (Second review, Phase 5 status).
2. Do Phase 6 on `feat/plugins-release`, branched from `feat/core-plugins`, in the order below.
3. Rebase `feat/plugins-release` onto `master`. Rerun `go vet`/`go test` in the root, every `plugins/*` module and `examples/plugins/hello`. Then one combined device pass: the 6.3 checks on the iOS Simulator (see the Mac Simulator notes in the session memory, or run on the Mac directly), the 6.1 install check (`scripts/install-check.sh --build android,xtool v0.27.0`), and a smoke run of splash-demo and firebase-demo on the emulator and Simulator.
4. Fast-forward `master` to `feat/plugins-release` (`feat/plugins` and `feat/core-plugins` are its ancestors; delete them afterwards). Release: `scripts/release.sh prepare v0.27.0`, push the release commit, `scripts/release.sh v0.27.0` (the workflow tags the root and every plugin). Then `scripts/install-check.sh --real --build android,xtool v0.27.0` against the real tags.

### Phase 6: release readiness

Runs before merge, on `feat/plugins-release` (see [Release sequence](#release-sequence)). Phases 1 to 5 make plugins work; Phase 6 makes them shippable: installable outside this repo, safe in ejected projects, and free of known iOS lifecycle bugs. When it is done, the first Drift release that ships plugins can be tagged. Each item below came from the second review (2026-10-02); file references are from `03e9be8` and may drift.

**Order:** distribution first (the only hard blocker: without it nobody outside the repo can install a plugin), then ejected projects (data loss), then iOS, then author docs (which describe the distribution workflow, so they come last).

#### 6.1 Distribution and versioning (hard blocker)

- **Plugin modules require `github.com/go-drift/drift v0.0.0` with `replace => ../..`** (`plugins/*/go.mod`), and `scripts/release.sh` tags only the root. Plugins declare no minimum drift version and nothing checks skew between a plugin's native code and the CLI's `DriftPluginAPI` templates (Unresolved question 4).
- **Do:** tag each plugin module as `plugins/<name>/vX.Y.Z` from the release script; require a real drift version in each plugin's `go.mod` (keep the `replace` for in-repo development through a `go.work` instead); have the bridge report the `pkg/plugin` version it was built with and the CLI fail with a clear message on a mismatch that the `DriftPluginAPI` templates cannot serve. Decide whether plugins version in lockstep with drift (simplest) or independently.
- **Done when:** an app outside the repo can `go get github.com/go-drift/drift/plugins/splash@<version>` and build on Android and xtool; a deliberately mismatched plugin fails the build with a message naming both versions. Before merge, test with the pushed branch (`@feat/plugins-release` resolves to a pseudo-version) or a local `GOPROXY`; after tagging, repeat with the real tags (Release sequence step 4).

##### 6.1 status

Done (`fddeba4`..`7118e69`). Decisions (2026-10-26):
- **Lockstep.** First-party plugins release with Drift: one version, tagged `vX.Y.Z` and `plugins/<name>/vX.Y.Z` on one commit, in one atomic push. Third-party plugins version freely; their contract is the drift version their `go.mod` requires.
- **`replace`, not `go.work`.** Every in-repo module requires the last released drift (now `v0.26.1`) and keeps its `replace`; the release commit (`release.sh prepare`) bumps the requirements. Consumers ignore a dependency's `replace`; `go.work` would break `go mod tidy` in plugin modules (tidy ignores it and would fetch a drift tag lacking new packages) and errors on the existing replaces.
- **Skew check** (answers Unresolved question 4; `APIVersion` stays the wire format). A release CLI refuses a plugin requiring a drift newer than the CLI or older than `templates.PluginAPIFloor` (`v0.27.0`), and a bridge (which now reports `drift_version`) on another drift major.minor. Development CLIs skip it. Fatal only on the plugin path; apps without plugins keep the `CheckVersionMismatch` warning.

| Commit | Content |
|--------|---------|
| `fddeba4` | In-repo modules require drift `v0.26.1` (was `v0.0.0`), keeping their `replace` |
| `8e93629` | Bridge responses carry `drift_version` from build info; the unused `version` command is gone |
| `0881edc` | `compat.go`: per-plugin required drift vs the CLI and `PluginAPIFloor` (`VersionSkewError`, every offender listed), bridge drift vs the CLI; run from `EnsureBridge`/`RunBridge`, so build, run, sync and list check |
| `97c99c3` | `scripts/releasetool` (bump, check, tags, proxy), `release.sh prepare`, `release-tags.sh` (atomic push; a rebuild tags nothing), workflow checks modules before the Skia build |
| `b8ae4dc` | `drift init` requires the CLI's own drift version (was `@latest`, which the bridge check would then refuse) |
| `7118e69` | `scripts/install-check.sh`: release rehearsal through a file `GOPROXY`, `--real` for published tags |

**Verified (Linux):** `go vet`/`go test` in the root and every module; `scripts/install-check.sh --build android,xtool v0.27.0`: the CLI installed at `v0.27.0` from the rehearsal proxy, `drift init` outside the repo, `plugins/splash@v0.27.0` with no `replace`, `drift build android` and `drift build xtool` succeed; a plugin requiring `v0.28.0` fails naming it, `v0.28.0` and the CLI's `v0.27.0`; an app on drift `v0.28.0` fails naming both. Still to do: the pushed-branch check and, after tagging, `--real`.

#### 6.2 Ejected projects

- **A plugin upgrade looks like a removal.** Lock keys include target content (for example the Gradle coordinate with its version), so a version bump fails the next ejected build once with "plugins removed ... left these edits" and leaves the old line next to the new one. Key the lock on the target without content.
- **The lock can delete a file the same build just wrote.** If a CLI upgrade renames an op type or changes its targets, the old key disappears while the hash still matches, so `SyncEjectedLock` deletes the path after `Apply` wrote it (reproduced with `app/google-services.json`; the next build recovers). Never delete a path a current op owns.
- **Plugins can overwrite user files.** `Apply` writes owned files unconditionally and `WriteXML` only reserves Drift's `plugin_*` values files, so a plugin can replace `drawable/launch_background.xml`, `values/styles.xml`, a user's image set or a Kotlin file in `com.drift.runner`; the lock then records Drift's hash and a later removal deletes it. In ejected mode, refuse to overwrite an existing file that is not in the previous lock and differs; reserve template resource paths and the `com.drift.runner` package.
- **The lock file is written non-atomically** (`lock.go`), so a crash mid-write fails every later build; paths read from it are not confined to the project.
- **Re-checked at `bd08dd7`** (all four hold). Also: content in the lock key hits `add_intent_filter` too (its XML hash is the member); `apply_plugin` keys carry no version, but in an ejected project a version bump fails `ApplyGradleProjectPlugins` on every build (Drift's own earlier line reads as a user conflict); a key whose path changed leaks the old path; every mutator writes non-atomically, not only the lock; `TestSyncEjectedLockContentChangeIsNotRemoval` covers only an owner op.
- **Done when:** a test ejects a project, upgrades a plugin version (lock unchanged, no "removed" error), renames an op type between two runs (nothing deleted), plants a user file at a plugin's path (build refuses with the path named), and interrupts a lock write (next build recovers).

##### 6.2 status

Done (`01a7136`..`596bd02`). Decisions (2026-10-26): removal follows what the project holds (paths, edits), not op identity; versioned Gradle lines Drift wrote are updated in place on a plugin upgrade (other shared-file edits are still never undone); the overwrite guard covers managed builds too (template collisions); a removed plugin's leftover edits still fail the build once. (Changed later on 2026-10-26: they are listed in a warning and the build succeeds, since failing once made the outcome depend on whether the previous lock was committed, so every fresh checkout of a stale lock failed.)

| Commit | Content |
|--------|---------|
| `01a7136` | `mutate.WriteFileAtomic`: every project write (seven `os.WriteFile` sites, the lock, the bridge source) is a temp file renamed into place |
| `efb41ae` | `ReadLock` before `Apply` (paths confined to the project, recovery hint on a parse error); `SyncEjectedLock` deletes files no current op owns and lists edits whose key and text no current op has; intent filter edits name the filter |
| `c51823f` | `Apply` refuses (`ForeignFileError`) to replace a file the previous lock does not record as Drift's, unchanged: a user file, an edited plugin file, a template file; Kotlin sources may not use `com.drift.runner` |
| `596bd02` | Gradle dependency lock key is `Artifact()` (no version); the lock records each versioned Gradle line, and the same plugin's next version replaces it in place; plugins pinning different versions keep a line each |

**Verified:** `go vet`/`go test` (tests for each Done-when: `TestEjectedPluginUpgradeUpdatesGradleVersionsInPlace`, `TestSyncEjectedLockRenamedOpIsNotRemoval`, `TestApplyRefusesToOverwriteAUserFile`, `TestSyncEjectedLockRecoversFromAFailedWrite`, plus `TestWriteFileAtomicKeepsOldContentWhenInterrupted`). Linux, ejected firebase-demo (scratch copy) on Android: the firebase plugin's `firebase-messaging` and `google-services` versions changed, the rebuild updated both lines in place with no error and Gradle built; editing the ejected `app/google-services.json` while the plugin's config changed refused the build naming it, and undoing the edit built. Managed splash-demo builds on Android and xtool. Not run: killing a real build mid-apply (the unit tests interrupt the rename).

#### 6.3 iOS lifecycle

**Splash**
- **Timeout counts from process launch.** `DriftSplashPlugin.swift` arms the timer in `register` (`didFinishLaunching`). Firebase adds the `remote-notification` mode, so a silent push can launch the process in the background; if the user opens the app after `max_duration_ms`, Preserve is ignored and the splash hides at the first content frame. Start the timer on first attach.
- **A missed `first_frame` leaves the splash up forever** (both platforms). Dismissal requires `contentShown` and the timeout never overrides it; `first_frame` is one-shot and needs the content frame to pass every guard (`nextDrawable`, `renderSync`; `slotIndex >= 0` on Android). Re-arm on a later frame, or add a hard ceiling that ignores `contentShown`.

**iOS detach and leaks**
- **Detach never runs on scene disconnect.** `CADisplayLink(target: self)` (`DriftViewController.swift`) holds the view controller strongly until `viewDidDisappear`, `SceneDelegate` has no `sceneDidDisconnect`, and detach depends on `deinit`. Plugins stay attached to a dead controller and the controller, Metal view and display link leak, even when no plugin retains the binding (`DriftViewBinding` also holds the controller strongly). Detach explicitly in `sceneDidDisconnect` and give the display link a weak proxy target.
- **Re-checked at `bd08dd7`** (all hold). Android arms the splash timer in `onRegister` (`Application.onCreate`) too, so a process started for a push has the same bug. iOS `first_frame` moved to `DriftRenderer.swift` (the `nextDrawable` guard is in `DriftMetalView.swift`); `firstFrameRendered` is set before the presented/completed handler fires, so a drawable never presented also loses it. xtool's `DriftApp` has no `dismantleUIViewController`. Android already detaches from `MainActivity.onDestroy`.
- **Done when:** on the Simulator, a background launch followed by opening the app after `max_duration_ms` still honours Preserve; a forced `first_frame` miss dismisses at the ceiling; disconnecting the scene (App Switcher, or multiple windows on iPad) calls `detach` and Instruments shows no leaked `DriftViewController`.

##### 6.3 status

Done (`3b630a9`, `36c6d5b`), verified on the emulator and the Simulator. Decisions (2026-10-26): both splash timers start at the first attach on both platforms; a hard ceiling, `max(30 s, max_duration_ms)` after the first attach and not configurable, dismisses the splash whatever its state, instead of re-arming `first_frame`; iOS detaches explicitly (`DriftViewController.tearDown` from `sceneDidDisconnect` and xtool's dismantle) and the display link targets a weak proxy; a missing `sceneDidDisconnect` in an ejected project is documented, not a build error.

| Commit | Content |
|--------|---------|
| `3b630a9` | Splash: `max_duration_ms` and the ceiling (`ceilingMs` / `CEILING_MS` in both generated configs) start at the first attach; the ceiling logs whether a frame was drawn |
| `36c6d5b` | iOS: `tearDown` (display link invalidated, plugins detached) from `SceneDelegate.sceneDidDisconnect` (window dropped) and `DriftViewControllerRepresentable.dismantleUIViewController`; `DisplayLinkProxy`; deinit invalidates, detaches and logs |

**Verified:** `go vet`/`go test` (`TestBuild_Ceiling`); `drift build xtool` and `drift build android` of splash-demo. API 36 emulator, scratch app (splash + notifications, `max_duration_ms: 5000`, `OnInit` Preserves for 4 s): the app killed, its process restarted in the background by a notification alarm, and opened 15 s later held the splash about 4 s; the previous plugin code, in the same run, showed content at once. With `OnInit` blocking 60 s, the splash went at 30 s (error logged, "first frame drawn: false"), then a black screen until content at 60 s. **Simulator** (2026-10-26, Xcode 27.1, iOS 26.x, scratch app with splash, imagepicker and a test plugin):
- Late first window: the Simulator does not launch a killed app for a `content-available` push (neither after `simctl terminate` nor a host-side `kill -9`), so the test plugin held `didFinishLaunching` 8 s (a launch argument), putting the first window past `max_duration_ms: 5000`, as a background launch does. The splash held for the 4 s Preserve; the previous plugin code, in the same run, showed content at once.
- Ceiling: `OnInit` blocking 60 s; the splash went about 31.5 s after attach (GCD timer leeway) with the error logged, black until content at 60 s.
- Scene disconnect (iPad (10th generation); iOS ignores `requestSceneSessionDestruction` for an iPhone app's only scene even with multiple scenes enabled): closing the scene logged the plugins' `detach` and then `DriftViewController released` (its deinit); the previous templates logged neither. Closing an iPad app's last scene ends the process, and this iPad declined a second scene (`FBSceneManager declined to create a scene`), so no live-process `heap` count after disconnect; the deinit log is the evidence of release.
- `drift run ios` fails on Xcode 27 ("failed to open Simulator app"; Xcode 27 has no Simulator.app); `master`'s `27d66f9` fixes it and arrives with the rebase. Apps were installed with `simctl install` meanwhile.

#### 6.4 Author experience

- **No local-development workflow in the guide.** It does not explain the app `go.mod` `replace` needed to try an unpublished plugin, and `examples/plugins/hello` uses a repo-relative replace plus `v0.0.0`, so copying it outside the repo breaks.
- **No example app consumes `hello`**, so an author cannot run it without writing an app first.
- **No iOS open-URL or user-activity hook.** Android plugins claim intents (`onNewIntent`), start Activities for results (`launchForResult`) and provide permissions (`registerPermission`), all documented since Phase 5. iOS lost its URL and user-activity hooks in `bc57176`, so a deep-link or OAuth plugin can only be written for Android. Add them to `DriftPlugin` (first claim wins, unclaimed URLs reach `DeepLinkHandler`), routed from `SceneDelegate` and the xtool `DriftApp`.
- **Done when:** following the guide from an empty directory produces a plugin, an app that uses it, and a run on the emulator and xtool, without reading Drift's source.

##### 6.4 status

Done (`cbcc119`..`2c08426`). Decisions (2026-10-26): the iOS hooks are `openURL(_:)` and `continueUserActivity(_:)` (UIKit's verbs), app-level like the notification hooks, first claim wins, unclaimed URLs reach `DeepLinkHandler`; on xtool SwiftUI delivers only universal links; ejected projects must route through `DriftPlugins` (two new wiring checks); the example app is `examples/plugins/hello-app`.

| Commit | Content |
|--------|---------|
| `cbcc119` | iOS `DriftPlugin.openURL` / `continueUserActivity`, offered from `SceneDelegate` (launch options, `openURLContexts`, `continue`) and xtool's `DriftApp`; ejected wiring checks |
| `40b48f8` | `examples/plugins/hello-app`: the smallest app using a plugin, with the `replace` the guide teaches |
| `a559999` | Guide: "Your first plugin" walkthrough (plugin module, app beside it, sync, run, publish) and "Publishing a plugin"; `TestGuideQuotesThisPlugin` checks the walkthrough's app code against hello-app's |
| `2c08426` | Fix found by following the walkthrough: `drift plugin sync --tidy` resolved plugins before tidying, so an app that had just required a local plugin (newer `go` line from `go mod init`) failed with "updates to go.mod needed"; `drift init`'s `drift.yaml` comment describes adding a plugin |

**Verified:** `go vet`/`go test` (`TestSyncTidyResolvesANewlyRequiredPlugin` fails without the fix). The walkthrough followed verbatim outside the repo (code blocks extracted from the guide, the CLI and modules at `v0.27.0` from the `install-check.sh` rehearsal proxy): plugin module, `drift init` app, `replace`, `drift plugin sync --tidy`, `drift build xtool`, and `drift run android` showing "Hello from drift.yaml" on the API 36 emulator. hello-app in the repo: emulator and `drift build xtool`. Simulator (Xcode 27.1): with the app running, a URL the test plugin claims never reaches `DeepLinkHandler`, an unclaimed one does (`Received deep link ... source=open_url`). Not run: a launch URL, since `simctl openurl` raises an "Open in ...?" prompt and Xcode 27 removed the SimulatorKit `idb` needs to tap it; the launch path shares the same routing helper.

## Verification matrix (merge gate)

Cells give status. "Accepted" cells were not run on hardware; they rest on the emulator and Simulator runs (Phase 4 decisions).

| Check | iOS xcodeproj (Mac + device) | Android device | xtool (Linux, best effort) |
|-------|------------------------------|----------------|----------------------------|
| `go vet ./...`, `go test ./...` (root and `plugins/*`) | done (Linux, CI) | done (Linux, CI) | done |
| Splash shows, holds on `Preserve`, fades on `Remove`, auto-dismisses on real first frame | done on iPhone (Phase 2) | accepted (emulator done) | build only |
| Splash survives background/foreground during launch, dark-mode toggle | done on iPhone (Phase 2) | accepted (emulator done) | n/a |
| Firebase init, FCM/APNs token delivered to Go, foreground + background message, tap opens app | accepted (Simulator done with `simctl push`, rerun on the call-ID transport 2026-10-21; needs the APNs `.p8`) | accepted (emulator done) | build done; device run optional (needs a paid team) |
| Removing a plugin from `drift.yaml` leaves a compiling project (managed, watch mode, ejected) | accepted (managed and ejected done on Android and xtool) | done on emulator: managed, watch, ejected | done: managed |
| Two plugins touching the same plist key / resource file report a conflict | unit test | unit test | |

## Known follow-ups

Not release blocking: the release-blocking items are [Phase 6](#phase-6-release-readiness). File references are from `03e9be8`.

### Backlog (after Phase 6)

**After Phase 5**
- **Per-plugin Gradle modules on Android.** Plugin Kotlin compiles into the app module (the v1 decision), so `internal` protects nothing, a plugin can write into `com.drift.runner`, and there is no manifest merge (each component needs an op). One Gradle library module per plugin would give scoping and manifest merge, mirroring the per-plugin SwiftPM targets on iOS. Do it if loose sources cause a real conflict.
- **Asset catalogs on xtool.** If xtool#219 (AssetKit) merges, follow the `FUTURE(xtool#219)` markers: image sets and the app icon move into one Drift-owned `Assets.xcassets` listed under `assetCatalogs:`, which also unlocks splash dark mode on xtool.
- **Background tasks plugin.** Deleted from core in Phase 5: it never ran app code. Go starts only with a view (`SkiaHostView.initEngine`, `DriftRenderer.init`), the Android worker reported completion at once and iOS after a hard-coded second, iOS registered `BGTaskScheduler` handlers after launch, and events went to a live channel nobody listened to. A plugin needs a headless Go start on both platforms (OnInit without a view), a way for Go to hold the task until it completes within the OS budget, and launch-time `BGTaskScheduler` registration (the plugin `register(host:)` hook runs at the right moment). Declares `fetch`/`processing` and `BGTaskSchedulerPermittedIdentifiers` (`ctx.AppID()` prefix) through `append_array_item`, and WorkManager on Android.
- **Android 14 partial photo access.** "Allow limited access" grants `READ_MEDIA_VISUAL_USER_SELECTED`, which the permissions plugin neither declares nor reads, so `permissions.Photos` reads denied; declare it and map it to `PermissionLimited`.
- **`androidx.security:security-crypto` is a deprecated alpha.** The secure storage plugin still encrypts with `EncryptedSharedPreferences`; move to the Android Keystore directly (AES-GCM keys, optionally with a biometric-bound `CryptoObject`, which would make Android's protection cryptographic like iOS's).
- **iOS `PickDirectory` security scope.** The folder picker returns a security-scoped URL, but only its path reaches Go and access is never started (or bookmarked), so reading files inside the chosen folder fails.
- **Android persisted URI grants.** Picked documents and folders are content URIs whose grants end with the Activity; `takePersistableUriPermission` would let an app reopen them after a restart.
- **Background audio.** The media plugin stops playback in the background on both platforms; background playback needs the iOS `audio` mode (through config) and an Android `MediaSessionService` with a media-playback foreground service.
- **WebView as a plugin**, if an app ever needs to drop it: it adds no permission or dependency, so it stays core for now.
- **Go's `time.Local` is UTC on iOS and Android**, so times formatted in Go show UTC (the location page's timestamp reads an hour behind under BST). Set `time.Local` from the platform's zone at startup.
- **iOS `SaveFile` returns the temporary copy's path**, not where the user saved it (the share sheet does not report the destination). Use `UIDocumentPickerViewController(forExporting:)` to get the destination, or document that the path is not the saved file.
- **Secure storage biometric prompt on a device.** Untested: the Simulator does not enforce keychain biometric access control.
- **Optional hardware features.** Google Play infers required features from permissions (location permissions imply `android.hardware.location`; `RECORD_AUDIO` implies `android.hardware.microphone`), which hides the app from devices without them. A plugin cannot mark them optional: add an `android.manifest.add_uses_feature` op when one needs it.

- **A permission's `max_sdk` change does not reach an existing declaration** in an ejected project: `ensurePermission` keeps the declaration it finds, so a plugin upgrade changing `max_sdk` leaves the old value.

**Correctness, low severity**
- **`theme` vs `android:theme` do not conflict.** `SetActivityAttr.Targets()` uses the raw attribute name, while `setNSAttr` adds the `android:` prefix; normalise in `Targets()`.
- **Bitmap target key names the wrong directory.** It claims `drawable/<stem>` but writes `drawable-nodpi/`, missing a real clash and reporting a false one. `WriteXML` into `values*/` claims only the file, so duplicate resource names inside it surface only in Gradle.
- **Android string values are not escaped** (`mutate/xml.go`), so an apostrophe fails aapt.
- **`drift plugin sync`'s schema pass does not type-check scalars** (`n: notanint` for an int field passes it; the `Build` pass that follows reports the decode error); a null optional struct is reported as "must be a mapping".
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
- **Android auto-grouped notifications**: tapping the system's group summary opens the app without the notification's extras, so no `Opens()` event, for both Firebase and the notifications plugin; tapping an individual notification works. Posting with an explicit group and summary would fix it.
- **Local notifications after reboot (Android).** AlarmManager alarms do not survive a restart; rescheduling needs `RECEIVE_BOOT_COMPLETED` and a boot receiver in the notifications plugin.

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
4. ~~Plugin compatibility metadata: should the bridge report its `pkg/plugin` version so the CLI can hard-fail on skew?~~ Yes, plus a per-plugin check against the drift its `go.mod` requires; plugins version in lockstep (see [6.1 status](#61-status)).
