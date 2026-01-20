package errors

import (
	"testing"
	"time"
)

func TestDriftErrorString(t *testing.T) {
	err := &DriftError{
		Op:   "test.operation",
		Kind: KindPlatform,
		Err:  &ParseError{Channel: "test", DataType: "TestData", Got: "invalid"},
	}
	got := err.Error()
	if got == "" {
		t.Error("expected non-empty error string")
	}
}

func TestDriftErrorWithChannel(t *testing.T) {
	err := &DriftError{
		Op:      "test.operation",
		Kind:    KindParsing,
		Channel: "drift/test/channel",
		Err:     &ParseError{Channel: "drift/test/channel", DataType: "TestData", Got: nil},
	}
	got := err.Error()
	if got == "" {
		t.Error("expected non-empty error string")
	}
	// Should contain channel info
	want := "channel=drift/test/channel"
	if !contains(got, want) {
		t.Errorf("error string %q should contain %q", got, want)
	}
}

func TestErrorKindString(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindPlatform, "platform"},
		{KindParsing, "parsing"},
		{KindInit, "init"},
		{KindRender, "render"},
		{KindPanic, "panic"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("ErrorKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestPanicErrorString(t *testing.T) {
	err := &PanicError{
		Value:     "test panic",
		Timestamp: time.Now(),
	}
	got := err.Error()
	want := "panic: test panic"
	if got != want {
		t.Errorf("PanicError.Error() = %q, want %q", got, want)
	}
}

func TestPanicErrorStringWithOp(t *testing.T) {
	err := &PanicError{
		Op:        "engine.HandlePointer",
		Value:     "test panic",
		Timestamp: time.Now(),
	}
	got := err.Error()
	want := "panic in engine.HandlePointer: test panic"
	if got != want {
		t.Errorf("PanicError.Error() = %q, want %q", got, want)
	}
}

func TestParseErrorString(t *testing.T) {
	err := &ParseError{
		Channel:  "drift/test",
		DataType: "TestEvent",
		Got:      123,
	}
	got := err.Error()
	if got == "" {
		t.Error("expected non-empty error string")
	}
}

func TestReport(t *testing.T) {
	var capturedErr *DriftError
	handler := &testHandler{
		onError: func(err *DriftError) {
			capturedErr = err
		},
	}

	oldHandler := DefaultHandler
	SetHandler(handler)
	defer SetHandler(oldHandler)

	Report(&DriftError{
		Op:   "test.op",
		Kind: KindInit,
		Err:  &ParseError{Channel: "test", DataType: "Test", Got: nil},
	})

	if capturedErr == nil {
		t.Error("expected error to be captured")
	}
	if capturedErr.Op != "test.op" {
		t.Errorf("Op = %q, want %q", capturedErr.Op, "test.op")
	}
	if capturedErr.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestReportPanic(t *testing.T) {
	var capturedPanic *PanicError
	handler := &testHandler{
		onPanic: func(err *PanicError) {
			capturedPanic = err
		},
	}

	oldHandler := DefaultHandler
	SetHandler(handler)
	defer SetHandler(oldHandler)

	ReportPanic(&PanicError{
		Value:     "test panic value",
		Timestamp: time.Now(),
	})

	if capturedPanic == nil {
		t.Error("expected panic to be captured")
	}
	if capturedPanic.Value != "test panic value" {
		t.Errorf("Value = %v, want %q", capturedPanic.Value, "test panic value")
	}
}

func TestRecover(t *testing.T) {
	var capturedPanic *PanicError
	handler := &testHandler{
		onPanic: func(err *PanicError) {
			capturedPanic = err
		},
	}

	oldHandler := DefaultHandler
	SetHandler(handler)
	defer SetHandler(oldHandler)

	func() {
		defer Recover("test.recover")
		panic("intentional test panic")
	}()

	if capturedPanic == nil {
		t.Error("expected panic to be recovered and captured")
	}
	if capturedPanic.Value != "intentional test panic" {
		t.Errorf("Value = %v, want %q", capturedPanic.Value, "intentional test panic")
	}
	if capturedPanic.Op != "test.recover" {
		t.Errorf("Op = %q, want %q", capturedPanic.Op, "test.recover")
	}
}

func TestCaptureStack(t *testing.T) {
	stack := CaptureStack()
	if stack == "" {
		t.Error("expected non-empty stack trace")
	}
	// Stack should contain some runtime info (either test function or testing infrastructure)
	if !contains(stack, "testing") && !contains(stack, "runtime") {
		t.Errorf("stack trace should contain testing or runtime frames, got: %s", stack)
	}
}

func TestSetHandlerNil(t *testing.T) {
	SetHandler(nil)
	if DefaultHandler == nil {
		t.Error("SetHandler(nil) should set default LogHandler, not nil")
	}
	if _, ok := DefaultHandler.(*LogHandler); !ok {
		t.Errorf("SetHandler(nil) should set LogHandler, got %T", DefaultHandler)
	}
}

type testHandler struct {
	onError func(*DriftError)
	onPanic func(*PanicError)
}

func (h *testHandler) HandleError(err *DriftError) {
	if h.onError != nil {
		h.onError(err)
	}
}

func (h *testHandler) HandlePanic(err *PanicError) {
	if h.onPanic != nil {
		h.onPanic(err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
