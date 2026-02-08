package platform

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

// VideoPlayerController provides video playback control with a native visual
// surface (ExoPlayer on Android, AVPlayer on iOS). The controller creates
// its platform view eagerly, so methods and callbacks work immediately
// after construction.
//
// Create with [NewVideoPlayerController] and manage lifecycle with
// [core.UseController]:
//
//	s.video = core.UseController(&s.StateBase, platform.NewVideoPlayerController)
//	s.video.OnPlaybackStateChanged = func(state platform.PlaybackState) { ... }
//	s.video.Load(url)
//
// Pass the controller to a [widgets.VideoPlayer] widget to embed the native
// surface in the widget tree.
//
// Set callback fields (OnPlaybackStateChanged, OnPositionChanged, OnError)
// before calling [VideoPlayerController.Load] or any other playback method
// to ensure no events are missed.
//
// All methods are safe for concurrent use. Callback fields should be set
// on the UI thread (e.g. in InitState or a UseController callback). Setting
// them before calling Load ensures no events are missed.
type VideoPlayerController struct {
	mu     sync.RWMutex
	view   *videoPlayerView // guarded by mu
	viewID int64            // guarded by mu

	// OnPlaybackStateChanged is called when the playback state changes.
	// Called on the UI thread.
	// Set this before calling [VideoPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPlaybackStateChanged func(PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	// The native platform fires this callback approximately every 250ms
	// while media is loaded.
	// Called on the UI thread.
	// Set this before calling [VideoPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	// The code parameter is one of [ErrCodeSourceError],
	// [ErrCodeDecoderError], or [ErrCodePlaybackFailed].
	// Called on the UI thread.
	// Set this before calling [VideoPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnError func(code, message string)
}

// NewVideoPlayerController creates a new video player controller.
// The underlying platform view is created eagerly so methods and callbacks
// work immediately.
func NewVideoPlayerController() *VideoPlayerController {
	c := &VideoPlayerController{}

	view, err := GetPlatformViewRegistry().Create("video_player", map[string]any{})
	if err != nil {
		errors.Report(&errors.DriftError{
			Op:  "NewVideoPlayerController",
			Err: fmt.Errorf("failed to create video player view: %w", err),
		})
		return c
	}

	videoView, ok := view.(*videoPlayerView)
	if !ok {
		errors.Report(&errors.DriftError{
			Op:  "NewVideoPlayerController",
			Err: fmt.Errorf("unexpected view type: %T", view),
		})
		return c
	}

	c.view = videoView
	c.viewID = videoView.ViewID()

	// Wire view callbacks to controller callback fields.
	videoView.OnPlaybackStateChanged = func(state PlaybackState) {
		if c.OnPlaybackStateChanged != nil {
			c.OnPlaybackStateChanged(state)
		}
	}
	videoView.OnPositionChanged = func(position, duration, buffered time.Duration) {
		if c.OnPositionChanged != nil {
			c.OnPositionChanged(position, duration, buffered)
		}
	}
	videoView.OnError = func(code, message string) {
		if c.OnError != nil {
			c.OnError(code, message)
		}
	}

	return c
}

// ViewID returns the platform view ID, or 0 if the view was not created.
func (c *VideoPlayerController) ViewID() int64 {
	c.mu.RLock()
	id := c.viewID
	c.mu.RUnlock()
	return id
}

// State returns the current playback state.
func (c *VideoPlayerController) State() PlaybackState {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.State()
	}
	return PlaybackStateIdle
}

// Position returns the current playback position.
func (c *VideoPlayerController) Position() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Position()
	}
	return 0
}

// Duration returns the total media duration.
func (c *VideoPlayerController) Duration() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Duration()
	}
	return 0
}

// Buffered returns the buffered position.
func (c *VideoPlayerController) Buffered() time.Duration {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v != nil {
		return v.Buffered()
	}
	return 0
}

// Load loads a new media URL, replacing the current media item.
// Call [VideoPlayerController.Play] to start playback.
func (c *VideoPlayerController) Load(url string) error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.Load(url)
}

// Play starts or resumes playback. Call [VideoPlayerController.Load] first
// to set the media URL.
func (c *VideoPlayerController) Play() error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.Play()
}

// Pause pauses playback.
func (c *VideoPlayerController) Pause() error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.Pause()
}

// Stop stops playback and resets the player to the idle state. The loaded
// media is retained, so calling Play will restart playback from the beginning.
// To release native resources, use Dispose instead.
func (c *VideoPlayerController) Stop() error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.Stop()
}

// SeekTo seeks to the given position.
func (c *VideoPlayerController) SeekTo(position time.Duration) error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.SeekTo(position)
}

// SetVolume sets the playback volume (0.0 to 1.0). Values outside this range
// are clamped by the native player.
func (c *VideoPlayerController) SetVolume(volume float64) error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.SetVolume(volume)
}

// SetLooping sets whether playback should loop.
func (c *VideoPlayerController) SetLooping(looping bool) error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.SetLooping(looping)
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal). The rate must be
// positive. Behavior for zero or negative values is platform-dependent.
func (c *VideoPlayerController) SetPlaybackSpeed(rate float64) error {
	c.mu.RLock()
	v := c.view
	c.mu.RUnlock()
	if v == nil {
		return ErrDisposed
	}
	return v.SetPlaybackSpeed(rate)
}

// Dispose releases the video player and its native resources. After disposal,
// this controller must not be reused. Dispose is idempotent; calling it more
// than once is safe.
func (c *VideoPlayerController) Dispose() {
	c.mu.Lock()
	id := c.viewID
	c.view = nil
	c.viewID = 0
	c.mu.Unlock()
	if id != 0 {
		GetPlatformViewRegistry().Dispose(id)
	}
}
