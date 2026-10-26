package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeNative stands in for the native halves: it answers each method with
// a canned reply or error, and records the args of each method's last call.
type fakeNative struct {
	replies map[string]any
	errs    map[string]error
	calls   []string
	args    map[string]any
}

func (f *fakeNative) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	f.calls = append(f.calls, channel+"."+method)
	decoded, err := platform.DefaultCodec.Decode(args)
	if err != nil {
		reply(nil, err)
		return
	}
	if f.args == nil {
		f.args = map[string]any{}
	}
	f.args[method] = decoded
	if err := f.errs[method]; err != nil {
		reply(nil, err)
		return
	}
	reply(platform.DefaultCodec.Encode(f.replies[method]))
}

// count returns how many times channel.method was called.
func (f *fakeNative) count(call string) int {
	n := 0
	for _, c := range f.calls {
		if c == call {
			n++
		}
	}
	return n
}

func useNative(t *testing.T, f *fakeNative) {
	t.Helper()
	platform.SetNativeBridge(f)
	platform.RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(platform.ResetForTest)
}

var fix = map[string]any{
	"latitude": 51.5, "longitude": -0.12, "altitude": 11.0, "accuracy": 5.0,
	"heading": 90.0, "speed": 1.5, "timestamp": 1700000000000.0, "isMocked": true,
}

func TestCurrent(t *testing.T) {
	native := &fakeNative{replies: map[string]any{"getCurrentLocation": fix}}
	useNative(t, native)
	got, err := Current(context.Background(), Options{HighAccuracy: true})
	if err != nil {
		t.Fatal(err)
	}
	want := Update{
		Latitude: 51.5, Longitude: -0.12, Altitude: 11, Accuracy: 5, Heading: 90, Speed: 1.5,
		Timestamp: time.UnixMilli(1700000000000), IsMocked: true,
	}
	if got != want {
		t.Errorf("Current = %+v, want %+v", got, want)
	}
	if len(native.calls) != 1 || native.calls[0] != "drift/location.getCurrentLocation" {
		t.Errorf("calls = %v", native.calls)
	}
}

func TestCurrentRejectsMalformedFix(t *testing.T) {
	useNative(t, &fakeNative{replies: map[string]any{"getCurrentLocation": map[string]any{"latitude": 1.0}}})
	if _, err := Current(context.Background(), Options{}); err == nil {
		t.Error("fix without longitude accepted")
	}
}

func TestNativeErrorCodesMapToSentinels(t *testing.T) {
	for code, want := range errorCodes {
		useNative(t, &fakeNative{errs: map[string]error{"getCurrentLocation": platform.NewChannelError(code, "native says no")}})
		if _, err := Current(context.Background(), Options{}); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
	}
}

func TestLastKnownNone(t *testing.T) {
	useNative(t, &fakeNative{replies: map[string]any{"getLastKnown": nil}})
	got, err := LastKnown(context.Background())
	if got != nil || err != nil {
		t.Errorf("LastKnown = %v, %v; want nil, nil", got, err)
	}
}

func TestIsEnabled(t *testing.T) {
	useNative(t, &fakeNative{replies: map[string]any{"isEnabled": map[string]any{"enabled": true}}})
	if enabled, err := IsEnabled(context.Background()); !enabled || err != nil {
		t.Errorf("IsEnabled = %v, %v", enabled, err)
	}
	useNative(t, &fakeNative{replies: map[string]any{"isEnabled": map[string]any{}}})
	if _, err := IsEnabled(context.Background()); err == nil {
		t.Error("malformed reply accepted")
	}
}

// Without the always: config the native side does not register the kind;
// the error says which config is missing.
func TestAlwaysNamesMissingConfig(t *testing.T) {
	useNative(t, &fakeNative{errs: map[string]error{
		"check": platform.NewChannelError("permission_not_registered", "no plugin registered location_always"),
	}})
	_, err := Permission.Always.Status()
	if !errors.Is(err, platform.ErrPermissionNotRegistered) {
		t.Fatalf("err = %v, want ErrPermissionNotRegistered", err)
	}
	if want := "always: config"; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to mention %q", err, want)
	}
}

// The first listener starts the platform's updates and the last stops
// them; updates and stream errors reach the listener.
func TestUpdatesStream(t *testing.T) {
	native := &fakeNative{}
	useNative(t, native)
	var updates []Update
	var failures []error
	unsubscribe := Updates().Listen(platform.StreamHandler[Update]{
		OnEvent: func(u Update) { updates = append(updates, u) },
		OnError: func(err error) { failures = append(failures, err) },
	})
	second := Updates().Listen(platform.StreamHandler[Update]{})
	if got := native.count("drift/streams.listen"); got != 1 {
		t.Errorf("listen sent %d times, want once for two listeners", got)
	}
	if args, _ := native.args["listen"].(map[string]any); args["channel"] != channelName {
		t.Errorf("listen args = %v, want channel %s", args, channelName)
	}

	data, err := platform.DefaultCodec.Encode(fix)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEvent(channelName, data); err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEventError(channelName, platform.NewChannelError("permission_denied", "location permission revoked")); err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].Latitude != 51.5 {
		t.Errorf("updates = %+v", updates)
	}
	if len(failures) != 1 || !errors.Is(failures[0], ErrPermissionDenied) {
		t.Errorf("failures = %v, want ErrPermissionDenied", failures)
	}

	second()
	unsubscribe()
	if got := native.count("drift/streams.cancel"); got != 1 {
		t.Errorf("cancel sent %d times, want once, after the last listener", got)
	}
}

// The stream's options reach the native side, Background always stated:
// changing a background stream's options without it ends its background
// use.
func TestSetUpdateOptionsSendsOptions(t *testing.T) {
	for _, background := range []bool{false, true} {
		native := &fakeNative{}
		useNative(t, native)
		opts := UpdateOptions{Options: Options{HighAccuracy: true, DistanceFilter: 10}, Background: background}
		if err := SetUpdateOptions(opts); err != nil {
			t.Fatal(err)
		}
		args, _ := native.args["setUpdateOptions"].(map[string]any)
		if args["background"] != background || args["highAccuracy"] != true || args["distanceFilter"] != 10.0 {
			t.Errorf("background=%v: setUpdateOptions args = %v", background, args)
		}
	}
}

// Background options fail, rather than silently running only in the
// foreground, without the always: config, without Always, or (Android)
// when the running stream cannot switch from the background.
func TestSetUpdateOptionsBackgroundErrors(t *testing.T) {
	for code, want := range map[string]error{
		"background_not_configured": ErrBackgroundNotConfigured,
		"always_denied":             ErrAlwaysDenied,
		"background_start":          ErrBackgroundStart,
	} {
		useNative(t, &fakeNative{errs: map[string]error{"setUpdateOptions": platform.NewChannelError(code, "native says no")}})
		if err := SetUpdateOptions(UpdateOptions{Background: true}); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
	}
	if want := "always: config"; !strings.Contains(ErrBackgroundNotConfigured.Error(), want) {
		t.Errorf("ErrBackgroundNotConfigured = %q, want it to mention %q", ErrBackgroundNotConfigured, want)
	}
}
