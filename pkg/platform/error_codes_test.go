package platform

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// failingBridge answers every call in place with err.
type failingBridge struct{ err error }

func (b failingBridge) InvokeMethod(_, _ string, _ []byte, reply func([]byte, error)) {
	reply(nil, b.err)
}

var (
	errTestDenied = errors.New("test: denied")
	errTestOther  = errors.New("test: other")
)

func TestInvokeMatchesTheChannelsErrorCodes(t *testing.T) {
	t.Cleanup(ResetForTest)
	channel := NewMethodChannel("test/error_codes").WithErrors(ErrorCodes{"denied": errTestDenied})
	other := NewMethodChannel("test/error_codes_other").WithErrors(ErrorCodes{"denied": errTestOther})

	SetNativeBridge(failingBridge{NewChannelError("denied", "the user said no")})
	_, err := channel.Invoke(context.Background(), "m", nil)
	if !errors.Is(err, errTestDenied) {
		t.Errorf("errors.Is(%v, errTestDenied) = false", err)
	}
	if errors.Is(err, errTestOther) {
		t.Errorf("%v matches another channel's sentinel", err)
	}
	var channelErr *ChannelError
	if !errors.As(err, &channelErr) || channelErr.Code != "denied" || channelErr.Message != "the user said no" {
		t.Errorf("err = %#v, want the native ChannelError", err)
	}

	_, err = other.Invoke(context.Background(), "m", nil)
	if !errors.Is(err, errTestOther) || errors.Is(err, errTestDenied) {
		t.Errorf("other channel: err = %v, want errTestOther only", err)
	}

	SetNativeBridge(failingBridge{NewChannelError("unknown", "?")})
	if _, err := channel.Invoke(context.Background(), "m", nil); errors.Is(err, errTestDenied) {
		t.Errorf("undeclared code matched: %v", err)
	}

	SetNativeBridge(failingBridge{NewChannelError("method_not_found", "no m")})
	if _, err := channel.Invoke(context.Background(), "m", nil); !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("transport code: err = %v, want ErrMethodNotFound", err)
	}
}

func TestInvokeLeavesTheNativeErrorUnchanged(t *testing.T) {
	t.Cleanup(ResetForTest)
	channel := NewMethodChannel("test/error_codes_shared").WithErrors(ErrorCodes{"denied": errTestDenied})
	native := NewChannelError("denied", "no")
	SetNativeBridge(failingBridge{native})
	if _, err := channel.Invoke(context.Background(), "m", nil); !errors.Is(err, errTestDenied) {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(native, errTestDenied) {
		t.Error("Invoke attached the codes to the bridge's own error value")
	}
}

func TestStreamErrorsMatchTheChannelsErrorCodes(t *testing.T) {
	t.Cleanup(ResetForTest)
	events := NewEventChannel("test/error_codes_events").WithErrors(ErrorCodes{"denied": errTestDenied})
	var got error
	events.Listen(EventHandler{OnError: func(err error) { got = err }})
	if err := HandleEventError("test/error_codes_events", "denied", "revoked"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(got, errTestDenied) || !strings.Contains(got.Error(), "revoked") {
		t.Errorf("stream error = %v, want errTestDenied with the native message", got)
	}
}

func TestErrorCodesNewError(t *testing.T) {
	codes := ErrorCodes{"denied": errTestDenied}
	if err := codes.NewError("denied", "no"); !errors.Is(err, errTestDenied) || err.Message != "no" {
		t.Errorf("NewError = %v", err)
	}
}

func TestWithErrorsRejectsMisuse(t *testing.T) {
	type uncomparable struct{ error }
	type noncomparableErr struct {
		uncomparable
		_ []int
	}
	cases := map[string]func(){
		"twice": func() {
			NewMethodChannel("test/twice").WithErrors(ErrorCodes{"a": errTestDenied}).WithErrors(ErrorCodes{"b": errTestOther})
		},
		"transport code": func() {
			NewMethodChannel("test/transport").WithErrors(ErrorCodes{"blocks_ui_thread": errTestDenied})
		},
		"nil sentinel": func() {
			NewEventChannel("test/nil").WithErrors(ErrorCodes{"a": nil})
		},
		"uncomparable sentinel": func() {
			NewEventChannel("test/uncomparable").WithErrors(ErrorCodes{"a": noncomparableErr{}})
		},
	}
	for name, declare := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			declare()
		})
	}
}

// Drift's own feature codes live on their channels, not in the transport
// table.
func TestFeatureCodesMatchOnTheirChannels(t *testing.T) {
	t.Cleanup(ResetForTest)
	SetNativeBridge(failingBridge{NewChannelError("storage_busy", "a picker is open")})
	if _, err := Storage.PickFile(context.Background(), PickFileOptions{}); !errors.Is(err, ErrStorageBusy) {
		t.Errorf("PickFile: err = %v, want ErrStorageBusy", err)
	}
	SetNativeBridge(failingBridge{NewChannelError("permission_not_registered", "no kind")})
	if _, err := NewPermission("test_kind").Status(); !errors.Is(err, ErrPermissionNotRegistered) {
		t.Errorf("Status: err = %v, want ErrPermissionNotRegistered", err)
	}
	if errors.Is(NewChannelError("storage_busy", ""), ErrStorageBusy) {
		t.Error("storage_busy matches without its channel")
	}
}
