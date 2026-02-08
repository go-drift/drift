package platform

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/errors"
)

var (
	audioService     *audioPlayerServiceState
	audioServiceOnce sync.Once

	audioRegistry   = map[int64]*AudioPlayerController{}
	audioRegistryMu sync.RWMutex

	audioPlayerNextID atomic.Int64
)

// AudioPlayerController provides audio playback control without a visual component.
// Audio has no visual surface, so this uses a standalone platform channel
// rather than the platform view system. Build your own UI around the controller.
//
// Multiple controllers may exist concurrently, each managing its own native
// player instance. Call [AudioPlayerController.Dispose] to release resources
// when a controller is no longer needed.
//
// Set callback fields (OnPlaybackStateChanged, OnPositionChanged, OnError)
// before calling [AudioPlayerController.Load] or any other playback method
// to ensure no events are missed.
//
// All methods are safe for concurrent use. Callback fields are plain struct
// fields; set them before calling Load and do not modify them after playback
// begins.
type AudioPlayerController struct {
	id  int64
	svc *audioPlayerServiceState
	mu  sync.RWMutex

	// guarded by mu
	state    PlaybackState
	position time.Duration
	duration time.Duration
	buffered time.Duration

	// OnPlaybackStateChanged is called when the playback state changes.
	// Called on the UI thread.
	// Set this before calling [AudioPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPlaybackStateChanged func(PlaybackState)

	// OnPositionChanged is called when the playback position updates.
	// Called on the UI thread.
	// Set this before calling [AudioPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	// Called on the UI thread.
	// Set this before calling [AudioPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnError func(code, message string)
}

// NewAudioPlayerController creates a new audio player controller.
// Each controller manages its own native player instance.
func NewAudioPlayerController() *AudioPlayerController {
	svc := ensureAudioService()
	id := audioPlayerNextID.Add(1)

	c := &AudioPlayerController{
		id:  id,
		svc: svc,
	}

	audioRegistryMu.Lock()
	audioRegistry[id] = c
	audioRegistryMu.Unlock()

	return c
}

// State returns the current playback state.
func (c *AudioPlayerController) State() PlaybackState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Position returns the current playback position.
func (c *AudioPlayerController) Position() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.position
}

// Duration returns the total media duration.
func (c *AudioPlayerController) Duration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.duration
}

// Buffered returns the buffered position.
func (c *AudioPlayerController) Buffered() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffered
}

type audioPlayerServiceState struct {
	channel *MethodChannel
	events  *EventChannel
	errors  *EventChannel
}

func ensureAudioService() *audioPlayerServiceState {
	audioServiceOnce.Do(func() {
		svc := &audioPlayerServiceState{
			channel: NewMethodChannel("drift/audio_player"),
			events:  NewEventChannel("drift/audio_player/events"),
			errors:  NewEventChannel("drift/audio_player/errors"),
		}

		// Shared event listener: routes events to the correct controller.
		svc.events.Listen(EventHandler{
			OnEvent: func(data any) {
				m, ok := data.(map[string]any)
				if !ok {
					return
				}
				playerID, _ := toInt64(m["playerId"])
				audioRegistryMu.RLock()
				c := audioRegistry[playerID]
				audioRegistryMu.RUnlock()
				if c == nil {
					return
				}

				stateInt, _ := toInt(m["playbackState"])
				positionMs, _ := toInt64(m["positionMs"])
				durationMs, _ := toInt64(m["durationMs"])
				bufferedMs, _ := toInt64(m["bufferedMs"])

				state := PlaybackState(stateInt)
				pos := time.Duration(positionMs) * time.Millisecond
				dur := time.Duration(durationMs) * time.Millisecond
				buf := time.Duration(bufferedMs) * time.Millisecond

				c.mu.Lock()
				stateChanged := state != c.state
				c.state = state
				c.position = pos
				c.duration = dur
				c.buffered = buf
				c.mu.Unlock()

				Dispatch(func() {
					if stateChanged && c.OnPlaybackStateChanged != nil {
						c.OnPlaybackStateChanged(state)
					}
					if c.OnPositionChanged != nil {
						c.OnPositionChanged(pos, dur, buf)
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

		// Shared error listener: routes errors to the correct controller.
		svc.errors.Listen(EventHandler{
			OnEvent: func(data any) {
				m, ok := data.(map[string]any)
				if !ok {
					return
				}
				playerID, _ := toInt64(m["playerId"])
				audioRegistryMu.RLock()
				c := audioRegistry[playerID]
				audioRegistryMu.RUnlock()
				if c == nil {
					return
				}

				code := parseString(m["code"])
				message := parseString(m["message"])

				Dispatch(func() {
					if c.OnError != nil {
						c.OnError(code, message)
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

		audioService = svc
	})
	return audioService
}

// Load prepares the given URL for playback. The native player begins buffering
// the media source. Call [AudioPlayerController.Play] to start playback.
func (c *AudioPlayerController) Load(url string) error {
	_, err := c.svc.channel.Invoke("load", map[string]any{
		"playerId": c.id,
		"url":      url,
	})
	return err
}

// Play starts or resumes playback. Call [AudioPlayerController.Load] first
// to set the media URL.
func (c *AudioPlayerController) Play() error {
	_, err := c.svc.channel.Invoke("play", map[string]any{
		"playerId": c.id,
	})
	return err
}

// Pause pauses playback.
func (c *AudioPlayerController) Pause() error {
	_, err := c.svc.channel.Invoke("pause", map[string]any{
		"playerId": c.id,
	})
	return err
}

// Stop stops playback and resets the player to the idle state.
func (c *AudioPlayerController) Stop() error {
	_, err := c.svc.channel.Invoke("stop", map[string]any{
		"playerId": c.id,
	})
	return err
}

// SeekTo seeks to the given position.
func (c *AudioPlayerController) SeekTo(position time.Duration) error {
	_, err := c.svc.channel.Invoke("seekTo", map[string]any{
		"playerId":   c.id,
		"positionMs": position.Milliseconds(),
	})
	return err
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (c *AudioPlayerController) SetVolume(volume float64) error {
	_, err := c.svc.channel.Invoke("setVolume", map[string]any{
		"playerId": c.id,
		"volume":   volume,
	})
	return err
}

// SetLooping sets whether playback should loop.
func (c *AudioPlayerController) SetLooping(looping bool) error {
	_, err := c.svc.channel.Invoke("setLooping", map[string]any{
		"playerId": c.id,
		"looping":  looping,
	})
	return err
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal).
func (c *AudioPlayerController) SetPlaybackSpeed(rate float64) error {
	_, err := c.svc.channel.Invoke("setPlaybackSpeed", map[string]any{
		"playerId": c.id,
		"rate":     rate,
	})
	return err
}

// Dispose releases the audio player and its native resources. After disposal,
// this controller must not be reused. Dispose is idempotent; calling it more
// than once is safe.
func (c *AudioPlayerController) Dispose() {
	if c.id == 0 {
		return
	}
	id := c.id
	c.id = 0

	audioRegistryMu.Lock()
	delete(audioRegistry, id)
	audioRegistryMu.Unlock()

	c.svc.channel.Invoke("dispose", map[string]any{
		"playerId": id,
	})
}
