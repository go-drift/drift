package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

// getRenderObject extracts the render object from an element if available.
func getRenderObject(e core.Element) layout.RenderObject {
	if ro, ok := e.(interface{ RenderObject() layout.RenderObject }); ok {
		return ro.RenderObject()
	}
	return nil
}

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
		Child: widgets.Container{
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
		Child: widgets.Container{
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
		Padding: layout.EdgeInsetsAll(16),
		Width:   200,
		Height:  100,
		Child:   widgets.Text{Content: "inside"},
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

func TestContainer_AlignmentWithSmallerChild(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Container with explicit size and a smaller child should center the child.
	// Wrap in Center to give the outer Container loose constraints.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Width:     100,
			Height:    100,
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  40,
				Height: 40,
				Color:  graphics.RGB(255, 0, 0),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	outerRO := getRenderObject(finder.At(0))
	innerRO := getRenderObject(finder.At(1))

	// Verify sizes.
	if outerRO.Size().Width != 100 || outerRO.Size().Height != 100 {
		t.Errorf("expected outer container size {100, 100}, got {%v, %v}", outerRO.Size().Width, outerRO.Size().Height)
	}
	if innerRO.Size().Width != 40 || innerRO.Size().Height != 40 {
		t.Errorf("expected inner container size {40, 40}, got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	// Child (40x40) centered in 100x100 should be at offset (30, 30).
	if pd.Offset.X != 30 || pd.Offset.Y != 30 {
		t.Errorf("expected child offset {30, 30}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestContainer_AlignmentWithTightConstraints(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Without Center, container receives tight constraints and expands to fill.
	// Container wants 100x100 but is forced to 200x200 by tight parent constraints.
	// Child should still be able to be smaller and align within the expanded space.
	tester.PumpWidget(widgets.Container{
		Width:     100,
		Height:    100,
		Alignment: layout.AlignmentCenter,
		Child: widgets.Container{
			Width:  40,
			Height: 40,
			Color:  graphics.RGB(255, 0, 0),
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	outerRO := getRenderObject(finder.At(0))
	innerRO := getRenderObject(finder.At(1))

	// Verify outer container was forced to 200x200 by tight constraints.
	if outerRO.Size().Width != 200 || outerRO.Size().Height != 200 {
		t.Errorf("expected outer container forced to {200, 200}, got {%v, %v}", outerRO.Size().Width, outerRO.Size().Height)
	}
	// Verify inner container kept its requested size.
	if innerRO.Size().Width != 40 || innerRO.Size().Height != 40 {
		t.Errorf("expected inner container size {40, 40}, got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	// Child (40x40) centered in 200x200 (expanded container) should be at offset (80, 80).
	if pd.Offset.X != 80 || pd.Offset.Y != 80 {
		t.Errorf("expected child offset {80, 80}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestContainer_LooseWidthOnly(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Container with only Width set; child should be capped at that width
	// but height remains unconstrained.
	// Wrap in Center to give the outer Container loose constraints.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Width:     100,
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  40,
				Height: 60,
				Color:  graphics.RGB(0, 255, 0),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// The child should keep its natural size.
	if innerRO.Size().Width != 40 || innerRO.Size().Height != 60 {
		t.Errorf("expected inner container size {40, 60}, got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	// Verify alignment works on the width axis (centered horizontally).
	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	// Child (40 wide) centered in 100 wide container: (100-40)/2 = 30
	// Height is unconstrained, so child fills parent height (60), offset Y = 0
	if pd.Offset.X != 30 {
		t.Errorf("expected child X offset 30, got %v", pd.Offset.X)
	}
}

func TestContainer_LooseWidthOnly_MaxConstraint(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Child wants 150 width but container only allows 100.
	// Child should be clamped to 100.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Width:     100,
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  150, // Wants more than available
				Height: 40,
				Color:  graphics.RGB(0, 255, 0),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// Child width should be clamped to 100, height stays at 40.
	if innerRO.Size().Width != 100 || innerRO.Size().Height != 40 {
		t.Errorf("expected inner container size {100, 40} (width clamped), got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}
}

func TestContainer_LooseHeightOnly(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Container with only Height set; child should be capped at that height
	// but width remains unconstrained.
	// Wrap in Center to give the outer Container loose constraints.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Height:    100,
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  60,
				Height: 40,
				Color:  graphics.RGB(0, 0, 255),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// The child should keep its natural size.
	if innerRO.Size().Width != 60 || innerRO.Size().Height != 40 {
		t.Errorf("expected inner container size {60, 40}, got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	// Verify alignment works on the height axis (centered vertically).
	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	// Child (40 tall) centered in 100 tall container: (100-40)/2 = 30
	// Width is unconstrained, so child fills parent width (60), offset X = 0
	if pd.Offset.Y != 30 {
		t.Errorf("expected child Y offset 30, got %v", pd.Offset.Y)
	}
}

func TestContainer_LooseHeightOnly_MaxConstraint(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Child wants 150 height but container only allows 100.
	// Child should be clamped to 100.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Height:    100,
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  40,
				Height: 150, // Wants more than available
				Color:  graphics.RGB(0, 0, 255),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// Child height should be clamped to 100, width stays at 40.
	if innerRO.Size().Width != 40 || innerRO.Size().Height != 100 {
		t.Errorf("expected inner container size {40, 100} (height clamped), got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}
}

func TestContainer_AlignmentWithPadding(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Container with padding; child should be centered within the content box.
	// Content box: 100 - 10*2 = 80x80, child 20x20 -> centered at (30, 30) in content
	// Plus padding offset of 10 -> final offset (40, 40) relative to container origin.
	// Wrap in Center to give the outer Container loose constraints.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Width:     100,
			Height:    100,
			Padding:   layout.EdgeInsetsAll(10),
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  20,
				Height: 20,
				Color:  graphics.RGB(255, 255, 0),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// Verify child kept its natural size.
	if innerRO.Size().Width != 20 || innerRO.Size().Height != 20 {
		t.Errorf("expected inner container size {20, 20}, got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	// Content box is 80x80 starting at (10,10). Child (20x20) centered in 80x80
	// is at (30,30) within content box, so (10+30, 10+30) = (40, 40).
	if pd.Offset.X != 40 || pd.Offset.Y != 40 {
		t.Errorf("expected child offset {40, 40}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestContainer_PaddingReducesMaxConstraint(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Container 100x100 with padding 20 on all sides.
	// Content box is 60x60. Child wants 80x80 but should be clamped to 60x60.
	tester.PumpWidget(widgets.Center{
		Child: widgets.Container{
			Width:     100,
			Height:    100,
			Padding:   layout.EdgeInsetsAll(20),
			Alignment: layout.AlignmentCenter,
			Child: widgets.Container{
				Width:  80, // Wants more than content box allows
				Height: 80,
				Color:  graphics.RGB(255, 0, 255),
			},
		},
	})

	// Finder returns depth-first pre-order: outer container at index 0, inner at index 1.
	finder := tester.Find(drifttest.ByType[widgets.Container]())
	if finder.Count() < 2 {
		t.Fatal("expected at least 2 Container elements")
	}

	innerRO := getRenderObject(finder.At(1))

	// Child should be clamped to content box size (60x60).
	if innerRO.Size().Width != 60 || innerRO.Size().Height != 60 {
		t.Errorf("expected inner container size {60, 60} (clamped by padding), got {%v, %v}", innerRO.Size().Width, innerRO.Size().Height)
	}

	// Child fills content box exactly, so offset should be at padding origin.
	pd, ok := innerRO.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	if pd.Offset.X != 20 || pd.Offset.Y != 20 {
		t.Errorf("expected child offset {20, 20}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestContainer_InnerShadow(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 100})

	tester.PumpWidget(widgets.Container{
		Width:        60,
		Height:       60,
		Color:        graphics.RGB(200, 200, 200),
		BorderRadius: 8,
		BorderWidth:  1,
		BorderColor:  graphics.RGBA(0, 0, 0, 0.3),
		Shadow: &graphics.BoxShadow{
			Color:      graphics.RGBA(0, 0, 0, 0.5),
			BlurRadius: 10,
			BlurStyle:  graphics.BlurStyleInner,
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_inner_shadow.snapshot.json")

	ops := snap.DisplayOps

	// For inner shadows, the shadow should be drawn AFTER the background.
	bgIdx := findOpIndex(ops, "drawRRect") // background (rounded rect)
	shadowIdx := findOpIndex(ops, "drawRRectShadow")

	if bgIdx < 0 {
		t.Fatal("expected drawRRect op for background")
	}
	if shadowIdx < 0 {
		t.Fatal("expected drawRRectShadow op")
	}
	if shadowIdx < bgIdx {
		t.Errorf("inner shadow (index %d) should paint AFTER background (index %d)", shadowIdx, bgIdx)
	}
}

func TestContainer_InnerShadow_RectWithClip(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 100})

	// Non-rounded container with OverflowClip and inner shadow.
	tester.PumpWidget(widgets.Container{
		Width:    60,
		Height:   60,
		Color:    graphics.RGB(200, 200, 200),
		Overflow: widgets.OverflowClip,
		Shadow: &graphics.BoxShadow{
			Color:      graphics.RGBA(0, 0, 0, 0.5),
			BlurRadius: 10,
			BlurStyle:  graphics.BlurStyleInner,
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/container_inner_shadow_rect_clip.snapshot.json")

	ops := snap.DisplayOps

	// For inner shadows with no border radius, we use drawRect/drawRectShadow.
	bgIdx := findOpIndex(ops, "drawRect")
	shadowIdx := findOpIndex(ops, "drawRectShadow")

	if bgIdx < 0 {
		t.Fatal("expected drawRect op for background")
	}
	if shadowIdx < 0 {
		t.Fatal("expected drawRectShadow op")
	}
	if shadowIdx < bgIdx {
		t.Errorf("inner shadow (index %d) should paint AFTER background (index %d)", shadowIdx, bgIdx)
	}
}
