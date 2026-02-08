---
id: intro
title: Introduction
sidebar_position: 1
---

# Drift

Drift is a cross-platform mobile UI framework for Go. Write your app once in Go, then build native Android and iOS apps.

<video src="/showcase.mp4" controls autoPlay muted playsInline width="300" />

## Why Drift?

- **Single codebase** - Write your app once in Go, deploy to Android and iOS
- **Go-native** - Use Go's tooling, testing, and ecosystem you already know
- **Skia rendering** - Hardware-accelerated graphics via the same engine Chrome and Flutter use
- **No bridge overhead** - Direct native compilation, no JavaScript or VM layer
- **iOS builds on Linux** - Build iOS apps without a Mac using [xtool](https://xtool.sh)

## How It Works

Drift apps are Go programs that return widgets. The Drift CLI compiles your Go code with CGO, links it against Skia, and packages it into a native Android APK or iOS app.

```go
func main() {
    drift.NewApp(App()).Run()
}

func App() core.Widget {
    return widgets.Centered(
        widgets.Text{Content: "Hello, Drift!"},
    )
}
```

## Requirements

- **Go 1.24** or later
- **Android**: Android SDK + NDK, Java 17+
- **iOS**: macOS with Xcode, or Linux with [xtool](/docs/guides/xtool-setup)

## Get Started

Ready to build your first app?

**[Getting Started Guide](/docs/guides/getting-started)** - Install the CLI and run your first app in minutes.

## Learn More

- [Widget Architecture](/docs/guides/widgets) - UI building blocks
- [Widget Catalog](/docs/category/widget-catalog) - Detailed usage for every widget
- [State Management](/docs/guides/state-management) - Managing app state
- [API Reference](/docs/api/core) - Full API documentation
