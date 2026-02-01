package layout

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

type testRenderBox struct {
	RenderBoxBase
	paintCalls int
}

func (r *testRenderBox) PerformLayout() {
	r.SetSize(graphics.Size{Width: 10, Height: 10})
}

func (r *testRenderBox) Paint(ctx *PaintContext) {
	r.paintCalls++
}

func (r *testRenderBox) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

func (r *testRenderBox) IsRepaintBoundary() bool {
	return true
}

func TestPaintChildWithLayer_UsesCachedLayerWhenClean(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	recorder := &graphics.PictureRecorder{}
	recordCanvas := recorder.BeginRecording(graphics.Size{Width: 10, Height: 10})
	recordCanvas.DrawRect(graphics.RectFromLTWH(0, 0, 10, 10), graphics.DefaultPaint())
	layer := recorder.EndRecording()

	child.SetLayer(layer)
	child.ClearNeedsPaint()

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 0 {
		t.Fatalf("expected cached layer to be used, but child.Paint was called %d times", child.paintCalls)
	}
}

func TestPaintChildWithLayer_PaintsChildWhenNoLayer(t *testing.T) {
	child := &testRenderBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	outputRecorder := &graphics.PictureRecorder{}
	ctx := &PaintContext{
		Canvas: outputRecorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}

	ctx.PaintChildWithLayer(child, graphics.Offset{})

	if child.paintCalls != 1 {
		t.Fatalf("expected child.Paint to be called once, got %d", child.paintCalls)
	}
}
