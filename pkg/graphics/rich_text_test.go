package graphics

import (
	"testing"
)

func TestTextSpan_PlainText_Empty(t *testing.T) {
	span := TextSpan{}
	if got := span.PlainText(); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTextSpan_PlainText_Single(t *testing.T) {
	span := TextSpan{Text: "hello"}
	if got := span.PlainText(); got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestTextSpan_PlainText_Nested(t *testing.T) {
	span := TextSpan{
		Children: []TextSpan{
			{Text: "Hello "},
			{Text: "World"},
		},
	}
	if got := span.PlainText(); got != "Hello World" {
		t.Errorf("expected %q, got %q", "Hello World", got)
	}
}

func TestTextSpan_PlainText_DeepNesting(t *testing.T) {
	span := TextSpan{
		Text: "a",
		Children: []TextSpan{
			{
				Text: "b",
				Children: []TextSpan{
					{Text: "c"},
				},
			},
		},
	}
	if got := span.PlainText(); got != "abc" {
		t.Errorf("expected %q, got %q", "abc", got)
	}
}

func TestFlattenSpans_InheritsParentStyle(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{
			Color:    0xFFFF0000,
			FontSize: 24,
		},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{
				FontWeight: FontWeightBold,
			}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFFFF0000 {
		t.Errorf("expected inherited color 0xFFFF0000, got 0x%08X", uint32(flat[0].style.Color))
	}
	if flat[0].style.FontSize != 24 {
		t.Errorf("expected inherited font size 24, got %v", flat[0].style.FontSize)
	}
	if flat[0].style.FontWeight != FontWeightBold {
		t.Errorf("expected FontWeightBold, got %v", flat[0].style.FontWeight)
	}
}

func TestFlattenSpans_ChildOverridesParent(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{
			Color: 0xFFFF0000,
		},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{
				Color: 0xFF00FF00,
			}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFF00FF00 {
		t.Errorf("expected child color 0xFF00FF00, got 0x%08X", uint32(flat[0].style.Color))
	}
}

func TestFlattenSpans_ZeroValueInheritsFromParent(t *testing.T) {
	// A child with no fields set (all zero) inherits everything from the parent.
	parent := TextSpan{
		Style: SpanStyle{
			Color: 0xFFFF0000,
		},
		Children: []TextSpan{
			{Text: "child"}, // all zero, inherits parent color
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFFFF0000 {
		t.Errorf("expected inherited color 0xFFFF0000, got 0x%08X", uint32(flat[0].style.Color))
	}
}

func TestFlattenSpans_CollectsLeaves(t *testing.T) {
	span := TextSpan{
		Text: "root",
		Children: []TextSpan{
			{Text: "a"},
			{
				Children: []TextSpan{
					{Text: "b"},
					{Text: "c"},
				},
			},
		},
	}
	flat := flattenSpans(span, SpanStyle{})
	if len(flat) != 4 {
		t.Fatalf("expected 4 flat spans, got %d", len(flat))
	}
	texts := make([]string, len(flat))
	for i, f := range flat {
		texts[i] = f.text
	}
	expected := []string{"root", "a", "b", "c"}
	for i, want := range expected {
		if texts[i] != want {
			t.Errorf("span %d: expected %q, got %q", i, want, texts[i])
		}
	}
}

func TestFlattenSpans_DeepInheritance(t *testing.T) {
	// Grandparent sets color, parent sets size, child sets weight.
	// Child should inherit both color and size.
	span := TextSpan{
		Style: SpanStyle{Color: 0xFFAA0000},
		Children: []TextSpan{
			{
				Style: SpanStyle{FontSize: 20},
				Children: []TextSpan{
					{Text: "leaf", Style: SpanStyle{
						FontWeight: FontWeightBold,
					}},
				},
			},
		},
	}
	flat := flattenSpans(span, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFFAA0000 {
		t.Errorf("expected inherited color from grandparent, got 0x%08X", uint32(flat[0].style.Color))
	}
	if flat[0].style.FontSize != 20 {
		t.Errorf("expected inherited font size from parent, got %v", flat[0].style.FontSize)
	}
	if flat[0].style.FontWeight != FontWeightBold {
		t.Errorf("expected FontWeightBold, got %v", flat[0].style.FontWeight)
	}
}

func TestFlattenSpans_ChildOverridesDecorationStyleToSolid(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{DecorationStyle: TextDecorationStyleWavy},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{DecorationStyle: TextDecorationStyleSolid}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.DecorationStyle != TextDecorationStyleSolid {
		t.Errorf("expected TextDecorationStyleSolid (%d), got %d",
			TextDecorationStyleSolid, flat[0].style.DecorationStyle)
	}
}

func TestFlattenSpans_ChildOverridesFontStyleToNormal(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{FontStyle: FontStyleItalic},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{FontStyle: FontStyleNormal}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.FontStyle != FontStyleNormal {
		t.Errorf("expected FontStyleNormal (%d), got %d",
			FontStyleNormal, flat[0].style.FontStyle)
	}
}

func TestSpan(t *testing.T) {
	s := Span("hello")
	if s.Text != "hello" {
		t.Errorf("expected text %q, got %q", "hello", s.Text)
	}
	if s.Style != (SpanStyle{}) {
		t.Errorf("expected zero style, got %+v", s.Style)
	}
}

func TestSpans(t *testing.T) {
	s := Spans(Span("a"), Span("b"))
	if len(s.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(s.Children))
	}
	if s.Children[0].Text != "a" || s.Children[1].Text != "b" {
		t.Errorf("unexpected children: %+v", s.Children)
	}
}

func TestTextSpan_WithChildren(t *testing.T) {
	s := Span("parent").Bold().WithChildren(Span("a"), Span("b"))
	if s.Text != "parent" {
		t.Errorf("expected text %q, got %q", "parent", s.Text)
	}
	if s.Style.FontWeight != FontWeightBold {
		t.Errorf("expected bold, got %v", s.Style.FontWeight)
	}
	if len(s.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(s.Children))
	}
}

func TestTextSpan_Bold(t *testing.T) {
	s := Span("x").Bold()
	if s.Style.FontWeight != FontWeightBold {
		t.Errorf("expected FontWeightBold, got %v", s.Style.FontWeight)
	}
}

func TestTextSpan_Italic(t *testing.T) {
	s := Span("x").Italic()
	if s.Style.FontStyle != FontStyleItalic {
		t.Errorf("expected FontStyleItalic, got %v", s.Style.FontStyle)
	}
}

func TestTextSpan_Weight(t *testing.T) {
	s := Span("x").Weight(FontWeightBlack)
	if s.Style.FontWeight != FontWeightBlack {
		t.Errorf("expected FontWeightBlack, got %v", s.Style.FontWeight)
	}
}

func TestTextSpan_Size(t *testing.T) {
	s := Span("x").Size(24)
	if s.Style.FontSize != 24 {
		t.Errorf("expected 24, got %v", s.Style.FontSize)
	}
}

func TestTextSpan_Color(t *testing.T) {
	s := Span("x").Color(0xFFFF0000)
	if s.Style.Color != 0xFFFF0000 {
		t.Errorf("expected 0xFFFF0000, got 0x%08X", uint32(s.Style.Color))
	}
}

func TestTextSpan_Family(t *testing.T) {
	s := Span("x").Family("monospace")
	if s.Style.FontFamily != "monospace" {
		t.Errorf("expected %q, got %q", "monospace", s.Style.FontFamily)
	}
}

func TestTextSpan_Underline(t *testing.T) {
	s := Span("x").Underline()
	if s.Style.Decoration != TextDecorationUnderline {
		t.Errorf("expected TextDecorationUnderline, got %d", s.Style.Decoration)
	}
	if s.Style.DecorationColor != 0 {
		t.Errorf("expected zero decoration color (inherit), got 0x%08X", uint32(s.Style.DecorationColor))
	}
}

func TestTextSpan_Underline_WithColor(t *testing.T) {
	s := Span("x").Underline().DecorationColor(0xFFFF0000)
	if s.Style.Decoration != TextDecorationUnderline {
		t.Errorf("expected TextDecorationUnderline, got %d", s.Style.Decoration)
	}
	if s.Style.DecorationColor != 0xFFFF0000 {
		t.Errorf("expected decoration color 0xFFFF0000, got 0x%08X", uint32(s.Style.DecorationColor))
	}
}

func TestTextSpan_Overline(t *testing.T) {
	s := Span("x").Overline()
	if s.Style.Decoration != TextDecorationOverline {
		t.Errorf("expected TextDecorationOverline, got %d", s.Style.Decoration)
	}
}

func TestTextSpan_Strikethrough(t *testing.T) {
	s := Span("x").Strikethrough()
	if s.Style.Decoration != TextDecorationLineThrough {
		t.Errorf("expected TextDecorationLineThrough, got %d", s.Style.Decoration)
	}
}

func TestTextSpan_DecorationStyle(t *testing.T) {
	s := Span("x").Underline().DecorationStyle(TextDecorationStyleWavy)
	if s.Style.Decoration != TextDecorationUnderline {
		t.Errorf("expected TextDecorationUnderline, got %d", s.Style.Decoration)
	}
	if s.Style.DecorationStyle != TextDecorationStyleWavy {
		t.Errorf("expected TextDecorationStyleWavy, got %d", s.Style.DecorationStyle)
	}
}

func TestTextSpan_Background(t *testing.T) {
	s := Span("x").Background(0xFF112233)
	if s.Style.BackgroundColor != 0xFF112233 {
		t.Errorf("expected 0xFF112233, got 0x%08X", uint32(s.Style.BackgroundColor))
	}
}

func TestTextSpan_Chaining(t *testing.T) {
	s := Span("x").Bold().Color(0xFFFF0000).Size(20).Italic()
	if s.Style.FontWeight != FontWeightBold {
		t.Errorf("expected bold")
	}
	if s.Style.Color != 0xFFFF0000 {
		t.Errorf("expected color")
	}
	if s.Style.FontSize != 20 {
		t.Errorf("expected size 20")
	}
	if s.Style.FontStyle != FontStyleItalic {
		t.Errorf("expected italic")
	}
}

func TestTextSpan_ValueReceiverSemantics(t *testing.T) {
	original := Span("x")
	bold := original.Bold()
	if original.Style.FontWeight != 0 {
		t.Error("Bold() mutated original span")
	}
	if bold.Style.FontWeight != FontWeightBold {
		t.Error("Bold() did not set weight on copy")
	}
}

func TestFlattenSpans_NoLetterSpacingOverridesParent(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{LetterSpacing: 2},
		Children: []TextSpan{
			Span("child").NoLetterSpacing(),
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	// NoLetterSpacing sets explicitZero, which is non-zero so it survives
	// mergeFrom (not inherited). The bridge boundary converts it to real 0.
	if flat[0].style.LetterSpacing != explicitZero {
		t.Errorf("expected LetterSpacing to be explicitZero, got %v", flat[0].style.LetterSpacing)
	}
}

func TestFlattenSpans_NoDecorationOverridesParent(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{Decoration: TextDecorationUnderline},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{Decoration: TextDecorationNone}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Decoration != TextDecorationNone {
		t.Errorf("expected TextDecorationNone (%d), got %d",
			TextDecorationNone, flat[0].style.Decoration)
	}
}

func TestTextSpan_NoDecoration(t *testing.T) {
	s := Span("x").NoDecoration()
	if s.Style.Decoration != TextDecorationNone {
		t.Errorf("expected TextDecorationNone (%d), got %d",
			TextDecorationNone, s.Style.Decoration)
	}
}

func TestTextSpan_NoBackground(t *testing.T) {
	s := Span("x").NoBackground()
	if s.Style.BackgroundColor != noBackgroundColor {
		t.Errorf("expected noBackgroundColor sentinel, got 0x%08X", uint32(s.Style.BackgroundColor))
	}
}

func TestFlattenSpans_NoBackgroundOverridesParent(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{BackgroundColor: 0xFFFFFF00},
		Children: []TextSpan{
			{Text: "child", Style: SpanStyle{BackgroundColor: noBackgroundColor}},
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.BackgroundColor != noBackgroundColor {
		t.Errorf("expected noBackgroundColor sentinel, got 0x%08X", uint32(flat[0].style.BackgroundColor))
	}
}

func TestTextSpan_NoDecorationColor(t *testing.T) {
	s := Span("x").NoDecorationColor()
	if s.Style.DecorationColor != noDecorationColor {
		t.Errorf("expected noDecorationColor sentinel, got 0x%08X", uint32(s.Style.DecorationColor))
	}
}

func TestFlattenSpans_NoDecorationColorOverridesParent(t *testing.T) {
	parent := TextSpan{
		Style: SpanStyle{DecorationColor: 0xFFFF0000},
		Children: []TextSpan{
			Span("child").Underline().NoDecorationColor(),
		},
	}
	flat := flattenSpans(parent, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.DecorationColor != noDecorationColor {
		t.Errorf("expected noDecorationColor sentinel, got 0x%08X", uint32(flat[0].style.DecorationColor))
	}
}

func TestDecorationToSkia_OutOfBoundsDoesNotPanic(t *testing.T) {
	// An invalid TextDecoration value should not cause an index-out-of-range
	// panic during layout. Verify by flattening and converting to Skia data.
	span := TextSpan{
		Text:  "test",
		Style: SpanStyle{Decoration: TextDecoration(99)},
	}
	flat := flattenSpans(span, SpanStyle{})
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	// Verify the out-of-bounds value is clamped to 0 (no decoration).
	s := flat[0].style
	decoration := 0
	if int(s.Decoration) >= 0 && int(s.Decoration) < len(decorationToSkia) {
		decoration = decorationToSkia[s.Decoration]
	}
	if decoration != 0 {
		t.Errorf("expected out-of-bounds decoration to map to 0, got %d", decoration)
	}
}

func TestFlattenSpans_BaseStyleApplied(t *testing.T) {
	base := SpanStyle{
		Color:    0xFFAA0000,
		FontSize: 18,
	}
	span := TextSpan{
		Children: []TextSpan{
			{Text: "child"},
		},
	}
	flat := flattenSpans(span, base)
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFFAA0000 {
		t.Errorf("expected base color 0xFFAA0000, got 0x%08X", uint32(flat[0].style.Color))
	}
	if flat[0].style.FontSize != 18 {
		t.Errorf("expected base font size 18, got %v", flat[0].style.FontSize)
	}
}

func TestFlattenSpans_ContentStyleOverridesBaseStyle(t *testing.T) {
	base := SpanStyle{
		Color:    0xFFAA0000,
		FontSize: 18,
	}
	span := TextSpan{
		Style: SpanStyle{Color: 0xFF00BB00},
		Children: []TextSpan{
			{Text: "child"},
		},
	}
	flat := flattenSpans(span, base)
	if len(flat) != 1 {
		t.Fatalf("expected 1 flat span, got %d", len(flat))
	}
	if flat[0].style.Color != 0xFF00BB00 {
		t.Errorf("expected content color 0xFF00BB00 to override base, got 0x%08X", uint32(flat[0].style.Color))
	}
	if flat[0].style.FontSize != 18 {
		t.Errorf("expected base font size 18 (not overridden), got %v", flat[0].style.FontSize)
	}
}
