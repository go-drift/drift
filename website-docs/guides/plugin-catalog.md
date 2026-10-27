---
id: plugin-catalog
title: Plugin Catalog
sidebar_position: 9
---

# Plugin Catalog

Drift's first-party plugins ship in the `github.com/go-drift/drift` module, at the same version as the rest of Drift, so an app needs no `go get` for them: [add one](/docs/guides/plugins#adding-a-plugin) to `drift.yaml` with its config, and import its runtime package. Each section shows the config and the Go API.

## Splash

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

## Firebase

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
messaging.Messages().Listen(platform.StreamHandler[messaging.Message]{OnEvent: func(m messaging.Message) {
    drift.Dispatch(func() { showBanner(m) })
}})

// A notification the user tapped, including the one that launched the app.
messaging.Opens().Listen(platform.StreamHandler[messaging.Message]{OnEvent: func(m messaging.Message) {
    drift.Dispatch(func() { navigate(m.Data["route"]) })
}})
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

## Location

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

// Updates run while the stream has listeners.
err := location.SetUpdateOptions(location.UpdateOptions{
    Options: location.Options{HighAccuracy: true, DistanceFilter: 10},
})
unsubscribe := location.Updates().Listen(platform.StreamHandler[location.Update]{
    OnEvent: func(u location.Update) {
        drift.Dispatch(func() { moveMarker(u) })
    },
    // A failed start, or the user revoking the permission mid-stream.
    OnError: func(err error) {
        if errors.Is(err, location.ErrPermissionDenied) {
            drift.Dispatch(showPermissionPrompt)
        }
    },
})
// Later: the last unsubscribe stops the platform's updates.
unsubscribe()
```

| Function | Waits | Notes |
|----------|-------|-------|
| `Permission.WhenInUse`, `Permission.Always` | `Request` does | `platform.Permission`s; approximate-only access on Android reads `PermissionLimited` |
| `Current(ctx, opts)` | yes | A fresh fix |
| `LastKnown(ctx)` | yes | The cached fix, or nil |
| `IsEnabled(ctx)` | yes | Whether location services are on |
| `Updates()` | no | A stream of fixes. The platform's updates start with its first listener and stop when the last unsubscribes, so no listener leaves the GPS off and none is left waiting on a stopped stream. A failed start and a failure mid-stream reach `OnError` |
| `SetUpdateOptions(opts)` | no | The stream's options, one setting for the app: a running stream changes at once, a later one starts with them. A running stream that has lost access fails it with `ErrPermissionDenied` or `ErrDisabled`. A failed call leaves the options and the stream as they were |

Functions that wait must be called from a goroutine; from the UI thread they fail with `platform.ErrBlocksUIThread`. Failures match `location.ErrPermissionDenied`, `location.ErrDisabled` (location services off) and `location.ErrUnavailable` (no fix) under `errors.Is`.

On iOS, Always can be requested only after WhenInUse is granted, and iOS asks at most once: once asked, Always reads `PermissionPermanentlyDenied` until the user grants it in Settings. On Android, Always is granted in Settings, where the request takes the user. Supported on iOS, xtool and Android (Google Play services).

### Background updates

With `Background: true`, the stream keeps delivering updates to the app's Go code while the app is in the background:

```go
err := location.SetUpdateOptions(location.UpdateOptions{
    Options:    location.Options{HighAccuracy: true, DistanceFilter: 10},
    Background: true,
})
```

It fails instead of quietly running only in the foreground: `SetUpdateOptions` with `location.ErrBackgroundNotConfigured` without the `always` config and with `location.ErrAlwaysDenied` until the user grants `Permission.Always`; on Android, a background stream starting while the app is not in the foreground fails with `location.ErrBackgroundStart` (from `SetUpdateOptions` on a running stream, otherwise on the listener's `OnError`), since Android 12+ starts the foreground service only from there. Start it while the app is on screen, for example from the button that starts a run. Changing a running background stream's options from the background is fine; unsubscribing the last listener ends it.

Updates reach Go, so they last only as long as the app's process:

| | While the stream is on | Ends when |
|---|---|---|
| iOS | The app keeps running in the background, with the blue location indicator in the status bar; updates do not pause automatically | The user force-quits the app (swipes it away in the app switcher) or the system terminates it; iOS does not relaunch the app for these updates |
| Android | A foreground service keeps the process alive, showing the ongoing notification from the `always` config; a tap opens the app | The process is killed (Force stop, Stop in the Task Manager, low memory); the service is not restarted. Swiping the app away from Recents usually leaves it running, though some manufacturers' builds kill it |

## Permissions

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

## Notifications

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
notifications.Opens().Listen(platform.StreamHandler[notifications.Open]{OnEvent: func(o notifications.Open) {
    drift.Dispatch(func() { navigate(o.Data["route"]) })
}})
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

- Android schedules with `AlarmManager`: exactly when the user lets the app schedule exact alarms (Settings, Alarms & reminders; the plugin declares `SCHEDULE_EXACT_ALARM`, which lists the app there), otherwise within Android's window for inexact alarms. Scheduled notifications are lost when the device restarts.
- Android: when the system groups several notifications from the app, tapping the group summary opens the app without an `Opens()` event, as for Firebase.
- `Deliveries()` reports notifications shown while the app runs; one that fires while the app is closed reaches only `Opens()`, if tapped.

Supported on iOS, xtool and Android.

## Image picker

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

`Capture` and `Pick` wait for the user, so call them from a goroutine. Each `Image` is a file in the app's cache directory (`Path`, `MimeType`, `Width`, `Height`, `Size`); move it to keep it. The file is always in a format Drift's `image.Decode` reads: iOS converts every image to JPEG, and Android converts a pick in any other format (HEIC, AVIF) to JPEG. Decode it with Drift's `image.Decode` (`github.com/go-drift/drift/pkg/image`), not the standard library's: photos keep the pixels as the camera stored them plus an EXIF orientation, which Drift's `Decode` applies so the image comes out upright. `Width` and `Height` are that upright size on both platforms. Failures match `imagepicker.ErrBusy` (a picker is open), `ErrCameraUnavailable` (no camera, such as the iOS Simulator), `ErrCameraNotConfigured` (iOS without `camera:`) and `ErrPermissionDenied` under `errors.Is`.

On Android the camera writes through Drift's `FileProvider`. If another plugin declares `CAMERA`, Android requires that permission for the camera app too, so `Capture` asks for it first. `FrontCamera` is a hint some Android camera apps ignore. Supported on iOS, xtool and Android.

## Secure storage

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

## Media

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
s.video, err = media.NewVideoPlayerController()
...
core.UseDisposable(s, s.video)
core.UseListenable(s, s.video.State()) // rebuild on playback state changes
s.video.Load(url)

// In Build:
media.VideoPlayer{Controller: s.video, Height: 225}
```

Controllers publish `State()` and `Progress()` as `*core.Derived` values that change on the UI thread, and playback failures on the `Errors()` stream (`media.ErrSource`, `ErrDecoder`, `ErrPlaybackFailed`). `media.NewAudioPlayerController()` has the same methods without a view; several can play at once. The [Media Player](/docs/guides/media-player) guide covers both. Playback stops when the app goes to the background. Supported on iOS, xtool and Android.

## Next Steps

- [Plugins](/docs/guides/plugins) - Adding, configuring and removing plugins
- [Media Player](/docs/guides/media-player) - Video and audio with the media plugin
- [Authoring Plugins](/docs/guides/plugin-authoring) - Writing your own
