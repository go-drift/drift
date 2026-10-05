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

`ctx` is reserved for APIs that wait for the user, so ctx cancellation
unblocks the Go caller:

- `Permission.Request(ctx)` and `NotificationPermission.RequestWithOptions(ctx)`
- `Storage.PickFile(ctx, ...)`, `Storage.PickDirectory(ctx)`, `Storage.SaveFile(ctx, ...)`

When ctx fires on these methods, the Go caller is unblocked promptly with the
package's stable error sentinels (`ErrCanceled` / `ErrTimeout` for permissions,
`ctx.Err()` for storage). The native operation continues to completion
in the background and its result is discarded.

These calls wait for the user, and the reply needs the platform main thread.
Go's UI thread (widget callbacks, `Build`, `InitState`) **is** that main
thread, so call them from a goroutine and publish the result with
`drift.Dispatch` or a `Signal`:

```go
go func() {
    result, err := platform.Notifications.Permission.Request(ctx)
    drift.Dispatch(func() { s.notificationsAllowed = err == nil && result == platform.PermissionGranted })
}()
```

From the UI thread, `Request` and `RequestWithOptions` fail at once with
`platform.ErrBlocksUIThread`, as plugin methods that reply later do. The
storage pickers and the date and time pickers still freeze the app when
called from the UI thread.

## Permissions

Permissions are attached to the features that use them. Each feature service provides a `Permission` field for checking and requesting access.

Drift owns the request mechanism; the plugin providing a feature registers its permission kind and declares what the platform needs (Android manifest permissions, iOS usage strings). A plugin's runtime package exposes its permissions, built with `platform.NewPermission(kind)`. Calling a permission whose plugin is missing from `drift.yaml` fails with `platform.ErrPermissionNotRegistered`; on Android, one whose manifest permission no plugin declares fails with `platform.ErrPermissionNotDeclared`.

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

### Plugin Permissions

Permissions for features that live in plugins come from those plugins: location from the [location plugin](/docs/guides/plugins#location), and contacts, calendar, microphone and photos from the [permissions plugin](/docs/guides/plugins#permissions). They are `platform.Permission` values like the ones above.

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

## Location

Location is a plugin: see [Location](/docs/guides/plugins#location) in the plugins guide.

## Notifications

Local notifications are a plugin: see [Notifications](/docs/guides/plugins#notifications) in the plugins guide; push notifications come from the [Firebase plugin](/docs/guides/plugins#firebase). Both use Drift's [notification permission](#notification-permissions).

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

Access files and directories:

```go
// Read a file
data, err := platform.Storage.ReadFile("/path/to/file.txt")

// Write a file
err := platform.Storage.WriteFile("/path/to/file.txt", []byte("content"))

// Delete a file
err := platform.Storage.DeleteFile("/path/to/file.txt")

// Get file info
info, err := platform.Storage.GetFileInfo("/path/to/file.txt")

// Get app directory
docsPath, err := platform.Storage.GetAppDirectory(platform.AppDirectoryDocuments)
cachePath, err := platform.Storage.GetAppDirectory(platform.AppDirectoryCache)
```

### File Picker

```go
// Open file picker (runs synchronously, call from a goroutine)
go func() {
    result, err := platform.Storage.PickFile(context.Background(), platform.PickFileOptions{
        AllowMultiple: false,
        AllowedTypes:  []string{"image/*", "application/pdf"},
    })
    drift.Dispatch(func() {
        if err != nil {
            handleError(err)
            return
        }
        if result.Cancelled {
            return
        }
        for _, file := range result.Files {
            handleSelectedFile(file)
        }
    })
}()
```

## Preferences

Store simple, unencrypted key-value data using platform-native storage (UserDefaults on iOS, SharedPreferences on Android). For sensitive data, use SecureStorage instead.

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

## Thread Safety

Platform services are safe to call from any goroutine, but calls that wait for the user must not run on the UI thread (see [Context conventions](#context-conventions)). When updating UI state from platform callbacks, use `drift.Dispatch`:

```go
unsubscribe := platform.DeepLinks.Links().Listen(func(link platform.DeepLink) {
    // Called from background goroutine
    drift.Dispatch(func() {
        // Now safe to update UI
        s.lastLink = link
    })
})
defer unsubscribe()
```

## Next Steps

- [Skia](/docs/guides/skia) - Building Skia from source
- [API Reference](/docs/api/platform) - Platform API documentation
