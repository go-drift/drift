package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeNative stands in for the native halves: it answers method calls
// with a canned reply.
type fakeNative struct {
	reply any
	err   error
	calls []string
}

func (f *fakeNative) InvokeMethod(_ context.Context, channel, method string, _ []byte) ([]byte, error) {
	f.calls = append(f.calls, channel+"."+method)
	if f.err != nil {
		return nil, f.err
	}
	return platform.DefaultCodec.Encode(f.reply)
}

func (f *fakeNative) StartEventStream(string) error { return nil }
func (f *fakeNative) StopEventStream(string) error  { return nil }

func useNative(t *testing.T, f *fakeNative) {
	t.Helper()
	platform.SetNativeBridge(f)
	t.Cleanup(platform.ResetForTest)
}

func TestGreeting(t *testing.T) {
	native := &fakeNative{reply: "hi"}
	useNative(t, native)
	got, err := Greeting(context.Background())
	if err != nil || got != "hi" {
		t.Fatalf("Greeting() = %q, %v; want hi", got, err)
	}
	if len(native.calls) != 1 || native.calls[0] != "example/hello.greeting" {
		t.Errorf("calls = %v, want [example/hello.greeting]", native.calls)
	}
}

func TestGreetingErrors(t *testing.T) {
	useNative(t, &fakeNative{err: errors.New("no handler")})
	if _, err := Greeting(context.Background()); err == nil {
		t.Error("native error not returned")
	}
	useNative(t, &fakeNative{reply: 42})
	if _, err := Greeting(context.Background()); err == nil {
		t.Error("non-string reply accepted")
	}
}
