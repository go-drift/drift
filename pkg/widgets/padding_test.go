package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestPadding_ChildOffset(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Padding{
		Padding:     layout.EdgeInsetsAll(16),
		ChildWidget: widgets.Text{Content: "padded"},
	})

	result := tester.Find(drifttest.ByType[widgets.Text]())
	if !result.Exists() {
		t.Fatal("expected Text element to exist")
	}
	ro := result.RenderObject()
	pd, ok := ro.ParentData().(*layout.BoxParentData)
	if !ok {
		t.Fatal("expected BoxParentData on child render object")
	}
	if pd.Offset.X != 16 || pd.Offset.Y != 16 {
		t.Errorf("expected child offset {16, 16}, got {%v, %v}", pd.Offset.X, pd.Offset.Y)
	}
}

func TestPadding_Size(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	// Wrap in Center to give the Padding loose constraints, so it can
	// size to its content rather than being forced to the tester surface size.
	tester.PumpWidget(widgets.Center{
		ChildWidget: widgets.Padding{
			Padding:     layout.EdgeInsetsOnly(10, 20, 30, 40),
			ChildWidget: widgets.SizedBox{Width: 50, Height: 50},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.Padding]())
	if !result.Exists() {
		t.Fatal("expected Padding element to exist")
	}
	size := result.RenderObject().Size()
	// Expected: child 50x50 + left 10 + right 30 = 90 wide, top 20 + bottom 40 = 110 tall
	if size.Width != 90 || size.Height != 110 {
		t.Errorf("expected padding size {90, 110}, got {%v, %v}", size.Width, size.Height)
	}
}

func TestPadding_ConstraintDeflation(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 200, Height: 200})

	tester.PumpWidget(widgets.Padding{
		Padding:     layout.EdgeInsetsAll(20),
		ChildWidget: widgets.SizedBox{Width: 500, Height: 500},
	})

	result := tester.Find(drifttest.ByType[widgets.Padding]())
	if !result.Exists() {
		t.Fatal("expected Padding element to exist")
	}
	size := result.RenderObject().Size()
	// Child requests 500x500 but constraints deflated by 40 each axis (20+20),
	// so max child size is 160x160. Padding render = child + insets = 200x200.
	if size.Width != 200 || size.Height != 200 {
		t.Errorf("expected padding size {200, 200}, got {%v, %v}", size.Width, size.Height)
	}
}

func TestPadding_DisplayOps_Translate(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Padding{
		Padding:     layout.EdgeInsetsAll(8),
		ChildWidget: widgets.Text{Content: "offset"},
	})

	snap := tester.CaptureSnapshot()
	snap.MatchesFile(t, "testdata/padding_translate.snapshot.json")

	ops := snap.DisplayOps
	found := false
	for _, op := range ops {
		if op.Op == "translate" {
			dx, _ := op.Params["dx"].(float64)
			dy, _ := op.Params["dy"].(float64)
			if dx == 8 && dy == 8 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected a translate op with dx=8, dy=8")
	}
}
