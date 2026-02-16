package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestRichText_Renders(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{
			Children: []graphics.TextSpan{
				{Text: "Hello "},
				{Text: "World"},
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.RichText]())
	if !result.Exists() {
		t.Fatal("expected RichText element to exist")
	}
	if result.RenderObject() == nil {
		t.Fatal("expected RichText render object to exist")
	}
}

func TestRichText_FindByType(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{Text: "typed"},
	})

	if !tester.Find(drifttest.ByType[widgets.RichText]()).Exists() {
		t.Error("ByType[RichText] should find the widget")
	}
}

func TestRichText_FindByText(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{
			Children: []graphics.TextSpan{
				{Text: "Hello "},
				{Text: "World"},
			},
		},
	})

	if !tester.Find(drifttest.ByText("Hello World")).Exists() {
		t.Error("ByText should match RichText by concatenated plain text")
	}
	if !tester.Find(drifttest.ByTextContaining("World")).Exists() {
		t.Error("ByTextContaining should match RichText by substring")
	}
}

func TestRichText_RenderTreeSnapshot(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{
			Children: []graphics.TextSpan{
				{Text: "Hello "},
				{Text: "World"},
			},
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/rich_text_render_tree.snapshot.json")

	node := findRenderNode(snap.RenderTree, "RenderRichText")
	if node == nil {
		t.Fatal("expected a RenderRichText node in the render tree")
	}
}

func TestRichText_DisplayOps(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{Text: "ops test"},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/rich_text_display_ops.snapshot.json")

	if len(findOps(snap.DisplayOps, "drawText")) == 0 {
		// In headless/stub builds the font manager produces no TextLayout,
		// so paint is a no-op. Verify the render tree instead.
		node := findRenderNode(snap.RenderTree, "RenderRichText")
		if node == nil {
			t.Error("expected RenderRichText node or drawText display op")
		}
	}
}

func TestRichText_WithStyle(t *testing.T) {
	original := widgets.RichText{
		Content: graphics.TextSpan{Text: "hello"},
	}
	styled := original.WithStyle(graphics.SpanStyle{
		Color:    0xFFFF0000,
		FontSize: 24,
	})
	if styled.Style.Color != 0xFFFF0000 {
		t.Errorf("expected color 0xFFFF0000, got 0x%08X", uint32(styled.Style.Color))
	}
	if styled.Style.FontSize != 24 {
		t.Errorf("expected font size 24, got %v", styled.Style.FontSize)
	}
	if original.Style != (graphics.SpanStyle{}) {
		t.Error("WithStyle mutated the original")
	}
}

func TestRichText_StyleInfluencesRenderObject(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.RichText{
		Content: graphics.Spans(
			graphics.Span("Hello "),
			graphics.Span("World").Bold(),
		),
		Style: graphics.SpanStyle{
			Color:    0xFFFF0000,
			FontSize: 24,
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/rich_text_with_style.snapshot.json")

	node := findRenderNode(snap.RenderTree, "RenderRichText")
	if node == nil {
		t.Fatal("expected a RenderRichText node in the render tree")
	}

	baseStyle, ok := node.Properties["baseStyle"].(map[string]any)
	if !ok {
		t.Fatal("expected baseStyle property in RenderRichText node")
	}

	if color, ok := baseStyle["Color"].(string); !ok || color != "0xFFFF0000" {
		t.Errorf("expected baseStyle.Color 0xFFFF0000, got %v", baseStyle["Color"])
	}
	if fontSize, ok := baseStyle["FontSize"].(float64); !ok || fontSize != 24 {
		t.Errorf("expected baseStyle.FontSize 24, got %v", baseStyle["FontSize"])
	}
}

func TestRichText_AlignCenter_ExpandsWidth(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 300, Height: 600})

	tester.PumpWidget(widgets.RichText{
		Content: graphics.TextSpan{Text: "short"},
		Wrap:    true,
		Align:   graphics.TextAlignCenter,
	})

	ro := tester.Find(drifttest.ByType[widgets.RichText]()).RenderObject()
	if ro == nil {
		t.Fatal("expected render object")
	}
	size := ro.Size()
	if size.Width != 300 {
		t.Errorf("center-aligned rich text width: expected 300, got %v", size.Width)
	}
}
