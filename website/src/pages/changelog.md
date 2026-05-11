---
title: Changelog
description: Release history for Drift
---

# Changelog

All notable changes to Drift are documented here. Patch releases are grouped under their parent minor version.

---

## v0.26.0

- **Network images**: `Image` widget loads from `http://` and `https://` URLs with in-memory LRU and on-disk caching
- **Material 3 color schemes**: HCT color space and seed-based color scheme generation for dynamic theming
- **Platform channels**: Method channels now thread `context.Context` through native calls for cancellation and deadline propagation
- **Permissions**: Consolidated platform permission implementations behind a shared core with expanded test coverage

---

## v0.25.x

### v0.25.0

- **Reactive primitives renamed**: `Observable`, `DerivedObservable`, and `ChangeNotifier` renamed to `Signal`, `Derived`, and `Notifier` for clarity and consistency

### v0.25.1

- Added `ListenableBuilder` convenience widget

---

## v0.24.0

- **App lifecycle callbacks**: `OnInit` and `OnDispose` callbacks for app-level setup and teardown
- **New hooks**: `UseSubscription` for stream subscriptions; `UseLifecycleObserver` for observing app lifecycle events
- **Fix**: `DidChangeDependencies` now called on initial mount

---

## v0.23.x

### v0.23.0

- **Android minimum SDK lowered**: Minimum Android SDK reduced from API 31 to API 29

### v0.23.1 to v0.23.4

- Added `DerivedObservable` and `GlobalKey`
- Added `LayoutIssue` error type for developer-friendly error screens
- Added `AllowsOverscroll` method to `ScrollPhysics` interface
- `OnEditingComplete` callback now receives the current text value
- Fixed unbounded constraint propagation to non-flex children

---

## v0.22.x

### v0.22.0

- **Vulkan rendering backend**: Android rendering migrated from OpenGL/EGL to Vulkan ([#34](https://github.com/go-drift/drift/pull/34))
- **Fade transition opacity**: Navigation fade transitions now support opacity
- **Performance**: Replaced reflection-based dirty checks with typed slot comparison; map-based dirty element dedup in `BuildOwner`; generics constrained to `comparable`
- **Refactoring**: Extracted `RenderObjectBase` and `StateBase` to reduce widget boilerplate; hoisted shared render and element tree helpers; consolidated type conversion helpers; simplified `Center` as stateless wrapper around `Align`
- **Skia bridge**: Moved shared bridge implementation to a dedicated compilation unit
- **Fixes**: Fixed navigation resource cleanup for routes and tab navigators; fixed transition disposal; fixed base64 decoding in storage handler; fixed SVG icon cache race on concurrent hit; fixed overlay rebuild after pending ops

### v0.22.1

- CLI warns when its version mismatches the project's drift dependency
- Replaced JSON frame snapshots with packed binary encoding for better performance
- Batched display list replay into single CGO calls

---

## v0.21.x

### v0.21.0

- **Platform view occlusion**: Occlusion region support for native view clipping ([#30](https://github.com/go-drift/drift/pull/30))
- **Navigation**: `TabScaffold` renamed to `TabNavigator`; `RouteConfig` and `ShellRoute` unified into `ScreenRoute`

### v0.21.1 to v0.21.3

- Added builder methods to `TextField` and `TextFormField` with expanded docs
- Converted `SvgIcon` from custom element to stateless widget
- Fixed occlusion emission for semi-transparent backgrounds

---

## v0.20.x

### v0.20.0

The v0.20.0 release focuses on API and developer experience improvements, reducing boilerplate across the framework.

- **Simplified state API**: `StatelessBase` and `StatefulBase` eliminate widget boilerplate
- **Builder-pattern APIs**: `Positioned` and other widgets now use a fluent builder pattern
- **Removed helpers**: `ColumnOf`/`RowOf` convenience helpers removed in favor of explicit children
- **Simplified widget setup**: Progress indicators and Lottie animations use streamlined constructors
- **Navigation**: Added `SimpleBuilder` helper for routes that ignore settings
- **Theme accessors**: Granular theme accessors replace verbose lookups

### v0.20.1

- Optimized frame scheduling and Metal presentation sync
- Fixed bare code fences in generated documentation

---

## v0.14.0

- **Hot-reload watch mode**: `drift run` now watches for changes and hot-reloads, with unified device management ([#25](https://github.com/go-drift/drift/pull/25))
- **Platform view pre-warming**: Expensive native views are pre-warmed at startup to reduce first-frame latency

---

## v0.13.0

- **RichText widget**: Styled span tree rendering for mixed-style text ([#28](https://github.com/go-drift/drift/pull/28))
- **Lottie animations**: Lottie support via Skia's Skottie module ([#27](https://github.com/go-drift/drift/pull/27))
- **Android SurfaceControl**: Replaced GLSurfaceView with SurfaceControl and split the frame pipeline ([#26](https://github.com/go-drift/drift/pull/26))
- **Parallax transitions**: New parallax route transition style
- **Navigation**: Pointer interaction is now blocked during route transitions
- **Text wrapping**: `Text` and `RichText` default to wrap-on, using an enum for wrap mode
- **Platform log bridging**: Native log output from Android and iOS is bridged into Drift's logger
- **Video controls**: Added `HideControls` option to toggle native transport controls

---

## v0.12.x

### v0.12.0

A large release covering on-demand rendering, platform view hit testing, and many new widgets.

- **On-demand frame scheduling**: Both iOS and Android replaced continuous vsync loops with on-demand rendering
- **Platform view hit testing**: Engine-side hit testing enables touch interception for platform views
- **Dialog widgets**: `ShowDialog` and `ShowAlertDialog` helpers with modal dialog widgets
- **Eject command**: `drift eject` exports platform projects for manual customization ([#3](https://github.com/go-drift/drift/pull/3))
- **URL launcher**: New platform service for opening URLs
- **Preferences service**: Simple key-value storage for user preferences
- **New widgets**: `Divider`, `VerticalDivider`, `Spacer`
- **TextAlign**: Paragraph-level horizontal alignment for text
- **Accessibility**: Improved screen reader support for overlays, radio buttons, and dropdowns
- **Fixes**: Multiline text input defaults, safe area insets before first frame, outer box shadow rendering

### v0.12.1

- Reduced frame scheduling latency and lock contention
- Fixed iOS transition jitter caused by CubicBezier solver instability
- Fixed scroll fling velocity smoothing and overscroll resistance
- Synchronized Metal presentation for rotation and platform views

### v0.12.2

- Raised minimum iOS deployment target from iOS 14 to iOS 16
- Added `drift` CLI generation of app icons and launch screens for iOS and Android
- Fixed multiline text input scroll state on iOS geometry changes

### v0.12.3 / v0.12.4

- Experimental SurfaceControl rendering pipeline for Android (reverted pending stability fixes)

---

## v0.11.x

### v0.11.0

- **Media player**: Audio and video playback support ([#19](https://github.com/go-drift/drift/pull/19))
- **Platform services**: Unified platform services behind consistent interfaces
- **Widget catalog**: New documentation with widget catalog and simplified guides
- **Enum defaults**: Enum zero values now match their default behavior

### v0.11.1

- **LayoutBuilder**: New widget for constraint-aware child building
- **Screen orientation**: Configurable screen orientation support
- **HTTP config**: `allow_http` scaffold config for cleartext HTTP traffic
- Fixed screen rotation handling on both iOS and Android
- Fixed missing Firebase configuration crash on Android

---

## v0.10.x

### v0.10.0

- **Layer tree and display lists**: Core rendering pipeline rewritten with display list recording, compositing, and repaint boundaries ([#14](https://github.com/go-drift/drift/pull/14))
- **Frame tracing**: Runtime diagnostics and frame tracing added to the debug server ([#16](https://github.com/go-drift/drift/pull/16))
- **Init command**: `drift init` now accepts a directory path instead of a bare project name ([#17](https://github.com/go-drift/drift/pull/17))
- **Platform view sync**: Render thread synchronization with native geometry to prevent torn frames
- **Performance**: `batchSetGeometry` made async to eliminate flush stalls

### v0.10.1

- Added code example image to the website homepage
- Fixed iOS simulator Intel arch naming (x64 to amd64)

---

## v0.9.x

### v0.9.0

- **Declarative Router**: Path parameters and route guards ([#4](https://github.com/go-drift/drift/pull/4))
- **Overlay system**: Modals and floating UI ([#6](https://github.com/go-drift/drift/pull/6))
- **Bottom sheet**: Navigation-integrated bottom sheet component ([#9](https://github.com/go-drift/drift/pull/9))
- **Flexible widget**: `FlexFit` support for flex layouts
- **Reproducible builds**: `SKIA_REV` environment variable pins the Skia version
- **CI**: Added GitHub Actions workflow for vet and tests
- Fixed hit testing through `Align` and `Center` widgets
- Fixed alignment-positioned children sizing in `StackFitExpand`

### v0.9.1

- SVG icon caching and optimized invalidation
- Fixed bottom sheet drag handle region
- Windows NDK host tag detection
- Fixed Android back button frame loop wake

### v0.9.2

- Fixed `gradlew` path construction on Windows

---

## v0.8.0

- **Breaking**: Renamed `ChildWidget`/`ChildrenWidgets` to `Child`/`Children` across all widgets

---

## v0.7.x

### v0.7.0

**Rendering**
- Skia paragraph API integration for text layout
- Native SVG rendering via Skia's SVG DOM (replacing oksvg)
- `DrawImageRect` with filter quality and image caching
- `ClipPath`, `SaveLayer`, `ColorFilter`, and `ImageFilter` support
- Extended paint properties for stroke styling and compositing
- Box shadows default to outer blur; inner shadow rendering fixed
- Shader warmup for reduced first-frame stutter

**Widgets**
- `Wrap` widget for flow layout with wrapping
- `Align` widget and border support for `Container`
- Gradient border support for `Container` and `DecoratedBox`
- `Offstage` widget to skip paint for hidden routes
- `TextFormField` theme support and fluent API
- Standardized widget construction API for v1.0.0
- Relative alignment coordinates for gradients and positioning
- Normalized 0 to 1 alpha values for the color API

**Performance**
- Child culling outside clip bounds during paint
- Skip layout for inactive `IndexedStack` children
- Skip child layout in `Offstage` when hidden
- Repaint boundaries on `Image` and `SvgImage`

**Developer Tools**
- Widget testing framework with `Text`, `Padding`, `Container`, and `Button` tests
- HTTP debug server for render tree inspection
- Widget-tree endpoint on the debug server
- Layout bounds debug overlay

**Fixes**
- Loose constraints for `Container` children with explicit dimensions
- Unbounded constraint handling in `Align`, `Center`, and `Flex`
- iOS `CLLocationManager` deadlock on dedicated thread
- iOS display link restart after modal dismissal

### v0.7.1 to v0.7.5

- Bundled `libc++_shared.so` in Android release builds to prevent NDK ABI mismatch
- NDK toolchain fallback for Apple Silicon Macs
- CLI version read from Go embedded build info
- Triple-buffering on Android to prevent flickering on physical devices
- Disabled outline atomics for older NDK compatibility

---

## v0.6.0

- **Platform views**: Synchronized batch geometry updates, clip bounds support, and visibility culling during scroll
- **TextInput migration**: `TextInput` migrated to the platform view system; added native `Switch`
- **TextFormField**: New form-aware text field widget
- **Progress indicators**: Linear and circular progress indicators
- **Date/time pickers**: Native date and time picker widgets
- **Diagnostics HUD**: FPS counter and frame graph overlay
- **Font weight fix**: Normalized font weight to prevent thin text on Android
- **Forms guide**: New documentation guide covering validation, selection controls, and pickers

---

## v0.5.0

- **Documentation website**: Launched Docusaurus-based docs at driftframework.dev
- **Error boundaries**: Panic recovery with error boundary widgets
- **Slot-based reconciliation**: Keyed reconciliation for the render tree
- **`InheritedProvider<T>`**: Generic zero-boilerplate inherited providers
- **Permissions API**: Unified permissions with blocking requests and context support
- **Secure storage**: Biometric-authenticated secure storage
- **Custom Skia path**: `DRIFT_SKIA_DIR` environment variable for custom Skia library locations

---

## v0.4.0

- **Repaint boundaries**: Optimized paint propagation with repaint boundary tracking
- **Relayout boundaries**: Optimized layout propagation with relayout boundary tracking
- **Semantics dirty tracking**: Accessibility semantics only rebuild when changed
- **Deferred semantics flushes**: Semantics updates deferred during animations
- **Aspect-based dependency tracking**: Granular inherited widget subscriptions
- **SafeAreaProvider**: Scoped rebuilds with batched safe area updates
- **Box shadows and backdrop blur**: Visual effects for decorated boxes
- **Text shadows**: Shadow support on text rendering
- `TextStyle.WithColor()`, `PushReplacementNamed`
- Reduced redundant keyboard hide calls and render cycles on Android

---

## v0.3.0

- **`drift fetch-skia`**: New CLI command with auto-fetch on build
- **Versioned cache**: Cache directories scoped by version with configurable root
- **Log filtering**: PID-based Android log filtering and `os_log` iOS filtering
- Fixed iOS accessibility files missing from Xcode project template
- Fixed iOS scheduled notification time interval trigger
- Release script confirmation prompt before pushing tags

---

## v0.2.0

- **Accessibility**: TalkBack and VoiceOver support with semantic tree
- **Theming**: Material Design 3 and Cupertino theme support with unified `AppTheme` provider
- **Gestures**: Drag gesture recognizers with axis-locked drag and arena hold mechanism
- **Positioned widget**: Layout support within `Stack`
- **Multi-touch**: Unique pointer ID tracking for multi-touch support
- **Navigation**: Scroll position preserved across route transitions
- **Keyboard navigation**: Focus target set when keyboard navigates between text fields
- **Performance**: O(1) dirty object lookup using maps in the layout pipeline
- Fixed bundle ID segment sanitization, platform channel names, and Android native library loading

---

## v0.1.0

Initial release of Drift with core framework functionality:
- Widget tree with stateless and stateful widgets
- Flex layout engine (Row, Column, Stack)
- Skia-based rendering via CGO
- Android (Kotlin) and iOS (Swift) embedders
- Navigation with named routes
- Text input and basic form widgets
- Scroll views
- Image and SVG rendering
- Platform channels for native communication
- `drift` CLI for init, build, run, and clean
