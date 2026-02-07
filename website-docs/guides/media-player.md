---
id: media-player
title: Media Player
sidebar_position: 12
---

# Media Player

Drift provides native media playback through two APIs: the `VideoPlayer` widget for embedded video with platform controls, and the `AudioPlayerController` for headless audio playback with a custom UI.

Both APIs deliver callbacks on the UI thread, so you can update widget state directly without wrapping calls in `drift.Dispatch`.

## Video Player

The `VideoPlayer` widget embeds a native video player (ExoPlayer on Android, AVPlayer on iOS) with built-in transport controls including play/pause, seek bar, and time display.

```go
import "github.com/go-drift/drift/pkg/widgets"

widgets.VideoPlayer{
    URL:    "https://example.com/video.mp4",
    Volume: 1.0,
    Height: 225,
}
```

Width and Height set explicit dimensions in logical pixels. To fill available width, wrap the widget in layout widgets such as `Expanded` inside a `Row`:

```go
widgets.Row{
    Children: []core.Widget{
        widgets.Expanded{
            Child: widgets.VideoPlayer{
                URL:    "https://example.com/video.mp4",
                Volume: 1.0,
                Height: 225,
            },
        },
    },
}
```

### VideoPlayer Fields

| Field | Type | Description |
|-------|------|-------------|
| `URL` | `string` | Media URL to play |
| `Controller` | `*VideoPlayerController` | Programmatic playback control |
| `AutoPlay` | `bool` | Start playback automatically when the view is created |
| `Looping` | `bool` | Restart playback when it reaches the end |
| `Volume` | `float64` | Playback volume (0.0 to 1.0). Zero means muted. |
| `Width` | `float64` | Player width in logical pixels |
| `Height` | `float64` | Player height in logical pixels |
| `OnPlaybackStateChanged` | `func(PlaybackState)` | Called when playback state changes (UI thread) |
| `OnPositionChanged` | `func(position, duration, buffered time.Duration)` | Called when playback position updates (UI thread) |
| `OnError` | `func(code, message string)` | Called when a playback error occurs (UI thread) |

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
                Volume:     1.0,
                Height:     225,
                OnPlaybackStateChanged: func(state platform.PlaybackState) {
                    s.status.Set(state.String())
                },
            },
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Play("https://example.com/video.mp4")
                    }),
                    theme.ButtonOf(ctx, "Pause", func() {
                        s.controller.Pause()
                    }),
                    theme.ButtonOf(ctx, "Seek +10s", func() {
                        pos := s.controller.Position()
                        s.controller.SeekTo(pos + 10*time.Second)
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
| `Play(url string)` | Load the URL (if not already loaded) and start playback. Resumes if the same URL is passed after a pause. |
| `Pause()` | Pause playback |
| `Stop()` | Stop playback and reset to idle. A subsequent Play call will reload the URL. |
| `SeekTo(position time.Duration)` | Seek to a position |
| `SetVolume(volume float64)` | Set volume (0.0 to 1.0) |
| `SetLooping(looping bool)` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64)` | Set playback speed (1.0 = normal) |
| `State() PlaybackState` | Current playback state |
| `Position() time.Duration` | Current playback position |
| `Duration() time.Duration` | Total media duration |
| `Buffered() time.Duration` | Buffered position |

### Dynamic Property Updates

When widget properties change between rebuilds, the native player is updated automatically. URL changes load the new media, and looping/volume changes are applied immediately:

```go
// Changing the URL triggers a native loadUrl call
widgets.VideoPlayer{
    URL:     s.currentURL.Get(), // rebuild with a new URL to switch videos
    Looping: s.loopEnabled.Get(),
    Volume:  1.0,
}
```

## Audio Player

`AudioPlayerController` provides audio playback without a visual component. It uses a standalone platform channel, so there is no embedded native view. Build your own UI around the controller.

Multiple controllers may exist concurrently, each managing its own native player instance. Call `Dispose` to release resources when a controller is no longer needed.

```go
import "github.com/go-drift/drift/pkg/platform"

type audioState struct {
    core.StateBase
    controller *platform.AudioPlayerController
    status     *core.ManagedState[string]
}

func (s *audioState) InitState() {
    s.status = core.NewManagedState(&s.StateBase, "Idle")
    s.controller = platform.NewAudioPlayerController()

    // Callbacks are delivered on the UI thread.
    s.controller.OnPlaybackStateChanged = func(state platform.PlaybackState) {
        s.status.Set(state.String())
    }
    s.controller.OnPositionChanged = func(position, duration, buffered time.Duration) {
        s.status.Set(state.String() + " " + position.String() + " / " + duration.String())
    }
    s.controller.OnError = func(code, message string) {
        s.status.Set("Error (" + code + "): " + message)
    }

    s.OnDispose(func() {
        s.controller.Dispose()
    })
}
```

### AudioPlayerController Methods

All methods are safe for concurrent use.

| Method | Description |
|--------|-------------|
| `Play(url string)` | Load the URL (if not already loaded) and start playback. Resumes if the same URL is passed after a pause. |
| `Pause()` | Pause playback |
| `Stop()` | Stop playback and reset to idle. A subsequent Play call will reload the URL. |
| `SeekTo(position time.Duration)` | Seek to a position |
| `SetVolume(volume float64)` | Set volume (0.0 to 1.0) |
| `SetLooping(looping bool)` | Enable or disable looping |
| `SetPlaybackSpeed(rate float64)` | Set playback speed (1.0 = normal) |
| `State() PlaybackState` | Current playback state |
| `Position() time.Duration` | Current playback position |
| `Duration() time.Duration` | Total media duration |
| `Buffered() time.Duration` | Buffered position |
| `Dispose()` | Release native resources. The controller must not be reused after disposal. |

### AudioPlayerController Callbacks

| Field | Type | Description |
|-------|------|-------------|
| `OnPlaybackStateChanged` | `func(PlaybackState)` | Called when playback state changes (UI thread) |
| `OnPositionChanged` | `func(position, duration, buffered time.Duration)` | Called when playback position updates (UI thread) |
| `OnError` | `func(code, message string)` | Called when a playback error occurs (UI thread) |

### Example: Transport Controls with Seek

```go
func (s *audioState) Build(ctx core.BuildContext) core.Widget {
    return widgets.Column{
        Children: []core.Widget{
            widgets.Text{Content: s.status.Get()},
            widgets.Row{
                Children: []core.Widget{
                    theme.ButtonOf(ctx, "Play", func() {
                        s.controller.Play("https://example.com/song.mp3")
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
                        pos := s.controller.Position()
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

Both video and audio players share the same `PlaybackState` enum (defined in `platform`). Use the `String()` method for human-readable labels.

| State | Value | Description |
|-------|-------|-------------|
| `PlaybackStateIdle` | 0 | Player created, no media loaded |
| `PlaybackStateBuffering` | 1 | Buffering media data before playback can continue |
| `PlaybackStatePlaying` | 2 | Actively playing media |
| `PlaybackStateCompleted` | 3 | Playback reached the end of the media |
| `PlaybackStatePaused` | 4 | Paused, can be resumed |

Errors are delivered through the `OnError` callback rather than as a playback state.

## Error Codes

Both players use canonical error codes that are consistent across Android and iOS:

| Code | Constant | Description |
|------|----------|-------------|
| `"source_error"` | `platform.ErrCodeSourceError` | Media source could not be loaded (network failure, invalid URL, unsupported format) |
| `"decoder_error"` | `platform.ErrCodeDecoderError` | Media could not be decoded or rendered (codec failure, DRM error) |
| `"playback_failed"` | `platform.ErrCodePlaybackFailed` | General playback failure that does not fit a more specific category |

```go
controller.OnError = func(code, message string) {
    switch code {
    case platform.ErrCodeSourceError:
        // Network or URL issue, prompt user to check connection
    case platform.ErrCodeDecoderError:
        // Format not supported on this device
    default:
        // General failure
    }
    log.Printf("playback error [%s]: %s", code, message)
}
```

## Next Steps

- [Platform Services](/docs/guides/platform) - Permissions, clipboard, haptics, and other platform APIs
- [Widgets](/docs/guides/widgets) - Built-in widget catalog
- [State Management](/docs/guides/state-management) - Managing widget state
