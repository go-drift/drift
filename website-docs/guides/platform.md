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

Request runtime permissions using the `Permissions` service:

```go
// Request a permission
result, err := platform.Permissions.Camera.Request()
if result == platform.PermissionGranted {
    openCamera()
}

// Check current permission status
status, err := platform.Permissions.Camera.Status()

// Convenience checks
if platform.Permissions.Camera.IsGranted() {
    // Camera is available
}

// Listen for permission changes
go func() {
    for result := range platform.Permissions.Camera.Changes() {
        drift.Dispatch(func() {
            updateUI(result)
        })
    }
}()

// Open app settings for manual permission management
platform.OpenAppSettings()
```

### Location Permissions

Location has two levels - when in use and always (background). Each level has its own
status, request, and change notification methods:

```go
// When-in-use location
result, err := platform.Permissions.Location.RequestWhenInUse()
status, err := platform.Permissions.Location.Status()

// Background (always) location
result, err := platform.Permissions.Location.RequestAlways()
status, err := platform.Permissions.Location.StatusAlways()
if platform.Permissions.Location.IsAlwaysGranted() {
    // Background location available
}

// Listen for permission changes
// IMPORTANT: Use Changes() for when-in-use, ChangesAlways() for background.
// These are separate event streams from the platform.
go func() {
    for result := range platform.Permissions.Location.Changes() {
        drift.Dispatch(func() { updateWhenInUseUI(result) })
    }
}()

go func() {
    for result := range platform.Permissions.Location.ChangesAlways() {
        drift.Dispatch(func() { updateAlwaysUI(result) })
    }
}()
```

### Notification Permissions

Notifications support iOS-specific options:

```go
// Request with default options (Alert, Sound, Badge)
result, err := platform.Permissions.Notification.Request()

// Request with specific options
result, err := platform.Permissions.Notification.Request(platform.NotificationOptions{
    Alert:       true,
    Sound:       true,
    Badge:       true,
    Provisional: false, // iOS provisional notifications
})
```

### Available Permissions

| Permission | Use |
|------------|-----|
| `Permissions.Camera` | Camera access |
| `Permissions.Microphone` | Microphone access |
| `Permissions.Location` | Location services |
| `Permissions.Storage` | File storage |
| `Permissions.Contacts` | Contacts access |
| `Permissions.Photos` | Photo library |
| `Permissions.Calendar` | Calendar access |
| `Permissions.Notification` | Push notifications |

### Permission Results

| Result | Meaning |
|--------|---------|
| `PermissionGranted` | Permission was granted |
| `PermissionDenied` | Permission was denied |
| `PermissionPermanentlyDenied` | User selected "Don't ask again" |
| `PermissionRestricted` | Restricted by device policy |
| `PermissionLimited` | Limited access granted (iOS photos) |
| `PermissionProvisional` | Provisional access (iOS notifications) |

## Notifications

Schedule local notifications:

```go
// Schedule a notification
err := platform.ScheduleLocalNotification(platform.NotificationRequest{
    ID:    "reminder-1",
    Title: "Reminder",
    Body:  "Meeting in 5 minutes",
    At:    time.Now().Add(5 * time.Minute),
    Data:  map[string]any{"meetingId": "123"},
})

// Cancel a notification
platform.CancelLocalNotification("reminder-1")

// Cancel all notifications
platform.CancelAllLocalNotifications()

// Set app badge count
platform.SetNotificationBadge(3)
```

### Listening for Notifications

```go
// Listen for delivered notifications
go func() {
    for event := range platform.Notifications() {
        drift.Dispatch(func() {
            handleNotification(event)
        })
    }
}()

// Listen for notification opens (user tapped notification)
go func() {
    for open := range platform.NotificationOpens() {
        drift.Dispatch(func() {
            navigateToContent(open.Data)
        })
    }
}()
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

## Location

Access device location:

```go
// Get current location
location, err := platform.GetCurrentLocation(platform.LocationOptions{
    HighAccuracy: true,
})
if err == nil {
    fmt.Printf("Lat: %f, Lng: %f\n", location.Latitude, location.Longitude)
}

// Start continuous location updates
platform.StartLocationUpdates(platform.LocationOptions{
    HighAccuracy:   true,
    DistanceFilter: 10, // meters
})

// Listen for updates
go func() {
    for update := range platform.LocationUpdates() {
        drift.Dispatch(func() {
            s.SetState(func() {
                s.userLocation = update
            })
        })
    }
}()

// Stop updates when done
platform.StopLocationUpdates()
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

Secure storage requires Android 6.0 (API 23) or higher. On older Android versions, operations return `ErrPlatformNotSupported`:

```go
if err == platform.ErrPlatformNotSupported {
    // Fall back to less secure storage or show error
}
```

## Thread Safety

Platform services are safe to call from any goroutine. However, when updating UI state from platform callbacks, use `drift.Dispatch`:

```go
go func() {
    for update := range platform.LocationUpdates() {
        // Called from background goroutine
        drift.Dispatch(func() {
            // Now safe to update UI
            s.SetState(func() {
                s.location = update
            })
        })
    }
}()
```

## Next Steps

- [Skia](/docs/guides/skia) - Building Skia from source
- [API Reference](/docs/api/platform) - Platform API documentation
