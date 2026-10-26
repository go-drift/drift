package runtime

import (
	"testing"
)

// showControlsCalls returns the show values sent by setShowControls.
func showControlsCalls(native *fakeNative) []bool {
	var shows []bool
	for _, c := range native.calls {
		if c.method == "invokeViewMethod" && c.args["method"] == "setShowControls" {
			shows = append(shows, c.args["show"].(bool))
		}
	}
	return shows
}

func TestVideoPlayerWithoutController(t *testing.T) {
	r := VideoPlayer{Width: 320, Height: 240}.CreateRenderObject(nil).(*renderVideoPlayer)
	if id := r.PlatformViewID(); id != -1 {
		t.Errorf("PlatformViewID = %d, want -1", id)
	}
}

func TestVideoPlayerEmbedsTheControllersView(t *testing.T) {
	native := useNative(t)
	c := newVideo(t)
	r := VideoPlayer{Controller: c, Height: 225}.CreateRenderObject(nil).(*renderVideoPlayer)
	if r.PlatformViewID() != c.ViewID() {
		t.Errorf("PlatformViewID = %d, want %d", r.PlatformViewID(), c.ViewID())
	}
	if shows := showControlsCalls(native); len(shows) != 0 {
		t.Errorf("controls changed without HideControls: %v", shows)
	}
}

// HideControls reaches the native view on creation, on toggling and when
// the controller is swapped.
func TestVideoPlayerHideControls(t *testing.T) {
	native := useNative(t)
	a, b := newVideo(t), newVideo(t)

	w := VideoPlayer{Controller: a, HideControls: true}
	r := w.CreateRenderObject(nil)
	w.UpdateRenderObject(nil, r) // unchanged: nothing sent
	VideoPlayer{Controller: a}.UpdateRenderObject(nil, r)
	VideoPlayer{Controller: a, HideControls: true}.UpdateRenderObject(nil, r)
	VideoPlayer{Controller: b, HideControls: true}.UpdateRenderObject(nil, r)

	want := []bool{false, true, false, false}
	got := showControlsCalls(native)
	if len(got) != len(want) {
		t.Fatalf("setShowControls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("setShowControls = %v, want %v", got, want)
		}
	}
}
