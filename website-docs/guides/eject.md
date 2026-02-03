---
id: eject
title: Ejecting
sidebar_position: 15
---

# Ejecting

By default, Drift manages platform projects (Xcode, Android Studio) behind the scenes in `~/.drift/build/`. Ejecting extracts the native project into your repository so you can customize it directly.

## When to Eject

Eject when you need to:

- Add native SDKs (Firebase, push notifications, analytics)
- Edit `Info.plist` or `AndroidManifest.xml` for custom permissions
- Modify the Xcode project or Gradle build configuration
- Open the project in Xcode or Android Studio for debugging

If you only need to change your app name, bundle ID, or other values in `drift.yaml`, you do not need to eject.

## Ejecting a Platform

```bash
drift eject ios        # Eject iOS project
drift eject android    # Eject Android project
drift eject all        # Eject both platforms
```

This creates a `platform/` directory in your project root:

```
myapp/
├── drift.yaml
├── main.go
└── platform/
    ├── ios/                      # Real Xcode project
    │   ├── Runner/
    │   │   ├── Info.plist
    │   │   └── AppDelegate.swift
    │   ├── Runner.xcodeproj/
    │   ├── bridge/               # Drift-managed, regenerated on build
    │   └── driftw                # Wrapper script for IDE builds
    └── android/                  # Real Android project
        ├── app/
        │   ├── build.gradle
        │   └── src/main/
        │       ├── AndroidManifest.xml
        │       └── java/com/example/myapp/
        │           └── MainActivity.kt
        ├── settings.gradle
        ├── bridge/               # Drift-managed, regenerated on build
        └── driftw                # Wrapper script for IDE builds
```

After ejecting, you can open the project directly:

- **iOS**: Open `platform/ios/Runner.xcodeproj` in Xcode
- **Android**: Open `platform/android/` in Android Studio

## Check Eject Status

```bash
drift status
```

```
Project: myapp (com.example.myapp)

Platforms:
  ios:     ejected → ./platform/ios/
  android: managed → ~/.drift/build/myapp/android/<hash>/
```

## What Changes After Ejecting

| Aspect | Before eject | After eject |
|--------|--------------|-------------|
| Build location | `~/.drift/build/` | `./platform/<platform>/` |
| Project files | Generated fresh each build | User-owned, never overwritten |
| `drift.yaml` | Used for all values | Only affects non-ejected platforms |
| IDE usage | Not practical | Full Xcode/Android Studio support |
| Version control | Nothing to commit | Commit `./platform/` to repo |

Values from `drift.yaml` (app name, bundle ID) are substituted at eject time. After ejecting, changes to `drift.yaml` will not affect the ejected platform. Edit the native project files directly instead.

## Building After Ejecting

Both `drift build` and `drift run` continue to work after ejecting. The only difference is that they build in `./platform/<platform>/` instead of `~/.drift/build/`, and your customizations are preserved.

### From the CLI

```bash
drift run ios      # Compiles Go, builds in ./platform/ios/, runs on device/simulator
drift run android  # Compiles Go, builds in ./platform/android/, runs on device/emulator
```

### From Xcode

Open `platform/ios/Runner.xcodeproj` and press Cmd+R. A build phase script calls `drift compile` automatically before the Swift build.

### From Android Studio

Open `platform/android/` and press Run. A Gradle task calls `drift compile` automatically before the Android build.

## File Ownership

After ejecting, some files belong to you and some are managed by Drift. Drift never overwrites your files on build.

**iOS:**

| Path | Owner |
|------|-------|
| `Runner/` (Swift files, Info.plist) | You |
| `Runner.xcodeproj/` | You |
| `bridge/` | Drift (regenerated on build) |
| `Runner/libdrift.a` | Drift (regenerated on build) |
| `Runner/libdrift_skia.a` | Drift (updated when Drift version changes) |

**Android:**

| Path | Owner |
|------|-------|
| `app/` (Kotlin, manifests, Gradle config) | You |
| `settings.gradle`, `gradle.properties`, `gradle/` | You |
| `bridge/` | Drift (regenerated on build) |
| `app/src/main/jniLibs/` | Drift (overwritten on build) |

:::warning
Do not place custom native libraries in `app/src/main/jniLibs/` as Drift overwrites this directory on each build. Use a separate directory and configure Gradle's `jniLibs.srcDirs` if you need additional native libraries.
:::

## `.gitignore` Setup

Add these entries to keep build artifacts out of version control:

```gitignore
# Drift build artifacts
platform/*/.drift.env
platform/ios/Runner/libdrift.a
platform/ios/Runner/libdrift_skia.a
platform/ios/Runner/.drift-skia-version
platform/android/app/src/main/jniLibs/
platform/*/bridge/
```

If your project already ignores `platform/` (common in some frameworks), update the rule to only ignore build artifacts:

```gitignore
# Before (ignores everything)
platform/

# After (ignore only build artifacts)
platform/*/.drift.env
platform/ios/Runner/libdrift.a
platform/ios/Runner/libdrift_skia.a
platform/ios/Runner/.drift-skia-version
platform/android/app/src/main/jniLibs/
platform/*/bridge/
```

## Re-ejecting

If a platform is already ejected, `drift eject` will refuse to overwrite it:

```
Error: platform/ios/ already exists. Use --force to overwrite (creates backup).
```

Use `--force` to back up the existing directory and eject a fresh copy:

```bash
drift eject --force ios
```

The backup is saved as `platform/ios.backup.<timestamp>/`.

## Undoing Eject

To return to managed mode, delete the platform directory:

```bash
rm -rf ./platform/ios      # Return iOS to managed mode
rm -rf ./platform/android  # Return Android to managed mode
rm -rf ./platform          # Return both to managed mode
```

After deletion, `drift build` and `drift run` will use `~/.drift/build/` again and regenerate everything from templates and `drift.yaml`.

Verify with `drift status`:

```bash
drift status
# ios:     managed → ~/.drift/build/...
# android: managed → ~/.drift/build/...
```

:::caution
Deleting the platform directory discards all native customizations you made while ejected (Info.plist changes, added SDKs, Gradle configuration, etc.). Make sure any important changes are backed up or recorded before deleting.
:::

If the ejected directory is tracked in git, also remove it from version control:

```bash
git rm -r ./platform/ios
git commit -m "Return iOS to managed mode"
```

## Next Steps

- [Platform Services](/docs/guides/platform) - Access native platform capabilities
- [iOS on Linux with xtool](/docs/guides/xtool-setup) - Build iOS apps without a Mac
