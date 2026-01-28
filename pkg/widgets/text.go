package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// Text displays a string with a single style.
//
// # Text Wrapping and Line Limits
//
// The Wrap and MaxLines fields control how text flows and truncates:
//
//   - Wrap=false (default): Text renders on a single line, extending beyond
//     the constraint width. Use for labels, buttons, and short text.
//
//   - Wrap=true: Text wraps at the constraint width, creating multiple lines.
//     Use for paragraphs, descriptions, and content that should fit a container.
//
//   - MaxLines: Limits the number of visible lines. When Wrap=true and text
//     exceeds MaxLines, it truncates. When MaxLines=0 (default), no limit applies.
//
// Common patterns:
//
//	// Single line, may overflow
//	Text{Content: "Label", Wrap: false}
//
//	// Wrapping paragraph
//	Text{Content: longText, Wrap: true}
//
//	// Preview text limited to 2 lines
//	Text{Content: description, Wrap: true, MaxLines: 2}
type Text struct {
	// Content is the text string to display.
	Content string
	// Style controls the font, size, color, and other text properties.
	Style graphics.TextStyle
	// MaxLines limits the number of visible lines (0 = unlimited).
	// Lines beyond this limit are not rendered.
	MaxLines int
	// Wrap enables text wrapping at the constraint width.
	// When false, text renders on a single line.
	Wrap bool
}

// TextOf creates a styled text widget with wrapping enabled.
// This is a convenience helper equivalent to:
//
//	Text{Content: content, Style: style, Wrap: true}
func TextOf(content string, style graphics.TextStyle) Text {
	return Text{Content: content, Style: style, Wrap: true}
}

func (t Text) CreateElement() core.Element {
	return core.NewRenderObjectElement(t, nil)
}

func (t Text) Key() any {
	return nil
}

// WithWrap returns a copy of the text with the specified wrap setting.
func (t Text) WithWrap(wrap bool) Text {
	t.Wrap = wrap
	return t
}

// WithStyle returns a copy of the text with the specified style.
func (t Text) WithStyle(style graphics.TextStyle) Text {
	t.Style = style
	return t
}

// WithMaxLines returns a copy of the text with the specified max lines limit.
func (t Text) WithMaxLines(maxLines int) Text {
	t.MaxLines = maxLines
	return t
}

func (t Text) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	text := &renderText{text: t.Content, style: t.Style, maxLines: t.MaxLines, wrap: t.Wrap}
	text.SetSelf(text)
	return text
}

func (t Text) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if text, ok := renderObject.(*renderText); ok {
		text.text = t.Content
		text.style = t.Style
		text.maxLines = t.MaxLines
		text.wrap = t.Wrap
		text.MarkNeedsLayout()
		text.MarkNeedsPaint()
	}
}

type renderText struct {
	layout.RenderBoxBase
	text     string
	style    graphics.TextStyle
	layout   *graphics.TextLayout
	maxLines int
	wrap     bool
	cache    textLayoutCache
}

type textLayoutCache struct {
	text     string
	style    graphics.TextStyle
	maxWidth float64
	maxLines int
	wrap     bool
}

func (r *renderText) PerformLayout() {
	constraints := r.Constraints()
	maxWidth := float64(0) // Default: no wrapping
	if r.wrap {
		maxWidth = constraints.MaxWidth
	}
	current := textLayoutCache{
		text:     r.text,
		style:    r.style,
		maxWidth: maxWidth,
		maxLines: r.maxLines,
		wrap:     r.wrap,
	}
	if r.layout != nil && r.cache == current {
		r.SetSize(constraints.Constrain(r.layout.Size))
		return
	}
	r.cache = current

	manager, _ := graphics.DefaultFontManagerErr()
	if manager == nil {
		// Error already reported by DefaultFontManagerErr
		r.layout = nil
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	layout, err := graphics.LayoutTextWithConstraintsAndMaxLines(r.text, r.style, manager, maxWidth, r.maxLines)
	if err != nil {
		r.layout = nil
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	r.layout = layout
	r.SetSize(constraints.Constrain(layout.Size))
}

func (r *renderText) Paint(ctx *layout.PaintContext) {
	if r.layout == nil {
		return
	}
	// NOTE: No clipping here (matches Flutter's Clip.none default) so text shadows
	// can paint outside bounds. This means all text can overflow, not just shadows.
	// If overflow becomes an issue, consider conditional clipping:
	//
	//   if r.layout.Style.Shadow == nil {
	//       ctx.Canvas.Save()
	//       size := r.Size()
	//       ctx.Canvas.ClipRect(graphics.Rect{Left: 0, Top: 0, Right: size.Width, Bottom: size.Height})
	//       defer ctx.Canvas.Restore()
	//   }
	//
	ctx.Canvas.DrawText(r.layout, graphics.Offset{})
}

func (r *renderText) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
