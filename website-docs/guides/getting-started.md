---
id: getting-started
title: Getting Started
sidebar_position: 1
---

# Getting Started

This guide walks you through installing Drift and running your first app.

## Prerequisites

### Go

Install **Go 1.24** or later from [go.dev](https://go.dev/dl/).

### Android

- Android SDK (via Android Studio or command-line tools)
- Android NDK
- Java 17+
- Target device running Android 12 (API 31) or later

Set these environment variables:

```bash
export ANDROID_HOME=/path/to/android/sdk
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<version>
```

Ensure an Android device is connected via USB (with USB debugging enabled) or an emulator is running.

### iOS

**On macOS**: Install Xcode from the App Store.

**On Linux**: See [iOS on Linux with xtool](/docs/guides/xtool-setup) to build iOS apps without a Mac.

## 1. Install the CLI

```bash
go install github.com/go-drift/drift/cmd/drift@latest
```

Ensure `$(go env GOPATH)/bin` is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### Verify Installation

```bash
drift --help
```

You should see the list of available commands.

## 2. Create a Project

```bash
drift init hello-drift
cd hello-drift
```

The argument is the directory to create. The project name is derived from its
basename, so `drift init ./projects/hello-drift` also works and produces a
project named `hello-drift`. You can optionally pass a Go module path as a
second argument (e.g. `drift init hello-drift github.com/username/hello-drift`).

This creates:
- `main.go` - Your app entry point
- `go.mod` - Go module file

The generated `main.go`:

```go
package main

import (
    "github.com/go-drift/drift/pkg/core"
    "github.com/go-drift/drift/pkg/drift"
    "github.com/go-drift/drift/pkg/widgets"
)

func main() {
    drift.NewApp(App()).Run()
}

func App() core.Widget {
    return widgets.Centered(
        widgets.Text{Content: "Hello, Drift!"},
    )
}
```

## 3. Run Your App

Choose your target platform:

### Android

```bash
drift run android
```

Requires a connected device or running emulator.

### iOS Simulator (macOS)

```bash
drift run ios
```

Runs on "iPhone 15" by default. Specify a different simulator:

```bash
drift run ios --simulator "iPhone 16"
```

List available simulators with `xcrun simctl list devices`.

### iOS Device (macOS)

```bash
drift run ios --device --team-id YOUR_TEAM_ID
```

Requires:
- A connected device with developer mode enabled
- Your Apple Developer Team ID (find it in Xcode or Apple Developer portal)
- `ios-deploy` installed (`brew install ios-deploy`)

### iOS from Linux (xtool)

```bash
drift run xtool
```

Requires xtool setup. See [iOS on Linux with xtool](/docs/guides/xtool-setup).

### First Run

On first run, Drift downloads Skia binaries for your target platform. This happens once and is cached.

## 4. Edit and Re-run

Try changing the text in `main.go`:

```go
func App() core.Widget {
    return widgets.Centered(
        widgets.Text{Content: "Hello, World!"},
    )
}
```

Run again to see your change:

```bash
drift run android  # or your target
```

## Configuration (Optional)

Create `drift.yaml` to customize your app:

```yaml
app:
  name: Hello Drift
  id: com.example.hellodrift
  orientation: portrait
  icon: assets/icon.png
  icon_background: "#FFFFFF"

engine:
  version: latest
```

| Field | Description |
|-------|-------------|
| `app.name` | Display name of your app |
| `app.id` | Bundle/package identifier |
| `app.orientation` | Supported orientations: `portrait` (default), `landscape`, or `all` |
| `app.allow_http` | Allow cleartext HTTP traffic (`true`/`false`, default `false`) |
| `app.icon` | Path to a square PNG (minimum 1024x1024). If omitted, a default icon is used. |
| `app.icon_background` | Hex color for the Android adaptive icon background (`#RGB` or `#RRGGBB`, default `#FFFFFF`). |
| `engine.version` | Drift engine version (`latest` or specific tag) |

## CLI Reference

| Command | Description |
|---------|-------------|
| `drift init <directory> [module-path]` | Create a new project |
| `drift run android` | Run on Android device/emulator |
| `drift run ios` | Run on iOS simulator (default: iPhone 15) |
| `drift run ios --simulator "<name>"` | Run on specific iOS simulator |
| `drift run ios --device --team-id ID` | Run on physical iOS device |
| `drift run xtool` | Run iOS from Linux via xtool |
| `drift build android\|ios\|xtool` | Build without running |
| `drift clean` | Clear build cache |
| `drift fetch-skia` | Download Skia binaries manually |

## Troubleshooting

### "drift: command not found"

Ensure `$(go env GOPATH)/bin` is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### Android: "ANDROID_HOME not set"

Set the Android SDK path:

```bash
export ANDROID_HOME=/path/to/android/sdk
```

### Android: "ANDROID_NDK_HOME not set"

Set the NDK path (find your version in `$ANDROID_HOME/ndk/`):

```bash
export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/27.0.12077973
```

### Android: "no devices/emulators found"

- Check `adb devices` shows your device
- Enable USB debugging on your device
- Or start an emulator via Android Studio

### iOS: "no simulator found"

List available simulators:

```bash
xcrun simctl list devices
```

Use the exact name in quotes:

```bash
drift run ios --simulator "iPhone 16"
```

## Next Steps

- [Widget Architecture](/docs/guides/widgets) - Build UI with widgets
- [Widget Catalog](/docs/category/widget-catalog) - Detailed usage for every Drift widget
- [Layout System](/docs/guides/layout) - Arrange widgets on screen
- [State Management](/docs/guides/state-management) - Handle app state
