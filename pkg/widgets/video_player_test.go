package widgets

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
)

func TestVolumeChanged(t *testing.T) {
	v1 := 0.5
	v2 := 0.8
	v1dup := 0.5

	tests := []struct {
		name string
		a, b *float64
		want bool
	}{
		{"both nil", nil, nil, false},
		{"a nil b set", nil, &v1, true},
		{"a set b nil", &v1, nil, true},
		{"same value", &v1, &v1dup, false},
		{"different value", &v1, &v2, true},
		{"same pointer", &v1, &v1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := volumeChanged(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("volumeChanged(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVideoPlayerController_NilViewMethods(t *testing.T) {
	// All methods should be safe to call with no view bound.
	c := &VideoPlayerController{}

	c.Play()
	c.Pause()
	c.SeekTo(1000)
	c.SetVolume(0.5)
	c.SetLooping(true)
	c.SetPlaybackSpeed(2.0)

	if c.State() != platform.PlaybackStateIdle {
		t.Errorf("State() with nil view: got %d, want %d", c.State(), platform.PlaybackStateIdle)
	}
	if c.PositionMs() != 0 {
		t.Error("PositionMs() with nil view should be 0")
	}
	if c.DurationMs() != 0 {
		t.Error("DurationMs() with nil view should be 0")
	}
	if c.BufferedMs() != 0 {
		t.Error("BufferedMs() with nil view should be 0")
	}
}

func TestVideoPlayerController_SetAndClearView(t *testing.T) {
	c := &VideoPlayerController{}

	view := platform.NewVideoPlayerView(42, nil)
	c.setView(view)

	c.mu.RLock()
	hasView := c.view != nil
	c.mu.RUnlock()
	if !hasView {
		t.Error("expected view to be set after setView")
	}

	c.clearView()

	c.mu.RLock()
	hasView = c.view != nil
	c.mu.RUnlock()
	if hasView {
		t.Error("expected view to be nil after clearView")
	}
}

// --- DidUpdateWidget tests ---

// videoBridge captures native method invocations for assertions.
type videoBridge struct {
	mu    sync.Mutex
	calls []videoBridgeCall
}

type videoBridgeCall struct {
	channel string
	method  string
	args    map[string]any
}

func (b *videoBridge) InvokeMethod(channel, method string, argsData []byte) ([]byte, error) {
	var args map[string]any
	if len(argsData) > 0 {
		json.Unmarshal(argsData, &args)
	}
	b.mu.Lock()
	b.calls = append(b.calls, videoBridgeCall{channel: channel, method: method, args: args})
	b.mu.Unlock()
	return platform.DefaultCodec.Encode(nil)
}

func (b *videoBridge) StartEventStream(string) error { return nil }
func (b *videoBridge) StopEventStream(string) error  { return nil }

func (b *videoBridge) findCall(viewMethod string) *videoBridgeCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.calls {
		c := &b.calls[i]
		if c.method == "invokeViewMethod" && c.args != nil {
			if m, _ := c.args["method"].(string); m == viewMethod {
				return c
			}
		}
	}
	return nil
}

func setupVideoBridge(t *testing.T) *videoBridge {
	bridge := &videoBridge{}
	platform.SetNativeBridge(bridge)
	t.Cleanup(func() { platform.ResetForTest() })
	return bridge
}

// newVideoPlayerState creates a videoPlayerState with an element whose current
// widget is newWidget, and a pre-set platform view so DidUpdateWidget can
// invoke methods on it.
func newVideoPlayerState(t *testing.T, newWidget VideoPlayer) *videoPlayerState {
	t.Helper()
	element := core.NewStatefulElement(newWidget, nil)
	state := &videoPlayerState{
		element:      element,
		platformView: platform.NewVideoPlayerView(1, nil),
	}
	return state
}

func TestDidUpdateWidget_URLChange(t *testing.T) {
	bridge := setupVideoBridge(t)

	newWidget := VideoPlayer{URL: "https://example.com/new.mp4"}
	state := newVideoPlayerState(t, newWidget)

	oldWidget := VideoPlayer{URL: "https://example.com/old.mp4"}
	state.DidUpdateWidget(oldWidget)

	call := bridge.findCall("loadUrl")
	if call == nil {
		t.Fatal("expected loadUrl call when URL changes")
	}
	if url, _ := call.args["url"].(string); url != "https://example.com/new.mp4" {
		t.Errorf("loadUrl url: got %q, want %q", url, "https://example.com/new.mp4")
	}
}

func TestDidUpdateWidget_URLUnchanged(t *testing.T) {
	bridge := setupVideoBridge(t)

	newWidget := VideoPlayer{URL: "https://example.com/same.mp4"}
	state := newVideoPlayerState(t, newWidget)

	oldWidget := VideoPlayer{URL: "https://example.com/same.mp4"}
	state.DidUpdateWidget(oldWidget)

	call := bridge.findCall("loadUrl")
	if call != nil {
		t.Error("should not call loadUrl when URL is unchanged")
	}
}

func TestDidUpdateWidget_LoopingChange(t *testing.T) {
	bridge := setupVideoBridge(t)

	newWidget := VideoPlayer{URL: "https://example.com/v.mp4", Looping: true}
	state := newVideoPlayerState(t, newWidget)

	oldWidget := VideoPlayer{URL: "https://example.com/v.mp4", Looping: false}
	state.DidUpdateWidget(oldWidget)

	call := bridge.findCall("setLooping")
	if call == nil {
		t.Fatal("expected setLooping call when Looping changes")
	}
	if looping, _ := call.args["looping"].(bool); !looping {
		t.Error("setLooping: expected true")
	}
}

func TestDidUpdateWidget_VolumeChange(t *testing.T) {
	bridge := setupVideoBridge(t)

	vol := 0.3
	newWidget := VideoPlayer{URL: "https://example.com/v.mp4", Volume: &vol}
	state := newVideoPlayerState(t, newWidget)

	oldWidget := VideoPlayer{URL: "https://example.com/v.mp4"}
	state.DidUpdateWidget(oldWidget)

	call := bridge.findCall("setVolume")
	if call == nil {
		t.Fatal("expected setVolume call when Volume changes from nil to value")
	}
	if v, _ := call.args["volume"].(float64); v != 0.3 {
		t.Errorf("setVolume: got %f, want 0.3", v)
	}
}

func TestDidUpdateWidget_ControllerSwap(t *testing.T) {
	setupVideoBridge(t)

	oldController := &VideoPlayerController{}
	newController := &VideoPlayerController{}

	newWidget := VideoPlayer{URL: "https://example.com/v.mp4", Controller: newController}
	state := newVideoPlayerState(t, newWidget)

	// Simulate old controller having the view bound
	oldController.setView(state.platformView)

	oldWidget := VideoPlayer{URL: "https://example.com/v.mp4", Controller: oldController}
	state.DidUpdateWidget(oldWidget)

	// Old controller should have its view cleared
	oldController.mu.RLock()
	oldHasView := oldController.view != nil
	oldController.mu.RUnlock()
	if oldHasView {
		t.Error("old controller should have view cleared after swap")
	}

	// New controller should have the view set
	newController.mu.RLock()
	newHasView := newController.view != nil
	newController.mu.RUnlock()
	if !newHasView {
		t.Error("new controller should have view set after swap")
	}
}

func TestDidUpdateWidget_NoPlatformView(t *testing.T) {
	bridge := setupVideoBridge(t)

	newWidget := VideoPlayer{URL: "https://example.com/new.mp4", Looping: true}
	element := core.NewStatefulElement(newWidget, nil)
	state := &videoPlayerState{
		element:      element,
		platformView: nil, // no platform view yet
	}

	oldWidget := VideoPlayer{URL: "https://example.com/old.mp4", Looping: false}
	state.DidUpdateWidget(oldWidget)

	// No calls should be made when platform view doesn't exist
	if call := bridge.findCall("loadUrl"); call != nil {
		t.Error("should not call loadUrl without a platform view")
	}
	if call := bridge.findCall("setLooping"); call != nil {
		t.Error("should not call setLooping without a platform view")
	}
}
