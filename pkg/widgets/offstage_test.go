package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

type testHitBox struct {
	layout.RenderBoxBase
	paintCalls int
}

func (b *testHitBox) PerformLayout() {
	b.SetSize(graphics.Size{Width: 10, Height: 10})
}

func (b *testHitBox) Paint(ctx *layout.PaintContext) {
	b.paintCalls++
}

func (b *testHitBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	size := b.Size()
	if position.X < 0 || position.Y < 0 || position.X > size.Width || position.Y > size.Height {
		return false
	}
	result.Add(b)
	return true
}

func TestOffstage_SkipsPaintAndHitTest(t *testing.T) {
	child := &testHitBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	offstage := &renderOffstage{offstage: true}
	offstage.SetSelf(offstage)
	offstage.SetSize(graphics.Size{Width: 10, Height: 10})
	offstage.SetChild(child)

	recorder := &graphics.PictureRecorder{}
	ctx := &layout.PaintContext{
		Canvas: recorder.BeginRecording(graphics.Size{Width: 10, Height: 10}),
	}
	offstage.Paint(ctx)
	if child.paintCalls != 0 {
		t.Fatalf("expected offstage child to skip paint, got %d paint calls", child.paintCalls)
	}

	result := &layout.HitTestResult{}
	if offstage.HitTest(graphics.Offset{X: 5, Y: 5}, result) {
		t.Fatal("expected offstage to skip hit testing, but it returned true")
	}
}

func TestOffstage_AllowsHitTestWhenVisible(t *testing.T) {
	child := &testHitBox{}
	child.SetSelf(child)
	child.SetSize(graphics.Size{Width: 10, Height: 10})

	offstage := &renderOffstage{offstage: false}
	offstage.SetSelf(offstage)
	offstage.SetSize(graphics.Size{Width: 10, Height: 10})
	offstage.SetChild(child)

	result := &layout.HitTestResult{}
	if !offstage.HitTest(graphics.Offset{X: 5, Y: 5}, result) {
		t.Fatal("expected visible offstage to hit test child")
	}
}

type testLayoutBox struct {
	layout.RenderBoxBase
	layoutCalls int
}

func (b *testLayoutBox) PerformLayout() {
	b.layoutCalls++
	b.SetSize(graphics.Size{Width: 10, Height: 10})
}

func (b *testLayoutBox) Paint(ctx *layout.PaintContext) {}

func (b *testLayoutBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

func TestOffstage_SkipsChildLayoutWhenHiddenAndConstraintsStable(t *testing.T) {
	child := &testLayoutBox{}
	child.SetSelf(child)

	offstage := &renderOffstage{offstage: true}
	offstage.SetSelf(offstage)
	offstage.SetChild(child)

	constraints := layout.Tight(graphics.Size{Width: 10, Height: 10})
	offstage.Layout(constraints, true)
	if child.layoutCalls != 1 {
		t.Fatalf("expected initial layout call, got %d", child.layoutCalls)
	}

	offstage.Layout(constraints, true)
	if child.layoutCalls != 1 {
		t.Fatalf("expected layout to be skipped when offstage, got %d", child.layoutCalls)
	}
}

func TestOffstage_ChildSwapClearsCachedSize(t *testing.T) {
	first := &testLayoutBox{}
	first.SetSelf(first)
	second := &testLayoutBox{}
	second.SetSelf(second)

	offstage := &renderOffstage{offstage: true}
	offstage.SetSelf(offstage)
	offstage.SetChild(first)

	constraints := layout.Tight(graphics.Size{Width: 10, Height: 10})
	offstage.Layout(constraints, true)
	if first.layoutCalls != 1 {
		t.Fatalf("expected first child layout, got %d", first.layoutCalls)
	}

	offstage.SetChild(second)
	offstage.Layout(constraints, true)
	if second.layoutCalls != 1 {
		t.Fatalf("expected second child layout after swap, got %d", second.layoutCalls)
	}
}
