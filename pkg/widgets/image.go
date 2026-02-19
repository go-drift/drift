package widgets

import (
	"fmt"
	"image"
	"image/draw"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// Image renders a bitmap image onto the canvas with configurable sizing and scaling.
//
// Image accepts a Go [image.Image] as its Source. The image is rendered using
// the specified Fit mode to control scaling behavior.
//
// # Creation Pattern
//
// Use struct literal:
//
//	widgets.Image{
//	    Source:        loadedImage,
//	    Width:         200,
//	    Height:        150,
//	    Fit:           widgets.ImageFitCover,
//	    SemanticLabel: "Product photo",
//	}
//
// # Image Fit Modes
//
//   - ImageFitContain: Scales to fit within the box while maintaining aspect ratio (default)
//   - ImageFitFill: Stretches the image to completely fill the box (may distort)
//   - ImageFitCover: Scales to cover the box while maintaining aspect ratio (may crop)
//   - ImageFitNone: Uses the image's intrinsic size
//   - ImageFitScaleDown: Like Contain, but never scales up
//
// For decorative images that don't convey information, set ExcludeFromSemantics
// to true to hide them from screen readers.
type Image struct {
	// Source is the image to render.
	Source image.Image
	// Width overrides the image width if non-zero.
	Width float64
	// Height overrides the image height if non-zero.
	Height float64
	// Fit controls how the image is scaled within its bounds.
	Fit ImageFit
	// Alignment positions the image within its bounds.
	Alignment layout.Alignment
	// SemanticLabel provides an accessibility description of the image.
	SemanticLabel string
	// ExcludeFromSemantics excludes the image from the semantics tree when true.
	// Use this for decorative images that don't convey meaningful content.
	ExcludeFromSemantics bool
}

// ImageFit controls how an image is scaled within its box.
type ImageFit int

const (
	// ImageFitContain scales the image to fit within its bounds.
	// This is the zero value, making it the default for [Image].
	ImageFitContain ImageFit = iota
	// ImageFitFill stretches the image to fill its bounds.
	ImageFitFill
	// ImageFitCover scales the image to cover its bounds.
	ImageFitCover
	// ImageFitNone leaves the image at its intrinsic size.
	ImageFitNone
	// ImageFitScaleDown fits the image if needed, otherwise keeps intrinsic size.
	ImageFitScaleDown
)

// String returns a human-readable representation of the image fit mode.
func (f ImageFit) String() string {
	switch f {
	case ImageFitFill:
		return "fill"
	case ImageFitContain:
		return "contain"
	case ImageFitCover:
		return "cover"
	case ImageFitNone:
		return "none"
	case ImageFitScaleDown:
		return "scale_down"
	default:
		return fmt.Sprintf("ImageFit(%d)", int(f))
	}
}

// WithFit returns a copy of the image with the specified fit mode.
func (i Image) WithFit(fit ImageFit) Image {
	i.Fit = fit
	return i
}

// WithSize returns a copy of the image with the specified width and height.
func (i Image) WithSize(width, height float64) Image {
	i.Width = width
	i.Height = height
	return i
}

// WithAlignment returns a copy of the image with the specified alignment.
func (i Image) WithAlignment(alignment layout.Alignment) Image {
	i.Alignment = alignment
	return i
}

func (i Image) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (i Image) Key() any {
	return nil
}

func (i Image) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderImage{
		source:               i.Source,
		width:                i.Width,
		height:               i.Height,
		fit:                  i.Fit,
		alignment:            i.Alignment,
		semanticLabel:        i.SemanticLabel,
		excludeFromSemantics: i.ExcludeFromSemantics,
	}
	box.SetSelf(box)
	return box
}

func (i Image) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderImage); ok {
		box.source = i.Source
		box.width = i.Width
		box.height = i.Height
		box.fit = i.Fit
		box.alignment = i.Alignment
		box.semanticLabel = i.SemanticLabel
		box.excludeFromSemantics = i.ExcludeFromSemantics
		box.updateImageCache()
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
		box.MarkNeedsSemanticsUpdate()
	}
}

type renderImage struct {
	layout.RenderBoxBase
	source               image.Image
	width                float64
	height               float64
	fit                  ImageFit
	alignment            layout.Alignment
	intrinsic            graphics.Size
	semanticLabel        string
	excludeFromSemantics bool

	// Cache - cachedRGBA holds the converted image, cachedSource tracks which
	// source was converted, and cacheID is an opaque identifier passed to Skia
	// for SkImage reuse. cacheID increments whenever the source changes.
	cachedRGBA   *image.RGBA
	cachedSource image.Image
	cacheID      uintptr
}

// IsRepaintBoundary isolates image repaints into their own layer.
func (r *renderImage) IsRepaintBoundary() bool {
	return true
}

func (r *renderImage) SetChild(child layout.RenderObject) {
	// Image has no children
}

func (r *renderImage) PerformLayout() {
	constraints := r.Constraints()
	if r.source == nil {
		r.intrinsic = graphics.Size{}
		r.cachedRGBA, r.cachedSource = nil, nil
		r.cacheID = 0
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	bounds := r.source.Bounds()
	intrinsic := graphics.Size{
		Width:  float64(bounds.Dx()),
		Height: float64(bounds.Dy()),
	}
	r.intrinsic = intrinsic
	r.updateImageCache()

	size := intrinsic
	if r.width > 0 && r.height > 0 {
		size = graphics.Size{Width: r.width, Height: r.height}
	} else if r.width > 0 && intrinsic.Width > 0 {
		scale := r.width / intrinsic.Width
		size = graphics.Size{Width: r.width, Height: intrinsic.Height * scale}
	} else if r.height > 0 && intrinsic.Height > 0 {
		scale := r.height / intrinsic.Height
		size = graphics.Size{Width: intrinsic.Width * scale, Height: r.height}
	}

	r.SetSize(constraints.Constrain(size))
}

func (r *renderImage) Paint(ctx *layout.PaintContext) {
	if r.source == nil || r.cachedRGBA == nil {
		return
	}
	size := r.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	if r.intrinsic.Width <= 0 || r.intrinsic.Height <= 0 {
		return
	}

	fit := r.fit
	alignment := r.alignment
	if alignment == (layout.Alignment{}) {
		alignment = layout.AlignmentCenter
	}

	srcRect, dstRect := r.computeFitRects(fit, alignment, size)
	if srcRect.IsEmpty() || dstRect.IsEmpty() {
		return
	}

	ctx.Canvas.Save()
	ctx.Canvas.ClipRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height))
	ctx.Canvas.DrawImageRect(r.cachedRGBA, srcRect, dstRect, graphics.FilterQualityLow, r.cacheKey())
	ctx.Canvas.Restore()
}

func (r *renderImage) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderImage) fitSize(fit ImageFit, size graphics.Size) graphics.Size {
	if r.intrinsic.Width <= 0 || r.intrinsic.Height <= 0 {
		return graphics.Size{}
	}

	switch fit {
	case ImageFitContain:
		scale := min(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return graphics.Size{}
		}
		return graphics.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	case ImageFitCover:
		scale := max(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return graphics.Size{}
		}
		return graphics.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	case ImageFitNone:
		return r.intrinsic
	case ImageFitScaleDown:
		if r.intrinsic.Width <= size.Width && r.intrinsic.Height <= size.Height {
			return r.intrinsic
		}
		scale := min(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return graphics.Size{}
		}
		return graphics.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	default:
		return size
	}
}

// imageCacheIDCounter is a global counter for generating unique cache IDs.
// Using a global ensures IDs are unique across all renderImage instances.
var imageCacheIDCounter atomic.Uintptr

func (r *renderImage) updateImageCache() {
	if r.source == nil {
		r.cachedRGBA = nil
		r.cachedSource = nil
		r.cacheID = 0
		return
	}

	// Cache hit: same source instance, assume data unchanged.
	// Note: If callers mutate pixel data in place on the same image.Image
	// instance, they must pass a new instance to trigger cache invalidation.
	if r.cachedSource == r.source && r.cachedRGBA != nil {
		return
	}

	// Convert and cache
	r.cachedRGBA = toRGBAImage(r.source)
	r.cachedSource = r.source
	r.cacheID = imageCacheIDCounter.Add(1)
}

func (r *renderImage) cacheKey() uintptr {
	return r.cacheID
}

func (r *renderImage) computeFitRects(fit ImageFit, align layout.Alignment, box graphics.Size) (src, dst graphics.Rect) {
	intrinsic := r.intrinsic
	fullSrc := graphics.RectFromLTWH(0, 0, intrinsic.Width, intrinsic.Height)

	switch fit {
	case ImageFitFill:
		return fullSrc, graphics.RectFromLTWH(0, 0, box.Width, box.Height)

	case ImageFitContain, ImageFitScaleDown:
		scale := min(box.Width/intrinsic.Width, box.Height/intrinsic.Height)
		if fit == ImageFitScaleDown && scale > 1 {
			scale = 1
		}
		drawSize := graphics.Size{Width: intrinsic.Width * scale, Height: intrinsic.Height * scale}
		offset := align.WithinRect(graphics.RectFromLTWH(0, 0, box.Width, box.Height), drawSize)
		return fullSrc, graphics.RectFromLTWH(offset.X, offset.Y, drawSize.Width, drawSize.Height)

	case ImageFitCover:
		scale := max(box.Width/intrinsic.Width, box.Height/intrinsic.Height)
		scaledSize := graphics.Size{Width: intrinsic.Width * scale, Height: intrinsic.Height * scale}
		offset := align.WithinRect(graphics.RectFromLTWH(0, 0, box.Width, box.Height), scaledSize)
		// Convert back to source coordinates
		srcX, srcY := -offset.X/scale, -offset.Y/scale
		srcW, srcH := box.Width/scale, box.Height/scale
		return graphics.RectFromLTWH(srcX, srcY, srcW, srcH), graphics.RectFromLTWH(0, 0, box.Width, box.Height)

	case ImageFitNone:
		offset := align.WithinRect(graphics.RectFromLTWH(0, 0, box.Width, box.Height), intrinsic)
		return fullSrc, graphics.RectFromLTWH(offset.X, offset.Y, intrinsic.Width, intrinsic.Height)
	}
	return fullSrc, graphics.RectFromLTWH(0, 0, box.Width, box.Height)
}

func toRGBAImage(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil
	}
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)
	return rgba
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderImage) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	if r.excludeFromSemantics {
		config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsIsHidden)
		return false
	}

	if r.semanticLabel == "" {
		return false
	}

	config.IsSemanticBoundary = true
	config.Properties.Label = r.semanticLabel
	config.Properties.Role = semantics.SemanticsRoleImage
	config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsIsImage)

	return true
}
