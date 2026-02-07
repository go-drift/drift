package platform

import (
	"testing"
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

func TestAudioPlayerController_PlayAutoLoads(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
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

	c := NewAudioPlayerController()
	defer c.Dispose()

	c.Play("https://example.com/song.mp3")
	c.Stop()

	// After Stop, the loaded URL is cleared, so Play should re-load.
	c.Play("https://example.com/song.mp3")
}
