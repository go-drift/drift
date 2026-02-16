package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// RichText displays a tree of styled text spans. Unlike [Text], which applies
// a single style to the entire paragraph, RichText supports inline style
// changes (color, weight, decoration, font) within a single paragraph.
//
// Style provides widget-level defaults (color, font size, etc.) that act as
// the lowest-priority base. The Content span tree's own styles override Style,
// and child spans override their parents as usual.
//
// Basic usage:
//
//	widgets.RichText{
//	    Content: graphics.Spans(
//	        graphics.Span("Hello "),
//	        graphics.Span("World").Bold(),
//	    ),
//	}.WithStyle(graphics.SpanStyle{Color: colors.OnSurface, FontSize: 16})
type RichText struct {
	// Content is the root span tree. Child spans inherit any style fields from
	// their parent for fields left at their zero value.
	Content graphics.TextSpan
	// Style is the widget-level default style. Spans inherit these values for
	// any zero-valued fields not already set by the span tree.
	Style    graphics.SpanStyle
	Align    graphics.TextAlign
	MaxLines int
	// Wrap enables text wrapping at the constraint width. When false (default),
	// text renders on a single line and may overflow.
	Wrap bool
}

// WithStyle returns a copy with the given widget-level default style.
func (r RichText) WithStyle(style graphics.SpanStyle) RichText {
	r.Style = style
	return r
}

// WithWrap returns a copy with text wrapping enabled or disabled.
func (r RichText) WithWrap(wrap bool) RichText {
	r.Wrap = wrap
	return r
}

// WithMaxLines returns a copy with the specified maximum line count.
func (r RichText) WithMaxLines(maxLines int) RichText {
	r.MaxLines = maxLines
	return r
}

// WithAlign returns a copy with the specified text alignment.
func (r RichText) WithAlign(align graphics.TextAlign) RichText {
	r.Align = align
	return r
}

func (r RichText) CreateElement() core.Element {
	return core.NewRenderObjectElement(r, nil)
}

func (r RichText) Key() any {
	return nil
}

func (r RichText) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	ro := &renderRichText{
		span:      r.Content,
		text:      r.Content.PlainText(),
		baseStyle: r.Style,
		align:     r.Align,
		maxLines:  r.MaxLines,
		wrap:      r.Wrap,
	}
	ro.SetSelf(ro)
	return ro
}

func (r RichText) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if ro, ok := renderObject.(*renderRichText); ok {
		ro.span = r.Content
		ro.text = r.Content.PlainText()
		ro.baseStyle = r.Style
		ro.align = r.Align
		ro.maxLines = r.MaxLines
		ro.wrap = r.Wrap
		ro.generation++
		ro.MarkNeedsLayout()
		ro.MarkNeedsPaint()
	}
}

type renderRichText struct {
	layout.RenderBoxBase
	span       graphics.TextSpan
	text       string
	baseStyle  graphics.SpanStyle
	align      graphics.TextAlign
	textLayout *graphics.TextLayout
	maxLines   int
	wrap       bool
	generation uint64
	cache      richTextLayoutCache
}

type richTextLayoutCache struct {
	generation uint64
	align      graphics.TextAlign
	maxWidth   float64
	maxLines   int
	wrap       bool
}

func (r *renderRichText) PerformLayout() {
	constraints := r.Constraints()
	maxWidth := float64(0)
	if r.wrap {
		maxWidth = constraints.MaxWidth
	}
	current := richTextLayoutCache{
		generation: r.generation,
		align:      r.align,
		maxWidth:   maxWidth,
		maxLines:   r.maxLines,
		wrap:       r.wrap,
	}
	if r.textLayout != nil && r.cache == current {
		r.SetSize(constraints.Constrain(textLayoutSize(r.textLayout.Size, r.align, maxWidth)))
		return
	}
	r.cache = current

	manager, _ := graphics.DefaultFontManagerErr()
	if manager == nil {
		r.textLayout = nil
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	tl, err := graphics.LayoutRichText(r.span, r.baseStyle, manager, graphics.ParagraphOptions{
		MaxWidth:  maxWidth,
		MaxLines:  r.maxLines,
		TextAlign: r.align,
	})
	if err != nil {
		r.textLayout = nil
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	r.textLayout = tl
	r.SetSize(constraints.Constrain(textLayoutSize(tl.Size, r.align, maxWidth)))
}

func (r *renderRichText) Paint(ctx *layout.PaintContext) {
	if r.textLayout == nil {
		return
	}
	ctx.Canvas.DrawText(r.textLayout, graphics.Offset{})
}

func (r *renderRichText) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
