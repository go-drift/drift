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
// release the native player. Subscribe to State, Progress and Errors before
// Load so no change is missed.
type AudioPlayerController struct {
	playback
	mu   sync.RWMutex
	id   int64 // 0 once disposed
	subs []*platform.Subscription
	// calls counts calls sent and not yet answered. Native creates a
	// player on any call but dispose, so dispose goes after them all: the
	// last call answered sends it when Dispose came first (disposeID).
	calls     int
	disposeID int64
}

// NewAudioPlayerController creates a controller; its native player is
// created on first use, so creation itself cannot fail.
func NewAudioPlayerController() *AudioPlayerController {
	c := &AudioPlayerController{
		playback: newPlayback("media.AudioPlayerController"),
		id:       nextAudioID.Add(1),
	}
	c.subs = []*platform.Subscription{
		audioEvents.Listen(platform.EventHandler{OnEvent: c.handleEvent, OnError: reportAudioStreamError}),
		audioErrors.Listen(platform.EventHandler{OnEvent: c.handleError, OnError: reportAudioStreamError}),
	}
	return c
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
	if id != 0 && c.calls > 0 {
		c.disposeID, id = id, 0
	}
	c.mu.Unlock()
	for _, sub := range subs {
		sub.Cancel()
	}
	disposeAudio(id)
}

// disposeAudio releases native player id; 0 is none.
func disposeAudio(id int64) {
	if id != 0 {
		_, _ = audioChannel.Invoke(context.Background(), "dispose", map[string]any{"playerId": id})
	}
}

func (c *AudioPlayerController) invoke(method string, args map[string]any) error {
	c.mu.Lock()
	id := c.id
	if id != 0 {
		c.calls++
	}
	c.mu.Unlock()
	if id == 0 {
		return platform.ErrDisposed
	}
	defer c.answered()
	if args == nil {
		args = map[string]any{}
	}
	args["playerId"] = id
	if _, err := audioChannel.Invoke(context.Background(), method, args); err != nil {
		return fmt.Errorf("media audio %s: %w", method, err)
	}
	return nil
}

// answered ends a call, sending the dispose Dispose left to the last one.
func (c *AudioPlayerController) answered() {
	c.mu.Lock()
	c.calls--
	var id int64
	if c.calls == 0 {
		id, c.disposeID = c.disposeID, 0
	}
	c.mu.Unlock()
	disposeAudio(id)
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
	c.setState(PlaybackState(state))
	c.setProgress(Progress{
		Position: time.Duration(positionMs) * time.Millisecond,
		Duration: time.Duration(durationMs) * time.Millisecond,
		Buffered: time.Duration(bufferedMs) * time.Millisecond,
	})
}

func (c *AudioPlayerController) handleError(data any) {
	m, ok := c.mine(data)
	if !ok {
		return
	}
	code, _ := stringArg(m, "code")
	message, _ := stringArg(m, "message")
	c.fail(code, message)
}

func reportAudioStreamError(err error) {
	errors.Report(&errors.DriftError{
		Op:      "media.AudioPlayerController",
		Kind:    errors.KindPlatform,
		Channel: audioEventsChannel,
		Err:     err,
	})
}
