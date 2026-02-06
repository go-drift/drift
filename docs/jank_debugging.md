# Jank Debugging/Profiling Plan (Debug Server + Frame Tracing)

This document proposes lightweight instrumentation for intermittent jank, and a debug server route to surface frame/phase timings and “dirty” counts in real time.

## Goals
- Identify which frame phase spikes: build, layout, semantics, paint, platform view flush.
- Correlate spikes with tree size, dirty boundary counts, and navigation depth.
- Keep overhead low and always-on in debug builds.

---

## Proposed Debug Server Route
**Route**: `/frame-timeline` (or `/frames`)

**Response shape (example)**
```json
{
  "samples": [
    {
      "ts": 1710000000,
      "frameMs": 18.2,
      "phases": {
        "buildMs": 1.9,
        "layoutMs": 3.2,
        "semanticsMs": 0.0,
        "recordMs": 4.2,
        "compositeMs": 4.5,
        "platformFlushMs": 2.1
      },
      "counts": {
        "dirtyLayout": 42,
        "dirtyPaintBoundaries": 5,
        "dirtySemantics": 1,
        "renderNodeCount": 680,
        "widgetNodeCount": 740,
        "routeDepth": 3
      },
      "flags": {
        "semanticsDeferred": false,
        "hasPlatformViews": true
      }
    }
  ],
  "droppedFrames": 4
}
```

**Why**
- Gives an at-a-glance view of where time is spent during a jank event.
- Counts help explain spikes (e.g., paint spike with many dirty boundaries, or layout spike with large tree).

---

## Minimal Instrumentation Plan

### 1) Capture phase timings in `engine.go`
Wrap these phases with `time.Now()` markers and store into a ring buffer:
- `FlushBuild`
- `FlushLayoutForRoot`
- `flushSemanticsIfNeeded`
- `paintBoundaryToLayer` loop
- `paintTreeWithLayers`
- `platform.GetPlatformViewRegistry().FlushGeometryBatch()`

Where:
- `pkg/engine/engine.go` (frame loop)

### 2) Track dirty counts
- **Dirty paint boundaries count**: `len(pipeline.FlushPaint())` before iterating.
- **Dirty semantics count**: `len(pipeline.FlushSemantics())` or wrap in `flushSemanticsIfNeeded` with a count.
- **Dirty layout count**: track size of `dirtyLayout` inside `PipelineOwner` before flush; if not exposed, add a small method or count inside engine.

Where:
- `pkg/layout/pipeline.go` (expose counts) or add getters
- `pkg/engine/engine.go` (collect counts per frame)

### 3) Tree size + navigation depth
- **Render tree node count**: traverse from `rootRender` (lightweight counter only, avoid full serialization).
- **Widget tree node count**: optional, or reuse debug server’s widget tree serializer in a counting mode.
- **Navigation depth**: count `routes` in `navigatorState` if accessible; or track during navigation observer callbacks.

Where:
- `pkg/engine/debug_server.go` (add `countRenderTree` helper)
- `pkg/navigation/navigator.go` (optional observer for depth tracking)

### 4) Store samples in a ring buffer
- Keep last N frames (e.g., 120 or 240).
- Include dropped frame count (frameMs > 16.6 or > 24). Threshold should be configurable.

Where:
- New type in `pkg/engine/` (e.g., `frame_trace.go`) used by `engine.go` and `debug_server.go`.

---

## Debug Server Endpoint Implementation

**Add to `startDebugServer(...)` mux**:
- `mux.HandleFunc("/frame-timeline", handleFrameTimeline)`

**`handleFrameTimeline`**
- Acquire `frameLock` briefly to read the ring buffer snapshot.
- Return JSON of most recent frames.
- Keep the response small (do not include full trees).

Where:
- `pkg/engine/debug_server.go`

---

## Optional Enhancements
- **Spike logging**: on frames > threshold, write a compact log line with phase timings and counts.
- **Toggleable**: add a diagnostics flag to enable/disable tracing to avoid extra overhead.
- **Per-route annotation**: store current top route name (if available) on each frame for correlation.

---

## Implementation Order (suggested)
1) Add ring buffer and frame trace struct in `pkg/engine/`.
2) Instrument `engine.go` phase timings and counts.
3) Add `/frame-timeline` endpoint in `debug_server.go`.
4) Add optional spike logging + diagnostics config toggle.

