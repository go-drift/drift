package engine

import (
	"testing"
	"time"
)

func TestFrameTimingBuffer_Add(t *testing.T) {
	buf := NewFrameTimingBuffer(3)

	// Initially empty
	if buf.Count() != 0 {
		t.Errorf("expected count 0, got %d", buf.Count())
	}
	if samples := buf.Samples(); samples != nil {
		t.Errorf("expected nil samples, got %v", samples)
	}

	// Add first sample
	buf.Add(10 * time.Millisecond)
	if buf.Count() != 1 {
		t.Errorf("expected count 1, got %d", buf.Count())
	}
	samples := buf.Samples()
	if len(samples) != 1 || samples[0] != 10*time.Millisecond {
		t.Errorf("expected [10ms], got %v", samples)
	}

	// Fill buffer
	buf.Add(20 * time.Millisecond)
	buf.Add(30 * time.Millisecond)
	if buf.Count() != 3 {
		t.Errorf("expected count 3, got %d", buf.Count())
	}
	samples = buf.Samples()
	if len(samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(samples))
	}
	// Verify order: [10, 20, 30]
	expected := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	for i, e := range expected {
		if samples[i] != e {
			t.Errorf("sample[%d]: expected %v, got %v", i, e, samples[i])
		}
	}

	// Overflow - should wrap around
	buf.Add(40 * time.Millisecond)
	if buf.Count() != 3 {
		t.Errorf("expected count 3 after overflow, got %d", buf.Count())
	}
	samples = buf.Samples()
	// Verify order: [20, 30, 40] (oldest sample dropped)
	expected = []time.Duration{20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond}
	for i, e := range expected {
		if samples[i] != e {
			t.Errorf("sample[%d]: expected %v, got %v", i, e, samples[i])
		}
	}
}

func TestFrameTimingBuffer_DefaultCapacity(t *testing.T) {
	buf := NewFrameTimingBuffer(0)
	// Should default to 60
	for i := range 65 {
		buf.Add(time.Duration(i) * time.Millisecond)
	}
	if buf.Count() != 60 {
		t.Errorf("expected capacity 60, got count %d", buf.Count())
	}
}

func TestDefaultDiagnosticsConfig(t *testing.T) {
	config := DefaultDiagnosticsConfig()

	if !config.ShowFPS {
		t.Error("expected ShowFPS to be true")
	}
	if !config.ShowFrameGraph {
		t.Error("expected ShowFrameGraph to be true")
	}
	if config.Position != DiagnosticsTopRight {
		t.Errorf("expected Position TopRight, got %v", config.Position)
	}
	if config.GraphSamples != 60 {
		t.Errorf("expected GraphSamples 60, got %d", config.GraphSamples)
	}
	// ~16.67ms for 60fps
	expectedTarget := 16667 * time.Microsecond
	if config.TargetFrameTime != expectedTarget {
		t.Errorf("expected TargetFrameTime %v, got %v", expectedTarget, config.TargetFrameTime)
	}
}

func TestFrameTimingBuffer_SamplesInto(t *testing.T) {
	buf := NewFrameTimingBuffer(5)

	// Add samples
	buf.Add(10 * time.Millisecond)
	buf.Add(20 * time.Millisecond)
	buf.Add(30 * time.Millisecond)

	// Test with exact size buffer
	dst := make([]time.Duration, 3)
	n := buf.SamplesInto(dst)
	if n != 3 {
		t.Errorf("expected 3 samples, got %d", n)
	}
	expected := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	for i, e := range expected {
		if dst[i] != e {
			t.Errorf("sample[%d]: expected %v, got %v", i, e, dst[i])
		}
	}

	// Test with larger buffer
	largeDst := make([]time.Duration, 10)
	n = buf.SamplesInto(largeDst)
	if n != 3 {
		t.Errorf("expected 3 samples with large buffer, got %d", n)
	}

	// Test with smaller buffer (should truncate)
	smallDst := make([]time.Duration, 2)
	n = buf.SamplesInto(smallDst)
	if n != 2 {
		t.Errorf("expected 2 samples with small buffer, got %d", n)
	}
	if smallDst[0] != 10*time.Millisecond || smallDst[1] != 20*time.Millisecond {
		t.Errorf("expected [10ms, 20ms], got %v", smallDst[:n])
	}

	// Test with wrapped buffer
	buf.Add(40 * time.Millisecond)
	buf.Add(50 * time.Millisecond)
	buf.Add(60 * time.Millisecond) // This wraps, drops 10ms

	dst = make([]time.Duration, 5)
	n = buf.SamplesInto(dst)
	if n != 5 {
		t.Errorf("expected 5 samples after wrap, got %d", n)
	}
	// Should be [20, 30, 40, 50, 60] after wrap
	expected = []time.Duration{20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond, 60 * time.Millisecond}
	for i, e := range expected {
		if dst[i] != e {
			t.Errorf("wrapped sample[%d]: expected %v, got %v", i, e, dst[i])
		}
	}
}
