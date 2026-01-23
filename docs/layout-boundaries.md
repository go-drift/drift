# Layout Boundaries

Layout boundaries are an optimization that prevents layout changes from propagating unnecessarily up the render tree. This document explains how they work in Drift, following Flutter's approach.

## The Problem

Without optimization, any layout change would require re-laying out the entire tree. If a text widget deep in the hierarchy changes its content, naive propagation would:

1. Mark the text as needing layout
2. Bubble up to the root
3. Re-layout the entire tree from root down

This is wasteful when the change is contained - for example, text inside a fixed-size container doesn't affect anything outside that container.

## What is a Relayout Boundary?

A relayout boundary is a render object that **contains** layout changes. When a descendant needs layout, the dirty marking stops at the boundary instead of continuing to the root. Layout then runs from the boundary down, skipping all ancestors.

A node becomes a relayout boundary when any of these conditions are true:

| Condition | Why it's a boundary |
|-----------|---------------------|
| `constraints.IsTight()` | Parent dictates exact size - child's internal changes can't affect parent |
| `parent == nil` | Root node - nothing above to propagate to |
| `parentUsesSize == false` | Parent doesn't care about our size - size changes are invisible to parent |

## Key Components

### RenderBoxBase.MarkNeedsLayout()

When a render object needs layout (e.g., its content changed), `MarkNeedsLayout()` is called. This method:

1. Sets `needsLayout = true` on the current node
2. Walks up to the parent and calls `parent.MarkNeedsLayout()`
3. Repeats until reaching a relayout boundary
4. The boundary schedules itself with `PipelineOwner.ScheduleLayout()`

```
MarkNeedsLayout() called on deep node
       │
       ▼
┌─────────────────────────────────────────────────────┐
│  Node A (boundary)     needsLayout = true           │◄── Scheduled
│    │                                                │
│    └─► Node B          needsLayout = true           │
│          │                                          │
│          └─► Node C    needsLayout = true           │◄── Original caller
└─────────────────────────────────────────────────────┘
```

Every node in the path gets `needsLayout = true`. This is critical - when layout runs from the boundary, it must propagate through all intermediate nodes to reach the originally dirty node.

### RenderBoxBase.Layout(constraints, parentUsesSize)

The base `Layout()` method handles boundary determination and delegation:

1. **Determine boundary status** - Check if this node should be a boundary based on the conditions above
2. **Skip if clean** - If `!needsLayout && constraints == cachedConstraints`, return early (key optimization)
3. **Clear dirty flag** - Set `needsLayout = false`
4. **Delegate** - Call `PerformLayout()` on the concrete implementation

```go
func (r *RenderBoxBase) Layout(constraints Constraints, parentUsesSize bool) {
    // 1. Determine boundary
    shouldBeBoundary := constraints.IsTight() || r.parent == nil || !parentUsesSize
    if shouldBeBoundary {
        r.relayoutBoundary = r.self
    } else if r.parent != nil {
        r.relayoutBoundary = parent.RelayoutBoundary() // inherit from parent
    }

    // 2. Skip if clean
    if !r.needsLayout && r.constraints == constraints {
        return
    }

    // 3. Clear dirty flag
    r.constraints = constraints
    r.needsLayout = false

    // 4. Delegate to concrete implementation
    r.self.PerformLayout()
}
```

### PerformLayout()

Concrete render objects implement `PerformLayout()` to:

1. Compute their own size based on constraints
2. Layout children by calling `child.Layout(childConstraints, parentUsesSize)`
3. Position children by setting their `parentData.Offset`

### PipelineOwner

The `PipelineOwner` tracks which boundaries need layout and processes them:

- `ScheduleLayout(object)` - Adds a boundary to the dirty list (O(1) dedup via map)
- `FlushLayoutForRoot(root, constraints)` - Layouts from root, then processes any newly dirty boundaries
- `flushDirtyBoundaries()` - Processes boundaries in depth order (parents first)

Processing parents first is important: if both a parent and child boundary are dirty, laying out the parent may layout the child as a side effect, avoiding redundant work.

## Layout Flow

### Frame Sequence

A typical frame follows this sequence:

1. **FlushBuild** - Rebuilds dirty elements, calls `UpdateRenderObject()` on render objects
2. **FlushLayoutForRoot** - Lays out from root through all dirty subtrees
3. **Paint** - Renders the tree

### When Content Changes

Example: User types in a text field, changing the text content.

```
1. Text content changes
   └─► renderText.update() called with new text
       └─► renderText.MarkNeedsLayout()

2. MarkNeedsLayout walks up the tree
   └─► Each node gets needsLayout = true
       └─► Stops at boundary, boundary is scheduled

3. Next frame: FlushLayoutForRoot()
   └─► root.Layout(screenConstraints, false)
       └─► Propagates down through dirty nodes
           └─► Each node with needsLayout=true runs PerformLayout()
               └─► Eventually reaches renderText
                   └─► renderText.PerformLayout() recomputes text layout
```

### When Only Paint Changes

If only visual properties change (e.g., color) without affecting size:

1. `UpdateRenderObject()` updates the color field
2. `MarkNeedsLayout()` is called (content might have changed)
3. `MarkNeedsPaint()` is called
4. During layout, if text metrics are unchanged, cached layout is reused
5. During paint, the new color is used

## Common Patterns

### Tight Constraints (Most Common Boundary)

```go
// Parent gives child exact size - child becomes a boundary
child.Layout(Constraints{
    MinWidth: 100, MaxWidth: 100,   // tight width
    MinHeight: 50, MaxHeight: 50,   // tight height
}, true)
```

The child's internal layout changes cannot affect the parent since the size is fixed.

### parentUsesSize = false

```go
// Parent doesn't care about child's size - child becomes a boundary
child.Layout(looseConstraints, false)
```

Used when the parent positions the child but doesn't use the child's size for its own layout decisions.

### Scrollable Viewports

Scroll containers typically make their content a boundary:

```go
// Viewport gives content unbounded height, doesn't use content size for own layout
content.Layout(Constraints{
    MinWidth: viewportWidth, MaxWidth: viewportWidth,
    MinHeight: 0, MaxHeight: math.Inf(1),
}, false)  // parentUsesSize = false
```

The content can be arbitrarily tall without affecting the viewport's layout.

## Debugging Layout Issues

If layout isn't propagating correctly:

1. **Check needsLayout marking** - Verify all nodes from the dirty node to the boundary have `needsLayout = true`
2. **Check boundary determination** - Verify the boundary is set correctly based on constraints and parentUsesSize
3. **Check skip condition** - The `!needsLayout && constraints == cached` check might be skipping layout unexpectedly
4. **Check PerformLayout** - Ensure the concrete implementation actually runs and updates state

## References

- Flutter's RenderObject: https://api.flutter.dev/flutter/rendering/RenderObject-class.html
- Flutter's layout protocol: https://docs.flutter.dev/resources/architectural-overview#layout-and-rendering
