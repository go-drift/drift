// Package runtime is the Go API of the Drift media plugin
// (github.com/go-drift/drift/plugins/media/plugin in drift.yaml): video in
// the widget tree and audio players without a view.
//
//	import media "github.com/go-drift/drift/plugins/media/runtime"
//
//	// In a State's InitState:
//	s.video = media.NewVideoPlayerController()
//	core.UseDisposable(s, s.video)
//	s.video.OnPlaybackStateChanged = func(state media.PlaybackState) { ... }
//	s.video.Load(url)
//
//	// In Build:
//	media.VideoPlayer{Controller: s.video, Height: 225}
//
// Controller methods are safe from any goroutine and do not wait for
// playback; results arrive through the callbacks, on the UI thread.
package runtime

// PlaybackState is the playback state of a video or audio player.
type PlaybackState int

const (
	// PlaybackStateIdle: no media loaded, or stopped.
	PlaybackStateIdle PlaybackState = iota
	// PlaybackStateBuffering: waiting for media data before playing.
	PlaybackStateBuffering
	// PlaybackStatePlaying: playing.
	PlaybackStatePlaying
	// PlaybackStateCompleted: reached the end of the media.
	PlaybackStateCompleted
	// PlaybackStatePaused: paused; Play resumes.
	PlaybackStatePaused
)

func (s PlaybackState) String() string {
	switch s {
	case PlaybackStateIdle:
		return "Idle"
	case PlaybackStateBuffering:
		return "Buffering"
	case PlaybackStatePlaying:
		return "Playing"
	case PlaybackStateCompleted:
		return "Completed"
	case PlaybackStatePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// Error codes passed to OnError callbacks, the same on both platforms.
const (
	// ErrCodeSourceError: the media could not be loaded (network, URL,
	// format, container parsing).
	ErrCodeSourceError = "source_error"
	// ErrCodeDecoderError: the media could not be decoded or rendered
	// (codec, audio track, DRM).
	ErrCodeDecoderError = "decoder_error"
	// ErrCodePlaybackFailed: any other playback failure.
	ErrCodePlaybackFailed = "playback_failed"
)

// The native halves send numbers as JSON numbers (float64 in Go).

func intArg(args map[string]any, key string) (int64, bool) {
	f, ok := args[key].(float64)
	return int64(f), ok && f == float64(int64(f))
}

func stringArg(args map[string]any, key string) (string, bool) {
	s, ok := args[key].(string)
	return s, ok
}
