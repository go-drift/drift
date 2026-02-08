package platform

import (
	"fmt"
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
// All methods are safe for concurrent use.
type VideoPlayerController struct {
	view   *VideoPlayerView
	viewID int64

	// OnPlaybackStateChanged is called when the playback state changes.
	// Called on the UI thread.
	// Set this before calling [VideoPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPlaybackStateChanged func(PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	// Called on the UI thread.
	// Set this before calling [VideoPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
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

	videoView, ok := view.(*VideoPlayerView)
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
	return c.viewID
}

// State returns the current playback state.
func (c *VideoPlayerController) State() PlaybackState {
	if c.view != nil {
		return c.view.State()
	}
	return PlaybackStateIdle
}

// Position returns the current playback position.
func (c *VideoPlayerController) Position() time.Duration {
	if c.view != nil {
		return c.view.Position()
	}
	return 0
}

// Duration returns the total media duration.
func (c *VideoPlayerController) Duration() time.Duration {
	if c.view != nil {
		return c.view.Duration()
	}
	return 0
}

// Buffered returns the buffered position.
func (c *VideoPlayerController) Buffered() time.Duration {
	if c.view != nil {
		return c.view.Buffered()
	}
	return 0
}

// Load loads a new media URL, replacing the current media item.
// Call [VideoPlayerController.Play] to start playback.
func (c *VideoPlayerController) Load(url string) error {
	if c.view != nil {
		return c.view.Load(url)
	}
	return nil
}

// Play starts or resumes playback. Call [VideoPlayerController.Load] first
// to set the media URL.
func (c *VideoPlayerController) Play() error {
	if c.view != nil {
		return c.view.Play()
	}
	return nil
}

// Pause pauses playback.
func (c *VideoPlayerController) Pause() error {
	if c.view != nil {
		return c.view.Pause()
	}
	return nil
}

// Stop stops playback and resets the player to the idle state.
func (c *VideoPlayerController) Stop() error {
	if c.view != nil {
		return c.view.Stop()
	}
	return nil
}

// SeekTo seeks to the given position.
func (c *VideoPlayerController) SeekTo(position time.Duration) error {
	if c.view != nil {
		return c.view.SeekTo(position)
	}
	return nil
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (c *VideoPlayerController) SetVolume(volume float64) error {
	if c.view != nil {
		return c.view.SetVolume(volume)
	}
	return nil
}

// SetLooping sets whether playback should loop.
func (c *VideoPlayerController) SetLooping(looping bool) error {
	if c.view != nil {
		return c.view.SetLooping(looping)
	}
	return nil
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (c *VideoPlayerController) SetPlaybackSpeed(rate float64) error {
	if c.view != nil {
		return c.view.SetPlaybackSpeed(rate)
	}
	return nil
}

// Dispose releases the video player and its native resources. After disposal,
// this controller must not be reused.
func (c *VideoPlayerController) Dispose() {
	if c.viewID != 0 {
		GetPlatformViewRegistry().Dispose(c.viewID)
		c.view = nil
		c.viewID = 0
	}
}
