![Drift logo](assets/logo.svg)

# Drift

Drift is a cross-platform mobile UI framework. Write your app once in Go,
then build native Android and iOS apps via the Drift CLI.

## Why Drift?

- **Single codebase** - Write once, deploy to Android and iOS
- **Go-native** - Use Go's tooling, testing, and ecosystem
- **Skia rendering** - Hardware-accelerated graphics (same engine as Chrome and Flutter)
- **No bridge overhead** - Direct native compilation, no VM layer
- **iOS builds on Linux** - Build iOS apps without a Mac using [xtool](https://driftframework.dev/docs/guides/xtool-setup)

## Quick Start

```bash
# Install the CLI
go install github.com/go-drift/drift/cmd/drift@latest

# Create and run a new project
drift init hello-drift
cd hello-drift
drift run android
```

Skia binaries are downloaded automatically on first run.

## Documentation

Full documentation is available at **[driftframework.dev](https://driftframework.dev)**:

- [Getting Started](https://driftframework.dev/docs/guides/getting-started) - Installation and first app
- [Widgets](https://driftframework.dev/docs/guides/widgets) - Available UI components
- [Layout](https://driftframework.dev/docs/guides/layout) - Arranging widgets
- [State Management](https://driftframework.dev/docs/guides/state-management) - Managing app state
- [Navigation](https://driftframework.dev/docs/guides/navigation) - Routes and deep linking
- [Theming](https://driftframework.dev/docs/guides/theming) - Colors and typography
- [Animation](https://driftframework.dev/docs/guides/animation) - Motion and transitions
- [Platform Services](https://driftframework.dev/docs/guides/platform) - Native APIs
- [Skia](https://driftframework.dev/docs/guides/skia) - Building Skia from source

## Showcase

The `showcase/` directory contains a demo app with examples of all widgets:

https://github.com/user-attachments/assets/41dcef40-21d1-44bd-adff-56f637ff91e0


```bash
cd showcase
drift run android
```

## Repo Layout

| Directory | Description |
|-----------|-------------|
| `cmd/drift` | CLI commands |
| `pkg/` | Runtime, widgets, and rendering |
| `showcase/` | Demo application |
| `scripts/` | Skia build helpers |
| `third_party/skia` | Skia source checkout |
| `third_party/drift_skia` | Skia bridge outputs |
| `SKIA_REV` | Pinned Skia commit hash |

## API Stability

Drift follows semantic versioning.

**Before v1.0.0**: Breaking changes may occur in any release.

**After v1.0.0**:
- Deprecated APIs are marked with `// Deprecated: use X instead`
- Deprecated APIs remain for at least 2 minor versions
- Breaking changes only in major versions

## Roadmap

**v1.0.0 targets:**

- Testing framework with widget tests and golden tests
- More widgets (Dialog, BottomSheet, Drawer, Snackbar, Slider)
- Internationalization (i18n/l10n)
- Developer tools (widget inspector, performance profiling)
- Hot reload

## Contributing

Contributions are welcome!

## License

MIT License. See [LICENSE](LICENSE) for details.
