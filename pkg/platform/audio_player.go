package platform

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

var (
	audioPlayerInstance *AudioPlayerController
	audioPlayerMu       sync.Mutex
)

// AudioPlayerState represents a snapshot of audio playback state, delivered
// via [AudioPlayerController.OnStateChanged]. Each update contains the current
// playback state along with timing information.
type AudioPlayerState struct {
	// PlaybackState is the current playback state.
	PlaybackState PlaybackState
	// Position is the current playback position.
	Position time.Duration
	// Duration is the total duration of the loaded media.
	// Zero if no media is loaded.
	Duration time.Duration
	// Buffered is the buffered position, indicating how far ahead the
	// player has downloaded content.
	Buffered time.Duration
}

// AudioPlayerError represents an audio playback error, delivered via
// [AudioPlayerController.OnError].
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
// Set [AudioPlayerController.OnStateChanged] and [AudioPlayerController.OnError]
// to receive playback updates. Callbacks are dispatched on the UI thread.
//
// Only one AudioPlayerController may exist at a time. Creating a second
// before disposing the first returns an error. Call [AudioPlayerController.Dispose]
// to release resources and allow a new instance to be created.
type AudioPlayerController struct {
	state     *audioPlayerServiceState
	loadedURL string
	disposed  bool
	eventSub  *Subscription
	errorSub  *Subscription

	// OnStateChanged is called when the playback state, position, or
	// buffered position changes. Called on the UI thread.
	OnStateChanged func(state AudioPlayerState)

	// OnError is called when a playback error occurs. Called on the UI thread.
	OnError func(err AudioPlayerError)
}

// NewAudioPlayerController creates a new audio player controller.
// Returns an error if another AudioPlayerController already exists and has not been disposed.
func NewAudioPlayerController() (*AudioPlayerController, error) {
	audioPlayerMu.Lock()
	defer audioPlayerMu.Unlock()

	if audioPlayerInstance != nil && !audioPlayerInstance.disposed {
		return nil, fmt.Errorf("drift: only one AudioPlayerController may exist at a time; call Dispose() on the previous instance first")
	}

	state := newAudioPlayerService()
	c := &AudioPlayerController{
		state: state,
	}

	// Listen for state events from native and dispatch to UI thread.
	c.eventSub = state.events.Listen(EventHandler{
		OnEvent: func(data any) {
			val, err := parseAudioPlayerState(data)
			if err != nil {
				errors.Report(&errors.DriftError{
					Op:      "AudioPlayerController.parseState",
					Kind:    errors.KindParsing,
					Channel: "drift/audio_player/events",
					Err:     err,
				})
				return
			}
			Dispatch(func() {
				if c.OnStateChanged != nil {
					c.OnStateChanged(val)
				}
			})
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "AudioPlayerController.stateStream",
				Kind:    errors.KindPlatform,
				Channel: "drift/audio_player/events",
				Err:     err,
			})
		},
	})

	// Listen for error events from native and dispatch to UI thread.
	c.errorSub = state.errors.Listen(EventHandler{
		OnEvent: func(data any) {
			val, err := parseAudioPlayerError(data)
			if err != nil {
				errors.Report(&errors.DriftError{
					Op:      "AudioPlayerController.parseError",
					Kind:    errors.KindParsing,
					Channel: "drift/audio_player/errors",
					Err:     err,
				})
				return
			}
			Dispatch(func() {
				if c.OnError != nil {
					c.OnError(val)
				}
			})
		},
		OnError: func(err error) {
			errors.Report(&errors.DriftError{
				Op:      "AudioPlayerController.errorStream",
				Kind:    errors.KindPlatform,
				Channel: "drift/audio_player/errors",
				Err:     err,
			})
		},
	})

	audioPlayerInstance = c
	return c, nil
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

// Play loads the given URL (if not already loaded) and starts playback.
// Calling Play with the same URL after a pause resumes playback.
func (c *AudioPlayerController) Play(url string) {
	if url != c.loadedURL {
		c.state.channel.Invoke("load", map[string]any{
			"url": url,
		})
		c.loadedURL = url
	}
	c.state.channel.Invoke("play", nil)
}

// Pause pauses playback.
func (c *AudioPlayerController) Pause() {
	c.state.channel.Invoke("pause", nil)
}

// Stop stops playback and resets the player to the idle state.
// A subsequent call to [AudioPlayerController.Play] will reload the URL.
func (c *AudioPlayerController) Stop() {
	c.state.channel.Invoke("stop", nil)
	c.loadedURL = ""
}

// SeekTo seeks to the given position.
func (c *AudioPlayerController) SeekTo(position time.Duration) {
	c.state.channel.Invoke("seekTo", map[string]any{
		"positionMs": position.Milliseconds(),
	})
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (c *AudioPlayerController) SetVolume(volume float64) {
	c.state.channel.Invoke("setVolume", map[string]any{
		"volume": volume,
	})
}

// SetLooping sets whether playback should loop.
func (c *AudioPlayerController) SetLooping(looping bool) {
	c.state.channel.Invoke("setLooping", map[string]any{
		"looping": looping,
	})
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (c *AudioPlayerController) SetPlaybackSpeed(rate float64) {
	c.state.channel.Invoke("setPlaybackSpeed", map[string]any{
		"rate": rate,
	})
}

// Dispose releases the audio player and its native resources. After disposal,
// this controller must not be reused. A new [AudioPlayerController] may be
// created after Dispose returns.
func (c *AudioPlayerController) Dispose() {
	if c.eventSub != nil {
		c.eventSub.Cancel()
	}
	if c.errorSub != nil {
		c.errorSub.Cancel()
	}

	c.state.channel.Invoke("dispose", nil)

	audioPlayerMu.Lock()
	c.disposed = true
	if audioPlayerInstance == c {
		audioPlayerInstance = nil
	}
	audioPlayerMu.Unlock()
}

func parseAudioPlayerState(data any) (AudioPlayerState, error) {
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
		Position:      time.Duration(positionMs) * time.Millisecond,
		Duration:      time.Duration(durationMs) * time.Millisecond,
		Buffered:      time.Duration(bufferedMs) * time.Millisecond,
	}, nil
}

func parseAudioPlayerError(data any) (AudioPlayerError, error) {
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
