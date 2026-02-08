package platform

import (
	"sync"
	"time"
)

// VideoPlayerView is a platform view that wraps native video playback
// (ExoPlayer on Android, AVPlayer on iOS). It provides transport controls,
// position/duration tracking, and playback state observation.
//
// Set callback fields (OnPlaybackStateChanged, OnPositionChanged, OnError)
// before calling [VideoPlayerView.Load] or any other playback method to
// ensure no events are missed. Cached state is also available via
// [VideoPlayerView.State], [VideoPlayerView.Position], etc.
type VideoPlayerView struct {
	basePlatformView
	mu sync.RWMutex

	// Cached playback state
	state    PlaybackState
	position time.Duration
	duration time.Duration
	buffered time.Duration

	// OnPlaybackStateChanged is called when the playback state changes.
	// Called on the UI thread via [Dispatch].
	// Set this before calling any playback method to avoid missing events.
	OnPlaybackStateChanged func(PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	// Called on the UI thread via [Dispatch].
	// Set this before calling any playback method to avoid missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	// Called on the UI thread via [Dispatch].
	// Set this before calling any playback method to avoid missing events.
	OnError func(code, message string)
}

// NewVideoPlayerView creates a new video player platform view with the given
// view ID. Set the callback fields to receive playback events.
func NewVideoPlayerView(viewID int64) *VideoPlayerView {
	return &VideoPlayerView{
		basePlatformView: basePlatformView{
			viewID:   viewID,
			viewType: "video_player",
		},
	}
}

// Create implements PlatformView. Video player lifecycle is managed entirely
// by the native side (ExoPlayer/AVPlayer) upon creation via the registry,
// so no additional initialization is needed here.
func (v *VideoPlayerView) Create(params map[string]any) error {
	return nil
}

// Dispose implements PlatformView. Cleanup is handled by the registry's
// Dispose method, which sends the dispose command to the native player.
func (v *VideoPlayerView) Dispose() {}

// Play starts playback.
func (v *VideoPlayerView) Play() error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "play", nil)
	return err
}

// Pause pauses playback.
func (v *VideoPlayerView) Pause() error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "pause", nil)
	return err
}

// Stop stops playback and resets the player to the idle state.
func (v *VideoPlayerView) Stop() error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "stop", nil)
	return err
}

// SeekTo seeks to the given position.
func (v *VideoPlayerView) SeekTo(position time.Duration) error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "seekTo", map[string]any{
		"positionMs": position.Milliseconds(),
	})
	return err
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (v *VideoPlayerView) SetVolume(volume float64) error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setVolume", map[string]any{
		"volume": volume,
	})
	return err
}

// SetLooping sets whether playback should loop.
func (v *VideoPlayerView) SetLooping(looping bool) error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setLooping", map[string]any{
		"looping": looping,
	})
	return err
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (v *VideoPlayerView) SetPlaybackSpeed(rate float64) error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setPlaybackSpeed", map[string]any{
		"rate": rate,
	})
	return err
}

// Load loads a new media URL, replacing the current media item.
// The native player prepares the new URL immediately. If looping was
// enabled, it remains active for the new item.
func (v *VideoPlayerView) Load(url string) error {
	_, err := GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "load", map[string]any{
		"url": url,
	})
	return err
}

// State returns the current playback state.
func (v *VideoPlayerView) State() PlaybackState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.state
}

// Position returns the current playback position.
func (v *VideoPlayerView) Position() time.Duration {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.position
}

// Duration returns the total media duration.
func (v *VideoPlayerView) Duration() time.Duration {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.duration
}

// Buffered returns the buffered position.
func (v *VideoPlayerView) Buffered() time.Duration {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.buffered
}

// handlePlaybackStateChanged processes state change events from native.
func (v *VideoPlayerView) handlePlaybackStateChanged(state PlaybackState) {
	v.mu.Lock()
	stateChanged := state != v.state
	v.state = state
	cb := v.OnPlaybackStateChanged
	v.mu.Unlock()

	if stateChanged && cb != nil {
		Dispatch(func() {
			cb(state)
		})
	}
}

// handlePositionChanged processes position update events from native.
func (v *VideoPlayerView) handlePositionChanged(position, duration, buffered time.Duration) {
	v.mu.Lock()
	v.position = position
	v.duration = duration
	v.buffered = buffered
	cb := v.OnPositionChanged
	v.mu.Unlock()

	if cb != nil {
		Dispatch(func() {
			cb(position, duration, buffered)
		})
	}
}

// handleError processes error events from native.
func (v *VideoPlayerView) handleError(code string, message string) {
	v.mu.RLock()
	cb := v.OnError
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(code, message)
		})
	}
}

// videoPlayerViewFactory creates video player platform views.
type videoPlayerViewFactory struct{}

func (f *videoPlayerViewFactory) ViewType() string {
	return "video_player"
}

func (f *videoPlayerViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	return NewVideoPlayerView(viewID), nil
}

func init() {
	GetPlatformViewRegistry().RegisterFactory(&videoPlayerViewFactory{})
}
