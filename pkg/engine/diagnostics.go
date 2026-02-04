package engine

import (
	"sync"
	"time"
)

// DiagnosticsPosition specifies where the diagnostics HUD is displayed.
type DiagnosticsPosition int

const (
	// DiagnosticsTopLeft positions the HUD in the top-left corner.
	DiagnosticsTopLeft DiagnosticsPosition = iota
	// DiagnosticsTopRight positions the HUD in the top-right corner.
	DiagnosticsTopRight
	// DiagnosticsBottomLeft positions the HUD in the bottom-left corner.
	DiagnosticsBottomLeft
	// DiagnosticsBottomRight positions the HUD in the bottom-right corner.
	DiagnosticsBottomRight
)

// DiagnosticsConfig controls what diagnostics are displayed.
type DiagnosticsConfig struct {
	// ShowFPS displays the frames per second counter.
	ShowFPS bool
	// ShowFrameGraph displays a graph of frame times.
	ShowFrameGraph bool
	// ShowLayoutBounds draws colored borders around all widget bounds.
	ShowLayoutBounds bool
	// Position controls where the HUD is displayed.
	Position DiagnosticsPosition
	// GraphSamples is the number of frame samples to display in the graph.
	// Defaults to 60 if zero.
	GraphSamples int
	// TargetFrameTime is the target frame duration for coloring the graph.
	// Defaults to 16.67ms (60fps) if zero.
	TargetFrameTime time.Duration
	// DebugServerPort enables an HTTP debug server on the specified port.
	// 0 = disabled, >0 = port number (e.g., 9999).
	// The server exposes /tree (render tree JSON) and /health endpoints.
	DebugServerPort int
	// RuntimeSampleInterval controls how often runtime samples are collected.
	// Defaults to 5s if zero.
	RuntimeSampleInterval time.Duration
	// RuntimeSampleWindow controls how much runtime history is kept.
	// Defaults to 60s if zero.
	RuntimeSampleWindow time.Duration
}

// DefaultDiagnosticsConfig returns a DiagnosticsConfig with sensible defaults.
func DefaultDiagnosticsConfig() *DiagnosticsConfig {
	return &DiagnosticsConfig{
		ShowFPS:               true,
		ShowFrameGraph:        true,
		Position:              DiagnosticsTopRight,
		GraphSamples:          60,
		TargetFrameTime:       16667 * time.Microsecond, // ~16.67ms for 60fps
		RuntimeSampleInterval: 5 * time.Second,
		RuntimeSampleWindow:   60 * time.Second,
	}
}

// FrameTimingBuffer is a ring buffer for storing frame durations.
type FrameTimingBuffer struct {
	mu       sync.RWMutex
	samples  []time.Duration
	index    int
	capacity int
	count    int
}

// NewFrameTimingBuffer creates a new FrameTimingBuffer with the given capacity.
func NewFrameTimingBuffer(capacity int) *FrameTimingBuffer {
	if capacity <= 0 {
		capacity = 60
	}
	return &FrameTimingBuffer{
		samples:  make([]time.Duration, capacity),
		capacity: capacity,
	}
}

// Add records a frame duration to the buffer.
func (b *FrameTimingBuffer) Add(duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.samples[b.index] = duration
	b.index = (b.index + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

// Samples returns a copy of the frame samples in chronological order.
func (b *FrameTimingBuffer) Samples() []time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	result := make([]time.Duration, b.count)
	if b.count < b.capacity {
		// Buffer not yet full - samples start at 0
		copy(result, b.samples[:b.count])
	} else {
		// Buffer full - oldest sample is at b.index
		copy(result, b.samples[b.index:])
		copy(result[b.capacity-b.index:], b.samples[:b.index])
	}
	return result
}

// SamplesInto copies frame samples into the provided slice and returns the number copied.
// This avoids allocation if dst is reused. Samples are in chronological order.
func (b *FrameTimingBuffer) SamplesInto(dst []time.Duration) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return 0
	}

	n := min(b.count, len(dst))

	if b.count < b.capacity {
		copy(dst[:n], b.samples[:n])
	} else {
		// Buffer full - oldest sample is at b.index
		firstPart := b.capacity - b.index
		if firstPart >= n {
			copy(dst[:n], b.samples[b.index:b.index+n])
		} else {
			copy(dst[:firstPart], b.samples[b.index:])
			copy(dst[firstPart:n], b.samples[:n-firstPart])
		}
	}
	return n
}

// Count returns the number of samples currently in the buffer.
func (b *FrameTimingBuffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}
