package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

// replyBridge answers every call with reply (or err) and records the last
// call's arguments.
type replyBridge struct {
	reply  any
	err    error
	method string
	args   map[string]any
}

func (b *replyBridge) InvokeMethod(_, method string, args []byte, reply func([]byte, error)) {
	b.method = method
	b.args = nil
	json.Unmarshal(args, &b.args)
	if b.err != nil {
		reply(nil, b.err)
		return
	}
	reply(DefaultCodec.Encode(b.reply))
}

func useReplyBridge(t *testing.T, b *replyBridge) {
	t.Helper()
	SetupTestBridge(t.Cleanup)
	SetNativeBridge(b)
}

func TestPickFile(t *testing.T) {
	b := &replyBridge{reply: map[string]any{"files": []any{
		map[string]any{"name": "a.txt", "path": "content://a", "uri": "content://a", "mimeType": "text/plain", "size": 61},
	}}}
	useReplyBridge(t, b)
	files, err := Storage.PickFile(context.Background(), PickFileOptions{AllowMultiple: true, AllowedTypes: []string{"text/plain"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []PickedFile{{Name: "a.txt", Path: "content://a", URI: "content://a", MimeType: "text/plain", Size: 61}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %+v, want %+v", files, want)
	}
	if b.method != "pickFile" || b.args["allowMultiple"] != true {
		t.Errorf("call = %s %v", b.method, b.args)
	}
}

// A canceled picker replies nil: no files, no path, no error.
func TestPickersCanceled(t *testing.T) {
	useReplyBridge(t, &replyBridge{})
	ctx := context.Background()
	if files, err := Storage.PickFile(ctx, PickFileOptions{}); files != nil || err != nil {
		t.Errorf("PickFile = %v, %v", files, err)
	}
	if path, err := Storage.PickDirectory(ctx); path != "" || err != nil {
		t.Errorf("PickDirectory = %q, %v", path, err)
	}
	if path, err := Storage.SaveFile(ctx, []byte("x"), SaveFileOptions{}); path != "" || err != nil {
		t.Errorf("SaveFile = %q, %v", path, err)
	}
}

func TestStorageBusy(t *testing.T) {
	useReplyBridge(t, &replyBridge{err: NewChannelError("storage_busy", "a storage picker is already open")})
	if _, err := Storage.PickDirectory(context.Background()); !errors.Is(err, ErrStorageBusy) {
		t.Errorf("err = %v, want ErrStorageBusy", err)
	}
}

// File contents cross the channel as base64 in both directions.
func TestFileContentsAreBase64(t *testing.T) {
	data := []byte{0, 1, 2, 255}
	b := &replyBridge{reply: map[string]any{"data": base64.StdEncoding.EncodeToString(data)}}
	useReplyBridge(t, b)
	got, err := Storage.ReadFile(context.Background(), "/f")
	if err != nil || !reflect.DeepEqual(got, data) {
		t.Errorf("ReadFile = %v, %v; want %v", got, err, data)
	}

	b.reply = nil
	if err := Storage.WriteFile(context.Background(), "/f", data); err != nil {
		t.Fatal(err)
	}
	if b.args["data"] != base64.StdEncoding.EncodeToString(data) {
		t.Errorf("writeFile data = %v", b.args["data"])
	}
	if _, err := Storage.SaveFile(context.Background(), data, SaveFileOptions{}); err != nil {
		t.Fatal(err)
	}
	if b.args["data"] != base64.StdEncoding.EncodeToString(data) {
		t.Errorf("saveFile data = %v", b.args["data"])
	}
}

func TestReadFileRejectsNonBase64Reply(t *testing.T) {
	useReplyBridge(t, &replyBridge{reply: map[string]any{"data": []any{1, 2}}})
	if _, err := Storage.ReadFile(context.Background(), "/f"); err == nil {
		t.Error("ReadFile accepted a number array")
	}
}

// Dates cross as calendar days, so a time zone cannot move them.
func TestDatePickerCalendarDays(t *testing.T) {
	b := &replyBridge{reply: map[string]any{"year": 2026, "month": 10, "day": 21}}
	useReplyBridge(t, b)
	tokyo := time.FixedZone("UTC+9", 9*60*60)
	min := time.Date(2026, 1, 1, 0, 30, 0, 0, tokyo) // Dec 31 in UTC
	got, err := ShowDatePicker(context.Background(), DatePickerConfig{
		InitialDate: time.Date(2026, 10, 5, 23, 0, 0, 0, time.UTC),
		MinDate:     &min,
	})
	if err != nil {
		t.Fatal(err)
	}
	if y, m, d := got.Date(); y != 2026 || m != time.October || d != 21 {
		t.Errorf("date = %v, want 2026-10-21", got)
	}
	day := func(v any) map[string]any { return v.(map[string]any) }
	if !reflect.DeepEqual(day(b.args["initialDate"]), map[string]any{"year": 2026.0, "month": 10.0, "day": 5.0}) {
		t.Errorf("initialDate = %v", b.args["initialDate"])
	}
	if !reflect.DeepEqual(day(b.args["minDate"]), map[string]any{"year": 2026.0, "month": 1.0, "day": 1.0}) {
		t.Errorf("minDate = %v", b.args["minDate"])
	}
	if _, present := b.args["maxDate"]; present {
		t.Error("maxDate sent without MaxDate")
	}
}

func TestPickersCanceledWithSentinel(t *testing.T) {
	useReplyBridge(t, &replyBridge{})
	if _, err := ShowDatePicker(context.Background(), DatePickerConfig{}); !errors.Is(err, ErrPickerCancelled) {
		t.Errorf("date err = %v", err)
	}
	if _, _, err := ShowTimePicker(context.Background(), TimePickerConfig{}); !errors.Is(err, ErrPickerCancelled) {
		t.Errorf("time err = %v", err)
	}
}

func TestTimePicker(t *testing.T) {
	is24 := true
	b := &replyBridge{reply: map[string]any{"hour": 15, "minute": 30}}
	useReplyBridge(t, b)
	hour, minute, err := ShowTimePicker(context.Background(), TimePickerConfig{Hour: 9, Is24Hour: &is24})
	if err != nil || hour != 15 || minute != 30 {
		t.Errorf("ShowTimePicker = %d, %d, %v", hour, minute, err)
	}
	if b.args["hour"] != 9.0 || b.args["is24Hour"] != true {
		t.Errorf("args = %v", b.args)
	}
}
