![Drift logo](assets/logo.svg)

# Drift

Drift is a cross-platform mobile UI framework in Go. It lets you write UI and
application logic in Go, then build native Android and iOS apps via the Drift
CLI, which generates platform scaffolding in a build cache and compiles your
app with CGO + Skia.

## Why Drift?

- **Single codebase** - Write your app once in Go, deploy to Android and iOS
- **Go-native** - Use Go's tooling, testing, and ecosystem you already know
- **Skia rendering** - Hardware-accelerated graphics via the same engine Chrome and Flutter use
- **No bridge overhead** - Direct native compilation, no JavaScript or VM layer
- **iOS builds on Linux** - Build iOS apps without a Mac using [xtool](docs/xtool-setup.md)

## Prerequisites

- Go 1.24
- Android builds: Android SDK + NDK, Java 17+, and `ANDROID_HOME` + `ANDROID_NDK_HOME` env vars
- iOS builds: macOS with Xcode installed
- Skia: prebuilt binaries, or see [skia.md](docs/skia.md) for building from source

## Quick Start

1. Create a new project and install Drift:

```bash
mkdir hello-drift && cd hello-drift
go mod init example.com/hello-drift
go get github.com/go-drift/drift@latest
go install github.com/go-drift/drift/cmd/drift@latest
```

Make sure `$(go env GOPATH)/bin` or `GOBIN` is on your `PATH` so the `drift`
command is available.

2. Create `main.go`:

```go
package main

import (
    "github.com/go-drift/drift/pkg/core"
    "github.com/go-drift/drift/pkg/drift"
    "github.com/go-drift/drift/pkg/widgets"
)

func main() {
    drift.NewApp(App()).Run()
}

func App() core.Widget {
    return widgets.Centered(
        widgets.Text{Content: "Hello, Drift!"},
    )
}
```

3. Fetch Skia binaries:

```bash
$(go env GOPATH)/pkg/mod/github.com/go-drift/drift@latest/scripts/fetch_skia_release.sh
```

4. Run your app:

```bash
drift run android
# or
drift run ios --simulator "iPhone 17"
```

5. (Optional) Add `drift.yaml` to customize app metadata:

```yaml
app:
  name: Hello Drift
  id: com.example.hellodrift
engine:
  version: latest
```

See the [usage guide](docs/usage-guide.md) for widget construction patterns, layout composition, state management, and theming.

## Skia Binaries

Drift requires prebuilt Skia libraries (`libdrift_skia.a`) which include both
Skia and the Drift bridge code. Download prebuilt artifacts from GitHub Releases
or build them locally (see [skia.md](docs/skia.md) for details).

```bash
DRIFT=$(go env GOMODCACHE)/github.com/go-drift/drift@<version>

# Fetch both Android and iOS
$DRIFT/scripts/fetch_skia_release.sh

# Fetch only one platform
$DRIFT/scripts/fetch_skia_release.sh --android
$DRIFT/scripts/fetch_skia_release.sh --ios
```

The script downloads binaries to `~/.drift/drift_skia/` where the drift CLI finds
them. Release artifacts are pinned to the Drift version and published under
`https://github.com/go-drift/drift/releases` with tags like `v<version>`. The
module cache is read-only, so source builds must run from a writable checkout
(see `docs/skia.md`).

For building Skia from source, see [skia.md](docs/skia.md).

## Build and Run Your App

From your app module root (where `go.mod` lives):

```bash
# Build
drift build android
drift build ios
drift build xtool

# Run on devices/simulators
drift run android
drift run ios --simulator "iPhone 17"
drift run xtool

# Clean build cache
drift clean
```

Notes:
- Android installs use `adb` and respect `ANDROID_SERIAL` if set.
- iOS builds require macOS and an Xcode project in the generated workspace.
- Xtool builds require xtool to be installed. See [xtool-setup.md](docs/xtool-setup.md).

## Repo Layout

- `cmd/drift`: Drift CLI commands
- `pkg/`: Drift runtime, widgets, and rendering
- `showcase/`: Demo application showcasing widgets
- `scripts/`: Skia build helpers
- `third_party/skia`: Skia source checkout
- `third_party/drift_skia`: Drift Skia bridge outputs

## Showcase App

The `showcase/` directory contains a full Drift demo. From the `showcase/`
directory, run:

```bash
drift run android
drift run ios
drift run xtool
```

## Features

### Implemented Features

#### Core Framework
- [x] Widget architecture - StatelessWidget, StatefulWidget, InheritedWidget patterns
- [x] Element tree - Efficient build/rebuild system with dirty tracking
- [x] Render tree - Constraint-based layout with RenderObjects
- [x] Build context - Tree traversal and dependency injection

#### Widgets
- [x] Layout - Row, Column, Stack, IndexedStack, Positioned, Center, Padding, SizedBox, Expanded, Container, SafeArea
- [x] Scrolling - ListView, ListViewBuilder, ScrollView with customizable physics
- [x] Display - Text, Icon, SVGIcon, Image
- [x] Input - Button, TextField, NativeTextField, Checkbox, Radio, Switch, Dropdown, Form
- [x] Decorative - ClipRRect, DecoratedBox, Opacity, AnimatedOpacity, AnimatedContainer
- [x] Navigation - TabBar, TabItem, TabScaffold
- [x] Platform - NativeWebView, NativeTextField

#### Layout System
- [x] Flex layout - MainAxisAlignment, CrossAxisAlignment, MainAxisSize
- [x] Alignment - EdgeInsets, Alignment, BoxParentData
- [x] Constraints - BoxConstraints with min/max width/height
- [x] Positioned stacking - Absolute positioning within Stack

#### Rendering (Skia)
- [x] Canvas API - DrawRect, DrawRRect, DrawOval, DrawPath, DrawText, DrawImage
- [x] Styling - Colors, gradients, paint styles, text styling
- [x] Geometry - Size, Offset, Rect, RRect, Path
- [x] Canvas state - Save/Restore, clipping, transforms

#### State Management
- [x] Core patterns - StatelessWidget, StatefulWidget, InheritedWidget
- [x] Lifecycle - InitState, Dispose, DidChangeDependencies, DidUpdateWidget
- [x] Helpers - ManagedState, Observable, Listenable
- [x] Hooks - UseController, UseObservable, UseListenable
- [x] Thread safety - drift.Dispatch for UI thread scheduling

#### Animation
- [x] Controllers - AnimationController with status tracking
- [x] Curves - Linear, Ease, EaseIn, EaseOut, EaseInOut, CubicBezier
- [x] Spring physics - SpringDescription, SpringSimulation, IOSSpring, BouncySpring
- [x] Animated widgets - AnimatedContainer, AnimatedOpacity
- [x] Ticker system - Frame-synchronized timing

#### Navigation & Routing
- [x] Navigator - Stack-based route management
- [x] Routes - MaterialPageRoute, custom Route interface
- [x] Route generation - OnGenerateRoute, OnUnknownRoute, InitialRoute
- [x] Navigation methods - Push, Pop, PushNamed, CanPop, MaybePop
- [x] Deep linking - DeepLinkController with URL parsing and routing
- [x] Back button - Platform back button handling

#### Gesture & Input
- [x] Gesture detection - GestureDetector, TapGestureRecognizer
- [x] Gesture arena - Multi-touch gesture competition with hold mechanism
- [x] Drag gestures - Pan, HorizontalDrag, VerticalDrag recognizers with axis locking
- [x] Focus management - FocusNode, FocusScopeNode, directional navigation
- [x] Scroll physics - ClampingScrollPhysics, BouncingScrollPhysics

#### Theming
- [x] Theme system - Theme widget with InheritedWidget propagation
- [x] Color scheme - Material Design 3 colors (primary, secondary, surface, etc.)
- [x] Typography - TextTheme with display, headline, title, body, label styles
- [x] Built-in themes - DefaultLightTheme, DefaultDarkTheme

#### Platform Support
- [x] Android - Full support via Android SDK/NDK
- [x] iOS - macOS builds + Linux builds via xtool
- [x] Cross-compilation - iOS builds from Linux using xtool

#### Platform Services
- [x] Clipboard - Copy/paste operations
- [x] Haptics - Light/medium/heavy impacts, selection vibration
- [x] Text input - Platform-native text editing with keyboard types
- [x] Permissions - Runtime permission handling
- [x] Notifications - Local and push notification support
- [x] Lifecycle - App lifecycle events
- [x] Location - Geolocation services
- [x] Camera - Camera integration
- [x] Background - Background task execution
- [x] Share - Native share sheet
- [x] Storage - File system and preferences
- [x] System UI - Status bar and navigation bar customization
- [x] Accessibility - TalkBack (Android) and VoiceOver (iOS) support with semantic labels

#### Build System
- [x] CLI - drift build/run/clean commands
- [x] Configuration - drift.yaml for app metadata
- [x] Skia integration - Prebuilt binary fetching and source builds

### Partially Implemented

- [ ] Hot reload - Not yet implemented
- [ ] Developer tools - Widget inspector, performance overlay

### Roadmap to v1.0.0

#### Must Have
- [ ] API stability - Finalize public API surface and deprecation policy
- [ ] Comprehensive documentation - API reference, tutorials, migration guides
- [ ] Testing framework - Widget testing utilities and golden tests
- [ ] Error boundaries - Graceful error handling in widget tree
- [ ] Hot reload - Development-time code reloading

#### Should Have
- [ ] More widgets - Dialog, BottomSheet, Drawer, Snackbar, DataTable, Slider
- [ ] Form validation - Built-in validators and form field error display
- [ ] Internationalization - i18n/l10n support
- [ ] Performance profiling - Built-in performance monitoring tools

#### Nice to Have
- [ ] Widget inspector - Visual debugging tool
- [ ] Adaptive widgets - Platform-aware widget variants
- [ ] Custom painters - User-defined canvas drawing widgets
- [ ] Implicit animations - More animated widget variants
- [ ] Plugin system - Third-party plugin architecture

## Contributing

Contributions are welcome!

## License

Drift is released under the MIT License. See [LICENSE](LICENSE) for details.
