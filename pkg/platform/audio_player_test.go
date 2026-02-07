package platform

import (
	"testing"
)

func TestAudioPlayerController_Lifecycle(t *testing.T) {
	setupTestBridge(t)

	c, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("NewAudioPlayerController failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil controller")
	}
	if c.disposed {
		t.Fatal("new controller should not be disposed")
	}

	c.Dispose()
	if !c.disposed {
		t.Error("controller should be disposed after Dispose()")
	}
}

func TestAudioPlayerController_SingletonError(t *testing.T) {
	setupTestBridge(t)

	c, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("first NewAudioPlayerController failed: %v", err)
	}
	defer c.Dispose()

	// Creating a second controller should return an error.
	c2, err := NewAudioPlayerController()
	if err == nil {
		t.Fatal("expected error when creating second controller")
	}
	if c2 != nil {
		t.Fatal("expected nil controller on error")
	}
}

func TestAudioPlayerController_ReuseAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c1, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("first NewAudioPlayerController failed: %v", err)
	}
	c1.Dispose()

	// Creating a new controller after disposing the first should succeed.
	c2, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("second NewAudioPlayerController failed: %v", err)
	}
	if c2 == nil {
		t.Fatal("expected non-nil controller after previous was disposed")
	}
	c2.Dispose()
}

func TestAudioPlayerController_PlayAutoLoads(t *testing.T) {
	setupTestBridge(t)

	c, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("NewAudioPlayerController failed: %v", err)
	}
	defer c.Dispose()

	// First Play should load and play.
	c.Play("https://example.com/song.mp3")

	// Second Play with same URL should only play (not re-load).
	c.Play("https://example.com/song.mp3")

	// Play with different URL should load the new URL.
	c.Play("https://example.com/other.mp3")
}

func TestAudioPlayerController_StopResetsLoadedURL(t *testing.T) {
	setupTestBridge(t)

	c, err := NewAudioPlayerController()
	if err != nil {
		t.Fatalf("NewAudioPlayerController failed: %v", err)
	}
	defer c.Dispose()

	c.Play("https://example.com/song.mp3")
	c.Stop()

	// After Stop, the loaded URL is cleared, so Play should re-load.
	c.Play("https://example.com/song.mp3")
}
