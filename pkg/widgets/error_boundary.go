package widgets

import (
	"reflect"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// ErrorBoundary catches panics from descendant widgets and displays a fallback
// widget instead of crashing the app. This provides scoped error handling for
// subtrees of the widget tree.
//
// # Error Handling Behavior
//
// In debug mode (core.DebugMode = true), uncaught panics anywhere in the app
// display a full-screen DebugErrorScreen with stack traces. In production mode,
// uncaught panics crash the app. Use ErrorBoundary to catch panics and show
// graceful fallback UI in production.
//
// ErrorBoundary catches panics during:
//   - Build: widget Build() methods
//   - Layout: RenderObject PerformLayout()
//   - Paint: RenderObject Paint()
//   - HitTest: RenderObject HitTest()
//
// # Scoped vs Global Error Handling
//
// Wrap specific subtrees to isolate failures while keeping the rest of the app
// running. Or wrap your entire app to provide custom error UI in production:
//
//	// Scoped: only RiskyWidget failures show fallback
//	Column{
//	    Children: []core.Widget{
//	        HeaderWidget{},
//	        ErrorBoundary{
//	            Child: RiskyWidget{},
//	            FallbackBuilder: func(err *errors.BoundaryError) core.Widget {
//	                return Text{Content: "Failed to load"}
//	            },
//	        },
//	        FooterWidget{},
//	    },
//	}
//
//	// Global: custom error UI for entire app in production
//	drift.NewApp(ErrorBoundary{
//	    Child: MyApp{},
//	    FallbackBuilder: func(err *errors.BoundaryError) core.Widget {
//	        return MyCustomErrorScreen{Error: err}
//	    },
//	}).Run()
//
// # Programmatic Control
//
// Use [ErrorBoundaryOf] to access the boundary's state from descendant widgets:
//
//	state := widgets.ErrorBoundaryOf(ctx)
//	if state != nil && state.HasError() {
//	    state.Reset() // Clear error and retry
//	}
type ErrorBoundary struct {
	// Child is the widget tree to wrap with error handling.
	Child core.Widget
	// FallbackBuilder creates a widget to show when an error is caught.
	// If nil, uses the default ErrorWidget.
	FallbackBuilder func(*errors.BoundaryError) core.Widget
	// OnError is called when an error is caught. Use for logging/analytics.
	OnError func(*errors.BoundaryError)
	// WidgetKey is an optional key for the widget. Changing the key forces
	// the ErrorBoundary to recreate its state, clearing any captured error.
	WidgetKey any
}

func (e ErrorBoundary) CreateElement() core.Element {
	return core.NewStatefulElement(e, nil)
}

func (e ErrorBoundary) Key() any {
	return e.WidgetKey
}

func (e ErrorBoundary) CreateState() core.State {
	return &errorBoundaryState{}
}

type errorBoundaryState struct {
	core.StateBase
	capturedError *errors.BoundaryError
}

func (s *errorBoundaryState) Build(ctx core.BuildContext) core.Widget {
	widget := ctx.Widget().(ErrorBoundary)

	// If we've captured an error, show the fallback
	if s.capturedError != nil {
		if widget.FallbackBuilder != nil {
			return widget.FallbackBuilder(s.capturedError)
		}
		return ErrorWidget{Error: s.capturedError}
	}

	// Wrap child in an inherited widget that marks this boundary,
	// and use a render widget to catch layout/paint/hittest panics
	return errorBoundaryScope{
		state: s,
		child: errorBoundaryRenderWidget{
			state: s,
			child: widget.Child,
		},
	}
}

// Reset clears the captured error and rebuilds the child.
// Use this to retry rendering after an error.
func (s *errorBoundaryState) Reset() {
	s.SetState(func() {
		s.capturedError = nil
	})
}

// HasError returns true if this boundary has captured an error.
func (s *errorBoundaryState) HasError() bool {
	return s.capturedError != nil
}

// Error returns the captured error, or nil if none.
func (s *errorBoundaryState) Error() *errors.BoundaryError {
	return s.capturedError
}

// captureBoundaryError captures an error and triggers a rebuild.
func (s *errorBoundaryState) captureBoundaryError(err *errors.BoundaryError) {
	// Get the parent ErrorBoundary widget for OnError callback
	if s.Element() != nil {
		parentWidget := s.Element().Widget()
		if boundary, ok := parentWidget.(ErrorBoundary); ok && boundary.OnError != nil {
			boundary.OnError(err)
		}
	}

	s.SetState(func() {
		s.capturedError = err
	})
}

// errorBoundaryScope is an InheritedWidget that marks the boundary in the tree.
// Child elements can find this to report errors to the boundary.
type errorBoundaryScope struct {
	state *errorBoundaryState
	child core.Widget
}

func (e errorBoundaryScope) CreateElement() core.Element {
	return newErrorBoundaryScopeElement(e)
}

func (e errorBoundaryScope) Key() any {
	return nil
}

func (e errorBoundaryScope) ChildWidget() core.Widget {
	return e.child
}

func (e errorBoundaryScope) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	return false // Scope widget never notifies - it's just for tree marking
}

func (e errorBoundaryScope) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return false
}

// errorBoundaryScopeElement wraps InheritedElement and implements ErrorBoundaryCapture.
// We need to override Mount and RebuildIfNeeded to ensure 'self' is passed as the parent
// so that findErrorBoundary can find this element.
type errorBoundaryScopeElement struct {
	*core.InheritedElement
	self core.Element // Keep reference to self for parent passing
}

func newErrorBoundaryScopeElement(widget errorBoundaryScope) *errorBoundaryScopeElement {
	inherited := core.NewInheritedElement(widget, nil)
	element := &errorBoundaryScopeElement{
		InheritedElement: inherited,
	}
	element.self = element // Set self reference
	return element
}

// Mount overrides InheritedElement.Mount to pass self as parent to children.
func (e *errorBoundaryScopeElement) Mount(parent core.Element, slot any) {
	e.InheritedElement.MountWithSelf(parent, slot, e.self)
}

// RebuildIfNeeded overrides to pass self as parent when updating children.
func (e *errorBoundaryScopeElement) RebuildIfNeeded() {
	e.InheritedElement.RebuildIfNeededWithSelf(e.self)
}

// CaptureError implements core.ErrorBoundaryCapture.
func (e *errorBoundaryScopeElement) CaptureError(err *errors.BoundaryError) bool {
	widget := e.Widget().(errorBoundaryScope)
	state := widget.state
	state.captureBoundaryError(err)
	return true
}

// ErrorBoundaryOf returns the nearest ErrorBoundary's state, or nil if none exists
// in the ancestor chain. Use this to programmatically interact with an error boundary:
//
//	state := widgets.ErrorBoundaryOf(ctx)
//	if state != nil {
//	    if state.HasError() {
//	        // An error was caught
//	        err := state.Error()
//	        state.Reset()  // Clear error and retry rendering
//	    }
//	}
//
// Returns nil if there is no ErrorBoundary ancestor.
func ErrorBoundaryOf(ctx core.BuildContext) *errorBoundaryState {
	inherited := ctx.DependOnInherited(reflect.TypeOf(errorBoundaryScope{}), nil)
	if inherited == nil {
		return nil
	}
	if scope, ok := inherited.(errorBoundaryScope); ok {
		return scope.state
	}
	return nil
}

// errorBoundaryRenderWidget creates a render object that catches layout/paint/hittest panics.
type errorBoundaryRenderWidget struct {
	state *errorBoundaryState
	child core.Widget
}

func (e errorBoundaryRenderWidget) CreateElement() core.Element {
	return core.NewRenderObjectElement(e, nil)
}

func (e errorBoundaryRenderWidget) Key() any {
	return nil
}

func (e errorBoundaryRenderWidget) ChildWidget() core.Widget {
	return e.child
}

func (e errorBoundaryRenderWidget) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	ro := &renderErrorBoundary{state: e.state}
	ro.SetSelf(ro)
	return ro
}

func (e errorBoundaryRenderWidget) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if ro, ok := renderObject.(*renderErrorBoundary); ok {
		ro.Update(e.state)
	}
}

// renderErrorBoundary is a render object that catches panics during layout, paint, and hit testing.
type renderErrorBoundary struct {
	layout.RenderBoxBase
	child    layout.RenderBox
	state    *errorBoundaryState
	hasError bool // Track if we've captured an error (prevents repeated panics)
}

func (r *renderErrorBoundary) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderErrorBoundary) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// Update is called by errorBoundaryRenderWidget.UpdateRenderObject
func (r *renderErrorBoundary) Update(state *errorBoundaryState) {
	r.state = state
	// Clear hasError when boundary has been reset (capturedError is nil)
	if state != nil && state.capturedError == nil {
		r.hasError = false
	}
}

func (r *renderErrorBoundary) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil || r.hasError {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	var panicked bool
	var panicValue any
	var stack string

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicked = true
				panicValue = rec
				stack = errors.CaptureStack()
			}
		}()
		r.child.Layout(constraints, true)
	}()

	if panicked {
		r.hasError = true
		r.deferErrorCapture("layout", panicValue, stack)
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	r.SetSize(r.child.Size())
}

func (r *renderErrorBoundary) Paint(ctx *layout.PaintContext) {
	if r.child == nil || r.hasError {
		return
	}

	var panicked bool
	var panicValue any
	var stack string

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicked = true
				panicValue = rec
				stack = errors.CaptureStack()
			}
		}()
		ctx.PaintChildWithLayer(r.child, getChildOffset(r.child))
	}()

	if panicked {
		r.hasError = true
		r.deferErrorCapture("paint", panicValue, stack)
	}
}

func (r *renderErrorBoundary) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child == nil || r.hasError {
		return false
	}

	var panicked bool
	var panicValue any
	var stack string
	var hitResult bool

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				panicked = true
				panicValue = rec
				stack = errors.CaptureStack()
			}
		}()
		offset := getChildOffset(r.child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		hitResult = r.child.HitTest(local, result)
	}()

	if panicked {
		r.hasError = true
		r.deferErrorCapture("hittest", panicValue, stack)
		return false
	}
	return hitResult
}

// deferErrorCapture schedules error capture for next frame to avoid re-entrancy
func (r *renderErrorBoundary) deferErrorCapture(phase string, value any, stack string) {
	// Get render object type name (e.g., "*widgets.renderFlex")
	renderType := ""
	if r.child != nil {
		renderType = reflect.TypeOf(r.child).String()
	}

	err := &errors.BoundaryError{
		Phase:        phase,
		RenderObject: renderType,
		Recovered:    value,
		StackTrace:   stack,
		Timestamp:    time.Now(),
	}

	// Report to global handler immediately
	errors.ReportBoundaryError(err)

	// Schedule state update for next frame (before layout/build)
	platform.Dispatch(func() {
		if r.state != nil {
			r.state.captureBoundaryError(err)
		}
	})
}
