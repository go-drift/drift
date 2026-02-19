package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/svg"
)

// SvgImage renders an SVG from a loaded [svg.Icon].
//
// The SVG is rendered using Skia's native SVG DOM, which supports gradients,
// filters, transforms, and most SVG features.
//
// # Creation Pattern
//
// Use struct literal:
//
//	icon, _ := svg.LoadFile("icon.svg")
//	widgets.SvgImage{
//	    Source:        icon,
//	    Width:         100,
//	    Height:        50,
//	    SemanticLabel: "Logo",
//	}
//
// # Sizing Behavior
//
// The SVG always scales to fill the widget bounds, respecting preserveAspectRatio
// (default: contain, centered). When Width and/or Height are specified, the SVG
// scales to fit. When both are zero, the widget uses the SVG's viewBox dimensions,
// and the SVG scales to match (visually equivalent to intrinsic sizing).
//
// # CSS Styling Limitation
//
// Skia's SVG DOM does not fully support CSS class-based styling. SVGs that use
// <style> blocks with class selectors (e.g., .st0{fill:#FF0000}) will render
// with default colors (typically black). This is common in SVGs exported from
// Adobe Illustrator.
//
// To fix, convert CSS classes to inline presentation attributes using a tool
// like svgo (https://github.com/nicolo-ribaudo/svgo-browser or npm svgo):
//
//	svgo --config '{"plugins":[{"name":"inlineStyles","params":{"onlyMatchedOnce":false}}]}' input.svg -o output.svg
//
// # Lifetime
//
// The Source must remain valid for as long as any widget or display list
// references it. Do not call [svg.Icon.Destroy] while widgets may still
// render the icon.
type SvgImage struct {
	// Source is the pre-loaded SVG to render.
	Source *svg.Icon

	// Width is the desired width. If zero and Height is set, calculated from aspect ratio.
	// If both zero, uses the SVG's intrinsic viewBox width.
	Width float64

	// Height is the desired height. If zero and Width is set, calculated from aspect ratio.
	// If both zero, uses the SVG's intrinsic viewBox height.
	Height float64

	// PreserveAspectRatio controls how the SVG scales within its bounds.
	// If nil, uses the SVG's intrinsic preserveAspectRatio (default: contain, centered).
	// Note: Setting this mutates the Source Icon.
	PreserveAspectRatio *svg.PreserveAspectRatio

	// TintColor replaces all SVG colors with this color while preserving alpha.
	// Zero (ColorTransparent) means no tinting - original colors are preserved.
	// Note: Tinting affects ALL content including gradients and embedded images.
	TintColor graphics.Color

	// SemanticLabel provides an accessibility description.
	SemanticLabel string

	// ExcludeFromSemantics excludes from the semantics tree when true.
	ExcludeFromSemantics bool
}

func (s SvgImage) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (s SvgImage) Key() any {
	return nil
}

func (s SvgImage) Child() core.Widget {
	return nil
}

func (s SvgImage) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderSvgImage{
		source:               s.Source,
		width:                s.Width,
		height:               s.Height,
		preserveAspectRatio:  s.PreserveAspectRatio,
		tintColor:            s.TintColor,
		semanticLabel:        s.SemanticLabel,
		excludeFromSemantics: s.ExcludeFromSemantics,
	}
	box.SetSelf(box)

	// Apply preserveAspectRatio if set
	if s.PreserveAspectRatio != nil && s.Source != nil {
		s.Source.SetPreserveAspectRatio(*s.PreserveAspectRatio)
	}

	return box
}

func (s SvgImage) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderSvgImage); ok {
		// Avoid invalidating layout/paint unless relevant fields actually changed.
		layoutChanged := box.source != s.Source || box.width != s.Width || box.height != s.Height
		paintChanged := layoutChanged || box.tintColor != s.TintColor || preserveAspectRatioChanged(box.preserveAspectRatio, s.PreserveAspectRatio)
		semanticsChanged := box.semanticLabel != s.SemanticLabel || box.excludeFromSemantics != s.ExcludeFromSemantics

		box.source = s.Source
		box.width = s.Width
		box.height = s.Height
		box.preserveAspectRatio = s.PreserveAspectRatio
		box.tintColor = s.TintColor
		box.semanticLabel = s.SemanticLabel
		box.excludeFromSemantics = s.ExcludeFromSemantics

		// Apply preserveAspectRatio if set and changed
		if paintChanged && s.PreserveAspectRatio != nil && s.Source != nil {
			s.Source.SetPreserveAspectRatio(*s.PreserveAspectRatio)
		}

		if layoutChanged {
			box.MarkNeedsLayout()
		}
		if paintChanged {
			box.MarkNeedsPaint()
		}
		if semanticsChanged {
			box.MarkNeedsSemanticsUpdate()
		}
	}
}

func preserveAspectRatioChanged(a, b *svg.PreserveAspectRatio) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return a.Align != b.Align || a.Scale != b.Scale
}

type renderSvgImage struct {
	layout.RenderBoxBase
	source               *svg.Icon
	width                float64
	height               float64
	preserveAspectRatio  *svg.PreserveAspectRatio
	tintColor            graphics.Color
	semanticLabel        string
	excludeFromSemantics bool
}

// IsRepaintBoundary isolates SVG repaints into their own layer.
func (r *renderSvgImage) IsRepaintBoundary() bool {
	return true
}

func (r *renderSvgImage) SetChild(child layout.RenderObject) {
	// SvgImage has no children
}

func (r *renderSvgImage) PerformLayout() {
	constraints := r.Constraints()
	var size graphics.Size

	if r.source != nil {
		vb := r.source.ViewBox()
		aspectRatio := 1.0
		if vb.Height() > 0 {
			aspectRatio = vb.Width() / vb.Height()
		}

		switch {
		case r.width > 0 && r.height > 0:
			// Both specified: use exact dimensions
			size = graphics.Size{Width: r.width, Height: r.height}
		case r.width > 0:
			// Width only: calculate height from aspect ratio
			size = graphics.Size{Width: r.width, Height: r.width / aspectRatio}
		case r.height > 0:
			// Height only: calculate width from aspect ratio
			size = graphics.Size{Width: r.height * aspectRatio, Height: r.height}
		default:
			// Neither: use viewBox size
			size = graphics.Size{Width: vb.Width(), Height: vb.Height()}
		}
	} else {
		size = graphics.Size{Width: 24, Height: 24}
	}

	r.SetSize(constraints.Constrain(size))
}

func (r *renderSvgImage) Paint(ctx *layout.PaintContext) {
	if r.source == nil {
		return
	}

	bounds := graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height)
	r.source.Draw(ctx.Canvas, bounds, r.tintColor)
}

func (r *renderSvgImage) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

// SvgIcon renders an SVG icon within a square bounding box.
// For rectangular bounds or more control over sizing, use [SvgImage] instead.
//
// The icon scales to fit within the square bounds while preserving aspect ratio
// (contain behavior). If the SVG's viewBox is not square, it will be centered
// within the square bounds.
//
// See [SvgImage] documentation for lifetime rules and CSS styling limitations.
//
// Example:
//
//	widgets.SvgIcon{
//	    Source: icon,
//	    Size:   24,
//	}
type SvgIcon struct {
	// Source is the pre-loaded SVG icon to render.
	Source *svg.Icon
	// Size is the width and height for the icon.
	// If zero, uses the SVG's intrinsic viewBox size.
	Size float64
	// TintColor replaces all SVG colors with this color while preserving alpha.
	// Zero (ColorTransparent) means no tinting - original colors are preserved.
	// Note: Tinting affects ALL content including gradients and embedded images.
	TintColor graphics.Color
	// SemanticLabel provides an accessibility description.
	SemanticLabel string
	// ExcludeFromSemantics excludes from the semantics tree when true.
	ExcludeFromSemantics bool
}

func (s SvgIcon) CreateElement() core.Element {
	return SvgImage{
		Source:               s.Source,
		Width:                s.Size,
		Height:               s.Size,
		TintColor:            s.TintColor,
		SemanticLabel:        s.SemanticLabel,
		ExcludeFromSemantics: s.ExcludeFromSemantics,
	}.CreateElement()
}

func (s SvgIcon) Key() any {
	return nil
}
