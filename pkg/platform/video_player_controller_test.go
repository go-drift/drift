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
	c.Load("https://example.com/video.mp4")
	c.Play()
	c.Pause()
	c.SeekTo(30 * time.Second)
	c.SetVolume(0.5)
	c.SetLooping(true)
	c.SetPlaybackSpeed(1.5)
	c.Stop()
}

func TestVideoPlayerController_MethodsNoOpAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c := NewVideoPlayerController()
	c.Dispose()

	// All methods should silently no-op after Dispose.
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
		if err := tc.fn(); err != nil {
			t.Errorf("%s after Dispose: got %v, want nil", tc.name, err)
		}
	}

	if c.State() != PlaybackStateIdle {
		t.Errorf("State() after Dispose: got %v, want Idle", c.State())
	}
	if c.Position() != 0 {
		t.Error("Position() after Dispose should be 0")
	}
}
