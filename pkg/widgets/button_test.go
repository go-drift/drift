package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

// --- GestureDetector tests ---

func TestGestureDetector_TapCallback(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tapped := false
	tester.PumpWidget(widgets.GestureDetector{
		OnTap: func() { tapped = true },
		Child: widgets.Container{
			Width:  100,
			Height: 50,
			Color:  graphics.RGB(200, 200, 200),
		},
	})

	if err := tester.Tap(drifttest.ByType[widgets.GestureDetector]()); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()

	if !tapped {
		t.Error("expected OnTap callback to fire")
	}
}

func TestGestureDetector_TapNoCallback(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.GestureDetector{
		Child: widgets.Container{
			Width:  100,
			Height: 50,
			Color:  graphics.RGB(200, 200, 200),
		},
	})

	// Should not panic when no OnTap is set; tap must still succeed (hit test hit).
	if err := tester.Tap(drifttest.ByType[widgets.GestureDetector]()); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()
}

// --- Button tests ---

func TestButton_Tap(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tapped := false
	tester.PumpWidget(widgets.Button{Label: "Click", OnTap: func() { tapped = true }, Haptic: true})

	if err := tester.Tap(drifttest.ByText("Click")); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()

	if !tapped {
		t.Error("expected button tap callback to fire")
	}
}

func TestButton_Disabled(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tapped := false
	tester.PumpWidget(widgets.Button{Label: "Click", OnTap: func() { tapped = true }, Disabled: true, Haptic: true})

	if err := tester.Tap(drifttest.ByText("Click")); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()

	if tapped {
		t.Error("disabled button should not fire tap callback")
	}
}

func TestButton_Label(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Button{Label: "Submit"})

	if !tester.Find(drifttest.ByText("Submit")).Exists() {
		t.Error("expected to find button label text \"Submit\"")
	}
}

func TestButton_WidgetTree(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tester.PumpWidget(widgets.Button{Label: "OK"})

	if !tester.Find(drifttest.ByType[widgets.GestureDetector]()).Exists() {
		t.Error("expected GestureDetector in button's widget tree")
	}
	if !tester.Find(drifttest.ByType[widgets.Text]()).Exists() {
		t.Error("expected Text in button's widget tree")
	}
}

func TestButton_DisabledNilsGesture(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)

	tapped := false
	tester.PumpWidget(widgets.Button{Label: "Nope", OnTap: func() { tapped = true }, Disabled: true, Haptic: true})

	// GestureDetector is in the tree but OnTap should be nil when disabled
	if !tester.Find(drifttest.ByType[widgets.GestureDetector]()).Exists() {
		t.Error("expected GestureDetector in disabled button's widget tree")
	}

	if err := tester.Tap(drifttest.ByType[widgets.GestureDetector]()); err != nil {
		t.Fatalf("Tap failed: %v", err)
	}
	tester.Pump()

	if tapped {
		t.Error("disabled button should not fire tap via GestureDetector")
	}
}
