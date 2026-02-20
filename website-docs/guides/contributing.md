---
id: contributing
title: Contributing
sidebar_position: 1
---

# Contributing

This guide covers how to set up a development environment for contributing to the Drift framework itself.

## Getting the Source

Clone the repository and install dependencies:

```bash
git clone https://github.com/go-drift/drift.git
cd drift
go mod tidy
```

## Building the CLI

The `drift` CLI is built from `cmd/drift/`. Templates for Android and iOS projects are embedded into the binary at compile time, so any changes to files in `cmd/drift/internal/templates/` require rebuilding:

```bash
make cli
```

After rebuilding, use the locally built `drift` binary instead of the installed one.

## Running Tests

```bash
go test ./...
```

To update snapshot tests:

```bash
DRIFT_UPDATE_SNAPSHOTS=1 go test ./pkg/...
```

## Code Style

Format and lint before submitting changes:

```bash
gofmt -w .
go vet ./...
```

## Generating Documentation

API reference docs are generated from Go source comments:

```bash
go run cmd/docgen/main.go
```

This copies hand-written guides from `website-docs/` into `website/docs/` and generates API docs using gomarkdoc. Preview with `cd website && npm start`.

## Project Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/drift/` | CLI tool (build, run, clean, init, devices, log) |
| `cmd/docgen/` | Documentation generator |
| `pkg/` | Core framework packages |
| `showcase/` | Demo application |
| `scripts/` | Skia build scripts |
| `third_party/` | Skia source and prebuilt binaries |
| `website-docs/` | Hand-written guides (source of truth) |
| `website/` | Docusaurus website config |

## Building Skia from Source

If your changes involve the Skia bridge or CGO layer, you may need to build Skia locally. See the [Skia Build](/docs/guides/skia) guide for instructions.

## Platform Templates

When adding iOS or Android native code, ensure the templates in `cmd/drift/internal/templates/` are updated:

- **iOS**: Update both `ios/Info.plist.tmpl` and `xtool/Info.plist.tmpl` (and corresponding xcodeproj/xtool templates) in the same change.
- **Android**: Update `AndroidManifest.xml` and project templates.

Template changes require rebuilding the CLI with `make cli`.
