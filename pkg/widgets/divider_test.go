package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

// --- Divider layout tests ---

func TestDivider_ExpandsToMaxWidth(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 300, Height: 200})

	// Column gives children tight width (stretches to column width) and
	// loose height, matching real-world usage.
	tester.PumpWidget(widgets.Column{
		Children: []core.Widget{
			widgets.Divider{
				Height:    16,
				Thickness: 1,
				Color:     graphics.RGB(200, 200, 200),
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.Divider]())
	if !result.Exists() {
		t.Fatal("expected Divider element to exist")
	}
	size := result.RenderObject().Size()
	if size.Width != 300 {
		t.Errorf("expected divider width 300, got %v", size.Width)
	}
	if size.Height != 16 {
		t.Errorf("expected divider height 16, got %v", size.Height)
	}
}

func TestDivider_HeightClampedByConstraints(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 8})

	// Tight constraints: divider wants height 16, but parent only allows 8.
	tester.PumpWidget(widgets.Divider{
		Height:    16,
		Thickness: 1,
		Color:     graphics.RGB(200, 200, 200),
	})

	result := tester.Find(drifttest.ByType[widgets.Divider]())
	if !result.Exists() {
		t.Fatal("expected Divider element to exist")
	}
	size := result.RenderObject().Size()
	if size.Height != 8 {
		t.Errorf("expected divider height clamped to 8, got %v", size.Height)
	}
}

// --- Divider paint tests ---

func TestDivider_DrawsRect(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Wrap in Column so the divider gets loose height and can choose 16.
	tester.PumpWidget(widgets.Column{
		Children: []core.Widget{
			widgets.Divider{
				Height:    16,
				Thickness: 2,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) == 0 {
		t.Fatal("expected at least one drawRect op for divider line")
	}

	// The rect should span the full width and be centered vertically
	// within the 16px height: top = (16-2)/2 = 7, bottom = 7+2 = 9.
	rect := rects[0].Params["rect"].(map[string]any)
	if left := rect["left"].(float64); left != 0 {
		t.Errorf("expected rect left 0, got %v", left)
	}
	if right := rect["right"].(float64); right != 200 {
		t.Errorf("expected rect right 200, got %v", right)
	}
	if top := rect["top"].(float64); top != 7 {
		t.Errorf("expected rect top 7, got %v", top)
	}
	if bottom := rect["bottom"].(float64); bottom != 9 {
		t.Errorf("expected rect bottom 9, got %v", bottom)
	}
}

func TestDivider_IndentReducesDrawWidth(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Column{
		Children: []core.Widget{
			widgets.Divider{
				Height:    16,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
				Indent:    20,
				EndIndent: 30,
			},
		},
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) == 0 {
		t.Fatal("expected at least one drawRect op")
	}

	rect := rects[0].Params["rect"].(map[string]any)
	if left := rect["left"].(float64); left != 20 {
		t.Errorf("expected rect left 20, got %v", left)
	}
	// right = indent + drawWidth = 20 + (200-20-30) = 20 + 150 = 170
	if right := rect["right"].(float64); right != 170 {
		t.Errorf("expected rect right 170, got %v", right)
	}
}

func TestDivider_NoPaintWhenZeroColor(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Divider{
		Height:    16,
		Thickness: 1,
		Color:     0, // transparent
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) != 0 {
		t.Errorf("expected no drawRect ops for transparent divider, got %d", len(rects))
	}
}

func TestDivider_NoPaintWhenZeroThickness(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Divider{
		Height:    16,
		Thickness: 0,
		Color:     graphics.RGB(100, 100, 100),
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) != 0 {
		t.Errorf("expected no drawRect ops for zero-thickness divider, got %d", len(rects))
	}
}

func TestDivider_NoPaintWhenIndentsExceedWidth(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 200})

	tester.PumpWidget(widgets.Divider{
		Height:    16,
		Thickness: 1,
		Color:     graphics.RGB(100, 100, 100),
		Indent:    60,
		EndIndent: 60, // 60 + 60 = 120 > 100
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) != 0 {
		t.Errorf("expected no drawRect ops when indents exceed width, got %d", len(rects))
	}
}

// --- VerticalDivider layout tests ---

func TestVerticalDivider_ExpandsToMaxHeight(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 300})

	// Row gives children tight height (stretches to row height) and
	// loose width, matching real-world usage.
	tester.PumpWidget(widgets.Row{
		Children: []core.Widget{
			widgets.VerticalDivider{
				Width:     16,
				Thickness: 1,
				Color:     graphics.RGB(200, 200, 200),
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.VerticalDivider]())
	if !result.Exists() {
		t.Fatal("expected VerticalDivider element to exist")
	}
	size := result.RenderObject().Size()
	if size.Height != 300 {
		t.Errorf("expected vertical divider height 300, got %v", size.Height)
	}
	if size.Width != 16 {
		t.Errorf("expected vertical divider width 16, got %v", size.Width)
	}
}

func TestVerticalDivider_WidthClampedByConstraints(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 8, Height: 100})

	// Tight constraints: VerticalDivider wants width 16, but parent only allows 8.
	tester.PumpWidget(widgets.VerticalDivider{
		Width:     16,
		Thickness: 1,
		Color:     graphics.RGB(200, 200, 200),
	})

	result := tester.Find(drifttest.ByType[widgets.VerticalDivider]())
	if !result.Exists() {
		t.Fatal("expected VerticalDivider element to exist")
	}
	size := result.RenderObject().Size()
	if size.Width != 8 {
		t.Errorf("expected vertical divider width clamped to 8, got %v", size.Width)
	}
}

// --- VerticalDivider paint tests ---

func TestVerticalDivider_DrawsRect(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Wrap in Row so the vertical divider gets loose width and can choose 16.
	tester.PumpWidget(widgets.Row{
		Children: []core.Widget{
			widgets.VerticalDivider{
				Width:     16,
				Thickness: 2,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) == 0 {
		t.Fatal("expected at least one drawRect op for vertical divider line")
	}

	// The rect should span the full height and be centered horizontally
	// within the 16px width: left = (16-2)/2 = 7, right = 7+2 = 9.
	rect := rects[0].Params["rect"].(map[string]any)
	if top := rect["top"].(float64); top != 0 {
		t.Errorf("expected rect top 0, got %v", top)
	}
	if bottom := rect["bottom"].(float64); bottom != 200 {
		t.Errorf("expected rect bottom 200, got %v", bottom)
	}
	if left := rect["left"].(float64); left != 7 {
		t.Errorf("expected rect left 7, got %v", left)
	}
	if right := rect["right"].(float64); right != 9 {
		t.Errorf("expected rect right 9, got %v", right)
	}
}

func TestVerticalDivider_IndentReducesDrawHeight(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Row{
		Children: []core.Widget{
			widgets.VerticalDivider{
				Width:     16,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
				Indent:    10,
				EndIndent: 20,
			},
		},
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) == 0 {
		t.Fatal("expected at least one drawRect op")
	}

	rect := rects[0].Params["rect"].(map[string]any)
	if top := rect["top"].(float64); top != 10 {
		t.Errorf("expected rect top 10, got %v", top)
	}
	// bottom = indent + drawHeight = 10 + (200-10-20) = 10 + 170 = 180
	if bottom := rect["bottom"].(float64); bottom != 180 {
		t.Errorf("expected rect bottom 180, got %v", bottom)
	}
}

func TestVerticalDivider_NoPaintWhenIndentsExceedHeight(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 100})

	tester.PumpWidget(widgets.VerticalDivider{
		Width:     16,
		Thickness: 1,
		Color:     graphics.RGB(100, 100, 100),
		Indent:    60,
		EndIndent: 60, // 60 + 60 = 120 > 100
	})

	snap := tester.CaptureSnapshot()
	rects := findOps(snap.DisplayOps, "drawRect")
	if len(rects) != 0 {
		t.Errorf("expected no drawRect ops when indents exceed height, got %d", len(rects))
	}
}

// --- UpdateRenderObject tests ---

func TestDivider_UpdateTriggersRelayoutOnHeightChange(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Column{
		Children: []core.Widget{
			widgets.Divider{
				Height:    16,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.Divider]())
	size1 := result.RenderObject().Size()
	if size1.Height != 16 {
		t.Fatalf("expected initial height 16, got %v", size1.Height)
	}

	// Update with different height.
	tester.PumpWidget(widgets.Column{
		Children: []core.Widget{
			widgets.Divider{
				Height:    24,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	result = tester.Find(drifttest.ByType[widgets.Divider]())
	size2 := result.RenderObject().Size()
	if size2.Height != 24 {
		t.Errorf("expected updated height 24, got %v", size2.Height)
	}
}

func TestVerticalDivider_UpdateTriggersRelayoutOnWidthChange(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Row{
		Children: []core.Widget{
			widgets.VerticalDivider{
				Width:     16,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.VerticalDivider]())
	size1 := result.RenderObject().Size()
	if size1.Width != 16 {
		t.Fatalf("expected initial width 16, got %v", size1.Width)
	}

	// Update with different width.
	tester.PumpWidget(widgets.Row{
		Children: []core.Widget{
			widgets.VerticalDivider{
				Width:     24,
				Thickness: 1,
				Color:     graphics.RGB(100, 100, 100),
			},
		},
	})

	result = tester.Find(drifttest.ByType[widgets.VerticalDivider]())
	size2 := result.RenderObject().Size()
	if size2.Width != 24 {
		t.Errorf("expected updated width 24, got %v", size2.Width)
	}
}
