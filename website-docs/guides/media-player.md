---
id: media-player
title: Media Player
sidebar_position: 12
---

# Media Player

Drift provides native media playback through two APIs: the `VideoPlayer` widget for embedded video with platform controls, and the `AudioPlayerController` for headless audio playback with a custom UI.

## Video Player

The `VideoPlayer` widget embeds a native video player (ExoPlayer on Android, AVPlayer on iOS) with built-in transport controls including play/pause, seek bar, and time display.

```go
import "github.com/go-drift/drift/pkg/widgets"

widgets.VideoPlayer{
    URL:      "https://example.com/video.mp4",
    AutoPlay: false,
    Width:    0,   // 0 = expand to fill available width
    Height:   225,
}
```

### VideoPlayer Fields

| Field | Type | Description |
|-------|------|-------------|
| `URL` | `string` | Media URL to play |
| `Controller` | `*VideoPlayerController` | Programmatic playback control |
| `AutoPlay` | `bool` | Start playback automatically when the view is created |
| `Looping` | `bool` | Restart playback when it reaches the end |
| `Volume` | `*float64` | Playback volume (0.0 to 1.0). Nil uses platform default (1.0) |
| `Width` | `float64` | Player width in logical pixels (0 = expand to fill) |
| `Height` | `float64` | Player height in logical pixels (0 = 200 default) |
| `OnPlaybackStateChanged` | `func(PlaybackState)` | Called when playback state changes |
| `OnPositionChanged` | `func(positionMs, durationMs, bufferedMs int64)` | Called when playback position updates |
| `OnError` | `func(code, message string)` | Called when a playback error occurs |

### Controller

Use `VideoPlayerController` for programmatic control. Create the controller once, pass it to the widget, and call methods from event handlers:

```go
import (
    "github.com/go-drift/drift/pkg/platform"
    "github.com/go-drift/drift/pkg/widgets"
)

type playerState struct {
    core.StateBase
    controller *widgets.VideoPlayerController
    status     *core.ManagedState[string]
}

func (s *playerState) InitState() {
    s.controller = &widgets.VideoPlayerController{}
    s.status = core.NewManagedState(&s.StateBase, "Idle")
}

func (s *playerState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.VideoPlayer{
                URL:        "https://example.com/video.mp4",
                Controller: s.controller,
                Height:     225,
                OnPlaybackStateChanged: func(state platform.PlaybackState) {
                    drift.Dispatch(func() {
                        s.status.Set(stateLabel(state))
                    })
                },
            },
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Play()
                    }),
                    theme.ButtonOf(ctx, "Pause", func() {
                        s.controller.Pause()
                    }),
                },
            },
        },
    }
}
```

### Controller Methods

All methods are safe for concurrent use. Methods are no-ops before the widget is first painted or after it is disposed.

| Method | Description |
|--------|-------------|
| `Play()` | Start or resume playback |
| `Pause()` | Pause playback |
| `SeekTo(positionMs int64)` | Seek to a position in milliseconds |
| `SetVolume(volume float64)` | Set volume (0.0 to 1.0) |
| `SetLooping(looping bool)` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64)` | Set playback speed (1.0 = normal) |
| `State() PlaybackState` | Current playback state |
| `PositionMs() int64` | Current position in milliseconds |
| `DurationMs() int64` | Total duration in milliseconds |
| `BufferedMs() int64` | Buffered position in milliseconds |

### Dynamic Property Updates

When widget properties change between rebuilds, the native player is updated automatically. URL changes load the new media, and looping/volume changes are applied immediately:

```go
// Changing the URL triggers a native loadUrl call
widgets.VideoPlayer{
    URL:     s.currentURL.Get(), // rebuild with a new URL to switch videos
    Looping: s.loopEnabled.Get(),
}
```

## Audio Player

`AudioPlayerController` provides audio playback without a visual component. It uses a standalone platform channel, so there is no embedded native view. Build your own UI around the controller.

Only one `AudioPlayerController` may exist at a time. Creating a second before disposing the first will panic.

```go
import "github.com/go-drift/drift/pkg/platform"

type audioState struct {
    core.StateBase
    controller   *platform.AudioPlayerController
    status       *core.ManagedState[string]
    unsubscribes []func()
}

func (s *audioState) InitState() {
    s.status = core.NewManagedState(&s.StateBase, "Idle")
    s.controller = platform.NewAudioPlayerController()

    // Subscribe to state updates
    unsub := s.controller.States().Listen(func(state platform.AudioPlayerState) {
        drift.Dispatch(func() {
            s.status.Set(formatAudioStatus(state))
        })
    })
    s.unsubscribes = append(s.unsubscribes, unsub)

    // Subscribe to errors
    errUnsub := s.controller.Errors().Listen(func(err platform.AudioPlayerError) {
        drift.Dispatch(func() {
            s.status.Set("Error: " + err.Message)
        })
    })
    s.unsubscribes = append(s.unsubscribes, errUnsub)

    // Clean up on dispose
    s.OnDispose(func() {
        for _, unsub := range s.unsubscribes {
            unsub()
        }
        s.controller.Dispose()
    })
}
```

### AudioPlayerController Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `Load(url string)` | `error` | Load a media URL for playback |
| `Play()` | `error` | Start or resume playback |
| `Pause()` | `error` | Pause playback |
| `Stop()` | `error` | Stop playback and reset to idle (reusable after calling Load again) |
| `SeekTo(positionMs int64)` | `error` | Seek to a position in milliseconds |
| `SetVolume(volume float64)` | `error` | Set volume (0.0 to 1.0) |
| `SetLooping(looping bool)` | `error` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64)` | `error` | Set playback speed (1.0 = normal) |
| `States()` | `*Stream[AudioPlayerState]` | Stream of playback state updates |
| `Errors()` | `*Stream[AudioPlayerError]` | Stream of playback errors |
| `Dispose()` | `error` | Release native resources (required before creating a new controller) |

### AudioPlayerState Fields

Each state update contains the current playback state and timing information:

| Field | Type | Description |
|-------|------|-------------|
| `PlaybackState` | `PlaybackState` | Current playback state |
| `PositionMs` | `int64` | Current playback position in milliseconds |
| `DurationMs` | `int64` | Total media duration in milliseconds (0 if no media loaded) |
| `BufferedMs` | `int64` | Buffered position in milliseconds |

### Example: Transport Controls

```go
func (s *audioState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: s.status.Get()},
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Load("https://example.com/song.mp3")
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
            // Playback speed controls
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "0.5x", func() {
                        s.controller.SetPlaybackSpeed(0.5)
                    }),
                    theme.ButtonOf(ctx, "1x", func() {
                        s.controller.SetPlaybackSpeed(1.0)
                    }),
                    theme.ButtonOf(ctx, "2x", func() {
                        s.controller.SetPlaybackSpeed(2.0)
                    }),
                },
            },
        },
    }
}
```

## Playback States

Both video and audio players share the same `PlaybackState` enum:

| State | Value | Description |
|-------|-------|-------------|
| `PlaybackStateIdle` | 0 | Player created, no media loaded |
| `PlaybackStateBuffering` | 2 | Buffering media data before playback can continue |
| `PlaybackStatePlaying` | 3 | Actively playing media |
| `PlaybackStateCompleted` | 4 | Playback reached the end of the media |
| `PlaybackStatePaused` | 6 | Paused, can be resumed |

:::note Reserved States
`PlaybackStateLoading` (1) and `PlaybackStateError` (5) are reserved for future use. Errors are delivered through separate callbacks (`OnError` for video, `Errors()` stream for audio) rather than as a playback state.
:::

## Thread Safety

`VideoPlayerController` methods are safe to call from any goroutine. When updating UI state from playback callbacks, wrap calls with `drift.Dispatch`:

```go
OnPlaybackStateChanged: func(state platform.PlaybackState) {
    drift.Dispatch(func() {
        s.status.Set(stateLabel(state))
    })
},
```

The same applies to `AudioPlayerController` stream listeners:

```go
s.controller.States().Listen(func(state platform.AudioPlayerState) {
    drift.Dispatch(func() {
        s.updateUI(state)
    })
})
```

## Next Steps

- [Platform Services](/docs/guides/platform) - Permissions, clipboard, haptics, and other platform APIs
- [Widgets](/docs/guides/widgets) - Built-in widget catalog
- [State Management](/docs/guides/state-management) - Managing widget state
