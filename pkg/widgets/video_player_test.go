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
	setupVideoBridge(t)

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

// setupVideoBridge configures a test bridge for widget-level tests.
func setupVideoBridge(t *testing.T) {
	t.Helper()
	platform.SetNativeBridge(&widgetTestBridge{})
	t.Cleanup(func() { platform.ResetForTest() })
}

type widgetTestBridge struct{}

func (b *widgetTestBridge) InvokeMethod(channel, method string, argsData []byte) ([]byte, error) {
	return platform.DefaultCodec.Encode(nil)
}

func (b *widgetTestBridge) StartEventStream(string) error { return nil }
func (b *widgetTestBridge) StopEventStream(string) error  { return nil }
