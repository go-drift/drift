package cmd

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestParseDeviceFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantID      string
		wantPresent bool
	}{
		{"empty args", nil, "", false},
		{"no device flag", []string{"--simulator", "iPhone 15"}, "", false},
		{"device with value", []string{"--device", "abc123"}, "abc123", true},
		{"device without value", []string{"--device"}, "", true},
		{"device followed by another flag", []string{"--device", "--verbose"}, "", true},
		{"device in the middle", []string{"--verbose", "--device", "abc123", "--release"}, "abc123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, present := parseDeviceFlag(tt.args)
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if present != tt.wantPresent {
				t.Errorf("present = %v, want %v", present, tt.wantPresent)
			}
		})
	}
}

func TestIsRelevantChange(t *testing.T) {
	tests := []struct {
		name  string
		event fsnotify.Event
		want  bool
	}{
		{"go file write", fsnotify.Event{Name: "/app/main.go", Op: fsnotify.Write}, true},
		{"go file create", fsnotify.Event{Name: "/app/handler.go", Op: fsnotify.Create}, true},
		{"go file remove", fsnotify.Event{Name: "/app/old.go", Op: fsnotify.Remove}, true},
		{"go file rename", fsnotify.Event{Name: "/app/old.go", Op: fsnotify.Rename}, true},
		{"drift.yaml write", fsnotify.Event{Name: "/app/drift.yaml", Op: fsnotify.Write}, true},
		{"drift.yml write", fsnotify.Event{Name: "/app/drift.yml", Op: fsnotify.Write}, true},
		{"non-go file", fsnotify.Event{Name: "/app/README.md", Op: fsnotify.Write}, false},
		{"chmod only", fsnotify.Event{Name: "/app/main.go", Op: fsnotify.Chmod}, false},
		{"go file in subdir", fsnotify.Event{Name: "/app/pkg/util.go", Op: fsnotify.Write}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRelevantChange(tt.event)
			if got != tt.want {
				t.Errorf("isRelevantChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
