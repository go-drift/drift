# Drift: Scroll Pipeline for Platform Views on Android

Drag Input to Skia/Platform View Coordinate Updates

---

## Overview

When a user drags to scroll on Android, the framework must update two independent rendering surfaces in sync:

- **Skia content**: rendered into a GLSurfaceView via OpenGL
- **Platform views**: native Android Views positioned via translationX/Y

This document traces every step from the raw MotionEvent through to the final coordinate updates on both surfaces, identifying exactly where lag can occur.

---

## Pipeline Stages

### Stage 1: Touch Input Capture (Android Main Thread)

| | |
|---|---|
| **File** | `DriftSurfaceView.kt:271-322` |
| **Thread** | Android main thread |

`onTouchEvent()` captures the raw MotionEvent. For ACTION_MOVE, it iterates all active pointers and calls into Go via JNI:

```kotlin
NativeBridge.pointerEvent(pointerID, 1 /* MOVE */, x, y)
```

After dispatching, it calls `renderNow()` (line 320) which:

1. Calls `NativeBridge.requestFrame()` to mark the engine dirty
2. Calls `requestRender()` to wake the GL thread
3. Calls `scheduleFrame()` for Choreographer follow-up

**Key detail:** `renderNow()` runs AFTER the pointer dispatch, so the GL thread sees the updated scroll offset.

---

### Stage 2: Pointer Event Dispatch (Go Engine)

| | |
|---|---|
| **File** | `engine.go:669-757` |
| **Thread** | Android main thread (via JNI) |

`HandlePointer()` processes the event:

1. Scales device pixels to logical: `position = {x/scale, y/scale}` (line 699)
2. Computes delta from last known position (line 701-706)
3. On DOWN: hit-tests the render tree to find gesture handlers (line 708-725)
4. Creates a `gestures.PointerEvent` and dispatches to all handlers (line 740-750)

---

### Stage 3: Drag Recognition and Scroll Offset Update

| | |
|---|---|
| **Recognizer** | `gestures/recognizers.go:310-380` |
| **Scroll widget** | `widgets/scroll.go:286-341` |
| **Scroll position** | `widgets/scroll.go:616-628` |

The `axisDragRecognizer.handleMove()` computes the primary axis delta and fires the `OnUpdate` callback (line 369), which calls:

```go
r.position.ApplyUserOffset(-details.PrimaryDelta)
```

`ApplyUserOffset()` applies scroll physics (clamping, boundary conditions) and sets the new offset:

```go
adjusted := physics.ApplyPhysicsToUserOffset(p, delta)
proposed := p.offset + adjusted
overscroll := physics.ApplyBoundaryConditions(p, proposed)
p.SetOffset(proposed - overscroll)
```

`SetOffset()` stores the value and calls `p.notify()` (line 603), which triggers the callback registered at scroll.go:118:

```go
scroll.MarkNeedsPaint()
scroll.MarkNeedsSemanticsUpdate()
```

`MarkNeedsPaint()` marks the scroll widget's repaint boundary as dirty and stops propagation (parents hold stable DrawChildLayer references).

---

### Stage 4: Record Phase (GL Thread)

| | |
|---|---|
| **Entry point** | `engine.go:616-622` |
| **Scroll paint** | `widgets/scroll.go:208-238` |
| **Display list** | `graphics/display_list.go` |

When the GL thread runs `Paint()`, it records dirty layers into display lists. For the scroll widget's repaint boundary:

1. A `PictureRecorder` creates a recording canvas
2. `renderScrollView.Paint()` is called, which:
   - a) Clips to the viewport: `Canvas.ClipRect(viewport)` (line 216). Records `opClipRect`.
   - b) Reads scroll offset: `offset := r.scrollOffset()` (line 221)
   - c) Translates the canvas: `Canvas.Translate(0, -offset)` (line 226). Records `opTranslate{0, -offset}`.
   - d) Tracks in PaintContext: `PushTranslation(0, -offset)` (line 227). Accumulates into `ctx.transform.Y`.
   - e) Paints children: `child.Paint(ctx)` (line 231). Platform views record `opEmbedPlatformView{viewID, size}`.
3. `layer.SetContent(recorder.EndRecording())` stores the display list

At this point, the display list contains the scroll offset baked into an `opTranslate` operation, followed by child draw operations and platform view embed markers.

---

### Stage 5: Composite Phase (GL Thread)

| | |
|---|---|
| **Entry point** | `engine.go:630-635` |
| **CompositingCanvas** | `engine/compositing_canvas.go` |
| **Geometry queue** | `platform/platform_view.go:282-340` |

A `CompositingCanvas` wraps the real Skia canvas and tracks an accumulated transform (`c.transform`). As the display list replays:

1. **opTranslate{0, -350.7}** executes on CompositingCanvas:
   - `c.transform.Y += -350.7` (compositing_canvas.go:78)
   - `c.inner.Translate(0, -350.7)` shifts the Skia canvas

2. **Skia draw operations** execute on the inner canvas, rendered at the translated position.

3. **opEmbedPlatformView{id, size}** executes:
   - Reads `offset := c.transform` (compositing_canvas.go:190)
   - Calls `sink.UpdateViewGeometry(id, offset, size, clipBounds)`
   - The geometry is queued in `batchUpdates` for the current frame

**Critical insight:** Both the Skia canvas position and the platform view geometry are derived from the *same accumulated transform*. At computation time, they are in perfect agreement.

---

### Stage 6: Geometry Flush (GL Thread to Main Thread)

| | |
|---|---|
| **Go side** | `platform/platform_view.go:356-439` |
| **Kotlin side** | `PlatformView.kt:371-426` |
| **Channel** | `PlatformChannel.kt:98` |

`FlushGeometryBatch()` (engine.go:647) encodes all queued geometry updates as JSON and calls the native side via JNI:

```go
channel.Invoke("batchSetGeometry", map[string]any{
    "frameSeq":   N,
    "geometries": [{viewId, x, y, width, height, clip...}],
})
```

The JNI call runs `handleMethodCallNative()` **on the GL thread** (not the main thread). Inside `batchSetGeometry()`:

1. Checks `frameSeq <= lastAppliedSeq` to skip stale batches
2. Detects it is NOT on the main thread (`Looper.myLooper() != MainLooper`)
3. Posts the geometry application closure to the main thread:

```kotlin
geometryHandler.postAtFrontOfQueue {
    applyGeometries()
    NativeBridge.geometryApplied()
}
```

4. Returns immediately (does not wait for main thread)

**This is the async boundary.** The closure is queued but not yet executed.

---

### Stage 7: Skia Flush and Geometry Wait (GL Thread)

| | |
|---|---|
| **File** | `engine_skia.go:126-129` |
| **Timeout** | 8ms (half a 60fps frame) |

After flushing geometry:

1. `surface.Flush()` submits all Skia GPU commands (line 126). The GPU begins executing draw operations in parallel.
2. `WaitGeometryApplied(8ms)` blocks the GL thread (line 129), waiting for a signal from the native side.

The 8ms timeout is chosen as roughly half a 60fps frame (16.7ms). If the main thread applies geometry within 8ms, the signal arrives and the GL thread proceeds. If not, it times out and swaps buffers anyway, causing the platform view to lag by one frame.

---

### Stage 8: Native Geometry Application (Main Thread)

| | |
|---|---|
| **File** | `PlatformView.kt:301-364` |
| **Thread** | Android main thread |

When the main thread Looper picks up the posted closure:

1. `applyViewGeometry()` sets position via RenderNode properties:

```kotlin
view.layoutParams = FrameLayout.LayoutParams(
    (width * density).toInt(),
    (height * density).toInt()
)
view.translationX = x * density   // RenderNode property
view.translationY = y * density   // RenderNode property
```

2. `applyClipBounds()` converts global clip to view-local coordinates and sets `view.clipBounds`
3. `NativeBridge.geometryApplied()` signals the GL thread via `SignalGeometryApplied()` (platform_view.go:465)

`translationX/Y` are RenderNode properties: they sync to the Android RenderThread without triggering a full measure/layout traversal.

---

### Stage 9: Buffer Swap and Surface Compositing

After `WaitGeometryApplied()` returns (either by signal or timeout):

1. `onDrawFrame()` returns on the GL thread
2. GLSurfaceView swaps buffers, making Skia content visible
3. Android's RenderThread picks up the new translationX/Y values and composites the native View at the updated position
4. SurfaceFlinger composites the GL surface and the View surface together

---

## Lag Analysis

### Source 1: Main Thread Contention (Most Likely)

The `postAtFrontOfQueue` closure needs the main thread Looper to execute. During scrolling, the main thread is also processing MotionEvents, accessibility dispatches, and other system work. If the main thread cannot run the closure within the 8ms `WaitGeometryApplied` timeout, the GL thread gives up and swaps buffers with stale platform view positions.

| | |
|---|---|
| **Symptom** | Platform views visibly trail Skia content by ~1 frame |
| **Frequency** | Worse on slower devices or with complex View hierarchies |

### Source 2: layoutParams Triggering Measure/Layout

Inside `applyViewGeometry()`, setting `view.layoutParams` schedules a measure/layout traversal on the main thread. Even though `translationX/Y` are fast RenderNode properties, the layoutParams assignment can delay the RenderThread from picking up the new position if it triggers a synchronous layout pass.

| | |
|---|---|
| **Symptom** | Intermittent stutter even when main thread is responsive |
| **Fix candidate** | Cache layoutParams, only set when size actually changes |

### Source 3: Two-Surface Compositing

Skia renders into a GLSurfaceView (one EGL surface). Platform views live in a separate window/surface layer. Android's SurfaceFlinger composites these independently. Even if both update within the same frame, SurfaceFlinger may present them at slightly different times, creating a visible tear during fast scrolling.

| | |
|---|---|
| **Symptom** | Subtle "tearing" or shimmer at platform view edges |
| **Mitigation** | Inherent to multi-surface architecture; reduce by syncing swapBuffers with geometry application |

---

## Frame Timeline Summary

| Step | Thread | Key Operation |
|------|--------|---------------|
| 1. Touch capture | Main | onTouchEvent, pointerEvent JNI call |
| 2. Pointer dispatch | Main (JNI) | HandlePointer, hit test, gesture dispatch |
| 3. Scroll offset | Main (JNI) | ApplyUserOffset, MarkNeedsPaint |
| 4. Trigger render | Main | renderNow: requestRender + scheduleFrame |
| 5. Record layers | GL | PictureRecorder, opTranslate, opEmbedPlatformView |
| 6. Composite | GL | CompositingCanvas accumulates transform, queues geometry |
| 7. Flush geometry | GL | batchSetGeometry via JNI, posts to main thread |
| 8. GPU submit | GL | surface.Flush(), GPU work begins in parallel |
| 9. Wait signal | GL | WaitGeometryApplied blocks up to 8ms |
| 10. Apply geometry | Main | translationX/Y set, geometryApplied signal sent |
| 11. Buffer swap | GL | onDrawFrame returns, GLSurfaceView swaps |
| 12. Composite | RenderThread | RenderNode picks up translationX/Y |
| 13. Display | SurfaceFlinger | GL surface + View surface composited |

---

## Key File Locations

| Component | File |
|-----------|------|
| Touch input | `DriftSurfaceView.kt:271-322` |
| Pointer dispatch | `pkg/engine/engine.go:669-757` |
| Drag recognizer | `pkg/gestures/recognizers.go:310-380` |
| Scroll widget | `pkg/widgets/scroll.go:208-238` |
| Display list ops | `pkg/graphics/display_list.go` |
| CompositingCanvas | `pkg/engine/compositing_canvas.go` |
| Paint pipeline | `pkg/engine/engine.go:600-647` |
| Geometry registry | `pkg/platform/platform_view.go:282-461` |
| Skia render | `pkg/engine/engine_skia.go:104-132` |
| Native geometry | `PlatformView.kt:301-426` |
