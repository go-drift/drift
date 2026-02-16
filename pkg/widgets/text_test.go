package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestText_Renders(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "hello"})

	result := tester.Find(drifttest.ByType[widgets.Text]())
	if !result.Exists() {
		t.Fatal("expected Text element to exist")
	}
	if result.RenderObject() == nil {
		t.Fatal("expected Text render object to exist")
	}
}

func TestText_FindByContent(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "drift"})

	if !tester.Find(drifttest.ByText("drift")).Exists() {
		t.Error("ByText(\"drift\") should find the Text widget")
	}
	if tester.Find(drifttest.ByText("other")).Exists() {
		t.Error("ByText(\"other\") should not find anything")
	}
}

func TestText_WidgetContent(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "count: 42"})

	result := tester.Find(drifttest.ByType[widgets.Text]())
	txt := result.Widget().(widgets.Text)
	if txt.Content != "count: 42" {
		t.Errorf("expected Content %q, got %q", "count: 42", txt.Content)
	}
}

func TestText_RenderTreeSnapshot(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "hello"})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/text_render_tree.snapshot.json")

	node := findRenderNode(snap.RenderTree, "RenderText")
	if node == nil {
		t.Fatal("expected a RenderText node in the render tree")
	}
	if got, ok := node.Properties["text"]; !ok || got != "hello" {
		t.Errorf("expected text property %q, got %v", "hello", got)
	}
}

func TestText_DisplayOps_DrawText(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "hello"})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/text_display_ops.snapshot.json")

	if len(findOps(snap.DisplayOps, "drawText")) == 0 {
		// In headless/stub builds the font manager produces no TextLayout,
		// so renderText.Paint is a no-op. Verify the render tree instead.
		node := findRenderNode(snap.RenderTree, "RenderText")
		if node == nil {
			t.Error("expected RenderText node or drawText display op")
		}
	}
}

func TestText_AlignDefault(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "hello"})

	txt := tester.Find(drifttest.ByType[widgets.Text]()).Widget().(widgets.Text)
	if txt.Align != graphics.TextAlignLeft {
		t.Errorf("expected default Align %v, got %v", graphics.TextAlignLeft, txt.Align)
	}
}

func TestText_AlignCenter(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Text{Content: "centered", Align: graphics.TextAlignCenter})

	txt := tester.Find(drifttest.ByType[widgets.Text]()).Widget().(widgets.Text)
	if txt.Align != graphics.TextAlignCenter {
		t.Errorf("expected Align %v, got %v", graphics.TextAlignCenter, txt.Align)
	}
}

func TestText_WithAlign(t *testing.T) {
	base := widgets.Text{Content: "hello"}
	centered := base.WithAlign(graphics.TextAlignCenter)

	if centered.Align != graphics.TextAlignCenter {
		t.Errorf("WithAlign: expected %v, got %v", graphics.TextAlignCenter, centered.Align)
	}
	if base.Align != graphics.TextAlignLeft {
		t.Errorf("WithAlign should not mutate receiver: expected %v, got %v", graphics.TextAlignLeft, base.Align)
	}
}

func TestText_AlignCenter_ExpandsWidth(t *testing.T) {
	// Center-aligned wrapping text should expand to the full constraint
	// width so Skia's centering within the paragraph layout width matches
	// the widget bounds.
	//
	// The test harness uses tight constraints (min=max), so this verifies
	// the widget-level textLayoutSize helper rather than exercising the
	// distinction between tight and longest-line width (which requires a
	// real Skia backend with loose constraints).
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 300, Height: 600})

	tester.PumpWidget(widgets.Text{Content: "short", Align: graphics.TextAlignCenter})

	ro := tester.Find(drifttest.ByType[widgets.Text]()).RenderObject()
	if ro == nil {
		t.Fatal("expected render object")
	}
	size := ro.Size()
	if size.Width != 300 {
		t.Errorf("center-aligned text width: expected 300, got %v", size.Width)
	}
}

// findRenderNode walks the render tree and returns the first node with the given type.
func findRenderNode(node *drifttest.RenderNode, typeName string) *drifttest.RenderNode {
	if node == nil {
		return nil
	}
	if node.Type == typeName {
		return node
	}
	for _, child := range node.Children {
		if found := findRenderNode(child, typeName); found != nil {
			return found
		}
	}
	return nil
}

func findOps(ops []drifttest.DisplayOp, name string) []drifttest.DisplayOp {
	var result []drifttest.DisplayOp
	for _, op := range ops {
		if op.Op == name {
			result = append(result, op)
		}
	}
	return result
}

func findOpIndex(ops []drifttest.DisplayOp, name string) int {
	for i, op := range ops {
		if op.Op == name {
			return i
		}
	}
	return -1
}
