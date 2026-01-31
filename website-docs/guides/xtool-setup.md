---
id: xtool-setup
title: iOS on Linux with xtool
sidebar_position: 17
---

# iOS on Linux with xtool

This guide covers setting up your Linux (or macOS) system to build and deploy iOS apps using the `drift build xtool` and `drift run xtool` commands, without requiring Xcode.

## Overview

The xtool platform target enables iOS development on Linux by using:
- **xtool** - Cross-compilation toolchain for iOS
- **Swift Package Manager** - Building Swift code with iOS SDK
- **libimobiledevice** - Communicating with iOS devices
- **zsign** (optional) - Code signing for device deployment

## Requirements

### 1. Install Swift, Clang, and LLVM Tools

You need Swift 5.9+, Clang, and LLVM tools (including `llvm-libtool-darwin`) for cross-compilation:

```bash
# Ubuntu/Debian
sudo apt install clang llvm lld libstdc++-12-dev
wget https://download.swift.org/swift-5.9.2-release/ubuntu2204/swift-5.9.2-RELEASE/swift-5.9.2-RELEASE-ubuntu22.04.tar.gz
tar xzf swift-5.9.2-RELEASE-ubuntu22.04.tar.gz
sudo mv swift-5.9.2-RELEASE-ubuntu22.04 /opt/swift
export PATH="/opt/swift/usr/bin:$PATH"

# Create libtool symlink for Swift (required for iOS builds)
sudo ln -sf /usr/bin/llvm-libtool-darwin /usr/local/bin/libtool

# Fedora
sudo dnf install clang llvm lld swift-lang
sudo ln -sf /usr/bin/llvm-libtool-darwin /usr/local/bin/libtool

# Arch Linux
sudo pacman -S clang llvm lld swift
sudo ln -sf /usr/bin/llvm-libtool-darwin /usr/local/bin/libtool
```

If `llvm-libtool-darwin` is not available, find it in your LLVM installation:
```bash
find /usr -name "llvm-libtool-darwin" 2>/dev/null
# Or check versioned paths like /usr/lib/llvm-18/bin/llvm-libtool-darwin
```

Verify installation:
```bash
swift --version
# Should show Swift 5.9+

clang --version
# Should show clang 14+

which libtool
# Should show /usr/local/bin/libtool
```

### 2. Install xtool and iOS SDK

xtool provides cross-compilation for iOS. Follow the official installation guide at [xtool.sh](https://xtool.sh) or use these steps:

#### Step 1: Install xtool

```bash
# Linux (AppImage)
curl -sSL https://github.com/xtool-org/xtool/releases/latest/download/xtool-x86_64.AppImage -o ~/.local/bin/xtool
chmod +x ~/.local/bin/xtool

# Ensure ~/.local/bin is in your PATH
export PATH="$HOME/.local/bin:$PATH"
```

#### Step 2: Download Xcode.xip

Download Xcode from the [Apple Developer portal](https://developer.apple.com/download/all/). You need the `.xip` file (not installed from the App Store).

#### Step 3: Run xtool setup

```bash
xtool setup /path/to/Xcode.xip
```

This extracts the iOS SDK from Xcode and registers it as a Swift SDK bundle. The SDK will be available at `~/.swiftpm/swift-sdks/`.

Verify the SDK is installed:
```bash
swift sdk list
# Should show: darwin
```

For additional help, see the [xtool documentation](https://xtool.sh) or run `xtool help setup`.

### 3. Install libimobiledevice

libimobiledevice provides tools to communicate with iOS devices:

```bash
# Ubuntu/Debian
sudo apt install libimobiledevice-utils ideviceinstaller usbmuxd

# Fedora
sudo dnf install libimobiledevice-utils ideviceinstaller usbmuxd

# Arch Linux
sudo pacman -S libimobiledevice ideviceinstaller usbmuxd

# macOS (via Homebrew)
brew install libimobiledevice ideviceinstaller
```

Start the usbmuxd service:
```bash
sudo systemctl enable --now usbmuxd
```

### 4. Install zsign (Optional, for code signing)

zsign enables code signing on Linux for device deployment:

```bash
git clone https://github.com/AnySign/zsign
cd zsign
make
sudo make install
```

Or:
```bash
# Build and install to ~/.local/bin
g++ -O3 -o ~/.local/bin/zsign *.cpp -lcrypto -lssl
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `XTOOL_CERT` | Path to signing certificate (.p12) | None |
| `XTOOL_PROFILE` | Path to provisioning profile (.mobileprovision) | None |

### Code Signing Setup

For deploying to physical devices, you need:

1. **Apple Developer Certificate** - Export from Keychain as .p12 file
2. **Provisioning Profile** - Download from Apple Developer portal

```bash
export XTOOL_CERT=~/certs/dev_certificate.p12
export XTOOL_PROFILE=~/certs/dev_profile.mobileprovision
```

## Skia Binaries

Ensure iOS Skia binaries are available before building. They are downloaded automatically on first build, or you can fetch them manually:

```bash
drift fetch-skia --ios
```

For more options, see the [Skia Build](/docs/guides/skia) guide.

## Usage

### Building

```bash
# Build debug version
drift build xtool

# Build release version
drift build xtool --release

# Build with device signing
drift build xtool --device
```

### Running on Device

```bash
# Build and run on connected device
drift run xtool

# Run on specific device (by UDID)
drift run xtool --device 00008030-001234567890

# Run without streaming logs
drift run xtool --no-logs
```

### Listing Connected Devices

```bash
# List connected iOS devices
idevice_id -l

# Get device info
ideviceinfo
```

## Troubleshooting

### "xtool not found"

Ensure xtool is in your PATH:
```bash
which xtool
# Should output: /home/user/.local/bin/xtool
```

### "No valid Swift SDK bundles found"

Ensure you've run `xtool setup` with Xcode.xip:
```bash
xtool setup /path/to/Xcode.xip

# Verify SDK is registered
swift sdk list
# Should show: darwin
```

### "swift build failed"

Ensure Swift can find the SDK:
```bash
swift --version
# Should show Swift 5.9+

# Verify SDK is available
swift sdk list
# Should show: darwin
```

### "ideviceinstaller not found"

Install libimobiledevice:
```bash
sudo apt install libimobiledevice-utils ideviceinstaller
```

### "Could not connect to device"

1. Ensure usbmuxd is running:
   ```bash
   sudo systemctl status usbmuxd
   ```

2. Trust the computer on your iOS device when prompted

3. Check device connection:
   ```bash
   idevice_id -l
   ```

### "Installation failed - code signature invalid"

You need to sign the app for device deployment:

1. Obtain a development certificate and provisioning profile from Apple Developer portal
2. Set environment variables:
   ```bash
   export XTOOL_CERT=/path/to/certificate.p12
   export XTOOL_PROFILE=/path/to/profile.mobileprovision
   ```
3. Rebuild with `--device` flag:
   ```bash
   drift build xtool --device
   ```

## Project Structure

When you run `drift build xtool`, the following structure is generated:

```
~/.drift/build/<module>/xtool/<hash>/xtool/
├── Package.swift              # SwiftPM manifest
├── Sources/Runner/
│   ├── main.swift             # App entry point
│   ├── AppDelegate.swift      # Application delegate
│   ├── SceneDelegate.swift    # Scene lifecycle
│   ├── DriftViewController.swift
│   ├── DriftMetalView.swift
│   ├── DriftRenderer.swift
│   ├── PlatformChannel.swift
│   ├── PlatformView.swift
│   ├── TextInput.swift
│   └── Resources/
│       ├── Info.plist
│       └── LaunchScreen.storyboard
├── Libraries/
│   ├── CDrift/
│   │   ├── module.modulemap
│   │   ├── libdrift.a         # Compiled Go code
│   │   └── libdrift.h
│   └── CSkia/
│       ├── module.modulemap
│       └── libdrift_skia.a    # Skia + drift bridge library
└── Runner.app/                # Final app bundle
```
