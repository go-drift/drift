package engine

import (
	"runtime"
	"sync"
	"time"
)

const (
	runtimeSampleIntervalDefault = 5 * time.Second
	runtimeSampleWindowDefault   = 60 * time.Second
	runtimeSampleMinInterval     = 1 * time.Second
	runtimeSampleMaxSamples      = 120
)

// RuntimeSample captures a snapshot of runtime memory/GC stats.
type RuntimeSample struct {
	Timestamp    int64  `json:"ts"`
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapInuse    uint64 `json:"heapInuse"`
	HeapSys      uint64 `json:"heapSys"`
	NumGC        uint32 `json:"numGC"`
	LastGCTime   int64  `json:"lastGCTime"`
	PauseTotalNs uint64 `json:"pauseTotalNs"`
	LastPauseNs  uint64 `json:"lastPauseNs"`
}

// RuntimeSampleBuffer stores recent runtime samples in a ring buffer.
type RuntimeSampleBuffer struct {
	mu       sync.RWMutex
	samples  []RuntimeSample
	index    int
	count    int
	interval time.Duration
	window   time.Duration
}

type runtimeSampler struct {
	mu   sync.Mutex
	stop chan struct{}
}

var runtimeSamplerState runtimeSampler

// NewRuntimeSampleBuffer creates a buffer sized for the configured window/interval.
func NewRuntimeSampleBuffer(window, interval time.Duration) *RuntimeSampleBuffer {
	interval = normalizeRuntimeInterval(interval)
	window = normalizeRuntimeWindow(window, interval)

	capacity := int(window / interval)
	if capacity < 1 {
		capacity = 1
	}
	if capacity > runtimeSampleMaxSamples {
		capacity = runtimeSampleMaxSamples
	}
	window = time.Duration(capacity) * interval

	return &RuntimeSampleBuffer{
		samples:  make([]RuntimeSample, capacity),
		interval: interval,
		window:   window,
	}
}

// Interval returns the sampling interval.
func (b *RuntimeSampleBuffer) Interval() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.interval
}

// Window returns the history window covered by the buffer.
func (b *RuntimeSampleBuffer) Window() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.window
}

// Add stores a runtime sample.
func (b *RuntimeSampleBuffer) Add(sample RuntimeSample) {
	b.mu.Lock()
	b.samples[b.index] = sample
	b.index = (b.index + 1) % len(b.samples)
	if b.count < len(b.samples) {
		b.count++
	}
	b.mu.Unlock()
}

// Snapshot returns samples in chronological order.
func (b *RuntimeSampleBuffer) Snapshot() []RuntimeSample {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	result := make([]RuntimeSample, b.count)
	if b.count < len(b.samples) {
		copy(result, b.samples[:b.count])
	} else {
		copy(result, b.samples[b.index:])
		copy(result[len(b.samples)-b.index:], b.samples[:b.index])
	}

	return result
}

func normalizeRuntimeInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = runtimeSampleIntervalDefault
	}
	if interval < runtimeSampleMinInterval {
		interval = runtimeSampleMinInterval
	}
	return interval
}

func normalizeRuntimeWindow(window, interval time.Duration) time.Duration {
	if window <= 0 {
		window = runtimeSampleWindowDefault
	}
	if window < interval {
		window = interval
	}
	return window
}

func runtimeSampleConfig(config *DiagnosticsConfig) (time.Duration, time.Duration) {
	if config == nil {
		return 0, 0
	}
	interval := normalizeRuntimeInterval(config.RuntimeSampleInterval)
	window := normalizeRuntimeWindow(config.RuntimeSampleWindow, interval)
	return interval, window
}

func readRuntimeSample() RuntimeSample {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	lastPause := uint64(0)
	if stats.NumGC > 0 {
		index := (stats.NumGC + 255) % 256
		lastPause = stats.PauseNs[index]
	}

	lastGC := int64(0)
	if stats.LastGC > 0 {
		lastGC = time.Unix(0, int64(stats.LastGC)).UnixMilli()
	}

	return RuntimeSample{
		Timestamp:    time.Now().UnixMilli(),
		HeapAlloc:    stats.HeapAlloc,
		HeapInuse:    stats.HeapInuse,
		HeapSys:      stats.HeapSys,
		NumGC:        stats.NumGC,
		LastGCTime:   lastGC,
		PauseTotalNs: stats.PauseTotalNs,
		LastPauseNs:  lastPause,
	}
}

func startRuntimeSampler(buffer *RuntimeSampleBuffer, interval time.Duration) {
	interval = normalizeRuntimeInterval(interval)
	if buffer == nil {
		stopRuntimeSampler()
		return
	}

	runtimeSamplerState.mu.Lock()
	if runtimeSamplerState.stop != nil {
		close(runtimeSamplerState.stop)
		runtimeSamplerState.stop = nil
	}
	stopCh := make(chan struct{})
	runtimeSamplerState.stop = stopCh
	runtimeSamplerState.mu.Unlock()

	buffer.Add(readRuntimeSample())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				buffer.Add(readRuntimeSample())
			case <-stopCh:
				return
			}
		}
	}()
}

func stopRuntimeSampler() {
	runtimeSamplerState.mu.Lock()
	if runtimeSamplerState.stop != nil {
		close(runtimeSamplerState.stop)
		runtimeSamplerState.stop = nil
	}
	runtimeSamplerState.mu.Unlock()
}
