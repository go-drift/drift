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
	if c.disposed {
		t.Fatal("new controller should not be disposed")
	}

	if err := c.Dispose(); err != nil {
		t.Fatalf("Dispose failed: %v", err)
	}
	if !c.disposed {
		t.Error("controller should be disposed after Dispose()")
	}
}

func TestAudioPlayerController_SingletonPanic(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when creating second controller")
		}
	}()

	// This should panic.
	NewAudioPlayerController()
}

func TestAudioPlayerController_ReuseAfterDispose(t *testing.T) {
	setupTestBridge(t)

	c1 := NewAudioPlayerController()
	c1.Dispose()

	// Creating a new controller after disposing the first should succeed.
	c2 := NewAudioPlayerController()
	if c2 == nil {
		t.Fatal("expected non-nil controller after previous was disposed")
	}
	c2.Dispose()
}

func TestAudioPlayerController_StreamsNotNil(t *testing.T) {
	setupTestBridge(t)

	c := NewAudioPlayerController()
	defer c.Dispose()

	if c.States() == nil {
		t.Error("States() should not be nil")
	}
	if c.Errors() == nil {
		t.Error("Errors() should not be nil")
	}
}
