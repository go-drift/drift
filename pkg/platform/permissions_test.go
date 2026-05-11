package platform

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	drifterrors "github.com/go-drift/drift/pkg/errors"
)

// permissionsBridge is a fake NativeBridge used by permissions tests.
// It records every InvokeMethod call and dispatches it to a per-method handler.
// Tests can read recorded calls and wait for the request flow to reach native.
type permissionsBridge struct {
	mu       sync.Mutex
	calls    []recordedCall
	handlers map[string]func(args map[string]any) (any, error)
}

type recordedCall struct {
	Channel string
	Method  string
	Args    map[string]any
}

func newPermissionsBridge() *permissionsBridge {
	return &permissionsBridge{handlers: map[string]func(args map[string]any) (any, error){}}
}

func (b *permissionsBridge) on(method string, fn func(args map[string]any) (any, error)) {
	b.mu.Lock()
	b.handlers[method] = fn
	b.mu.Unlock()
}

func (b *permissionsBridge) InvokeMethod(channel, method string, args []byte) ([]byte, error) {
	var argsMap map[string]any
	if len(args) > 0 {
		decoded, _ := DefaultCodec.Decode(args)
		if m, ok := decoded.(map[string]any); ok {
			argsMap = m
		}
	}

	b.mu.Lock()
	b.calls = append(b.calls, recordedCall{Channel: channel, Method: method, Args: argsMap})
	handler := b.handlers[method]
	b.mu.Unlock()

	if handler != nil {
		result, err := handler(argsMap)
		if err != nil {
			return nil, err
		}
		return DefaultCodec.Encode(result)
	}
	return DefaultCodec.Encode(nil)
}

func (b *permissionsBridge) StartEventStream(string) error { return nil }
func (b *permissionsBridge) StopEventStream(string) error  { return nil }

func (b *permissionsBridge) snapshot() []recordedCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]recordedCall, len(b.calls))
	copy(out, b.calls)
	return out
}

// waitForCall blocks until the bridge has recorded a call to method, or fails.
func (b *permissionsBridge) waitForCall(t *testing.T, method string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		for _, c := range b.calls {
			if c.Method == method {
				b.mu.Unlock()
				return
			}
		}
		b.mu.Unlock()
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q call; recorded=%v", method, b.snapshot())
}

func setupPermissions(t *testing.T, bridge *permissionsBridge) {
	t.Helper()
	SetNativeBridge(bridge)
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)
}

func dispatchChange(t *testing.T, name string, status PermissionResult) {
	t.Helper()
	payload, err := DefaultCodec.Encode(map[string]any{
		"permission": name,
		"status":     string(status),
	})
	if err != nil {
		t.Fatalf("encode change event: %v", err)
	}
	if err := HandleEvent("drift/permissions/changes", payload); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}

func TestRequest_FastPathTerminalStatus(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "granted"}, nil
	})
	setupPermissions(t, bridge)

	got, err := newPerm("camera").Request(context.Background())
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if got != PermissionGranted {
		t.Errorf("status = %q, want %q", got, PermissionGranted)
	}
	for _, c := range bridge.snapshot() {
		if c.Method == "request" {
			t.Errorf("expected no native request call when status is terminal; got %+v", c)
		}
	}
}

func TestRequest_AsyncResolutionViaEvent(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	p := newPerm("camera")
	resultCh := make(chan PermissionResult, 1)
	errCh := make(chan error, 1)
	go func() {
		got, err := p.Request(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- got
	}()

	bridge.waitForCall(t, "request")
	dispatchChange(t, "camera", PermissionGranted)

	select {
	case got := <-resultCh:
		if got != PermissionGranted {
			t.Errorf("status = %q, want %q", got, PermissionGranted)
		}
	case err := <-errCh:
		t.Fatalf("Request returned error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Request did not resolve after matching change event")
	}
}

func TestRequest_IgnoresOtherPermissions(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	p := newPerm("camera")
	resultCh := make(chan PermissionResult, 1)
	go func() {
		got, _ := p.Request(context.Background())
		resultCh <- got
	}()

	bridge.waitForCall(t, "request")

	dispatchChange(t, "microphone", PermissionGranted)
	select {
	case got := <-resultCh:
		t.Fatalf("Request resolved on unrelated permission change: %q", got)
	case <-time.After(50 * time.Millisecond):
	}

	dispatchChange(t, "camera", PermissionDenied)
	select {
	case got := <-resultCh:
		if got != PermissionDenied {
			t.Errorf("status = %q, want %q", got, PermissionDenied)
		}
	case <-time.After(time.Second):
		t.Fatal("Request did not resolve after matching change event")
	}
}

func TestRequest_CtxCanceled(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan PermissionResult, 1)
	errCh := make(chan error, 1)
	go func() {
		got, err := newPerm("camera").Request(ctx)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- got
	}()

	bridge.waitForCall(t, "request")
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrCanceled) {
			t.Errorf("err = %v, want ErrCanceled", err)
		}
	case got := <-resultCh:
		t.Fatalf("Request resolved instead of canceling: %q", got)
	case <-time.After(time.Second):
		t.Fatal("Request did not unblock after ctx cancel")
	}
}

func TestListen_FiresOnMatchingChange(t *testing.T) {
	setupPermissions(t, newPermissionsBridge())

	got := make(chan PermissionResult, 1)
	unsubscribe := newPerm("camera").Listen(func(s PermissionResult) { got <- s })
	defer unsubscribe()

	dispatchChange(t, "camera", PermissionGranted)
	select {
	case s := <-got:
		if s != PermissionGranted {
			t.Errorf("status = %q, want %q", s, PermissionGranted)
		}
	case <-time.After(time.Second):
		t.Fatal("listener never fired")
	}
}

func TestListen_IgnoresOther(t *testing.T) {
	setupPermissions(t, newPermissionsBridge())

	got := make(chan PermissionResult, 1)
	unsubscribe := newPerm("camera").Listen(func(s PermissionResult) { got <- s })
	defer unsubscribe()

	dispatchChange(t, "microphone", PermissionGranted)
	select {
	case s := <-got:
		t.Fatalf("listener fired on unrelated permission: %q", s)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestListen_Unsubscribe(t *testing.T) {
	setupPermissions(t, newPermissionsBridge())

	got := make(chan PermissionResult, 1)
	unsubscribe := newPerm("camera").Listen(func(s PermissionResult) { got <- s })
	unsubscribe()

	dispatchChange(t, "camera", PermissionGranted)
	select {
	case s := <-got:
		t.Fatalf("listener fired after unsubscribe: %q", s)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestParsePermissionChange_BadPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload any
	}{
		{"non-map", "not a map"},
		{"missing permission key", map[string]any{"status": "granted"}},
		{"empty permission key", map[string]any{"permission": "", "status": "granted"}},
		{"missing status key", map[string]any{"permission": "camera"}},
		{"empty status key", map[string]any{"permission": "camera", "status": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parsePermissionChange(tc.payload)
			if err == nil {
				t.Fatal("expected error for malformed payload")
			}
		})
	}
}

func TestParsePermissionChange_ReportedViaStream(t *testing.T) {
	setupPermissions(t, newPermissionsBridge())

	handler := &capturingHandler{}
	prev := drifterrors.DefaultHandler
	drifterrors.SetHandler(handler)
	t.Cleanup(func() { drifterrors.SetHandler(prev) })

	// One subscriber so the Stream forwards parse failures through its parser.
	unsubscribe := newPerm("camera").Listen(func(PermissionResult) {})
	defer unsubscribe()

	payload, err := DefaultCodec.Encode("not a map")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := HandleEvent("drift/permissions/changes", payload); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	if got := handler.snapshot(); len(got) == 0 {
		t.Fatal("expected at least one DriftError reported, got none")
	}
}

func TestLocationAlways_ShouldShowRationale(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("shouldShowRationale", func(args map[string]any) (any, error) {
		return map[string]any{"shouldShow": true}, nil
	})
	setupPermissions(t, bridge)

	if _, err := newPerm("location_always").ShouldShowRationale(context.Background()); err != nil {
		t.Fatalf("ShouldShowRationale: %v", err)
	}

	for _, c := range bridge.snapshot() {
		if c.Method != "shouldShowRationale" {
			continue
		}
		if got := c.Args["permission"]; got != "location_always" {
			t.Errorf("permission arg = %v, want \"location_always\"", got)
		}
		return
	}
	t.Fatal("no shouldShowRationale call recorded")
}

func TestNotifications_RequestWithOptions(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	n := newNotificationPerm()
	resultCh := make(chan PermissionResult, 1)
	go func() {
		got, _ := n.RequestWithOptions(context.Background(), NotificationPermissionOptions{
			Alert:       true,
			Sound:       false,
			Badge:       true,
			Provisional: true,
		})
		resultCh <- got
	}()

	bridge.waitForCall(t, "request")
	dispatchChange(t, "notifications", PermissionGranted)
	<-resultCh

	for _, c := range bridge.snapshot() {
		if c.Method != "request" {
			continue
		}
		if got := c.Args["permission"]; got != "notifications" {
			t.Errorf("permission = %v, want notifications", got)
		}
		if got := c.Args["alert"]; got != true {
			t.Errorf("alert = %v, want true", got)
		}
		if got := c.Args["sound"]; got != false {
			t.Errorf("sound = %v, want false", got)
		}
		if got := c.Args["badge"]; got != true {
			t.Errorf("badge = %v, want true", got)
		}
		if got := c.Args["provisional"]; got != true {
			t.Errorf("provisional = %v, want true", got)
		}
		return
	}
	t.Fatal("no request call recorded")
}

// Status and ShouldShowRationale currently ignore ctx cancellation.
// These tests lock that documented behavior in so future changes can't
// silently introduce half-cancellation semantics.
func TestStatus_IgnoresCanceledCtx(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "granted"}, nil
	})
	setupPermissions(t, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := newPerm("camera").Status(ctx)
	if err != nil {
		t.Fatalf("Status returned error on canceled ctx: %v", err)
	}
	if got != PermissionGranted {
		t.Errorf("status = %q, want %q", got, PermissionGranted)
	}
}

func TestShouldShowRationale_IgnoresCanceledCtx(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("shouldShowRationale", func(map[string]any) (any, error) {
		return map[string]any{"shouldShow": true}, nil
	})
	setupPermissions(t, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := newPerm("camera").ShouldShowRationale(ctx)
	if err != nil {
		t.Fatalf("ShouldShowRationale returned error on canceled ctx: %v", err)
	}
	if !got {
		t.Errorf("shouldShow = false, want true")
	}
}

func TestShouldShowRationale_PropagatesBridgeError(t *testing.T) {
	bridge := newPermissionsBridge()
	wantErr := errors.New("bridge boom")
	bridge.on("shouldShowRationale", func(map[string]any) (any, error) {
		return nil, wantErr
	})
	setupPermissions(t, bridge)

	got, err := newPerm("camera").ShouldShowRationale(context.Background())
	if err == nil {
		t.Fatal("expected error from bridge, got nil")
	}
	if got {
		t.Errorf("shouldShow = true, want false on error")
	}
}

// capturingHandler records every reported DriftError for assertion.
type capturingHandler struct {
	mu     sync.Mutex
	errors []*drifterrors.DriftError
}

func (h *capturingHandler) HandleError(err *drifterrors.DriftError) {
	h.mu.Lock()
	h.errors = append(h.errors, err)
	h.mu.Unlock()
}
func (h *capturingHandler) HandlePanic(*drifterrors.PanicError)            {}
func (h *capturingHandler) HandleBoundaryError(*drifterrors.BoundaryError) {}
func (h *capturingHandler) snapshot() []*drifterrors.DriftError {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*drifterrors.DriftError, len(h.errors))
	copy(out, h.errors)
	return out
}

// Compile-time guard: notificationPermission must satisfy NotificationPermission.
var _ NotificationPermission = (*notificationPermission)(nil)

// Compile-time guard: permission must satisfy Permission.
var _ Permission = (*permission)(nil)
