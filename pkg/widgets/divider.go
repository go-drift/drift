package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Divider renders a thin horizontal line with optional insets and spacing.
//
// Divider is typically used as a child of a [Column] or any vertically-stacked
// layout. It expands to fill the available width and occupies Height pixels of
// vertical space, drawing a centered line of the given Thickness.
//
// # Styling Model
//
// Divider is explicit by default: all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - Height: 0 means zero vertical space (not rendered)
//   - Thickness: 0 means no visible line
//   - Color: 0 means transparent (invisible)
//
// For theme-styled dividers, use [theme.DividerOf] which pre-fills values from
// the current theme's [DividerThemeData] (color from OutlineVariant, 16px space,
// 1px thickness).
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Divider{
//	    Height:    16,
//	    Thickness: 1,
//	    Color:     graphics.RGB(200, 200, 200),
//	    Indent:    16,  // 16px left inset
//	}
//
// Themed (reads from current theme):
//
//	theme.DividerOf(ctx)
type Divider struct {
	// Height is the total vertical space the divider occupies.
	Height float64
	// Thickness is the thickness of the drawn line.
	Thickness float64
	// Color is the line color. Zero means transparent.
	Color graphics.Color
	// Indent is the left inset from the leading edge.
	Indent float64
	// EndIndent is the right inset from the trailing edge.
	EndIndent float64
}

func (d Divider) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d Divider) Key() any {
	return nil
}

func (d Divider) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderDivider{
		height:    d.Height,
		thickness: d.Thickness,
		color:     d.Color,
		indent:    d.Indent,
		endIndent: d.EndIndent,
	}
	r.SetSelf(r)
	return r
}

func (d Divider) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderDivider); ok {
		needsLayout := r.height != d.Height
		r.height = d.Height
		r.thickness = d.Thickness
		r.color = d.Color
		r.indent = d.Indent
		r.endIndent = d.EndIndent
		if needsLayout {
			r.MarkNeedsLayout()
		}
		r.MarkNeedsPaint()
	}
}

type renderDivider struct {
	layout.RenderBoxBase
	height    float64
	thickness float64
	color     graphics.Color
	indent    float64
	endIndent float64
}

func (r *renderDivider) PerformLayout() {
	constraints := r.Constraints()
	size := constraints.Constrain(graphics.Size{Width: constraints.MaxWidth, Height: r.height})
	r.SetSize(size)
}

func (r *renderDivider) Paint(ctx *layout.PaintContext) {
	if r.color == 0 || r.thickness <= 0 {
		return
	}
	size := r.Size()
	drawWidth := size.Width - r.indent - r.endIndent
	if drawWidth <= 0 {
		return
	}
	top := (size.Height - r.thickness) / 2
	paint := graphics.DefaultPaint()
	paint.Color = r.color
	rect := graphics.RectFromLTWH(r.indent, top, drawWidth, r.thickness)
	ctx.Canvas.DrawRect(rect, paint)
}

func (r *renderDivider) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

// VerticalDivider renders a thin vertical line with optional insets and spacing.
//
// VerticalDivider is typically used as a child of a [Row] or any
// horizontally-stacked layout. It expands to fill the available height and
// occupies Width pixels of horizontal space, drawing a centered line of the
// given Thickness.
//
// # Styling Model
//
// VerticalDivider is explicit by default: all visual properties use their struct
// field values directly. A zero value means zero, not "use theme default."
// For example:
//
//   - Width: 0 means zero horizontal space (not rendered)
//   - Thickness: 0 means no visible line
//   - Color: 0 means transparent (invisible)
//
// For theme-styled vertical dividers, use [theme.VerticalDividerOf] which
// pre-fills values from the current theme's [DividerThemeData] (color from
// OutlineVariant, 16px space, 1px thickness).
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.VerticalDivider{
//	    Width:     16,
//	    Thickness: 1,
//	    Color:     graphics.RGB(200, 200, 200),
//	    Indent:    8,  // 8px top inset
//	}
//
// Themed (reads from current theme):
//
//	theme.VerticalDividerOf(ctx)
type VerticalDivider struct {
	// Width is the total horizontal space the divider occupies.
	Width float64
	// Thickness is the thickness of the drawn line.
	Thickness float64
	// Color is the line color. Zero means transparent.
	Color graphics.Color
	// Indent is the top inset.
	Indent float64
	// EndIndent is the bottom inset.
	EndIndent float64
}

func (d VerticalDivider) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d VerticalDivider) Key() any {
	return nil
}

func (d VerticalDivider) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderVerticalDivider{
		width:     d.Width,
		thickness: d.Thickness,
		color:     d.Color,
		indent:    d.Indent,
		endIndent: d.EndIndent,
	}
	r.SetSelf(r)
	return r
}

func (d VerticalDivider) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderVerticalDivider); ok {
		needsLayout := r.width != d.Width
		r.width = d.Width
		r.thickness = d.Thickness
		r.color = d.Color
		r.indent = d.Indent
		r.endIndent = d.EndIndent
		if needsLayout {
			r.MarkNeedsLayout()
		}
		r.MarkNeedsPaint()
	}
}

type renderVerticalDivider struct {
	layout.RenderBoxBase
	width     float64
	thickness float64
	color     graphics.Color
	indent    float64
	endIndent float64
}

func (r *renderVerticalDivider) PerformLayout() {
	constraints := r.Constraints()
	size := constraints.Constrain(graphics.Size{Width: r.width, Height: constraints.MaxHeight})
	r.SetSize(size)
}

func (r *renderVerticalDivider) Paint(ctx *layout.PaintContext) {
	if r.color == 0 || r.thickness <= 0 {
		return
	}
	size := r.Size()
	drawHeight := size.Height - r.indent - r.endIndent
	if drawHeight <= 0 {
		return
	}
	left := (size.Width - r.thickness) / 2
	paint := graphics.DefaultPaint()
	paint.Color = r.color
	rect := graphics.RectFromLTWH(left, r.indent, r.thickness, drawHeight)
	ctx.Canvas.DrawRect(rect, paint)
}

func (r *renderVerticalDivider) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}
