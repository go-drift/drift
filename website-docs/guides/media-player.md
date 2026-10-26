---
id: media-player
title: Media Player
sidebar_position: 2
---

# Media Player

The `media` plugin provides native media playback. Add it to `drift.yaml`:

```yaml
plugins:
  - package: github.com/go-drift/drift/plugins/media/plugin
```

and import its Go API as `media "github.com/go-drift/drift/plugins/media/runtime"`. The [plugin's section](/docs/guides/plugin-catalog#media) lists its config (DASH on Android).

Up to Drift v0.26 these types were core: `platform.VideoPlayerController`, `platform.AudioPlayerController`, `platform.PlaybackState` and the `widgets.VideoPlayer` widget. In the `media` package the controllers publish playback as state instead of callbacks: `OnPlaybackStateChanged` and `OnPositionChanged` became `State()` and `Progress()`, `OnError` became the `Errors()` stream with the `media.ErrSource`, `ErrDecoder` and `ErrPlaybackFailed` sentinels in place of the `ErrCode*` strings, and `NewVideoPlayerController` returns an error.

The plugin has two APIs: the `VideoPlayerController` for embedded video with platform controls, and the `AudioPlayerController` for headless audio playback with a custom UI. Audio and video controllers share the same method set but are separate types. The video controller creates its native surface eagerly on construction, while the audio controller defers native player creation until the first method call.

Each controller publishes:

- `State()`, a `*core.Derived[media.PlaybackState]`, and `Progress()`, a `*core.Derived[media.Progress]` (position, duration and buffered length, about every 250ms while media is loaded). Both change on the UI thread, so `core.UseListenable` rebuilds a widget on them.
- `Errors()`, a `*platform.Stream[error]` of playback failures. Like every stream, its handlers run off the UI thread: update widget state inside `drift.Dispatch`.

## Video Player

The `VideoPlayer` widget embeds a native video player (ExoPlayer on Android, AVPlayer on iOS) with built-in transport controls including play/pause, seek bar, and time display.

Create a `VideoPlayerController`, subscribe to what the widget shows, and pass it to the widget:

```go
import (
    "log"
    "time"

    "github.com/go-drift/drift/pkg/core"
    "github.com/go-drift/drift/pkg/drift"
    "github.com/go-drift/drift/pkg/platform"
    "github.com/go-drift/drift/pkg/theme"
    "github.com/go-drift/drift/pkg/widgets"
    media "github.com/go-drift/drift/plugins/media/runtime"
)

type playerState struct {
    core.StateBase
    controller *media.VideoPlayerController
    failure    *core.Signal[string]
}

func (s *playerState) InitState() {
    controller, err := media.NewVideoPlayerController()
    if err != nil {
        log.Printf("video: %v", err) // the media plugin is missing, say
        return
    }
    s.controller = controller
    core.UseDisposable(&s.StateBase, controller)
    core.UseListenable(&s.StateBase, controller.State())
    core.UseListenable(&s.StateBase, controller.Progress())

    s.failure = core.NewSignal("")
    core.UseListenable(&s.StateBase, s.failure)
    s.OnDispose(controller.Errors().Listen(platform.StreamHandler[error]{
        OnEvent: func(err error) {
            drift.Dispatch(func() { s.failure.Set(err.Error()) })
        },
    }))

    controller.Load("https://example.com/video.mp4")
}

func (s *playerState) Build(ctx core.BuildContext) core.Widget {
    if s.controller == nil {
        return widgets.Text{Content: "Video unavailable"}
    }
    progress := s.controller.Progress().Value()
    return widgets.Column{
        Children: []core.Widget{
            media.VideoPlayer{
                Controller: s.controller,
                Height:     225,
            },
            widgets.Text{Content: s.controller.State().Value().String() + " " +
                progress.Position.String() + " / " + progress.Duration.String() + " " + s.failure.Value()},
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Pause", func() {
                        s.controller.Pause()
                    }),
                    theme.ButtonOf(ctx, "Seek +10s", func() {
                        s.controller.SeekTo(progress.Position + 10*time.Second)
                    }),
                },
            },
        },
    }
}
```

`NewVideoPlayerController` fails when the native player cannot be created: the media plugin is missing from `drift.yaml`, or the platform could not make the player.

Width and Height set explicit dimensions in logical pixels. To fill available width, wrap the widget in layout widgets such as `Expanded` inside a `Row`:

```go
widgets.Row{
    Children: []core.Widget{
        widgets.Expanded{
            Child: media.VideoPlayer{
                Controller: s.controller,
                Height:     225,
            },
        },
    },
}
```

Subscribe to `Errors()` before calling `Load`: a failure is an event, delivered only to the subscribers present when it happens. `State()` and `Progress()` are state, so their current values are always at hand.

`UseDisposable` registers a dispose callback automatically, so the controller is released when the widget is removed from the tree. Disposing a controller that is still buffering silently cancels playback; nothing changes after that. For non-widget contexts (tests, standalone services), use `media.NewVideoPlayerController()` directly and call `Dispose()` manually.

### VideoPlayer Widget Fields

| Field | Type | Description |
|-------|------|-------------|
| `Controller` | `*media.VideoPlayerController` | The controller that provides the native surface and playback control |
| `Width` | `float64` | Player width in logical pixels |
| `Height` | `float64` | Player height in logical pixels |
| `HideControls` | `bool` | Hide native transport controls. Use when building custom Drift widget controls on top of the video surface. |

### VideoPlayerController Methods

All methods are safe for concurrent use.

| Method | Description |
|--------|-------------|
| `Load(url string) error` | Load a media URL. The native player begins buffering the media source. |
| `Play() error` | Start or resume playback |
| `Pause() error` | Pause playback |
| `Stop() error` | Stop playback and reset to idle. Media stays loaded; calling `Play` restarts from the beginning. Use `Dispose` to release resources. |
| `SeekTo(position time.Duration) error` | Seek to a position |
| `SetVolume(volume float64) error` | Set volume (0.0 to 1.0). Values outside this range are clamped by the native player. |
| `SetLooping(looping bool) error` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64) error` | Set playback speed (1.0 = normal). Must be positive; behavior for zero or negative values is platform-dependent. |
| `SetShowControls(show bool) error` | Show or hide native transport controls at runtime. |
| `State() *core.Derived[PlaybackState]` | Playback state, changing on the UI thread |
| `Progress() *core.Derived[Progress]` | Position, duration and buffered length, changing on the UI thread about every 250ms while media is loaded |
| `Errors() *platform.Stream[error]` | Playback failures, matching `ErrSource`, `ErrDecoder` or `ErrPlaybackFailed` |
| `ViewID() int64` | Platform view ID, or 0 if the view was not created |
| `Dispose()` | Release native resources. Later calls return `platform.ErrDisposed`; idempotent. |

## Audio Player

`AudioPlayerController` provides audio playback without a visual component. It uses a standalone platform channel, so there is no embedded native view. Build your own UI around the controller.

Multiple controllers may exist concurrently, each managing its own native player instance. Call `Dispose` to release resources when a controller is no longer needed.

```go
import (
    "log"
    "time"

    "github.com/go-drift/drift/pkg/core"
    "github.com/go-drift/drift/pkg/platform"
    "github.com/go-drift/drift/pkg/theme"
    "github.com/go-drift/drift/pkg/widgets"
    media "github.com/go-drift/drift/plugins/media/runtime"
)

type audioState struct {
    core.StateBase
    controller *media.AudioPlayerController
}

func (s *audioState) InitState() {
    s.controller = media.NewAudioPlayerController()
    core.UseDisposable(&s.StateBase, s.controller)
    core.UseListenable(&s.StateBase, s.controller.State())
    core.UseListenable(&s.StateBase, s.controller.Progress())
    s.OnDispose(s.controller.Errors().Listen(platform.StreamHandler[error]{
        OnEvent: func(err error) { log.Printf("audio: %v", err) },
    }))

    s.controller.Load("https://example.com/song.mp3")
}
```

Subscribe to `Errors()` before calling `Load`, as for video.

`UseDisposable` registers a dispose callback automatically, so the controller is released when the widget is removed from the tree. Disposing a controller that is still buffering silently cancels playback; nothing changes after that. For non-widget contexts (tests, standalone services), use `media.NewAudioPlayerController()` directly and call `Dispose()` manually.

### AudioPlayerController Methods

All methods are safe for concurrent use.

| Method | Description |
|--------|-------------|
| `Load(url string) error` | Load a media URL. The native player begins buffering the media source. |
| `Play() error` | Start or resume playback |
| `Pause() error` | Pause playback |
| `Stop() error` | Stop playback and reset to idle. Media stays loaded; calling `Play` restarts from the beginning. Use `Dispose` to release resources. |
| `SeekTo(position time.Duration) error` | Seek to a position |
| `SetVolume(volume float64) error` | Set volume (0.0 to 1.0). Values outside this range are clamped by the native player. |
| `SetLooping(looping bool) error` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64) error` | Set playback speed (1.0 = normal). Must be positive; behavior for zero or negative values is platform-dependent. |
| `State() *core.Derived[PlaybackState]` | Playback state, changing on the UI thread |
| `Progress() *core.Derived[Progress]` | Position, duration and buffered length, changing on the UI thread about every 250ms while media is loaded |
| `Errors() *platform.Stream[error]` | Playback failures, matching `ErrSource`, `ErrDecoder` or `ErrPlaybackFailed` |
| `Dispose()` | Release native resources. Later calls return `platform.ErrDisposed`; idempotent. |

### Example: Transport Controls with Seek

```go
func (s *audioState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: s.controller.State().Value().String()},
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Play()
                    }),
                    theme.ButtonOf(ctx, "Pause", func() {
                        s.controller.Pause()
                    }),
                    theme.ButtonOf(ctx, "Stop", func() {
                        s.controller.Stop()
                    }),
                },
            },
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Seek +10s", func() {
                        pos := s.controller.Progress().Value().Position
                        s.controller.SeekTo(pos + 10*time.Second)
                    }),
                    theme.ButtonOf(ctx, "Loop", func() {
                        s.controller.SetLooping(true)
                    }),
                    theme.ButtonOf(ctx, "Mute", func() {
                        s.controller.SetVolume(0)
                    }),
                },
            },
        },
    }
}
```

## Playback States

Both video and audio players share the same `PlaybackState` enum (defined in `media`). Use the `String()` method for human-readable labels.

| State | Value | Description |
|-------|-------|-------------|
| `PlaybackStateIdle` | 0 | Player created, no media loaded |
| `PlaybackStateBuffering` | 1 | Buffering media data before playback can continue |
| `PlaybackStatePlaying` | 2 | Actively playing media |
| `PlaybackStateCompleted` | 3 | Playback reached the end of the media |
| `PlaybackStatePaused` | 4 | Paused, can be resumed |

Failures arrive on `Errors()`, not as a playback state; the player goes idle after one.

## Errors

Control methods like `Load` and `Play` return an `error` that indicates a communication failure with the native player (for example, calling a method after disposal). `Errors()` streams playback-time failures the native player reports, such as network failures or unsupported codecs, the same on Android and iOS:

| Sentinel | Wire code | Meaning |
|----------|-----------|---------|
| `media.ErrSource` | `source_error` | Media source could not be loaded (network failure, invalid URL, unsupported format) |
| `media.ErrDecoder` | `decoder_error` | Media could not be decoded or rendered (codec failure, DRM error) |
| `media.ErrPlaybackFailed` | `playback_failed` | General playback failure that does not fit a more specific category |

```go
controller.Errors().Listen(platform.StreamHandler[error]{OnEvent: func(err error) {
    switch {
    case errors.Is(err, media.ErrSource):
        // Network or URL issue, prompt user to check connection
    case errors.Is(err, media.ErrDecoder):
        // Format not supported on this device
    }
    log.Printf("playback error: %v", err) // includes the platform's message
}})
```

## Custom Controls

To build your own playback controls in Drift widgets, set `HideControls: true` on the `VideoPlayer` widget. This hides the native transport UI (play/pause button, seek bar, time display) and leaves a bare video surface that you can overlay with Drift widgets using `Stack` and `Positioned`:

```go
widgets.Stack{
    Children: []core.Widget{
        media.VideoPlayer{
            Controller:   s.controller,
            Height:       300,
            HideControls: true,
        },
        widgets.Positioned(widgets.Row{
            Children: []core.Widget{
                theme.ButtonOf(ctx, "Play", func() {
                    s.controller.Play()
                }),
                theme.ButtonOf(ctx, "Pause", func() {
                    s.controller.Pause()
                }),
            },
        }).Bottom(8).Left(8),
    },
}
```

You can also toggle controls at runtime via the controller:

```go
s.controller.SetShowControls(false) // hide
s.controller.SetShowControls(true)  // show again
```

## Cleartext HTTP URLs

Android (API 28+) and iOS block cleartext HTTP traffic by default. If you load an `http://` URL, the native player will report a `source_error`. HTTPS URLs work without any extra configuration.

To allow HTTP URLs, set `allow_http: true` in your `drift.yaml`:

```yaml
app:
  allow_http: true
```

This adds `android:usesCleartextTraffic="true"` to the Android manifest and an `NSAppTransportSecurity` exception to the iOS Info.plist. Use HTTPS whenever possible and only enable this setting when your media source does not support it.

:::tip
`UseDisposable` is documented in the [State Management](/docs/guides/state-management#usedisposable) guide.
:::

## Next Steps

- [Platform Services](/docs/guides/platform) - Permissions, clipboard, haptics, and other platform APIs
- [Widget Catalog](/docs/category/widget-catalog) - Detailed usage for every Drift widget
- [State Management](/docs/guides/state-management) - Managing widget state
