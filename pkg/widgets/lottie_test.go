package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

func TestRenderLottie_NilSource_ZeroSize(t *testing.T) {
	r := &renderLottie{}
	r.SetSelf(r)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  500,
		MinHeight: 0,
		MaxHeight: 500,
	}
	r.Layout(constraints, false)

	size := r.Size()
	if size.Width != 0 || size.Height != 0 {
		t.Fatalf("expected zero size for nil source, got %v", size)
	}
}

func TestRenderLottie_ExplicitDimensions(t *testing.T) {
	r := &renderLottie{width: 200, height: 100}
	r.SetSelf(r)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  500,
		MinHeight: 0,
		MaxHeight: 500,
	}
	r.Layout(constraints, false)

	size := r.Size()
	// With nil source but explicit dimensions, source is nil so size is zero.
	// The explicit dimensions only apply when source is non-nil.
	if size.Width != 0 || size.Height != 0 {
		t.Fatalf("expected zero size when source is nil (even with explicit dimensions), got %v", size)
	}
}

func TestRenderLottie_NilSource_PaintIsNoOp(t *testing.T) {
	r := &renderLottie{}
	r.SetSelf(r)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  500,
		MinHeight: 0,
		MaxHeight: 500,
	}
	r.Layout(constraints, false)

	recorder := &graphics.PictureRecorder{}
	canvas := recorder.BeginRecording(graphics.Size{Width: 500, Height: 500})

	ctx := &layout.PaintContext{Canvas: canvas}
	// Should not panic with nil source.
	r.Paint(ctx)
}

func TestRenderLottie_ConstraintsClamping(t *testing.T) {
	// Even with nil source (zero desired size), min constraints should be applied.
	r := &renderLottie{}
	r.SetSelf(r)

	constraints := layout.Constraints{
		MinWidth:  50,
		MaxWidth:  500,
		MinHeight: 30,
		MaxHeight: 500,
	}
	r.Layout(constraints, false)

	size := r.Size()
	if size.Width != 50 || size.Height != 30 {
		t.Fatalf("expected min-constrained size (50, 30), got %v", size)
	}
}

func TestRenderLottie_HitTest_NilSource(t *testing.T) {
	r := &renderLottie{}
	r.SetSelf(r)

	constraints := layout.Constraints{
		MinWidth:  0,
		MaxWidth:  500,
		MinHeight: 0,
		MaxHeight: 500,
	}
	r.Layout(constraints, false)

	result := &layout.HitTestResult{}

	// Position outside zero-size widget should not be hit.
	hit := r.HitTest(graphics.Offset{X: 5, Y: 5}, result)
	if hit {
		t.Fatal("expected no hit outside zero-size widget")
	}
}
