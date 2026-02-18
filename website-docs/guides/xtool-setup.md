---
id: xtool-setup
title: iOS on Linux with xtool
sidebar_position: 18
---

# iOS on Linux with xtool

This guide covers setting up your Linux system to build and deploy iOS apps using the `drift build xtool` and `drift run xtool` commands.

## Prerequisites

### 1. Install xtool

[xtool](https://xtool.sh) is a cross-platform Xcode replacement that enables iOS development on Linux. Follow the official [Getting Started](https://xtool.sh/documentation/xtool/installation-linux) guide to install xtool and its prerequisites:

- **Swift 6.2** from [swift.org](https://swift.org/install/linux)
- **usbmuxd** for communicating with iOS devices
- **Xcode.xip** downloaded from Apple (needed for the iOS SDK)
- **xtool** itself, installed via its [GitHub releases](https://github.com/xtool-org/xtool/releases)

After following the xtool setup guide, verify everything is working:

```bash
xtool --help
# OVERVIEW: Cross-platform Xcode replacement

swift sdk list
# darwin
```

### 2. Fetch iOS Skia binaries

Skia binaries are downloaded automatically on first build, or you can fetch them manually:

```bash
drift fetch-skia --ios
```

For more options, see the [Skia Build](/docs/guides/skia) guide.

## Usage

App icons are configured via `app.icon` and `app.icon_background` in `drift.yaml`, the same as other platforms. See the [Configuration](/docs/guides/getting-started#configuration-optional) section for details.

### Building

```bash
# Build debug version
drift build xtool

# Build release version
drift build xtool --release
```

### Running on Device

```bash
# Build and run on connected device
drift run xtool

# Run on specific device (by UDID)
drift run xtool --device 00008030-001234567890

# Run without streaming logs
drift run xtool --no-logs

# Watch mode: rebuild on file changes
drift run xtool --watch
```

**Note:** with `--watch`, Drift attempts to relaunch the app automatically after each rebuild. If the relaunch fails (e.g. the device is locked), open the app manually.

### Streaming Logs

You can stream device logs independently of `drift run`:

```bash
# Stream logs from connected device
drift log xtool

# Stream logs from a specific device
drift log xtool --device 00008030-001234567890
```

### Listing Connected Devices

```bash
drift devices
```

## Troubleshooting

### "xtool not found"

Ensure xtool is in your PATH:
```bash
which xtool
```

### "No valid Swift SDK bundles found"

Re-run `xtool setup` and verify the SDK is registered:
```bash
swift sdk list
# Should show: darwin
```

### "Could not connect to device"

1. Ensure usbmuxd is running:
   ```bash
   sudo systemctl status usbmuxd
   ```

2. Trust the computer on your iOS device when prompted

3. Check device connection:
   ```bash
   drift devices
   ```

For additional troubleshooting, see the [xtool documentation](https://xtool.sh).
