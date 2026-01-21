package widgets

import (
	"image"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// Image renders a bitmap image onto the canvas.
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
	// ImageFitFill stretches the image to fill its bounds.
	ImageFitFill ImageFit = iota
	// ImageFitContain scales the image to fit within its bounds.
	ImageFitContain
	// ImageFitCover scales the image to cover its bounds.
	ImageFitCover
	// ImageFitNone leaves the image at its intrinsic size.
	ImageFitNone
	// ImageFitScaleDown fits the image if needed, otherwise keeps intrinsic size.
	ImageFitScaleDown
)

func (i Image) CreateElement() core.Element {
	return core.NewRenderObjectElement(i, nil)
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
	intrinsic            rendering.Size
	semanticLabel        string
	excludeFromSemantics bool
}

func (r *renderImage) SetChild(child layout.RenderObject) {
	// Image has no children
}

func (r *renderImage) Layout(constraints layout.Constraints) {
	if r.source == nil {
		r.intrinsic = rendering.Size{}
		r.SetSize(constraints.Constrain(rendering.Size{}))
		return
	}

	bounds := r.source.Bounds()
	intrinsic := rendering.Size{
		Width:  float64(bounds.Dx()),
		Height: float64(bounds.Dy()),
	}
	r.intrinsic = intrinsic

	size := intrinsic
	if r.width > 0 && r.height > 0 {
		size = rendering.Size{Width: r.width, Height: r.height}
	} else if r.width > 0 && intrinsic.Width > 0 {
		scale := r.width / intrinsic.Width
		size = rendering.Size{Width: r.width, Height: intrinsic.Height * scale}
	} else if r.height > 0 && intrinsic.Height > 0 {
		scale := r.height / intrinsic.Height
		size = rendering.Size{Width: intrinsic.Width * scale, Height: r.height}
	}

	r.SetSize(constraints.Constrain(size))
}

func (r *renderImage) Paint(ctx *layout.PaintContext) {
	if r.source == nil {
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
	if fit == 0 {
		fit = ImageFitFill
	}
	alignment := r.alignment
	if alignment == (layout.Alignment{}) {
		alignment = layout.AlignmentCenter
	}

	drawSize := r.fitSize(fit, size)
	if drawSize.Width <= 0 || drawSize.Height <= 0 {
		return
	}

	scaleX := drawSize.Width / r.intrinsic.Width
	scaleY := drawSize.Height / r.intrinsic.Height
	offset := alignment.WithinRect(rendering.RectFromLTWH(0, 0, size.Width, size.Height), drawSize)

	ctx.Canvas.Save()
	ctx.Canvas.ClipRect(rendering.RectFromLTWH(0, 0, size.Width, size.Height))
	ctx.Canvas.Translate(offset.X, offset.Y)
	if scaleX != 1 || scaleY != 1 {
		ctx.Canvas.Scale(scaleX, scaleY)
	}
	ctx.Canvas.DrawImage(r.source, rendering.Offset{})
	ctx.Canvas.Restore()
}

func (r *renderImage) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderImage) fitSize(fit ImageFit, size rendering.Size) rendering.Size {
	if r.intrinsic.Width <= 0 || r.intrinsic.Height <= 0 {
		return rendering.Size{}
	}

	switch fit {
	case ImageFitContain:
		scale := min(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return rendering.Size{}
		}
		return rendering.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	case ImageFitCover:
		scale := max(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return rendering.Size{}
		}
		return rendering.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	case ImageFitNone:
		return r.intrinsic
	case ImageFitScaleDown:
		if r.intrinsic.Width <= size.Width && r.intrinsic.Height <= size.Height {
			return r.intrinsic
		}
		scale := min(size.Width/r.intrinsic.Width, size.Height/r.intrinsic.Height)
		if scale <= 0 {
			return rendering.Size{}
		}
		return rendering.Size{Width: r.intrinsic.Width * scale, Height: r.intrinsic.Height * scale}
	default:
		return size
	}
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
