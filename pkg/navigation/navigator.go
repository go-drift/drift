package navigation

import (
	"reflect"
	"sync"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/overlay"
	"github.com/go-drift/drift/pkg/widgets"
)

// NavigationScope tracks navigator hierarchy and determines which navigator
// receives back button events and deep links.
//
// In apps with multiple navigators (e.g., [TabScaffold] with per-tab stacks),
// NavigationScope ensures the correct navigator handles user input:
//
//   - The root navigator (IsRoot=true) handles deep links
//   - The active navigator (set by TabScaffold on tab change) handles back button
//   - When the active navigator can't pop, falls back to root
//
// You typically don't interact with NavigationScope directly. It's managed
// automatically by [Navigator] and [TabScaffold].
type NavigationScope struct {
	mu              sync.Mutex
	root            NavigatorState
	activeNavigator NavigatorState
}

var globalScope = &NavigationScope{}

// SetRoot registers the root navigator (called by Navigator with IsRoot=true).
func (s *NavigationScope) SetRoot(nav NavigatorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = nav
	if s.activeNavigator == nil {
		s.activeNavigator = nav
	}
}

// SetActiveNavigator sets which navigator receives back button events.
// TabScaffold calls this on tab change.
func (s *NavigationScope) SetActiveNavigator(nav NavigatorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeNavigator = nav
}

// ActiveNavigator returns the currently focused navigator.
func (s *NavigationScope) ActiveNavigator() NavigatorState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeNavigator != nil {
		return s.activeNavigator
	}
	return s.root
}

// ClearActiveIf clears activeNavigator if it matches nav (call in Dispose).
func (s *NavigationScope) ClearActiveIf(nav NavigatorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeNavigator == nav {
		s.activeNavigator = s.root // Fall back to root
	}
}

// ClearRootIf clears root navigator if it matches nav (call in Dispose).
func (s *NavigationScope) ClearRootIf(nav NavigatorState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nav {
		s.root = nil
	}
	if s.activeNavigator == nav {
		s.activeNavigator = nil
	}
}

// HandleBackButton attempts to pop the active navigator's route stack.
//
// Call this from platform back button handlers. It tries the active navigator
// first (e.g., the current tab's navigator), then falls back to the root
// navigator if the active one can't pop.
//
// Returns true if a route was popped, false if at root (app should exit).
//
//	// In platform back button handler:
//	if !navigation.HandleBackButton() {
//	    // At root - exit app or show confirmation
//	}
func HandleBackButton() bool {
	globalScope.mu.Lock()
	nav := globalScope.activeNavigator
	root := globalScope.root
	globalScope.mu.Unlock()

	if nav == nil {
		return false
	}

	// Try active navigator first
	if nav.CanPop() {
		return nav.MaybePop(nil)
	}

	// Only fall back to root if:
	// 1. Active is different from root (nested navigator)
	// 2. Root can actually pop
	// This prevents accidentally popping a parent flow when tab is at root
	if nav != root && root != nil && root.CanPop() {
		return root.MaybePop(nil)
	}

	return false
}

// RootNavigator returns the root navigator registered with IsRoot=true.
//
// Use this for deep links and external navigation that should always target
// the main app navigator, regardless of which tab or nested navigator is
// currently active.
//
//	// In a deep link handler:
//	if nav := navigation.RootNavigator(); nav != nil {
//	    nav.PushNamed("/product", map[string]any{"id": productID})
//	}
//
// Returns nil if no root navigator has been registered.
func RootNavigator() NavigatorState {
	globalScope.mu.Lock()
	defer globalScope.mu.Unlock()
	return globalScope.root
}

// Navigator manages a stack of routes using imperative navigation.
//
// Navigator provides push/pop stack semantics similar to mobile navigation
// patterns. For declarative route configuration, see [Router] instead.
//
// Basic usage:
//
//	navigation.Navigator{
//	    InitialRoute: "/",
//	    IsRoot:       true, // Required for back button handling
//	    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
//	        switch settings.Name {
//	        case "/":
//	            return navigation.NewMaterialPageRoute(buildHome, settings)
//	        case "/details":
//	            return navigation.NewMaterialPageRoute(buildDetails, settings)
//	        }
//	        return nil
//	    },
//	}
//
// With route guards for authentication:
//
//	navigation.Navigator{
//	    InitialRoute:      "/",
//	    IsRoot:            true,
//	    RefreshListenable: authState, // Re-evaluate on auth changes
//	    Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	        if !authState.IsLoggedIn() && isProtectedRoute(ctx.ToPath) {
//	            return navigation.RedirectTo("/login")
//	        }
//	        return navigation.NoRedirect()
//	    },
//	    OnGenerateRoute: generateRoute,
//	}
type Navigator struct {
	// InitialRoute is the name of the first route to display.
	InitialRoute string

	// OnGenerateRoute creates routes from route settings.
	OnGenerateRoute func(settings RouteSettings) Route

	// OnUnknownRoute is called when OnGenerateRoute returns nil.
	OnUnknownRoute func(settings RouteSettings) Route

	// Observers receive navigation events.
	Observers []NavigatorObserver

	// IsRoot marks this as the app's primary navigator.
	// Only root navigators register with the NavigationScope for back button handling.
	// Set this to true for your main navigator, false for nested navigators (e.g., in tabs).
	IsRoot bool

	// Redirect is called before every navigation.
	// Return NoRedirect() to allow navigation, or RedirectTo()/RedirectWithArgs() to redirect.
	// Only applies to named routes (Push with non-empty Settings().Name, PushNamed, etc.).
	Redirect func(ctx RedirectContext) RedirectResult

	// RefreshListenable triggers redirect re-evaluation when notified.
	// Use this when auth state changes to re-check if the current route is still accessible.
	RefreshListenable core.Listenable
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

// NavigatorState provides methods to manipulate the navigation stack.
//
// Obtain a NavigatorState using [NavigatorOf] from within the widget tree,
// or [RootNavigator] from outside it (e.g., for deep links).
//
//	nav := navigation.NavigatorOf(ctx)
//	nav.PushNamed("/details", map[string]any{"id": 123})
type NavigatorState interface {
	// Push adds a route to the top of the stack.
	// If a Redirect callback is configured and the route has a name,
	// the redirect logic is applied before pushing.
	Push(route Route)

	// PushNamed creates and pushes a route by name.
	// The route is created via OnGenerateRoute with the given name and args.
	// Redirect logic is applied if configured.
	PushNamed(name string, args any)

	// PushReplacementNamed replaces the current route with a new named route.
	// The old route receives DidPop, the new route receives DidPush.
	// Redirect logic is applied if configured.
	PushReplacementNamed(name string, args any)

	// Pop removes the current route from the stack.
	// The result is passed to the popped route's DidPop callback.
	// Does nothing if only one route remains (can't pop the root).
	Pop(result any)

	// PopUntil removes routes until the predicate returns true for the top route.
	// Each route's WillPop is checked before removal; removal stops if WillPop
	// returns false. Routes are removed without animation.
	PopUntil(predicate func(Route) bool)

	// PushReplacement replaces the current route with a new route.
	// If a Redirect callback is configured and the route has a name,
	// the redirect logic is applied before replacing.
	PushReplacement(route Route)

	// CanPop returns true if there are routes that can be popped.
	// Returns false if only the root route remains.
	CanPop() bool

	// MaybePop attempts to pop if possible.
	// Checks CanPop and the top route's WillPop before popping.
	// Returns true if a route was popped, false otherwise.
	MaybePop(result any) bool
}

type navigatorState struct {
	element            *core.StatefulElement
	navigator          Navigator
	routes             []Route
	exitingRoute       Route        // route currently animating out
	overlayState       OverlayState // stored when OnOverlayReady fires
	isRefreshing       bool         // guard against re-entrant refresh
	unsubscribeRefresh func()       // cleanup for RefreshListenable
}

func (s *navigatorState) setElement(element *core.StatefulElement) {
	s.element = element
}

func (s *navigatorState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *navigatorState) InitState() {
	s.navigator = s.element.Widget().(Navigator)

	// Register as root navigator if IsRoot is set
	if s.navigator.IsRoot {
		globalScope.SetRoot(s)
	}

	// Set up RefreshListenable for auth state changes
	if s.navigator.RefreshListenable != nil {
		s.unsubscribeRefresh = s.navigator.RefreshListenable.AddListener(s.onRefresh)
	}

	// Push the initial route (with redirect support)
	if s.navigator.InitialRoute != "" && s.navigator.OnGenerateRoute != nil {
		initialPath := s.navigator.InitialRoute
		var initialArgs any

		// Apply redirect to initial route
		if s.navigator.Redirect != nil {
			finalPath, finalArgs, _, _ := s.applyRedirect("", initialPath, initialArgs)
			initialPath = finalPath
			initialArgs = finalArgs
		}

		settings := RouteSettings{Name: initialPath, Arguments: initialArgs}
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
	// Register with TabScaffold if we're inside one (for active navigator tracking)
	tryRegisterTabNavigator(ctx, s)

	// Check if top route is transparent (needs previous routes visible)
	topIsTransparent := false
	if len(s.routes) > 0 {
		if tr, ok := s.routes[len(s.routes)-1].(TransparentRoute); ok {
			topIsTransparent = tr.IsTransparent()
		}
	}

	// Build all routes in a Stack
	children := make([]core.Widget, 0, len(s.routes)+1)
	for i, route := range s.routes {
		isTop := i == len(s.routes)-1
		rb := routeBuilder{
			route: route,
			isTop: isTop,
		}

		// Route is visible if:
		// - It's the top route, OR
		// - Top route is transparent and this is the route directly below it
		isVisible := isTop || (topIsTransparent && i == len(s.routes)-2)

		// Always wrap in ExcludeSemantics to maintain element tree identity.
		// Non-top routes are excluded from accessibility (hidden behind the top route).
		children = append(children, widgets.ExcludeSemantics{
			Child: widgets.Offstage{
				Offstage: !isVisible,
				Child:    rb,
			},
			Excluding: !isTop,
		})
	}

	// Add exiting route on top (it's animating out)
	if s.exitingRoute != nil {
		children = append(children, widgets.ExcludeSemantics{
			Child: routeBuilder{
				route: s.exitingRoute,
				isTop: false, // No longer visually on top
			},
			Excluding: true, // Exclude from accessibility - user is navigating away
		})
	}

	// Build route stack
	routeStack := widgets.Stack{
		Children: children,
		Fit:      widgets.StackFitExpand,
	}

	// Wrap in Overlay for modal routes support
	overlayWidget := overlay.Overlay{
		Child: routeStack,
		OnOverlayReady: func(overlayState OverlayState) {
			// Called via Dispatch, safe to mutate
			s.overlayState = overlayState
			// Notify existing routes that overlay is ready
			for _, route := range s.routes {
				route.SetOverlay(overlayState)
			}
			// Rebuild navigator to update routes that switched rendering mode
			// (ModalRoute switches from direct to overlay rendering)
			s.SetState(func() {})
		},
	}

	// Wrap in inherited widget so descendants can access NavigatorState
	return navigatorInherited{
		state: s,
		child: overlayWidget,
	}
}

func (s *navigatorState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *navigatorState) Dispose() {
	// Unsubscribe from RefreshListenable
	if s.unsubscribeRefresh != nil {
		s.unsubscribeRefresh()
		s.unsubscribeRefresh = nil
	}

	// Clear from NavigationScope
	globalScope.ClearActiveIf(s)
	if s.navigator.IsRoot {
		globalScope.ClearRootIf(s)
	}
}

func (s *navigatorState) DidChangeDependencies() {}

func (s *navigatorState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	s.navigator = s.element.Widget().(Navigator)
}

// Push adds a route to the stack.
// If the route has a name and a Redirect callback is configured, the redirect
// will be applied. Routes with empty Settings().Name skip redirect checks.
func (s *navigatorState) Push(route Route) {
	fromPath := ""
	if len(s.routes) > 0 {
		fromPath = s.routes[len(s.routes)-1].Settings().Name
	}

	toPath := route.Settings().Name
	toArgs := route.Settings().Arguments

	// Guard: only apply redirect if route has a name
	// Push(route) with empty name is unguarded
	if toPath != "" && s.navigator.Redirect != nil {
		finalPath, finalArgs, replace, _ := s.applyRedirect(fromPath, toPath, toArgs)

		if finalPath != toPath {
			// Redirect occurred - generate new route
			newSettings := RouteSettings{Name: finalPath, Arguments: finalArgs}
			route = s.navigator.OnGenerateRoute(newSettings)
			if route == nil && s.navigator.OnUnknownRoute != nil {
				route = s.navigator.OnUnknownRoute(newSettings)
			}
			if route == nil {
				return
			}
		}

		if replace && len(s.routes) > 0 {
			s.doPushReplacement(route)
			return
		}
	}

	s.doPush(route)
}

// doPush performs the actual push without redirect checks.
func (s *navigatorState) doPush(route Route) {
	s.SetState(func() {
		var previousTop Route
		if len(s.routes) > 0 {
			previousTop = s.routes[len(s.routes)-1]
			previousTop.DidChangeNext(route)
		}
		s.routes = append(s.routes, route)

		// Notify new route of its previous
		route.DidChangePrevious(previousTop)

		// Pass overlay state to the route if available
		if s.overlayState != nil {
			route.SetOverlay(s.overlayState)
		}

		route.DidPush()

		// Notify observers
		for _, observer := range s.navigator.Observers {
			observer.DidPush(route, previousTop)
		}
	})
}

func (s *navigatorState) routeFromName(name string, args any) Route {
	if s.navigator.OnGenerateRoute == nil {
		return nil
	}
	settings := RouteSettings{Name: name, Arguments: args}
	route := s.navigator.OnGenerateRoute(settings)
	if route == nil && s.navigator.OnUnknownRoute != nil {
		route = s.navigator.OnUnknownRoute(settings)
	}
	return route
}

// PushNamed pushes a route by name, applying redirect if configured.
func (s *navigatorState) PushNamed(name string, args any) {
	fromPath := ""
	if len(s.routes) > 0 {
		fromPath = s.routes[len(s.routes)-1].Settings().Name
	}

	finalPath := name
	finalArgs := args
	replace := false

	// Apply redirect
	if s.navigator.Redirect != nil {
		finalPath, finalArgs, replace, _ = s.applyRedirect(fromPath, name, args)
	}

	route := s.routeFromName(finalPath, finalArgs)
	if route == nil {
		return
	}

	if replace && len(s.routes) > 0 {
		s.doPushReplacement(route)
	} else {
		s.doPush(route)
	}
}

// PushReplacementNamed replaces the current route, applying redirect if configured.
func (s *navigatorState) PushReplacementNamed(name string, args any) {
	fromPath := ""
	if len(s.routes) > 0 {
		fromPath = s.routes[len(s.routes)-1].Settings().Name
	}

	finalPath := name
	finalArgs := args

	// Apply redirect
	if s.navigator.Redirect != nil {
		finalPath, finalArgs, _, _ = s.applyRedirect(fromPath, name, args)
	}

	route := s.routeFromName(finalPath, finalArgs)
	if route != nil {
		s.doPushReplacement(route)
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
// Routes are removed immediately without animation. Each route's WillPop is
// checked before removal - if WillPop returns false, the removal stops.
// Observer DidRemove callbacks are fired for each removed route.
func (s *navigatorState) PopUntil(predicate func(Route) bool) {
	s.SetState(func() {
		for len(s.routes) > 1 {
			top := s.routes[len(s.routes)-1]
			if predicate(top) {
				break
			}
			// Check WillPop before removing
			if !top.WillPop() {
				break
			}
			s.routes = s.routes[:len(s.routes)-1]

			// Fire lifecycle and observers
			var previous Route
			if len(s.routes) > 0 {
				previous = s.routes[len(s.routes)-1]
			}
			s.removeRoute(top, previous)
		}
		// Notify new top
		if len(s.routes) > 0 {
			s.routes[len(s.routes)-1].DidChangeNext(nil)
		}
	})
}

// removeRoute fires DidPop and observer callbacks for a removed route.
func (s *navigatorState) removeRoute(route Route, previousRoute Route) {
	route.DidPop(nil)
	for _, observer := range s.navigator.Observers {
		observer.DidRemove(route, previousRoute)
	}
}

// PushReplacement replaces the current route.
// If the route has a name and a Redirect callback is configured, the redirect
// will be applied.
func (s *navigatorState) PushReplacement(route Route) {
	if len(s.routes) == 0 {
		s.Push(route)
		return
	}

	fromPath := s.routes[len(s.routes)-1].Settings().Name
	toPath := route.Settings().Name
	toArgs := route.Settings().Arguments

	// Apply redirect if route has a name
	if toPath != "" && s.navigator.Redirect != nil {
		finalPath, finalArgs, _, _ := s.applyRedirect(fromPath, toPath, toArgs)

		if finalPath != toPath {
			// Redirect occurred - generate new route
			newSettings := RouteSettings{Name: finalPath, Arguments: finalArgs}
			route = s.navigator.OnGenerateRoute(newSettings)
			if route == nil && s.navigator.OnUnknownRoute != nil {
				route = s.navigator.OnUnknownRoute(newSettings)
			}
			if route == nil {
				return
			}
		}
	}

	s.doPushReplacement(route)
}

// doPushReplacement performs the actual replacement without redirect checks.
func (s *navigatorState) doPushReplacement(route Route) {
	if len(s.routes) == 0 {
		s.doPush(route)
		return
	}
	s.SetState(func() {
		oldRoute := s.routes[len(s.routes)-1]

		// Get previous of old route (for new route's DidChangePrevious)
		var previousOfOld Route
		if len(s.routes) > 1 {
			previousOfOld = s.routes[len(s.routes)-2]
		}

		s.routes[len(s.routes)-1] = route
		oldRoute.DidPop(nil)

		// Notify new route of previous
		route.DidChangePrevious(previousOfOld)

		// Pass overlay state to the route if available
		if s.overlayState != nil {
			route.SetOverlay(s.overlayState)
		}

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
	state *navigatorState
	child core.Widget
}

func (n navigatorInherited) CreateElement() core.Element {
	return core.NewInheritedElement(n, nil)
}

func (n navigatorInherited) Key() any {
	return nil
}

func (n navigatorInherited) ChildWidget() core.Widget {
	return n.child
}

func (n navigatorInherited) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(navigatorInherited); ok {
		return n.state != old.state
	}
	return true
}

// UpdateShouldNotifyDependent returns true for any aspects since navigatorInherited
// doesn't support granular aspect tracking yet.
func (n navigatorInherited) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return n.UpdateShouldNotify(oldWidget)
}

var navigatorInheritedType = reflect.TypeOf(navigatorInherited{})

// NavigatorOf returns the NavigatorState from the nearest Navigator ancestor.
// Returns nil if no Navigator is found.
func NavigatorOf(ctx core.BuildContext) NavigatorState {
	inherited := ctx.DependOnInherited(navigatorInheritedType, nil)
	if inherited == nil {
		return nil
	}
	if nav, ok := inherited.(navigatorInherited); ok {
		return nav.state
	}
	return nil
}

// RedirectContext provides information about the navigation being attempted,
// allowing the Redirect callback to make informed decisions.
type RedirectContext struct {
	// FromPath is the current route's path before navigation.
	// Empty string on initial route or when navigating from outside a route.
	FromPath string

	// ToPath is the intended destination path.
	ToPath string

	// Arguments are the navigation arguments being passed.
	Arguments any
}

// RedirectResult tells the navigator how to handle the navigation.
//
// Create using helper functions:
//   - [NoRedirect] to allow navigation to proceed
//   - [RedirectTo] to redirect to a different path
//   - [RedirectWithArgs] to redirect with custom arguments
type RedirectResult struct {
	// Path is the redirect destination. Empty string means no redirect.
	Path string

	// Arguments for the redirect destination.
	// If nil, original arguments are discarded.
	Arguments any

	// Replace controls whether to replace the current route (true) or push (false).
	// RedirectTo and RedirectWithArgs set this to true by default.
	Replace bool
}

// NoRedirect returns a result that allows navigation to proceed normally.
// Use this in your Redirect callback when the route should be accessible.
//
//	Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	    if isPublicRoute(ctx.ToPath) {
//	        return navigation.NoRedirect()
//	    }
//	    // ... check auth
//	}
func NoRedirect() RedirectResult {
	return RedirectResult{}
}

// RedirectTo creates a redirect to a different path.
// The current route is replaced (not pushed) by default.
//
//	Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	    if !isLoggedIn {
//	        return navigation.RedirectTo("/login")
//	    }
//	    return navigation.NoRedirect()
//	}
func RedirectTo(path string) RedirectResult {
	return RedirectResult{Path: path, Replace: true}
}

// RedirectWithArgs creates a redirect with custom arguments.
// Useful for preserving the intended destination through a login flow.
//
//	Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	    if !isLoggedIn && isProtected(ctx.ToPath) {
//	        return navigation.RedirectWithArgs("/login", map[string]any{
//	            "returnTo": ctx.ToPath,
//	        })
//	    }
//	    return navigation.NoRedirect()
//	}
func RedirectWithArgs(path string, args any) RedirectResult {
	return RedirectResult{Path: path, Arguments: args, Replace: true}
}

const (
	maxRedirects      = 10
	redirectErrorPath = "/_redirect_error"
)

// applyRedirect checks and applies redirect for navigation.
// Returns the final path, args, whether a replace occurred, and any error.
func (s *navigatorState) applyRedirect(fromPath, toPath string, args any) (string, any, bool, error) {
	if s.navigator.Redirect == nil {
		return toPath, args, false, nil
	}

	seen := make(map[string]bool)
	currentPath := toPath
	currentArgs := args
	replace := false

	for i := 0; i < maxRedirects; i++ {
		if seen[currentPath] {
			// Loop detected - return error path for OnUnknownRoute
			return redirectErrorPath, map[string]any{
				"error": "redirect_loop",
				"path":  currentPath,
			}, true, nil
		}
		seen[currentPath] = true

		ctx := RedirectContext{
			FromPath:  fromPath,
			ToPath:    currentPath,
			Arguments: currentArgs,
		}
		result := s.navigator.Redirect(ctx)

		if result.Path == "" || result.Path == currentPath {
			return currentPath, currentArgs, replace, nil
		}

		currentPath = result.Path
		currentArgs = result.Arguments
		replace = replace || result.Replace
	}

	// Max redirects exceeded - return error path
	return redirectErrorPath, map[string]any{
		"error": "max_redirects",
		"limit": maxRedirects,
	}, true, nil
}

// onRefresh handles RefreshListenable notifications to re-evaluate redirects.
func (s *navigatorState) onRefresh() {
	// Guard against re-entrancy
	if s.isRefreshing {
		return
	}

	// Guard: only process refresh if this is the active navigator
	// Prevents duplicate redirects across nested navigators
	if globalScope.ActiveNavigator() != NavigatorState(s) {
		return
	}

	// Guard: need routes and redirect callback
	if len(s.routes) == 0 || s.navigator.Redirect == nil {
		return
	}

	s.isRefreshing = true
	defer func() { s.isRefreshing = false }()

	current := s.routes[len(s.routes)-1]
	ctx := RedirectContext{
		FromPath:  current.Settings().Name,
		ToPath:    current.Settings().Name,
		Arguments: current.Settings().Arguments,
	}
	result := s.navigator.Redirect(ctx)

	if result.Path != "" && result.Path != current.Settings().Name {
		s.PushReplacementNamed(result.Path, result.Arguments)
	}
}
