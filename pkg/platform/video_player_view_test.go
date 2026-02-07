package platform

import (
	"testing"
)

func TestVideoPlayerView_HandlePlaybackStateChanged(t *testing.T) {
	setupTestBridge(t)

	var receivedState PlaybackState
	client := &testVideoPlayerClient{
		onPlaybackStateChanged: func(state PlaybackState) {
			receivedState = state
		},
	}

	view := NewVideoPlayerView(1, client)

	view.handlePlaybackStateChanged(PlaybackStatePlaying)

	if receivedState != PlaybackStatePlaying {
		t.Errorf("expected state %d, got %d", PlaybackStatePlaying, receivedState)
	}
	if view.State() != PlaybackStatePlaying {
		t.Errorf("cached state: expected %d, got %d", PlaybackStatePlaying, view.State())
	}
}

func TestVideoPlayerView_HandlePositionChanged(t *testing.T) {
	setupTestBridge(t)

	var gotPos, gotDur, gotBuf int64
	client := &testVideoPlayerClient{
		onPositionChanged: func(positionMs, durationMs, bufferedMs int64) {
			gotPos = positionMs
			gotDur = durationMs
			gotBuf = bufferedMs
		},
	}

	view := NewVideoPlayerView(1, client)
	view.handlePositionChanged(5000, 120000, 30000)

	if gotPos != 5000 || gotDur != 120000 || gotBuf != 30000 {
		t.Errorf("position callback: got (%d, %d, %d), want (5000, 120000, 30000)", gotPos, gotDur, gotBuf)
	}
	if view.PositionMs() != 5000 {
		t.Errorf("cached position: got %d, want 5000", view.PositionMs())
	}
	if view.DurationMs() != 120000 {
		t.Errorf("cached duration: got %d, want 120000", view.DurationMs())
	}
	if view.BufferedMs() != 30000 {
		t.Errorf("cached buffered: got %d, want 30000", view.BufferedMs())
	}
}

func TestVideoPlayerView_HandleError(t *testing.T) {
	setupTestBridge(t)

	var gotCode, gotMsg string
	client := &testVideoPlayerClient{
		onError: func(code string, message string) {
			gotCode = code
			gotMsg = message
		},
	}

	view := NewVideoPlayerView(1, client)
	view.handleError("ERR_DECODE", "codec not supported")

	if gotCode != "ERR_DECODE" {
		t.Errorf("error code: got %q, want %q", gotCode, "ERR_DECODE")
	}
	if gotMsg != "codec not supported" {
		t.Errorf("error message: got %q, want %q", gotMsg, "codec not supported")
	}
}

func TestVideoPlayerView_NilClientDoesNotPanic(t *testing.T) {
	setupTestBridge(t)

	view := NewVideoPlayerView(1, nil)

	// None of these should panic with a nil client.
	view.handlePlaybackStateChanged(PlaybackStatePlaying)
	view.handlePositionChanged(1000, 2000, 1500)
	view.handleError("ERR", "test")
}

func TestVideoPlayerView_SetClient(t *testing.T) {
	setupTestBridge(t)

	view := NewVideoPlayerView(1, nil)

	var called bool
	view.SetClient(&testVideoPlayerClient{
		onPlaybackStateChanged: func(state PlaybackState) {
			called = true
		},
	})

	view.handlePlaybackStateChanged(PlaybackStatePaused)

	if !called {
		t.Error("expected callback after SetClient")
	}
}

// testVideoPlayerClient is a test implementation of VideoPlayerClient.
type testVideoPlayerClient struct {
	onPlaybackStateChanged func(state PlaybackState)
	onPositionChanged      func(positionMs, durationMs, bufferedMs int64)
	onError                func(code string, message string)
}

func (c *testVideoPlayerClient) OnPlaybackStateChanged(state PlaybackState) {
	if c.onPlaybackStateChanged != nil {
		c.onPlaybackStateChanged(state)
	}
}

func (c *testVideoPlayerClient) OnPositionChanged(positionMs, durationMs, bufferedMs int64) {
	if c.onPositionChanged != nil {
		c.onPositionChanged(positionMs, durationMs, bufferedMs)
	}
}

func (c *testVideoPlayerClient) OnError(code string, message string) {
	if c.onError != nil {
		c.onError(code, message)
	}
}
