package platform

// PlaybackState represents the current state of audio or video playback.
// Errors are delivered through separate error callbacks rather than as
// a playback state.
type PlaybackState int

const (
	// PlaybackStateIdle indicates the player has been created but no media is loaded.
	PlaybackStateIdle PlaybackState = iota

	// PlaybackStateBuffering indicates the player is buffering media data before playback can continue.
	PlaybackStateBuffering

	// PlaybackStatePlaying indicates the player is actively playing media.
	PlaybackStatePlaying

	// PlaybackStateCompleted indicates playback has reached the end of the media.
	PlaybackStateCompleted

	// PlaybackStatePaused indicates the player is paused and can be resumed.
	PlaybackStatePaused
)

// String returns a human-readable label for the playback state.
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
