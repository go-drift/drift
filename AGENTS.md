# AGENTS.md

## Project Overview
Drift is a cross-platform mobile UI framework written in Go. It uses Skia for GPU-accelerated rendering and targets Android and iOS.

## Build Commands
```bash
make cli                # Build drift CLI
go build ./...          # Build all Go packages
go test ./...           # Run tests
go mod tidy             # Sync dependencies
gofmt -w .              # Format code
go vet ./...            # Lint
```

## Project Structure
- `cmd/drift/` - CLI tool (build, run, clean, init, devices, log)
- `pkg/` - Core framework packages (core, engine, graphics, layout, widgets, animation, theme, navigation, gestures, platform, skia, svg)
- `showcase/` - Demo application
- `scripts/` - Skia build scripts
- `third_party/` - Skia source and prebuilt binaries

## Code Style
- Use `gofmt` for formatting
- CamelCase for exported, lowerCamelCase for unexported
- Group stdlib imports first, then module imports
- Wrap errors with context: `fmt.Errorf("...: %w", err)`
- Keep interfaces small and defined where used

## Widget Patterns
- Prefer struct literals for simple widgets
- Use helper constructors when defaults matter (e.g., `widgets.ButtonOf`)
- Compose UIs by nesting widgets
- Only mutate state inside `SetState`

## CGO and Platform
- CGO bridges Go to Skia (C++)
- Android: Kotlin embedder, JNI bridge
- iOS: Swift embedder, Metal rendering
- Keep bridge functions thin; delegate logic to Go
