package platform

import (
	"sync"
	"time"
)

// PlaybackState represents the current state of video/audio playback.
//
// Native players currently emit Idle(0), Buffering(2), Playing(3),
// Completed(4), and Paused(6). Loading(1) and Error(5) are reserved
// for future use; errors are delivered through separate error callbacks
// rather than as a playback state.
type PlaybackState int

const (
	// PlaybackStateIdle indicates the player has been created but no media is loaded.
	PlaybackStateIdle PlaybackState = 0

	// PlaybackStateLoading is reserved for future use. Not currently emitted by native players.
	PlaybackStateLoading PlaybackState = 1

	// PlaybackStateBuffering indicates the player is buffering media data before playback can continue.
	PlaybackStateBuffering PlaybackState = 2

	// PlaybackStatePlaying indicates the player is actively playing media.
	PlaybackStatePlaying PlaybackState = 3

	// PlaybackStateCompleted indicates playback has reached the end of the media.
	PlaybackStateCompleted PlaybackState = 4

	// PlaybackStateError is reserved for future use. Errors are delivered through
	// separate error callbacks ([VideoPlayerClient.OnError], [AudioPlayerController.OnError])
	// rather than as a playback state.
	PlaybackStateError PlaybackState = 5

	// PlaybackStatePaused indicates the player is paused and can be resumed.
	PlaybackStatePaused PlaybackState = 6
)

// String returns a human-readable label for the playback state.
func (s PlaybackState) String() string {
	switch s {
	case PlaybackStateIdle:
		return "Idle"
	case PlaybackStateLoading:
		return "Loading"
	case PlaybackStateBuffering:
		return "Buffering"
	case PlaybackStatePlaying:
		return "Playing"
	case PlaybackStateCompleted:
		return "Completed"
	case PlaybackStateError:
		return "Error"
	case PlaybackStatePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// VideoPlayerClient receives playback callbacks from a [VideoPlayerView].
// Callbacks are dispatched on the UI thread via [Dispatch].
type VideoPlayerClient interface {
	// OnPlaybackStateChanged is called when the playback state changes.
	OnPlaybackStateChanged(state PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	OnPositionChanged(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	OnError(code string, message string)
}

// VideoPlayerView is a platform view that wraps native video playback
// (ExoPlayer on Android, AVPlayer on iOS). It provides transport controls,
// position/duration tracking, and playback state observation.
//
// Use [VideoPlayerView.SetClient] to receive playback callbacks, or read
// cached state directly via [VideoPlayerView.State], [VideoPlayerView.Position], etc.
type VideoPlayerView struct {
	basePlatformView
	client VideoPlayerClient
	mu     sync.RWMutex

	// Cached playback state
	state    PlaybackState
	position time.Duration
	duration time.Duration
	buffered time.Duration
}

// NewVideoPlayerView creates a new video player platform view with the given
// view ID and optional callback client. The client may be nil and set later
// via [VideoPlayerView.SetClient].
func NewVideoPlayerView(viewID int64, client VideoPlayerClient) *VideoPlayerView {
	return &VideoPlayerView{
		basePlatformView: basePlatformView{
			viewID:   viewID,
			viewType: "video_player",
		},
		client: client,
	}
}

// SetClient sets the callback client for this view.
func (v *VideoPlayerView) SetClient(client VideoPlayerClient) {
	v.mu.Lock()
	v.client = client
	v.mu.Unlock()
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
func (v *VideoPlayerView) Play() {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "play", nil)
}

// Pause pauses playback.
func (v *VideoPlayerView) Pause() {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "pause", nil)
}

// SeekTo seeks to the given position.
func (v *VideoPlayerView) SeekTo(position time.Duration) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "seekTo", map[string]any{
		"positionMs": position.Milliseconds(),
	})
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (v *VideoPlayerView) SetVolume(volume float64) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setVolume", map[string]any{
		"volume": volume,
	})
}

// SetLooping sets whether playback should loop.
func (v *VideoPlayerView) SetLooping(looping bool) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setLooping", map[string]any{
		"looping": looping,
	})
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (v *VideoPlayerView) SetPlaybackSpeed(rate float64) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "setPlaybackSpeed", map[string]any{
		"rate": rate,
	})
}

// LoadURL loads a new media URL, replacing the current media item.
// The native player prepares the new URL immediately. If looping was
// enabled, it remains active for the new item.
func (v *VideoPlayerView) LoadURL(url string) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "loadUrl", map[string]any{
		"url": url,
	})
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
	v.state = state
	client := v.client
	v.mu.Unlock()

	if client != nil {
		Dispatch(func() {
			client.OnPlaybackStateChanged(state)
		})
	}
}

// handlePositionChanged processes position update events from native.
func (v *VideoPlayerView) handlePositionChanged(position, duration, buffered time.Duration) {
	v.mu.Lock()
	v.position = position
	v.duration = duration
	v.buffered = buffered
	client := v.client
	v.mu.Unlock()

	if client != nil {
		Dispatch(func() {
			client.OnPositionChanged(position, duration, buffered)
		})
	}
}

// handleError processes error events from native.
func (v *VideoPlayerView) handleError(code string, message string) {
	v.mu.RLock()
	client := v.client
	v.mu.RUnlock()

	if client != nil {
		Dispatch(func() {
			client.OnError(code, message)
		})
	}
}

// videoPlayerViewFactory creates video player platform views.
type videoPlayerViewFactory struct{}

func (f *videoPlayerViewFactory) ViewType() string {
	return "video_player"
}

func (f *videoPlayerViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	return NewVideoPlayerView(viewID, nil), nil
}

func init() {
	GetPlatformViewRegistry().RegisterFactory(&videoPlayerViewFactory{})
}
