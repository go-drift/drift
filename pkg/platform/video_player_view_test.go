package platform

import (
	"testing"
	"time"
)

func TestVideoPlayerView_HandlePlaybackStateChanged(t *testing.T) {
	setupTestBridge(t)

	var receivedState PlaybackState
	view := NewVideoPlayerView(1)
	view.OnPlaybackStateChanged = func(state PlaybackState) {
		receivedState = state
	}

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
	view := NewVideoPlayerView(1)
	view.OnPositionChanged = func(position, duration, buffered time.Duration) {
		gotPos = position
		gotDur = duration
		gotBuf = buffered
	}

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
	view := NewVideoPlayerView(1)
	view.OnError = func(code string, message string) {
		gotCode = code
		gotMsg = message
	}

	view.handleError("ERR_DECODE", "codec not supported")

	if gotCode != "ERR_DECODE" {
		t.Errorf("error code: got %q, want %q", gotCode, "ERR_DECODE")
	}
	if gotMsg != "codec not supported" {
		t.Errorf("error message: got %q, want %q", gotMsg, "codec not supported")
	}
}

func TestVideoPlayerView_NilCallbacksDoNotPanic(t *testing.T) {
	setupTestBridge(t)

	view := NewVideoPlayerView(1)

	// None of these should panic with nil callbacks.
	view.handlePlaybackStateChanged(PlaybackStatePlaying)
	view.handlePositionChanged(time.Second, 2*time.Second, time.Second+500*time.Millisecond)
	view.handleError("ERR", "test")
}
