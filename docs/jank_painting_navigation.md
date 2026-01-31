# Jank: Painting/Rendering + Navigation Issues (Fix Order)

This document captures the current suspects for intermittent jank related to painting/rendering and navigation. Items are ordered by likely impact and ease of verification, with context to help plan fixes.

## 1) Repaint boundary caching is effectively unused
**Context**
- `PaintContext.PaintChildWithLayer` supports reusing a cached display list for repaint boundaries, but it is never called by render objects.
- Most render objects paint via `ctx.PaintChild(...)`, which always repaints children and bypasses cached layers.
- `engine.paintTreeWithLayers(...)` only reuses the root-level boundary’s cached layer when it is itself a repaint boundary. Subtree caching isn’t used in normal render object `Paint` methods.

**Why this matters**
- Repaint boundaries are meant to reduce paint cost for static subtrees. If caching is never used, any “static” UI still repaints every frame, which can cause occasional spikes (e.g., when GPU driver or system workload fluctuates).

**Where**
- `pkg/layout/paint.go` (PaintChildWithLayer)
- `pkg/widgets/stack.go` (renderStack.Paint)
- `pkg/widgets/view.go` (renderView.Paint)
- Many widget render objects (e.g., container, decorated box, opacity, etc.)

**Fix direction**
- Audit render objects and switch to `PaintChildWithLayer` when painting children so repaint boundary caching is respected.
- Ensure any render object that is a repaint boundary (or is likely to have repaint boundary children) uses the layer-aware path.
- Validate that cached layers are invalidated on `MarkNeedsPaint` and re-recorded by `paintBoundaryToLayer`.

---

## 2) Navigator paints every route in the stack (not just the current route)
**Context**
- `navigatorState.Build` builds *all routes* into a `Stack` and wraps each route in `ExcludeSemantics`. That only affects accessibility; it does *not* prevent paint or layout.
- `renderStack.Paint` iterates all children and paints them in order.
- Result: every route in the navigation stack is painted every frame, even when completely obscured by the top route.

**Why this matters**
- Deep navigation stacks increase paint cost linearly.
- Occasional spikes could correlate with expensive hidden route paints (images, shadows, filters).

**Where**
- `pkg/navigation/navigator.go` (Build)
- `pkg/widgets/stack.go` (renderStack.Paint)

**Fix direction**
- Change `Navigator` to paint only the active route and (optionally) the exiting route during transitions.
- Maintain element tree identity for non-top routes without painting (e.g., by using an offstage/visibility wrapper or a specialized “no paint” widget).
- Consider a “keep alive but not painted” approach for non-top routes to preserve state.

---

## 3) TabScaffold uses IndexedStack: only one tab is painted but all tabs are laid out
**Context**
- `TabScaffold` uses `IndexedStack` for tab bodies, which only paints the active child, but `renderIndexedStack.PerformLayout` lays out all children.
- Large/complex tab trees still incur layout cost even when not visible.

**Why this matters**
- Layout is often cheaper than paint, but still expensive on complex trees. Can contribute to short-term jank when multiple tabs are heavy.

**Where**
- `pkg/navigation/tab_scaffold.go`
- `pkg/widgets/stack.go` (renderIndexedStack.PerformLayout)

**Fix direction**
- Optional optimization: only layout the active tab + keep cached size for inactive tabs.
- Alternatively, add a mode for lazy tabs or an “inactive tabs offstage” widget that short-circuits layout and paint.

---

## 4) Limited paint culling outside ScrollView
**Context**
- `renderScrollView` has special-case paint culling logic (only for certain child types like `renderFlex`/`renderPadding`).
- Most render objects do not cull based on clip bounds.

**Why this matters**
- Large offscreen content still paints unless it is in ScrollView with the specialized path. This can create sporadic spikes when offscreen content is complex.

**Where**
- `pkg/widgets/scroll.go` (paintCulled, paintFlex)

**Fix direction**
- Expand culling in ScrollView to more child types or introduce a generic culling mechanism in layout/paint.
- Consider clip-aware painting in common containers or lists.

---

## 5) Repaint boundary usage is uneven across widgets
**Context**
- Only a few widgets declare `IsRepaintBoundary()` (e.g., `renderScrollView`, `renderOpacity`, `renderBackdropFilter`, diagnostics HUD).
- Many potential “isolate this subtree” candidates do not use boundaries, which can lead to wider repaints than necessary.

**Why this matters**
- Even with layer caching fixed, the tree needs appropriate boundary placement to reduce repaints.

**Where**
- `pkg/widgets/repaint_boundary.go`
- Specific widgets: `pkg/widgets/scroll.go`, `pkg/widgets/opacity.go`, `pkg/widgets/backdrop_filter.go`, `pkg/widgets/diagnostics_hud.go`

**Fix direction**
- After enabling layer reuse, add boundaries to common expensive widgets or components that are good isolation candidates (images, complex containers, etc.).
- Consider an app-level `RepaintBoundary` wrapper in sample apps to verify impact.

