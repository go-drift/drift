# Repaint Boundaries

Repaint boundaries are an optimization that prevents paint changes from requiring a full tree repaint. This document explains how they work in Drift, following Flutter's approach.

## The Problem

Without optimization, any visual change would require repainting the entire tree. If a button deep in the hierarchy animates its color, naive painting would:

1. Clear the entire canvas
2. Repaint every widget from root down
3. Repeat 60+ times per second during animation

This is wasteful when the change is isolated - for example, a blinking cursor shouldn't cause the entire screen to repaint.

## What is a Repaint Boundary?

A repaint boundary is a render object that **isolates** its paint. When a descendant needs repaint, the dirty marking stops at the boundary. The boundary's content is painted to a cached `DisplayList` (layer), which can be replayed without re-executing all the paint commands.

Unlike relayout boundaries (which are determined automatically by constraints), repaint boundaries are **opt-in**. A render object becomes a repaint boundary by overriding `IsRepaintBoundary()` to return `true`.

| Widget | Why it's a boundary |
|--------|---------------------|
| `RepaintBoundary` | Explicit boundary widget for isolating static content |
| `ScrollView` | Scrolling content benefits from cached layers |
| `Opacity` (0 < α < 1) | Uses `SaveLayerAlpha` - already composited separately |
| `BackdropFilter` | Uses blur layer - already composited separately |

## Key Components

### RenderBoxBase Fields

```go
type RenderBoxBase struct {
    // ... other fields ...
    repaintBoundary RenderObject           // cached nearest repaint boundary
    needsPaint      bool                   // local dirty flag
    layer           *graphics.DisplayList // cached paint output (boundaries only)
}
```

### RenderBoxBase.MarkNeedsPaint()

When a render object needs repainting (e.g., color changed), `MarkNeedsPaint()` is called. This method:

1. Invalidates the cached layer (`r.layer = nil`)
2. Sets `needsPaint = true`
3. Walks up to the parent and calls `parent.MarkNeedsPaint()`
4. Repeats until reaching a repaint boundary
5. The boundary schedules itself with `PipelineOwner.SchedulePaint()`

```
MarkNeedsPaint() called on deep node
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│  Boundary A        needsPaint = true, layer = nil       │◄── Scheduled
│    │                                                    │
│    └─► Node B      needsPaint = true                    │
│          │                                              │
│          └─► Node C needsPaint = true                   │◄── Original caller
└─────────────────────────────────────────────────────────┘
```

### RenderBoxBase.Layout() - Boundary Resolution

Repaint boundaries are resolved during layout, similar to relayout boundaries:

```go
// In Layout(), after relayout boundary resolution:
if r.self != nil && r.self.IsRepaintBoundary() {
    r.repaintBoundary = r.self
} else if r.parent != nil {
    r.repaintBoundary = parent.RepaintBoundary() // inherit from parent
}
```

This happens every time `Layout()` is called, even if layout is skipped due to clean constraints.

### PipelineOwner.FlushPaint()

The `FlushPaint()` method returns dirty boundaries sorted by depth (parents first):

```go
func (p *PipelineOwner) FlushPaint() []RenderObject {
    // Sort by depth - parents first
    // Filter to boundaries that still need paint
    // Return sorted list, clear dirty set
}
```

Processing parents first ensures that if both a parent and child boundary are dirty, the parent paints first and may invalidate the child's cached position.

### Layer Caching

When a boundary is painted, its output is recorded to a `DisplayList`:

```go
func paintBoundaryToLayer(boundary layout.RenderObject) {
    recorder := &graphics.PictureRecorder{}
    canvas := recorder.BeginRecording(boundary.Size())

    // Paint boundary's content to recorded canvas
    paintTreeWithLayers(ctx, boundary, Offset{})

    layer := recorder.EndRecording()
    boundary.SetLayer(layer)
    boundary.ClearNeedsPaint()
}
```

During compositing, clean boundaries replay their cached layer instead of repainting:

```go
func paintTreeWithLayers(ctx, node, offset) {
    if node.IsRepaintBoundary() {
        if layer := node.Layer(); layer != nil && !node.NeedsPaint() {
            layer.Paint(ctx.Canvas)  // Replay cached commands
            return
        }
    }
    node.Paint(ctx)  // Full repaint
}
```

## Paint Flow

### Frame Sequence

A typical frame follows this sequence:

1. **FlushBuild** - Rebuilds dirty elements, may call `MarkNeedsPaint()`
2. **FlushLayoutForRoot** - Lays out dirty subtrees, resolves repaint boundaries
3. **FlushPaint** - Returns dirty boundaries sorted by depth
4. **Paint boundaries to layers** - Record each dirty boundary's content
5. **Composite** - Walk tree, replay cached layers or paint directly

### When Visual Properties Change

Example: A button's background color animates.

```
1. Animation ticks, color updates
   └─► renderButton.MarkNeedsPaint()

2. MarkNeedsPaint walks up the tree
   └─► Each node gets needsPaint = true, layer = nil
       └─► Stops at boundary, boundary is scheduled

3. Next frame: FlushPaint()
   └─► Returns [boundary] sorted by depth

4. paintBoundaryToLayer(boundary)
   └─► Records boundary's paint commands to DisplayList
       └─► Sets boundary.layer, clears needsPaint

5. Composite from root
   └─► When reaching boundary, replays cached layer
       └─► Avoids repainting unchanged siblings
```

### When Only Layout Changes

If size changes but visuals don't:

1. `MarkNeedsLayout()` is called
2. Layout propagates to relayout boundary
3. `MarkNeedsPaint()` is typically called too (size affects paint)
4. Repaint boundary caches new content

## Common Patterns

### Isolating Static Content

Wrap expensive-to-paint but rarely-changing content in `RepaintBoundary`:

```go
RepaintBoundary{
    Child: ExpensiveChart{Data: staticData},
}
```

The chart is painted once and cached. Animations elsewhere don't trigger a repaint.

### Scrollable Content

`ScrollView` is automatically a repaint boundary:

```go
func (r *renderScrollView) IsRepaintBoundary() bool {
    return true
}
```

When scrolling, only the scroll view's layer is updated - siblings aren't repainted.

### Opacity and Effects

Widgets that already use compositor layers become boundaries to leverage caching:

```go
func (r *renderOpacity) IsRepaintBoundary() bool {
    return r.opacity > 0 && r.opacity < 1  // Only when using SaveLayerAlpha
}
```

### Implicit vs Explicit Boundaries

| Type | Example | When to use |
|------|---------|-------------|
| Implicit | `ScrollView`, `Opacity` | Automatic - these widgets benefit from isolation |
| Explicit | `RepaintBoundary` widget | Manual - wrap content you know is expensive and stable |

## Differences from Relayout Boundaries

| Aspect | Relayout Boundary | Repaint Boundary |
|--------|-------------------|------------------|
| Determination | Automatic (based on constraints) | Opt-in (`IsRepaintBoundary()`) |
| Caching | None - just limits propagation | Caches paint output in `DisplayList` |
| Scheduling dedup | Via `needsLayout` flag | Via `SchedulePaint()` map check |
| Processing order | Depth-first (parents first) | Depth-first (parents first) |

## Debugging Paint Issues

If repainting isn't working correctly:

1. **Check boundary resolution** - Verify `repaintBoundary` is set correctly after layout
2. **Check scheduling** - Verify the boundary is in `dirtyPaint` after `MarkNeedsPaint()`
3. **Check layer invalidation** - Verify `layer = nil` when content changes
4. **Check needsPaint clearing** - Verify `ClearNeedsPaint()` is called after painting to layer

### Common Issues

**Paint not updating during scroll/animation:**
- The `needsPaint` flag may be stuck `true` without scheduling
- Ensure `MarkNeedsPaint()` always reaches `SchedulePaint()` for boundaries

**Stale cached content:**
- The layer wasn't invalidated (`layer = nil`)
- Ensure `MarkNeedsPaint()` is called when content changes

**Performance regression:**
- Too many boundaries can hurt performance (layer overhead)
- Use boundaries strategically for expensive, stable content

## References

- Flutter's RenderObject.isRepaintBoundary: https://api.flutter.dev/flutter/rendering/RenderObject/isRepaintBoundary.html
- Flutter's RenderRepaintBoundary: https://api.flutter.dev/flutter/rendering/RenderRepaintBoundary-class.html
- Flutter's Layer tree: https://docs.flutter.dev/resources/architectural-overview#layer-and-compositing
