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
// a canned reply or error.
type fakeNative struct {
	replies map[string]any
	errs    map[string]error
	calls   []string
}

func (f *fakeNative) InvokeMethod(channel, method string, _ []byte, reply func([]byte, error)) {
	f.calls = append(f.calls, channel+"."+method)
	if err := f.errs[method]; err != nil {
		reply(nil, err)
		return
	}
	reply(platform.DefaultCodec.Encode(f.replies[method]))
}

func (f *fakeNative) StartEventStream(string) error { return nil }
func (f *fakeNative) StopEventStream(string) error  { return nil }

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
	for code, want := range nativeErrors {
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

func TestUpdatesStream(t *testing.T) {
	useNative(t, &fakeNative{})
	got := make(chan Update, 1)
	unsubscribe := Updates().Listen(func(u Update) { got <- u })
	defer unsubscribe()
	data, err := platform.DefaultCodec.Encode(fix)
	if err != nil {
		t.Fatal(err)
	}
	if err := platform.HandleEvent(updatesChannel, data); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-got:
		if u.Latitude != 51.5 {
			t.Errorf("update = %+v", u)
		}
	case <-time.After(time.Second):
		t.Fatal("no update delivered")
	}
}
