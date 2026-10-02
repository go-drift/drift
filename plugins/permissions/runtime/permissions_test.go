package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeNative answers every drift/permissions call with reply or err,
// recording the kinds asked for.
type fakeNative struct {
	reply any
	err   error
	kinds []string
}

func (f *fakeNative) InvokeMethod(_, _ string, args []byte, reply func([]byte, error)) {
	decoded, _ := platform.DefaultCodec.Decode(args)
	kind, _ := decoded.(map[string]any)["permission"].(string)
	f.kinds = append(f.kinds, kind)
	if f.err != nil {
		reply(nil, f.err)
		return
	}
	reply(platform.DefaultCodec.Encode(f.reply))
}

func (f *fakeNative) StartEventStream(string) error { return nil }
func (f *fakeNative) StopEventStream(string) error  { return nil }

func useNative(t *testing.T, f *fakeNative) {
	t.Helper()
	platform.SetNativeBridge(f)
	t.Cleanup(platform.ResetForTest)
}

func TestKinds(t *testing.T) {
	native := &fakeNative{reply: map[string]any{"status": "granted"}}
	useNative(t, native)
	for _, p := range []platform.Permission{Contacts, Calendar, Microphone, Photos} {
		if status, err := p.Request(context.Background()); err != nil || status != platform.PermissionGranted {
			t.Errorf("Request = %q, %v", status, err)
		}
	}
	if got := strings.Join(native.kinds, ","); got != "contacts,calendar,microphone,photos" {
		t.Errorf("kinds = %s", got)
	}
}

// A kind left out of the config is not registered natively; the error
// names the missing section.
func TestUnconfiguredKindNamesSection(t *testing.T) {
	useNative(t, &fakeNative{err: platform.NewChannelError("permission_not_registered", "no plugin registered photos")})
	_, err := Photos.Status()
	if !errors.Is(err, platform.ErrPermissionNotRegistered) {
		t.Fatalf("err = %v, want ErrPermissionNotRegistered", err)
	}
	if !strings.Contains(err.Error(), "photos: section") {
		t.Errorf("err = %v, want it to name the photos: section", err)
	}
}
