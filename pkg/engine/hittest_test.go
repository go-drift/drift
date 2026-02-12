package engine

import (
	"testing"

	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Mock render objects for hit test scenarios.

// decorativeEntry has no interactive interfaces.
type decorativeEntry struct {
	layout.RenderBoxBase
}

func (d *decorativeEntry) PerformLayout()                                            {}
func (d *decorativeEntry) Paint(ctx *layout.PaintContext)                            {}
func (d *decorativeEntry) HitTest(pos graphics.Offset, r *layout.HitTestResult) bool { return false }

// pointerHandlerEntry implements PointerHandler but not PlatformViewOwner.
type pointerHandlerEntry struct {
	layout.RenderBoxBase
}

func (p *pointerHandlerEntry) PerformLayout()                 {}
func (p *pointerHandlerEntry) Paint(ctx *layout.PaintContext) {}
func (p *pointerHandlerEntry) HitTest(pos graphics.Offset, r *layout.HitTestResult) bool {
	return false
}
func (p *pointerHandlerEntry) HandlePointer(event gestures.PointerEvent) {}

// platformViewEntry implements PlatformViewOwner but not PointerHandler
// (like renderVideoPlayer and renderNativeWebView).
type platformViewEntry struct {
	layout.RenderBoxBase
	viewID int64
}

func (v *platformViewEntry) PerformLayout()                                            {}
func (v *platformViewEntry) Paint(ctx *layout.PaintContext)                            {}
func (v *platformViewEntry) HitTest(pos graphics.Offset, r *layout.HitTestResult) bool { return false }
func (v *platformViewEntry) PlatformViewID() int64                                     { return v.viewID }

// platformViewWithPointerEntry implements both (like renderSwitch, renderTextInput).
type platformViewWithPointerEntry struct {
	layout.RenderBoxBase
	viewID int64
}

func (v *platformViewWithPointerEntry) PerformLayout()                 {}
func (v *platformViewWithPointerEntry) Paint(ctx *layout.PaintContext) {}
func (v *platformViewWithPointerEntry) HitTest(pos graphics.Offset, r *layout.HitTestResult) bool {
	return false
}
func (v *platformViewWithPointerEntry) HandlePointer(event gestures.PointerEvent) {}
func (v *platformViewWithPointerEntry) PlatformViewID() int64                     { return v.viewID }

// hitTestRoot is a mock root render object that returns a pre-configured hit test result.
type hitTestRoot struct {
	layout.RenderBoxBase
	entries []layout.RenderObject
}

func (r *hitTestRoot) PerformLayout()                 {}
func (r *hitTestRoot) Paint(ctx *layout.PaintContext) {}
func (r *hitTestRoot) HitTest(pos graphics.Offset, result *layout.HitTestResult) bool {
	for _, e := range r.entries {
		result.Add(e)
	}
	return len(r.entries) > 0
}

func TestHitTestPlatformView(t *testing.T) {
	tests := []struct {
		name    string
		viewID  int64
		entries []layout.RenderObject
		want    bool
	}{
		{
			name:    "PlatformViewOwner only, matching ID",
			viewID:  42,
			entries: []layout.RenderObject{&platformViewEntry{viewID: 42}},
			want:    true,
		},
		{
			name:    "PlatformViewOwner only, non-matching ID",
			viewID:  99,
			entries: []layout.RenderObject{&platformViewEntry{viewID: 42}},
			want:    false,
		},
		{
			name:    "PlatformViewOwner with PointerHandler, matching ID",
			viewID:  42,
			entries: []layout.RenderObject{&platformViewWithPointerEntry{viewID: 42}},
			want:    true,
		},
		{
			name:    "PointerHandler before PlatformViewOwner obscures view",
			viewID:  42,
			entries: []layout.RenderObject{&pointerHandlerEntry{}, &platformViewEntry{viewID: 42}},
			want:    false,
		},
		{
			name:    "decorative entries skipped, PlatformViewOwner matched",
			viewID:  42,
			entries: []layout.RenderObject{&decorativeEntry{}, &platformViewEntry{viewID: 42}},
			want:    true,
		},
		{
			name:    "decorative entries only",
			viewID:  42,
			entries: []layout.RenderObject{&decorativeEntry{}},
			want:    false,
		},
		{
			name:    "no entries",
			viewID:  42,
			entries: nil,
			want:    false,
		},
		{
			name:    "nil root render",
			viewID:  42,
			entries: nil, // root will be set to nil below
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := app
			defer func() { app = saved }()

			app = newAppRunner()
			app.deviceScale = 1.0

			if tt.name == "nil root render" {
				app.rootRender = nil
			} else {
				root := &hitTestRoot{entries: tt.entries}
				root.SetSelf(root)
				app.rootRender = root
			}

			got := HitTestPlatformView(tt.viewID, 50, 50)
			if got != tt.want {
				t.Errorf("HitTestPlatformView() = %v, want %v", got, tt.want)
			}
		})
	}
}
