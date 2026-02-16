package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Text displays a string with a single style.
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Text{
//	    Content: "Hello, Drift",
//	    Style:   graphics.TextStyle{Color: colors.OnSurface, FontSize: 16},
//	}
//
// Using text styles from theme:
//
//	_, _, textTheme := theme.UseTheme(ctx)
//	widgets.Text{Content: "Title", Style: textTheme.HeadlineLarge}
//
// Themed (using [theme.TextOf]):
//
//	theme.TextOf(ctx, "Welcome", textTheme.HeadlineMedium)
//
// # Text Wrapping, Line Limits, and Alignment
//
// The Wrap, MaxLines, and Align fields control how text flows, truncates,
// and aligns:
//
//   - Wrap=TextWrapWrap (default zero value): Text wraps at the constraint
//     width, creating multiple lines. Use for paragraphs, descriptions, and
//     content that should fit a container.
//
//   - Wrap=TextWrapNoWrap: Text renders on a single line, extending beyond
//     the constraint width. Use for labels, buttons, and short text.
//
//   - MaxLines: Limits the number of visible lines. When text wraps and
//     exceeds MaxLines, it truncates. When MaxLines=0 (default), no limit applies.
//
//   - Align: Controls horizontal alignment of lines within the paragraph.
//     Alignment only takes effect when text wraps, because unwrapped text
//     has no paragraph width to align within. Use [Text.WithAlign] for chaining.
//
// Common patterns:
//
//	// Wrapping paragraph (default)
//	Text{Content: longText}
//
//	// Single line, may overflow
//	Text{Content: "Label", Wrap: graphics.TextWrapNoWrap}
//
//	// Preview text limited to 2 lines
//	Text{Content: description, MaxLines: 2}
//
//	// Centered wrapping text
//	Text{Content: longText, Align: graphics.TextAlignCenter}
type Text struct {
	// Content is the text string to display.
	Content string
	// Style controls the font, size, color, and other text properties.
	Style graphics.TextStyle
	// Align controls paragraph-level horizontal text alignment.
	// Zero value is left-aligned. Only takes effect when text wraps;
	// unwrapped text has no paragraph width to align within.
	Align graphics.TextAlign
	// MaxLines limits the number of visible lines (0 = unlimited).
	// Lines beyond this limit are not rendered.
	MaxLines int
	// Wrap controls text wrapping behavior. The zero value
	// ([graphics.TextWrapWrap]) wraps text at the constraint width.
	// Set to [graphics.TextWrapNoWrap] for single-line text.
	Wrap graphics.TextWrap
}

func (t Text) CreateElement() core.Element {
	return core.NewRenderObjectElement(t, nil)
}

func (t Text) Key() any {
	return nil
}

// WithWrap returns a copy of the text with the specified wrap mode.
func (t Text) WithWrap(wrap graphics.TextWrap) Text {
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

// WithAlign returns a copy of the text with the specified alignment.
// Alignment only takes effect when text wraps. See [graphics.TextAlign]
// for the available alignment options.
func (t Text) WithAlign(align graphics.TextAlign) Text {
	t.Align = align
	return t
}

func (t Text) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	text := &renderText{text: t.Content, style: t.Style, align: t.Align, maxLines: t.MaxLines, wrapMode: t.Wrap}
	text.SetSelf(text)
	return text
}

func (t Text) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if text, ok := renderObject.(*renderText); ok {
		text.text = t.Content
		text.style = t.Style
		text.align = t.Align
		text.maxLines = t.MaxLines
		text.wrapMode = t.Wrap
		text.MarkNeedsLayout()
		text.MarkNeedsPaint()
	}
}

type renderText struct {
	layout.RenderBoxBase
	text     string
	style    graphics.TextStyle
	align    graphics.TextAlign
	layout   *graphics.TextLayout
	maxLines int
	wrapMode graphics.TextWrap
	cache    textLayoutCache
}

type textLayoutCache struct {
	text     string
	style    graphics.TextStyle
	align    graphics.TextAlign
	maxWidth float64
	maxLines int
	wrapMode graphics.TextWrap
}

// textLayoutSize returns the widget size for a laid-out paragraph. When text
// alignment is non-left, Skia positions lines within the full paragraph layout
// width, so the widget must claim that width for its bounds to agree with the
// rendered text positions.
func textLayoutSize(layoutSize graphics.Size, align graphics.TextAlign, maxWidth float64) graphics.Size {
	switch align {
	case graphics.TextAlignLeft, graphics.TextAlignStart:
		// Left-flush alignments: use the tight (longest-line) width.
		// TextAlignStart resolves to left in LTR; if RTL support is added,
		// this case will need to check the text direction.
	default:
		if maxWidth > 0 {
			layoutSize.Width = maxWidth
		}
	}
	return layoutSize
}

func (r *renderText) PerformLayout() {
	constraints := r.Constraints()
	maxWidth := constraints.MaxWidth // Default: wrap
	if r.wrapMode == graphics.TextWrapNoWrap {
		maxWidth = 0
	}
	current := textLayoutCache{
		text:     r.text,
		style:    r.style,
		align:    r.align,
		maxWidth: maxWidth,
		maxLines: r.maxLines,
		wrapMode: r.wrapMode,
	}
	if r.layout != nil && r.cache == current {
		r.SetSize(constraints.Constrain(textLayoutSize(r.layout.Size, r.align, maxWidth)))
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

	layout, err := graphics.LayoutTextWithOptions(r.text, r.style, manager, graphics.ParagraphOptions{
		MaxWidth:  maxWidth,
		MaxLines:  r.maxLines,
		TextAlign: r.align,
	})
	if err != nil {
		r.layout = nil
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	r.layout = layout
	r.SetSize(constraints.Constrain(textLayoutSize(layout.Size, r.align, maxWidth)))
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
