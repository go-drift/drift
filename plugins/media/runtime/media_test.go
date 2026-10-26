package runtime

import (
	"errors"
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

// Each player sees only its own events; state callbacks fire on change only.
func TestAudioEventsReachTheirPlayer(t *testing.T) {
	useNative(t)
	a, b := NewAudioPlayerController(), NewAudioPlayerController()
	defer a.Dispose()
	defer b.Dispose()
	var states []PlaybackState
	var position, duration, buffered time.Duration
	a.OnPlaybackStateChanged = func(s PlaybackState) { states = append(states, s) }
	a.OnPositionChanged = func(p, d, buf time.Duration) { position, duration, buffered = p, d, buf }
	b.OnPlaybackStateChanged = func(PlaybackState) { t.Error("b got a's event") }
	b.OnError = func(string, string) { t.Error("b got a's error") }

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
	if position != 700*time.Millisecond || duration != 9*time.Second || buffered != 4*time.Second {
		t.Errorf("position callback = %v %v %v", position, duration, buffered)
	}
	if a.State() != PlaybackStatePaused || a.Position() != 700*time.Millisecond || a.Duration() != 9*time.Second || a.Buffered() != 4*time.Second {
		t.Errorf("a cached %v %v %v %v", a.State(), a.Position(), a.Duration(), a.Buffered())
	}
	if b.State() != PlaybackStateIdle || b.Position() != 0 {
		t.Errorf("b cached %v %v", b.State(), b.Position())
	}

	var code, message string
	a.OnError = func(c, m string) { code, message = c, m }
	send(t, audioErrorsChannel, map[string]any{"playerId": a.id, "code": ErrCodeSourceError, "message": "404"})
	if code != ErrCodeSourceError || message != "404" {
		t.Errorf("error callback = %q %q", code, message)
	}
}

// A disposed player tells native once, rejects calls and hears nothing.
func TestAudioDispose(t *testing.T) {
	native := useNative(t)
	a := NewAudioPlayerController()
	id := a.id
	a.OnPlaybackStateChanged = func(PlaybackState) { t.Error("disposed player got an event") }
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

func TestVideoCreatesItsView(t *testing.T) {
	native := useNative(t)
	c := NewVideoPlayerController()
	defer c.Dispose()
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
	c := NewVideoPlayerController()
	defer c.Dispose()
	var state PlaybackState
	var position time.Duration
	var code string
	c.OnPlaybackStateChanged = func(s PlaybackState) { state = s }
	c.OnPositionChanged = func(p, _, _ time.Duration) { position = p }
	c.OnError = func(c, _ string) { code = c }

	id := c.ViewID()
	send(t, "drift/platform_views", map[string]any{"method": "onPlaybackStateChanged", "viewId": id, "state": 2})
	send(t, "drift/platform_views", map[string]any{"method": "onPositionChanged", "viewId": id, "positionMs": 1200, "durationMs": 60000, "bufferedMs": 3000})
	send(t, "drift/platform_views", map[string]any{"method": "onVideoError", "viewId": id, "code": ErrCodeDecoderError, "message": "codec"})

	if state != PlaybackStatePlaying || c.State() != PlaybackStatePlaying {
		t.Errorf("state = %v, cached %v", state, c.State())
	}
	if position != 1200*time.Millisecond || c.Duration() != time.Minute || c.Buffered() != 3*time.Second {
		t.Errorf("position = %v, cached %v %v", position, c.Duration(), c.Buffered())
	}
	if code != ErrCodeDecoderError {
		t.Errorf("error code = %q", code)
	}
}

func TestVideoRejectsMalformedEvents(t *testing.T) {
	useNative(t)
	c := NewVideoPlayerController()
	defer c.Dispose()
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
	c := NewVideoPlayerController()
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
