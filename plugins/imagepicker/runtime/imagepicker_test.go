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
	calls []string
}

func (f *fakeNative) InvokeMethod(channel, method string, _ []byte, reply func([]byte, error)) {
	f.calls = append(f.calls, channel+"."+method)
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

var jpeg = map[string]any{"path": "/cache/a.jpg", "mimeType": "image/jpeg", "width": 640.0, "height": 480.0, "size": 1234.0}

func TestCapture(t *testing.T) {
	native := &fakeNative{reply: map[string]any{"image": jpeg}}
	useNative(t, native)
	img, err := Capture(context.Background(), CaptureOptions{})
	if err != nil || img == nil || *img != (Image{Path: "/cache/a.jpg", MimeType: "image/jpeg", Width: 640, Height: 480, Size: 1234}) {
		t.Fatalf("Capture = %+v, %v", img, err)
	}
	if native.calls[0] != "drift/imagepicker.capture" {
		t.Errorf("calls = %v", native.calls)
	}
}

func TestCaptureCanceled(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"image": nil}})
	if img, err := Capture(context.Background(), CaptureOptions{}); img != nil || err != nil {
		t.Errorf("Capture = %+v, %v; want nil, nil", img, err)
	}
}

func TestPick(t *testing.T) {
	useNative(t, &fakeNative{reply: map[string]any{"images": []any{jpeg, jpeg}}})
	images, err := Pick(context.Background(), PickOptions{Multiple: true})
	if err != nil || len(images) != 2 {
		t.Fatalf("Pick = %+v, %v", images, err)
	}
	useNative(t, &fakeNative{reply: map[string]any{"images": []any{map[string]any{"path": "x"}}}})
	if _, err := Pick(context.Background(), PickOptions{}); err == nil {
		t.Error("image without type and size accepted")
	}
}

func TestNativeErrorCodesMapToSentinels(t *testing.T) {
	for code, want := range errorCodes {
		useNative(t, &fakeNative{err: platform.NewChannelError(code, "native says no")})
		if _, err := Capture(context.Background(), CaptureOptions{}); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", code, err, want)
		}
	}
}
