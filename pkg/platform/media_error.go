package platform

// Canonical media error codes shared by audio and video playback.
// Native implementations (ExoPlayer on Android, AVPlayer on iOS) map
// platform-specific errors to these codes so that Go callbacks receive
// consistent values across platforms.
const (
	// ErrCodeSourceError indicates the media source could not be loaded.
	// Covers network failures, invalid URLs, unsupported formats, and
	// container parsing errors.
	ErrCodeSourceError = "source_error"

	// ErrCodeDecoderError indicates the media could not be decoded or
	// rendered. Covers codec failures, audio track initialization errors,
	// and DRM-related errors.
	ErrCodeDecoderError = "decoder_error"

	// ErrCodePlaybackFailed indicates a general playback failure that
	// does not fit a more specific category.
	ErrCodePlaybackFailed = "playback_failed"
)
