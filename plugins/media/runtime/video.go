package runtime

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// videoViewType must match the type the native halves register
// (plugin/ios/DriftMediaPlugin.swift, plugin/android/DriftMediaPlugin.kt).
const videoViewType = "drift/media/video"

func init() {
	platform.GetPlatformViewRegistry().RegisterFactory(videoViewFactory{})
}

// VideoPlayerController controls a native video player (ExoPlayer on
// Android, AVPlayer on iOS) shown by a [VideoPlayer] widget. It creates the
// native view at once, so its methods and callbacks work right after
// construction; manage its lifetime with core.UseDisposable.
//
// Set the callbacks before Load so no event is missed. They run on the UI
// thread.
type VideoPlayerController struct {
	mu   sync.RWMutex
	view *videoView // nil once disposed, or if creation failed

	state    PlaybackState
	position time.Duration
	duration time.Duration
	buffered time.Duration

	// OnPlaybackStateChanged is called when the playback state changes.
	OnPlaybackStateChanged func(PlaybackState)
	// OnPositionChanged is called about every 250ms while media is loaded.
	OnPositionChanged func(position, duration, buffered time.Duration)
	// OnError is called on a playback error; code is one of the ErrCode
	// constants.
	OnError func(code, message string)
}

// NewVideoPlayerController creates a controller and its native player.
func NewVideoPlayerController() *VideoPlayerController {
	c := &VideoPlayerController{}
	view, err := platform.GetPlatformViewRegistry().Create(videoViewType, map[string]any{})
	if err != nil {
		errors.Report(&errors.DriftError{
			Op:  "media.NewVideoPlayerController",
			Err: fmt.Errorf("create the video view: %w", err),
		})
		return c
	}
	v := view.(*videoView)
	v.mu.Lock()
	v.controller = c
	v.mu.Unlock()
	c.view = v
	return c
}

// ViewID returns the native view's ID, 0 once disposed.
func (c *VideoPlayerController) ViewID() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.view == nil {
		return 0
	}
	return c.view.ViewID()
}

// State returns the current playback state.
func (c *VideoPlayerController) State() PlaybackState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Position returns the current playback position.
func (c *VideoPlayerController) Position() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.position
}

// Duration returns the media's duration.
func (c *VideoPlayerController) Duration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.duration
}

// Buffered returns how far the media is buffered.
func (c *VideoPlayerController) Buffered() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffered
}

// Load loads url for playback; Play starts it.
func (c *VideoPlayerController) Load(url string) error {
	return c.invoke("load", map[string]any{"url": url})
}

// Play starts or resumes playback.
func (c *VideoPlayerController) Play() error { return c.invoke("play", nil) }

// Pause pauses playback.
func (c *VideoPlayerController) Pause() error { return c.invoke("pause", nil) }

// Stop stops playback and returns to the start; the media stays loaded.
func (c *VideoPlayerController) Stop() error { return c.invoke("stop", nil) }

// SeekTo seeks to position.
func (c *VideoPlayerController) SeekTo(position time.Duration) error {
	return c.invoke("seekTo", map[string]any{"positionMs": position.Milliseconds()})
}

// SetVolume sets the volume, from 0 to 1.
func (c *VideoPlayerController) SetVolume(volume float64) error {
	return c.invoke("setVolume", map[string]any{"volume": volume})
}

// SetLooping sets whether playback restarts at the end.
func (c *VideoPlayerController) SetLooping(looping bool) error {
	return c.invoke("setLooping", map[string]any{"looping": looping})
}

// SetPlaybackSpeed sets the playback rate (1 is normal speed).
func (c *VideoPlayerController) SetPlaybackSpeed(rate float64) error {
	return c.invoke("setPlaybackSpeed", map[string]any{"rate": rate})
}

// SetShowControls shows or hides the native controls.
func (c *VideoPlayerController) SetShowControls(show bool) error {
	return c.invoke("setShowControls", map[string]any{"show": show})
}

// Dispose releases the native player. Later calls return
// platform.ErrDisposed; Dispose itself is idempotent.
func (c *VideoPlayerController) Dispose() {
	c.mu.Lock()
	view := c.view
	c.view = nil
	c.mu.Unlock()
	if view != nil {
		platform.GetPlatformViewRegistry().Dispose(view.ViewID())
	}
}

func (c *VideoPlayerController) invoke(method string, args map[string]any) error {
	id := c.ViewID()
	if id == 0 {
		return platform.ErrDisposed
	}
	if _, err := platform.GetPlatformViewRegistry().InvokeViewMethod(id, method, args); err != nil {
		return fmt.Errorf("media video %s: %w", method, err)
	}
	return nil
}

// videoView is the Go side of the native video view.
type videoView struct {
	platform.PlatformViewBase
	mu         sync.RWMutex
	controller *VideoPlayerController
}

type videoViewFactory struct{}

func (videoViewFactory) ViewType() string { return videoViewType }

func (videoViewFactory) Create(viewID int64, _ map[string]any) (platform.PlatformView, error) {
	return &videoView{PlatformViewBase: platform.NewPlatformViewBase(viewID, videoViewType)}, nil
}

func (v *videoView) Create(map[string]any) error { return nil }

// Dispose implements platform.PlatformView; the registry disposes the
// native player.
func (v *videoView) Dispose() {}

// HandleEvent implements platform.PlatformView: playback events from the
// native player.
func (v *videoView) HandleEvent(method string, args map[string]any) error {
	v.mu.RLock()
	c := v.controller
	v.mu.RUnlock()
	if c == nil {
		return nil
	}
	switch method {
	case "onPlaybackStateChanged":
		state, ok := intArg(args, "state")
		if !ok {
			return fmt.Errorf("%s: state %v is not an integer", method, args["state"])
		}
		c.mu.Lock()
		c.state = PlaybackState(state)
		cb := c.OnPlaybackStateChanged
		c.mu.Unlock()
		if cb != nil {
			platform.Dispatch(func() { cb(PlaybackState(state)) })
		}
	case "onPositionChanged":
		var ms [3]int64
		for i, key := range []string{"positionMs", "durationMs", "bufferedMs"} {
			value, ok := intArg(args, key)
			if !ok {
				return fmt.Errorf("%s: %s %v is not an integer", method, key, args[key])
			}
			ms[i] = value
		}
		position := time.Duration(ms[0]) * time.Millisecond
		duration := time.Duration(ms[1]) * time.Millisecond
		buffered := time.Duration(ms[2]) * time.Millisecond
		c.mu.Lock()
		c.position, c.duration, c.buffered = position, duration, buffered
		cb := c.OnPositionChanged
		c.mu.Unlock()
		if cb != nil {
			platform.Dispatch(func() { cb(position, duration, buffered) })
		}
	case "onVideoError":
		code, ok1 := stringArg(args, "code")
		message, ok2 := stringArg(args, "message")
		if !ok1 || !ok2 {
			return fmt.Errorf("%s: want string code and message, got %v", method, args)
		}
		c.mu.RLock()
		cb := c.OnError
		c.mu.RUnlock()
		if cb != nil {
			platform.Dispatch(func() { cb(code, message) })
		}
	default:
		return fmt.Errorf("%s: %w", method, platform.ErrMethodNotFound)
	}
	return nil
}
