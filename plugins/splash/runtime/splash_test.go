package runtime

import (
	"context"
	"sync"
	"testing"

	"github.com/go-drift/drift/pkg/platform"
)

// fakeBridge captures InvokeMethod calls so we can assert the wire shape
// of Preserve/Remove without a real Drift bridge.
type fakeBridge struct {
	mu       sync.Mutex
	channels []string
	methods  []string
}

func (b *fakeBridge) InvokeMethod(_ context.Context, channel, method string, _ []byte) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.channels = append(b.channels, channel)
	b.methods = append(b.methods, method)
	resp, _ := platform.DefaultCodec.Encode(nil)
	return resp, nil
}

func (b *fakeBridge) StartEventStream(string) error { return nil }
func (b *fakeBridge) StopEventStream(string) error  { return nil }

func (b *fakeBridge) snapshot() ([]string, []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.channels...),
		append([]string(nil), b.methods...)
}

func setupBridge(t *testing.T) *fakeBridge {
	t.Helper()
	platform.ResetForTest()
	platform.RegisterDispatch(func(cb func()) { cb() })
	bridge := &fakeBridge{}
	platform.SetNativeBridge(bridge)
	// Re-create the package-level channel against the reset registry.
	channel = platform.NewMethodChannel(channelName)
	return bridge
}

func TestPreserveInvokesPreserveMethod(t *testing.T) {
	bridge := setupBridge(t)
	Preserve()
	channels, methods := bridge.snapshot()
	if len(channels) != 1 || channels[0] != channelName {
		t.Fatalf("expected one call on %q, got channels=%v", channelName, channels)
	}
	if methods[0] != "preserve" {
		t.Errorf("method = %q, want preserve", methods[0])
	}
}

func TestRemoveInvokesRemoveMethod(t *testing.T) {
	bridge := setupBridge(t)
	Remove()
	channels, methods := bridge.snapshot()
	if len(channels) != 1 || channels[0] != channelName {
		t.Fatalf("expected one call on %q, got channels=%v", channelName, channels)
	}
	if methods[0] != "remove" {
		t.Errorf("method = %q, want remove", methods[0])
	}
}

func TestPreserveRemovePairOrdering(t *testing.T) {
	bridge := setupBridge(t)
	Preserve()
	Remove()
	_, methods := bridge.snapshot()
	if len(methods) != 2 {
		t.Fatalf("expected 2 invocations, got %d", len(methods))
	}
	if methods[0] != "preserve" || methods[1] != "remove" {
		t.Errorf("methods = %v, want [preserve remove]", methods)
	}
}
