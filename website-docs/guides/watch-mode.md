---
id: watch-mode
title: Watch Mode
sidebar_position: 3
---

# Watch Mode

Watch mode automatically rebuilds and relaunches your app when source files change, giving you a faster development loop.

## How It Works

When you pass `--watch` to `drift run`, Drift starts a file watcher on your project directory. Any change to a `.go` file or `drift.yaml`/`drift.yml` triggers a rebuild after a 500ms debounce window. The app is stopped, rebuilt, and relaunched automatically.

## Usage

Add `--watch` to any `drift run` command:

```bash
# Android
drift run android --watch

# iOS Simulator (macOS)
drift run ios --watch

# iOS Device (macOS)
drift run ios --device --watch --team-id YOUR_TEAM_ID

# iOS from Linux (xtool)
drift run xtool --watch
```

After the initial build, Drift prints "Watching for changes..." and waits. Edit your code, save, and the rebuild starts automatically.

Press **Ctrl+C** to stop watch mode.

## Log Streaming

In watch mode, device logs are streamed to your terminal by default. Suppress them with `--no-logs`:

```bash
drift run android --watch --no-logs
```

Log streaming works differently per platform:

- **Android**: logs are filtered by Drift-specific logcat tags (`DriftJNI`, `Go`, `AndroidRuntime`, etc.). Tag-based filtering survives app restarts, so logs continue seamlessly across rebuilds.
- **iOS Simulator**: logs are streamed via `simctl launch --console`, which captures the app's stdout/stderr directly. Each rebuild terminates and relaunches the app, restarting the console stream automatically.
- **iOS Device (Xcode)**: logs are streamed via `devicectl --console`, which works the same way as the simulator path.
- **iOS Device (xtool)**: logs are streamed via the device syslog, filtered by process name.

## What Triggers a Rebuild

Only changes to these files trigger a rebuild:

- `.go` files (your application source)
- `drift.yaml` or `drift.yml` (project configuration)

Other file types (images, assets, etc.) are ignored by the watcher.

## Skipped Directories

The watcher skips these directories to avoid unnecessary rebuilds:

- Hidden directories (names starting with `.`)
- `vendor`
- `platform`
- `third_party`

## Android ABI Optimization

In watch mode, Drift detects the connected device's ABI (e.g. `arm64-v8a`) and compiles only for that architecture. This significantly speeds up incremental rebuilds compared to a full multi-ABI build.

## xtool Notes

When using `drift run xtool --watch`, Drift attempts to kill and relaunch the app automatically after each rebuild. If the relaunch times out or fails (e.g. the device is locked), you will need to open the app manually on your device.

## Next Steps

- [Getting Started](/docs/guides/getting-started) for initial setup
- [iOS on Linux with xtool](/docs/guides/xtool-setup) for xtool configuration
