package engine

import (
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
)

const (
	frameTraceSamplesDefault   = 240
	defaultFrameTraceThreshold = 16667 * time.Microsecond
)

// FramePhaseTimings captures time spent in each frame phase (ms).
type FramePhaseTimings struct {
	DispatchMs      float64 `json:"dispatchMs"`
	AnimateMs       float64 `json:"animateMs"`
	BuildMs         float64 `json:"buildMs"`
	LayoutMs        float64 `json:"layoutMs"`
	SemanticsMs     float64 `json:"semanticsMs"`
	RecordMs        float64 `json:"recordMs"`
	GeometryMs      float64 `json:"geometryMs"`
	TraceOverheadMs float64 `json:"traceOverheadMs"`
}

// FrameCounts captures per-frame workload indicators.
type FrameCounts struct {
	DirtyLayout          int `json:"dirtyLayout"`
	DirtyPaintBoundaries int `json:"dirtyPaintBoundaries"`
	DirtySemantics       int `json:"dirtySemantics"`
	RenderNodeCount      int `json:"renderNodeCount"`
	WidgetNodeCount      int `json:"widgetNodeCount"`
	PlatformViewCount    int `json:"platformViewCount"`
}

// FrameFlags captures contextual flags for a frame.
type FrameFlags struct {
	SemanticsDeferred bool   `json:"semanticsDeferred"`
	LifecycleState    string `json:"lifecycleState,omitempty"`
	ResumedThisFrame  bool   `json:"resumedThisFrame,omitempty"`
}

// FrameSample is a single frame trace sample.
type FrameSample struct {
	Timestamp  int64             `json:"ts"`
	FrameMs    float64           `json:"frameMs"`
	Phases     FramePhaseTimings `json:"phases"`
	Counts     FrameCounts       `json:"counts"`
	Flags      FrameFlags        `json:"flags"`
	DirtyTypes FrameDirtyTypes   `json:"dirtyTypes,omitempty"`
}

// FrameDirtyTypes provides the most common dirty types per phase.
type FrameDirtyTypes struct {
	Layout    []layout.TypeCount `json:"layout,omitempty"`
	Paint     []layout.TypeCount `json:"paint,omitempty"`
	Semantics []layout.TypeCount `json:"semantics,omitempty"`
}

// FrameTimeline is the debug server response shape.
type FrameTimeline struct {
	Samples       []FrameSample `json:"samples"`
	DroppedFrames int           `json:"droppedFrames"`
	ThresholdMs   float64       `json:"thresholdMs"`
}

// FrameTraceBuffer stores recent frame samples in a ring buffer.
type FrameTraceBuffer struct {
	mu        sync.RWMutex
	samples   []FrameSample
	index     int
	count     int
	dropped   int
	threshold time.Duration
}

// NewFrameTraceBuffer creates a new frame trace buffer.
func NewFrameTraceBuffer(capacity int, threshold time.Duration) *FrameTraceBuffer {
	if capacity <= 0 {
		capacity = frameTraceSamplesDefault
	}
	if threshold <= 0 {
		threshold = defaultFrameTraceThreshold
	}
	return &FrameTraceBuffer{
		samples:   make([]FrameSample, capacity),
		threshold: threshold,
	}
}

// Capacity returns the buffer capacity.
func (b *FrameTraceBuffer) Capacity() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.samples)
}

// SetThreshold updates the dropped frame threshold.
func (b *FrameTraceBuffer) SetThreshold(threshold time.Duration) {
	if threshold <= 0 {
		threshold = defaultFrameTraceThreshold
	}
	b.mu.Lock()
	b.threshold = threshold
	b.mu.Unlock()
}

// Threshold returns the dropped frame threshold.
func (b *FrameTraceBuffer) Threshold() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.threshold
}

// Add records a frame sample and updates dropped frame count.
func (b *FrameTraceBuffer) Add(sample FrameSample, frameDuration time.Duration) {
	b.mu.Lock()
	b.samples[b.index] = sample
	b.index = (b.index + 1) % len(b.samples)
	if b.count < len(b.samples) {
		b.count++
	}
	if frameDuration > b.threshold {
		b.dropped++
	}
	b.mu.Unlock()
}

// Snapshot returns a chronological copy of samples and stats.
func (b *FrameTraceBuffer) Snapshot() FrameTimeline {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return FrameTimeline{ThresholdMs: durationToMillis(b.threshold)}
	}

	result := make([]FrameSample, b.count)
	if b.count < len(b.samples) {
		copy(result, b.samples[:b.count])
	} else {
		copy(result, b.samples[b.index:])
		copy(result[len(b.samples)-b.index:], b.samples[:b.index])
	}

	return FrameTimeline{
		Samples:       result,
		DroppedFrames: b.dropped,
		ThresholdMs:   durationToMillis(b.threshold),
	}
}

func durationToMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func countRenderTree(root layout.RenderObject) int {
	if root == nil {
		return 0
	}
	count := 1
	if cv, ok := root.(layout.ChildVisitor); ok {
		cv.VisitChildren(func(child layout.RenderObject) {
			count += countRenderTree(child)
		})
	}
	return count
}

func countWidgetTree(root core.Element) int {
	if root == nil {
		return 0
	}
	count := 1
	root.VisitChildren(func(child core.Element) bool {
		count += countWidgetTree(child)
		return true
	})
	return count
}
