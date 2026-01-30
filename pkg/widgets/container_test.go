package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestContainer_Color(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 50})

	tester.PumpWidget(widgets.Container{
		Color:  graphics.RGB(255, 0, 0),
		Width:  100,
		Height: 50,
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_color.snapshot.json")

	rects := findOps(snap.DisplayOps, "drawRect")
	found := false
	for _, op := range rects {
		if c, ok := op.Params["color"].(string); ok && c == "0xFFFF0000" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a drawRect op with color 0xFFFF0000")
	}
}

func TestContainer_Gradient(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 50})

	tester.PumpWidget(widgets.Container{
		Width:  100,
		Height: 50,
		Gradient: graphics.NewLinearGradient(
			graphics.AlignTopLeft,
			graphics.AlignBottomRight,
			[]graphics.GradientStop{
				{Position: 0.0, Color: graphics.RGB(66, 133, 244)},
				{Position: 1.0, Color: graphics.RGB(15, 157, 88)},
			},
		),
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_gradient.snapshot.json")

	if len(findOps(snap.DisplayOps, "drawRect")) == 0 {
		t.Error("expected at least one drawRect op for gradient container")
	}
}

func TestContainer_Shadow(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 50})

	tester.PumpWidget(widgets.Container{
		Width:  100,
		Height: 50,
		Color:  graphics.RGB(200, 200, 200),
		Shadow: &graphics.BoxShadow{
			Color:      graphics.RGBA(0, 0, 0, 0.25),
			BlurRadius: 8,
			Offset:     graphics.Offset{X: 0, Y: 4},
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_shadow.snapshot.json")

	if len(findOps(snap.DisplayOps, "drawRectShadow")) == 0 {
		t.Error("expected at least one drawRectShadow op")
	}
}

func TestContainer_PaintOrder(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 100})

	// Use a colored Container child so we get a second drawRect op
	// (Text produces no drawText in headless/stub builds).
	tester.PumpWidget(widgets.Container{
		Width:  200,
		Height: 100,
		Color:  graphics.RGB(100, 100, 100),
		Shadow: &graphics.BoxShadow{
			Color:      graphics.RGBA(0, 0, 0, 0.25),
			BlurRadius: 4,
		},
		ChildWidget: widgets.Container{
			Width:  50,
			Height: 50,
			Color:  graphics.RGB(200, 0, 0),
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_paint_order.snapshot.json")

	ops := snap.DisplayOps

	shadowIdx := findOpIndex(ops, "drawRectShadow")
	if shadowIdx < 0 {
		t.Fatal("expected drawRectShadow op")
	}

	// Find the background drawRect (parent container color) and the child
	// drawRect (child container color). The background should come first.
	var bgIdx, childIdx int = -1, -1
	for i, op := range ops {
		if op.Op == "drawRect" {
			if bgIdx < 0 {
				bgIdx = i
			} else {
				childIdx = i
			}
		}
	}
	if bgIdx < 0 {
		t.Fatal("expected at least one drawRect op for background")
	}
	if childIdx < 0 {
		t.Fatal("expected a second drawRect op for child container")
	}
	if shadowIdx >= bgIdx {
		t.Errorf("shadow (index %d) should paint before background (index %d)", shadowIdx, bgIdx)
	}
	if bgIdx >= childIdx {
		t.Errorf("background (index %d) should paint before child (index %d)", bgIdx, childIdx)
	}
}

func TestContainer_FixedSize(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Wrap in Center to give the Container loose constraints.
	tester.PumpWidget(widgets.Center{
		ChildWidget: widgets.Container{
			Width:  120,
			Height: 80,
		},
	})

	result := tester.Find(drifttest.ByType[widgets.Container]())
	if !result.Exists() {
		t.Fatal("expected Container element to exist")
	}
	size := result.RenderObject().Size()
	if size.Width != 120 || size.Height != 80 {
		t.Errorf("expected size {120, 80}, got {%v, %v}", size.Width, size.Height)
	}
}

func TestContainer_Padding(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Container{
		Padding:     layout.EdgeInsetsAll(16),
		Width:       200,
		Height:      100,
		ChildWidget: widgets.Text{Content: "inside"},
	})

	result := tester.Find(drifttest.ByType[widgets.Text]())
	if !result.Exists() {
		t.Fatal("expected Text element to exist")
	}
	pd, ok := result.RenderObject().ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	if pd.Offset.X < 16 || pd.Offset.Y < 16 {
		t.Errorf("expected child offset >= {16, 16}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestContainer_OverflowClip(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 50})

	tester.PumpWidget(widgets.Container{
		Width:  100,
		Height: 50,
		Gradient: graphics.NewLinearGradient(
			graphics.AlignTopLeft,
			graphics.AlignBottomRight,
			[]graphics.GradientStop{
				{Position: 0.0, Color: graphics.RGB(66, 133, 244)},
				{Position: 1.0, Color: graphics.RGB(15, 157, 88)},
			},
		),
		Overflow: widgets.OverflowClip,
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_overflow_clip.snapshot.json")

	ops := snap.DisplayOps

	saveIdx := findOpIndex(ops, "save")
	clipIdx := findOpIndex(ops, "clipRect")
	drawIdx := findOpIndex(ops, "drawRect")
	restoreIdx := findOpIndex(ops, "restore")

	if saveIdx < 0 {
		t.Fatal("expected save op")
	}
	if clipIdx < 0 {
		t.Fatal("expected clipRect op")
	}
	if drawIdx < 0 {
		t.Fatal("expected drawRect op")
	}
	if restoreIdx < 0 {
		t.Fatal("expected restore op")
	}
	if !(saveIdx < clipIdx && clipIdx < drawIdx && drawIdx < restoreIdx) {
		t.Errorf("expected save(%d) < clipRect(%d) < drawRect(%d) < restore(%d)",
			saveIdx, clipIdx, drawIdx, restoreIdx)
	}
}
