---
id: platform
title: Platform Services
sidebar_position: 1
---

# Platform Services

Drift provides access to native platform capabilities through the `platform` package.

## Context conventions

Most platform methods are simple native invocations or native-owned waits where
Drift does not expose cancellation.

`ctx` is reserved for APIs that wait for the user or for file I/O, so ctx
cancellation unblocks the Go caller:

- `Permission.Request(ctx)` and `NotificationPermission.RequestWithOptions(ctx)`
- `Storage.PickFile(ctx, ...)`, `Storage.PickDirectory(ctx)`, `Storage.SaveFile(ctx, ...)`
- `Storage.ReadFile`, `WriteFile`, `DeleteFile` and `GetFileInfo`
- `ShowDatePicker(ctx, ...)` and `ShowTimePicker(ctx, ...)`

When ctx fires on these methods, the Go caller is unblocked promptly with the
package's stable error sentinels (`ErrCanceled` / `ErrTimeout` for permissions,
`ctx.Err()` for the others). The native operation continues to completion
in the background and its result is discarded.

These calls wait, and the reply needs the platform main thread.
Go's UI thread (widget callbacks, `Build`, `InitState`) **is** that main
thread, so call them from a goroutine and publish the result with
`drift.Dispatch` or a `Signal`:

```go
go func() {
    result, err := platform.Notifications.Permission.Request(ctx)
    drift.Dispatch(func() { s.notificationsAllowed = err == nil && result == platform.PermissionGranted })
}()
```

From the UI thread they fail at once with `platform.ErrBlocksUIThread`, as
plugin methods that reply later do. The `DatePicker` and `TimePicker`
widgets call their pickers from a goroutine for you.

## Clipboard

Copy and paste text:

```go
import "github.com/go-drift/drift/pkg/platform"

// Copy text to clipboard
err := platform.Clipboard.SetText("Hello, World!")

// Get text from clipboard
text, err := platform.Clipboard.GetText()

// Check if clipboard has text
hasText, err := platform.Clipboard.HasText()

// Clear clipboard
err := platform.Clipboard.Clear()
```

### Example: Copy Button

```go
// In a Build method with access to ctx:
theme.ButtonOf(ctx, "Copy Link", func() {
    platform.Clipboard.SetText(shareURL)
    platform.Haptics.LightImpact()
    showToast("Link copied!")
})
```

## Haptic Feedback

Provide tactile feedback to users:

```go
// Light feedback for selections
platform.Haptics.LightImpact()

// Medium feedback for toggles and confirmations
platform.Haptics.MediumImpact()

// Heavy feedback for errors or warnings
platform.Haptics.HeavyImpact()

// Selection change feedback
platform.Haptics.SelectionClick()

// Custom vibration duration (milliseconds)
platform.Haptics.Vibrate(100)
```

### When to Use Haptics

| Feedback Type | Use Case |
|--------------|----------|
| `LightImpact` | List item selection, minor interactions |
| `MediumImpact` | Toggle switches, button taps |
| `HeavyImpact` | Errors, deletions, significant actions |
| `SelectionClick` | Picker value changes, slider movements |

## App Lifecycle

### App-Level Init and Dispose

Use `OnInit` and `OnDispose` on `drift.App` to run one-time setup and teardown at the application level, outside the widget tree. `OnInit` runs in a background goroutine before the root widget is mounted. While it executes, the platform's native splash screen stays visible.

```go
app := drift.NewApp(root)
app.OnInit = func(ctx context.Context) error {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return err
    }
    appDB = db
    return nil
}
app.OnDispose = func() {
    appDB.Close()
}
app.Run()
```

If `OnInit` returns an error, a debug error screen is shown instead of the root widget. The user can tap "Restart" to retry with the normal root widget (OnInit does not re-run on restart).

The `ctx` passed to `OnInit` is cancelled before `OnDispose` runs, so long-running initialization can select on `ctx.Done()` for clean shutdown:

```go
app.OnInit = func(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case result := <-loadConfig():
        return result.Err
    }
}
```

:::tip When to use OnInit vs InitState
Use `OnInit` for work that must complete before any widget mounts: opening databases, loading configuration, restoring authentication tokens. Use `InitState` in a stateful widget for setup that belongs to a specific screen or component.
:::

### Widget-Level Lifecycle Observation

Respond to app lifecycle state changes using `UseLifecycleObserver`. The handler is automatically cleaned up when the state is disposed, and always runs on the UI thread via `Dispatch`:

```go
func (s *myState) InitState() {
    platform.UseLifecycleObserver(s, func(state platform.LifecycleState) {
        switch state {
        case platform.LifecycleStateResumed:
            // App came to foreground
            s.refreshData()
        case platform.LifecycleStatePaused:
            // App went to background
            s.saveState()
        case platform.LifecycleStateInactive:
            // App is transitioning (e.g., receiving a phone call)
        case platform.LifecycleStateDetached:
            // App is detached from any view
        }
    })
}
```

For advanced use cases (e.g., subscribing outside a stateful widget), you can use `AddHandler` and `OnDispose` manually:

```go
removeHandler := platform.Lifecycle.AddHandler(func(state platform.LifecycleState) {
    // handle state change
})
s.OnDispose(removeHandler)
```

### Lifecycle States

| State | Description |
|-------|-------------|
| `LifecycleStateResumed` | App is visible and responding to user input |
| `LifecycleStateInactive` | App is transitioning (system dialog, app switcher) |
| `LifecycleStatePaused` | App is not visible but still running |
| `LifecycleStateDetached` | App is detached from any view |

### Checking Current State

```go
// Get current state
currentState := platform.Lifecycle.State()

// Convenience methods
if platform.Lifecycle.IsResumed() {
    // App is active
}

if platform.Lifecycle.IsPaused() {
    // App is in background
}
```

## System UI

Drift draws edge to edge on both platforms: content extends behind the status bar and navigation bar, so inset it with `SafeArea`. Choose the status bar's visibility and icon colour to suit the content behind it:

```go
platform.SetSystemUI(platform.SystemUIStyle{
    StatusBarHidden: false,
    // StatusBarStyleDefault follows the system theme; StatusBarStyleLight
    // (light icons) suits dark content, StatusBarStyleDark light content.
    StatusBarStyle: platform.StatusBarStyleDark,
})
```

The style persists until the next call, including across Android Activity recreation (dark mode or locale changes).

## Permissions

Permissions are attached to the features that use them. Each feature service provides a `Permission` field for checking and requesting access.

Drift owns the request mechanism; the plugin providing a feature registers its permission kind and declares what the platform needs (Android manifest permissions, iOS usage strings). A plugin's runtime package exposes its permissions, built with `platform.NewPermission(kind)`. Calling a permission whose plugin is missing from `drift.yaml` fails with `platform.ErrPermissionNotRegistered`; on Android, one whose manifest permission no plugin declares fails with `platform.ErrPermissionNotDeclared`.

One prompt per permission shows at a time: a `Request` made while that permission's prompt is up waits for it and returns the same answer.

### Notification Permissions

Drift provides the notification permission, which the notifications and Firebase plugins share; on Android those plugins declare `POST_NOTIFICATIONS`, and without one of them the permission fails with `platform.ErrPermissionNotDeclared`. It supports iOS-specific options via `RequestWithOptions`:

```go
ctx := context.Background()

// Request with default options (Alert, Sound, Badge)
result, err := platform.Notifications.Permission.Request(ctx)

// Request with specific options
result, err := platform.Notifications.Permission.RequestWithOptions(ctx,
    platform.NotificationPermissionOptions{
        Alert:       true,
        Sound:       true,
        Badge:       true,
        Provisional: false, // iOS provisional notifications
    },
)
```

A `RequestWithOptions` made while a notification prompt is up returns that prompt's answer; its own options are not used.

### Plugin Permissions

Permissions for features that live in plugins come from those plugins: location from the [location plugin](/docs/guides/plugins#location), and contacts, calendar, microphone and photos from the [permissions plugin](/docs/guides/plugins#permissions). They are `platform.Permission` values like the ones above.

Up to Drift v0.26, these permissions were fields of core services. Each now comes with its plugin in `drift.yaml`:

| Up to v0.26 | Now |
|-------------|-----|
| `platform.Location.Permission.WhenInUse`, `.Always` | `location.Permission.WhenInUse`, `location.Permission.Always` (`github.com/go-drift/drift/plugins/location/runtime`); `Always` needs the plugin's `always` config |
| `platform.Contacts.Permission`, `platform.Calendar.Permission`, `platform.Microphone.Permission`, `platform.Photos.Permission` | `permissions.Contacts`, `permissions.Calendar`, `permissions.Microphone`, `permissions.Photos` (`github.com/go-drift/drift/plugins/permissions/runtime`), each with its section in the plugin's config |
| `platform.Camera.Permission` | None: `imagepicker.Capture` asks for camera access itself where the platform needs it, and fails with `imagepicker.ErrPermissionDenied` when denied |

### Open App Settings

```go
// Open app settings for manual permission management
platform.OpenAppSettings()
```

### Permission Results

| Result | Meaning |
|--------|---------|
| `PermissionGranted` | Permission was granted |
| `PermissionDenied` | Permission was denied (the app may request again) |
| `PermissionPermanentlyDenied` | User selected "Don't ask again" |
| `PermissionRestricted` | Restricted by device policy |
| `PermissionLimited` | Partial access: selected photos (iOS), approximate location (Android) |
| `PermissionProvisional` | Provisional access (iOS notifications) |
| `PermissionNotDetermined` | Permission has not been requested yet |
| `PermissionResultUnknown` | Permission status could not be determined |

## Camera

Taking photos and picking images is a plugin: see [Image picker](/docs/guides/plugins#image-picker) in the plugins guide.

Up to Drift v0.26 this was `platform.Camera`. Apps moving to the plugin use its runtime (`imagepicker`, `github.com/go-drift/drift/plugins/imagepicker/runtime`):

| Up to v0.26 | Now |
|-------------|-----|
| `CapturePhoto(ctx, platform.CapturePhotoOptions{UseFrontCamera: ...})` | `imagepicker.Capture(ctx, imagepicker.CaptureOptions{FrontCamera: ...})`, from a goroutine: the `*imagepicker.Image` taken, or nil if the user canceled |
| `PickFromGallery(ctx, platform.PickFromGalleryOptions{AllowMultiple: ...})` | `imagepicker.Pick(ctx, imagepicker.PickOptions{Multiple: ...})`, from a goroutine: the `[]imagepicker.Image` picked, empty if the user canceled; iOS also picks several |
| `CameraResult` (`Media`, `MediaList`, `Cancelled`, `Error`) | The return values above; failures are the returned error |
| `CapturedMedia` | `imagepicker.Image`, with the same fields; `Width` and `Height` are as displayed. Decode the file with `image.Decode` from `github.com/go-drift/drift/pkg/image` |
| `ErrCameraBusy` | `imagepicker.ErrBusy` |
| `Permission` | See [Plugin Permissions](#plugin-permissions) |

## Location

Location is a plugin: see [Location](/docs/guides/plugins#location) in the plugins guide.

Up to Drift v0.26 this was `platform.Location`. Apps moving to the plugin use its runtime (`location`, `github.com/go-drift/drift/plugins/location/runtime`):

| Up to v0.26 | Now |
|-------------|-----|
| `GetCurrent(platform.LocationOptions{...})` | `location.Current(ctx, location.Options{...})`, from a goroutine; returns a `location.Update`, not a pointer |
| `LastKnown()`, `IsEnabled()` | `location.LastKnown(ctx)`, `location.IsEnabled(ctx)`, from a goroutine |
| `StartUpdates(platform.LocationOptions{...})` | `location.SetUpdateOptions(location.UpdateOptions{Options: location.Options{...}})`, then listen to `location.Updates()`: updates run while it has listeners; `Background: true` keeps them coming in the background |
| `StopUpdates()`, `Updates()` | Unsubscribe the last listener of `location.Updates()` |
| `LocationUpdate`, `LocationOptions` | `location.Update`, `location.Options`, with the same fields |
| `Permission.WhenInUse`, `Permission.Always` | See [Plugin Permissions](#plugin-permissions) |

## Notifications

Local notifications are a plugin: see [Notifications](/docs/guides/plugins#notifications) in the plugins guide; push notifications come from the [Firebase plugin](/docs/guides/plugins#firebase). Both use Drift's [notification permission](#notification-permissions).

Up to Drift v0.26, `platform.Notifications` also scheduled notifications and handled push. Apps moving to the plugins replace those calls with the notifications runtime (`notifications`, `github.com/go-drift/drift/plugins/notifications/runtime`) and Firebase messaging (`messaging`, `github.com/go-drift/drift/plugins/firebase/runtime/messaging`):

| Up to v0.26 | Now |
|-------------|-----|
| `platform.Notifications.Permission` | Unchanged |
| `Schedule(platform.NotificationRequest{...})` | `notifications.Schedule(ctx, notifications.Request{...})`, from a goroutine; `IntervalSeconds` is now `Interval`, a `time.Duration` |
| `Cancel(id)`, `CancelAll()`, `SetBadge(n)` | `notifications.Cancel(id)`, `notifications.CancelAll()`, `notifications.SetBadge(n)` |
| `Settings()` | `notifications.CurrentSettings(ctx)`, from a goroutine; the permission's status is `platform.Notifications.Permission.Status()` |
| `Deliveries()`, `Opens()` | Local notifications: `notifications.Deliveries()`, `notifications.Opens()`. Push: `messaging.Messages()`, `messaging.Opens()` |
| `RegisterForPush()` | Not needed: the Firebase plugin registers at launch |
| `Tokens()`, `GetPushToken()` | `messaging.Token()`, a `*core.Derived[string]`: `Value()` reads it, `AddListener` follows changes |
| `SubscribeToTopic(topic)`, `UnsubscribeFromTopic(topic)` | `messaging.SubscribeToTopic(ctx, topic)`, `messaging.UnsubscribeFromTopic(ctx, topic)`, from a goroutine |
| `DeletePushToken()` | `messaging.DeleteToken(ctx)`, from a goroutine |
| `Errors()` | Each call returns its own error; push registration and token failures are logged natively |
| `NotificationRequest`, `NotificationEvent`, `NotificationOpen`, `NotificationSettings` | `notifications.Request`, `notifications.Notification`, `notifications.Open`, `notifications.Settings`; push messages and taps are `messaging.Message` |

## Background Tasks

Up to Drift v0.26, `platform.Background` scheduled background tasks (`Schedule`, `Cancel`, `CancelAll`, `CancelByTag`, `Complete`, `IsRefreshAvailable`, `Events`). It is removed, with no replacement: it never ran the app's Go code in the background.

## Share

Open the native share sheet:

```go
// Share text
result, err := platform.Share.ShareText("Check out this app!")

// Share text with subject (for email)
result, err := platform.Share.ShareTextWithSubject("Check out this!", "App Recommendation")

// Share a URL
result, err := platform.Share.ShareURL("https://example.com")

// Share URL with text
result, err := platform.Share.ShareURLWithText("https://example.com", "Check out this link!")

// Share a file
result, err := platform.Share.ShareFile("/path/to/file.pdf", "application/pdf")
```

## URL Launcher

Open URLs in the system browser or check whether the device can handle a given URL scheme:

```go
import "github.com/go-drift/drift/pkg/platform"

// Open a URL in the system browser
err := platform.URLLauncher.OpenURL("https://example.com")

// Check if the system can open a URL
canOpen, err := platform.URLLauncher.CanOpenURL("https://example.com")

// Open other URL schemes
platform.URLLauncher.OpenURL("mailto:hello@example.com")
platform.URLLauncher.OpenURL("tel:+1234567890")
```

### Example: External Link Button

```go
theme.ButtonOf(ctx, "Visit Website", func() {
    if err := platform.URLLauncher.OpenURL("https://example.com"); err != nil {
        showToast("Could not open link")
    }
})
```

### Supported Schemes

Both platform templates include `http`, `https`, `mailto`, `tel`, and `sms` by default. To query or open custom URL schemes, update the platform manifests:

- **iOS**: Add schemes to `LSApplicationQueriesSchemes` in Info.plist
- **Android**: Add schemes to the `<queries>` block in AndroidManifest.xml

:::note
`CanOpenURL` only reports schemes declared in the app's manifest. URLs with undeclared schemes return `false` even if another app on the device can handle them.
:::

## File Storage

Access files and directories. Paths may be content URIs on Android (what the
pickers return). Every call except `GetAppDirectory` waits for file I/O off
the main thread, so call them from a goroutine:

```go
go func() {
    // Read a file
    data, err := platform.Storage.ReadFile(ctx, "/path/to/file.txt")

    // Write a file
    err = platform.Storage.WriteFile(ctx, "/path/to/file.txt", []byte("content"))

    // Delete a file
    err = platform.Storage.DeleteFile(ctx, "/path/to/file.txt")

    // Get file info
    info, err := platform.Storage.GetFileInfo(ctx, "/path/to/file.txt")
}()

// Get app directory (does not wait; any thread)
docsPath, err := platform.Storage.GetAppDirectory(platform.AppDirectoryDocuments)
cachePath, err := platform.Storage.GetAppDirectory(platform.AppDirectoryCache)
```

### File Picker

The pickers return nothing (no files, an empty path, no error) when the user
cancels, and `platform.ErrStorageBusy` if a picker is already open.

```go
go func() {
    files, err := platform.Storage.PickFile(context.Background(), platform.PickFileOptions{
        AllowMultiple: false,
        AllowedTypes:  []string{"image/*", "application/pdf"},
    })
    drift.Dispatch(func() {
        if err != nil {
            handleError(err)
            return
        }
        for _, file := range files { // none if canceled
            handleSelectedFile(file)
        }
    })
}()
```

`PickDirectory(ctx)` returns the chosen directory and `SaveFile(ctx, data, opts)`
the saved file, each `""` if canceled.

## Preferences

Store simple, unencrypted key-value data using platform-native storage (UserDefaults on iOS, SharedPreferences on Android). For sensitive data, use [secure storage](#secure-storage) instead.

```go
import "github.com/go-drift/drift/pkg/platform"

// Store a value
err := platform.Preferences.Set("username", "alice")

// Retrieve a value
username, err := platform.Preferences.Get("username")

// Check if a key exists (useful to distinguish missing keys from empty values)
exists, err := platform.Preferences.Contains("username")

// Delete a value
err = platform.Preferences.Delete("username")

// List all keys
keys, err := platform.Preferences.GetAllKeys()

// Delete all stored values
err = platform.Preferences.DeleteAll()
```

## Secure Storage

Encrypted storage, with optional biometric protection, is a plugin: see [Secure storage](/docs/guides/plugins#secure-storage) in the plugins guide. For values that need no protection, use [Preferences](#preferences).

Up to Drift v0.26 this was `platform.SecureStorage`. Apps moving to the plugin use its runtime (`securestorage`, `github.com/go-drift/drift/plugins/securestorage/runtime`). Every call now takes a `ctx` and waits for the platform, so call them from a goroutine:

| Up to v0.26 | Now |
|-------------|-----|
| `Set(key, value, opts)` | `securestorage.Set(ctx, key, value, opts)` |
| `Get(key, opts)`: `""` for a missing key | `securestorage.Get(ctx, key, opts)`: the value and whether there is one |
| `Delete`, `Contains`, `DeleteAll` | `securestorage.Delete`, `securestorage.Contains`, `securestorage.DeleteAll`, each with `ctx` first |
| `GetAllKeys(opts)` | `securestorage.Keys(ctx, opts)` |
| `GetBiometricType()`, `IsBiometricAvailable()` | `securestorage.AvailableBiometric(ctx)`; none available is `securestorage.BiometricNone` |
| `ErrAuthPending`, `Listen()` | Removed: `Get` waits for the biometric prompt and returns its result |
| `SecureStorageError` codes `auth_failed`, `auth_cancelled`, `biometric_not_available`, `biometric_not_enrolled` | `securestorage.ErrAuthFailed`, `securestorage.ErrAuthCanceled`, `securestorage.ErrBiometricsUnavailable` (both of the last two), under `errors.Is` |
| `ErrPlatformNotSupported` | Removed: every Android version Drift supports has secure storage |
| `SecureStorageOptions{KeychainAccessibility, RequireBiometric, BiometricPrompt, Service}` | `securestorage.Options{Accessibility, RequireBiometric, BiometricPrompt, Service}`; `RequireBiometric` needs the plugin's `biometrics` config |
| `KeychainAccessibility` (`AccessibleWhenUnlocked`, ...) | `securestorage.Accessibility` (`securestorage.WhenUnlocked`, `AfterFirstUnlock`, `WhenUnlockedThisDeviceOnly`, `AfterFirstUnlockThisDeviceOnly`) |
| `BiometricType` (`BiometricTypeFaceID`, ...) | `securestorage.Biometric` (`securestorage.BiometricFaceID`, `BiometricTouchID`, `BiometricFingerprint`, `BiometricFace`, `BiometricNone`; also `BiometricOpticID`) |

## Thread Safety

Platform services are safe to call from any goroutine, but calls that wait for the user must not run on the UI thread (see [Context conventions](#context-conventions)). When updating UI state from platform callbacks, use `drift.Dispatch`:

```go
unsubscribe := platform.DeepLinks.Links().Listen(platform.StreamHandler[platform.DeepLink]{OnEvent: func(link platform.DeepLink) {
    // Called from background goroutine
    drift.Dispatch(func() {
        // Now safe to update UI
        s.lastLink = link
    })
}})
defer unsubscribe()
```

## Next Steps

- [Skia](/docs/guides/skia) - Building Skia from source
- [API Reference](/docs/api/platform) - Platform API documentation
