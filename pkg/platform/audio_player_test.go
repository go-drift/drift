package platform

import (
	"testing"
	"time"
)

func TestAudioPlayerController_Lifecycle(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	if c == nil {
		t.Fatal("expected non-nil controller")
	}

	c.Dispose()
}

func TestAudioPlayerController_MultiInstance(t *testing.T) {
	setupTestBridge(t)

	c1 := NewAudioPlayerController()
	c2 := NewAudioPlayerController()

	if c1.id == c2.id {
		t.Error("expected different IDs for each controller")
	}

	c1.Dispose()
	c2.Dispose()
}

func TestAudioPlayerController_LoadAndPlay(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	// Load prepares the URL, Play starts playback.
	if err := c.Load("https://example.com/song.mp3"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}

	// Load a different URL.
	if err := c.Load("https://example.com/other.mp3"); err != nil {
		t.Fatalf("Load (second): %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play (second): %v", err)
	}
}

func TestAudioPlayerController_StateGetters_DefaultValues(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	if c.State() != PlaybackStateIdle {
		t.Errorf("initial State(): got %v, want Idle", c.State())
	}
	if c.Position() != 0 {
		t.Error("initial Position() should be 0")
	}
	if c.Duration() != 0 {
		t.Error("initial Duration() should be 0")
	}
	if c.Buffered() != 0 {
		t.Error("initial Buffered() should be 0")
	}
}

// sendAudioEvent simulates a native playback event arriving for the given controller.
func sendAudioEvent(t *testing.T, c *AudioPlayerController, state int, posMs, durMs, bufMs int64) {
	t.Helper()
	data, err := DefaultCodec.Encode(map[string]any{
		"playerId":      c.id,
		"playbackState": state,
		"positionMs":    posMs,
		"durationMs":    durMs,
		"bufferedMs":    bufMs,
	})
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	if err := HandleEvent("drift/audio_player/events", data); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}

// sendAudioError simulates a native error event arriving for the given controller.
func sendAudioError(t *testing.T, c *AudioPlayerController, code, message string) {
	t.Helper()
	data, err := DefaultCodec.Encode(map[string]any{
		"playerId": c.id,
		"code":     code,
		"message":  message,
	})
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if err := HandleEvent("drift/audio_player/errors", data); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}

func TestAudioPlayerController_StateCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	var received []PlaybackState
	c.OnPlaybackStateChanged = func(state PlaybackState) {
		received = append(received, state)
	}

	sendAudioEvent(t, c, 1, 0, 0, 0)        // Buffering
	sendAudioEvent(t, c, 2, 0, 180000, 0)   // Playing
	sendAudioEvent(t, c, 2, 500, 180000, 0) // Playing again (dedup)
	sendAudioEvent(t, c, 4, 500, 180000, 0) // Paused

	want := []PlaybackState{PlaybackStateBuffering, PlaybackStatePlaying, PlaybackStatePaused}
	if len(received) != len(want) {
		t.Fatalf("callback count: got %d, want %d", len(received), len(want))
	}
	for i := range want {
		if received[i] != want[i] {
			t.Errorf("callback[%d]: got %v, want %v", i, received[i], want[i])
		}
	}
}

func TestAudioPlayerController_PositionCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	var gotPos, gotDur, gotBuf time.Duration
	c.OnPositionChanged = func(position, duration, buffered time.Duration) {
		gotPos = position
		gotDur = duration
		gotBuf = buffered
	}

	sendAudioEvent(t, c, 2, 5000, 120000, 30000)

	if gotPos != 5*time.Second {
		t.Errorf("position: got %v, want 5s", gotPos)
	}
	if gotDur != 2*time.Minute {
		t.Errorf("duration: got %v, want 2m0s", gotDur)
	}
	if gotBuf != 30*time.Second {
		t.Errorf("buffered: got %v, want 30s", gotBuf)
	}
}

func TestAudioPlayerController_StateGetters_CachedFromEvents(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	sendAudioEvent(t, c, 2, 5000, 120000, 30000)

	if c.State() != PlaybackStatePlaying {
		t.Errorf("State(): got %v, want Playing", c.State())
	}
	if c.Position() != 5*time.Second {
		t.Errorf("Position(): got %v, want 5s", c.Position())
	}
	if c.Duration() != 2*time.Minute {
		t.Errorf("Duration(): got %v, want 2m0s", c.Duration())
	}
	if c.Buffered() != 30*time.Second {
		t.Errorf("Buffered(): got %v, want 30s", c.Buffered())
	}
}

func TestAudioPlayerController_ErrorCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	var gotCode, gotMsg string
	c.OnError = func(code, message string) {
		gotCode = code
		gotMsg = message
	}

	sendAudioError(t, c, "source_error", "network timeout")

	if gotCode != "source_error" {
		t.Errorf("error code: got %q, want %q", gotCode, "source_error")
	}
	if gotMsg != "network timeout" {
		t.Errorf("error message: got %q, want %q", gotMsg, "network timeout")
	}
}

func TestAudioPlayerController_NilCallbacksDoNotPanic(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	// No callbacks set; these should not panic.
	sendAudioEvent(t, c, 2, 1000, 60000, 5000)
	sendAudioError(t, c, "playback_failed", "test")
}

func TestAudioPlayerController_TransportMethods(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	// All transport methods should execute without error.
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Load", func() error { return c.Load("https://example.com/song.mp3") }},
		{"Play", func() error { return c.Play() }},
		{"Pause", func() error { return c.Pause() }},
		{"SeekTo", func() error { return c.SeekTo(30 * time.Second) }},
		{"SetVolume", func() error { return c.SetVolume(0.5) }},
		{"SetLooping", func() error { return c.SetLooping(true) }},
		{"SetPlaybackSpeed", func() error { return c.SetPlaybackSpeed(1.5) }},
		{"Stop", func() error { return c.Stop() }},
	} {
		if err := tc.fn(); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

func TestAudioPlayerController_PlayPauseSeekCycle(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	var states []PlaybackState
	c.OnPlaybackStateChanged = func(state PlaybackState) {
		states = append(states, state)
	}

	// Simulate: load, play, buffering, playing, pause, seek, resume, completed
	if err := c.Load("https://example.com/song.mp3"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	sendAudioEvent(t, c, 1, 0, 0, 0)     // Buffering
	sendAudioEvent(t, c, 2, 0, 60000, 0) // Playing
	if err := c.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	sendAudioEvent(t, c, 4, 15000, 60000, 60000) // Paused at 15s
	if err := c.SeekTo(30 * time.Second); err != nil {
		t.Fatalf("SeekTo: %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play (resume): %v", err)
	}
	sendAudioEvent(t, c, 2, 30000, 60000, 60000) // Playing from 30s
	sendAudioEvent(t, c, 3, 60000, 60000, 60000) // Completed

	want := []PlaybackState{
		PlaybackStateBuffering,
		PlaybackStatePlaying,
		PlaybackStatePaused,
		PlaybackStatePlaying,
		PlaybackStateCompleted,
	}
	if len(states) != len(want) {
		t.Fatalf("state count: got %d, want %d\ngot: %v", len(states), len(want), states)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Errorf("state[%d]: got %v, want %v", i, states[i], want[i])
		}
	}

	// Final cached state should be Completed.
	if c.State() != PlaybackStateCompleted {
		t.Errorf("final State(): got %v, want Completed", c.State())
	}
	if c.Position() != 60*time.Second {
		t.Errorf("final Position(): got %v, want 1m0s", c.Position())
	}
}

func TestAudioPlayerController_PlayAfterStop(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	var states []PlaybackState
	c.OnPlaybackStateChanged = func(state PlaybackState) {
		states = append(states, state)
	}

	// Load and play.
	if err := c.Load("https://example.com/song.mp3"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	sendAudioEvent(t, c, 1, 0, 0, 0)         // Buffering
	sendAudioEvent(t, c, 2, 0, 180000, 0)     // Playing
	sendAudioEvent(t, c, 2, 5000, 180000, 0)  // still Playing, no state change

	// Stop resets to idle.
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	sendAudioEvent(t, c, 0, 0, 180000, 0) // Idle

	// Play again after stop should work.
	if err := c.Play(); err != nil {
		t.Fatalf("Play (after stop): %v", err)
	}
	sendAudioEvent(t, c, 1, 0, 180000, 0)    // Buffering
	sendAudioEvent(t, c, 2, 0, 180000, 0)    // Playing

	want := []PlaybackState{
		PlaybackStateBuffering, // initial buffer
		PlaybackStatePlaying,   // first play
		PlaybackStateIdle,      // stop
		PlaybackStateBuffering, // restart buffer
		PlaybackStatePlaying,   // restart play
	}
	if len(states) != len(want) {
		t.Fatalf("state count: got %d, want %d\ngot: %v", len(states), len(want), states)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Errorf("state[%d]: got %v, want %v", i, states[i], want[i])
		}
	}
}

func TestAudioPlayerController_StopResetsPosition(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	// Play to a mid-stream position.
	sendAudioEvent(t, c, 2, 45000, 180000, 90000) // Playing at 45s

	if c.Position() != 45*time.Second {
		t.Errorf("Position before stop: got %v, want 45s", c.Position())
	}

	// Stop resets position to zero.
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	sendAudioEvent(t, c, 0, 0, 180000, 0) // Idle at 0

	if c.State() != PlaybackStateIdle {
		t.Errorf("State after stop: got %v, want Idle", c.State())
	}
	if c.Position() != 0 {
		t.Errorf("Position after stop: got %v, want 0", c.Position())
	}
}

func TestAudioPlayerController_MethodsReturnErrDisposedAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	c.Dispose()

	// All methods should return ErrDisposed after Dispose.
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Load", func() error { return c.Load("https://example.com/song.mp3") }},
		{"Play", func() error { return c.Play() }},
		{"Pause", func() error { return c.Pause() }},
		{"Stop", func() error { return c.Stop() }},
		{"SeekTo", func() error { return c.SeekTo(time.Second) }},
		{"SetVolume", func() error { return c.SetVolume(0.5) }},
		{"SetLooping", func() error { return c.SetLooping(true) }},
		{"SetPlaybackSpeed", func() error { return c.SetPlaybackSpeed(1.5) }},
	} {
		if err := tc.fn(); err != ErrDisposed {
			t.Errorf("%s after Dispose: got %v, want ErrDisposed", tc.name, err)
		}
	}

	if c.State() != PlaybackStateIdle {
		t.Errorf("State() after Dispose: got %v, want Idle", c.State())
	}
	if c.Position() != 0 {
		t.Error("Position() after Dispose should be 0")
	}
}

func TestAudioPlayerController_DoubleDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	c.Dispose()
	c.Dispose() // second call should be a safe no-op
}

func TestAudioPlayerController_EventAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	id := c.id

	var callbackFired bool
	c.OnPlaybackStateChanged = func(PlaybackState) { callbackFired = true }
	c.OnPositionChanged = func(_, _, _ time.Duration) { callbackFired = true }
	c.OnError = func(_, _ string) { callbackFired = true }

	c.Dispose()

	// Simulate native events arriving for the now-disposed player ID.
	// The registry lookup should return nil, so nothing should happen.
	eventData, err := DefaultCodec.Encode(map[string]any{
		"playerId":      id,
		"playbackState": 2,
		"positionMs":    int64(1000),
		"durationMs":    int64(60000),
		"bufferedMs":    int64(5000),
	})
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	if err := HandleEvent("drift/audio_player/events", eventData); err != nil {
		t.Fatalf("HandleEvent (state): %v", err)
	}

	errorData, err := DefaultCodec.Encode(map[string]any{
		"playerId": id,
		"code":     "test",
		"message":  "post-dispose",
	})
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if err := HandleEvent("drift/audio_player/errors", errorData); err != nil {
		t.Fatalf("HandleEvent (error): %v", err)
	}

	if callbackFired {
		t.Error("callbacks should not fire for a disposed controller")
	}
}

func TestAudioPlayerController_EventRoutesToCorrectInstance(t *testing.T) {
	setupTestBridge(t)

	c1 := NewAudioPlayerController()
	c2 := NewAudioPlayerController()
	defer c1.Dispose()
	defer c2.Dispose()

	var c1State, c2State PlaybackState
	c1.OnPlaybackStateChanged = func(s PlaybackState) { c1State = s }
	c2.OnPlaybackStateChanged = func(s PlaybackState) { c2State = s }

	// Send event only to c2.
	sendAudioEvent(t, c2, 2, 0, 0, 0)

	if c1State != PlaybackStateIdle {
		t.Errorf("c1 should be Idle, got %v", c1State)
	}
	if c2State != PlaybackStatePlaying {
		t.Errorf("c2 should be Playing, got %v", c2State)
	}
}
