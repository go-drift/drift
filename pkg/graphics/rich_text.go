package graphics

import (
	"errors"
	"math"
	"runtime"
	"strings"

	"github.com/go-drift/drift/pkg/skia"
)

// explicitZero is a sentinel for explicitly setting a float64 field to zero
// when the plain zero value means "unset" or "inherit." It is small enough
// (~5e-324) to be visually indistinguishable from zero in any rendering
// context. Users should prefer the No* builder methods (e.g. NoLetterSpacing)
// rather than using this constant directly.
const explicitZero float64 = math.SmallestNonzeroFloat64

// noBackgroundColor is a sentinel Color that explicitly clears an inherited
// background. Its value (0x00000001, alpha 0) is visually indistinguishable
// from fully transparent. Users should call TextSpan.NoBackground rather than
// using this constant directly.
const noBackgroundColor Color = 1

// noDecorationColor is a sentinel Color that explicitly clears an inherited
// decoration color, causing the decoration to use the span's text color
// instead. Its value (0x00000001, alpha 0) is visually indistinguishable
// from fully transparent. Users should call TextSpan.NoDecorationColor rather
// than using this constant directly.
const noDecorationColor Color = 1

// TextDecoration selects a single text decoration line. Like [FontStyle] and
// [TextDecorationStyle], the zero value means "inherit from parent."
type TextDecoration int

const (
	textDecorationUnset TextDecoration = 0 // zero value = inherit

	// TextDecorationNone explicitly removes decoration, overriding any
	// value inherited from a parent span.
	TextDecorationNone TextDecoration = 1

	// TextDecorationUnderline draws a line below the text baseline.
	TextDecorationUnderline TextDecoration = 2

	// TextDecorationOverline draws a line above the text.
	TextDecorationOverline TextDecoration = 3

	// TextDecorationLineThrough draws a line through the middle of the text.
	TextDecorationLineThrough TextDecoration = 4
)

// decorationToSkia maps 1-based TextDecoration values to Skia's
// kNoDecoration=0, kUnderline=0x1, kOverline=0x2, kLineThrough=0x4.
var decorationToSkia = [5]int{0, 0, 1, 2, 4}

// TextDecorationStyle controls the appearance of decoration lines.
type TextDecorationStyle int

const (
	textDecorationStyleUnset TextDecorationStyle = 0 // zero value = inherit

	// TextDecorationStyleSolid draws a single continuous line.
	TextDecorationStyleSolid TextDecorationStyle = 1

	// TextDecorationStyleDouble draws two parallel lines.
	TextDecorationStyleDouble TextDecorationStyle = 2

	// TextDecorationStyleDotted draws a dotted line.
	TextDecorationStyleDotted TextDecorationStyle = 3

	// TextDecorationStyleDashed draws a dashed line.
	TextDecorationStyleDashed TextDecorationStyle = 4

	// TextDecorationStyleWavy draws a sinusoidal wave.
	TextDecorationStyleWavy TextDecorationStyle = 5
)

// SpanStyle describes the visual style for a text span. During span tree
// flattening, zero-valued fields inherit from the parent span's resolved style.
// Non-zero fields override the parent. Two patterns allow children to reset
// inherited values:
//
//   - 1-based enums ([FontStyle], [TextDecorationStyle], [TextDecoration]):
//     0 = inherit, 1+ = explicit values (e.g. [TextDecorationNone] = 1).
//   - No* builder methods ([TextSpan.NoLetterSpacing], [TextSpan.NoWordSpacing],
//     [TextSpan.NoHeight], [TextSpan.NoBackground], [TextSpan.NoDecorationColor])
//     for resetting inherited values.
type SpanStyle struct {
	Color           Color
	FontFamily      string
	FontSize        float64
	FontWeight      FontWeight
	FontStyle       FontStyle
	LetterSpacing   float64
	WordSpacing     float64
	Height          float64
	Decoration      TextDecoration
	DecorationColor Color
	DecorationStyle TextDecorationStyle
	BackgroundColor Color
}

// mergeFrom copies parent field values into s for any field that is zero-valued
// in s. Non-zero fields in s are left untouched (child overrides parent).
func (s SpanStyle) mergeFrom(parent SpanStyle) SpanStyle {
	if s.Color == 0 {
		s.Color = parent.Color
	}
	if s.FontFamily == "" {
		s.FontFamily = parent.FontFamily
	}
	if s.FontSize == 0 {
		s.FontSize = parent.FontSize
	}
	if s.FontWeight == 0 {
		s.FontWeight = parent.FontWeight
	}
	if s.FontStyle == 0 {
		s.FontStyle = parent.FontStyle
	}
	if s.LetterSpacing == 0 {
		s.LetterSpacing = parent.LetterSpacing
	}
	if s.WordSpacing == 0 {
		s.WordSpacing = parent.WordSpacing
	}
	if s.Height == 0 {
		s.Height = parent.Height
	}
	if s.Decoration == 0 {
		s.Decoration = parent.Decoration
	}
	if s.DecorationColor == 0 {
		s.DecorationColor = parent.DecorationColor
	}
	if s.DecorationStyle == 0 {
		s.DecorationStyle = parent.DecorationStyle
	}
	if s.BackgroundColor == 0 {
		s.BackgroundColor = parent.BackgroundColor
	}
	return s
}

// TextSpan represents a node in a tree of styled text. A span renders its own
// Text first, then its Children in order. Child spans inherit style fields
// from their parent for any field left at its zero value.
type TextSpan struct {
	Text     string
	Style    SpanStyle
	Children []TextSpan
}

// PlainText returns the concatenation of all text in the span tree.
func (s TextSpan) PlainText() string {
	if len(s.Children) == 0 {
		return s.Text
	}
	var b strings.Builder
	b.WriteString(s.Text)
	for _, child := range s.Children {
		b.WriteString(child.PlainText())
	}
	return b.String()
}

// Span creates a leaf TextSpan with the given text.
func Span(text string) TextSpan {
	return TextSpan{Text: text}
}

// Spans creates a container TextSpan whose children are the provided spans.
// Style methods chained on the result set defaults inherited by all children.
func Spans(children ...TextSpan) TextSpan {
	return TextSpan{Children: children}
}

// WithChildren returns a copy with the given child spans. Useful for adding
// children to a span that also carries its own text or style defaults.
func (s TextSpan) WithChildren(children ...TextSpan) TextSpan {
	s.Children = children
	return s
}

// Bold returns a copy with FontWeight set to FontWeightBold.
func (s TextSpan) Bold() TextSpan {
	s.Style.FontWeight = FontWeightBold
	return s
}

// Italic returns a copy with FontStyle set to FontStyleItalic.
func (s TextSpan) Italic() TextSpan {
	s.Style.FontStyle = FontStyleItalic
	return s
}

// Weight returns a copy with the specified font weight.
func (s TextSpan) Weight(w FontWeight) TextSpan {
	s.Style.FontWeight = w
	return s
}

// Size returns a copy with the specified font size.
func (s TextSpan) Size(size float64) TextSpan {
	s.Style.FontSize = size
	return s
}

// Color returns a copy with the specified text color.
func (s TextSpan) Color(c Color) TextSpan {
	s.Style.Color = c
	return s
}

// Family returns a copy with the specified font family.
func (s TextSpan) Family(name string) TextSpan {
	s.Style.FontFamily = name
	return s
}

// Underline returns a copy with an underline decoration. If no DecorationColor
// is set on this span or any ancestor, the decoration line uses the span's text
// color. Use DecorationColor to set an explicit color, or NoDecorationColor to
// clear an inherited one.
func (s TextSpan) Underline() TextSpan {
	s.Style.Decoration = TextDecorationUnderline
	return s
}

// Overline returns a copy with an overline decoration. If no DecorationColor
// is set on this span or any ancestor, the decoration line uses the span's text
// color. Use DecorationColor to set an explicit color, or NoDecorationColor to
// clear an inherited one.
func (s TextSpan) Overline() TextSpan {
	s.Style.Decoration = TextDecorationOverline
	return s
}

// Strikethrough returns a copy with a line-through decoration. If no
// DecorationColor is set on this span or any ancestor, the decoration line uses
// the span's text color. Use DecorationColor to set an explicit color, or
// NoDecorationColor to clear an inherited one.
func (s TextSpan) Strikethrough() TextSpan {
	s.Style.Decoration = TextDecorationLineThrough
	return s
}

// NoDecoration returns a copy with decoration explicitly set to none. This
// allows a child span to remove a decoration inherited from a parent.
func (s TextSpan) NoDecoration() TextSpan {
	s.Style.Decoration = TextDecorationNone
	return s
}

// DecorationColor returns a copy with the specified decoration line color.
func (s TextSpan) DecorationColor(c Color) TextSpan {
	s.Style.DecorationColor = c
	return s
}

// NoDecorationColor returns a copy that explicitly clears the decoration color,
// overriding any value inherited from a parent span. When the decoration color
// is unset, Skia uses the span's text color for the decoration line.
func (s TextSpan) NoDecorationColor() TextSpan {
	s.Style.DecorationColor = noDecorationColor
	return s
}

// DecorationStyle returns a copy with the specified decoration line style
// (solid, double, dotted, dashed, or wavy).
func (s TextSpan) DecorationStyle(style TextDecorationStyle) TextSpan {
	s.Style.DecorationStyle = style
	return s
}

// LetterSpacing returns a copy with the specified letter spacing.
func (s TextSpan) LetterSpacing(v float64) TextSpan {
	s.Style.LetterSpacing = v
	return s
}

// NoLetterSpacing returns a copy that explicitly resets letter spacing to zero,
// overriding any value inherited from a parent span.
func (s TextSpan) NoLetterSpacing() TextSpan {
	s.Style.LetterSpacing = explicitZero
	return s
}

// WordSpacing returns a copy with the specified word spacing.
func (s TextSpan) WordSpacing(v float64) TextSpan {
	s.Style.WordSpacing = v
	return s
}

// NoWordSpacing returns a copy that explicitly resets word spacing to zero,
// overriding any value inherited from a parent span.
func (s TextSpan) NoWordSpacing() TextSpan {
	s.Style.WordSpacing = explicitZero
	return s
}

// Height returns a copy with the specified line height multiplier.
func (s TextSpan) Height(v float64) TextSpan {
	s.Style.Height = v
	return s
}

// NoHeight returns a copy that explicitly resets the line height multiplier
// to zero, overriding any value inherited from a parent span.
func (s TextSpan) NoHeight() TextSpan {
	s.Style.Height = explicitZero
	return s
}

// Background returns a copy with the specified background color.
func (s TextSpan) Background(c Color) TextSpan {
	s.Style.BackgroundColor = c
	return s
}

// NoBackground returns a copy that explicitly clears background color,
// overriding any value inherited from a parent span.
func (s TextSpan) NoBackground() TextSpan {
	s.Style.BackgroundColor = noBackgroundColor
	return s
}

// flatSpan is a resolved text + style pair produced by flattening a TextSpan tree.
type flatSpan struct {
	text  string
	style SpanStyle
}

// flattenSpans walks a TextSpan tree depth-first, collecting leaf (text, style)
// pairs. Each child's style is merged with the parent's resolved style so that
// unset fields are inherited.
func flattenSpans(span TextSpan, baseStyle SpanStyle) []flatSpan {
	return flattenSpansInherited(span, baseStyle)
}

func flattenSpansInherited(span TextSpan, parentStyle SpanStyle) []flatSpan {
	resolved := span.Style.mergeFrom(parentStyle)
	var result []flatSpan
	if span.Text != "" {
		result = append(result, flatSpan{text: span.Text, style: resolved})
	}
	for _, child := range span.Children {
		result = append(result, flattenSpansInherited(child, resolved)...)
	}
	return result
}

// LayoutRichText measures and shapes a tree of styled text spans. The returned
// TextLayout can be rendered with Canvas.DrawText, just like single-style text.
func LayoutRichText(span TextSpan, baseStyle SpanStyle, manager *FontManager, opts ParagraphOptions) (*TextLayout, error) {
	if manager == nil {
		return nil, errors.New("font manager required")
	}
	flat := flattenSpans(span, baseStyle)
	if len(flat) == 0 {
		return nil, errors.New("rich text: no text spans")
	}

	skiaSpans := make([]skia.TextSpanData, len(flat))
	for i, f := range flat {
		s := f.style
		// Collapse explicitZero sentinel to real zero at the bridge boundary.
		letterSpacing := float32(s.LetterSpacing)
		if s.LetterSpacing == explicitZero {
			letterSpacing = 0
		}
		wordSpacing := float32(s.WordSpacing)
		if s.WordSpacing == explicitZero {
			wordSpacing = 0
		}
		height := float32(s.Height)
		if s.Height == explicitZero {
			height = 0
		}
		decoration := 0
		if int(s.Decoration) >= 0 && int(s.Decoration) < len(decorationToSkia) {
			decoration = decorationToSkia[s.Decoration]
		}
		decorationColor := uint32(s.DecorationColor)
		if s.DecorationColor == noDecorationColor {
			decorationColor = 0
		}
		skiaSpans[i] = skia.TextSpanData{
			Text:            f.text,
			Family:          s.FontFamily,
			Size:            float32(s.FontSize),
			Weight:          int(s.FontWeight),
			Style:           fontStyleBridgeValue(s.FontStyle),
			Color:           uint32(s.Color),
			Decoration:      decoration,
			DecorationColor: decorationColor,
			DecorationStyle: max(int(s.DecorationStyle)-1, 0),
			LetterSpacing:   letterSpacing,
			WordSpacing:     wordSpacing,
			Height:          height,
			HasBackground:   s.BackgroundColor != 0 && s.BackgroundColor != noBackgroundColor,
			BackgroundColor: uint32(s.BackgroundColor),
		}
	}

	maxWidth := opts.MaxWidth
	if maxWidth < 0 {
		maxWidth = 0
	}

	paragraph, err := skia.NewRichParagraph(skiaSpans, opts.MaxLines, int(opts.TextAlign))
	if err != nil {
		return nil, err
	}
	paragraph.Layout(float32(maxWidth))

	metrics, err := paragraph.Metrics()
	if err != nil {
		paragraph.Destroy()
		return nil, err
	}
	lineMetrics, err := paragraph.LineMetrics()
	if err != nil {
		paragraph.Destroy()
		return nil, err
	}

	lines := make([]TextLine, 0, len(lineMetrics.Widths))
	for _, width := range lineMetrics.Widths {
		lines = append(lines, TextLine{Text: "", Width: width})
	}
	if len(lines) == 0 {
		lines = []TextLine{{Text: "", Width: 0}}
	}

	lineHeight := 0.0
	ascent := 0.0
	descent := 0.0
	if len(lineMetrics.Heights) > 0 {
		lineHeight = lineMetrics.Heights[0]
	}
	if len(lineMetrics.Ascents) > 0 {
		ascent = lineMetrics.Ascents[0]
	}
	if len(lineMetrics.Descents) > 0 {
		descent = lineMetrics.Descents[0]
	}
	if ascent < 0 {
		ascent = -ascent
	}
	if lineHeight == 0 {
		lineHeight = ascent + descent
	}

	layoutSize := Size{Width: metrics.LongestLine, Height: metrics.Height}
	if layoutSize.Height == 0 {
		layoutSize.Height = lineHeight * float64(len(lines))
	}

	layout := &TextLayout{
		Text:       span.PlainText(),
		Size:       layoutSize,
		Ascent:     ascent,
		Descent:    descent,
		LineHeight: lineHeight,
		Lines:      lines,
		paragraph:  paragraph,
	}
	runtime.SetFinalizer(layout, func(l *TextLayout) {
		if l != nil && l.paragraph != nil {
			l.paragraph.Destroy()
			l.paragraph = nil
		}
	})
	return layout, nil
}
