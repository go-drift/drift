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
// All methods are safe for concurrent use. Callback fields should be set
// on the UI thread (e.g. in InitState or a UseController callback). Setting
// them before calling Load ensures no events are missed.
type AudioPlayerController struct {
	svc *audioPlayerServiceState
	mu  sync.RWMutex

	// guarded by mu
	id       int64
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
	// The native platform fires this callback approximately every 250ms
	// while media is loaded.
	// Called on the UI thread.
	// Set this before calling [AudioPlayerController.Load] or any other
	// playback method to avoid missing events.
	OnPositionChanged func(position, duration, buffered time.Duration)

	// OnError is called when a playback error occurs.
	// The code parameter is one of [ErrCodeSourceError],
	// [ErrCodeDecoderError], or [ErrCodePlaybackFailed].
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
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("load", map[string]any{
		"playerId": id,
		"url":      url,
	})
	return err
}

// Play starts or resumes playback. Call [AudioPlayerController.Load] first
// to set the media URL.
func (c *AudioPlayerController) Play() error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("play", map[string]any{
		"playerId": id,
	})
	return err
}

// Pause pauses playback.
func (c *AudioPlayerController) Pause() error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("pause", map[string]any{
		"playerId": id,
	})
	return err
}

// Stop stops playback and resets the player to the idle state. The loaded
// media is retained, so calling Play will restart playback from the beginning.
// To release native resources, use Dispose instead.
func (c *AudioPlayerController) Stop() error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("stop", map[string]any{
		"playerId": id,
	})
	return err
}

// SeekTo seeks to the given position.
func (c *AudioPlayerController) SeekTo(position time.Duration) error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("seekTo", map[string]any{
		"playerId":   id,
		"positionMs": position.Milliseconds(),
	})
	return err
}

// SetVolume sets the playback volume (0.0 to 1.0). Values outside this range
// are clamped by the native player.
func (c *AudioPlayerController) SetVolume(volume float64) error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("setVolume", map[string]any{
		"playerId": id,
		"volume":   volume,
	})
	return err
}

// SetLooping sets whether playback should loop.
func (c *AudioPlayerController) SetLooping(looping bool) error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("setLooping", map[string]any{
		"playerId": id,
		"looping":  looping,
	})
	return err
}

// SetPlaybackSpeed sets the playback speed (1.0 = normal). The rate must be
// positive. Behavior for zero or negative values is platform-dependent.
func (c *AudioPlayerController) SetPlaybackSpeed(rate float64) error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return ErrDisposed
	}
	_, err := c.svc.channel.Invoke("setPlaybackSpeed", map[string]any{
		"playerId": id,
		"rate":     rate,
	})
	return err
}

// Dispose releases the audio player and its native resources. After disposal,
// this controller must not be reused. Dispose is idempotent; calling it more
// than once is safe.
func (c *AudioPlayerController) Dispose() {
	c.mu.Lock()
	id := c.id
	c.id = 0
	c.mu.Unlock()
	if id == 0 {
		return
	}

	audioRegistryMu.Lock()
	delete(audioRegistry, id)
	audioRegistryMu.Unlock()

	c.svc.channel.Invoke("dispose", map[string]any{
		"playerId": id,
	})
}
