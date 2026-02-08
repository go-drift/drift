---
id: skia
title: Skia Build
sidebar_position: 17
---

# Skia Build

Drift uses `libdrift_skia.a`, which combines Skia with the Drift bridge code. The bridge is pre-compiled at CI/build time, so app developers using prebuilt binaries don't need Skia headers - only the `skia_bridge.h` header included in the drift module. Building from source still requires a full Skia checkout.

The drift CLI searches for libraries in these locations (in order):

1. `<drift_module>/third_party/drift_skia/` - source builds take priority
2. `<project>/third_party/drift_skia/` - project-local customization
3. `~/.drift/drift_skia/` - downloaded prebuilt binaries (fallback)

Skia source is still checked out under `third_party/skia`; build outputs are copied into `third_party/drift_skia/`.

The library paths (relative to any of the above locations) are:

- Android: `android/<arch>/libdrift_skia.a`
- iOS device: `ios/<arch>/libdrift_skia.a`
- iOS simulator: `ios-simulator/<arch>/libdrift_skia.a`

Supported architectures:

| Platform      | Architectures           |
|---------------|-------------------------|
| Android       | `arm64`, `arm`, `amd64` |
| iOS device    | `arm64`                 |
| iOS simulator | `arm64`, `amd64`        |

## Automatic Download (Recommended)

Skia libraries are automatically downloaded when you run `drift build` for the first time. No manual setup is required for most users.

When you run `drift build`, missing Skia libraries are automatically downloaded:

```bash
drift build android    # auto-downloads Android libraries if missing
drift build ios        # auto-downloads iOS libraries if missing
```

To disable auto-download and fail with an error instead:

```bash
drift build android --no-fetch
```

## Fetch via CLI

You can manually download libraries using the `fetch-skia` command:

```bash
drift fetch-skia              # fetch all platforms
drift fetch-skia --android    # android only
drift fetch-skia --ios        # ios only
drift fetch-skia --version v0.2.0  # specific version
```

The command downloads tarballs from `https://github.com/go-drift/drift/releases`, verifies SHA256 checksums, and extracts them to `~/.drift/lib/<version>/`.

Version is determined automatically from the CLI version, or you can set `DRIFT_VERSION` environment variable or use the `--version` flag.

## Build from Source

Building from source is intended for drift contributors or users who need custom Skia builds. Most app developers should use the prebuilt binaries.

For source builds, you need a writable checkout of the drift repository:

```bash
git clone https://github.com/go-drift/drift.git
DRIFT_SRC=$PWD/drift
```

Building from source requires:
- Python 3
- Ninja build system
- For Android: `ANDROID_NDK_HOME` environment variable set

### Pinned Skia Version

The `SKIA_REV` file in the repository root pins the exact Skia commit used for builds. This ensures all contributors and CI workflows build against the same Skia version.

To update to a newer Skia version:

```bash
# Update to latest Skia main
$DRIFT_SRC/scripts/update_skia_rev.sh

# Or pin a specific commit
$DRIFT_SRC/scripts/update_skia_rev.sh --rev <commit-hash>

# Update and fetch in one step
$DRIFT_SRC/scripts/update_skia_rev.sh --sync
```

To build with a custom Skia checkout (e.g., for testing local patches), set `SKIP_SKIA_REV=1`:

```bash
SKIP_SKIA_REV=1 $DRIFT_SRC/scripts/build_skia_android.sh
```

### Fetch Skia Source

```bash
$DRIFT_SRC/scripts/fetch_skia.sh
```

This clones Skia from `https://skia.googlesource.com/skia`, checks out the revision specified in `SKIA_REV`, and runs `git-sync-deps` to fetch dependencies.

### Build for Android

```bash
export ANDROID_NDK_HOME=/path/to/android-ndk
$DRIFT_SRC/scripts/build_skia_android.sh
```

Builds `libdrift_skia.a` for `arm64`, `arm`, and `amd64` using NDK API level 21 with OpenGL ES support. The script compiles Skia, then compiles the drift bridge and combines them into a single static library. Output is written to `third_party/drift_skia/android/`.

### Build for iOS

Requires macOS with Xcode command line tools (the script uses `xcrun` and `libtool`).

```bash
$DRIFT_SRC/scripts/build_skia_ios.sh
```

Builds `libdrift_skia.a` for:
- Device: `arm64`
- Simulator: `arm64` (Apple Silicon), `amd64` (Intel)

Uses Metal for GPU graphics. The script compiles Skia, then compiles the drift bridge and combines them using libtool. Output is written to `third_party/drift_skia/ios/` and `third_party/drift_skia/ios-simulator/`.

### Build for iOS using xtool (Linux)

If you have [xtool](https://xtool.sh) set up for iOS development on Linux, you can cross-compile Skia without macOS:

```bash
$DRIFT_SRC/scripts/build_skia_xtool.sh
```

The script auto-detects the iOS SDK from `~/.xtool/sdk/iPhoneOS.sdk` (set up via `xtool setup`) and uses the system clang with `--target` flags for cross-compilation. It creates wrapper scripts to translate Skia's macOS-style `-arch` flags.

Requirements:
- xtool with iOS SDK (`xtool setup /path/to/Xcode.xip`)
- System clang (14+)
- Python 3, Ninja

Output is written to `third_party/drift_skia/ios/`. Simulator builds are skipped if `iPhoneSimulator.sdk` is not present.
