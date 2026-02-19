package overlay

import (
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"

	dtesting "github.com/go-drift/drift/pkg/testing"
)

// dialogTestWidget is a helper that wraps content in an Overlay whose child
// calls ShowDialog (or ShowAlertDialog) from a BuildContext that is a
// descendant of the overlay, ensuring OverlayOf(ctx) finds the overlay.
//
// The onBuild callback receives a valid BuildContext and should call
// ShowDialog/ShowAlertDialog. It fires once on first build.
type dialogTestWidget struct {
	core.StatelessBase
	onBuild func(ctx core.BuildContext)
}

func (w dialogTestWidget) Build(ctx core.BuildContext) core.Widget {
	return Overlay{
		Child: dialogTrigger{onBuild: w.onBuild},
	}
}

// dialogTrigger is the Overlay's child. Its BuildContext is below the
// overlayInherited, so OverlayOf(ctx) works correctly.
type dialogTrigger struct {
	core.StatefulBase
	onBuild func(ctx core.BuildContext)
}

func (d dialogTrigger) CreateState() core.State {
	return &dialogTriggerState{}
}

type dialogTriggerState struct {
	core.StateBase
	fired bool
}

func (s *dialogTriggerState) Build(ctx core.BuildContext) core.Widget {
	if !s.fired {
		s.fired = true
		widget := s.Element().Widget().(dialogTrigger)
		if widget.onBuild != nil {
			widget.onBuild(ctx)
		}
	}
	return widgets.SizedBox{Width: 400, Height: 400}
}

// TestShowDialog_CreatesEntries verifies that ShowDialog returns a dismiss function
// and inserts a ModalBarrier entry, a Center-wrapped dialog entry, and renders
// the builder's content.
func TestShowDialog_CreatesEntries(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var dismiss func()
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			dismiss = ShowDialog(ctx, DialogOptions{
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.Text{Content: "dialog content"}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if dismiss == nil {
		t.Fatal("expected dismiss to be set")
	}
	if !tester.Find(dtesting.ByType[ModalBarrier]()).Exists() {
		t.Error("expected ModalBarrier entry")
	}
	if !tester.Find(dtesting.ByType[widgets.Center]()).Exists() {
		t.Error("expected Center entry (dialog)")
	}
	if !tester.Find(dtesting.ByText("dialog content")).Exists() {
		t.Error("expected dialog content to be rendered")
	}
}

// TestShowDialog_DismissRemovesBothEntries verifies that dismiss removes barrier and dialog.
func TestShowDialog_DismissRemovesBothEntries(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var dismiss func()
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			dismiss = ShowDialog(ctx, DialogOptions{
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.Text{Content: "dialog"}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !tester.Find(dtesting.ByText("dialog")).Exists() {
		t.Fatal("expected dialog to be visible")
	}

	dismiss()
	tester.Pump()

	if tester.Find(dtesting.ByText("dialog")).Exists() {
		t.Error("expected dialog to be removed after dismiss")
	}
}

// TestShowDialog_DismissIsIdempotent verifies that calling dismiss multiple times is safe.
func TestShowDialog_DismissIsIdempotent(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var dismiss func()
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			dismiss = ShowDialog(ctx, DialogOptions{
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.Text{Content: "idempotent"}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic on multiple dismiss calls.
	dismiss()
	dismiss()
	dismiss()
	tester.Pump()
}

// TestShowDialog_PersistentBarrier verifies that Persistent=true creates a
// non-dismissible barrier and that tapping the barrier does not dismiss the dialog.
func TestShowDialog_PersistentBarrier(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowDialog(ctx, DialogOptions{
				Persistent: true,
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.SizedBox{Width: 10, Height: 10}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	result := tester.Find(dtesting.ByType[ModalBarrier]())
	if !result.Exists() {
		t.Fatal("expected ModalBarrier")
	}
	barrier := result.First().Widget().(ModalBarrier)
	if barrier.Dismissible {
		t.Error("expected barrier to be non-dismissible when Persistent=true")
	}

	// Tap the barrier area; the dialog should survive.
	if err := tester.TapAt(graphics.Offset{X: 5, Y: 5}); err != nil {
		t.Fatal(err)
	}
	tester.Pump()

	if !tester.Find(dtesting.ByType[ModalBarrier]()).Exists() {
		t.Error("expected dialog to remain after barrier tap with Persistent=true")
	}
}

// TestShowDialog_ZeroBarrierColor verifies that an unset BarrierColor passes
// through as transparent (zero value).
func TestShowDialog_ZeroBarrierColor(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowDialog(ctx, DialogOptions{
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.SizedBox{Width: 10, Height: 10}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	result := tester.Find(dtesting.ByType[ModalBarrier]())
	if !result.Exists() {
		t.Fatal("expected ModalBarrier")
	}
	barrier := result.First().Widget().(ModalBarrier)
	if barrier.Color != 0 {
		t.Errorf("expected transparent barrier color (0), got %v", barrier.Color)
	}
}

// TestShowAlertDialog_BarrierColor verifies that ShowAlertDialog applies the
// theme's Scrim color at 50% alpha.
func TestShowAlertDialog_BarrierColor(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowAlertDialog(ctx, AlertDialogOptions{
				Title:        "Test",
				ConfirmLabel: "OK",
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	result := tester.Find(dtesting.ByType[ModalBarrier]())
	if !result.Exists() {
		t.Fatal("expected ModalBarrier")
	}
	barrier := result.First().Widget().(ModalBarrier)
	if barrier.Color == 0 {
		t.Error("expected non-zero barrier color from ShowAlertDialog")
	}
	alpha := barrier.Color.Alpha()
	if alpha < 0.49 || alpha > 0.51 {
		t.Errorf("expected barrier alpha ~0.5, got %f", alpha)
	}
}

// TestShowDialog_CustomBarrierColor verifies that a custom barrier color is used.
func TestShowDialog_CustomBarrierColor(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	customColor := graphics.RGBA(255, 0, 0, 0.3)
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowDialog(ctx, DialogOptions{
				BarrierColor: customColor,
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.SizedBox{Width: 10, Height: 10}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	result := tester.Find(dtesting.ByType[ModalBarrier]())
	if !result.Exists() {
		t.Fatal("expected ModalBarrier")
	}
	barrier := result.First().Widget().(ModalBarrier)
	if barrier.Color != customColor {
		t.Errorf("expected custom barrier color %v, got %v", customColor, barrier.Color)
	}
}

// TestShowDialog_BarrierTapDismisses verifies that tapping the barrier (outside
// the dialog content) dismisses the dialog, confirming that Opaque on the dialog
// entry does not block hits on the barrier entry.
func TestShowDialog_BarrierTapDismisses(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowDialog(ctx, DialogOptions{
				Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
					return widgets.SizedBox{Width: 10, Height: 10}
				},
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !tester.Find(dtesting.ByType[ModalBarrier]()).Exists() {
		t.Fatal("expected barrier to be visible")
	}

	// Tap a corner, well outside the centered 10x10 dialog content.
	// The hit passes through the dialog entry (miss) and lands on the
	// barrier entry, which calls dismiss via OnDismiss.
	if err := tester.TapAt(graphics.Offset{X: 5, Y: 5}); err != nil {
		t.Fatal(err)
	}
	tester.Pump()

	if tester.Find(dtesting.ByType[ModalBarrier]()).Exists() {
		t.Error("expected dialog to be dismissed after barrier tap")
	}
}

// TestShowDialog_NoOverlay verifies that ShowDialog returns a no-op without panic
// when there is no Overlay ancestor.
func TestShowDialog_NoOverlay(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(noOverlayShowDialog{})
	if err != nil {
		t.Fatal(err)
	}

	tester.Pump()
}

// noOverlayShowDialog is a widget that calls ShowDialog without an overlay.
type noOverlayShowDialog struct {
	core.StatelessBase
}

func (n noOverlayShowDialog) Build(ctx core.BuildContext) core.Widget {
	dismiss := ShowDialog(ctx, DialogOptions{
		Builder: func(ctx core.BuildContext, dismiss func()) core.Widget {
			return widgets.SizedBox{}
		},
	})
	dismiss()
	return widgets.SizedBox{Width: 10, Height: 10}
}

// TestShowAlertDialog_ConfirmCallsCallback verifies that the confirm button fires OnConfirm and dismisses.
func TestShowAlertDialog_ConfirmCallsCallback(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	confirmCalled := false
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowAlertDialog(ctx, AlertDialogOptions{
				Title:        "Delete?",
				Content:      "This cannot be undone.",
				ConfirmLabel: "Delete",
				OnConfirm:    func() { confirmCalled = true },
				CancelLabel:  "Cancel",
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !tester.Find(dtesting.ByText("Delete?")).Exists() {
		t.Error("expected title")
	}
	if !tester.Find(dtesting.ByText("This cannot be undone.")).Exists() {
		t.Error("expected content")
	}

	if err := tester.Tap(dtesting.ByText("Delete")); err != nil {
		t.Fatal(err)
	}
	tester.Pump()

	if !confirmCalled {
		t.Error("expected OnConfirm to be called")
	}

	if tester.Find(dtesting.ByText("Delete?")).Exists() {
		t.Error("expected dialog to be dismissed after confirm")
	}
}

// TestShowAlertDialog_CancelCallsCallback verifies that the cancel button fires OnCancel and dismisses.
func TestShowAlertDialog_CancelCallsCallback(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	cancelCalled := false
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowAlertDialog(ctx, AlertDialogOptions{
				Title:        "Warning",
				ConfirmLabel: "OK",
				CancelLabel:  "Cancel",
				OnCancel:     func() { cancelCalled = true },
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if err := tester.Tap(dtesting.ByText("Cancel")); err != nil {
		t.Fatal(err)
	}
	tester.Pump()

	if !cancelCalled {
		t.Error("expected OnCancel to be called")
	}

	if tester.Find(dtesting.ByText("Warning")).Exists() {
		t.Error("expected dialog to be dismissed after cancel")
	}
}

// TestShowAlertDialog_DestructiveUsesErrorColor verifies that Destructive=true
// styles the confirm button with the Error color.
func TestShowAlertDialog_DestructiveUsesErrorColor(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowAlertDialog(ctx, AlertDialogOptions{
				Title:        "Delete?",
				ConfirmLabel: "Delete",
				Destructive:  true,
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	result := tester.Find(dtesting.ByType[widgets.Button]())
	if !result.Exists() {
		t.Fatal("expected Button widget")
	}
	btn := result.First().Widget().(widgets.Button)
	expectedColor := theme.LightColorScheme().Error
	if btn.Color != expectedColor {
		t.Errorf("expected confirm button color %v (Error), got %v", expectedColor, btn.Color)
	}
}

// TestShowAlertDialog_ConfirmOnly verifies that ShowAlertDialog works with only
// a confirm button (no cancel).
func TestShowAlertDialog_ConfirmOnly(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	confirmCalled := false
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			ShowAlertDialog(ctx, AlertDialogOptions{
				Title:        "Notice",
				Content:      "Something happened.",
				ConfirmLabel: "OK",
				OnConfirm:    func() { confirmCalled = true },
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Only one button should be present.
	buttons := tester.Find(dtesting.ByType[widgets.Button]())
	if buttons.Count() != 1 {
		t.Errorf("expected 1 button, got %d", buttons.Count())
	}

	if err := tester.Tap(dtesting.ByText("OK")); err != nil {
		t.Fatal(err)
	}
	tester.Pump()

	if !confirmCalled {
		t.Error("expected OnConfirm to be called")
	}
	if tester.Find(dtesting.ByText("Notice")).Exists() {
		t.Error("expected dialog to be dismissed")
	}
}

// TestShowDialog_NilBuilder verifies that ShowDialog with a nil Builder
// returns a no-op dismiss without panicking.
func TestShowDialog_NilBuilder(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var dismiss func()
	err := tester.PumpWidget(dialogTestWidget{
		onBuild: func(ctx core.BuildContext) {
			dismiss = ShowDialog(ctx, DialogOptions{})
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if dismiss == nil {
		t.Fatal("expected dismiss to be set")
	}
	// Should be safe to call.
	dismiss()
}
