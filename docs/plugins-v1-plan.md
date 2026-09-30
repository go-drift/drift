# Plugins v1: plan

Handoff document for finishing the `feat/plugins` branch. Read [plugins.md](plugins.md) first for how the system works. Line numbers below are from commit `261864d` and may drift.

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

### Phase 1: core fixes (before building more on top)

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

### Phase 2: splash works on devices

- Install the overlay in `onAttach`.
- Gate the Android 12 controller to API 31+. Make `first_frame` independent of the keep-on-screen draw suppression; for example, emit from the engine when the first non-empty layer tree is presented, then release keep-on-screen.
- Emit `first_frame` only after the root is mounted and composited (not during `OnInit`).
- Retry a pending dismiss on `didBecomeActive`.
- Add a configurable max-duration timeout. Surface `Invoke` errors (log or return).
- Fix the colour file clash, which the target-keyed identity from phase 1 should catch.
- Define hex as `#RRGGBB` / `#RRGGBBAA` and convert for Android.
- Emit branding and dark variants, or delete those config fields.
- Update `examples/splash-demo/drift.yaml` (its comment about xtool is stale).

### Phase 3: Firebase plugin, push moves out of core

1. **Spike on a Mac first.** Write a throwaway plugin with `AddPackageDependency(firebase-ios-sdk, FirebaseCore)` and a Swift source that imports FirebaseCore. Confirm SwiftPM resolves `package: "firebase-ios-sdk"` and that the import compiles. The result decides phase 1, item 6.
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
   - `ios.app_delegate_registrant` callbacks: keep only those Firebase uses.
   - `DriftPluginCoordinator` background-fetch merge.
   - `android.assets.add`.
   - `android.gradle.apply_plugin`: needed for google-services.
   - `android.app_module.add_file`: needed for `google-services.json`.

### Phase 4: docs and merge

- Add `website-docs/guides/plugins.md` covering using plugins, authoring plugins (build half, runtime half, native code, testing with `NewTestCtx`) and the op reference.
- Update `docs/plugins.md` to match the final design.
- Walk through the device verification matrix below, then merge.

### Phase 5: core feature migration

**Rule:** a feature becomes a plugin if it adds permissions, usage strings, entitlements, background modes or heavy/third-party dependencies. Everything else stays core. This matches Flutter, where every "move" row below is a plugin.

**Why it matters:** today every Drift app declares all of the permissions below. Google Play rejects `ACCESS_BACKGROUND_LOCATION` without a declaration form, and App Store review rejects unused background modes.

| Feature | Move? | What it adds to every app today | Go API (`pkg/platform`) | Native (templates) |
|---------|-------|---------------------------------|-------------------------|--------------------|
| Push notifications | **Phase 3** (Firebase plugin) | Firebase BOM + messaging, `POST_NOTIFICATIONS`, `DriftNotificationReceiver`, `remote-notification` background mode | `notifications.go`, `notification_permission.go` | `NotificationHandler` (iOS in `PlatformChannel.swift`, Android `NotificationHandler.kt`, `NotificationBridge.kt`, `DriftNotificationReceiver.kt`) |
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
- Plugins needing `<service>`/`<receiver>`/`<provider>` or background modes depend on phase 3's new ops (or phase 1's per-plugin Gradle modules).
- Showcase (`showcase/`) uses many of these APIs; update it as each feature moves.

## Verification matrix (merge gate)

| Check | iOS xcodeproj (Mac + device) | Android device | xtool (Linux, best effort) |
|-------|------------------------------|----------------|----------------------------|
| `go vet ./...`, `go test ./...` | | | |
| Coordinator harness (`xcodebuild test` in `cmd/drift/internal/plugin/coordinator_test`), if kept | ✓ | | |
| Splash shows, holds on `Preserve`, fades on `Remove`, auto-dismisses on real first frame | ✓ | ✓ (API 29, 31+) | ✓ |
| Splash survives background/foreground during launch, dark-mode toggle | ✓ | ✓ | |
| Firebase init, FCM/APNs token delivered to Go, foreground + background message, tap opens app | ✓ | ✓ | |
| Removing a plugin from `drift.yaml` leaves a compiling project (managed, watch mode, ejected) | ✓ | ✓ | ✓ |
| Two plugins touching the same plist key / resource file report a conflict | unit test | unit test | |

## Known follow-ups (not merge blocking)

- **xtool may create two windows (unverified).** `SceneDelegate.swift` is copied into xtool (`scaffold/xtool.go:52-57`) and named in `xtool/Info.plist.tmpl` next to a SwiftUI `WindowGroup`. The `xtool/AppDelegate.swift` comment claims there is no SceneDelegate.
- **xtool launch storyboard.** It is uncompiled and sits in the SwiftPM resource bundle, while `UILaunchStoryboardName` looks in the main bundle.
- **xtool forwards only web-browsing activities.** `DriftApp.swift` forwards only `NSUserActivityTypeBrowsingWeb` activities to plugins.
- **Sticky event replay** can arrive after a newer live event (`pkg/platform/channel.go` Listen). `ResetForTest` does not clear the replay slot.
- **Bridge cache entries** under the cache root are never garbage-collected.
- **`drift plugin list --resolve`** rewrites the bridge file even when plugins are missing.

## Unresolved questions

1. Per-plugin SwiftPM targets / Gradle modules, or loose sources? Decide after the phase 3 Firebase spike.
2. Do local notifications stay core until a `notifications` plugin exists, or move with push?
3. Keep the xcodeproj/xtool dual path for plugins that need SwiftPM products on xtool, or declare some plugins xcodeproj-only?
4. Plugin compatibility metadata: should the bridge report its `pkg/plugin` version so the CLI can hard-fail on skew (today `APIVersion` stays `1`)?
