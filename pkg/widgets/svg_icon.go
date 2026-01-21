package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/svg"
)

// SVGIcon renders an SVG icon from a file or loaded icon.
type SVGIcon struct {
	// Icon is the pre-loaded SVG icon to render.
	Icon *svg.Icon
	// Size is the desired size (width and height) for the icon.
	// If zero, uses the SVG's intrinsic viewBox size.
	Size float64
	// Color overrides the SVG's fill colors with a tint color.
	// If zero (transparent), uses the SVG's original colors.
	Color rendering.Color
	// SemanticLabel provides an accessibility description of the icon.
	SemanticLabel string
	// ExcludeFromSemantics excludes the icon from the semantics tree when true.
	ExcludeFromSemantics bool
}

func (s SVGIcon) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s SVGIcon) Key() any {
	return nil
}

func (s SVGIcon) Child() core.Widget {
	return nil
}

func (s SVGIcon) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	box := &renderSVGIcon{
		icon:                 s.Icon,
		size:                 s.Size,
		color:                s.Color,
		semanticLabel:        s.SemanticLabel,
		excludeFromSemantics: s.ExcludeFromSemantics,
	}
	box.SetSelf(box)
	return box
}

func (s SVGIcon) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if box, ok := renderObject.(*renderSVGIcon); ok {
		box.icon = s.Icon
		box.size = s.Size
		box.color = s.Color
		box.semanticLabel = s.SemanticLabel
		box.excludeFromSemantics = s.ExcludeFromSemantics
		box.MarkNeedsLayout()
		box.MarkNeedsPaint()
	}
}

type renderSVGIcon struct {
	layout.RenderBoxBase
	icon                 *svg.Icon
	size                 float64
	color                rendering.Color
	semanticLabel        string
	excludeFromSemantics bool
}

func (r *renderSVGIcon) SetChild(child layout.RenderObject) {
	// SVGIcon has no children
}

func (r *renderSVGIcon) Layout(constraints layout.Constraints) {
	var size rendering.Size

	if r.size > 0 {
		// Use specified size
		size = rendering.Size{Width: r.size, Height: r.size}
	} else if r.icon != nil {
		// Use viewBox size
		vb := r.icon.ViewBox()
		size = rendering.Size{Width: vb.Width(), Height: vb.Height()}
	} else {
		// Default size
		size = rendering.Size{Width: 24, Height: 24}
	}

	r.SetSize(constraints.Constrain(size))
}

func (r *renderSVGIcon) Paint(ctx *layout.PaintContext) {
	if r.icon == nil {
		return
	}

	bounds := rendering.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height)
	r.icon.Draw(ctx.Canvas, bounds, r.color)
}

func (r *renderSVGIcon) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
