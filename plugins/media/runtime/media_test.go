package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeNative records each call and answers with nil.
type fakeNative struct {
	calls []call
}

type call struct {
	channel, method string
	args            map[string]any
}

func (f *fakeNative) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	decoded, _ := platform.DefaultCodec.Decode(args)
	m, _ := decoded.(map[string]any)
	f.calls = append(f.calls, call{channel, method, m})
	reply(platform.DefaultCodec.Encode(nil))
}

// last returns the last call on channel.
func (f *fakeNative) last(t *testing.T, channel string) call {
	t.Helper()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].channel == channel {
			return f.calls[i]
		}
	}
	t.Fatalf("no call on %s in %+v", channel, f.calls)
	return call{}
}

func useNative(t *testing.T) *fakeNative {
	t.Helper()
	f := &fakeNative{}
	platform.SetNativeBridge(f)
	platform.RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(platform.ResetForTest)
	return f
}

// send delivers a native event on channel.
func send(t *testing.T, channel string, event map[string]any) {
	t.Helper()
	data, err := platform.DefaultCodec.Encode(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEvent(channel, data); err != nil {
		t.Fatalf("HandleEvent(%s): %v", channel, err)
	}
}

func TestPlaybackStateString(t *testing.T) {
	for state, want := range map[PlaybackState]string{
		PlaybackStateIdle:      "Idle",
		PlaybackStateBuffering: "Buffering",
		PlaybackStatePlaying:   "Playing",
		PlaybackStateCompleted: "Completed",
		PlaybackStatePaused:    "Paused",
		PlaybackState(99):      "Unknown",
	} {
		if got := state.String(); got != want {
			t.Errorf("PlaybackState(%d) = %q, want %q", state, got, want)
		}
	}
}

func TestAudioCallsCarryPlayerID(t *testing.T) {
	native := useNative(t)
	a, b := NewAudioPlayerController(), NewAudioPlayerController()
	defer a.Dispose()
	defer b.Dispose()
	if a.id == b.id {
		t.Fatalf("both players have id %d", a.id)
	}
	if err := a.Load("https://example.com/a.mp3"); err != nil {
		t.Fatal(err)
	}
	got := native.last(t, audioChannelName)
	if got.method != "load" || got.args["url"] != "https://example.com/a.mp3" || got.args["playerId"] != float64(a.id) {
		t.Errorf("load call = %+v", got)
	}
	if err := b.SeekTo(1500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	got = native.last(t, audioChannelName)
	if got.method != "seekTo" || got.args["positionMs"] != float64(1500) || got.args["playerId"] != float64(b.id) {
		t.Errorf("seekTo call = %+v", got)
	}
}

// Each player sees only its own events; State changes on change only.
func TestAudioEventsReachTheirPlayer(t *testing.T) {
	useNative(t)
	a, b := NewAudioPlayerController(), NewAudioPlayerController()
	defer a.Dispose()
	defer b.Dispose()
	var states []PlaybackState
	a.State().AddListener(func() { states = append(states, a.State().Value()) })
	b.State().AddListener(func() { t.Error("b got a's event") })
	b.Errors().Listen(platform.StreamHandler[error]{OnEvent: func(error) { t.Error("b got a's error") }})

	event := func(state int, pos int64) map[string]any {
		return map[string]any{"playerId": a.id, "playbackState": state, "positionMs": pos, "durationMs": 9000, "bufferedMs": 4000}
	}
	send(t, audioEventsChannel, event(1, 0))
	send(t, audioEventsChannel, event(2, 0))
	send(t, audioEventsChannel, event(2, 500))
	send(t, audioEventsChannel, event(4, 700))

	want := []PlaybackState{PlaybackStateBuffering, PlaybackStatePlaying, PlaybackStatePaused}
	if len(states) != len(want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Errorf("states = %v, want %v", states, want)
		}
	}
	if got, want := a.Progress().Value(), (Progress{Position: 700 * time.Millisecond, Duration: 9 * time.Second, Buffered: 4 * time.Second}); got != want {
		t.Errorf("a progress = %+v, want %+v", got, want)
	}
	if b.State().Value() != PlaybackStateIdle || b.Progress().Value() != (Progress{}) {
		t.Errorf("b = %v %+v", b.State().Value(), b.Progress().Value())
	}

	var failure error
	a.Errors().Listen(platform.StreamHandler[error]{OnEvent: func(err error) { failure = err }})
	send(t, audioErrorsChannel, map[string]any{"playerId": a.id, "code": "source_error", "message": "404"})
	if !errors.Is(failure, ErrSource) || !strings.Contains(failure.Error(), "404") {
		t.Errorf("error = %v, want ErrSource with the native message", failure)
	}
}

// A disposed player tells native once, rejects calls and hears nothing.
func TestAudioDispose(t *testing.T) {
	native := useNative(t)
	a := NewAudioPlayerController()
	id := a.id
	a.State().AddListener(func() { t.Error("disposed player got an event") })
	a.Dispose()
	a.Dispose()

	disposes := 0
	for _, c := range native.calls {
		if c.channel == audioChannelName && c.method == "dispose" {
			disposes++
			if c.args["playerId"] != float64(id) {
				t.Errorf("dispose args = %v", c.args)
			}
		}
	}
	if disposes != 1 {
		t.Errorf("dispose sent %d times, want 1", disposes)
	}
	if err := a.Play(); !errors.Is(err, platform.ErrDisposed) {
		t.Errorf("Play after Dispose = %v, want ErrDisposed", err)
	}
	send(t, audioEventsChannel, map[string]any{"playerId": id, "playbackState": 2, "positionMs": 0, "durationMs": 0, "bufferedMs": 0})
}

// newVideo creates a video controller, failing the test if it cannot.
func newVideo(t *testing.T) *VideoPlayerController {
	t.Helper()
	c, err := NewVideoPlayerController()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Dispose)
	return c
}

func TestVideoCreatesItsView(t *testing.T) {
	native := useNative(t)
	c := newVideo(t)
	create := native.last(t, "drift/platform_views")
	if create.method != "create" || create.args["viewType"] != videoViewType || create.args["viewId"] != float64(c.ViewID()) {
		t.Fatalf("create call = %+v", create)
	}
	if err := c.Load("https://example.com/v.m3u8"); err != nil {
		t.Fatal(err)
	}
	load := native.last(t, "drift/platform_views")
	if load.method != "invokeViewMethod" || load.args["method"] != "load" || load.args["url"] != "https://example.com/v.m3u8" {
		t.Errorf("load call = %+v", load)
	}
}

// Native view events reach the controller through the platform view
// registry.
func TestVideoEvents(t *testing.T) {
	useNative(t)
	c := newVideo(t)
	var failure error
	c.Errors().Listen(platform.StreamHandler[error]{OnEvent: func(err error) { failure = err }})

	id := c.ViewID()
	send(t, "drift/platform_views", map[string]any{"method": "onPlaybackStateChanged", "viewId": id, "state": 2})
	send(t, "drift/platform_views", map[string]any{"method": "onPositionChanged", "viewId": id, "positionMs": 1200, "durationMs": 60000, "bufferedMs": 3000})
	send(t, "drift/platform_views", map[string]any{"method": "onVideoError", "viewId": id, "code": "decoder_error", "message": "codec"})

	if got := c.State().Value(); got != PlaybackStatePlaying {
		t.Errorf("state = %v", got)
	}
	if got, want := c.Progress().Value(), (Progress{Position: 1200 * time.Millisecond, Duration: time.Minute, Buffered: 3 * time.Second}); got != want {
		t.Errorf("progress = %+v, want %+v", got, want)
	}
	if !errors.Is(failure, ErrDecoder) {
		t.Errorf("error = %v, want ErrDecoder", failure)
	}
}

// failingCreate fails every platform view creation, as native does when
// the factory throws.
type failingCreate struct{ fakeNative }

func (f *failingCreate) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	if method == "create" {
		reply(nil, platform.NewChannelError("native_error", "no decoder"))
		return
	}
	f.fakeNative.InvokeMethod(channel, method, args, reply)
}

func TestNewVideoPlayerControllerFails(t *testing.T) {
	useNative(t)
	platform.SetNativeBridge(&failingCreate{})
	c, err := NewVideoPlayerController()
	if err == nil || c != nil || !strings.Contains(err.Error(), "no decoder") {
		t.Errorf("NewVideoPlayerController = %v, %v; want the native error", c, err)
	}
	if n := platform.GetPlatformViewRegistry().ViewCount(); n != 0 {
		t.Errorf("%d views left registered", n)
	}
}

func TestVideoRejectsMalformedEvents(t *testing.T) {
	useNative(t)
	c := newVideo(t)
	v := c.view
	for _, tc := range []struct {
		method string
		args   map[string]any
	}{
		{"onPlaybackStateChanged", map[string]any{"state": "playing"}},
		{"onPositionChanged", map[string]any{"positionMs": 1.5, "durationMs": 0.0, "bufferedMs": 0.0}},
		{"onVideoError", map[string]any{"code": 3.0}},
	} {
		if err := v.HandleEvent(tc.method, tc.args); err == nil {
			t.Errorf("HandleEvent(%s, %v) = nil, want an error", tc.method, tc.args)
		}
	}
	if err := v.HandleEvent("onSomethingElse", nil); !errors.Is(err, platform.ErrMethodNotFound) {
		t.Errorf("unknown event = %v, want ErrMethodNotFound", err)
	}
}

func TestVideoDispose(t *testing.T) {
	native := useNative(t)
	c := newVideo(t)
	id := c.ViewID()
	c.Dispose()
	c.Dispose()
	if c.ViewID() != 0 {
		t.Errorf("ViewID after Dispose = %d", c.ViewID())
	}
	if err := c.Play(); !errors.Is(err, platform.ErrDisposed) {
		t.Errorf("Play after Dispose = %v, want ErrDisposed", err)
	}
	disposes := 0
	for _, call := range native.calls {
		if call.method == "dispose" && call.args["viewId"] == float64(id) {
			disposes++
		}
	}
	if disposes != 1 {
		t.Errorf("dispose sent %d times, want 1", disposes)
	}
}
