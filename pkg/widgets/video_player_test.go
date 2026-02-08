package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

func TestVideoPlayer_NilController(t *testing.T) {
	// Widget should not panic when Controller is nil.
	w := VideoPlayer{
		Width:  320,
		Height: 240,
	}

	if w.Key() != nil {
		t.Error("expected nil key")
	}

	elem := w.CreateElement()
	if elem == nil {
		t.Error("expected non-nil element")
	}
}

func TestVideoPlayer_WithController(t *testing.T) {
	platform.SetupTestBridge(t.Cleanup)

	c := platform.NewVideoPlayerController()
	defer c.Dispose()

	w := VideoPlayer{
		Controller: c,
		Height:     225,
	}

	elem := w.CreateElement()
	if elem == nil {
		t.Error("expected non-nil element")
	}

	// Controller should have a valid ViewID.
	if c.ViewID() == 0 {
		t.Error("expected non-zero ViewID from controller")
	}
}
