package platform

import (
	stderrors "errors"
	"strconv"
	"testing"

	"github.com/go-drift/drift/pkg/errors"
)

// captureReports collects what Drift's error handling receives during the
// test.
func captureReports(t *testing.T) *[]*errors.DriftError {
	t.Helper()
	var reported []*errors.DriftError
	errors.SetHandler(errorHandlerFunc(func(err *errors.DriftError) {
		reported = append(reported, err)
	}))
	t.Cleanup(func() { errors.SetHandler(nil) })
	return &reported
}

func parseInt(data any) (int, error) {
	s, ok := data.(string)
	if !ok {
		return 0, stderrors.New("not a string")
	}
	return strconv.Atoi(s)
}

func TestStreamDeliversEventsAndNativeErrors(t *testing.T) {
	t.Cleanup(ResetForTest)
	reported := captureReports(t)
	stream := NewStream("test/stream", NewEventChannel("test/stream").WithErrors(ErrorCodes{"denied": errTestDenied}), parseInt)
	var events []int
	var failures []error
	stop := stream.Listen(StreamHandler[int]{
		OnEvent: func(v int) { events = append(events, v) },
		OnError: func(err error) { failures = append(failures, err) },
	})

	send := func(v string) {
		data, _ := DefaultCodec.Encode(v)
		if err := HandleEvent("test/stream", data); err != nil {
			t.Fatal(err)
		}
	}
	send("1")
	if err := HandleEventError("test/stream", NewChannelError("denied", "revoked")); err != nil {
		t.Fatal(err)
	}
	send("2")
	send("not a number")

	if len(events) != 2 || events[0] != 1 || events[1] != 2 {
		t.Errorf("events = %v, want [1 2]", events)
	}
	if len(failures) != 1 || !stderrors.Is(failures[0], errTestDenied) {
		t.Errorf("failures = %v, want one matching errTestDenied", failures)
	}
	if len(*reported) != 1 || (*reported)[0].Kind != errors.KindParsing {
		t.Errorf("reported = %v, want the one malformed event", *reported)
	}

	stop()
	send("3")
	if len(events) != 2 {
		t.Errorf("an event reached a canceled subscription: %v", events)
	}
}

func TestStreamWithoutOnErrorReportsErrors(t *testing.T) {
	t.Cleanup(ResetForTest)
	reported := captureReports(t)
	stream := NewStream("test/stream_report", NewEventChannel("test/stream_report"), parseInt)
	stream.Listen(StreamHandler[int]{OnEvent: func(int) {}})
	if err := HandleEventError("test/stream_report", NewChannelError("native_error", "boom")); err != nil {
		t.Fatal(err)
	}
	if len(*reported) != 1 || (*reported)[0].Kind != errors.KindPlatform || (*reported)[0].Channel != "test/stream_report" {
		t.Errorf("reported = %v, want the stream error", *reported)
	}
}

func TestEmitter(t *testing.T) {
	emitter := NewEmitter[int]("test/emitter")
	var events []int
	var failures []error
	stop := emitter.Stream().Listen(StreamHandler[int]{
		OnEvent: func(v int) { events = append(events, v) },
		OnError: func(err error) { failures = append(failures, err) },
	})
	emitter.Emit(1)
	emitter.Error(errTestOther)
	stop()
	emitter.Emit(2)
	if len(events) != 1 || events[0] != 1 || len(failures) != 1 || failures[0] != errTestOther {
		t.Errorf("events = %v, failures = %v", events, failures)
	}
	if registry.getEventChannel("test/emitter") != nil {
		t.Error("an emitter's channel is registered for native events")
	}
}

// A nil value of an interface type is a value like any other.
func TestEmitterNilInterfaceValue(t *testing.T) {
	emitter := NewEmitter[error]("test/emitter_nil")
	var events []error
	defer emitter.Stream().Listen(StreamHandler[error]{
		OnEvent: func(v error) { events = append(events, v) },
	})()
	emitter.Emit(nil)
	if len(events) != 1 || events[0] != nil {
		t.Errorf("events = %v, want [<nil>]", events)
	}
}
