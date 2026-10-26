---
id: plugins
title: Plugins
sidebar_position: 8
---

# Plugins

Plugins add native capabilities to a Drift app: SDKs, permissions, native UI, and platform hooks. The app never edits the generated Xcode or Android Studio projects; Drift applies each plugin's changes on every build.

A plugin is a Go module (a first-party plugin is a directory of the Drift module) with up to three parts:

| Part | Package | Runs | Purpose |
|------|---------|------|---------|
| Build half | `<module>/plugin` | At build time | Reads the plugin's config from `drift.yaml` and records the native project changes it needs (**ops**) |
| Native code | Swift and Kotlin embedded in the build half | In the app | The platform side: SDK calls, lifecycle hooks, overlays |
| Runtime half | `<module>/runtime` (any name) | In the app | The Go API the app calls; talks to the native code over platform channels |

First-party plugins ship in the `github.com/go-drift/drift` module, at the same version as the rest of Drift:

- [`plugins/splash`](#splash): a native splash screen held until the first frame.
- [`plugins/firebase`](#firebase): Firebase Core and Cloud Messaging (push notifications).
- [`plugins/location`](#location): the device's location and the location permissions.
- [`plugins/permissions`](#permissions): contacts, calendar, microphone and photos permissions.
- [`plugins/notifications`](#notifications): local notifications and taps on them.
- [`plugins/imagepicker`](#image-picker): a photo from the camera, or images from the photo library.
- [`plugins/securestorage`](#secure-storage): encrypted key-value storage, optionally behind biometrics.
- [`plugins/media`](#media): video and audio playback.

## Using plugins

### Adding a plugin

List the plugin's build half in `drift.yaml`. A first-party plugin needs nothing else: it comes with your app's `github.com/go-drift/drift` dependency.

```yaml
# drift.yaml
plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
      background_color: "#1A2238"
```

A third-party plugin is a dependency of its own; add its module first:

```bash
go get example.com/hello@<version>
```

The next `drift build` or `drift run` picks it up. Plugins run in `drift.yaml` order, which also decides which plugin gets first claim on shared events such as notification taps.

`config` is checked against the plugin's typed config: unknown keys, missing required keys, malformed colours and missing files fail the build with the offending key named.

### The plugin bridge

Drift runs build halves through a small generated program, `tools/drift-plugins/main.go`, built and cached by the CLI. Commit it: it imports every configured plugin, which keeps their modules in `go.mod` when you run `go mod tidy`.

Drift never edits `go.mod` or `go.sum` during a build. If an entry is missing, the build stops and tells you to run `go mod tidy` (or `go get` for a third-party plugin that is not a dependency yet). A first-party plugin that is not found has a wrong package path, or is newer than the app's Drift: the error names the Drift version to update to (see [Versions](#versions)).

```bash
drift plugin sync          # regenerate the bridge, check drift.yaml
drift plugin sync --tidy   # same, running go mod tidy first
drift plugin list          # configured plugins
drift plugin list --resolve  # with module versions and status
drift plugin list --json
```

`drift plugin sync` checks each plugin's config against its schema and then runs the plugin's `Build` without building the app, so it rejects the config a build would. It exits nonzero on invalid config, so CI can run it.

### Versions

First-party plugins are part of the `github.com/go-drift/drift` module, so they always match the Drift your app uses. Use that module and the `drift` CLI at one version, and upgrade them together; for example, to v0.28.0:

```bash
go install github.com/go-drift/drift/cmd/drift@v0.28.0
go get github.com/go-drift/drift@v0.28.0
```

A plugin's native code compiles against the plugin API the CLI ships, so a release CLI checks every build and stops on a mismatch, naming the versions involved:

| Error | Cause | Fix |
|-------|-------|-----|
| Plugins require a newer drift than this CLI | A plugin's `go.mod` requires a newer Drift minor release than the CLI's (v0.28.x against a v0.27.x CLI), so its native code may use API the CLI lacks; patch releases do not change the plugin API | Install the CLI the error names |
| Plugins were written for an older plugin API | A plugin requires a Drift older than the oldest plugin API the CLI still serves | Update the plugin, or use the CLI it was written for |
| The app builds with drift X, but this CLI is Y | The app's `go.mod` selects a Drift whose major.minor differs from the CLI's | Install the CLI matching the app, or change the app's Drift version |

Development builds of the CLI (built from a checkout) skip these checks.

### Removing a plugin

Delete its entry from `drift.yaml`. Managed builds regenerate the native project from scratch, and `drift run --watch` regenerates it as soon as the plugin set changes, so nothing is left behind. With no plugins left, Drift also deletes `tools/drift-plugins/main.go`; run `go mod tidy` to drop the module.

### Ejected projects

Plugins work in [ejected](/docs/guides/eject) projects too, applied to your files on every build. Drift owns these files and rewrites them each build:

| iOS (`platform/ios`) | Android (`platform/android`) |
|----------------------|------------------------------|
| `Drift/Plugins/` (a local Swift package holding every plugin's Swift module) | `app/src/main/java/com/drift/runner/` (plugin host support and the generated registrant) |
| `Runner/DriftPlugins.swift`, `Runner/DriftPluginRegistrant.swift` | |

Plugins also edit shared files: `Info.plist`, `Runner.entitlements`, `AndroidManifest.xml`, `build.gradle`, resource values. Drift records what it applied in `.drift/plugins.lock.json` in each platform project; commit it.

When you remove a plugin, Drift deletes the files it owned (unless you changed them), restores the template's `LaunchScreen.storyboard` if a plugin replaced it and you have not changed it since, and the next build warns with a list of the edits it made inside shared files, such as an `Info.plist` key or a manifest permission. Drift cannot tell those apart from your own edits, so you remove them, or keep them if you still want them. The build still succeeds; commit the updated lock, or each build from a checkout of the old one warns again.

When a plugin upgrade changes a Gradle dependency's version or a Gradle plugin's version, Drift updates the line it wrote in place. If you changed that line yourself, Drift leaves it and adds its own.

Drift never overwrites a file it did not write. If a plugin would replace one of your files, or a plugin file you edited, the build fails and names each file. That includes `LaunchScreen.storyboard`: a plugin such as splash replaces it only while it is the template's or what the plugin wrote, never your own launch screen. Move yours aside (or undo your change) to let the plugin write it, or remove the plugin. Like any plugin op that fails, this stops the build before Drift changes a file in the project.

If `.drift/plugins.lock.json` stops parsing (a merge conflict, say), resolve it, or delete it: Drift then treats plugin files as new, and refuses only those whose content differs from what the plugin writes.

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
| `max_duration_ms` | `10000` | This long after the app's window appears, `Preserve` no longer holds the splash |
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

Calls are counted: the splash fades when every `Preserve` has a matching `Remove` and the first frame is drawn. After `max_duration_ms`, unmatched `Preserve` calls stop holding it, so a missed `Remove` cannot keep it up forever; a slow `App.OnInit` still does, since there is nothing to show yet, for up to 30 seconds (or `max_duration_ms`, if longer). After that the splash goes whatever the state, so the app is never hidden for good. Both times count from when the app's window first appears, not from process start: an app launched in the background, for a push or an alarm, keeps its full splash time for when the user opens it.

Supported on iOS, xtool and Android. On xtool, whether the replacement launch storyboard renders is unverified; the native overlay that follows it works.

### Firebase

Firebase Core and Cloud Messaging: an FCM token, messages, notification taps, and topics.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/firebase/plugin
    config:
      ios:
        config_file: firebase/GoogleService-Info.plist
      android:
        config_file: firebase/google-services.json
```

Register both apps in the Firebase console with `drift.yaml`'s `app.id` as the iOS bundle ID and Android package name, and download their config files. The build checks that each file names your `app.id` and has the values Firebase checks when the app starts (app ID, project ID, API key, sender ID), so a file that would stop Firebase at launch fails the build instead. Each platform's section is required to build for that platform. iOS also needs an APNs key uploaded to the Firebase project and a signing team; the plugin adds the push entitlement. [`examples/firebase-demo`](https://github.com/go-drift/drift/tree/master/examples/firebase-demo) walks through the setup and includes a test sender.

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

Topic subscriptions and token deletion wait for Firebase's servers, so call them from a goroutine; from the UI thread they fail with `platform.ErrBlocksUIThread`.

```go
// Receive messages sent to a topic.
go func() {
    if err := messaging.SubscribeToTopic(ctx, "news"); err != nil {
        log.Printf("subscribe: %v", err)
    }
}()

// On logout: stop this user's messages, topics included.
go func() {
    if err := messaging.DeleteToken(ctx); err != nil {
        log.Printf("delete token: %v", err)
    }
}()
```

A topic name is 1 to 900 characters from `a-z`, `A-Z`, `0-9` and `-_.~%`; others fail with `messaging.ErrInvalidTopic` before reaching Firebase. `DeleteToken` deletes the token, so neither messages sent to it nor topic messages reach the device; `Token()` is `""` when it returns. Firebase issues a new token by the time the app next starts, and `Token()` reports it like any other: send it to your server, and subscribe to the user's topics again, only once a user logs in.

| Situation | What happens |
|-----------|--------------|
| Notification message, app in the foreground | Reaches `Messages()` only; nothing is shown, on both platforms |
| Notification message, app in the background or not running | The system shows it; a tap opens the app and reaches `Opens()` |
| Data message, app running | Reaches `Messages()` with `Foreground` set accordingly |

Limitations:

- Push on iOS needs a paid Apple Developer team, on xtool as with Xcode. With a free team the app still runs, but Apple grants no push capability, so no token or message arrives.
- Android: when the system groups several notifications from the app, tapping the group summary opens the app without an `Opens()` event. Tapping an individual notification works.

### Location

The device's location, once or as a stream, and the location permissions.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/location/plugin
    config:
      when_in_use_description: Shows stores near you.
      always:                       # optional
        description: Records your runs while the app is in the background.
        notification_title: Recording your run
        notification_text: Your route is being recorded.
```

| Key | Default | Meaning |
|-----|---------|---------|
| `when_in_use_description` | required | Why the app needs location; iOS shows it in the permission prompt |
| `always.description` | none | Enables the Always permission and background updates; iOS shows it when asking to upgrade |
| `always.notification_title` | `Using your location` | Title of the ongoing notification Android shows during background updates |
| `always.notification_text` | `Location updates continue while the app is in the background.` | Its text |

Leave `always` out unless the app needs location while in the background. It declares:

- iOS: `NSLocationAlwaysAndWhenInUseUsageDescription` and the `location` background mode (`UIBackgroundModes`), which App Review expects the app to justify.
- Android: `ACCESS_BACKGROUND_LOCATION`, which Google Play accepts only with a background location declaration; a foreground service of type `location` (with `FOREGROUND_SERVICE` and `FOREGROUND_SERVICE_LOCATION`); and `POST_NOTIFICATIONS`, so the app can request `platform.Notifications.Permission`, without which Android 13+ keeps the service's notification out of the notification drawer.

Without it, `Permission.Always` fails with `platform.ErrPermissionNotRegistered`, and background updates with `location.ErrBackgroundNotConfigured`.

```go
import location "github.com/go-drift/drift/plugins/location/runtime"

go func() {
    status, err := location.Permission.WhenInUse.Request(ctx)
    if err != nil || (status != platform.PermissionGranted && status != platform.PermissionLimited) {
        return
    }
    fix, err := location.Current(ctx, location.Options{HighAccuracy: true})
    if err != nil {
        log.Printf("location: %v", err)
        return
    }
    drift.Dispatch(func() { showOnMap(fix) })
}()

// A stream of updates while StartUpdates is in effect.
unsubscribe := location.Updates().Listen(func(u location.Update) {
    drift.Dispatch(func() { moveMarker(u) })
})
err := location.StartUpdates(location.UpdateOptions{
    Options: location.Options{HighAccuracy: true, DistanceFilter: 10},
})
```

| Function | Waits | Notes |
|----------|-------|-------|
| `Permission.WhenInUse`, `Permission.Always` | `Request` does | `platform.Permission`s; approximate-only access on Android reads `PermissionLimited` |
| `Current(ctx, opts)` | yes | A fresh fix |
| `LastKnown(ctx)` | yes | The cached fix, or nil |
| `IsEnabled(ctx)` | yes | Whether location services are on |
| `StartUpdates(opts)`, `StopUpdates()` | no | Start or stop `Updates()`; a failed start leaves the running stream as it was |

Functions that wait must be called from a goroutine; from the UI thread they fail with `platform.ErrBlocksUIThread`. Failures match `location.ErrPermissionDenied`, `location.ErrDisabled` (location services off) and `location.ErrUnavailable` (no fix, Android) under `errors.Is`.

On iOS, Always can be requested only after WhenInUse is granted, and iOS asks at most once: once asked, Always reads `PermissionPermanentlyDenied` until the user grants it in Settings. On Android, Always is granted in Settings, where the request takes the user. Supported on iOS, xtool and Android (Google Play services).

#### Background updates

A stream started with `Background: true` keeps delivering updates to the app's Go code while the app is in the background:

```go
err := location.StartUpdates(location.UpdateOptions{
    Options:    location.Options{HighAccuracy: true, DistanceFilter: 10},
    Background: true,
})
```

It fails instead of quietly running only in the foreground: with `location.ErrBackgroundNotConfigured` without the `always` config, with `location.ErrAlwaysDenied` until the user grants `Permission.Always`, and on Android with `location.ErrBackgroundStart` when the app is not in the foreground, since Android 12+ starts the foreground service only from there. Start it while the app is on screen, for example from the button that starts a run. Restarting the stream from the background with new options is fine; `StopUpdates` ends it.

Updates reach Go, so they last only as long as the app's process:

| | While the stream is on | Ends when |
|---|---|---|
| iOS | The app keeps running in the background, with the blue location indicator in the status bar; updates do not pause automatically | The user force-quits the app (swipes it away in the app switcher) or the system terminates it; iOS does not relaunch the app for these updates |
| Android | A foreground service keeps the process alive, showing the ongoing notification from the `always` config; a tap opens the app | The process is killed (Force stop, Stop in the Task Manager, low memory); the service is not restarted. Swiping the app away from Recents usually leaves it running, though some manufacturers' builds kill it |

### Permissions

Contacts, calendar, microphone and photos permissions, for apps whose own native code or SDKs use them. Add a section for each permission the app needs; the others are not declared.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/permissions/plugin
    config:
      contacts:
        description: Finds friends who already use the app.
      photos:
        description: Picks a profile photo.
```

| Section | Android | iOS usage string |
|---------|---------|------------------|
| `contacts` | `READ_CONTACTS` | `NSContactsUsageDescription` |
| `calendar` | `READ_CALENDAR` | `NSCalendarsFullAccessUsageDescription`, `NSCalendarsUsageDescription` |
| `microphone` | `RECORD_AUDIO` | `NSMicrophoneUsageDescription` |
| `photos` | `READ_MEDIA_IMAGES`, `READ_EXTERNAL_STORAGE` up to API 32 | `NSPhotoLibraryUsageDescription` |

Each `description` is the text iOS shows in the permission prompt. A permission without a section fails with `platform.ErrPermissionNotRegistered`, naming the missing section.

```go
import permissions "github.com/go-drift/drift/plugins/permissions/runtime"

go func() {
    status, err := permissions.Contacts.Request(ctx)
    ...
}()
```

`permissions.Contacts`, `Calendar`, `Microphone` and `Photos` are `platform.Permission`s. Selected photos (iOS) and write-only calendar access (iOS 17) read `PermissionLimited`. Supported on iOS, xtool and Android.

### Notifications

Local notifications the app schedules, and taps on them. The plugin takes no config:

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/notifications/plugin
```

```go
import notifications "github.com/go-drift/drift/plugins/notifications/runtime"

// Ask once, from a goroutine.
go platform.Notifications.Permission.Request(ctx)

go func() {
    err := notifications.Schedule(ctx, notifications.Request{
        ID:    "standup",
        Title: "Stand-up",
        Body:  "In five minutes",
        At:    time.Now().Add(55 * time.Minute),
        Data:  map[string]any{"route": "/meetings"},
    })
    ...
}()

// A tap on one of the app's notifications, including the one that launched it.
notifications.Opens().Listen(func(o notifications.Open) {
    drift.Dispatch(func() { navigate(o.Data["route"]) })
})
```

| Function | Notes |
|----------|-------|
| `Schedule(ctx, req)` | Scheduling an `ID` again replaces it. `At` is when; without it, after `Interval`. `Repeats` shows it every `Interval` (at least a minute) from now, and cannot be combined with `At`. Waits for the platform to accept the request, so call it from a goroutine; a request the platform rejects fails with its error |
| `Cancel(id)`, `CancelAll()` | Remove scheduled and shown notifications; `CancelAll` leaves other plugins' (push) alone |
| `SetBadge(n)` | iOS app icon badge; Android launchers show their own |
| `CurrentSettings(ctx)` | Whether alerts, sounds and badges are on; waits for the platform, so call it from a goroutine |
| `Deliveries()` | The app's notifications as they are shown while it runs, with `Foreground` set |
| `Opens()` | Taps, queued until the app listens |

The permission is Drift's own, `platform.Notifications.Permission`, shared with the Firebase plugin. Notifications are shown in the foreground too, on both platforms. Each plugin claims only its own notifications, so this plugin and Firebase work side by side.

Limitations:

- Android schedules with `AlarmManager`: exactly when the user lets the app schedule exact alarms (Settings, Alarms & reminders), otherwise within Android's window for inexact alarms. Scheduled notifications are lost when the device restarts.
- Android: when the system groups several notifications from the app, tapping the group summary opens the app without an `Opens()` event, as for Firebase.
- `Deliveries()` reports notifications shown while the app runs; one that fires while the app is closed reaches only `Opens()`, if tapped.

Supported on iOS, xtool and Android.

### Image picker

A photo from the camera, or images from the photo library, through the system's own camera and photo picker.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/imagepicker/plugin
    config:
      camera:                       # optional: needed to take photos on iOS
        description: Takes a photo for your profile.
```

`camera.description` is the text iOS shows when asking for camera access. Picking from the library needs no permission on either platform, and Android takes photos through the system camera app, which needs none either.

```go
import imagepicker "github.com/go-drift/drift/plugins/imagepicker/runtime"

go func() {
    images, err := imagepicker.Pick(ctx, imagepicker.PickOptions{Multiple: true})
    if err != nil || len(images) == 0 {
        return // failed, or the user canceled
    }
    drift.Dispatch(func() { s.show(images) })
}()

photo, err := imagepicker.Capture(ctx, imagepicker.CaptureOptions{}) // nil if canceled
```

`Capture` and `Pick` wait for the user, so call them from a goroutine. Each `Image` is a file in the app's cache directory (`Path`, `MimeType`, `Width`, `Height`, `Size`); move it to keep it. Decode it with Drift's `image.Decode` (`github.com/go-drift/drift/pkg/image`), not the standard library's: photos keep the pixels as the camera stored them plus an EXIF orientation, which Drift's `Decode` applies so the image comes out upright. `Width` and `Height` are that upright size on both platforms. Failures match `imagepicker.ErrBusy` (a picker is open), `ErrCameraUnavailable` (no camera, such as the iOS Simulator), `ErrCameraNotConfigured` (iOS without `camera:`) and `ErrPermissionDenied` under `errors.Is`.

On Android the camera writes through Drift's `FileProvider`. If another plugin declares `CAMERA`, Android requires that permission for the camera app too, so `Capture` asks for it first. `FrontCamera` is a hint some Android camera apps ignore. Supported on iOS, xtool and Android.

### Secure storage

String values encrypted at rest (the iOS Keychain, Android `EncryptedSharedPreferences`), optionally readable only after biometric authentication.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/securestorage/plugin
    config:
      biometrics:                   # optional: RequireBiometric
        face_id_description: Unlocks your saved passwords.
```

`biometrics.face_id_description` is the text iOS shows when asking to use Face ID. Without the section, the app carries no biometric code (Android's `androidx.biometric` and its `USE_BIOMETRIC`), and `RequireBiometric` fails with `securestorage.ErrBiometricsNotConfigured`.

```go
import securestorage "github.com/go-drift/drift/plugins/securestorage/runtime"

go func() {
    err := securestorage.Set(ctx, "refresh_token", token, &securestorage.Options{RequireBiometric: true})
    ...
    token, found, err := securestorage.Get(ctx, "refresh_token", &securestorage.Options{
        BiometricPrompt: "Sign in to your account",
    })
}()
```

| Function | Notes |
|----------|-------|
| `Set(ctx, key, value, opts)` | Replaces any value under `key`; never prompts |
| `Get(ctx, key, opts)` | Value and whether there is one; prompts for a `RequireBiometric` value |
| `Delete`, `DeleteAll`, `Contains`, `Keys` | Never prompt |
| `AvailableBiometric(ctx)` | `BiometricFaceID`, `BiometricTouchID`, `BiometricOpticID`, `BiometricFingerprint`, `BiometricFace` or `BiometricNone` |

`Options` sets the `Service` (a namespace), iOS `Accessibility` (when a value can be read, such as `AfterFirstUnlock` for background work), `RequireBiometric` and the `BiometricPrompt` text. Every call waits for the platform, so call them from a goroutine. Failures match `ErrAuthCanceled`, `ErrAuthFailed`, `ErrBiometricsUnavailable` (no hardware or nothing enrolled), `ErrBiometricsNotConfigured` and `ErrLocked` under `errors.Is`.

Biometric protection guards reading a value. On iOS the Keychain enforces it; on Android the plugin does, before it decrypts the value (BiometricPrompt without a CryptoObject), so it stops another person using the unlocked device, not code running as the app. Supported on iOS, xtool and Android.

On Android, values last as long as the install that wrote them. Auto Backup and device transfer copy the encrypted files but not the Android Keystore key that decrypts them (it never leaves the device), and uninstalling deletes the key, so a restored store opens empty: the plugin resets it and logs a warning. Keep what secure storage holds recoverable, such as a token the user gets back by signing in again. On iOS, Keychain items move with an encrypted backup or device transfer unless their `Accessibility` is a `ThisDeviceOnly` one.

### Media

Video in the widget tree, with the platform's own controls, and audio players without a view. Android plays through media3 ExoPlayer and iOS through AVPlayer.

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/media/plugin
    config:
      android:                      # optional
        dash: true                  # DASH streams on Android
```

HLS is always supported. `android.dash` adds media3's DASH module; iOS has no DASH support. Playback needs no permission.

```go
import media "github.com/go-drift/drift/plugins/media/runtime"

// In InitState:
s.video = media.NewVideoPlayerController()
core.UseDisposable(s, s.video)
s.video.OnPlaybackStateChanged = func(state media.PlaybackState) { ... }
s.video.Load(url)

// In Build:
media.VideoPlayer{Controller: s.video, Height: 225}
```

`media.NewAudioPlayerController()` has the same methods and callbacks without a view; several can play at once. The [Media Player](/docs/guides/media-player) guide covers both. Playback stops when the app goes to the background. Supported on iOS, xtool and Android.

## Authoring plugins

The code on this page is [`examples/plugins/hello`](https://github.com/go-drift/drift/tree/master/examples/plugins/hello), the smallest complete plugin: it takes a greeting from `drift.yaml` and ships native code that returns it to Go. [`examples/plugins/hello-app`](https://github.com/go-drift/drift/tree/master/examples/plugins/hello-app) is an app using it. The splash and Firebase plugins are full references: splash for native UI and lifecycle, Firebase for an SDK with a SwiftPM package, a Gradle plugin, config files, entitlements and a manifest service.

### Your first plugin

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

### Module layout

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

The `github.com/go-drift/drift` version the plugin's `go.mod` requires declares the plugin API its native code is written against: require the oldest Drift release whose API the plugin uses. A release CLI refuses a plugin requiring a newer Drift minor release than its own (patch releases do not change the plugin API), or one older than the oldest plugin API it still serves (see [Versions](#versions)). Version the plugin itself however you like.

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
- Record ops for every platform unconditionally; Drift applies the ones for the platform being built. `ctx.Platform()` (`"ios"`, `"xtool"` or `"android"`) is for the rare plugin whose ops must differ between them. Under `drift plugin sync` it is `"all"`: record for every platform the config enables, and check the config they need.
- Support xtool. It is how iOS apps are developed on Linux, so a plugin that rejects it blocks development of every app that uses it. If something cannot work there (for example, a capability xtool cannot sign), build anyway and document what is missing.

### Config

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
        host.registerChannel("example/hello") { channel in
            channel.method("greeting") { _ in
                Bundle.main.object(forInfoDictionaryKey: "HelloGreeting") as? String
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
            method("greeting") { _ ->
                // Plugin code cannot see the app's R class; look the
                // resource up by name.
                val res = host.context.resources
                val id = res.getIdentifier("hello_greeting", "string", host.context.packageName)
                res.getString(id)
            }
        }
    }
}
```

Android libraries come from `ctx.Android.AddGradleDependency`; components the system starts are declared with `ctx.Android.Manifest.AddService` (a `FirebaseMessagingService`, say) and `AddReceiver` (a `BroadcastReceiver`, such as an alarm's).

### Lifecycle

The app creates one instance of each plugin class per process, in `drift.yaml` order, and calls it on the main thread. Every method except registration has a default no-op implementation.

| | iOS (`DriftPlugin` protocol) | Android (`DriftPlugin` interface) |
|---|---|---|
| Once per process | `register(host:)`, then `didFinishLaunching(_:options:)` | `onRegister(host)`, when the process starts (`Application.onCreate`), before any Activity, service or receiver; a process may start without an Activity, for a push message or an alarm |
| View bound | `attach(DriftViewBinding)` / `detach()` around the Drift view controller, which detaches when its scene disconnects | `onAttach(DriftActivityBinding)` / `onDetach()` around each Activity (recreation detaches and attaches again) |
| Overlays | `binding.overlayView`: above Drift's content and platform views; touches pass through where it is empty | `binding.overlayView`: the window's decor view |
| App events | `openURL` (a URL the app is opened with, such as an OAuth redirect), `continueUserActivity` (universal links, Handoff, Spotlight; on xtool universal links only), `didRegisterForRemoteNotifications`, `didFailToRegisterForRemoteNotifications`, `didReceiveRemoteNotification`, `willPresentNotification`, `didReceiveNotificationResponse`; launch URLs and activities included | `binding.addOnNewIntentListener`; the launch intent is `binding.activity.intent`, to act on only when `binding.isNewLaunch` (false for a recreated Activity or one relaunched from Recents) |

Register channels and observers once, in the registration method. Keep view state (overlays, view references) between attach and detach, and never keep a binding past detach.

App events are offered to plugins in `drift.yaml` order. A hook that returns `true` (or presentation options) claims the event: later plugins and Drift's own handling do not see it. Claim only your own events, such as notifications from your push provider. A plugin that claims `didReceiveRemoteNotification` must call its completion handler exactly once.

### Host API

`DriftPluginHost` (passed to the registration method) has the same shape on both platforms:

| Method | Purpose |
|--------|---------|
| `registerChannel(name) { ... }` | Handle method calls from Go, declaring each method in the block |
| `sendEvent(channel, data)` | Send an event to Go |
| `sendEventError(channel, code, message)` / `sendEventDone(channel)` | Report a stream error or end |
| `observeEvent(channel, handler)` | Receive events sent on a channel by any native module, such as the engine's `first_frame` on `drift/rendering/frame_events`; returns a subscription to cancel |
| `registerPermission(kind, permission)` | Provide a runtime permission kind (see [Providing permissions](#providing-permissions)) |
| `registerPlatformView(type, factory)` | Provide a native view type to embed in widgets (see [Platform views](#platform-views)) |
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

To fail a call with an error Go can match, throw a `DriftCallError(code, message)` (Swift and Kotlin alike), or pass it to `result.error`; an optional `details` argument carries JSON-like data. Go receives a `*platform.ChannelError` with that `Code`, `Message` and `Details`; the runtime half maps codes to its own sentinel errors.

- Missing or malformed arguments: `DriftCallError.invalidArguments(message)` on iOS, an `IllegalArgumentException` (as `require` throws) on Android. Both match `platform.ErrInvalidArguments`.
- Any other error reaches Go as `native_error`, with the `NSError` domain and code (iOS) or the exception class (Android) in `Details`.

Values crossing the channel are JSON-like: nil, booleans, finite numbers, strings, lists and string-keyed maps. Send bytes as base64 strings (a Go `[]byte` argument already arrives as one). A reply or event outside that set, such as a NaN, an Infinity or a `Date`, never reaches Go as data: the call fails with `platform.ErrNotEncodable`, and an event arrives as that error on the stream's error handler, its message naming the call or channel and the offending part (`value.items[2] is NaN, not a finite number`). Native logs it too.

On Android, start an Activity for its result (a system picker, a camera app) with `launchForResult(activity, contract, input) { output -> ... }` from `com.drift.runner`, passing the binding's `FragmentActivity`. It calls back once: with the output, or with `null` if the Activity is destroyed first, so an `asyncMethod` waiting on it always replies.

Name channels `<vendor>/<feature>`, such as `acme/camera`; `drift/` is reserved for Drift and its first-party plugins. Registering a channel name twice traps at startup.

### Threading

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

### Providing permissions

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

### Platform views

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

Drift creates and drops a throwaway WebView at launch so the first real one opens without a stall. A view whose framework is slow to load can do the same: observe the warm-up event (`host.observeEvent(driftPlatformViewWarmUpEvent)` on iOS, `DRIFT_PLATFORM_VIEW_WARM_UP_EVENT` on Android), which Drift posts once at launch unless the app called `engine.DisableViewWarmUp`.

In Go, implement `platform.PlatformView` (embed `platform.PlatformViewBase` for its ID and type), register a `platform.PlatformViewFactory` for the same type, and call the view with `GetPlatformViewRegistry().InvokeViewMethod`. Events arrive in the view's `HandleEvent(method, args)`; return `platform.ErrMethodNotFound` for one it does not know. A widget embeds the view by implementing `layout.PlatformViewOwner` and calling `ctx.EmbedPlatformView` while painting; [`plugins/media`](#media) is a complete example.

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

For events from native code, pick the channel kind by what a late subscriber should see:

| Constructor | A subscriber that joins late gets | Use for |
|-------------|-----------------------------------|---------|
| `platform.NewEventChannel(name)` | Only later events | Live updates |
| `platform.NewStickyEventChannel(name)` | The most recent event, then later ones | One-shot signals, such as `first_frame` |
| `platform.NewQueuedEventChannel(name, capacity)` | Events sent while nobody listened, in order, once | Events that must each be handled, such as the tap that launched the app |

Native code is not told when Go subscribes or cancels; it sends events whenever it has them. Start and stop native work with method calls, as the location plugin's `StartUpdates` and `StopUpdates` do.

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

- `platform.SetNativeBridge(fake)` with a `platform.NativeBridge` implementation captures method calls and answers them through the `reply` callback, in place or later from another goroutine (as an async method would).
- `platform.HandleEvent(channel, data)` delivers an event as native code would; encode `data` with `platform.DefaultCodec.Encode`.
- Call `platform.ResetForTest()` in `t.Cleanup` to clear subscriptions, sticky values and queues between tests.

### Publishing a plugin

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
| `Android.Manifest.AddPermission(name)` / `AddPermissionMaxSDK(name, maxSDK)` | `android.manifest.add_permission` | `<uses-permission>`, optionally with `android:maxSdkVersion` | Set |
| `Android.Manifest.AddIntentFilter(activity, xml)` | `android.manifest.add_intent_filter` | `<intent-filter>` in an activity | Set |
| `Android.Manifest.SetActivityAttr(activity, attr, value)` / `SetActivityTheme(activity, theme)` | `android.manifest.set_activity_attr` | Activity attribute, such as `android:theme` | Same location |
| `Android.Manifest.AddMetaData(parent, name, value)` | `android.manifest.add_meta_data` | `<meta-data>` in `application` or `activity:<name>` | Same location |
| `Android.Manifest.AddService(xml)` | `android.manifest.add_service` | `<service>` in `<application>`, named by its fully qualified class | Same location |
| `Android.Manifest.AddReceiver(xml)` | `android.manifest.add_receiver` | `<receiver>` in `<application>`, named by its fully qualified class | Same location |
| `Android.Resources.Colors.Set(name, hex)` | `android.color.set` | `@color/<name>`, from a Drift hex colour | Same location |
| `Android.Resources.Strings.Set(name, value)` | `android.string.set` | `@string/<name>`, from literal text (Drift escapes it for Android) | Same location |
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
