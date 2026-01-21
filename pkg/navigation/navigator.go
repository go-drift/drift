package navigation

import (
	"reflect"
	"sync"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/widgets"
)

// Global navigator for back button handling.
var (
	globalNavigator   NavigatorState
	globalNavigatorMu sync.Mutex
)

// HandleBackButton attempts to pop the active navigator.
// Returns true if handled (route popped), false if at root.
func HandleBackButton() bool {
	globalNavigatorMu.Lock()
	nav := globalNavigator
	globalNavigatorMu.Unlock()

	if nav == nil {
		return false
	}
	return nav.MaybePop(nil)
}

// GlobalNavigator returns the active navigator, if any.
func GlobalNavigator() NavigatorState {
	globalNavigatorMu.Lock()
	nav := globalNavigator
	globalNavigatorMu.Unlock()
	return nav
}

// Navigator manages a stack of routes.
type Navigator struct {
	// InitialRoute is the name of the first route to display.
	InitialRoute string

	// OnGenerateRoute creates routes from route settings.
	OnGenerateRoute func(settings RouteSettings) Route

	// OnUnknownRoute is called when OnGenerateRoute returns nil.
	OnUnknownRoute func(settings RouteSettings) Route

	// Observers receive navigation events.
	Observers []NavigatorObserver
}

// CreateElement returns a StatefulElement for this Navigator.
func (n Navigator) CreateElement() core.Element {
	return core.NewStatefulElement(n, nil)
}

// Key returns nil (no key).
func (n Navigator) Key() any {
	return nil
}

// CreateState creates the NavigatorState.
func (n Navigator) CreateState() core.State {
	return &navigatorState{}
}

// NavigatorState is the mutable state for a Navigator.
type NavigatorState interface {
	// Push adds a route to the stack.
	Push(route Route)

	// PushNamed pushes a route by name.
	PushNamed(name string, args any)

	// Pop removes the current route and optionally returns a result.
	Pop(result any)

	// PopUntil removes routes until the predicate returns true.
	PopUntil(predicate func(Route) bool)

	// PushReplacement replaces the current route.
	PushReplacement(route Route)

	// CanPop returns true if there's a route to pop.
	CanPop() bool

	// MaybePop pops if possible, returns true if popped.
	MaybePop(result any) bool
}

type navigatorState struct {
	element      *core.StatefulElement
	navigator    Navigator
	routes       []Route
	exitingRoute Route // route currently animating out
}

func (s *navigatorState) setElement(element *core.StatefulElement) {
	s.element = element
}

func (s *navigatorState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *navigatorState) InitState() {
	s.navigator = s.element.Widget().(Navigator)

	// Register as the global navigator for back button handling
	globalNavigatorMu.Lock()
	globalNavigator = s
	globalNavigatorMu.Unlock()

	// Push the initial route
	if s.navigator.InitialRoute != "" && s.navigator.OnGenerateRoute != nil {
		settings := RouteSettings{Name: s.navigator.InitialRoute}
		route := s.navigator.OnGenerateRoute(settings)
		if route == nil && s.navigator.OnUnknownRoute != nil {
			route = s.navigator.OnUnknownRoute(settings)
		}
		if route != nil {
			// Mark as initial route (no animation)
			if mr, ok := route.(*MaterialPageRoute); ok {
				mr.SetInitialRoute()
			}
			s.routes = []Route{route}
			route.DidPush()
		}
	}
}

func (s *navigatorState) Build(ctx core.BuildContext) core.Widget {
	// Build all routes in a Stack
	children := make([]core.Widget, 0, len(s.routes)+1)
	for i, route := range s.routes {
		isTop := i == len(s.routes)-1 && s.exitingRoute == nil
		rb := routeBuilder{
			route: route,
			isTop: isTop,
		}
		// Exclude non-top routes from accessibility (they're hidden behind the top route)
		if !isTop {
			children = append(children, widgets.ExcludeSemantics{
				ChildWidget: rb,
				Excluding:   true,
			})
		} else {
			children = append(children, rb)
		}
	}

	// Add exiting route on top (it's animating out)
	if s.exitingRoute != nil {
		children = append(children, routeBuilder{
			route: s.exitingRoute,
			isTop: true,
		})
	}

	// Wrap in inherited widget so descendants can access NavigatorState
	return navigatorInherited{
		state: s,
		childWidget: widgets.Stack{
			ChildrenWidgets: children,
			Fit:             widgets.StackFitExpand,
		},
	}
}

func (s *navigatorState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *navigatorState) Dispose() {
	// Unregister from global navigator if this is the active one
	globalNavigatorMu.Lock()
	if globalNavigator == s {
		globalNavigator = nil
	}
	globalNavigatorMu.Unlock()
}

func (s *navigatorState) DidChangeDependencies() {}

func (s *navigatorState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	s.navigator = s.element.Widget().(Navigator)
}

// Push adds a route to the stack.
func (s *navigatorState) Push(route Route) {
	s.SetState(func() {
		// Notify current top route
		if len(s.routes) > 0 {
			s.routes[len(s.routes)-1].DidChangeNext(route)
		}
		s.routes = append(s.routes, route)
		route.DidPush()
		// Notify observers
		var previousRoute Route
		if len(s.routes) > 1 {
			previousRoute = s.routes[len(s.routes)-2]
		}
		for _, observer := range s.navigator.Observers {
			observer.DidPush(route, previousRoute)
		}
	})
}

// PushNamed pushes a route by name.
func (s *navigatorState) PushNamed(name string, args any) {
	if s.navigator.OnGenerateRoute == nil {
		return
	}
	settings := RouteSettings{Name: name, Arguments: args}
	route := s.navigator.OnGenerateRoute(settings)
	if route == nil && s.navigator.OnUnknownRoute != nil {
		route = s.navigator.OnUnknownRoute(settings)
	}
	if route != nil {
		s.Push(route)
	}
}

// Pop removes the current route.
func (s *navigatorState) Pop(result any) {
	if len(s.routes) <= 1 {
		return
	}
	// Don't pop if already animating an exit
	if s.exitingRoute != nil {
		return
	}

	s.SetState(func() {
		popped := s.routes[len(s.routes)-1]
		s.routes = s.routes[:len(s.routes)-1]

		// Keep the popped route visible while it animates out
		s.exitingRoute = popped

		// Start the exit animation
		popped.DidPop(result)

		// Set up callback to remove route when animation completes
		if mr, ok := popped.(*MaterialPageRoute); ok && mr.controller != nil {
			mr.controller.AddStatusListener(func(status animation.AnimationStatus) {
				if status == animation.AnimationDismissed {
					s.SetState(func() {
						s.exitingRoute = nil
					})
				}
			})
		} else {
			// No animation, remove immediately
			s.exitingRoute = nil
		}

		// Notify new top route
		if len(s.routes) > 0 {
			s.routes[len(s.routes)-1].DidChangeNext(nil)
		}

		// Notify observers
		var previousRoute Route
		if len(s.routes) > 0 {
			previousRoute = s.routes[len(s.routes)-1]
		}
		for _, observer := range s.navigator.Observers {
			observer.DidPop(popped, previousRoute)
		}
	})
}

// PopUntil removes routes until the predicate returns true.
func (s *navigatorState) PopUntil(predicate func(Route) bool) {
	s.SetState(func() {
		for len(s.routes) > 1 {
			top := s.routes[len(s.routes)-1]
			if predicate(top) {
				break
			}
			s.routes = s.routes[:len(s.routes)-1]
			top.DidPop(nil)
		}
		// Notify new top
		if len(s.routes) > 0 {
			s.routes[len(s.routes)-1].DidChangeNext(nil)
		}
	})
}

// PushReplacement replaces the current route.
func (s *navigatorState) PushReplacement(route Route) {
	if len(s.routes) == 0 {
		s.Push(route)
		return
	}
	s.SetState(func() {
		oldRoute := s.routes[len(s.routes)-1]
		s.routes[len(s.routes)-1] = route
		oldRoute.DidPop(nil)
		route.DidPush()
		// Notify observers
		for _, observer := range s.navigator.Observers {
			observer.DidReplace(route, oldRoute)
		}
	})
}

// CanPop returns true if there are routes to pop.
func (s *navigatorState) CanPop() bool {
	return len(s.routes) > 1
}

// MaybePop pops if possible and WillPop returns true.
func (s *navigatorState) MaybePop(result any) bool {
	if !s.CanPop() {
		return false
	}
	top := s.routes[len(s.routes)-1]
	if !top.WillPop() {
		return false
	}
	s.Pop(result)
	return true
}

// routeBuilder wraps a route for building.
type routeBuilder struct {
	route Route
	isTop bool
}

func (r routeBuilder) CreateElement() core.Element {
	return core.NewStatelessElement(r, nil)
}

func (r routeBuilder) Key() any {
	return r.route
}

func (r routeBuilder) Build(ctx core.BuildContext) core.Widget {
	return r.route.Build(ctx)
}

// navigatorInherited provides NavigatorState to descendants.
type navigatorInherited struct {
	state       *navigatorState
	childWidget core.Widget
}

func (n navigatorInherited) CreateElement() core.Element {
	return core.NewInheritedElement(n, nil)
}

func (n navigatorInherited) Key() any {
	return nil
}

func (n navigatorInherited) Child() core.Widget {
	return n.childWidget
}

func (n navigatorInherited) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(navigatorInherited); ok {
		return n.state != old.state
	}
	return true
}

var navigatorInheritedType = reflect.TypeOf(navigatorInherited{})

// NavigatorOf returns the NavigatorState from the nearest Navigator ancestor.
// Returns nil if no Navigator is found.
func NavigatorOf(ctx core.BuildContext) NavigatorState {
	inherited := ctx.DependOnInherited(navigatorInheritedType)
	if inherited == nil {
		return nil
	}
	if nav, ok := inherited.(navigatorInherited); ok {
		return nav.state
	}
	return nil
}
