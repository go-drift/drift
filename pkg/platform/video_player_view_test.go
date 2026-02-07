package platform

import (
	"testing"
	"time"
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

	var gotPos, gotDur, gotBuf time.Duration
	client := &testVideoPlayerClient{
		onPositionChanged: func(position, duration, buffered time.Duration) {
			gotPos = position
			gotDur = duration
			gotBuf = buffered
		},
	}

	view := NewVideoPlayerView(1, client)
	view.handlePositionChanged(5*time.Second, 2*time.Minute, 30*time.Second)

	if gotPos != 5*time.Second || gotDur != 2*time.Minute || gotBuf != 30*time.Second {
		t.Errorf("position callback: got (%v, %v, %v), want (5s, 2m0s, 30s)", gotPos, gotDur, gotBuf)
	}
	if view.Position() != 5*time.Second {
		t.Errorf("cached position: got %v, want 5s", view.Position())
	}
	if view.Duration() != 2*time.Minute {
		t.Errorf("cached duration: got %v, want 2m0s", view.Duration())
	}
	if view.Buffered() != 30*time.Second {
		t.Errorf("cached buffered: got %v, want 30s", view.Buffered())
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
	view.handlePositionChanged(time.Second, 2*time.Second, time.Second+500*time.Millisecond)
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
	onPositionChanged      func(position, duration, buffered time.Duration)
	onError                func(code string, message string)
}

func (c *testVideoPlayerClient) OnPlaybackStateChanged(state PlaybackState) {
	if c.onPlaybackStateChanged != nil {
		c.onPlaybackStateChanged(state)
	}
}

func (c *testVideoPlayerClient) OnPositionChanged(position, duration, buffered time.Duration) {
	if c.onPositionChanged != nil {
		c.onPositionChanged(position, duration, buffered)
	}
}

func (c *testVideoPlayerClient) OnError(code string, message string) {
	if c.onError != nil {
		c.onError(code, message)
	}
}
