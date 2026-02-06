package engine

import (
	"testing"
	"time"
)

func TestRuntimeSampleBuffer_EmptySnapshot(t *testing.T) {
	buf := NewRuntimeSampleBuffer(60*time.Second, 5*time.Second)
	snap := buf.Snapshot()
	if snap != nil {
		t.Fatalf("expected nil, got %d samples", len(snap))
	}
}

func TestRuntimeSampleBuffer_AddSnapshotOrdering(t *testing.T) {
	buf := NewRuntimeSampleBuffer(60*time.Second, 5*time.Second)
	for i := 0; i < 5; i++ {
		buf.Add(RuntimeSample{Timestamp: int64(i)})
	}
	snap := buf.Snapshot()
	if len(snap) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(snap))
	}
	for i, s := range snap {
		if s.Timestamp != int64(i) {
			t.Errorf("sample[%d]: expected ts=%d, got %d", i, i, s.Timestamp)
		}
	}
}

func TestRuntimeSampleBuffer_WrapAround(t *testing.T) {
	// capacity = window/interval = 4s/1s = 4
	buf := NewRuntimeSampleBuffer(4*time.Second, 1*time.Second)
	for i := 0; i < 7; i++ {
		buf.Add(RuntimeSample{Timestamp: int64(i)})
	}
	snap := buf.Snapshot()
	if len(snap) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(snap))
	}
	for i, s := range snap {
		want := int64(i + 3)
		if s.Timestamp != want {
			t.Errorf("sample[%d]: expected ts=%d, got %d", i, want, s.Timestamp)
		}
	}
}

func TestNormalizeRuntimeInterval(t *testing.T) {
	tests := []struct {
		name   string
		input  time.Duration
		expect time.Duration
	}{
		{"zero returns default", 0, runtimeSampleIntervalDefault},
		{"negative returns default", -1, runtimeSampleIntervalDefault},
		{"below min clamped to min", 500 * time.Millisecond, runtimeSampleMinInterval},
		{"valid passes through", 3 * time.Second, 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRuntimeInterval(tt.input)
			if got != tt.expect {
				t.Errorf("normalizeRuntimeInterval(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestNormalizeRuntimeWindow(t *testing.T) {
	interval := 5 * time.Second
	tests := []struct {
		name   string
		input  time.Duration
		expect time.Duration
	}{
		{"zero returns default", 0, runtimeSampleWindowDefault},
		{"negative returns default", -1, runtimeSampleWindowDefault},
		{"below interval clamped to interval", 2 * time.Second, interval},
		{"valid passes through", 30 * time.Second, 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeRuntimeWindow(tt.input, interval)
			if got != tt.expect {
				t.Errorf("normalizeRuntimeWindow(%v, %v) = %v, want %v", tt.input, interval, got, tt.expect)
			}
		})
	}
}

func TestRuntimeSampleBuffer_CapacityClamping(t *testing.T) {
	// window/interval = 600/1 = 600, but max is 120
	buf := NewRuntimeSampleBuffer(600*time.Second, 1*time.Second)
	// Fill to capacity
	for i := 0; i < 200; i++ {
		buf.Add(RuntimeSample{Timestamp: int64(i)})
	}
	snap := buf.Snapshot()
	if len(snap) != runtimeSampleMaxSamples {
		t.Fatalf("expected %d samples (max), got %d", runtimeSampleMaxSamples, len(snap))
	}
}

func TestRuntimeSampleConfig_NilConfig(t *testing.T) {
	interval, window := runtimeSampleConfig(nil)
	if interval != 0 || window != 0 {
		t.Fatalf("expected (0, 0), got (%v, %v)", interval, window)
	}
}

func TestRuntimeSampleConfig_ValidConfig(t *testing.T) {
	config := &DiagnosticsConfig{
		RuntimeSampleInterval: 2 * time.Second,
		RuntimeSampleWindow:   30 * time.Second,
	}
	interval, window := runtimeSampleConfig(config)
	if interval != 2*time.Second {
		t.Errorf("expected interval=2s, got %v", interval)
	}
	if window != 30*time.Second {
		t.Errorf("expected window=30s, got %v", window)
	}
}

func TestReadRuntimeSample(t *testing.T) {
	sample := readRuntimeSample()
	if sample.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
	now := time.Now().UnixMilli()
	if sample.Timestamp > now || sample.Timestamp < now-1000 {
		t.Errorf("timestamp %d not recent (now=%d)", sample.Timestamp, now)
	}
	// HeapAlloc should be non-zero in any running Go process
	if sample.HeapAlloc == 0 {
		t.Error("expected non-zero HeapAlloc")
	}
}
