package gestures

import (
	"github.com/go-drift/drift/pkg/graphics"
	"math"
	"time"
)

// DragStartDetails describes the start of a drag.
type DragStartDetails struct {
	// Position is the global position where the drag started.
	Position graphics.Offset
}

// DragUpdateDetails describes a drag update.
type DragUpdateDetails struct {
	// Position is the current global position of the pointer.
	Position graphics.Offset
	// Delta is the change in position since the last update.
	Delta graphics.Offset
	// PrimaryDelta is the axis-specific delta (0 for Pan, non-zero for axis-locked drags).
	PrimaryDelta float64
}

// DragEndDetails describes the end of a drag.
type DragEndDetails struct {
	// Position is the final global position of the pointer.
	Position graphics.Offset
	// Velocity is the velocity of the pointer at release in pixels per second.
	Velocity graphics.Offset
	// PrimaryVelocity is the axis-specific velocity (0 for Pan, non-zero for axis-locked drags).
	PrimaryVelocity float64
}

// TapGestureRecognizer detects taps.
type TapGestureRecognizer struct {
	Arena   *GestureArena
	OnTap   func()
	pointer int64
	start   graphics.Offset
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
	start    graphics.Offset
	last     graphics.Offset
	lastTime time.Time
	velocity graphics.Offset
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
	p.velocity = graphics.Offset{}
	p.slop = DefaultTouchSlop
	p.accepted = false
	p.reject = false
	p.started = false
	p.Arena.Add(event.PointerID, p)
	// Hold immediately to prevent auto-resolve on Close before slop is exceeded
	p.Arena.Hold(event.PointerID, p)
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
		delta := graphics.Offset{X: dx, Y: dy}
		if dt > 0 {
			inst := graphics.Offset{X: dx / dt, Y: dy / dt}
			p.velocity = graphics.Offset{
				X: p.velocity.X*0.8 + inst.X*0.2,
				Y: p.velocity.Y*0.8 + inst.Y*0.2,
			}
		}
		total := graphics.Offset{X: event.Position.X - p.start.X, Y: event.Position.Y - p.start.Y}
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

func distance(offset graphics.Offset) float64 {
	return math.Hypot(offset.X, offset.Y)
}

// DragAxis specifies the direction for axis-locked drag recognizers.
type DragAxis int

const (
	// DragAxisHorizontal restricts drag detection to horizontal movement.
	DragAxisHorizontal DragAxis = iota
	// DragAxisVertical restricts drag detection to vertical movement.
	DragAxisVertical
)

// axisDragRecognizer is the shared implementation for axis-locked drags.
type axisDragRecognizer struct {
	Arena    *GestureArena
	OnStart  func(DragStartDetails)
	OnUpdate func(DragUpdateDetails)
	OnEnd    func(DragEndDetails)
	OnCancel func()

	axis     DragAxis
	self     ArenaMember // concrete type for arena registration
	pointer  int64
	start    graphics.Offset
	last     graphics.Offset
	lastTime time.Time
	velocity float64 // primary axis velocity
	slop     float64
	accepted bool
	reject   bool
	started  bool
}

func (d *axisDragRecognizer) primaryOffset(offset graphics.Offset) float64 {
	if d.axis == DragAxisHorizontal {
		return offset.X
	}
	return offset.Y
}

func (d *axisDragRecognizer) orthogonalOffset(offset graphics.Offset) float64 {
	if d.axis == DragAxisHorizontal {
		return offset.Y
	}
	return offset.X
}

func (d *axisDragRecognizer) addPointer(event PointerEvent) {
	if d.Arena == nil || d.self == nil {
		return
	}
	d.pointer = event.PointerID
	d.start = event.Position
	d.last = event.Position
	d.lastTime = time.Now()
	d.velocity = 0
	d.slop = DefaultTouchSlop
	d.accepted = false
	d.reject = false
	d.started = false
	d.Arena.Add(event.PointerID, d.self)
	// Hold immediately to prevent auto-resolve on Close before we determine axis
	d.Arena.Hold(event.PointerID, d.self)
}

func (d *axisDragRecognizer) handleEvent(event PointerEvent) {
	if event.PointerID != d.pointer || d.reject {
		return
	}
	switch event.Phase {
	case PointerPhaseMove:
		d.handleMove(event)
	case PointerPhaseUp:
		d.handleUp(event)
	case PointerPhaseCancel:
		d.handleCancel()
	}
}

func (d *axisDragRecognizer) handleMove(event PointerEvent) {
	now := time.Now()
	dt := now.Sub(d.lastTime).Seconds()

	total := graphics.Offset{X: event.Position.X - d.start.X, Y: event.Position.Y - d.start.Y}
	primary := math.Abs(d.primaryOffset(total))
	orthogonal := math.Abs(d.orthogonalOffset(total))

	// Check if we should resolve or reject (we hold from addPointer)
	if !d.accepted {
		if primary > d.slop && primary >= orthogonal {
			// Primary axis wins (>= handles ties in favor of primary)
			d.Arena.Resolve(d.pointer, d.self)
		} else if orthogonal > d.slop {
			// Orthogonal axis exceeds slop first - reject
			d.reject = true
			d.Arena.Reject(d.pointer, d.self)
			return
		}
	}

	// Update velocity tracking
	delta := graphics.Offset{X: event.Position.X - d.last.X, Y: event.Position.Y - d.last.Y}
	primaryDelta := d.primaryOffset(delta)
	if dt > 0 {
		// Ignore extremely small dt samples and clamp instantaneous velocity spikes.
		// This makes fling handoff stable when event cadence is irregular.
		const minVelocityDt = 1.0 / 240.0
		const maxInstantVelocity = 12000.0
		if dt >= minVelocityDt {
			inst := primaryDelta / dt
			if inst > maxInstantVelocity {
				inst = maxInstantVelocity
			} else if inst < -maxInstantVelocity {
				inst = -maxInstantVelocity
			}
			// Time-based EMA keeps smoothing consistent across frame rates.
			const smoothingTau = 0.06
			alpha := dt / (smoothingTau + dt)
			d.velocity = d.velocity*(1-alpha) + inst*alpha
		}
	}

	if d.accepted {
		d.ensureStarted()
		if d.OnUpdate != nil {
			d.OnUpdate(DragUpdateDetails{
				Position:     event.Position,
				Delta:        delta,
				PrimaryDelta: primaryDelta,
			})
		}
	}

	d.last = event.Position
	d.lastTime = now
}

func (d *axisDragRecognizer) handleUp(event PointerEvent) {
	if d.accepted {
		if d.OnEnd != nil {
			var vel graphics.Offset
			if d.axis == DragAxisHorizontal {
				vel = graphics.Offset{X: d.velocity, Y: 0}
			} else {
				vel = graphics.Offset{X: 0, Y: d.velocity}
			}
			d.OnEnd(DragEndDetails{
				Position:        event.Position,
				Velocity:        vel,
				PrimaryVelocity: d.velocity,
			})
		}
	} else {
		d.Arena.Reject(d.pointer, d.self)
	}
}

func (d *axisDragRecognizer) handleCancel() {
	if d.accepted && d.OnCancel != nil {
		d.OnCancel()
	}
	d.reject = true
	d.Arena.Reject(d.pointer, d.self)
}

func (d *axisDragRecognizer) acceptGesture(pointerID int64) {
	if pointerID != d.pointer || d.reject {
		return
	}
	d.accepted = true
	d.ensureStarted()
}

func (d *axisDragRecognizer) rejectGesture(pointerID int64) {
	if pointerID != d.pointer {
		return
	}
	d.reject = true
}

func (d *axisDragRecognizer) ensureStarted() {
	if d.started {
		return
	}
	d.started = true
	if d.OnStart != nil {
		d.OnStart(DragStartDetails{Position: d.start})
	}
}

// HorizontalDragGestureRecognizer detects horizontal drag gestures.
// It wins when |deltaX| > slop before |deltaY| > slop.
type HorizontalDragGestureRecognizer struct {
	axisDragRecognizer
}

// NewHorizontalDragGestureRecognizer creates a horizontal drag recognizer.
func NewHorizontalDragGestureRecognizer(arena *GestureArena) *HorizontalDragGestureRecognizer {
	h := &HorizontalDragGestureRecognizer{}
	h.Arena = arena
	h.axis = DragAxisHorizontal
	h.self = h // set self-reference for arena registration
	return h
}

// AddPointer registers a pointer down event.
func (h *HorizontalDragGestureRecognizer) AddPointer(event PointerEvent) {
	h.addPointer(event)
}

// HandleEvent processes pointer events for drag detection.
func (h *HorizontalDragGestureRecognizer) HandleEvent(event PointerEvent) {
	h.handleEvent(event)
}

// AcceptGesture is called by the arena when this recognizer wins.
func (h *HorizontalDragGestureRecognizer) AcceptGesture(pointerID int64) {
	h.acceptGesture(pointerID)
}

// RejectGesture is called by the arena when this recognizer loses.
func (h *HorizontalDragGestureRecognizer) RejectGesture(pointerID int64) {
	h.rejectGesture(pointerID)
}

// Dispose releases resources for the recognizer.
func (h *HorizontalDragGestureRecognizer) Dispose() {}

// VerticalDragGestureRecognizer detects vertical drag gestures.
// It wins when |deltaY| > slop before |deltaX| > slop.
type VerticalDragGestureRecognizer struct {
	axisDragRecognizer
}

// NewVerticalDragGestureRecognizer creates a vertical drag recognizer.
func NewVerticalDragGestureRecognizer(arena *GestureArena) *VerticalDragGestureRecognizer {
	v := &VerticalDragGestureRecognizer{}
	v.Arena = arena
	v.axis = DragAxisVertical
	v.self = v // set self-reference for arena registration
	return v
}

// AddPointer registers a pointer down event.
func (v *VerticalDragGestureRecognizer) AddPointer(event PointerEvent) {
	v.addPointer(event)
}

// HandleEvent processes pointer events for drag detection.
func (v *VerticalDragGestureRecognizer) HandleEvent(event PointerEvent) {
	v.handleEvent(event)
}

// AcceptGesture is called by the arena when this recognizer wins.
func (v *VerticalDragGestureRecognizer) AcceptGesture(pointerID int64) {
	v.acceptGesture(pointerID)
}

// RejectGesture is called by the arena when this recognizer loses.
func (v *VerticalDragGestureRecognizer) RejectGesture(pointerID int64) {
	v.rejectGesture(pointerID)
}

// Dispose releases resources for the recognizer.
func (v *VerticalDragGestureRecognizer) Dispose() {}
