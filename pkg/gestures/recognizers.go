package gestures

import (
	"github.com/go-drift/drift/pkg/rendering"
	"math"
	"time"
)

// DragStartDetails describes the start of a drag.
type DragStartDetails struct {
	Position rendering.Offset
}

// DragUpdateDetails describes a drag update.
type DragUpdateDetails struct {
	Position rendering.Offset
	Delta    rendering.Offset
}

// DragEndDetails describes the end of a drag.
type DragEndDetails struct {
	Position rendering.Offset
	Velocity rendering.Offset
}

// TapGestureRecognizer detects taps.
type TapGestureRecognizer struct {
	Arena   *GestureArena
	OnTap   func()
	pointer int64
	start   rendering.Offset
	slop    float64
	won     bool
	reject  bool
	up      bool
	fired   bool
}

// NewTapGestureRecognizer creates a tap recognizer.
func NewTapGestureRecognizer(arena *GestureArena) *TapGestureRecognizer {
	return &TapGestureRecognizer{Arena: arena}
}

// AddPointer registers a pointer down event.
func (t *TapGestureRecognizer) AddPointer(event PointerEvent) {
	if t.Arena == nil {
		return
	}
	t.pointer = event.PointerID
	t.start = event.Position
	t.slop = DefaultTouchSlop
	t.won = false
	t.reject = false
	t.up = false
	t.fired = false
	t.Arena.Add(event.PointerID, t)
}

// HandleEvent processes pointer events for tap detection.
func (t *TapGestureRecognizer) HandleEvent(event PointerEvent) {
	if event.PointerID != t.pointer || t.reject {
		return
	}
	switch event.Phase {
	case PointerPhaseMove:
		dx := event.Position.X - t.start.X
		dy := event.Position.Y - t.start.Y
		if dx*dx+dy*dy > t.slop*t.slop {
			t.reject = true
			t.Arena.Reject(event.PointerID, t)
		}
	case PointerPhaseUp:
		t.up = true
		if !t.reject {
			t.Arena.Resolve(event.PointerID, t)
			t.tryFire()
		}
	case PointerPhaseCancel:
		t.reject = true
		t.Arena.Reject(event.PointerID, t)
	}
}

// AcceptGesture is called by the arena when this recognizer wins.
func (t *TapGestureRecognizer) AcceptGesture(pointerID int64) {
	if pointerID != t.pointer || t.reject {
		return
	}
	t.won = true
	t.tryFire()
}

// RejectGesture is called by the arena when this recognizer loses.
func (t *TapGestureRecognizer) RejectGesture(pointerID int64) {
	if pointerID != t.pointer {
		return
	}
	t.reject = true
}

// Dispose releases resources for the recognizer.
func (t *TapGestureRecognizer) Dispose() {}

func (t *TapGestureRecognizer) tryFire() {
	if t.fired || t.reject || !t.won || !t.up {
		return
	}
	t.fired = true
	if t.OnTap != nil {
		t.OnTap()
	}
}

// PanGestureRecognizer detects pan gestures.
type PanGestureRecognizer struct {
	Arena    *GestureArena
	OnStart  func(DragStartDetails)
	OnUpdate func(DragUpdateDetails)
	OnEnd    func(DragEndDetails)
	OnCancel func()
	pointer  int64
	start    rendering.Offset
	last     rendering.Offset
	lastTime time.Time
	velocity rendering.Offset
	slop     float64
	accepted bool
	reject   bool
	started  bool
}

// NewPanGestureRecognizer creates a pan recognizer.
func NewPanGestureRecognizer(arena *GestureArena) *PanGestureRecognizer {
	return &PanGestureRecognizer{Arena: arena}
}

// AddPointer registers a pointer down event.
func (p *PanGestureRecognizer) AddPointer(event PointerEvent) {
	if p.Arena == nil {
		return
	}
	p.pointer = event.PointerID
	p.start = event.Position
	p.last = event.Position
	p.lastTime = time.Now()
	p.velocity = rendering.Offset{}
	p.slop = DefaultTouchSlop
	p.accepted = false
	p.reject = false
	p.started = false
	p.Arena.Add(event.PointerID, p)
}

// HandleEvent processes pointer events for drag detection.
func (p *PanGestureRecognizer) HandleEvent(event PointerEvent) {
	if event.PointerID != p.pointer || p.reject {
		return
	}
	switch event.Phase {
	case PointerPhaseMove:
		now := time.Now()
		dt := now.Sub(p.lastTime).Seconds()
		dx := event.Position.X - p.last.X
		dy := event.Position.Y - p.last.Y
		delta := rendering.Offset{X: dx, Y: dy}
		if dt > 0 {
			inst := rendering.Offset{X: dx / dt, Y: dy / dt}
			p.velocity = rendering.Offset{
				X: p.velocity.X*0.8 + inst.X*0.2,
				Y: p.velocity.Y*0.8 + inst.Y*0.2,
			}
		}
		total := rendering.Offset{X: event.Position.X - p.start.X, Y: event.Position.Y - p.start.Y}
		if !p.accepted && distance(total) > p.slop {
			p.Arena.Resolve(event.PointerID, p)
		}
		if p.accepted {
			p.ensureStarted()
			if p.OnUpdate != nil {
				p.OnUpdate(DragUpdateDetails{Position: event.Position, Delta: delta})
			}
		}
		p.last = event.Position
		p.lastTime = now
	case PointerPhaseUp:
		if p.accepted {
			if p.OnEnd != nil {
				p.OnEnd(DragEndDetails{Position: event.Position, Velocity: p.velocity})
			}
		} else {
			p.Arena.Reject(event.PointerID, p)
		}
	case PointerPhaseCancel:
		if p.accepted && p.OnCancel != nil {
			p.OnCancel()
		}
		p.reject = true
		p.Arena.Reject(event.PointerID, p)
	}
}

// AcceptGesture is called by the arena when this recognizer wins.
func (p *PanGestureRecognizer) AcceptGesture(pointerID int64) {
	if pointerID != p.pointer || p.reject {
		return
	}
	p.accepted = true
	p.ensureStarted()
}

// RejectGesture is called by the arena when this recognizer loses.
func (p *PanGestureRecognizer) RejectGesture(pointerID int64) {
	if pointerID != p.pointer {
		return
	}
	p.reject = true
}

// Dispose releases resources for the recognizer.
func (p *PanGestureRecognizer) Dispose() {}

func (p *PanGestureRecognizer) ensureStarted() {
	if p.started {
		return
	}
	p.started = true
	if p.OnStart != nil {
		p.OnStart(DragStartDetails{Position: p.start})
	}
}

func distance(offset rendering.Offset) float64 {
	return math.Hypot(offset.X, offset.Y)
}
