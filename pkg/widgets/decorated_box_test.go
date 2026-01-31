package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestDecoratedBox_ChildClipping_RoundedCorners(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 100})

	// DecoratedBox with rounded corners and OverflowClip should clip children
	// to the rounded shape. Use OverflowVisible on child to isolate the parent's
	// child clip from the child's own background clip.
	tester.PumpWidget(widgets.DecoratedBox{
		Color:        graphics.RGB(200, 200, 200),
		BorderRadius: 12,
		Overflow:     widgets.OverflowClip,
		ChildWidget: widgets.Container{
			Width:    100,
			Height:   100,
			Color:    graphics.RGB(255, 0, 0),
			Overflow: widgets.OverflowVisible, // no self-clipping
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/decorated_box_child_clip_rounded.snapshot.json")

	ops := snap.DisplayOps

	// Find all clipRRect operations. There should be two:
	// 1. Background clip (for the DecoratedBox background)
	// 2. Child clip (for clipping children to rounded bounds)
	var clipRRectIndices []int
	for i, op := range ops {
		if op.Op == "clipRRect" {
			clipRRectIndices = append(clipRRectIndices, i)
		}
	}
	if len(clipRRectIndices) < 2 {
		t.Fatalf("expected at least 2 clipRRect ops (background + child), got %d", len(clipRRectIndices))
	}

	// The second clipRRect is the child clip
	childClipIdx := clipRRectIndices[1]

	// Find the child's drawRect (the red container) - must come after child clip
	childDrawIdx := -1
	for i, op := range ops {
		if op.Op == "drawRect" {
			if c, ok := op.Params["color"].(string); ok && c == "0xFFFF0000" {
				childDrawIdx = i
				break
			}
		}
	}
	if childDrawIdx < 0 {
		t.Fatal("expected child drawRect")
	}
	if childDrawIdx <= childClipIdx {
		t.Errorf("child drawRect (index %d) should come after child clipRRect (index %d)", childDrawIdx, childClipIdx)
	}
}

func TestDecoratedBox_ChildClipping_RectangularBounds(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 100})

	// DecoratedBox with no border radius and OverflowClip should clip children
	// to the rectangular bounds. Use OverflowVisible on child to isolate the
	// parent's child clip from the child's own background clip.
	tester.PumpWidget(widgets.DecoratedBox{
		Color:    graphics.RGB(200, 200, 200),
		Overflow: widgets.OverflowClip,
		ChildWidget: widgets.Container{
			Width:    100,
			Height:   100,
			Color:    graphics.RGB(0, 255, 0),
			Overflow: widgets.OverflowVisible, // no self-clipping
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/decorated_box_child_clip_rect.snapshot.json")

	ops := snap.DisplayOps

	// Find all clipRect operations. There should be two:
	// 1. Background clip (for the DecoratedBox background)
	// 2. Child clip (for clipping children to rectangular bounds)
	var clipRectIndices []int
	for i, op := range ops {
		if op.Op == "clipRect" {
			clipRectIndices = append(clipRectIndices, i)
		}
	}
	if len(clipRectIndices) < 2 {
		t.Fatalf("expected at least 2 clipRect ops (background + child), got %d", len(clipRectIndices))
	}

	// The second clipRect is the child clip
	childClipIdx := clipRectIndices[1]

	// Find the child's drawRect (the green container) - must come after child clip
	childDrawIdx := -1
	for i, op := range ops {
		if op.Op == "drawRect" {
			if c, ok := op.Params["color"].(string); ok && c == "0xFF00FF00" {
				childDrawIdx = i
				break
			}
		}
	}
	if childDrawIdx < 0 {
		t.Fatal("expected child drawRect")
	}
	if childDrawIdx <= childClipIdx {
		t.Errorf("child drawRect (index %d) should come after child clipRect (index %d)", childDrawIdx, childClipIdx)
	}

	// Should not have clipRRect when border radius is 0
	for _, op := range ops {
		if op.Op == "clipRRect" {
			t.Error("did not expect clipRRect op when BorderRadius is 0")
			break
		}
	}
}

func TestDecoratedBox_OverflowVisible_NoChildClipping(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 100, Height: 100})

	// DecoratedBox with OverflowVisible should not clip children.
	// Note: Child Container also uses OverflowVisible to avoid its own clipping.
	tester.PumpWidget(widgets.DecoratedBox{
		Color:        graphics.RGB(200, 200, 200),
		BorderRadius: 12,
		Overflow:     widgets.OverflowVisible,
		ChildWidget: widgets.Container{
			Width:    100,
			Height:   100,
			Color:    graphics.RGB(0, 0, 255),
			Overflow: widgets.OverflowVisible,
		},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/decorated_box_overflow_visible.snapshot.json")

	ops := snap.DisplayOps

	// Count clip operations - there should be none when both parent and child
	// use OverflowVisible with solid colors.
	clipCount := 0
	for _, op := range ops {
		if op.Op == "clipRect" || op.Op == "clipRRect" {
			clipCount++
		}
	}
	if clipCount != 0 {
		t.Errorf("expected no clip operations with OverflowVisible, got %d", clipCount)
	}
}
