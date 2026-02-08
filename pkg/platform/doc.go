// Package platform provides global singletons for platform services.
//
// It assumes a single application per process. Platform services are
// initialized during package init and activated when [SetNativeBridge]
// is called by the bridge package.
//
// # Global Services
//
// The package exposes singleton services for platform capabilities:
// [Lifecycle], [SafeArea], [Accessibility], [Haptics], [Clipboard], etc.
// These are safe for concurrent use from any goroutine.
//
// # Layer Boundary Types
//
// This package defines its own [EdgeInsets] struct as a simple 4-field
// data carrier from native code. This is intentionally separate from
// [layout.EdgeInsets], which is the canonical app-facing type with rich
// helper methods (Horizontal, Vertical, Add, etc.). Conversion between
// the two occurs at the layer boundary in the widgets package (e.g.,
// the SafeArea widget).
//
// Similarly, the PointerEvent/PointerPhase types in the engine package
// represent raw device-pixel input from embedders, while the gestures
// package defines its own processed versions with logical coordinates.
// These duplications are intentional: collapsing them would couple
// embedder APIs to framework internals.
//
// # Media Controllers
//
// AudioPlayerController and VideoPlayerController share an identical method
// set (Load, Play, Pause, Stop, SeekTo, SetVolume, SetLooping,
// SetPlaybackSpeed, State, Position, Duration, Buffered, Dispose) and
// callback fields (OnPlaybackStateChanged, OnPositionChanged, OnError).
// A shared interface is intentionally omitted today because there are no
// consumers that need to abstract over both types. If generic media
// components (seek bar, notification manager) are added later, extract a
// MediaController interface at that point rather than forcing one
// prematurely.
package platform
