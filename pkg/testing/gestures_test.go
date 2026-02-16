package testing

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/testing/internal/testbed"
)

func TestTap_Counter(t *testing.T) {
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.Counter{Initial: 0})

	if err := tester.Tap(ByText("0")); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()

	if !tester.Find(ByText("1")).Exists() {
		t.Error("expected count to be 1 after tap")
	}
}

func TestTap_CounterMultiple(t *testing.T) {
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.Counter{Initial: 0})

	for range 3 {
		tester.Tap(ByType[testbed.Counter]())
		tester.Pump()
	}

	if !tester.Find(ByText("3")).Exists() {
		t.Error("expected count to be 3 after three taps")
	}
}

func TestTap_Callback(t *testing.T) {
	var lastCount int
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.Counter{
		Initial: 10,
		OnTap:   func(count int) { lastCount = count },
	})

	tester.Tap(ByType[testbed.Counter]())
	tester.Pump()

	if lastCount != 11 {
		t.Errorf("expected callback with count 11, got %d", lastCount)
	}
}

func TestTap_NoMatch(t *testing.T) {
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.Counter{Initial: 0})

	err := tester.Tap(ByText("nonexistent"))
	if err == nil {
		t.Error("expected error when tapping nonexistent element")
	}
}

func TestTapAt(t *testing.T) {
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.LayoutBox{Width: 100, Height: 100})

	// TapAt should not error on a mounted widget
	err := tester.TapAt(graphics.Offset{X: 50, Y: 50})
	if err != nil {
		t.Errorf("TapAt failed: %v", err)
	}
}

func TestSendPointerDown_NoWidget(t *testing.T) {
	tester := NewWidgetTesterWithT(t)

	err := tester.SendPointerDown(graphics.Offset{}, 1)
	if err == nil {
		t.Error("expected error when sending pointer with no widget mounted")
	}
}

func TestDrag(t *testing.T) {
	tester := NewWidgetTesterWithT(t)
	tester.PumpWidget(testbed.LayoutBox{Width: 200, Height: 200})

	err := tester.DragFrom(
		graphics.Offset{X: 100, Y: 100},
		graphics.Offset{X: 50, Y: 0},
	)
	if err != nil {
		t.Errorf("DragFrom failed: %v", err)
	}
}
