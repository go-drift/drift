package widgets

import (
	"math"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// ScrollView provides scrollable content in a single direction.
//
// ScrollView wraps a single child widget and enables scrolling when the child
// exceeds the viewport. It supports both vertical (default) and horizontal
// scrolling via ScrollDirection.
//
// # Scroll Physics
//
// The Physics field controls scroll behavior:
//   - [ClampingScrollPhysics] (default): Stops at edges, no overscroll
//   - [BouncingScrollPhysics]: iOS-style bounce effect at edges
//
// # Scroll Controller
//
// Use a [ScrollController] to programmatically control or observe scroll position:
//
//	controller := &widgets.ScrollController{}
//	controller.AddListener(func() {
//	    fmt.Println("Offset:", controller.Offset())
//	})
//
// # Safe Area Handling
//
// Use [SafeAreaPadding] for proper inset handling on devices with notches:
//
//	ScrollView{
//	    Padding: widgets.SafeAreaPadding(ctx).Add(24),
//	    ChildWidget: content,
//	}
//
// For scrollable lists, consider [ListView] or [ListViewBuilder] which provide
// additional features like item-based layout and virtualization.
type ScrollView struct {
	ChildWidget core.Widget
	// ScrollDirection is the axis along which the view scrolls.
	// Defaults to AxisVertical (the zero value).
	ScrollDirection Axis
	Controller      *ScrollController
	Physics         ScrollPhysics
	Padding         layout.EdgeInsets
}

func (s ScrollView) CreateElement() core.Element {
	return core.NewStatelessElement(s, nil)
}

func (s ScrollView) Key() any {
	return nil
}

func (s ScrollView) Build(ctx core.BuildContext) core.Widget {
	child := s.ChildWidget
	if s.Padding != (layout.EdgeInsets{}) {
		child = Padding{
			Padding:     s.Padding,
			ChildWidget: child,
		}
	}

	return scrollViewCore{
		ChildWidget:     child,
		ScrollDirection: s.ScrollDirection,
		Controller:      s.Controller,
		Physics:         s.Physics,
	}
}

// scrollViewCore is the internal render object widget for ScrollView.
type scrollViewCore struct {
	ChildWidget     core.Widget
	ScrollDirection Axis
	Controller      *ScrollController
	Physics         ScrollPhysics
}

func (s scrollViewCore) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s scrollViewCore) Key() any {
	return nil
}

func (s scrollViewCore) Child() core.Widget {
	return s.ChildWidget
}

func (s scrollViewCore) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	controller := s.Controller
	if controller == nil {
		controller = &ScrollController{}
	}
	physics := s.Physics
	if physics == nil {
		physics = ClampingScrollPhysics{}
	}
	scroll := &renderScrollView{
		direction:  s.ScrollDirection,
		controller: controller,
		physics:    physics,
	}
	scroll.SetSelf(scroll)
	scroll.position = NewScrollPosition(controller, physics, func() {
		scroll.MarkNeedsPaint()
		scroll.MarkNeedsSemanticsUpdate()
	})
	scroll.configurePan()
	return scroll
}

func (s scrollViewCore) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if scroll, ok := renderObject.(*renderScrollView); ok {
		scroll.direction = s.ScrollDirection
		scroll.updateController(s.Controller)
		scroll.updatePhysics(s.Physics)
		scroll.configurePan()
		scroll.MarkNeedsLayout()
		scroll.MarkNeedsPaint()
	}
}

type renderScrollView struct {
	layout.RenderBoxBase
	child          layout.RenderBox
	direction      Axis
	controller     *ScrollController
	physics        ScrollPhysics
	position       *ScrollPosition
	horizontalDrag *gestures.HorizontalDragGestureRecognizer
	verticalDrag   *gestures.VerticalDragGestureRecognizer
}

// IsRepaintBoundary returns true - scrolling content benefits from isolation.
func (r *renderScrollView) IsRepaintBoundary() bool {
	return true
}

func (r *renderScrollView) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		setParentOnChild(r.child, r)
	}
}

func (r *renderScrollView) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderScrollView) PerformLayout() {
	constraints := r.Constraints()
	size := graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight}
	if size.Width <= 0 {
		size.Width = constraints.MinWidth
	}
	if size.Height <= 0 {
		size.Height = constraints.MinHeight
	}
	r.SetSize(size)
	if r.controller != nil {
		viewport := size.Height
		if r.direction == AxisHorizontal {
			viewport = size.Width
		}
		r.controller.setViewportExtent(viewport)
	}
	if r.child != nil {
		childConstraints := layout.Constraints{
			MinWidth:  size.Width,
			MaxWidth:  size.Width,
			MinHeight: 0,
			MaxHeight: math.MaxFloat64,
		}
		if r.direction == AxisHorizontal {
			childConstraints = layout.Constraints{
				MinWidth:  0,
				MaxWidth:  math.MaxFloat64,
				MinHeight: size.Height,
				MaxHeight: size.Height,
			}
		}
		r.child.Layout(childConstraints, true) // true: we read child.Size() for scroll extents
		r.child.SetParentData(&layout.BoxParentData{})
	}
	r.updateExtents()
}

func (r *renderScrollView) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	size := r.Size()
	clipRect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)

	ctx.Canvas.Save()
	ctx.Canvas.ClipRect(clipRect)

	// Push clip BEFORE scroll translation (clip is viewport-relative)
	ctx.PushClipRect(clipRect)

	offset := r.scrollOffset()
	if r.direction == AxisHorizontal {
		ctx.Canvas.Translate(-offset, 0)
		ctx.PushTranslation(-offset, 0)
	} else {
		ctx.Canvas.Translate(0, -offset)
		ctx.PushTranslation(0, -offset)
	}

	if !r.paintCulled(ctx, size, offset) {
		r.child.Paint(ctx)
	}

	// Pop in reverse order
	ctx.PopTranslation()
	ctx.PopClipRect()
	ctx.Canvas.Restore()
}

func (r *renderScrollView) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	size := r.Size()
	if position.X < 0 || position.Y < 0 || position.X > size.Width || position.Y > size.Height {
		return false
	}
	if r.child != nil {
		local := position
		offset := r.scrollOffset()
		if r.direction == AxisHorizontal {
			local.X += offset
		} else {
			local.Y += offset
		}
		if r.child.HitTest(local, result) {
			result.Add(r)
			return true
		}
	}
	result.Add(r)
	return true
}

func (r *renderScrollView) HandlePointer(event gestures.PointerEvent) {
	var recognizer interface {
		AddPointer(gestures.PointerEvent)
		HandleEvent(gestures.PointerEvent)
	}
	if r.direction == AxisHorizontal {
		recognizer = r.horizontalDrag
	} else {
		recognizer = r.verticalDrag
	}
	if recognizer == nil {
		return
	}
	switch event.Phase {
	case gestures.PointerPhaseDown:
		if r.position != nil {
			r.position.StopBallistic()
		}
		recognizer.AddPointer(event)
	default:
		recognizer.HandleEvent(event)
	}
}

func (r *renderScrollView) configurePan() {
	r.configureDrag()
}

func (r *renderScrollView) configureDrag() {
	onStart := func(details gestures.DragStartDetails) {
		if r.position != nil {
			r.position.StopBallistic()
		}
	}
	onUpdate := func(details gestures.DragUpdateDetails) {
		if r.position == nil {
			return
		}
		r.position.ApplyUserOffset(-details.PrimaryDelta)
	}
	onEnd := func(details gestures.DragEndDetails) {
		if r.position == nil {
			return
		}
		r.position.StartBallistic(-details.PrimaryVelocity)
	}
	onCancel := func() {
		if r.position != nil {
			r.position.StopBallistic()
		}
	}

	if r.direction == AxisHorizontal {
		if r.horizontalDrag == nil {
			r.horizontalDrag = gestures.NewHorizontalDragGestureRecognizer(gestures.DefaultArena)
		}
		r.horizontalDrag.OnStart = onStart
		r.horizontalDrag.OnUpdate = onUpdate
		r.horizontalDrag.OnEnd = onEnd
		r.horizontalDrag.OnCancel = onCancel
		// Dispose vertical if direction changed
		if r.verticalDrag != nil {
			r.verticalDrag.Dispose()
			r.verticalDrag = nil
		}
	} else {
		if r.verticalDrag == nil {
			r.verticalDrag = gestures.NewVerticalDragGestureRecognizer(gestures.DefaultArena)
		}
		r.verticalDrag.OnStart = onStart
		r.verticalDrag.OnUpdate = onUpdate
		r.verticalDrag.OnEnd = onEnd
		r.verticalDrag.OnCancel = onCancel
		// Dispose horizontal if direction changed
		if r.horizontalDrag != nil {
			r.horizontalDrag.Dispose()
			r.horizontalDrag = nil
		}
	}
}

func (r *renderScrollView) updateController(controller *ScrollController) {
	if controller == nil {
		return
	}
	if r.controller == controller {
		return
	}
	if r.controller != nil && r.position != nil {
		r.controller.detach(r.position)
	}
	r.controller = controller
	if r.position != nil {
		r.position.controller = controller
		r.controller.attach(r.position)
	}
}

func (r *renderScrollView) updatePhysics(physics ScrollPhysics) {
	if physics == nil {
		return
	}
	r.physics = physics
	if r.position != nil {
		r.position.physics = physics
	}
}

func (r *renderScrollView) updateExtents() {
	if r.position == nil {
		return
	}
	size := r.Size()
	content := 0.0
	if r.child != nil {
		childSize := r.child.Size()
		if r.direction == AxisHorizontal {
			content = childSize.Width
		} else {
			content = childSize.Height
		}
	}
	viewport := size.Height
	if r.direction == AxisHorizontal {
		viewport = size.Width
	}
	max := content - viewport
	if max < 0 {
		max = 0
	}
	r.position.SetExtents(0, max)
}

func (r *renderScrollView) scrollOffset() float64 {
	if r.position == nil {
		return 0
	}
	return r.position.Offset()
}

func (r *renderScrollView) ScrollOffset() graphics.Offset {
	offset := r.scrollOffset()
	if r.direction == AxisHorizontal {
		return graphics.Offset{X: -offset}
	}
	return graphics.Offset{Y: -offset}
}

// SemanticScrollOffset implements layout.ScrollOffsetProvider.
// Returns the scroll offset to subtract from child positions in the semantics tree.
func (r *renderScrollView) SemanticScrollOffset() graphics.Offset {
	offset := r.scrollOffset()
	if r.direction == AxisHorizontal {
		return graphics.Offset{X: offset}
	}
	return graphics.Offset{Y: offset}
}

func (r *renderScrollView) paintCulled(ctx *layout.PaintContext, size graphics.Size, scrollOffset float64) bool {
	if flex, ok := r.child.(*renderFlex); ok {
		r.paintFlex(ctx, flex, graphics.Offset{}, size, scrollOffset)
		return true
	}
	if padding, ok := r.child.(*renderPadding); ok {
		if flex, ok := padding.child.(*renderFlex); ok {
			contentOffset := graphics.Offset{X: padding.padding.Left, Y: padding.padding.Top}
			r.paintFlex(ctx, flex, contentOffset, size, scrollOffset)
			return true
		}
	}
	return false
}

func (r *renderScrollView) paintFlex(
	ctx *layout.PaintContext,
	flex *renderFlex,
	contentOffset graphics.Offset,
	size graphics.Size,
	scrollOffset float64,
) {
	viewportSize := size.Height
	if r.direction == AxisHorizontal {
		viewportSize = size.Width
	}
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + viewportSize

	for _, child := range flex.children {
		parentData, _ := child.ParentData().(*layout.BoxParentData)
		offset := contentOffset
		if parentData != nil {
			offset = graphics.Offset{
				X: contentOffset.X + parentData.Offset.X,
				Y: contentOffset.Y + parentData.Offset.Y,
			}
		}
		childSize := child.Size()
		var childStart, childEnd float64
		if r.direction == AxisHorizontal {
			childStart = offset.X
			childEnd = offset.X + childSize.Width
		} else {
			childStart = offset.Y
			childEnd = offset.Y + childSize.Height
		}
		if childEnd < visibleStart || childStart > visibleEnd {
			continue
		}
		ctx.PaintChildWithLayer(child, offset)
	}
}

// ScrollController controls scroll position.
type ScrollController struct {
	InitialScrollOffset float64
	positions           []*ScrollPosition
	viewportExtent      float64
	listeners           map[int]func()
	nextListenerID      int
}

// Offset returns the current scroll offset.
func (c *ScrollController) Offset() float64 {
	if len(c.positions) > 0 {
		return c.positions[0].Offset()
	}
	return c.InitialScrollOffset
}

// ViewportExtent returns the current viewport extent.
func (c *ScrollController) ViewportExtent() float64 {
	return c.viewportExtent
}

// AddListener registers a callback for scroll changes.
func (c *ScrollController) AddListener(listener func()) func() {
	if listener == nil {
		return func() {}
	}
	if c.listeners == nil {
		c.listeners = make(map[int]func())
	}
	id := c.nextListenerID
	c.nextListenerID++
	c.listeners[id] = listener
	return func() {
		delete(c.listeners, id)
	}
}

// JumpTo moves all attached positions to a new offset.
func (c *ScrollController) JumpTo(offset float64) {
	c.InitialScrollOffset = offset
	if len(c.positions) == 0 {
		c.notifyListeners()
		return
	}
	for _, position := range c.positions {
		position.SetOffset(offset)
	}
}

// AnimateTo moves to a new offset immediately (placeholder for animations).
func (c *ScrollController) AnimateTo(offset float64, _ time.Duration) {
	c.JumpTo(offset)
}

func (c *ScrollController) attach(position *ScrollPosition) {
	for _, existing := range c.positions {
		if existing == position {
			return
		}
	}
	c.positions = append(c.positions, position)
}

func (c *ScrollController) detach(position *ScrollPosition) {
	for i, existing := range c.positions {
		if existing == position {
			c.positions = append(c.positions[:i], c.positions[i+1:]...)
			return
		}
	}
}

func (c *ScrollController) setViewportExtent(extent float64) {
	if extent == c.viewportExtent {
		return
	}
	c.viewportExtent = extent
	c.notifyListeners()
}

func (c *ScrollController) notifyListeners() {
	for _, listener := range c.listeners {
		listener()
	}
}

// ScrollPosition stores the current scroll offset and extents.
type ScrollPosition struct {
	offset     float64
	min        float64
	max        float64
	physics    ScrollPhysics
	onUpdate   func()
	controller *ScrollController
	ballistic  *ballisticState
}

// NewScrollPosition creates a new scroll position.
func NewScrollPosition(controller *ScrollController, physics ScrollPhysics, onUpdate func()) *ScrollPosition {
	if physics == nil {
		physics = ClampingScrollPhysics{}
	}
	position := &ScrollPosition{
		offset:     0,
		physics:    physics,
		onUpdate:   onUpdate,
		controller: controller,
	}
	if controller != nil {
		position.offset = controller.InitialScrollOffset
		controller.attach(position)
	}
	return position
}

// Offset returns the current scroll offset.
func (p *ScrollPosition) Offset() float64 {
	return p.offset
}

// SetOffset updates the scroll offset.
func (p *ScrollPosition) SetOffset(value float64) {
	allowOverscroll := isBouncing(p.physics)
	clamped := p.clampOffset(value, allowOverscroll)
	if clamped == p.offset {
		return
	}
	p.offset = clamped
	p.notify()
}

// SetExtents updates the min/max scroll extents.
func (p *ScrollPosition) SetExtents(min, max float64) {
	if max < min {
		max = min
	}
	p.min = min
	p.max = max
	p.SetOffset(p.offset)
}

// ApplyUserOffset applies a drag delta with physics.
func (p *ScrollPosition) ApplyUserOffset(delta float64) {
	p.StopBallistic()
	if p.physics == nil {
		p.SetOffset(p.offset + delta)
		return
	}
	adjusted := p.physics.ApplyPhysicsToUserOffset(p, delta)
	proposed := p.offset + adjusted
	overscroll := p.physics.ApplyBoundaryConditions(p, proposed)
	proposed -= overscroll
	p.SetOffset(proposed)
}

// StartBallistic begins inertial scrolling with the provided velocity.
func (p *ScrollPosition) StartBallistic(velocity float64) {
	p.StopBallistic()
	// Always animate back when overscrolled (iOS-style spring)
	if isOverscrolled(p) {
		p.ballistic = newBallisticState(p, velocity)
		registerBallistic(p)
		p.notify()
		return
	}
	if math.Abs(velocity) < 5 {
		return
	}
	p.ballistic = newBallisticState(p, velocity)
	registerBallistic(p)
	p.notify()
}

// StopBallistic halts any ongoing inertial scroll.
func (p *ScrollPosition) StopBallistic() {
	if p.ballistic != nil {
		unregisterBallistic(p)
		p.ballistic = nil
	}
}

func (p *ScrollPosition) notify() {
	if p.onUpdate != nil {
		p.onUpdate()
	}
	if p.controller != nil {
		p.controller.notifyListeners()
	}
}

// ScrollPhysics determines scroll behavior.
type ScrollPhysics interface {
	ApplyPhysicsToUserOffset(position *ScrollPosition, offset float64) float64
	ApplyBoundaryConditions(position *ScrollPosition, value float64) float64
}

// ClampingScrollPhysics clamps at edges (Android default).
type ClampingScrollPhysics struct{}

// ApplyPhysicsToUserOffset returns the raw delta for clamping physics.
func (ClampingScrollPhysics) ApplyPhysicsToUserOffset(_ *ScrollPosition, offset float64) float64 {
	return offset
}

// ApplyBoundaryConditions clamps at the min/max extents.
func (ClampingScrollPhysics) ApplyBoundaryConditions(position *ScrollPosition, value float64) float64 {
	if value < position.min {
		return value - position.min
	}
	if value > position.max {
		return value - position.max
	}
	return 0
}

// BouncingScrollPhysics adds resistance near edges.
type BouncingScrollPhysics struct{}

// ApplyPhysicsToUserOffset reduces delta when overscrolling.
func (BouncingScrollPhysics) ApplyPhysicsToUserOffset(position *ScrollPosition, offset float64) float64 {
	if (position.offset <= position.min && offset < 0) || (position.offset >= position.max && offset > 0) {
		return offset * 0.5
	}
	return offset
}

// ApplyBoundaryConditions still clamps to avoid runaway offsets.
func (BouncingScrollPhysics) ApplyBoundaryConditions(position *ScrollPosition, value float64) float64 {
	return 0
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (p *ScrollPosition) clampOffset(value float64, allowOverscroll bool) float64 {
	if !allowOverscroll {
		return clamp(value, p.min, p.max)
	}
	limit := 120.0
	return clamp(value, p.min-limit, p.max+limit)
}

func isBouncing(physics ScrollPhysics) bool {
	switch physics.(type) {
	case BouncingScrollPhysics:
		return true
	default:
		return false
	}
}

func isOverscrolled(position *ScrollPosition) bool {
	return position.offset < position.min || position.offset > position.max
}

type ballisticState struct {
	position *ScrollPosition
	velocity float64
	lastTime time.Time
	spring   *animation.SpringSimulation
}

func newBallisticState(position *ScrollPosition, velocity float64) *ballisticState {
	b := &ballisticState{
		position: position,
		velocity: velocity,
		lastTime: animation.Now(),
	}
	// If overscrolled, create spring simulation immediately
	if isOverscrolled(position) && isBouncing(position.physics) {
		b.initSpring()
	}
	return b
}

func (b *ballisticState) initSpring() {
	pos := b.position
	var target float64
	if pos.offset < pos.min {
		target = pos.min
	} else {
		target = pos.max
	}
	b.spring = animation.NewSpringSimulation(
		animation.IOSSpring(),
		pos.offset,
		b.velocity,
		target,
	)
}

func (b *ballisticState) step(now time.Time) bool {
	if now.Before(b.lastTime) {
		b.lastTime = now
		return false
	}
	dt := now.Sub(b.lastTime).Seconds()
	b.lastTime = now
	if dt <= 0 {
		return false
	}
	// Cap dt to avoid large jumps on first frame or after stalls.
	// This prevents the animation from "catching up" all at once.
	const maxDt = 0.032 // ~30fps, allows smooth animation even with some lag
	if dt > maxDt {
		dt = maxDt
	}
	return b.advance(dt)
}

func (b *ballisticState) advance(dt float64) bool {
	if dt <= 0 {
		return false
	}
	pos := b.position
	velocity := b.velocity
	offset := pos.offset
	overscrolled := offset < pos.min || offset > pos.max

	// Use spring simulation for bounce-back (iOS-style)
	if b.spring != nil {
		done := b.spring.Step(dt)
		pos.offset = b.spring.Position()
		b.velocity = b.spring.Velocity()
		pos.notify()
		return done
	}

	// Check if we need to start spring animation (crossed boundary during fling)
	if overscrolled && isBouncing(pos.physics) {
		b.initSpring()
		done := b.spring.Step(dt)
		pos.offset = b.spring.Position()
		b.velocity = b.spring.Velocity()
		pos.notify()
		return done
	}

	// Normal deceleration when not overscrolled
	const decel = 2400.0
	if velocity > 0 {
		velocity -= decel * dt
		if velocity < 0 {
			velocity = 0
		}
	} else if velocity < 0 {
		velocity += decel * dt
		if velocity > 0 {
			velocity = 0
		}
	}
	offset += velocity * dt

	b.velocity = velocity
	pos.offset = pos.clampOffset(offset, isBouncing(pos.physics))
	pos.notify()

	if math.Abs(velocity) < 5 {
		return true
	}
	return false
}

var ballisticMu sync.Mutex
var ballisticPositions = make(map[*ScrollPosition]struct{})

func registerBallistic(position *ScrollPosition) {
	ballisticMu.Lock()
	ballisticPositions[position] = struct{}{}
	ballisticMu.Unlock()
}

func unregisterBallistic(position *ScrollPosition) {
	ballisticMu.Lock()
	delete(ballisticPositions, position)
	ballisticMu.Unlock()
}

// HasActiveBallistics returns true if any scroll simulations are running.
func HasActiveBallistics() bool {
	ballisticMu.Lock()
	defer ballisticMu.Unlock()
	return len(ballisticPositions) > 0
}

// StepBallistics advances any active scroll simulations.
func StepBallistics() {
	ballisticMu.Lock()
	if len(ballisticPositions) == 0 {
		ballisticMu.Unlock()
		return
	}
	now := animation.Now()
	positions := make([]*ScrollPosition, 0, len(ballisticPositions))
	for position := range ballisticPositions {
		positions = append(positions, position)
	}
	ballisticMu.Unlock()

	for _, position := range positions {
		if position.ballistic == nil {
			continue
		}
		if position.ballistic.step(now) {
			position.StopBallistic()
		}
	}
}
