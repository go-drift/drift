package widgets

import (
	"math"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// sheetDragRegion handles vertical drag gestures with conditional acceptance.
type sheetDragRegion struct {
	Child       core.Widget
	ShouldStart func(totalDelta float64) bool
	OnStart     func(DragStartDetails)
	OnUpdate    func(DragUpdateDetails)
	OnEnd       func(DragEndDetails)
	OnCancel    func()
}

func (s sheetDragRegion) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s sheetDragRegion) Key() any {
	return nil
}

func (s sheetDragRegion) ChildWidget() core.Widget {
	return s.Child
}

func (s sheetDragRegion) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderSheetDragRegion{}
	r.SetSelf(r)
	r.configure(s)
	return r
}

func (s sheetDragRegion) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderSheetDragRegion); ok {
		r.configure(s)
		r.MarkNeedsPaint()
	}
}

// renderSheetDragRegion is the render object for sheetDragRegion.
// It wraps a child and intercepts pointer events for the drag recognizer.
type renderSheetDragRegion struct {
	layout.RenderBoxBase
	child      layout.RenderBox
	recognizer *conditionalVerticalDragRecognizer
}

func (r *renderSheetDragRegion) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderSheetDragRegion) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderSheetDragRegion) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	r.child.Layout(constraints, true)
	r.SetSize(r.child.Size())
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderSheetDragRegion) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
}

func (r *renderSheetDragRegion) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil {
		r.child.HitTest(position, result)
	}
	result.Add(r)
	return true
}

func (r *renderSheetDragRegion) HandlePointer(event gestures.PointerEvent) {
	if r.recognizer == nil {
		return
	}
	if event.Phase == gestures.PointerPhaseDown {
		r.recognizer.AddPointer(event)
		return
	}
	r.recognizer.HandleEvent(event)
}

func (r *renderSheetDragRegion) configure(s sheetDragRegion) {
	if r.recognizer == nil {
		r.recognizer = newConditionalVerticalDragRecognizer(gestures.DefaultArena)
	}
	r.recognizer.ShouldAccept = s.ShouldStart
	r.recognizer.OnStart = s.OnStart
	r.recognizer.OnUpdate = s.OnUpdate
	r.recognizer.OnEnd = s.OnEnd
	r.recognizer.OnCancel = s.OnCancel
}

// conditionalVerticalDragRecognizer is a gesture recognizer for vertical drags
// that can conditionally accept or reject based on the drag direction and context.
// This enables content-aware dragging where the sheet decides whether to handle
// a gesture or let it pass through to scrollable content.
type conditionalVerticalDragRecognizer struct {
	Arena        *gestures.GestureArena
	ShouldAccept func(totalDelta float64) bool // called when drag exceeds slop to decide acceptance
	OnStart      func(DragStartDetails)
	OnUpdate     func(DragUpdateDetails)
	OnEnd        func(DragEndDetails)
	OnCancel     func()

	pointer  int64           // current pointer being tracked
	start    graphics.Offset // initial touch position
	last     graphics.Offset // most recent touch position
	lastTime time.Time       // timestamp of last update (for velocity)
	velocity float64         // smoothed vertical velocity in pixels/second
	slop     float64         // minimum distance before recognizing a drag
	accepted bool            // true after winning gesture arena
	reject   bool            // true if gesture was rejected
	started  bool            // true after OnStart has been called
}

func newConditionalVerticalDragRecognizer(arena *gestures.GestureArena) *conditionalVerticalDragRecognizer {
	return &conditionalVerticalDragRecognizer{Arena: arena}
}

func (c *conditionalVerticalDragRecognizer) AddPointer(event gestures.PointerEvent) {
	if c.Arena == nil {
		return
	}
	c.pointer = event.PointerID
	c.start = event.Position
	c.last = event.Position
	c.lastTime = time.Now()
	c.velocity = 0
	c.slop = gestures.DefaultTouchSlop
	c.accepted = false
	c.reject = false
	c.started = false
	c.Arena.Add(event.PointerID, c)
	c.Arena.Hold(event.PointerID, c)
}

func (c *conditionalVerticalDragRecognizer) HandleEvent(event gestures.PointerEvent) {
	if event.PointerID != c.pointer || c.reject {
		return
	}
	switch event.Phase {
	case gestures.PointerPhaseMove:
		c.handleMove(event)
	case gestures.PointerPhaseUp:
		c.handleUp(event)
	case gestures.PointerPhaseCancel:
		c.handleCancel()
	}
}

// handleMove processes pointer move events, determining whether to accept the gesture
// and tracking velocity for fling detection.
func (c *conditionalVerticalDragRecognizer) handleMove(event gestures.PointerEvent) {
	now := time.Now()
	dt := now.Sub(c.lastTime).Seconds()

	// Calculate total movement from start
	total := graphics.Offset{X: event.Position.X - c.start.X, Y: event.Position.Y - c.start.Y}
	primary := math.Abs(total.Y)
	orthogonal := math.Abs(total.X)

	// Gesture recognition: decide to accept or reject once slop is exceeded
	if !c.accepted {
		if primary > c.slop && primary >= orthogonal {
			// Vertical movement dominant: ask callback if we should accept
			shouldAccept := true
			if c.ShouldAccept != nil {
				shouldAccept = c.ShouldAccept(total.Y)
			}
			if shouldAccept {
				c.Arena.Resolve(c.pointer, c)
			} else {
				// Callback rejected: let other recognizers handle it
				c.reject = true
				c.Arena.Reject(c.pointer, c)
				return
			}
		} else if orthogonal > c.slop {
			// Horizontal movement dominant: reject (likely a horizontal scroll)
			c.reject = true
			c.Arena.Reject(c.pointer, c)
			return
		}
	}

	// Update velocity using exponential smoothing for stable fling detection
	delta := graphics.Offset{X: event.Position.X - c.last.X, Y: event.Position.Y - c.last.Y}
	if dt > 0 {
		inst := delta.Y / dt
		c.velocity = c.velocity*0.8 + inst*0.2
	}

	// Dispatch update if gesture is accepted
	if c.accepted {
		c.ensureStarted()
		if c.OnUpdate != nil {
			c.OnUpdate(DragUpdateDetails{
				Position:     event.Position,
				Delta:        delta,
				PrimaryDelta: delta.Y,
			})
		}
	}

	c.last = event.Position
	c.lastTime = now
}

func (c *conditionalVerticalDragRecognizer) handleUp(event gestures.PointerEvent) {
	if c.accepted {
		if c.OnEnd != nil {
			c.OnEnd(DragEndDetails{
				Position:        event.Position,
				Velocity:        graphics.Offset{X: 0, Y: c.velocity},
				PrimaryVelocity: c.velocity,
			})
		}
	} else {
		c.Arena.Reject(c.pointer, c)
	}
}

func (c *conditionalVerticalDragRecognizer) handleCancel() {
	if c.accepted && c.OnCancel != nil {
		c.OnCancel()
	}
	c.reject = true
	c.Arena.Reject(c.pointer, c)
}

func (c *conditionalVerticalDragRecognizer) AcceptGesture(pointerID int64) {
	if pointerID != c.pointer || c.reject {
		return
	}
	c.accepted = true
	c.ensureStarted()
}

func (c *conditionalVerticalDragRecognizer) RejectGesture(pointerID int64) {
	if pointerID != c.pointer {
		return
	}
	c.reject = true
}

func (c *conditionalVerticalDragRecognizer) ensureStarted() {
	if c.started {
		return
	}
	c.started = true
	if c.OnStart != nil {
		c.OnStart(DragStartDetails{Position: c.start})
	}
}
