package platform

import (
	"sync"
)

// PlaybackState represents the current state of video/audio playback.
//
// Native players currently emit Idle(0), Buffering(2), Playing(3),
// Completed(4), and Paused(6). Loading(1) and Error(5) are reserved
// for future use; errors are delivered through separate error callbacks
// rather than as a playback state.
type PlaybackState int

const (
	PlaybackStateIdle      PlaybackState = 0
	PlaybackStateLoading   PlaybackState = 1 // Reserved: not currently emitted by native players.
	PlaybackStateBuffering PlaybackState = 2
	PlaybackStatePlaying   PlaybackState = 3
	PlaybackStateCompleted PlaybackState = 4
	PlaybackStateError     PlaybackState = 5 // Reserved: errors use separate error callbacks.
	PlaybackStatePaused    PlaybackState = 6
)

// VideoPlayerClient receives callbacks from native video player view.
type VideoPlayerClient interface {
	// OnPlaybackStateChanged is called when the playback state changes.
	OnPlaybackStateChanged(state PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	OnPositionChanged(positionMs, durationMs, bufferedMs int64)

	// OnError is called when a playback error occurs.
	OnError(code string, message string)
}

// VideoPlayerView is a platform view for native video playback.
type VideoPlayerView struct {
	basePlatformView
	client VideoPlayerClient
	mu     sync.RWMutex

	// Cached playback state
	state      PlaybackState
	positionMs int64
	durationMs int64
	bufferedMs int64
}

// NewVideoPlayerView creates a new video player platform view.
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

// SeekTo seeks to a position in milliseconds.
func (v *VideoPlayerView) SeekTo(positionMs int64) {
	GetPlatformViewRegistry().InvokeViewMethod(v.viewID, "seekTo", map[string]any{
		"positionMs": positionMs,
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

// PositionMs returns the current playback position in milliseconds.
func (v *VideoPlayerView) PositionMs() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.positionMs
}

// DurationMs returns the total duration in milliseconds.
func (v *VideoPlayerView) DurationMs() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.durationMs
}

// BufferedMs returns the buffered position in milliseconds.
func (v *VideoPlayerView) BufferedMs() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.bufferedMs
}

// handlePlaybackStateChanged processes state change events from native.
func (v *VideoPlayerView) handlePlaybackStateChanged(state PlaybackState) {
	v.mu.Lock()
	v.state = state
	client := v.client
	v.mu.Unlock()

	if client != nil {
		client.OnPlaybackStateChanged(state)
	}
}

// handlePositionChanged processes position update events from native.
func (v *VideoPlayerView) handlePositionChanged(positionMs, durationMs, bufferedMs int64) {
	v.mu.Lock()
	v.positionMs = positionMs
	v.durationMs = durationMs
	v.bufferedMs = bufferedMs
	client := v.client
	v.mu.Unlock()

	if client != nil {
		client.OnPositionChanged(positionMs, durationMs, bufferedMs)
	}
}

// handleError processes error events from native.
func (v *VideoPlayerView) handleError(code string, message string) {
	v.mu.RLock()
	client := v.client
	v.mu.RUnlock()

	if client != nil {
		client.OnError(code, message)
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
