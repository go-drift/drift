package platform

import (
	"sync"

	"github.com/go-drift/drift/pkg/errors"
)

var (
	audioPlayerInstance *AudioPlayerController
	audioPlayerMu       sync.Mutex
)

// AudioPlayerState represents a snapshot of audio playback state, delivered
// via [AudioPlayerController.States]. Each update contains the current
// playback state along with timing information.
type AudioPlayerState struct {
	// PlaybackState is the current playback state.
	PlaybackState PlaybackState
	// PositionMs is the current playback position in milliseconds.
	PositionMs int64
	// DurationMs is the total duration of the loaded media in milliseconds.
	// Zero if no media is loaded.
	DurationMs int64
	// BufferedMs is the buffered position in milliseconds, indicating how
	// far ahead the player has downloaded content.
	BufferedMs int64
}

// AudioPlayerError represents an audio playback error, delivered via
// [AudioPlayerController.Errors].
type AudioPlayerError struct {
	// Code is a platform-specific error code.
	Code string
	// Message is a human-readable error description.
	Message string
}

// AudioPlayerController provides audio playback control without a visual component.
// Audio has no visual surface, so this uses a standalone platform channel
// rather than the platform view system.
//
// Subscribe to playback updates via [AudioPlayerController.States] and errors
// via [AudioPlayerController.Errors].
//
// Only one AudioPlayerController may exist at a time. Creating a second
// before disposing the first will panic. Call [AudioPlayerController.Dispose]
// to release resources and allow a new instance to be created.
type AudioPlayerController struct {
	state    *audioPlayerServiceState
	states   *Stream[AudioPlayerState]
	errs     *Stream[AudioPlayerError]
	disposed bool
}

// NewAudioPlayerController creates a new audio player controller.
// Panics if another AudioPlayerController already exists and has not been disposed.
func NewAudioPlayerController() *AudioPlayerController {
	audioPlayerMu.Lock()
	defer audioPlayerMu.Unlock()

	if audioPlayerInstance != nil && !audioPlayerInstance.disposed {
		panic("drift: only one AudioPlayerController may exist at a time; call Dispose() on the previous instance first")
	}

	state := newAudioPlayerService()
	c := &AudioPlayerController{
		state:  state,
		states: NewStream("drift/audio_player/events", state.events, parseAudioPlayerStateWithError),
		errs:   NewStream("drift/audio_player/errors", state.errors, parseAudioPlayerErrorWithError),
	}
	audioPlayerInstance = c
	return c
}

type audioPlayerServiceState struct {
	channel *MethodChannel
	events  *EventChannel
	errors  *EventChannel
}

func newAudioPlayerService() *audioPlayerServiceState {
	return &audioPlayerServiceState{
		channel: NewMethodChannel("drift/audio_player"),
		events:  NewEventChannel("drift/audio_player/events"),
		errors:  NewEventChannel("drift/audio_player/errors"),
	}
}

// Load loads a media URL for playback.
func (c *AudioPlayerController) Load(url string) error {
	_, err := c.state.channel.Invoke("load", map[string]any{
		"url": url,
	})
	return err
}

// Play starts or resumes playback.
func (c *AudioPlayerController) Play() error {
	_, err := c.state.channel.Invoke("play", nil)
	return err
}

// Pause pauses playback.
func (c *AudioPlayerController) Pause() error {
	_, err := c.state.channel.Invoke("pause", nil)
	return err
}

// Stop stops playback and resets the player to the idle state.
// The player can be reused by calling [AudioPlayerController.Load] again.
// To fully release resources, use [AudioPlayerController.Dispose] instead.
func (c *AudioPlayerController) Stop() error {
	_, err := c.state.channel.Invoke("stop", nil)
	return err
}

// SeekTo seeks to a position in milliseconds.
func (c *AudioPlayerController) SeekTo(positionMs int64) error {
	_, err := c.state.channel.Invoke("seekTo", map[string]any{
		"positionMs": positionMs,
	})
	return err
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (c *AudioPlayerController) SetVolume(volume float64) error {
	_, err := c.state.channel.Invoke("setVolume", map[string]any{
		"volume": volume,
	})
	return err
}

// SetLooping sets whether playback should loop.
func (c *AudioPlayerController) SetLooping(looping bool) error {
	_, err := c.state.channel.Invoke("setLooping", map[string]any{
		"looping": looping,
	})
	return err
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (c *AudioPlayerController) SetPlaybackSpeed(rate float64) error {
	_, err := c.state.channel.Invoke("setPlaybackSpeed", map[string]any{
		"rate": rate,
	})
	return err
}

// States returns a stream of [AudioPlayerState] updates. The stream emits
// a new value whenever the playback state, position, or buffered position changes.
func (c *AudioPlayerController) States() *Stream[AudioPlayerState] {
	return c.states
}

// Errors returns a stream of [AudioPlayerError] values. Errors are delivered
// separately from state updates and indicate issues such as network failures
// or unsupported media formats.
func (c *AudioPlayerController) Errors() *Stream[AudioPlayerError] {
	return c.errs
}

// Dispose releases the audio player and its native resources. After disposal,
// this controller must not be reused. A new [AudioPlayerController] may be
// created after Dispose returns.
func (c *AudioPlayerController) Dispose() error {
	_, err := c.state.channel.Invoke("dispose", nil)

	audioPlayerMu.Lock()
	c.disposed = true
	if audioPlayerInstance == c {
		audioPlayerInstance = nil
	}
	audioPlayerMu.Unlock()

	return err
}

func parseAudioPlayerStateWithError(data any) (AudioPlayerState, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return AudioPlayerState{}, &errors.ParseError{
			Channel:  "drift/audio_player/events",
			DataType: "AudioPlayerState",
			Got:      data,
		}
	}

	stateInt, _ := toInt(m["playbackState"])
	positionMs, _ := toInt64(m["positionMs"])
	durationMs, _ := toInt64(m["durationMs"])
	bufferedMs, _ := toInt64(m["bufferedMs"])

	return AudioPlayerState{
		PlaybackState: PlaybackState(stateInt),
		PositionMs:    positionMs,
		DurationMs:    durationMs,
		BufferedMs:    bufferedMs,
	}, nil
}

func parseAudioPlayerErrorWithError(data any) (AudioPlayerError, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return AudioPlayerError{}, &errors.ParseError{
			Channel:  "drift/audio_player/errors",
			DataType: "AudioPlayerError",
			Got:      data,
		}
	}
	return AudioPlayerError{
		Code:    parseString(m["code"]),
		Message: parseString(m["message"]),
	}, nil
}
