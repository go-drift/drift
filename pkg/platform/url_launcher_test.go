package platform

import (
	"context"
	"fmt"
	"testing"
)

func TestURLLauncherServiceInitialization(t *testing.T) {
	if URLLauncher == nil {
		t.Fatal("URLLauncher service is nil")
	}

	if URLLauncher.channel == nil {
		t.Error("URLLauncher.channel is nil")
	}

	if URLLauncher.channel.Name() != "drift/url_launcher" {
		t.Errorf("expected channel name %q, got %q", "drift/url_launcher", URLLauncher.channel.Name())
	}
}

// urlLauncherBridge returns a canned response or error for method calls.
type urlLauncherBridge struct {
	response any
	err      error
}

func (b *urlLauncherBridge) InvokeMethod(_ context.Context, channel, method string, args []byte) ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	return DefaultCodec.Encode(b.response)
}
func (b *urlLauncherBridge) StartEventStream(string) error { return nil }
func (b *urlLauncherBridge) StopEventStream(string) error  { return nil }

func TestCanOpenURL(t *testing.T) {
	tests := []struct {
		name      string
		response  any
		wantBool  bool
		wantError bool
	}{
		{
			name:     "returns true",
			response: map[string]any{"canOpen": true},
			wantBool: true,
		},
		{
			name:     "returns false",
			response: map[string]any{"canOpen": false},
			wantBool: false,
		},
		{
			name:      "nil response",
			response:  nil,
			wantError: true,
		},
		{
			name:      "missing canOpen key",
			response:  map[string]any{"other": "value"},
			wantError: true,
		},
		{
			name:      "wrong type for canOpen",
			response:  map[string]any{"canOpen": "yes"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := &urlLauncherBridge{response: tt.response}
			SetNativeBridge(bridge)
			RegisterDispatch(func(cb func()) { cb() })
			t.Cleanup(ResetForTest)

			result, err := URLLauncher.CanOpenURL("https://example.com")

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got result=%v", result)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.wantBool {
				t.Errorf("expected %v, got %v", tt.wantBool, result)
			}
		})
	}
}

func TestOpenURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		bridge := &urlLauncherBridge{response: nil}
		SetNativeBridge(bridge)
		RegisterDispatch(func(cb func()) { cb() })
		t.Cleanup(ResetForTest)

		err := URLLauncher.OpenURL("https://example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("bridge error", func(t *testing.T) {
		bridge := &urlLauncherBridge{err: fmt.Errorf("no handler")}
		SetNativeBridge(bridge)
		RegisterDispatch(func(cb func()) { cb() })
		t.Cleanup(ResetForTest)

		err := URLLauncher.OpenURL("https://example.com")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestURLValidation(t *testing.T) {
	SetNativeBridge(noopBridge{})
	RegisterDispatch(func(cb func()) { cb() })
	t.Cleanup(ResetForTest)

	tests := []struct {
		name string
		url  string
	}{
		{"empty string", ""},
		{"no scheme", "example.com"},
	}

	for _, tt := range tests {
		t.Run("OpenURL/"+tt.name, func(t *testing.T) {
			err := URLLauncher.OpenURL(tt.url)
			if err == nil {
				t.Error("expected error for invalid URL")
			}
		})
		t.Run("CanOpenURL/"+tt.name, func(t *testing.T) {
			_, err := URLLauncher.CanOpenURL(tt.url)
			if err == nil {
				t.Error("expected error for invalid URL")
			}
		})
	}
}
