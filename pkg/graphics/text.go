package graphics

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/skia"
	"golang.org/x/image/font"
)

import stderrors "errors"

const (
	// defaultFontSize is used when no font size is specified.
	defaultFontSize = 16
)

// FontWeight represents a numeric font weight.
type FontWeight int

const (
	FontWeightThin       FontWeight = 100
	FontWeightExtraLight FontWeight = 200
	FontWeightLight      FontWeight = 300
	FontWeightNormal     FontWeight = 400
	FontWeightMedium     FontWeight = 500
	FontWeightSemibold   FontWeight = 600
	FontWeightBold       FontWeight = 700
	FontWeightExtraBold  FontWeight = 800
	FontWeightBlack      FontWeight = 900
)

// String returns a human-readable representation of the font weight.
func (w FontWeight) String() string {
	switch w {
	case FontWeightThin:
		return "thin"
	case FontWeightExtraLight:
		return "extra_light"
	case FontWeightLight:
		return "light"
	case FontWeightNormal:
		return "normal"
	case FontWeightMedium:
		return "medium"
	case FontWeightSemibold:
		return "semibold"
	case FontWeightBold:
		return "bold"
	case FontWeightExtraBold:
		return "extra_bold"
	case FontWeightBlack:
		return "black"
	default:
		return fmt.Sprintf("FontWeight(%d)", int(w))
	}
}

// FontStyle represents normal or italic text styles.
type FontStyle int

const (
	fontStyleUnset FontStyle = 0 // zero value = inherit

	// FontStyleNormal selects the upright (roman) variant of the typeface.
	FontStyleNormal FontStyle = 1

	// FontStyleItalic selects the italic variant of the typeface.
	FontStyleItalic FontStyle = 2
)

// fontStyleBridgeValue converts a 1-based FontStyle to Skia's 0-based enum.
// A zero value (unset) maps to 0 (upright), preserving default behavior.
func fontStyleBridgeValue(fs FontStyle) int {
	return max(int(fs)-1, 0)
}

// String returns a human-readable representation of the font style.
func (s FontStyle) String() string {
	switch s {
	case FontStyleNormal:
		return "normal"
	case FontStyleItalic:
		return "italic"
	case fontStyleUnset:
		return "normal"
	default:
		return fmt.Sprintf("FontStyle(%d)", int(s))
	}
}

// TextAlign controls paragraph-level horizontal alignment for wrapped text.
//
// Alignment only has a visible effect when the text is laid out with a
// constrained width (i.e., when [widgets.Text].Wrap is true). Without
// wrapping there is no paragraph width to align within.
//
// Values are ordered to match Skia's textlayout::TextAlign enum so they
// can be passed through the C bridge without translation.
type TextAlign int

const (
	// TextAlignLeft aligns lines to the left edge of the paragraph.
	TextAlignLeft TextAlign = iota
	// TextAlignRight aligns lines to the right edge of the paragraph.
	TextAlignRight
	// TextAlignCenter centers each line horizontally within the paragraph.
	TextAlignCenter
	// TextAlignJustify stretches lines so both edges are flush with the
	// paragraph bounds. The last line of a paragraph is left-aligned.
	TextAlignJustify
	// TextAlignStart aligns lines to the start edge based on text direction.
	// Currently behaves like [TextAlignLeft] (LTR only).
	TextAlignStart
	// TextAlignEnd aligns lines to the end edge based on text direction.
	// Currently behaves like [TextAlignRight] (LTR only).
	TextAlignEnd
)

// String returns a human-readable representation of the text alignment.
func (a TextAlign) String() string {
	switch a {
	case TextAlignLeft:
		return "left"
	case TextAlignCenter:
		return "center"
	case TextAlignRight:
		return "right"
	case TextAlignJustify:
		return "justify"
	case TextAlignStart:
		return "start"
	case TextAlignEnd:
		return "end"
	default:
		return fmt.Sprintf("TextAlign(%d)", int(a))
	}
}

// TextStyle describes how text should be rendered.
type TextStyle struct {
	Color              Color
	Gradient           *Gradient
	FontFamily         string
	FontSize           float64
	FontWeight         FontWeight
	FontStyle          FontStyle
	PreserveWhitespace bool
	Shadow             *TextShadow
}

// WithColor returns a copy of the TextStyle with the specified color.
func (s TextStyle) WithColor(c Color) TextStyle {
	s.Color = c
	return s
}

// TextLine represents a single laid-out line of text.
type TextLine struct {
	Text  string
	Width float64
}

// TextLayout contains measured text metrics and a resolved font face.
type TextLayout struct {
	Text       string
	Style      TextStyle
	Size       Size
	Ascent     float64
	Descent    float64
	Face       font.Face
	LineHeight float64
	Lines      []TextLine
	paragraph  *skia.Paragraph
}

// FontManager manages font registration for text graphics.
type FontManager struct {
	mu          sync.RWMutex
	fonts       map[string]struct{}
	defaultName string
}

var (
	defaultFontManager     *FontManager
	defaultFontManagerErr  error
	defaultFontManagerOnce sync.Once
)

// NewFontManager creates a font manager using system defaults.
func NewFontManager() (*FontManager, error) {
	manager := &FontManager{
		fonts:       make(map[string]struct{}),
		defaultName: "",
	}
	return manager, nil
}

// DefaultFontManagerErr returns a shared font manager with a bundled font.
// It returns both the manager and any error that occurred during initialization.
func DefaultFontManagerErr() (*FontManager, error) {
	defaultFontManagerOnce.Do(func() {
		manager, err := NewFontManager()
		if err != nil {
			defaultFontManagerErr = err
			errors.Report(&errors.DriftError{
				Op:   "graphics.DefaultFontManager",
				Kind: errors.KindInit,
				Err:  err,
			})
			return
		}
		defaultFontManager = manager
	})
	return defaultFontManager, defaultFontManagerErr
}

// DefaultFontManager returns a shared font manager with a bundled font.
// For backward compatibility, returns nil on error.
func DefaultFontManager() *FontManager {
	manager, _ := DefaultFontManagerErr()
	return manager
}

// RegisterFont registers a new font family from TrueType data.
func (m *FontManager) RegisterFont(name string, data []byte) error {
	if name == "" {
		return stderrors.New("font name required")
	}
	if err := skia.RegisterFont(name, data); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fonts[name] = struct{}{}
	return nil
}

// Face resolves a font face for the given style.
// Skia-backed builds do not expose font.Face instances.
func (m *FontManager) Face(style TextStyle) (font.Face, error) {
	return nil, stderrors.New("skia backend does not expose font faces")
}

// ParagraphOptions controls paragraph-level layout behavior such as line
// wrapping, line limits, and horizontal alignment. The zero value produces
// an unconstrained, left-aligned, single-pass layout.
type ParagraphOptions struct {
	// MaxWidth is the width available for line wrapping.
	// 0 means no width constraint (text will not wrap).
	MaxWidth float64
	// MaxLines limits the number of visible lines.
	// 0 means unlimited (all lines are shown).
	MaxLines int
	// TextAlign controls horizontal alignment of lines within the paragraph.
	// The zero value ([TextAlignLeft]) aligns lines to the left edge.
	TextAlign TextAlign
}

// LayoutText measures and shapes text using the provided font manager.
// It uses default paragraph options (no wrapping, left-aligned, unlimited lines).
// For wrapping or alignment control, use [LayoutTextWithOptions].
func LayoutText(text string, style TextStyle, manager *FontManager) (*TextLayout, error) {
	return LayoutTextWithOptions(text, style, manager, ParagraphOptions{})
}

// LayoutTextWithOptions measures, wraps, and aligns text according to the
// given [ParagraphOptions]. The returned [TextLayout] contains computed
// metrics and a native paragraph handle for rendering.
func LayoutTextWithOptions(text string, style TextStyle, manager *FontManager, opts ParagraphOptions) (*TextLayout, error) {
	if manager == nil {
		return nil, stderrors.New("font manager required")
	}
	family := style.FontFamily
	if family == "" && manager.defaultName != "" {
		family = manager.defaultName
		style.FontFamily = family
	}
	size := style.FontSize
	if size <= 0 {
		size = defaultFontSize
	}
	weight := int(style.FontWeight)
	if weight < 100 {
		weight = int(FontWeightNormal)
	}
	layout, err := layoutParagraph(text, style, family, size, weight, opts)
	if err != nil {
		return nil, err
	}
	// Release the native Skia paragraph when the Go layout is garbage collected.
	runtime.SetFinalizer(layout, func(layout *TextLayout) {
		if layout != nil && layout.paragraph != nil {
			layout.paragraph.Destroy()
			layout.paragraph = nil
		}
	})
	return layout, nil
}

// layoutParagraph creates a Skia paragraph for text shaping and line breaking.
// It returns a TextLayout containing both the computed metrics and the native
// paragraph handle for later graphics.
func layoutParagraph(text string, style TextStyle, family string, size float64, weight int, opts ParagraphOptions) (*TextLayout, error) {
	maxWidth := opts.MaxWidth
	if maxWidth < 0 || math.IsInf(maxWidth, 0) {
		maxWidth = 0
	}
	maxLines := opts.MaxLines
	textAlign := opts.TextAlign

	var shadow *skia.ParagraphShadow
	if style.Shadow != nil {
		shadow = &skia.ParagraphShadow{
			Color:   uint32(style.Shadow.Color),
			OffsetX: float32(style.Shadow.Offset.X),
			OffsetY: float32(style.Shadow.Offset.Y),
			Sigma:   float32(style.Shadow.Sigma()),
		}
	}

	// For gradients, we need actual layout dimensions to resolve relative coordinates.
	// Do a two-pass approach: first layout without gradient to get size, then
	// recreate with gradient using actual bounds.
	hasGradient := style.Gradient != nil && style.Gradient.IsValid()

	// First pass: create paragraph (without gradient if we'll need a second pass)
	var gradientType int32
	var colors []uint32
	var positions []float32
	var startX, startY, endX, endY, centerX, centerY, radius float32

	paragraph, err := skia.NewParagraph(
		text,
		family,
		float32(size),
		weight,
		fontStyleBridgeValue(style.FontStyle),
		uint32(style.Color),
		maxLines,
		gradientType,
		startX, startY, endX, endY, centerX, centerY, radius,
		colors, positions,
		shadow,
		int(textAlign),
	)
	if err != nil {
		return nil, err
	}
	paragraph.Layout(float32(maxWidth))

	// If there's a gradient, get actual size and recreate paragraph with correct gradient bounds
	if hasGradient {
		metrics, err := paragraph.Metrics()
		if err != nil {
			paragraph.Destroy()
			return nil, err
		}
		actualBounds := RectFromLTWH(0, 0, metrics.LongestLine, metrics.Height)
		payload, _ := buildGradientPayload(style.Gradient, actualBounds)
		gradientType = payload.gradientType
		colors = payload.colors
		positions = payload.positions
		startX = float32(payload.start.X)
		startY = float32(payload.start.Y)
		endX = float32(payload.end.X)
		endY = float32(payload.end.Y)
		centerX = float32(payload.center.X)
		centerY = float32(payload.center.Y)
		radius = float32(payload.radius)

		// Destroy first paragraph and create new one with gradient
		paragraph.Destroy()
		paragraph, err = skia.NewParagraph(
			text,
			family,
			float32(size),
			weight,
			fontStyleBridgeValue(style.FontStyle),
			uint32(style.Color),
			maxLines,
			gradientType,
			startX, startY, endX, endY, centerX, centerY, radius,
			colors, positions,
			shadow,
			int(textAlign),
		)
		if err != nil {
			return nil, err
		}
		paragraph.Layout(float32(maxWidth))
	}
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
	// Skia reports ascent as a negative value (distance above baseline);
	// convert to positive for consistency with the rest of the layout API.
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
	// For empty text or missing paragraph metrics, fall back to font metrics
	// so that empty text widgets still reserve the correct line height.
	if layoutSize.Height == 0 {
		fallback, err := skia.FontMetrics(family, size, weight, fontStyleBridgeValue(style.FontStyle))
		if err == nil {
			fallbackLineHeight := fallback.Ascent + fallback.Descent + fallback.Leading
			if fallbackLineHeight == 0 {
				fallbackLineHeight = fallback.Ascent + fallback.Descent
			}
			if fallbackLineHeight > 0 {
				layoutSize.Height = fallbackLineHeight
				lineHeight = fallbackLineHeight
				ascent = fallback.Ascent
				descent = fallback.Descent
			}
		}
	}
	return &TextLayout{
		Text:       text,
		Style:      style,
		Size:       layoutSize,
		Ascent:     ascent,
		Descent:    descent,
		Face:       nil,
		LineHeight: lineHeight,
		Lines:      lines,
		paragraph:  paragraph,
	}, nil
}
