package engine

import (
	"testing"
	"time"
)

func TestFrameTraceBuffer_EmptySnapshot(t *testing.T) {
	buf := NewFrameTraceBuffer(10, 16*time.Millisecond)
	snap := buf.Snapshot()
	if len(snap.Samples) != 0 {
		t.Fatalf("expected 0 samples, got %d", len(snap.Samples))
	}
	if snap.DroppedFrames != 0 {
		t.Fatalf("expected 0 dropped, got %d", snap.DroppedFrames)
	}
	wantThreshold := durationToMillis(16 * time.Millisecond)
	if snap.ThresholdMs != wantThreshold {
		t.Fatalf("expected thresholdMs=%.2f, got %.2f", wantThreshold, snap.ThresholdMs)
	}
}

func TestFrameTraceBuffer_AddSnapshotOrdering(t *testing.T) {
	buf := NewFrameTraceBuffer(10, 100*time.Millisecond)
	for i := 0; i < 5; i++ {
		buf.Add(FrameSample{Timestamp: int64(i)}, time.Millisecond)
	}
	snap := buf.Snapshot()
	if len(snap.Samples) != 5 {
		t.Fatalf("expected 5 samples, got %d", len(snap.Samples))
	}
	for i, s := range snap.Samples {
		if s.Timestamp != int64(i) {
			t.Errorf("sample[%d]: expected ts=%d, got %d", i, i, s.Timestamp)
		}
	}
}

func TestFrameTraceBuffer_WrapAround(t *testing.T) {
	buf := NewFrameTraceBuffer(4, 100*time.Millisecond)
	// Add 7 samples to a capacity-4 buffer
	for i := 0; i < 7; i++ {
		buf.Add(FrameSample{Timestamp: int64(i)}, time.Millisecond)
	}
	snap := buf.Snapshot()
	if len(snap.Samples) != 4 {
		t.Fatalf("expected 4 samples, got %d", len(snap.Samples))
	}
	// Should contain the last 4 samples (3,4,5,6) in chronological order
	for i, s := range snap.Samples {
		want := int64(i + 3)
		if s.Timestamp != want {
			t.Errorf("sample[%d]: expected ts=%d, got %d", i, want, s.Timestamp)
		}
	}
}

func TestFrameTraceBuffer_DroppedFrameCounting(t *testing.T) {
	threshold := 10 * time.Millisecond
	buf := NewFrameTraceBuffer(10, threshold)

	// Below threshold: not counted
	buf.Add(FrameSample{}, 5*time.Millisecond)
	buf.Add(FrameSample{}, 10*time.Millisecond) // exactly at threshold: not counted
	snap := buf.Snapshot()
	if snap.DroppedFrames != 0 {
		t.Fatalf("expected 0 dropped, got %d", snap.DroppedFrames)
	}

	// Above threshold: counted
	buf.Add(FrameSample{}, 11*time.Millisecond)
	buf.Add(FrameSample{}, 20*time.Millisecond)
	snap = buf.Snapshot()
	if snap.DroppedFrames != 2 {
		t.Fatalf("expected 2 dropped, got %d", snap.DroppedFrames)
	}
}

func TestFrameTraceBuffer_DefaultCapacityAndThreshold(t *testing.T) {
	// Zero values should use defaults
	buf := NewFrameTraceBuffer(0, 0)
	if buf.Capacity() != frameTraceSamplesDefault {
		t.Errorf("expected capacity %d, got %d", frameTraceSamplesDefault, buf.Capacity())
	}
	if buf.Threshold() != defaultFrameTraceThreshold {
		t.Errorf("expected threshold %v, got %v", defaultFrameTraceThreshold, buf.Threshold())
	}

	// Negative values should also use defaults
	buf = NewFrameTraceBuffer(-1, -1)
	if buf.Capacity() != frameTraceSamplesDefault {
		t.Errorf("expected capacity %d, got %d", frameTraceSamplesDefault, buf.Capacity())
	}
	if buf.Threshold() != defaultFrameTraceThreshold {
		t.Errorf("expected threshold %v, got %v", defaultFrameTraceThreshold, buf.Threshold())
	}
}

func TestFrameTraceBuffer_SetThreshold(t *testing.T) {
	buf := NewFrameTraceBuffer(10, 16*time.Millisecond)
	if buf.Threshold() != 16*time.Millisecond {
		t.Fatalf("expected 16ms, got %v", buf.Threshold())
	}

	buf.SetThreshold(32 * time.Millisecond)
	if buf.Threshold() != 32*time.Millisecond {
		t.Fatalf("expected 32ms, got %v", buf.Threshold())
	}

	// Zero/negative should fall back to default
	buf.SetThreshold(0)
	if buf.Threshold() != defaultFrameTraceThreshold {
		t.Fatalf("expected default %v, got %v", defaultFrameTraceThreshold, buf.Threshold())
	}
}

func TestDurationToMillis(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  float64
	}{
		{0, 0},
		{time.Millisecond, 1.0},
		{16667 * time.Microsecond, 16.667},
		{time.Second, 1000.0},
	}
	for _, tt := range tests {
		got := durationToMillis(tt.input)
		if got != tt.want {
			t.Errorf("durationToMillis(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
