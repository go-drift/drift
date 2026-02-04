package widgets

import (
	"math"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// SnapPoint defines a height position where a bottom sheet can snap.
// FractionalHeight is relative to available height (screen height minus safe area insets).
type SnapPoint struct {
	FractionalHeight float64
	Name             string
}

// Common snap point presets.
var (
	SnapThird = SnapPoint{FractionalHeight: 0.33, Name: "third"}
	SnapHalf  = SnapPoint{FractionalHeight: 0.5, Name: "half"}
	SnapFull  = SnapPoint{FractionalHeight: 1.0, Name: "full"}
)

// DefaultSnapPoints is used when no snap points are provided.
var DefaultSnapPoints = []SnapPoint{SnapFull}

// NormalizeSnapPoints validates and normalizes snap points.
// It clamps values to [0.1, 1.0], sorts ascending, and removes duplicates.
func NormalizeSnapPoints(points []SnapPoint) []SnapPoint {
	if len(points) == 0 {
		return DefaultSnapPoints
	}
	result := make([]SnapPoint, 0, len(points))
	for _, p := range points {
		p.FractionalHeight = clampFloat(p.FractionalHeight, 0.1, 1.0)
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FractionalHeight < result[j].FractionalHeight
	})
	return dedupByHeight(result)
}

// dedupByHeight removes snap points with duplicate fractional heights,
// keeping only the first occurrence of each height.
func dedupByHeight(points []SnapPoint) []SnapPoint {
	if len(points) == 0 {
		return points
	}
	seen := make(map[float64]bool)
	result := make([]SnapPoint, 0, len(points))
	for _, p := range points {
		if !seen[p.FractionalHeight] {
			seen[p.FractionalHeight] = true
			result = append(result, p)
		}
	}
	return result
}

// ValidateInitialSnap ensures the initial snap index is valid.
func ValidateInitialSnap(index int, points []SnapPoint) int {
	if index < 0 || index >= len(points) {
		return 0
	}
	return index
}

// clampFloat constrains v to the range [min, max].
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// DragMode controls how drag gestures interact with bottom sheet content.
type DragMode int

const (
	// DragModeAuto picks DragModeContentAware for multi-snap sheets and DragModeSheet otherwise.
	DragModeAuto DragMode = iota
	// DragModeContentAware coordinates drags with scrollable content when possible.
	DragModeContentAware
	// DragModeSheet drags the entire sheet regardless of content.
	DragModeSheet
	// DragModeHandleOnly only accepts drags from the handle area.
	DragModeHandleOnly
)

// SnapBehavior configures snapping and dismissal thresholds.
type SnapBehavior struct {
	// DismissFactor is the fraction of the minimum snap height below which a drag-down can dismiss.
	DismissFactor float64
	// MinFlingVelocity is the velocity (px/s) that triggers fast dismiss when dragging downward.
	MinFlingVelocity float64
	// SnapVelocityThreshold is the velocity (px/s) above which we snap in the direction of travel.
	SnapVelocityThreshold float64
}

// DefaultSnapBehavior returns recommended defaults.
func DefaultSnapBehavior() SnapBehavior {
	return SnapBehavior{
		DismissFactor:         0.5,
		MinFlingVelocity:      1200,
		SnapVelocityThreshold: 400,
	}
}

// normalizeSnapBehavior fills in zero values with defaults.
func normalizeSnapBehavior(value SnapBehavior) SnapBehavior {
	defaults := DefaultSnapBehavior()
	if value.DismissFactor <= 0 {
		value.DismissFactor = defaults.DismissFactor
	}
	if value.MinFlingVelocity <= 0 {
		value.MinFlingVelocity = defaults.MinFlingVelocity
	}
	if value.SnapVelocityThreshold <= 0 {
		value.SnapVelocityThreshold = defaults.SnapVelocityThreshold
	}
	return value
}

// BottomSheetController controls a bottom sheet's behavior.
// Create with NewBottomSheetController and pass to BottomSheet widget.
// Content inside the sheet can access the controller via BottomSheetScope.Of(ctx).
type BottomSheetController struct {
	mu sync.Mutex

	dismissFunc        func(any)
	snapToIndexFunc    func(int)
	snapToFractionFunc func(float64)
	extentFunc         func() float64
	registerScrollable func(*ScrollController) func()

	dismissCalled bool
	pendingResult any

	progress     float64
	listeners    map[int]func(float64)
	nextListener int
}

// NewBottomSheetController creates a new controller for a bottom sheet.
func NewBottomSheetController() *BottomSheetController {
	return &BottomSheetController{}
}

// Close triggers the sheet's dismiss animation with the given result.
// The result will be passed to OnDismiss when animation completes.
// Safe to call multiple times - only the first call has effect.
func (c *BottomSheetController) Close(result any) {
	c.mu.Lock()
	if c.dismissCalled {
		c.mu.Unlock()
		return
	}
	c.dismissCalled = true
	dismissFunc := c.dismissFunc
	c.pendingResult = result
	c.mu.Unlock()

	if dismissFunc != nil {
		dismissFunc(result)
	}
}

// SnapTo animates the sheet to the snap point at the given index.
func (c *BottomSheetController) SnapTo(index int) {
	c.mu.Lock()
	snapFunc := c.snapToIndexFunc
	c.mu.Unlock()
	if snapFunc != nil {
		snapFunc(index)
	}
}

// SnapToFraction animates the sheet to a fractional snap point.
// The value is clamped to [0, 1].
func (c *BottomSheetController) SnapToFraction(fraction float64) {
	c.mu.Lock()
	snapFunc := c.snapToFractionFunc
	c.mu.Unlock()
	if snapFunc != nil {
		snapFunc(fraction)
	}
}

// Extent returns the current sheet extent in pixels.
func (c *BottomSheetController) Extent() float64 {
	c.mu.Lock()
	extentFunc := c.extentFunc
	c.mu.Unlock()
	if extentFunc == nil {
		return 0
	}
	return extentFunc()
}

// Progress returns the current open progress from 0.0 to 1.0.
func (c *BottomSheetController) Progress() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.progress
}

// AddProgressListener registers a callback for progress changes.
func (c *BottomSheetController) AddProgressListener(listener func(float64)) func() {
	if listener == nil {
		return func() {}
	}
	c.mu.Lock()
	if c.listeners == nil {
		c.listeners = make(map[int]func(float64))
	}
	id := c.nextListener
	c.nextListener++
	c.listeners[id] = listener
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		delete(c.listeners, id)
		c.mu.Unlock()
	}
}

// attach binds the controller to a bottom sheet's state callbacks.
// Called by bottomSheetState when initializing or when the controller changes.
// If Close was called before attach, the pending dismiss is executed immediately.
func (c *BottomSheetController) attach(
	dismiss func(any),
	snapToIndex func(int),
	snapToFraction func(float64),
	extent func() float64,
	registerScrollable func(*ScrollController) func(),
) {
	c.mu.Lock()
	c.dismissFunc = dismiss
	c.snapToIndexFunc = snapToIndex
	c.snapToFractionFunc = snapToFraction
	c.extentFunc = extent
	c.registerScrollable = registerScrollable
	dismissCalled := c.dismissCalled
	pending := c.pendingResult
	c.mu.Unlock()

	// Handle early-dismiss: Close() was called before the sheet was ready
	if dismissCalled && dismiss != nil {
		dismiss(pending)
	}
}

// detach unbinds the controller from the sheet's state callbacks.
// Called when the sheet is disposed or the controller is replaced.
func (c *BottomSheetController) detach() {
	c.mu.Lock()
	c.dismissFunc = nil
	c.snapToIndexFunc = nil
	c.snapToFractionFunc = nil
	c.extentFunc = nil
	c.registerScrollable = nil
	c.mu.Unlock()
}

// setProgress updates the open progress and notifies all listeners.
// Called by the sheet state during animations and drag updates.
func (c *BottomSheetController) setProgress(value float64) {
	value = clampFloat(value, 0, 1)
	c.mu.Lock()
	if c.progress == value {
		c.mu.Unlock()
		return
	}
	c.progress = value
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(float64), 0, len(c.listeners))
	for _, listener := range c.listeners {
		listeners = append(listeners, listener)
	}
	c.mu.Unlock()
	for _, listener := range listeners {
		listener(value)
	}
}

func (c *BottomSheetController) registerScrollableInternal(controller *ScrollController) func() {
	c.mu.Lock()
	register := c.registerScrollable
	c.mu.Unlock()
	if register == nil || controller == nil {
		return func() {}
	}
	return register(controller)
}

// BottomSheetTheme holds theming data for a bottom sheet.
// This is passed from the route which has access to the theme system.
type BottomSheetTheme struct {
	BackgroundColor     graphics.Color
	HandleColor         graphics.Color
	BorderRadius        float64
	HandleWidth         float64
	HandleHeight        float64
	HandleTopPadding    float64
	HandleBottomPadding float64
}

// DefaultBottomSheetTheme returns default theme values.
func DefaultBottomSheetTheme() BottomSheetTheme {
	return BottomSheetTheme{
		BackgroundColor:     graphics.ColorWhite,
		HandleColor:         graphics.RGBA(200, 200, 200, 255),
		BorderRadius:        16,
		HandleWidth:         32,
		HandleHeight:        4,
		HandleTopPadding:    8,
		HandleBottomPadding: 8,
	}
}

// BottomSheet is the widget for rendering a bottom sheet with animations and gestures.
// Use BottomSheetController to programmatically dismiss or snap the sheet.
type BottomSheet struct {
	// Builder creates the sheet content.
	Builder func(ctx core.BuildContext) core.Widget
	// Controller allows programmatic control of the sheet.
	Controller *BottomSheetController
	// SnapPoints defines the snap positions as fractions of available height.
	// When empty, the sheet sizes to content height.
	SnapPoints []SnapPoint
	// InitialSnap is the index into SnapPoints to open at.
	InitialSnap int

	// EnableDrag toggles drag gestures for the sheet.
	EnableDrag bool
	// DragMode defines how drag gestures interact with content.
	DragMode DragMode
	// ShowHandle displays the drag handle at the top of the sheet.
	ShowHandle bool
	// UseSafeArea applies safe area insets to the sheet layout.
	UseSafeArea bool

	// Theme configures the sheet's visual styling.
	Theme BottomSheetTheme
	// SnapBehavior customizes snapping and dismiss thresholds.
	SnapBehavior SnapBehavior
	// OnDismiss fires after the sheet finishes its dismiss animation.
	OnDismiss func(result any)
}

func (b BottomSheet) CreateElement() core.Element {
	return core.NewStatefulElement(b, nil)
}

func (b BottomSheet) Key() any {
	return nil
}

func (b BottomSheet) CreateState() core.State {
	return &bottomSheetState{}
}

// bottomSheetState manages the runtime state for a BottomSheet widget.
// It handles animations, drag gestures, snap point calculations, and
// coordinates with scrollable content for content-aware dragging.
type bottomSheetState struct {
	core.StateBase

	// Configuration from widget
	snapPoints   []SnapPoint // normalized snap points (sorted, deduped)
	snapHeights  []float64   // snap points converted to pixel heights
	initialIndex int         // which snap point to open at
	contentSized bool        // true if sheet sizes to content (no snap points provided)

	enableDrag  bool
	dragMode    DragMode
	showHandle  bool
	useSafeArea bool

	theme        BottomSheetTheme
	snapBehavior SnapBehavior
	onDismiss    func(any)
	controller   *BottomSheetController
	builder      func(core.BuildContext) core.Widget

	// Layout metrics from positioner callback
	metricsReady    bool    // true after first layout pass
	availableHeight float64 // screen height minus safe area insets
	screenHeight    float64
	topInset        float64
	bottomInset     float64

	// Animation state
	currentExtent float64 // current sheet height in pixels (animated)
	targetExtent  float64 // target sheet height for animations
	isDragging    bool    // true while user is actively dragging
	isDismissing  bool    // true while dismiss animation is running

	spring *animation.SpringSimulation // physics simulation for snapping
	ticker *animation.Ticker           // drives animation frames

	// Content-aware drag coordination
	scrollController *ScrollController // registered scrollable's controller
	scrollOffset     float64           // current scroll position (for drag decisions)
	scrollRemove     func()            // cleanup function for scroll listener
}

func (s *bottomSheetState) InitState() {
	w := s.Element().Widget().(BottomSheet)
	s.applyWidget(w)

	if s.controller != nil {
		s.controller.attach(
			s.requestDismiss,
			s.snapToIndex,
			s.snapToFraction,
			s.currentExtentPx,
			s.registerScrollable,
		)
	}
}

func (s *bottomSheetState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	w := s.Element().Widget().(BottomSheet)
	old := oldWidget.(BottomSheet)
	s.applyWidget(w)

	if w.Controller != old.Controller {
		if old.Controller != nil {
			old.Controller.detach()
		}
		if s.controller != nil {
			s.controller.attach(
				s.requestDismiss,
				s.snapToIndex,
				s.snapToFraction,
				s.currentExtentPx,
				s.registerScrollable,
			)
		}
	}
}

func (s *bottomSheetState) applyWidget(w BottomSheet) {
	s.contentSized = len(w.SnapPoints) == 0
	if s.contentSized {
		s.snapPoints = nil
		s.initialIndex = 0
	} else {
		s.snapPoints = NormalizeSnapPoints(w.SnapPoints)
		s.initialIndex = ValidateInitialSnap(w.InitialSnap, s.snapPoints)
	}

	s.enableDrag = w.EnableDrag
	s.dragMode = w.DragMode
	s.showHandle = w.ShowHandle
	s.useSafeArea = w.UseSafeArea
	s.theme = w.Theme
	s.snapBehavior = normalizeSnapBehavior(w.SnapBehavior)
	s.onDismiss = w.OnDismiss
	s.controller = w.Controller
	s.builder = w.Builder
}

func (s *bottomSheetState) Dispose() {
	if s.controller != nil {
		s.controller.detach()
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.scrollRemove != nil {
		s.scrollRemove()
		s.scrollRemove = nil
	}
}

func (s *bottomSheetState) Build(ctx core.BuildContext) core.Widget {
	var content core.Widget
	if s.controller != nil {
		content = bottomSheetScopeBuilder{
			controller: s.controller,
			builder:    s.builder,
		}
	} else if s.builder != nil {
		content = s.builder(ctx)
	}

	var handleWidget core.Widget = SizedBox{}
	if s.showHandle {
		handleWidget = s.buildHandle()
	}

	dragMode := s.resolveDragMode()
	if s.enableDrag && dragMode == DragModeHandleOnly && s.showHandle {
		handleWidget = sheetDragRegion{
			Child:       handleWidget,
			ShouldStart: s.shouldStartHandleDrag,
			OnStart:     s.onDragStart,
			OnUpdate:    s.onDragUpdate,
			OnEnd:       s.onDragEnd,
		}
	}

	var topInset, bottomInset float64
	if s.useSafeArea {
		topInset = SafeAreaTopOf(ctx)
		bottomInset = SafeAreaBottomOf(ctx)
	}

	body := bottomSheetBody{
		Handle:       handleWidget,
		Content:      content,
		BottomInset:  bottomInset,
		Background:   s.theme.BackgroundColor,
		BorderRadius: s.theme.BorderRadius,
		ContentSized: s.contentSized,
	}

	var sheet core.Widget = body
	if s.enableDrag && dragMode != DragModeHandleOnly {
		sheet = sheetDragRegion{
			Child:       body,
			ShouldStart: s.shouldStartDrag,
			OnStart:     s.onDragStart,
			OnUpdate:    s.onDragUpdate,
			OnEnd:       s.onDragEnd,
		}
	}

	return bottomSheetPositioner{
		Extent:       s.currentExtent,
		TopInset:     topInset,
		BottomInset:  bottomInset,
		ContentSized: s.contentSized,
		Child:        sheet,
		OnMetrics:    s.onMetrics,
	}
}

// resolveDragMode converts DragModeAuto to a concrete mode based on configuration.
// Multi-snap sheets default to content-aware dragging; single-snap sheets default to sheet dragging.
func (s *bottomSheetState) resolveDragMode() DragMode {
	if s.dragMode == DragModeAuto {
		if len(s.snapPoints) > 1 {
			return DragModeContentAware
		}
		return DragModeSheet
	}
	return s.dragMode
}

// onMetrics is called by the positioner after layout with current screen dimensions.
// It converts fractional snap points to pixel heights and triggers the opening animation
// on first layout, or rescales extents when the available height changes (e.g., rotation).
func (s *bottomSheetState) onMetrics(metrics sheetMetrics) {
	if metrics.AvailableHeight <= 0 {
		return
	}

	prevAvailable := s.availableHeight
	s.metricsReady = true
	s.availableHeight = metrics.AvailableHeight
	s.screenHeight = metrics.ScreenHeight
	s.topInset = metrics.TopInset
	s.bottomInset = metrics.BottomInset

	// Content-sized sheets: derive snap height from actual content size
	if s.contentSized {
		if metrics.ContentHeight > 0 {
			contentExtent := clampFloat(metrics.ContentHeight, 0, s.availableHeight)
			if len(s.snapHeights) == 0 || s.snapHeights[0] != contentExtent {
				s.snapHeights = []float64{contentExtent}
			}
			// First layout or no target yet: start opening animation
			if prevAvailable <= 0 || s.targetExtent == 0 {
				s.currentExtent = 0
				s.targetExtent = contentExtent
				s.animateTo(s.targetExtent, 0)
			} else if s.targetExtent != contentExtent {
				// Content size changed: update target, clamp current
				s.targetExtent = contentExtent
				if s.currentExtent > contentExtent {
					s.currentExtent = contentExtent
				}
			}
		}
		s.updateProgress()
		return
	}

	// Convert fractional snap points to pixel heights
	s.snapHeights = make([]float64, len(s.snapPoints))
	for i, snap := range s.snapPoints {
		s.snapHeights[i] = snap.FractionalHeight * s.availableHeight
	}
	if len(s.snapHeights) == 0 {
		return
	}

	// First layout: start opening animation to initial snap point
	if prevAvailable <= 0 {
		s.currentExtent = 0
		s.targetExtent = s.snapHeights[s.initialIndex]
		s.animateTo(s.targetExtent, 0)
		return
	}

	// Available height changed (rotation, keyboard): scale extents proportionally
	if prevAvailable != s.availableHeight {
		ratio := s.availableHeight / prevAvailable
		s.currentExtent = clampFloat(s.currentExtent*ratio, 0, s.availableHeight)
		s.targetExtent = clampFloat(s.targetExtent*ratio, 0, s.availableHeight)
	}
	s.updateProgress()
}

func (s *bottomSheetState) currentExtentPx() float64 {
	return s.currentExtent
}

func (s *bottomSheetState) requestDismiss(result any) {
	if s.isDismissing {
		return
	}
	s.isDismissing = true
	s.animateToDismiss(result)
}

func (s *bottomSheetState) snapToIndex(index int) {
	if !s.metricsReady || len(s.snapHeights) == 0 {
		return
	}
	if index < 0 || index >= len(s.snapHeights) {
		index = 0
	}
	s.animateTo(s.snapHeights[index], 0)
}

func (s *bottomSheetState) snapToFraction(fraction float64) {
	if !s.metricsReady {
		return
	}
	fraction = clampFloat(fraction, 0, 1)
	s.animateTo(fraction*s.availableHeight, 0)
}

func (s *bottomSheetState) animateTo(target, velocity float64) {
	if !s.metricsReady {
		return
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.isDragging = false
	s.isDismissing = false

	s.targetExtent = clampFloat(target, 0, s.availableHeight)

	s.spring = animation.NewSpringSimulation(
		animation.IOSSpring(),
		s.currentExtent,
		velocity,
		s.targetExtent,
	)

	s.startSpring(nil)
}

func (s *bottomSheetState) animateToDismiss(result any) {
	if !s.metricsReady {
		if s.onDismiss != nil {
			s.onDismiss(result)
		}
		return
	}

	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.spring = animation.NewSpringSimulation(
		animation.IOSSpring(),
		s.currentExtent,
		0,
		0,
	)

	s.startSpring(result)
}

func (s *bottomSheetState) startSpring(dismissResult any) {
	lastTime := animation.Now()
	s.ticker = animation.NewTicker(func(elapsed time.Duration) {
		if s.spring == nil {
			s.ticker.Stop()
			return
		}
		now := animation.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		done := s.spring.Step(dt)
		newExtent := s.spring.Position()
		s.SetState(func() {
			s.currentExtent = clampFloat(newExtent, 0, s.availableHeight)
		})
		s.updateProgress()

		if done {
			s.ticker.Stop()
			if s.currentExtent <= 0.5 && s.isDismissing {
				if s.onDismiss != nil {
					s.onDismiss(dismissResult)
				}
			}
		}
	})
	s.ticker.Start()
}

func (s *bottomSheetState) updateProgress() {
	if s.controller == nil {
		return
	}
	maxExtent := s.maxSnapHeight()
	if maxExtent <= 0 {
		s.controller.setProgress(0)
		return
	}
	s.controller.setProgress(s.currentExtent / maxExtent)
}

func (s *bottomSheetState) maxSnapHeight() float64 {
	if len(s.snapHeights) == 0 {
		if s.contentSized {
			return s.targetExtent
		}
		return 0
	}
	return s.snapHeights[len(s.snapHeights)-1]
}

func (s *bottomSheetState) onDragStart(_ DragStartDetails) {
	if !s.enableDrag || !s.metricsReady {
		return
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.spring = nil
	s.isDragging = true
}

func (s *bottomSheetState) onDragUpdate(d DragUpdateDetails) {
	if !s.isDragging || !s.metricsReady {
		return
	}
	delta := -d.PrimaryDelta
	s.SetState(func() {
		s.currentExtent = clampFloat(s.currentExtent+delta, 0, s.availableHeight)
	})
	s.updateProgress()
}

func (s *bottomSheetState) onDragEnd(d DragEndDetails) {
	if !s.isDragging || !s.metricsReady {
		return
	}
	s.isDragging = false

	velocity := -d.PrimaryVelocity
	target := s.findTargetSnap(s.currentExtent, velocity)

	if target <= 0 {
		s.isDismissing = true
		s.animateToDismiss(nil)
		return
	}

	s.animateTo(target, velocity)
}

// findTargetSnap determines which snap point to animate to after a drag ends.
// Returns 0 to indicate the sheet should dismiss.
// The decision considers current position, drag velocity, and snap behavior thresholds.
func (s *bottomSheetState) findTargetSnap(position, velocity float64) float64 {
	if len(s.snapHeights) == 0 {
		return 0
	}

	minSnap := s.snapHeights[0]
	dismissThreshold := minSnap * s.snapBehavior.DismissFactor

	// Below dismiss threshold with downward or no velocity: dismiss
	if position < dismissThreshold && velocity <= 0 {
		return 0
	}

	// Fast downward fling near bottom: dismiss
	if velocity < -s.snapBehavior.MinFlingVelocity && position < minSnap*0.8 {
		return 0
	}

	// High velocity: snap in direction of travel
	if math.Abs(velocity) > s.snapBehavior.SnapVelocityThreshold {
		if velocity < 0 {
			// Dragging down: go to next lower snap (but not dismiss)
			lower := s.nextLowerSnap(position)
			if lower <= 0 {
				return minSnap
			}
			return lower
		}
		// Dragging up: go to next higher snap
		return s.nextHigherSnap(position)
	}

	// Low velocity: snap to nearest point
	return s.nearestSnap(position)
}

func (s *bottomSheetState) nextLowerSnap(position float64) float64 {
	for i := len(s.snapHeights) - 1; i >= 0; i-- {
		if s.snapHeights[i] < position-1 {
			return s.snapHeights[i]
		}
	}
	return 0
}

func (s *bottomSheetState) nextHigherSnap(position float64) float64 {
	for i := 0; i < len(s.snapHeights); i++ {
		if s.snapHeights[i] > position+1 {
			return s.snapHeights[i]
		}
	}
	return s.snapHeights[len(s.snapHeights)-1]
}

func (s *bottomSheetState) nearestSnap(position float64) float64 {
	nearest := s.snapHeights[0]
	minDist := math.Abs(position - nearest)
	for _, snap := range s.snapHeights[1:] {
		dist := math.Abs(position - snap)
		if dist < minDist {
			minDist = dist
			nearest = snap
		}
	}
	return nearest
}

func (s *bottomSheetState) shouldStartHandleDrag(totalDelta float64) bool {
	return s.enableDrag
}

// shouldStartDrag decides whether to accept a vertical drag gesture.
// For content-aware mode, this coordinates with scrollable content:
// - If content is scrolled down, let the scroll view handle the gesture
// - If at scroll top and dragging down, the sheet takes over
// - If sheet is not at max height and dragging up, the sheet expands first
func (s *bottomSheetState) shouldStartDrag(totalDelta float64) bool {
	if !s.enableDrag {
		return false
	}
	dragMode := s.resolveDragMode()
	if dragMode == DragModeSheet {
		return true
	}
	if dragMode != DragModeContentAware {
		return false
	}

	// No scrollable registered: sheet always handles drags
	if s.scrollController == nil {
		return true
	}
	// Content is scrolled down: let scroll view handle the gesture
	if s.scrollOffset > 0 {
		return false
	}
	// At scroll top, dragging down: sheet takes over to collapse/dismiss
	if totalDelta > 0 {
		return true
	}
	// Dragging up: sheet expands if not at max height, else scroll view handles
	maxExtent := s.maxSnapHeight()
	return s.currentExtent < maxExtent
}

func (s *bottomSheetState) registerScrollable(controller *ScrollController) func() {
	if controller == nil {
		return func() {}
	}
	if s.scrollRemove != nil {
		s.scrollRemove()
	}
	s.scrollController = controller
	s.scrollOffset = controller.Offset()
	s.scrollRemove = controller.AddListener(func() {
		s.scrollOffset = controller.Offset()
	})
	return func() {
		if s.scrollRemove != nil {
			s.scrollRemove()
			s.scrollRemove = nil
		}
		if s.scrollController == controller {
			s.scrollController = nil
		}
	}
}

func (s *bottomSheetState) buildHandle() core.Widget {
	handle := Container{
		Width:        s.theme.HandleWidth,
		Height:       s.theme.HandleHeight,
		Color:        s.theme.HandleColor,
		BorderRadius: s.theme.HandleHeight / 2,
	}

	return Row{
		MainAxisSize:      MainAxisSizeMax,
		MainAxisAlignment: MainAxisAlignmentCenter,
		Children: []core.Widget{
			Padding{
				Padding: layout.EdgeInsets{
					Top:    s.theme.HandleTopPadding,
					Bottom: s.theme.HandleBottomPadding,
				},
				Child: handle,
			},
		},
	}
}

// bottomSheetScopeBuilder wraps the content builder in a scope widget.
// This allows the builder to be called with a context that has the scope as an ancestor.
type bottomSheetScopeBuilder struct {
	controller *BottomSheetController
	builder    func(core.BuildContext) core.Widget
}

func (b bottomSheetScopeBuilder) CreateElement() core.Element {
	return core.NewStatelessElement(b, nil)
}

func (b bottomSheetScopeBuilder) Key() any {
	return nil
}

func (b bottomSheetScopeBuilder) Build(ctx core.BuildContext) core.Widget {
	return bottomSheetScope{
		controller: b.controller,
		child: bottomSheetContentBuilder{
			builder: b.builder,
		},
	}
}

// bottomSheetContentBuilder calls the user's builder.
type bottomSheetContentBuilder struct {
	builder func(core.BuildContext) core.Widget
}

func (b bottomSheetContentBuilder) CreateElement() core.Element {
	return core.NewStatelessElement(b, nil)
}

func (b bottomSheetContentBuilder) Key() any {
	return nil
}

func (b bottomSheetContentBuilder) Build(ctx core.BuildContext) core.Widget {
	if b.builder == nil {
		return nil
	}
	return b.builder(ctx)
}

// bottomSheetScope is an InheritedWidget that provides the controller to descendants.
type bottomSheetScope struct {
	controller *BottomSheetController
	child      core.Widget
}

func (b bottomSheetScope) CreateElement() core.Element {
	return core.NewInheritedElement(b, nil)
}

func (b bottomSheetScope) Key() any {
	return nil
}

func (b bottomSheetScope) ChildWidget() core.Widget {
	return b.child
}

func (b bottomSheetScope) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(bottomSheetScope); ok {
		return b.controller != old.controller
	}
	return true
}

func (b bottomSheetScope) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return b.UpdateShouldNotify(oldWidget)
}

var bottomSheetScopeType = reflect.TypeFor[bottomSheetScope]()

// BottomSheetScope provides access to the bottom sheet controller from within sheet content.
// Use BottomSheetScope.Of(ctx).Close(result) to dismiss the sheet with animation.
type BottomSheetScope struct{}

// Of returns the BottomSheetController for the enclosing bottom sheet.
// Returns nil if not inside a bottom sheet or if the sheet has no controller.
func (BottomSheetScope) Of(ctx core.BuildContext) *BottomSheetController {
	inherited := ctx.DependOnInherited(bottomSheetScopeType, nil)
	if inherited == nil {
		return nil
	}
	if scope, ok := inherited.(bottomSheetScope); ok {
		return scope.controller
	}
	return nil
}

// BottomSheetScrollable bridges a scroll controller to the enclosing bottom sheet
// so the sheet can coordinate content-aware dragging.
//
// Example:
//
//	return widgets.BottomSheetScrollable{
//		Builder: func(controller *widgets.ScrollController) core.Widget {
//			return widgets.ListView{
//				Controller: controller,
//				Children:   items,
//			}
//		},
//	}
type BottomSheetScrollable struct {
	Controller *ScrollController
	Builder    func(controller *ScrollController) core.Widget
}

func (b BottomSheetScrollable) CreateElement() core.Element {
	return core.NewStatefulElement(b, nil)
}

func (b BottomSheetScrollable) Key() any {
	return nil
}

func (b BottomSheetScrollable) CreateState() core.State {
	return &bottomSheetScrollableState{}
}

type bottomSheetScrollableState struct {
	core.StateBase
	controller *ScrollController
	unregister func()
	registered bool
}

func (s *bottomSheetScrollableState) InitState() {
	widget := s.Element().Widget().(BottomSheetScrollable)
	s.ensureController(widget)
	s.registered = false
}

func (s *bottomSheetScrollableState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	widget := s.Element().Widget().(BottomSheetScrollable)
	old := oldWidget.(BottomSheetScrollable)
	if widget.Controller != old.Controller {
		s.detach()
		s.ensureController(widget)
		s.registered = false
	}
}

func (s *bottomSheetScrollableState) Dispose() {
	s.detach()
}

func (s *bottomSheetScrollableState) Build(ctx core.BuildContext) core.Widget {
	widget := s.Element().Widget().(BottomSheetScrollable)
	if !s.registered {
		s.register(ctx)
	}
	if widget.Builder == nil {
		return nil
	}
	return widget.Builder(s.controller)
}

func (s *bottomSheetScrollableState) ensureController(widget BottomSheetScrollable) {
	s.controller = widget.Controller
	if s.controller == nil {
		s.controller = &ScrollController{}
	}
}

func (s *bottomSheetScrollableState) register(ctx core.BuildContext) {
	if s.controller == nil {
		return
	}
	scope := BottomSheetScope{}.Of(ctx)
	if scope == nil {
		return
	}
	s.unregister = scope.registerScrollableInternal(s.controller)
	s.registered = true
}

func (s *bottomSheetScrollableState) detach() {
	if s.unregister != nil {
		s.unregister()
		s.unregister = nil
	}
	s.registered = false
}
