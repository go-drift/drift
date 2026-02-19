package widgets_test

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	drifttest "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

func TestLayoutBuilder_ReceivesConstraints(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	var received layout.Constraints
	tester.PumpWidget(widgets.LayoutBuilder{
		Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
			received = constraints
			return widgets.SizedBox{Width: 100, Height: 50}
		},
	})

	// Root uses tight constraints matching the surface size (via DeviceScale wrapper)
	if received.MaxWidth == 0 {
		t.Fatal("expected builder to receive non-zero constraints")
	}
	if received.MaxWidth != 400 || received.MaxHeight != 300 {
		t.Errorf("expected max constraints 400x300, got %vx%v", received.MaxWidth, received.MaxHeight)
	}
}

func TestLayoutBuilder_ChildRebuildsOnConstraintChange(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	buildCount := 0
	builder := func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
		buildCount++
		return widgets.SizedBox{Width: constraints.MaxWidth, Height: 50}
	}

	// First pump at 400x300
	tester.PumpWidget(widgets.LayoutBuilder{Builder: builder})
	if buildCount != 1 {
		t.Fatalf("expected 1 build, got %d", buildCount)
	}

	// Change size and repump
	tester.SetSize(graphics.Size{Width: 200, Height: 300})
	tester.PumpWidget(widgets.LayoutBuilder{Builder: builder})
	if buildCount < 2 {
		t.Errorf("expected at least 2 builds after constraint change, got %d", buildCount)
	}
}

func TestLayoutBuilder_ChildRebuildsOnBuilderChange(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	tester.PumpWidget(widgets.LayoutBuilder{
		Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
			return widgets.Text{Content: "first"}
		},
	})

	result := tester.Find(drifttest.ByText("first"))
	if !result.Exists() {
		t.Fatal("expected to find 'first' text")
	}

	// Pump with a different builder
	tester.PumpWidget(widgets.LayoutBuilder{
		Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
			return widgets.Text{Content: "second"}
		},
	})

	result = tester.Find(drifttest.ByText("second"))
	if !result.Exists() {
		t.Fatal("expected to find 'second' text after builder change")
	}
}

func TestLayoutBuilder_NilBuilder(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	// Wrap in Center so LayoutBuilder gets loose constraints and can shrink to zero
	tester.PumpWidget(widgets.Center{
		Child: widgets.LayoutBuilder{
			Builder: nil,
		},
	})

	result := tester.Find(drifttest.ByType[widgets.LayoutBuilder]())
	if !result.Exists() {
		t.Fatal("expected LayoutBuilder element to exist")
	}
	size := result.RenderObject().Size()
	if size.Width != 0 || size.Height != 0 {
		t.Errorf("expected zero size for nil builder, got %vx%v", size.Width, size.Height)
	}
}

func TestLayoutBuilder_NestedInSizedBox(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	var received layout.Constraints
	// Wrap in Center so SizedBox gets loose constraints and can apply its own size
	tester.PumpWidget(widgets.Center{
		Child: widgets.SizedBox{
			Width:  200,
			Height: 100,
			Child: widgets.LayoutBuilder{
				Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
					received = constraints
					return widgets.SizedBox{Width: 50, Height: 50}
				},
			},
		},
	})

	// SizedBox with Width=200 and Height=100 gives tight 200x100 constraints
	if received.MaxWidth != 200 || received.MaxHeight != 100 {
		t.Errorf("expected constraints 200x100 from SizedBox, got max %vx%v",
			received.MaxWidth, received.MaxHeight)
	}
}

func TestLayoutBuilder_ChildContainsRenderObjects(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	// Wrap in Center so LayoutBuilder gets loose constraints
	tester.PumpWidget(widgets.Center{
		Child: widgets.LayoutBuilder{
			Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
				return widgets.Padding{
					Padding: layout.EdgeInsetsAll(10),
					Child:   widgets.SizedBox{Width: 100, Height: 50},
				}
			},
		},
	})

	// Verify the Padding render object exists and has correct size
	result := tester.Find(drifttest.ByType[widgets.Padding]())
	if !result.Exists() {
		t.Fatal("expected Padding inside LayoutBuilder to exist")
	}

	size := result.RenderObject().Size()
	// 100 + 20 padding = 120 wide, 50 + 20 padding = 70 tall
	if size.Width != 120 || size.Height != 70 {
		t.Errorf("expected padding size 120x70, got %vx%v", size.Width, size.Height)
	}
}

func TestLayoutBuilder_Unmount(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	tester.PumpWidget(widgets.LayoutBuilder{
		Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
			return widgets.SizedBox{Width: 100, Height: 50}
		},
	})

	// Verify the tree exists
	result := tester.Find(drifttest.ByType[widgets.LayoutBuilder]())
	if !result.Exists() {
		t.Fatal("expected LayoutBuilder element to exist before unmount")
	}

	// Replace the entire tree with something else, unmounting the LayoutBuilder
	tester.PumpWidget(widgets.SizedBox{Width: 10, Height: 10})

	// LayoutBuilder should be gone
	result = tester.Find(drifttest.ByType[widgets.LayoutBuilder]())
	if result.Exists() {
		t.Fatal("expected LayoutBuilder element to be unmounted")
	}
}

func TestLayoutBuilder_ComponentElementChild(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	// Builder returns a StatelessWidget (component element), not a
	// RenderObjectWidget directly. This exercises the case where the
	// LayoutBuilder's immediate child element is not a renderObjectHost,
	// confirming the render tree connects correctly through deeper nesting.
	tester.PumpWidget(widgets.Center{
		Child: widgets.LayoutBuilder{
			Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
				return wrappedSizedBox{width: 80, height: 40}
			},
		},
	})

	result := tester.Find(drifttest.ByType[widgets.SizedBox]())
	if !result.Exists() {
		t.Fatal("expected SizedBox inside stateless wrapper to exist")
	}
	size := result.RenderObject().Size()
	if size.Width != 80 || size.Height != 40 {
		t.Errorf("expected size 80x40, got %vx%v", size.Width, size.Height)
	}
}

// wrappedSizedBox is a stateless widget that wraps a SizedBox.
// Used to test that LayoutBuilder works when its child is a component
// element (not a direct renderObjectHost).
type wrappedSizedBox struct {
	core.StatelessBase
	width, height float64
}

func (w wrappedSizedBox) Build(ctx core.BuildContext) core.Widget {
	return widgets.SizedBox{Width: w.width, Height: w.height}
}

func TestLayoutBuilder_InheritedDependencyUpdate(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	// Pump with initial inherited value. The LayoutBuilder's builder reads
	// the inherited string via ProviderOf and renders it as Text content.
	tester.PumpWidget(core.InheritedProvider[string]{
		Value: "hello",
		Child: widgets.LayoutBuilder{
			Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
				label, _ := core.ProviderOf[string](ctx)
				return widgets.Text{Content: label}
			},
		},
	})

	result := tester.Find(drifttest.ByText("hello"))
	if !result.Exists() {
		t.Fatal("expected to find 'hello' text")
	}

	// Update the inherited value without changing constraints. The builder
	// must re-run so the new value is reflected in the output.
	tester.PumpWidget(core.InheritedProvider[string]{
		Value: "world",
		Child: widgets.LayoutBuilder{
			Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
				label, _ := core.ProviderOf[string](ctx)
				return widgets.Text{Content: label}
			},
		},
	})

	result = tester.Find(drifttest.ByText("world"))
	if !result.Exists() {
		t.Fatal("expected to find 'world' text after inherited update")
	}

	// Verify a second update also propagates (guards against the dirty flag
	// not being cleared properly, causing MarkNeedsBuild to early-return).
	tester.PumpWidget(core.InheritedProvider[string]{
		Value: "again",
		Child: widgets.LayoutBuilder{
			Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
				label, _ := core.ProviderOf[string](ctx)
				return widgets.Text{Content: label}
			},
		},
	})

	result = tester.Find(drifttest.ByText("again"))
	if !result.Exists() {
		t.Fatal("expected to find 'again' text after second inherited update")
	}
}

func TestLayoutBuilder_NoRebuildOnBareRelayout(t *testing.T) {
	tester := drifttest.NewWidgetTesterWithT(t)
	tester.SetSize(graphics.Size{Width: 400, Height: 300})

	buildCount := 0
	tester.PumpWidget(widgets.LayoutBuilder{
		Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
			buildCount++
			return widgets.SizedBox{Width: 100, Height: 50}
		},
	})
	if buildCount != 1 {
		t.Fatalf("expected 1 build after first pump, got %d", buildCount)
	}

	// Pump a bare frame without changing the widget tree. No elements are
	// dirty and no render objects need layout, so the layout callback should
	// not fire and the builder should not be re-invoked.
	tester.Pump()
	if buildCount != 1 {
		t.Errorf("expected builder not re-invoked on bare pump, build count = %d", buildCount)
	}
}
