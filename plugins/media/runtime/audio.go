package runtime

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// Audio channels. Must match the literals in the plugin's native sources
// (plugin/ios/AudioPlayers.swift, plugin/android/AudioPlayers.kt).
const (
	audioChannelName   = "drift/media/audio"
	audioEventsChannel = "drift/media/audio/events"
	audioErrorsChannel = "drift/media/audio/errors"
)

var (
	audioChannel = platform.NewMethodChannel(audioChannelName)
	audioEvents  = platform.NewEventChannel(audioEventsChannel)
	audioErrors  = platform.NewEventChannel(audioErrorsChannel)
	nextAudioID  atomic.Int64
)

// AudioPlayerController controls a native audio player without a view;
// build the UI around it. Several can play at once. Call Dispose to
// release the native player.
//
// Set the callbacks before Load so no event is missed. They run on the UI
// thread.
type AudioPlayerController struct {
	mu   sync.RWMutex
	id   int64 // 0 once disposed
	subs []*platform.Subscription

	state    PlaybackState
	position time.Duration
	duration time.Duration
	buffered time.Duration

	// OnPlaybackStateChanged is called when the playback state changes.
	OnPlaybackStateChanged func(PlaybackState)
	// OnPositionChanged is called about every 250ms while media is loaded.
	OnPositionChanged func(position, duration, buffered time.Duration)
	// OnError is called on a playback error; code is one of the ErrCode
	// constants.
	OnError func(code, message string)
}

// NewAudioPlayerController creates a controller; its native player is
// created on first use.
func NewAudioPlayerController() *AudioPlayerController {
	c := &AudioPlayerController{id: nextAudioID.Add(1)}
	c.subs = []*platform.Subscription{
		audioEvents.Listen(platform.EventHandler{OnEvent: c.handleEvent, OnError: reportAudioStreamError}),
		audioErrors.Listen(platform.EventHandler{OnEvent: c.handleError, OnError: reportAudioStreamError}),
	}
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

// Duration returns the media's duration.
func (c *AudioPlayerController) Duration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.duration
}

// Buffered returns how far the media is buffered.
func (c *AudioPlayerController) Buffered() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffered
}

// Load loads url for playback; Play starts it.
func (c *AudioPlayerController) Load(url string) error {
	return c.invoke("load", map[string]any{"url": url})
}

// Play starts or resumes playback.
func (c *AudioPlayerController) Play() error { return c.invoke("play", nil) }

// Pause pauses playback.
func (c *AudioPlayerController) Pause() error { return c.invoke("pause", nil) }

// Stop stops playback and returns to the start; the media stays loaded.
func (c *AudioPlayerController) Stop() error { return c.invoke("stop", nil) }

// SeekTo seeks to position.
func (c *AudioPlayerController) SeekTo(position time.Duration) error {
	return c.invoke("seekTo", map[string]any{"positionMs": position.Milliseconds()})
}

// SetVolume sets the volume, from 0 to 1.
func (c *AudioPlayerController) SetVolume(volume float64) error {
	return c.invoke("setVolume", map[string]any{"volume": volume})
}

// SetLooping sets whether playback restarts at the end.
func (c *AudioPlayerController) SetLooping(looping bool) error {
	return c.invoke("setLooping", map[string]any{"looping": looping})
}

// SetPlaybackSpeed sets the playback rate (1 is normal speed).
func (c *AudioPlayerController) SetPlaybackSpeed(rate float64) error {
	return c.invoke("setPlaybackSpeed", map[string]any{"rate": rate})
}

// Dispose releases the native player. Later calls return
// platform.ErrDisposed; Dispose itself is idempotent.
func (c *AudioPlayerController) Dispose() {
	c.mu.Lock()
	id := c.id
	c.id = 0
	subs := c.subs
	c.subs = nil
	c.mu.Unlock()
	if id == 0 {
		return
	}
	for _, sub := range subs {
		sub.Cancel()
	}
	_, _ = audioChannel.Invoke(context.Background(), "dispose", map[string]any{"playerId": id})
}

func (c *AudioPlayerController) invoke(method string, args map[string]any) error {
	c.mu.RLock()
	id := c.id
	c.mu.RUnlock()
	if id == 0 {
		return platform.ErrDisposed
	}
	if args == nil {
		args = map[string]any{}
	}
	args["playerId"] = id
	if _, err := audioChannel.Invoke(context.Background(), method, args); err != nil {
		return fmt.Errorf("media audio %s: %w", method, err)
	}
	return nil
}

// mine returns data as a map if it is an event for this player.
func (c *AudioPlayerController) mine(data any) (map[string]any, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, false
	}
	id, ok := intArg(m, "playerId")
	c.mu.RLock()
	defer c.mu.RUnlock()
	return m, ok && id == c.id && c.id != 0
}

func (c *AudioPlayerController) handleEvent(data any) {
	m, ok := c.mine(data)
	if !ok {
		return
	}
	state, _ := intArg(m, "playbackState")
	positionMs, _ := intArg(m, "positionMs")
	durationMs, _ := intArg(m, "durationMs")
	bufferedMs, _ := intArg(m, "bufferedMs")
	position := time.Duration(positionMs) * time.Millisecond
	duration := time.Duration(durationMs) * time.Millisecond
	buffered := time.Duration(bufferedMs) * time.Millisecond

	c.mu.Lock()
	changed := PlaybackState(state) != c.state
	c.state = PlaybackState(state)
	c.position, c.duration, c.buffered = position, duration, buffered
	onState, onPosition := c.OnPlaybackStateChanged, c.OnPositionChanged
	c.mu.Unlock()

	platform.Dispatch(func() {
		if changed && onState != nil {
			onState(PlaybackState(state))
		}
		if onPosition != nil {
			onPosition(position, duration, buffered)
		}
	})
}

func (c *AudioPlayerController) handleError(data any) {
	m, ok := c.mine(data)
	if !ok {
		return
	}
	code, _ := stringArg(m, "code")
	message, _ := stringArg(m, "message")
	c.mu.RLock()
	onError := c.OnError
	c.mu.RUnlock()
	if onError != nil {
		platform.Dispatch(func() { onError(code, message) })
	}
}

func reportAudioStreamError(err error) {
	errors.Report(&errors.DriftError{
		Op:      "media.AudioPlayerController",
		Kind:    errors.KindPlatform,
		Channel: audioEventsChannel,
		Err:     err,
	})
}
