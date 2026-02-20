package engine

import (
	"image"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

// GeometryCanvas is a no-op canvas that tracks only translation and clip state.
// When EmbedPlatformView is called, it resolves the global offset and clip and
// buffers the geometry. When OccludePlatformViews is called, it records an
// occlusion region. FlushToSink applies z-order occlusion and reports final
// geometry to the sink.
//
// Uses the same transform/clip logic as CompositingCanvas (via transformTracker)
// but without forwarding to any inner canvas.
type GeometryCanvas struct {
	tracker    transformTracker
	sink       PlatformViewSink
	size       graphics.Size
	views      []pendingViewGeometry
	occlusions []occlusionRegion
	seqCounter int
}

// pendingViewGeometry holds buffered platform view geometry awaiting occlusion.
type pendingViewGeometry struct {
	viewID     int64
	offset     graphics.Offset
	size       graphics.Size
	clipBounds *graphics.Rect
	seqIndex   int
}

// occlusionRegion is a path in global coordinates that occludes platform
// views with a lower sequence index.
type occlusionRegion struct {
	path     *graphics.Path
	seqIndex int
}

// NewGeometryCanvas creates a geometry-only canvas that reports platform view
// positions to the given sink.
func NewGeometryCanvas(size graphics.Size, sink PlatformViewSink) *GeometryCanvas {
	return &GeometryCanvas{
		size: size,
		sink: sink,
	}
}

func (c *GeometryCanvas) Save()                                        { c.tracker.save() }
func (c *GeometryCanvas) SaveLayerAlpha(_ graphics.Rect, _ float64)    { c.tracker.save() }
func (c *GeometryCanvas) SaveLayer(_ graphics.Rect, _ *graphics.Paint) { c.tracker.save() }
func (c *GeometryCanvas) Restore()                                     { c.tracker.restore() }
func (c *GeometryCanvas) Translate(dx, dy float64)                     { c.tracker.translate(dx, dy) }
func (c *GeometryCanvas) ClipRect(rect graphics.Rect)                  { c.tracker.clipRect(rect) }
func (c *GeometryCanvas) ClipRRect(rrect graphics.RRect)               { c.tracker.clipRRect(rrect) }
func (c *GeometryCanvas) SaveLayerBlur(_ graphics.Rect, _, _ float64)  { c.tracker.save() }

// Scale is a no-op. Platform view geometry is reported in logical coordinates;
// the consumer (e.g. Android UI thread) applies device density scaling.
func (c *GeometryCanvas) Scale(_, _ float64) {}
func (c *GeometryCanvas) Rotate(_ float64)   {}

func (c *GeometryCanvas) ClipPath(_ *graphics.Path, _ graphics.ClipOp, _ bool) {}

func (c *GeometryCanvas) Clear(_ graphics.Color)                                    {}
func (c *GeometryCanvas) DrawRect(_ graphics.Rect, _ graphics.Paint)                {}
func (c *GeometryCanvas) DrawRRect(_ graphics.RRect, _ graphics.Paint)              {}
func (c *GeometryCanvas) DrawCircle(_ graphics.Offset, _ float64, _ graphics.Paint) {}
func (c *GeometryCanvas) DrawLine(_, _ graphics.Offset, _ graphics.Paint)           {}
func (c *GeometryCanvas) DrawText(_ *graphics.TextLayout, _ graphics.Offset)        {}
func (c *GeometryCanvas) DrawImage(_ image.Image, _ graphics.Offset)                {}
func (c *GeometryCanvas) DrawImageRect(_ image.Image, _, _ graphics.Rect, _ graphics.FilterQuality, _ uintptr) {
}
func (c *GeometryCanvas) DrawPath(_ *graphics.Path, _ graphics.Paint)                       {}
func (c *GeometryCanvas) DrawRectShadow(_ graphics.Rect, _ graphics.BoxShadow)              {}
func (c *GeometryCanvas) DrawRRectShadow(_ graphics.RRect, _ graphics.BoxShadow)            {}
func (c *GeometryCanvas) DrawSVG(_ unsafe.Pointer, _ graphics.Rect)                         {}
func (c *GeometryCanvas) DrawSVGTinted(_ unsafe.Pointer, _ graphics.Rect, _ graphics.Color) {}
func (c *GeometryCanvas) DrawLottie(_ unsafe.Pointer, _ graphics.Rect, _ float64)           {}

// EmbedPlatformView resolves transform+clip and buffers the view geometry with
// a z-order sequence index for later occlusion processing.
func (c *GeometryCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {
	offset := c.tracker.transform
	clipBounds := c.tracker.currentClip()
	c.views = append(c.views, pendingViewGeometry{
		viewID:     viewID,
		offset:     offset,
		size:       size,
		clipBounds: clipBounds,
		seqIndex:   c.seqCounter,
	})
	c.seqCounter++
}

// OccludePlatformViews records a path mask (in local coordinates) that occludes
// platform views painted before this call. The path is translated to global
// coordinates using the current translation.
func (c *GeometryCanvas) OccludePlatformViews(mask *graphics.Path) {
	globalPath := mask.Translate(c.tracker.transform.X, c.tracker.transform.Y)
	c.occlusions = append(c.occlusions, occlusionRegion{
		path:     globalPath,
		seqIndex: c.seqCounter,
	})
	c.seqCounter++
}

// FlushToSink applies z-order occlusion and sends final view geometry to the sink.
// Fast path: if no occlusion ops were recorded, views are sent directly.
func (c *GeometryCanvas) FlushToSink() {
	if c.sink == nil {
		return
	}

	// Fast path: no occlusions recorded this frame.
	if len(c.occlusions) == 0 {
		for _, v := range c.views {
			viewBounds := graphics.RectFromLTWH(v.offset.X, v.offset.Y, v.size.Width, v.size.Height)
			visibleRect := viewBounds
			if v.clipBounds != nil {
				visibleRect = viewBounds.Intersect(*v.clipBounds)
			}
			c.sink.UpdateViewGeometry(v.viewID, v.offset, v.size, v.clipBounds, visibleRect, []*graphics.Path{})
		}
		return
	}

	// Slow path: apply occlusion to each view.
	for _, v := range c.views {
		viewBounds := graphics.RectFromLTWH(v.offset.X, v.offset.Y, v.size.Width, v.size.Height)

		// Compute visibleRect: view bounds intersected with parent clip.
		visibleRect := viewBounds
		if v.clipBounds != nil {
			visibleRect = viewBounds.Intersect(*v.clipBounds)
		}

		// Collect intersecting occlusion paths from higher z-order items.
		// Use path bounding rect for intersection pre-filter.
		var occlusionPaths []*graphics.Path
		for _, occ := range c.occlusions {
			if occ.seqIndex <= v.seqIndex {
				continue
			}
			occBounds := occ.path.Bounds()
			clipped := occBounds.Intersect(visibleRect)
			if !clipped.IsEmpty() {
				occlusionPaths = append(occlusionPaths, occ.path)
			}
		}

		// Merge overlapping paths into single rect paths. This prevents
		// even-odd fill issues on iOS where multiple overlapping subpaths
		// (e.g. barrier + dialog both emitting full-screen occlusion)
		// cancel each other out in the CAShapeLayer mask.
		occlusionPaths = mergeOverlappingPaths(occlusionPaths)

		// Cap at 8 paths; collapse to bounding rect path if exceeded.
		occlusionPaths = capOcclusionPaths(occlusionPaths)

		// Compute collapsed clipBounds via iterative subtractRect (Android fallback).
		collapsedRect := visibleRect
		hidden := false
		for _, occ := range c.occlusions {
			if occ.seqIndex <= v.seqIndex {
				continue
			}
			collapsedRect, hidden = subtractRect(collapsedRect, occ.path.Bounds())
			if hidden {
				break
			}
		}

		if hidden || collapsedRect.IsEmpty() {
			emptyClip := graphics.Rect{}
			c.sink.UpdateViewGeometry(v.viewID, v.offset, v.size, &emptyClip, visibleRect, occlusionPaths)
		} else {
			c.sink.UpdateViewGeometry(v.viewID, v.offset, v.size, &collapsedRect, visibleRect, occlusionPaths)
		}
	}
}

// ResetFrame clears all buffered state for the next frame.
func (c *GeometryCanvas) ResetFrame() {
	c.views = c.views[:0]
	c.occlusions = c.occlusions[:0]
	c.seqCounter = 0
	c.tracker = transformTracker{}
}

func (c *GeometryCanvas) Size() graphics.Size {
	return c.size
}

// mergeOverlappingPaths merges occlusion paths whose bounding rects overlap
// into single rectangular paths. This prevents even-odd fill issues on iOS
// where multiple overlapping subpaths in a CAShapeLayer mask cancel each
// other out. Non-overlapping paths are preserved as-is, keeping precise
// shapes (e.g. rounded rects for buttons).
//
// The merge is iterative: when two paths merge, the result may overlap with
// additional paths, so the loop repeats until stable.
func mergeOverlappingPaths(paths []*graphics.Path) []*graphics.Path {
	if len(paths) <= 1 {
		return paths
	}

	type entry struct {
		path   *graphics.Path
		bounds graphics.Rect
		alive  bool
	}
	entries := make([]entry, len(paths))
	for i, p := range paths {
		entries[i] = entry{path: p, bounds: p.Bounds(), alive: true}
	}

	// Repeat until no merges occur.
	for {
		merged := false
		for i := range entries {
			if !entries[i].alive {
				continue
			}
			for j := i + 1; j < len(entries); j++ {
				if !entries[j].alive {
					continue
				}
				overlap := entries[i].bounds.Intersect(entries[j].bounds)
				if !overlap.IsEmpty() {
					// Merge: union of bounding rects.
					union := graphics.Rect{
						Left:   min(entries[i].bounds.Left, entries[j].bounds.Left),
						Top:    min(entries[i].bounds.Top, entries[j].bounds.Top),
						Right:  max(entries[i].bounds.Right, entries[j].bounds.Right),
						Bottom: max(entries[i].bounds.Bottom, entries[j].bounds.Bottom),
					}
					newPath := graphics.NewPath()
					newPath.AddRect(union)
					entries[i] = entry{path: newPath, bounds: union, alive: true}
					entries[j].alive = false
					merged = true
				}
			}
		}
		if !merged {
			break
		}
	}

	var result []*graphics.Path
	for _, e := range entries {
		if e.alive {
			result = append(result, e.path)
		}
	}
	if result == nil {
		result = []*graphics.Path{}
	}
	return result
}

// capOcclusionPaths caps the number of occlusion paths at 8. If the count
// exceeds 8, all paths collapse into a single bounding-rect path.
// Returns the input slice unchanged if len <= 8.
func capOcclusionPaths(paths []*graphics.Path) []*graphics.Path {
	if len(paths) <= 8 {
		return paths
	}
	// Compute union of all path bounding rects.
	bounds := paths[0].Bounds()
	for _, p := range paths[1:] {
		b := p.Bounds()
		if b.Left < bounds.Left {
			bounds.Left = b.Left
		}
		if b.Top < bounds.Top {
			bounds.Top = b.Top
		}
		if b.Right > bounds.Right {
			bounds.Right = b.Right
		}
		if b.Bottom > bounds.Bottom {
			bounds.Bottom = b.Bottom
		}
	}
	collapsed := graphics.NewPath()
	collapsed.AddRect(bounds)
	return []*graphics.Path{collapsed}
}

// subtractRect subtracts the occluder from the view rect.
// Returns the remaining visible rect and whether the view is fully hidden.
//
// Safety-first: only produces a result when the subtraction yields a single
// rectangle (edge removal). Center holes, corner bites, and other non-rectangular
// results hide the view entirely to prevent native content from leaking through.
func subtractRect(view, occluder graphics.Rect) (graphics.Rect, bool) {
	overlap := view.Intersect(occluder)
	if overlap.IsEmpty() {
		// No overlap: view unchanged.
		return view, false
	}

	// Full containment: view is completely hidden.
	if occluder.Left <= view.Left && occluder.Right >= view.Right &&
		occluder.Top <= view.Top && occluder.Bottom >= view.Bottom {
		return graphics.Rect{}, true
	}

	// Edge removal: occluder covers one full side of the view.
	// Check each edge and return the remaining strip.

	// Occluder covers the full width and removes from the top.
	if occluder.Left <= view.Left && occluder.Right >= view.Right && occluder.Top <= view.Top && occluder.Bottom < view.Bottom {
		return graphics.Rect{Left: view.Left, Top: occluder.Bottom, Right: view.Right, Bottom: view.Bottom}, false
	}

	// Occluder covers the full width and removes from the bottom.
	if occluder.Left <= view.Left && occluder.Right >= view.Right && occluder.Bottom >= view.Bottom && occluder.Top > view.Top {
		return graphics.Rect{Left: view.Left, Top: view.Top, Right: view.Right, Bottom: occluder.Top}, false
	}

	// Occluder covers the full height and removes from the left.
	if occluder.Top <= view.Top && occluder.Bottom >= view.Bottom && occluder.Left <= view.Left && occluder.Right < view.Right {
		return graphics.Rect{Left: occluder.Right, Top: view.Top, Right: view.Right, Bottom: view.Bottom}, false
	}

	// Occluder covers the full height and removes from the right.
	if occluder.Top <= view.Top && occluder.Bottom >= view.Bottom && occluder.Right >= view.Right && occluder.Left > view.Left {
		return graphics.Rect{Left: view.Left, Top: view.Top, Right: occluder.Left, Bottom: view.Bottom}, false
	}

	// Non-rectangular result (center hole, corner bite, etc.): hide entirely.
	return graphics.Rect{}, true
}
