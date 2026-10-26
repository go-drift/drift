package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

type fakeNative struct {
	reply any
	err   error
	args  map[string]any
}

func (f *fakeNative) InvokeMethod(_, _ string, args []byte, reply func([]byte, error)) {
	decoded, _ := platform.DefaultCodec.Decode(args)
	f.args, _ = decoded.(map[string]any)
	if f.err != nil {
		reply(nil, f.err)
		return
	}
	reply(platform.DefaultCodec.Encode(f.reply))
}

func useNative(t *testing.T, f *fakeNative) {
	t.Helper()
	platform.SetNativeBridge(f)
	t.Cleanup(platform.ResetForTest)
}

func TestSetSendsOptions(t *testing.T) {
	native := &fakeNative{}
	useNative(t, native)
	err := Set(context.Background(), "k", "v", &Options{Service: "s", RequireBiometric: true, Accessibility: AfterFirstUnlock})
	if err != nil {
		t.Fatal(err)
	}
	a := native.args
	if a["key"] != "k" || a["value"] != "v" || a["service"] != "s" || a["requireBiometric"] != true || a["accessibility"] != "after_first_unlock" {
		t.Errorf("args = %v", a)
	}
}

func TestGet(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"value": "secret"}})
	if v, found, err := Get(context.Background(), "k", nil); v != "secret" || !found || err != nil {
		t.Errorf("Get = %q, %v, %v", v, found, err)
	}
	useNative(t, &fakeNative{reply: map[string]any{"value": nil}})
	if v, found, err := Get(context.Background(), "k", nil); v != "" || found || err != nil {
		t.Errorf("Get missing = %q, %v, %v", v, found, err)
	}
	useNative(t, &fakeNative{reply: map[string]any{"value": 3}})
	if _, _, err := Get(context.Background(), "k", nil); err == nil {
		t.Error("non-string value accepted")
	}
}

func TestKeys(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"keys": []any{"a", "b"}}})
	keys, err := Keys(context.Background(), nil)
	if err != nil || len(keys) != 2 || keys[0] != "a" {
		t.Errorf("Keys = %v, %v", keys, err)
	}
}

func TestAvailableBiometric(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"type": "optic_id"}})
	if b, err := AvailableBiometric(context.Background()); b != BiometricOpticID || err != nil {
		t.Errorf("AvailableBiometric = %q, %v", b, err)
	}
}

func TestNativeErrorCodesMapToSentinels(t *testing.T) {
	for code, want := range nativeErrors {
		useNative(t, &fakeNative{err: platform.NewChannelError(code, "native says no")})
		if _, _, err := Get(context.Background(), "k", nil); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
	}
}
