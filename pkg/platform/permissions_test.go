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

// errNoReply, returned by a handler, makes the call wait forever for its
// reply, like a prompt the user has not answered.
var errNoReply = errors.New("test: native does not reply")

type recordedCall struct {
	Channel string
	Method  string
	Args    map[string]any
}

func newPermissionsBridge() *permissionsBridge {
	return &permissionsBridge{
		handlers: map[string]func(args map[string]any) (any, error){},
	}
}

// hold makes calls to method wait forever for their reply.
func (b *permissionsBridge) hold(method string) {
	b.on(method, func(map[string]any) (any, error) { return nil, errNoReply })
}

func (b *permissionsBridge) on(method string, fn func(args map[string]any) (any, error)) {
	b.mu.Lock()
	b.handlers[method] = fn
	b.mu.Unlock()
}

func (b *permissionsBridge) InvokeMethod(channel, method string, args []byte, reply func([]byte, error)) {
	data, err := b.answer(channel, method, args)
	if !errors.Is(err, errNoReply) {
		reply(data, err)
	}
}

func (b *permissionsBridge) answer(channel, method string, args []byte) ([]byte, error) {
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

func TestRequest_RepliesWithStatus(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("request", func(map[string]any) (any, error) {
		return map[string]any{"status": "limited"}, nil
	})
	setupPermissions(t, bridge)

	got, err := NewPermission("location").Request(context.Background())
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if got != PermissionLimited {
		t.Errorf("status = %q, want %q", got, PermissionLimited)
	}
	calls := bridge.snapshot()
	if len(calls) != 1 || calls[0].Method != "request" || calls[0].Args["permission"] != "location" {
		t.Errorf("calls = %+v, want one request for location", calls)
	}
}

func TestRequest_MalformedReply(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("request", func(map[string]any) (any, error) {
		return map[string]any{"granted": true}, nil
	})
	setupPermissions(t, bridge)

	_, err := NewPermission("camera").Request(context.Background())
	var parseErr *drifterrors.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("err = %v, want a ParseError", err)
	}
}

func TestStatus_NativeErrorCodes(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{"permission_not_registered", ErrPermissionNotRegistered},
		{"permission_not_declared", ErrPermissionNotDeclared},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			bridge := newPermissionsBridge()
			bridge.on("check", func(map[string]any) (any, error) {
				return nil, NewChannelError(tc.code, "native says no")
			})
			setupPermissions(t, bridge)

			_, err := NewPermission("contacts").Status()
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewPermission_EmptyKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewPermission(\"\") did not panic")
		}
	}()
	NewPermission("")
}

func TestRequest_CtxCanceled(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.hold("request")
	setupPermissions(t, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := NewPermission("camera").Request(ctx)
		errCh <- err
	}()

	bridge.waitForCall(t, "request")
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrCanceled) {
			t.Errorf("err = %v, want ErrCanceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Request did not unblock after ctx cancel")
	}
}

// startHeldRequest starts a camera Request whose prompt stays up, and
// returns once it has reached native.
func startHeldRequest(t *testing.T, bridge *permissionsBridge) {
	t.Helper()
	go func() { _, _ = NewPermission("camera").Request(context.Background()) }()
	bridge.waitForCall(t, "request")
}

// requestWithin runs Request and fails the test unless it returns within a
// second.
func requestWithin(t *testing.T, ctx context.Context, p Permission) error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		_, err := p.Request(ctx)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		return err
	case <-time.After(time.Second):
		t.Fatal("Request blocked behind the open prompt")
		return nil
	}
}

// A Request made while the kind's prompt is up goes straight to native,
// which joins it to that prompt, so its ctx still bounds the wait.
func TestRequest_WhilePromptUp_HonoursCtx(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.hold("request")
	setupPermissions(t, bridge)
	startHeldRequest(t, bridge)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := requestWithin(t, ctx, NewPermission("camera")); !errors.Is(err, ErrTimeout) {
		t.Errorf("err = %v, want ErrTimeout", err)
	}

	requests := 0
	for _, c := range bridge.snapshot() {
		if c.Method == "request" {
			requests++
		}
	}
	if requests != 2 {
		t.Errorf("native saw %d requests, want 2 (Go must not hold the second back)", requests)
	}
}

// A UI-thread Request made while the kind's prompt is up gets native's
// in-place refusal instead of waiting for the prompt, which would deadlock
// (the prompt's answer arrives on the UI thread).
func TestRequest_WhilePromptUp_UIThreadRefused(t *testing.T) {
	bridge := newPermissionsBridge()
	var mu sync.Mutex
	first := true
	bridge.on("request", func(map[string]any) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		if first {
			first = false
			return nil, errNoReply
		}
		// Native refuses an async method called on the UI thread.
		return nil, NewChannelError("blocks_ui_thread", "called on the UI thread")
	})
	setupPermissions(t, bridge)
	startHeldRequest(t, bridge)

	if err := requestWithin(t, context.Background(), NewPermission("camera")); !errors.Is(err, ErrBlocksUIThread) {
		t.Errorf("err = %v, want ErrBlocksUIThread", err)
	}
}

func TestListen_FiresOnMatchingChange(t *testing.T) {
	setupPermissions(t, newPermissionsBridge())

	got := make(chan PermissionResult, 1)
	unsubscribe := NewPermission("camera").Listen(func(s PermissionResult) { got <- s })
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
	unsubscribe := NewPermission("camera").Listen(func(s PermissionResult) { got <- s })
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
	unsubscribe := NewPermission("camera").Listen(func(s PermissionResult) { got <- s })
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
	unsubscribe := NewPermission("camera").Listen(func(PermissionResult) {})
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

	if _, err := NewPermission("location_always").ShouldShowRationale(); err != nil {
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
	bridge.on("request", func(map[string]any) (any, error) {
		return map[string]any{"status": "provisional"}, nil
	})
	setupPermissions(t, bridge)

	got, err := newNotificationPerm().RequestWithOptions(context.Background(), NotificationPermissionOptions{
		Alert:       true,
		Sound:       false,
		Badge:       true,
		Provisional: true,
	})
	if err != nil || got != PermissionProvisional {
		t.Fatalf("RequestWithOptions = %q, %v; want provisional", got, err)
	}

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

// TestRequest_PreCanceledCtx: with a pre-canceled ctx no native "request"
// call is made, and cancellation is normalized to ErrCanceled (not raw
// context.Canceled).
func TestRequest_PreCanceledCtx(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewPermission("camera").Request(ctx)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
	for _, c := range bridge.snapshot() {
		if c.Method == "request" {
			t.Errorf("expected no native request call when ctx pre-canceled; got %+v", c)
		}
	}
}

// TestRequest_PreCanceledCtx_DeadlineExceeded asserts that a ctx whose deadline
// already passed maps to ErrTimeout (not raw context.DeadlineExceeded).
func TestRequest_PreCanceledCtx_DeadlineExceeded(t *testing.T) {
	bridge := newPermissionsBridge()
	bridge.on("check", func(map[string]any) (any, error) {
		return map[string]any{"status": "not_determined"}, nil
	})
	bridge.on("request", func(map[string]any) (any, error) { return nil, nil })
	setupPermissions(t, bridge)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := NewPermission("camera").Request(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestShouldShowRationale_PropagatesBridgeError(t *testing.T) {
	bridge := newPermissionsBridge()
	wantErr := errors.New("bridge boom")
	bridge.on("shouldShowRationale", func(map[string]any) (any, error) {
		return nil, wantErr
	})
	setupPermissions(t, bridge)

	got, err := NewPermission("camera").ShouldShowRationale()
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
