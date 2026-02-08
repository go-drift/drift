package platform

import (
	"testing"
	"time"
)

func TestVideoPlayerController_Lifecycle(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	if c == nil {
		t.Fatal("expected non-nil controller")
	}
	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID")
	}

	c.Dispose()

	if c.ViewID() != 0 {
		t.Error("expected zero ViewID after Dispose")
	}
}

func TestVideoPlayerController_StateGetters_DefaultValues(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
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

func TestVideoPlayerController_ViewID(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID from controller")
	}
}

// sendVideoViewEvent simulates a native event arriving for a video platform view.
func sendVideoViewEvent(t *testing.T, method string, args map[string]any) {
	t.Helper()
	args["method"] = method
	data, err := DefaultCodec.Encode(args)
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	if err := HandleEvent("drift/platform_views", data); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}

func TestVideoPlayerController_PlaybackStateCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	var received []PlaybackState
	c.OnPlaybackStateChanged = func(state PlaybackState) {
		received = append(received, state)
	}

	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  1, // Buffering
	})
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  2, // Playing
	})
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  2, // Playing again (dedup)
	})
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  4, // Paused
	})

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

func TestVideoPlayerController_PositionCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	var gotPos, gotDur, gotBuf time.Duration
	c.OnPositionChanged = func(position, duration, buffered time.Duration) {
		gotPos = position
		gotDur = duration
		gotBuf = buffered
	}

	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     c.ViewID(),
		"positionMs": int64(5000),
		"durationMs": int64(120000),
		"bufferedMs": int64(30000),
	})

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

func TestVideoPlayerController_ErrorCallback(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	var gotCode, gotMsg string
	c.OnError = func(code, message string) {
		gotCode = code
		gotMsg = message
	}

	sendVideoViewEvent(t, "onVideoError", map[string]any{
		"viewId":  c.ViewID(),
		"code":    "source_error",
		"message": "network timeout",
	})

	if gotCode != "source_error" {
		t.Errorf("error code: got %q, want %q", gotCode, "source_error")
	}
	if gotMsg != "network timeout" {
		t.Errorf("error message: got %q, want %q", gotMsg, "network timeout")
	}
}

func TestVideoPlayerController_NilCallbacksDoNotPanic(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	// No callbacks set; these should not panic.
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  2,
	})
	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     c.ViewID(),
		"positionMs": int64(1000),
		"durationMs": int64(60000),
		"bufferedMs": int64(5000),
	})
	sendVideoViewEvent(t, "onVideoError", map[string]any{
		"viewId":  c.ViewID(),
		"code":    "test",
		"message": "test",
	})
}

func TestVideoPlayerController_StateGetters_CachedFromEvents(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(),
		"state":  2, // Playing
	})
	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     c.ViewID(),
		"positionMs": int64(5000),
		"durationMs": int64(120000),
		"bufferedMs": int64(30000),
	})

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

func TestVideoPlayerController_TransportMethods(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	// All transport methods should execute without error.
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Load", func() error { return c.Load("https://example.com/video.mp4") }},
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

func TestVideoPlayerController_PlayAfterStop(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	var states []PlaybackState
	c.OnPlaybackStateChanged = func(state PlaybackState) {
		states = append(states, state)
	}

	// Load and play.
	if err := c.Load("https://example.com/video.mp4"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 1, // Buffering
	})
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 2, // Playing
	})

	// Stop resets to idle.
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 0, // Idle
	})

	// Play again after stop should work.
	if err := c.Play(); err != nil {
		t.Fatalf("Play (after stop): %v", err)
	}
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 1, // Buffering
	})
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 2, // Playing
	})

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

func TestVideoPlayerController_StopResetsPosition(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	defer c.Dispose()

	// Play to a mid-stream position.
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 2, // Playing
	})
	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     c.ViewID(),
		"positionMs": int64(45000),
		"durationMs": int64(180000),
		"bufferedMs": int64(90000),
	})

	if c.Position() != 45*time.Second {
		t.Errorf("Position before stop: got %v, want 45s", c.Position())
	}

	// Stop resets position to zero.
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": c.ViewID(), "state": 0, // Idle
	})
	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     c.ViewID(),
		"positionMs": int64(0),
		"durationMs": int64(180000),
		"bufferedMs": int64(0),
	})

	if c.State() != PlaybackStateIdle {
		t.Errorf("State after stop: got %v, want Idle", c.State())
	}
	if c.Position() != 0 {
		t.Errorf("Position after stop: got %v, want 0", c.Position())
	}
}

func TestVideoPlayerController_DoubleDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	c.Dispose()
	c.Dispose() // second call should be a safe no-op
}

func TestVideoPlayerController_EventAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	viewID := c.ViewID()

	var callbackFired bool
	c.OnPlaybackStateChanged = func(PlaybackState) { callbackFired = true }
	c.OnPositionChanged = func(_, _, _ time.Duration) { callbackFired = true }
	c.OnError = func(_, _ string) { callbackFired = true }

	c.Dispose()

	// Simulate native events arriving for the now-disposed view ID.
	// The registry lookup should return nil, so nothing should happen.
	sendVideoViewEvent(t, "onPlaybackStateChanged", map[string]any{
		"viewId": viewID,
		"state":  2,
	})
	sendVideoViewEvent(t, "onPositionChanged", map[string]any{
		"viewId":     viewID,
		"positionMs": int64(1000),
		"durationMs": int64(60000),
		"bufferedMs": int64(5000),
	})
	sendVideoViewEvent(t, "onVideoError", map[string]any{
		"viewId":  viewID,
		"code":    "test",
		"message": "post-dispose",
	})

	if callbackFired {
		t.Error("callbacks should not fire for a disposed controller")
	}
}

func TestVideoPlayerController_MethodsReturnErrDisposedAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	c.Dispose()

	// All methods should return ErrDisposed after Dispose.
	for _, tc := range []struct {
		name string
		fn   func() error
	}{
		{"Load", func() error { return c.Load("https://example.com/video.mp4") }},
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
