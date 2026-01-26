package rendering

import (
	"math"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

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

// FontStyle represents normal or italic text styles.
type FontStyle int

const (
	FontStyleNormal FontStyle = iota
	FontStyleItalic
)

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
}

// FontManager manages font registration for text rendering.
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
				Op:   "rendering.DefaultFontManager",
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

// LayoutText measures and shapes the given text using the provided font manager.
func LayoutText(text string, style TextStyle, manager *FontManager) (*TextLayout, error) {
	return LayoutTextWithConstraints(text, style, manager, 0)
}

// LayoutTextWithConstraints measures and wraps text within the given width.
func LayoutTextWithConstraints(text string, style TextStyle, manager *FontManager, maxWidth float64) (*TextLayout, error) {
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
	metrics, err := skia.FontMetrics(family, size, weight, int(style.FontStyle))
	if err != nil {
		return nil, err
	}
	ascent := metrics.Ascent
	descent := metrics.Descent
	lineHeight := ascent + descent + metrics.Leading
	if lineHeight == 0 {
		lineHeight = ascent + descent
	}
	var measureErr error
	measure := func(s string) float64 {
		width, err := skia.MeasureTextWidth(s, family, size, weight, int(style.FontStyle))
		if err != nil {
			measureErr = err
			return 0
		}
		return width
	}
	lines := layoutLines(text, maxWidth, measure, style.PreserveWhitespace)
	if measureErr != nil {
		return nil, measureErr
	}
	maxLineWidth := 0.0
	for _, line := range lines {
		maxLineWidth = math.Max(maxLineWidth, line.Width)
	}
	if len(lines) == 0 {
		lines = []TextLine{{Text: "", Width: 0}}
	}
	layoutSize := Size{Width: maxLineWidth, Height: lineHeight * float64(len(lines))}
	return &TextLayout{
		Text:       text,
		Style:      style,
		Size:       layoutSize,
		Ascent:     ascent,
		Descent:    descent,
		Face:       nil,
		LineHeight: lineHeight,
		Lines:      lines,
	}, nil
}

func layoutLines(text string, maxWidth float64, measure func(string) float64, preserveWhitespace bool) []TextLine {
	if maxWidth < 0 || math.IsInf(maxWidth, 0) {
		maxWidth = 0
	}
	paragraphs := strings.Split(text, "\n")
	lines := make([]TextLine, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph == "" {
			lines = append(lines, TextLine{})
			continue
		}
		if maxWidth == 0 {
			lines = append(lines, TextLine{Text: paragraph, Width: measure(paragraph)})
			continue
		}
		for _, line := range wrapParagraph(paragraph, maxWidth, measure, preserveWhitespace) {
			lines = append(lines, TextLine{Text: line, Width: measure(line)})
		}
	}
	return lines
}

func wrapParagraph(text string, maxWidth float64, measure func(string) float64, preserveWhitespace bool) []string {
	var lines []string
	start := 0
	for start < len(text) {
		lastBreak := -1
		lastFit := -1
		for i := start; i < len(text); {
			r, size := utf8.DecodeRuneInString(text[i:])
			next := i + size
			width := measure(text[start:next])
			if width > maxWidth {
				break
			}
			lastFit = next
			if unicode.IsSpace(r) {
				lastBreak = next
			}
			i = next
		}
		if lastFit == -1 {
			_, size := utf8.DecodeRuneInString(text[start:])
			lastFit = start + size
		}
		cut := lastFit
		if lastFit < len(text) && lastBreak > start && lastBreak < lastFit {
			cut = lastBreak
		}
		line := text[start:cut]
		if !preserveWhitespace {
			line = strings.TrimRightFunc(line, unicode.IsSpace)
		}
		lines = append(lines, line)
		start = cut
		if preserveWhitespace {
			continue
		}
		for start < len(text) {
			r, size := utf8.DecodeRuneInString(text[start:])
			if !unicode.IsSpace(r) {
				break
			}
			start += size
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
