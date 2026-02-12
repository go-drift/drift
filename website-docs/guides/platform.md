---
id: platform
title: Platform Services
sidebar_position: 11
---

# Platform Services

Drift provides access to native platform capabilities through the `platform` package.

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

Respond to app lifecycle state changes:

```go
func (s *myState) InitState() {
    removeHandler := platform.Lifecycle.AddHandler(func(state platform.LifecycleState) {
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

    // Clean up when widget is disposed
    s.OnDispose(removeHandler)
}
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

Customize the status bar and system chrome:

```go
// Set system UI style
platform.SetSystemUI(platform.SystemUIStyle{
    StatusBarHidden: false,
    StatusBarStyle:  platform.StatusBarStyleLight, // or StatusBarStyleDark
    TitleBarHidden:  false,        // Android only
    Transparent:     false,        // Android only
    BackgroundColor: &colors.Surface, // Android only
})
```

## Permissions

Permissions are attached to the features that use them. Each feature service provides a `Permission` field for checking and requesting access.

### Camera Permission

```go
ctx := context.Background()

// Check status
status, err := platform.Camera.Permission.Status(ctx)

// Request permission
result, err := platform.Camera.Permission.Request(ctx)
if result == platform.PermissionGranted {
    // Camera is available
}

// Convenience checks
if platform.Camera.Permission.IsGranted(ctx) {
    // Camera is available
}

// Listen for permission changes
unsubscribe := platform.Camera.Permission.Listen(func(status platform.PermissionStatus) {
    drift.Dispatch(func() {
        updateUI(status)
    })
})
defer unsubscribe()
```

### Location Permissions

Location has two permission levels - when in use and always (background):

```go
ctx := context.Background()

// When-in-use location
status, err := platform.Location.Permission.WhenInUse.Status(ctx)
result, err := platform.Location.Permission.WhenInUse.Request(ctx)

// Background (always) location
// Note: On iOS, WhenInUse must be granted before requesting Always.
status, err := platform.Location.Permission.Always.Status(ctx)
result, err := platform.Location.Permission.Always.Request(ctx)

// Convenience check
if platform.Location.Permission.Always.IsGranted(ctx) {
    // Background location available
}

// Listen for permission changes
whenInUseUnsub := platform.Location.Permission.WhenInUse.Listen(func(status platform.PermissionStatus) {
    drift.Dispatch(func() { updateWhenInUseUI(status) })
})
alwaysUnsub := platform.Location.Permission.Always.Listen(func(status platform.PermissionStatus) {
    drift.Dispatch(func() { updateAlwaysUI(status) })
})
defer whenInUseUnsub()
defer alwaysUnsub()
```

### Notification Permissions

Notifications support iOS-specific options via `RequestWithOptions`:

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

### Other Permissions

For permissions without dedicated feature services:

```go
ctx := context.Background()

// Microphone
status, err := platform.Microphone.Permission.Status(ctx)
result, err := platform.Microphone.Permission.Request(ctx)

// Photos
status, err := platform.Photos.Permission.Status(ctx)
result, err := platform.Photos.Permission.Request(ctx)

// Contacts
status, err := platform.Contacts.Permission.Status(ctx)
result, err := platform.Contacts.Permission.Request(ctx)

// Calendar
status, err := platform.Calendar.Permission.Status(ctx)
result, err := platform.Calendar.Permission.Request(ctx)

// Storage
status, err := platform.StoragePermission.Permission.Status(ctx)
result, err := platform.StoragePermission.Permission.Request(ctx)
```

### Open App Settings

```go
// Open app settings for manual permission management
ctx := context.Background()
platform.OpenAppSettings(ctx)
```

### Permission Results

| Result | Meaning |
|--------|---------|
| `PermissionGranted` | Permission was granted |
| `PermissionDenied` | Permission was denied (the app may request again) |
| `PermissionPermanentlyDenied` | User selected "Don't ask again" |
| `PermissionRestricted` | Restricted by device policy |
| `PermissionLimited` | Limited access granted (iOS photos) |
| `PermissionProvisional` | Provisional access (iOS notifications) |
| `PermissionNotDetermined` | Permission has not been requested yet |
| `PermissionResultUnknown` | Permission status could not be determined |

## Camera

Capture photos and select images from the photo library using the synchronous API.
Methods block until the user completes or cancels the operation.

**Important:** Always use a context with a timeout. If the native layer fails to respond
and the context has no deadline, the call blocks forever and holds the camera mutex,
preventing further operations.

**Note:** Only one camera operation can run at a time. If another operation is in progress,
`CapturePhoto` and `PickFromGallery` return `platform.ErrCameraBusy`.

```go
import "github.com/go-drift/drift/pkg/platform"

// Request permission first
ctx := context.Background()
if !platform.Camera.Permission.IsGranted(ctx) {
    platform.Camera.Permission.Request(ctx)
}

// Capture a photo (blocks until complete or cancelled)
// Call from a goroutine, not the main/render thread
go func() {
    // Use timeout to prevent blocking forever if native layer fails
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    result, err := platform.Camera.CapturePhoto(ctx, platform.CapturePhotoOptions{})
    drift.Dispatch(func() {
        if err != nil {
            handleError(err)
            return
        }
        if result.Cancelled {
            return
        }
        handleMedia(result.Media)
    })
}()

// Capture a selfie with the front camera
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    result, err := platform.Camera.CapturePhoto(ctx, platform.CapturePhotoOptions{
        UseFrontCamera: true,
    })
    // ... handle result
}()

// Pick from gallery (blocks until complete)
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    result, err := platform.Camera.PickFromGallery(ctx, platform.PickFromGalleryOptions{})
    // ... handle result
}()

// Pick multiple images (Android only; iOS returns single image)
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    result, err := platform.Camera.PickFromGallery(ctx, platform.PickFromGalleryOptions{
        AllowMultiple: true,
    })
    // ... handle result.MediaList for multiple images
}()
```

### CapturedMedia Fields

| Field | Type | Description |
|-------|------|-------------|
| `Path` | `string` | Absolute path to the image file |
| `MimeType` | `string` | Media type (e.g., "image/jpeg") |
| `Width` | `int` | Image width in pixels |
| `Height` | `int` | Image height in pixels |
| `Size` | `int64` | File size in bytes |

### Example: Camera Page

```go
type cameraState struct {
    core.StateBase
    status           *core.ManagedState[string]
    image            *core.ManagedState[image.Image]
    permissionStatus *core.ManagedState[platform.PermissionStatus]
}

func (s *cameraState) InitState() {
    s.status = core.NewManagedState(&s.StateBase, "Tap to capture")
    s.image = core.NewManagedState[image.Image](&s.StateBase, nil)
    s.permissionStatus = core.NewManagedState(&s.StateBase, platform.PermissionNotDetermined)

    ctx := context.Background()

    // Check initial permission
    go func() {
        status, _ := platform.Camera.Permission.Status(ctx)
        drift.Dispatch(func() { s.permissionStatus.Set(status) })
    }()

    // Listen for permission changes
    unsub := platform.Camera.Permission.Listen(func(status platform.PermissionStatus) {
        drift.Dispatch(func() { s.permissionStatus.Set(status) })
    })
    s.OnDispose(unsub)
}

func (s *cameraState) takePhoto() {
    s.status.Set("Opening camera...")
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
        defer cancel()

        result, err := platform.Camera.CapturePhoto(ctx, platform.CapturePhotoOptions{})
        drift.Dispatch(func() {
            if err != nil {
                s.status.Set("Error: " + err.Error())
                return
            }
            if result.Cancelled {
                s.status.Set("Cancelled")
                return
            }
            if result.Media != nil {
                img, err := loadImage(result.Media.Path)
                if err != nil {
                    s.status.Set("Failed to load: " + err.Error())
                    return
                }
                s.image.Set(img)
                s.status.Set("Photo captured!")
            }
        })
    }()
}
```

:::note Platform Notes
- **iOS**: Multi-select (`AllowMultiple`) is not supported; only a single image is returned
- **iOS**: Camera availability is checked; an error is returned if no camera is present (e.g., on simulator)
- **Android**: The front camera hint (`UseFrontCamera`) is not guaranteed to be honored by all camera apps
- Captured images are saved to the app's temp directory as JPEGs
- Gallery selections are copied to temp files for reliable cross-process access
:::

## Location

Access device location using the Location service:

```go
ctx := context.Background()

// Request permission first
platform.Location.Permission.WhenInUse.Request(ctx)

// Get current location (synchronous)
loc, err := platform.Location.GetCurrent(ctx, platform.LocationOptions{
    HighAccuracy: true,
})
if err == nil {
    fmt.Printf("Lat: %f, Lng: %f\n", loc.Latitude, loc.Longitude)
}

// Start continuous location updates
platform.Location.StartUpdates(ctx, platform.LocationOptions{
    HighAccuracy:   true,
    DistanceFilter: 10, // meters
})

// Listen for updates using Stream
unsubscribe := platform.Location.Updates().Listen(func(update platform.LocationUpdate) {
    drift.Dispatch(func() {
        s.userLocation = &update
    })
})
defer unsubscribe()

// Stop updates when done
platform.Location.StopUpdates(ctx)

// Check if location services are enabled
enabled, err := platform.Location.IsEnabled(ctx)

// Get last known location without triggering a new request
lastKnown, err := platform.Location.LastKnown(ctx)
```

### Location Data

| Field | Type | Description |
|-------|------|-------------|
| `Latitude` | `float64` | Latitude in degrees |
| `Longitude` | `float64` | Longitude in degrees |
| `Altitude` | `float64` | Altitude in meters |
| `Accuracy` | `float64` | Accuracy in meters |
| `Heading` | `float64` | Direction in degrees |
| `Speed` | `float64` | Speed in m/s |
| `Timestamp` | `time.Time` | When reading was taken |

## Notifications

Manage local and push notifications using the Notifications service:

```go
ctx := context.Background()

// Request permission with options
status, err := platform.Notifications.Permission.RequestWithOptions(ctx,
    platform.NotificationPermissionOptions{Alert: true, Sound: true, Badge: true},
)

// Schedule a local notification
platform.Notifications.Schedule(ctx, platform.NotificationRequest{
    ID:    "reminder-1",
    Title: "Reminder",
    Body:  "Meeting in 5 minutes",
    At:    time.Now().Add(5 * time.Minute),
    Data:  map[string]any{"meetingId": "123"},
})

// Cancel a notification
platform.Notifications.Cancel(ctx, "reminder-1")

// Cancel all notifications
platform.Notifications.CancelAll(ctx)

// Set app badge count
platform.Notifications.SetBadge(ctx, 3)

// Get notification settings
settings, err := platform.Notifications.Settings(ctx)
```

### Listening for Notifications

```go
// Listen for delivered notifications
deliveriesUnsub := platform.Notifications.Deliveries().Listen(func(event platform.NotificationEvent) {
    drift.Dispatch(func() {
        handleNotification(event)
    })
})
defer deliveriesUnsub()

// Listen for notification opens (user tapped notification)
opensUnsub := platform.Notifications.Opens().Listen(func(open platform.NotificationOpen) {
    drift.Dispatch(func() {
        navigateToContent(open.Data)
    })
})
defer opensUnsub()

// Listen for device token updates (push notifications)
tokensUnsub := platform.Notifications.Tokens().Listen(func(token platform.DeviceToken) {
    drift.Dispatch(func() {
        sendTokenToServer(token.Token)
    })
})
defer tokensUnsub()
```

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
data, err := platform.ReadFile("/path/to/file.txt")

// Write a file
err := platform.WriteFile("/path/to/file.txt", []byte("content"))

// Delete a file
err := platform.DeleteFile("/path/to/file.txt")

// Get file info
info, err := platform.GetFileInfo("/path/to/file.txt")

// Get app directory
docsPath, err := platform.GetAppDirectory(platform.AppDirectoryDocuments)
cachePath, err := platform.GetAppDirectory(platform.AppDirectoryCache)
```

### File Picker

```go
// Open file picker
platform.PickFile(platform.PickFileOptions{
    AllowMultiple: false,
    AllowedTypes:  []string{"image/*", "application/pdf"},
})

// Listen for results
go func() {
    for result := range platform.StorageResults() {
        if result.Cancelled {
            continue
        }
        for _, file := range result.Files {
            handleSelectedFile(file)
        }
    }
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

Store sensitive data securely using platform-native encryption (iOS Keychain, Android EncryptedSharedPreferences):

```go
import "github.com/go-drift/drift/pkg/platform"

// Store a value securely
err := platform.SecureStorage.Set("auth_token", "secret123", nil)

// Retrieve a value
token, err := platform.SecureStorage.Get("auth_token", nil)

// Check if a key exists
exists, err := platform.SecureStorage.Contains("auth_token", nil)

// Delete a value
err := platform.SecureStorage.Delete("auth_token", nil)

// List all keys
keys, err := platform.SecureStorage.GetAllKeys(nil)

// Delete all stored values
err := platform.SecureStorage.DeleteAll(nil)
```

### Biometric Protection

Require Face ID, Touch ID, or fingerprint authentication to access values:

```go
// Store with biometric protection
err := platform.SecureStorage.Set("sensitive_key", "secret", &platform.SecureStorageOptions{
    RequireBiometric: true,
    BiometricPrompt:  "Authenticate to save your data",
})

// Retrieve biometric-protected value (prompts user)
value, err := platform.SecureStorage.Get("sensitive_key", &platform.SecureStorageOptions{
    BiometricPrompt: "Authenticate to access your data",
})
```

### Handling Async Biometric Auth (Android)

On Android, biometric operations are asynchronous. Check for `ErrAuthPending` and listen for results:

```go
func (s *myState) InitState() {
    // Listen for biometric auth results
    go func() {
        for event := range platform.SecureStorage.Listen() {
            drift.Dispatch(func() {
                if event.Success {
                    s.handleValue(event.Key, event.Value)
                } else {
                    s.showError("Authentication failed: " + event.Error)
                }
            })
        }
    }()
}

func (s *myState) loadSecret() {
    value, err := platform.SecureStorage.Get("my_key", nil)
    if err == platform.ErrAuthPending {
        s.showMessage("Authenticating...")
        return // Result will come via Listen()
    }
    if err != nil {
        s.showError(err.Error())
        return
    }
    s.handleValue("my_key", value)
}
```

### Check Biometric Availability

```go
// Check if biometrics are available
available, err := platform.SecureStorage.IsBiometricAvailable()

// Get biometric type
biometricType, err := platform.SecureStorage.GetBiometricType()
// Returns: BiometricTypeFaceID, BiometricTypeTouchID,
//          BiometricTypeFingerprint, BiometricTypeFace, or BiometricTypeNone
```

### Security Notes

| Platform | Encryption | Biometric Protection |
|----------|------------|---------------------|
| iOS | Keychain (hardware-backed) | Hardware-enforced via SecAccessControl |
| Android | EncryptedSharedPreferences (AES-256) | App-level UI gate (BiometricPrompt) |

:::note Android Biometric Limitation
On Android, biometric protection is an app-enforced policy, not cryptographically tied to key unlocking. Data is still encrypted at rest, but the biometric check is a UI gate rather than hardware-enforced per-operation verification.
:::

### Platform Support

Drift requires Android 12 (API 31) or later, which satisfies the secure storage minimum. On unsupported platforms, operations return `ErrPlatformNotSupported`:

```go
if err == platform.ErrPlatformNotSupported {
    // Fall back to less secure storage or show error
}
```

## Thread Safety

Platform services are safe to call from any goroutine. However, when updating UI state from platform callbacks, use `drift.Dispatch`:

```go
unsubscribe := platform.Location.Updates().Listen(func(update platform.LocationUpdate) {
    // Called from background goroutine
    drift.Dispatch(func() {
        // Now safe to update UI
        s.location = &update
    })
})
defer unsubscribe()
```

## Next Steps

- [Skia](/docs/guides/skia) - Building Skia from source
- [API Reference](/docs/api/platform) - Platform API documentation
