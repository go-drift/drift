---
id: plugins
title: Plugins
sidebar_position: 8
---

# Plugins

Plugins add native capabilities to a Drift app: SDKs, permissions, native UI, and platform hooks. The app never edits the generated Xcode or Android Studio projects; Drift applies each plugin's changes on every build.

A plugin is a Go module (a first-party plugin is a directory of the Drift module) with up to three parts:

| Part | Package | Runs | Purpose |
|------|---------|------|---------|
| Build half | `<module>/plugin` | At build time | Reads the plugin's config from `drift.yaml` and records the native project changes it needs (**ops**) |
| Native code | Swift and Kotlin embedded in the build half | In the app | The platform side: SDK calls, lifecycle hooks, overlays |
| Runtime half | `<module>/runtime` (any name) | In the app | The Go API the app calls; talks to the native code over platform channels |

First-party plugins ship in the `github.com/go-drift/drift` module, at the same version as the rest of Drift; the [catalogue](/docs/guides/plugin-catalog) covers each one:

- [`plugins/splash`](/docs/guides/plugin-catalog#splash): a native splash screen held until the first frame.
- [`plugins/firebase`](/docs/guides/plugin-catalog#firebase): Firebase Core and Cloud Messaging (push notifications).
- [`plugins/location`](/docs/guides/plugin-catalog#location): the device's location and the location permissions.
- [`plugins/permissions`](/docs/guides/plugin-catalog#permissions): contacts, calendar, microphone and photos permissions.
- [`plugins/notifications`](/docs/guides/plugin-catalog#notifications): local notifications and taps on them.
- [`plugins/imagepicker`](/docs/guides/plugin-catalog#image-picker): a photo from the camera, or images from the photo library.
- [`plugins/securestorage`](/docs/guides/plugin-catalog#secure-storage): encrypted key-value storage, optionally behind biometrics.
- [`plugins/media`](/docs/guides/plugin-catalog#media): video and audio playback.

## Adding a plugin

List the plugin's build half in `drift.yaml`. A first-party plugin needs nothing else: it comes with your app's `github.com/go-drift/drift` dependency.

```yaml
# drift.yaml
plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
      background_color: "#1A2238"
```

A third-party plugin is a dependency of its own; add its module first:

```bash
go get example.com/hello@<version>
```

The next `drift build` or `drift run` picks it up. Plugins run in `drift.yaml` order, which also decides which plugin gets first claim on shared events such as notification taps.

`config` is checked against the plugin's typed config: unknown keys, missing required keys, malformed colours and missing files fail the build with the offending key named.

## The plugin bridge

Drift runs build halves through a small generated program, `tools/drift-plugins/main.go`, built and cached by the CLI. Commit it: it imports every configured plugin, which keeps their modules in `go.mod` when you run `go mod tidy`.

Drift never edits `go.mod` or `go.sum` during a build. If an entry is missing, the build stops and tells you to run `go mod tidy` (or `go get` for a third-party plugin that is not a dependency yet). A first-party plugin that is not found has a wrong package path, or is newer than the app's Drift: the error names the Drift version to update to (see [Versions](#versions)).

```bash
drift plugin sync          # regenerate the bridge, check drift.yaml
drift plugin sync --tidy   # same, running go mod tidy first
drift plugin list          # configured plugins
drift plugin list --resolve  # with module versions and status
drift plugin list --json
```

`drift plugin sync` checks each plugin's config against its schema and then runs the plugin's `Build` without building the app, so it rejects the config a build would. It exits nonzero on invalid config, so CI can run it.

## Versions

First-party plugins are part of the `github.com/go-drift/drift` module, so they always match the Drift your app uses. Use that module and the `drift` CLI at one version, and upgrade them together; for example, to v0.28.0:

```bash
go install github.com/go-drift/drift/cmd/drift@v0.28.0
go get github.com/go-drift/drift@v0.28.0
```

A plugin's native code compiles against the plugin API the CLI ships, so a release CLI checks every build and stops on a mismatch, naming the versions involved:

| Error | Cause | Fix |
|-------|-------|-----|
| Plugins require a newer drift than this CLI | A plugin's `go.mod` requires a newer Drift minor release than the CLI's (v0.28.x against a v0.27.x CLI), so its native code may use API the CLI lacks; patch releases do not change the plugin API | Install the CLI the error names |
| Plugins were written for an older plugin API | A plugin requires a Drift older than the oldest plugin API the CLI still serves | Update the plugin, or use the CLI it was written for |
| The app builds with drift X, but this CLI is Y | The app's `go.mod` selects a Drift whose major.minor differs from the CLI's | Install the CLI matching the app, or change the app's Drift version |

Development builds of the CLI (built from a checkout) skip these checks.

## Removing a plugin

Delete its entry from `drift.yaml`. Managed builds regenerate the native project from scratch, and `drift run --watch` regenerates it as soon as the plugin set changes, so nothing is left behind. With no plugins left, Drift also deletes `tools/drift-plugins/main.go`; run `go mod tidy` to drop the module.

## Ejected projects

Plugins work in [ejected](/docs/guides/eject) projects too, applied to your files on every build. [Ejecting](/docs/guides/eject#plugins) covers the files Drift owns there, the lock that records plugin output, and what happens when you remove a plugin.

## Next Steps

- [Plugin Catalog](/docs/guides/plugin-catalog) - Drift's own plugins and their config
- [Authoring Plugins](/docs/guides/plugin-authoring) - Writing a plugin, and every op one can record
- [Platform Services](/docs/guides/platform) - Built-in native services
- [Ejecting](/docs/guides/eject) - Owning the native projects
