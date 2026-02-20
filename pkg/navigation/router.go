package navigation

import (
	"reflect"
	"strings"

	"github.com/go-drift/drift/pkg/core"
)

// ScreenRoute defines a route in the declarative [Router].
//
// A ScreenRoute can serve as a leaf route (with Screen), a layout wrapper
// (with Wrap), or both. Routes with only Children act as prefix groups.
//
// Path patterns support:
//   - Static segments: "/products", "/users/list"
//   - Parameters: "/products/:id", "/users/:userId/posts/:postId"
//   - Wildcards: "/files/*path" (captures remaining path)
//
// Example with nested routes:
//
//	navigation.ScreenRoute{
//	    Path:   "/products",
//	    Screen: buildProductList,
//	    Children: []navigation.ScreenRoute{
//	        {
//	            Path:   "/:id",           // Matches /products/:id
//	            Screen: buildProductDetail,
//	        },
//	        {
//	            Path:   "/:id/reviews",   // Matches /products/:id/reviews
//	            Screen: buildProductReviews,
//	        },
//	    },
//	}
//
// Wrap child routes in a persistent layout (tabs, sidebars, etc.):
//
//	navigation.ScreenRoute{
//	    Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
//	        return widgets.Column{
//	            Children: []core.Widget{
//	                MyNavigationBar{},
//	                widgets.Expanded{Child: child},
//	            },
//	        }
//	    },
//	    Children: []navigation.ScreenRoute{
//	        {Path: "/home", Screen: buildHome},
//	        {Path: "/profile", Screen: buildProfile},
//	    },
//	}
type ScreenRoute struct {
	// Path is the URL pattern for this route.
	// Use :param for path parameters and *param for wildcards.
	// Nested routes inherit the parent's path as a prefix.
	Path string

	// Screen creates the widget for this route.
	// RouteSettings includes extracted Params and Query from the URL.
	// Nil for pure wrapper or prefix-group routes.
	Screen func(ctx core.BuildContext, settings RouteSettings) core.Widget

	// Wrap wraps child routes in a persistent layout.
	// The child parameter is the matched child route's widget.
	// Wrap applies only to Children, not to this route's own Screen.
	// Nil for leaf-only routes.
	Wrap func(ctx core.BuildContext, child core.Widget) core.Widget

	// Redirect defines redirect logic for this route and its descendants.
	// Checked after the Router's global Redirect callback.
	// Ancestor redirects are evaluated outermost-first before the
	// matched route's own Redirect.
	Redirect func(ctx RedirectContext) RedirectResult

	// Children defines nested child routes.
	// Child paths are concatenated with this route's path.
	// If Wrap is set, all children are wrapped by it.
	Children []ScreenRoute

	// Future: StackKey string
	// Per-subtree navigator isolation for stateful shells. When set,
	// matched routes within this subtree would render in a dedicated
	// nested Navigator identified by this key, preserving navigation
	// history independently (e.g., tab branches). Not implemented in v1;
	// use TabScaffold for stateful tab navigation.
}

// ScreenOnly adapts a plain widget builder to the [ScreenRoute.Screen]
// signature. Use this for routes that don't need access to path parameters,
// query strings, or navigation arguments from [RouteSettings].
//
// Without ScreenOnly, routes that ignore settings require a boilerplate
// closure:
//
//	navigation.ScreenRoute{
//	    Path: "/settings",
//	    Screen: func(ctx core.BuildContext, _ navigation.RouteSettings) core.Widget {
//	        return buildSettings(ctx)
//	    },
//	}
//
// With ScreenOnly:
//
//	navigation.ScreenRoute{
//	    Path:   "/settings",
//	    Screen: navigation.ScreenOnly(buildSettings),
//	}
//
// Routes that read path parameters or query values should use the full
// [ScreenRoute.Screen] signature directly.
func ScreenOnly(build func(core.BuildContext) core.Widget) func(core.BuildContext, RouteSettings) core.Widget {
	return func(ctx core.BuildContext, _ RouteSettings) core.Widget {
		return build(ctx)
	}
}

// Router provides declarative route configuration with automatic path matching
// and parameter extraction.
//
// Router is the recommended approach for apps with URL-based routing. It
// internally creates a [Navigator] with IsRoot=true, so you don't need to
// manage navigator registration manually.
//
// IMPORTANT: Router is designed to be used as a singleton at the root of your
// app. Do not nest Routers or use Router inside [TabScaffold] tabs. For tabs
// with their own navigation stacks, use [Navigator] with [Tab.OnGenerateRoute]
// instead.
//
// Basic usage:
//
//	navigation.Router{
//	    InitialPath: "/",
//	    Routes: []navigation.ScreenRoute{
//	        {Path: "/", Screen: buildHome},
//	        {Path: "/products/:id", Screen: buildProduct},
//	    },
//	    ErrorBuilder: build404Page,
//	}
//
// With authentication:
//
//	navigation.Router{
//	    InitialPath:       "/",
//	    RefreshListenable: authState,
//	    Redirect: func(ctx navigation.RedirectContext) navigation.RedirectResult {
//	        if !authState.IsLoggedIn() && isProtected(ctx.ToPath) {
//	            return navigation.RedirectTo("/login")
//	        }
//	        return navigation.NoRedirect()
//	    },
//	    Routes: routes,
//	}
//
// Access the router for navigation using [RouterOf]:
//
//	router := navigation.RouterOf(ctx)
//	router.Go("/products/123", nil)
type Router struct {
	core.StatefulBase

	// Routes defines the route tree.
	Routes []ScreenRoute

	// Redirect is the global redirect callback, checked before every navigation.
	// Return [NoRedirect] to allow, or [RedirectTo]/[RedirectWithArgs] to redirect.
	// Route-specific redirects in [ScreenRoute.Redirect] are checked after this.
	Redirect func(ctx RedirectContext) RedirectResult

	// ErrorBuilder creates a widget for unmatched routes (404 pages).
	// If nil, navigation to unknown routes is silently ignored.
	ErrorBuilder func(ctx core.BuildContext, settings RouteSettings) core.Widget

	// InitialPath is the starting route path.
	// Defaults to "/" if not specified.
	InitialPath string

	// TrailingSlashBehavior controls how trailing slashes are handled in matching.
	// Default is [TrailingSlashStrip] which treats "/path/" same as "/path".
	TrailingSlashBehavior TrailingSlashBehavior

	// CaseSensitivity controls case handling in path matching.
	// Default is [CaseSensitive] which requires exact case match.
	CaseSensitivity CaseSensitivity

	// RefreshListenable triggers redirect re-evaluation when notified.
	// Connect this to auth state changes to automatically redirect users
	// when they log in or out.
	RefreshListenable core.Listenable
}

// CreateState creates the RouterState.
func (r Router) CreateState() core.State {
	return &routerState{}
}

// RouterState extends [NavigatorState] with path-based navigation methods.
//
// Obtain a RouterState using [RouterOf] from within the widget tree.
// RouterState embeds NavigatorState, so all standard navigation methods
// (Push, Pop, etc.) are available.
//
//	router := navigation.RouterOf(ctx)
//	router.Go("/products/123", nil)    // Navigate to path
//	router.Replace("/home", nil)       // Replace current route
//	router.Pop(nil)                    // Go back
type RouterState interface {
	NavigatorState // All NavigatorState methods are available

	// Go navigates to the given path, pushing a new route onto the stack.
	// Equivalent to PushNamed but with clearer intent for URL-based navigation.
	Go(path string, args any)

	// Replace replaces the current route with the given path.
	// The current route is removed and the new route takes its place.
	// Equivalent to PushReplacementNamed.
	Replace(path string, args any)
}

// routeIndex stores compiled route patterns for efficient lookup.
type routeIndex struct {
	patterns []*indexedRoute
}

type indexedRoute struct {
	pattern   *PathPattern
	route     ScreenRoute
	fullPath  string
	wraps     []func(core.BuildContext, core.Widget) core.Widget
	redirects []func(RedirectContext) RedirectResult // ancestor redirects, outermost first
}

type routerState struct {
	element     *core.StatefulElement
	router      Router
	internalNav *navigatorState
	routeIndex  *routeIndex
}

func (s *routerState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *routerState) InitState() {
	s.router = s.element.Widget().(Router)
	s.routeIndex = s.buildRouteIndex()
}

func (s *routerState) buildRouteIndex() *routeIndex {
	index := &routeIndex{}
	s.indexRoutes("", indexContext{}, s.router.Routes, index)
	return index
}

type indexContext struct {
	wraps     []func(core.BuildContext, core.Widget) core.Widget
	redirects []func(RedirectContext) RedirectResult
}

func (s *routerState) indexRoutes(prefix string, ctx indexContext, routes []ScreenRoute, index *routeIndex) {
	for _, r := range routes {
		fullPath := prefix + r.Path

		// If this route has a Screen, index it as a matchable pattern
		if r.Screen != nil {
			pattern := NewPathPattern(
				fullPath,
				WithTrailingSlash(s.router.TrailingSlashBehavior),
				WithCaseSensitivity(s.router.CaseSensitivity),
			)
			index.patterns = append(index.patterns, &indexedRoute{
				pattern:   pattern,
				route:     r,
				fullPath:  fullPath,
				wraps:     ctx.wraps,
				redirects: ctx.redirects,
			})
		}

		// Build child context: accumulate Wrap and Redirect for children
		childCtx := ctx
		if r.Wrap != nil {
			childCtx.wraps = append([]func(core.BuildContext, core.Widget) core.Widget{}, ctx.wraps...)
			childCtx.wraps = append(childCtx.wraps, r.Wrap)
		}
		if r.Redirect != nil {
			childCtx.redirects = append([]func(RedirectContext) RedirectResult{}, ctx.redirects...)
			childCtx.redirects = append(childCtx.redirects, r.Redirect)
		}

		// Recurse into children
		if len(r.Children) > 0 {
			s.indexRoutes(fullPath, childCtx, r.Children, index)
		}
	}
}

func (s *routerState) findRoute(path string) (*indexedRoute, RouteSettings) {
	// Extract query string but preserve path for pattern matching
	// (patterns handle trailing slash behavior themselves)
	_, query := ParsePath(path)

	// Get path portion without query/fragment for matching
	pathOnly := path
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		pathOnly = path[:idx]
	}

	for _, ir := range s.routeIndex.patterns {
		params, ok := ir.pattern.Match(pathOnly)
		if ok {
			return ir, RouteSettings{
				Name:   path,
				Params: params,
				Query:  query,
			}
		}
	}
	return nil, RouteSettings{}
}

func (s *routerState) generateRoute(settings RouteSettings) Route {
	ir, matchedSettings := s.findRoute(settings.Name)
	if ir == nil {
		return nil
	}

	// Merge arguments
	matchedSettings.Arguments = settings.Arguments

	// Capture for closure
	screen := ir.route.Screen
	wraps := ir.wraps

	builder := func(ctx core.BuildContext) core.Widget {
		// Build the route's widget
		child := screen(ctx, matchedSettings)

		// Apply wraps from innermost to outermost
		// (wraps slice is outermost-first, so iterate in reverse)
		for i := len(wraps) - 1; i >= 0; i-- {
			child = wraps[i](ctx, child)
		}

		return child
	}

	return NewAnimatedPageRoute(builder, matchedSettings)
}

func (s *routerState) unknownRoute(settings RouteSettings) Route {
	if s.router.ErrorBuilder == nil {
		return nil
	}

	builder := func(ctx core.BuildContext) core.Widget {
		return s.router.ErrorBuilder(ctx, settings)
	}

	return NewAnimatedPageRoute(builder, settings)
}

func (s *routerState) applyRedirect(ctx RedirectContext) RedirectResult {
	// First check router-level redirect
	if s.router.Redirect != nil {
		result := s.router.Redirect(ctx)
		if result.Path != "" {
			return result
		}
	}

	// Then check ancestor redirects (outermost first), then the route itself
	ir, _ := s.findRoute(ctx.ToPath)
	if ir != nil {
		for _, redirect := range ir.redirects {
			result := redirect(ctx)
			if result.Path != "" {
				return result
			}
		}
		if ir.route.Redirect != nil {
			return ir.route.Redirect(ctx)
		}
	}

	return NoRedirect()
}

func (s *routerState) Build(ctx core.BuildContext) core.Widget {
	initialPath := s.router.InitialPath
	if initialPath == "" {
		initialPath = "/"
	}

	// Build internal Navigator
	nav := Navigator{
		IsRoot:            true,
		InitialRoute:      initialPath,
		OnGenerateRoute:   s.generateRoute,
		OnUnknownRoute:    s.unknownRoute,
		Redirect:          s.applyRedirect,
		RefreshListenable: s.router.RefreshListenable,
	}

	// Wrap in inherited widget for RouterOf access
	return routerInherited{
		state: s,
		child: nav,
	}
}

func (s *routerState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *routerState) Dispose() {}

func (s *routerState) DidChangeDependencies() {}

func (s *routerState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	s.router = s.element.Widget().(Router)
	s.routeIndex = s.buildRouteIndex()
}

// NavigatorState interface implementation - delegate to RootNavigator

func (s *routerState) Push(route Route) {
	if nav := RootNavigator(); nav != nil {
		nav.Push(route)
	}
}

func (s *routerState) PushNamed(name string, args any) {
	if nav := RootNavigator(); nav != nil {
		nav.PushNamed(name, args)
	}
}

func (s *routerState) PushReplacementNamed(name string, args any) {
	if nav := RootNavigator(); nav != nil {
		nav.PushReplacementNamed(name, args)
	}
}

func (s *routerState) Pop(result any) {
	if nav := RootNavigator(); nav != nil {
		nav.Pop(result)
	}
}

func (s *routerState) PopUntil(predicate func(Route) bool) {
	if nav := RootNavigator(); nav != nil {
		nav.PopUntil(predicate)
	}
}

func (s *routerState) PushReplacement(route Route) {
	if nav := RootNavigator(); nav != nil {
		nav.PushReplacement(route)
	}
}

func (s *routerState) CanPop() bool {
	if nav := RootNavigator(); nav != nil {
		return nav.CanPop()
	}
	return false
}

func (s *routerState) MaybePop(result any) bool {
	if nav := RootNavigator(); nav != nil {
		return nav.MaybePop(result)
	}
	return false
}

// RouterState-specific methods

// Go navigates to the given path.
func (s *routerState) Go(path string, args any) {
	s.PushNamed(path, args)
}

// Replace replaces the current route with the given path.
func (s *routerState) Replace(path string, args any) {
	s.PushReplacementNamed(path, args)
}

// routerInherited provides RouterState to descendants.
type routerInherited struct {
	core.InheritedBase
	state *routerState
	child core.Widget
}

func (r routerInherited) ChildWidget() core.Widget { return r.child }

func (r routerInherited) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(routerInherited); ok {
		return r.state != old.state
	}
	return true
}

var routerInheritedType = reflect.TypeFor[routerInherited]()

// RouterOf returns the [RouterState] from the nearest [Router] ancestor.
//
// Use this for navigation from within the widget tree when you need the
// Router's path-based methods (Go, Replace). Returns nil if no Router
// ancestor exists.
//
//	func handleProductTap(ctx core.BuildContext, productID string) {
//	    if router := navigation.RouterOf(ctx); router != nil {
//	        router.Go("/products/"+productID, nil)
//	    }
//	}
//
// RouterState embeds [NavigatorState], so you can also use standard methods:
//
//	router := navigation.RouterOf(ctx)
//	router.Pop(nil)  // Go back
func RouterOf(ctx core.BuildContext) RouterState {
	inherited := ctx.DependOnInherited(routerInheritedType, nil)
	if inherited == nil {
		return nil
	}
	if router, ok := inherited.(routerInherited); ok {
		return router.state
	}
	return nil
}
