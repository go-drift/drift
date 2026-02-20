package navigation

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
)

func TestRouter_RouteIndex_Basic(t *testing.T) {
	// Create a router state and manually build the index
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/", Screen: stubScreen},
			{Path: "/products", Screen: stubScreen},
			{Path: "/settings", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	if len(index.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(index.patterns))
	}

	// Verify paths are indexed correctly
	paths := []string{"/", "/products", "/settings"}
	for i, ir := range index.patterns {
		if ir.fullPath != paths[i] {
			t.Errorf("Pattern %d: expected path %q, got %q", i, paths[i], ir.fullPath)
		}
	}
}

func TestRouter_RouteIndex_NestedRoutes(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/", Screen: stubScreen},
			{
				Path:   "/products",
				Screen: stubScreen,
				Children: []ScreenRoute{
					{Path: "/:id", Screen: stubScreen},
					{Path: "/:id/reviews", Screen: stubScreen},
				},
			},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	if len(index.patterns) != 4 {
		t.Errorf("Expected 4 patterns (root + products + 2 nested), got %d", len(index.patterns))
	}

	// Check nested paths are correctly concatenated
	expectedPaths := map[string]bool{
		"/":                     true,
		"/products":             true,
		"/products/:id":         true,
		"/products/:id/reviews": true,
	}

	for _, ir := range index.patterns {
		if !expectedPaths[ir.fullPath] {
			t.Errorf("Unexpected path in index: %q", ir.fullPath)
		}
		delete(expectedPaths, ir.fullPath)
	}

	if len(expectedPaths) > 0 {
		t.Errorf("Missing paths: %v", expectedPaths)
	}
}

func TestRouter_RouteIndex_Wrap(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/login", Screen: stubScreen},
			{
				Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child
				},
				Children: []ScreenRoute{
					{Path: "/home", Screen: stubScreen},
					{Path: "/profile", Screen: stubScreen},
				},
			},
		},
	}

	state := &routerState{router: router}
	index := state.buildRouteIndex()

	// Wrap-only route doesn't add patterns, just its children
	if len(index.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(index.patterns))
	}

	expectedPaths := map[string]bool{
		"/login":   true,
		"/home":    true,
		"/profile": true,
	}

	for _, ir := range index.patterns {
		if !expectedPaths[ir.fullPath] {
			t.Errorf("Unexpected path: %q", ir.fullPath)
		}
	}
}

func TestRouter_FindRoute_Basic(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/", Screen: stubScreen},
			{Path: "/products", Screen: stubScreen},
			{Path: "/products/:id", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	tests := []struct {
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{"/", true, nil},
		{"/products", true, nil},
		{"/products/123", true, map[string]string{"id": "123"}},
		{"/unknown", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ir, settings := state.findRoute(tt.path)
			gotMatch := ir != nil

			if gotMatch != tt.wantMatch {
				t.Errorf("findRoute(%q) match = %v, want %v", tt.path, gotMatch, tt.wantMatch)
			}

			if tt.wantMatch && tt.wantParams != nil {
				for k, v := range tt.wantParams {
					if settings.Params[k] != v {
						t.Errorf("findRoute(%q) param[%s] = %q, want %q", tt.path, k, settings.Params[k], v)
					}
				}
			}
		})
	}
}

func TestRouter_FindRoute_WithQuery(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/search", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, settings := state.findRoute("/search?q=hello&page=2")
	if ir == nil {
		t.Fatal("Expected to find /search route")
	}

	if settings.QueryValue("q") != "hello" {
		t.Errorf("QueryValue(q) = %q, want hello", settings.QueryValue("q"))
	}
	if settings.QueryValue("page") != "2" {
		t.Errorf("QueryValue(page) = %q, want 2", settings.QueryValue("page"))
	}
}

func TestRouter_FindRoute_TrailingSlash(t *testing.T) {
	// Default behavior: strip trailing slash
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/products/:id", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, _ := state.findRoute("/products/123/")
	if ir == nil {
		t.Error("Should match /products/123/ with trailing slash strip")
	}

	// Strict mode
	routerStrict := Router{
		TrailingSlashBehavior: TrailingSlashStrict,
		Routes: []ScreenRoute{
			{Path: "/products/:id", Screen: stubScreen},
		},
	}

	stateStrict := &routerState{router: routerStrict}
	stateStrict.routeIndex = stateStrict.buildRouteIndex()

	ir, _ = stateStrict.findRoute("/products/123/")
	if ir != nil {
		t.Error("Should not match /products/123/ with trailing slash strict")
	}

	ir, _ = stateStrict.findRoute("/products/123")
	if ir == nil {
		t.Error("Should match /products/123 without trailing slash")
	}
}

func TestRouter_FindRoute_CaseSensitivity(t *testing.T) {
	// Default: case sensitive
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/Products", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	ir, _ := state.findRoute("/products")
	if ir != nil {
		t.Error("Case sensitive: should not match /products for /Products")
	}

	ir, _ = state.findRoute("/Products")
	if ir == nil {
		t.Error("Case sensitive: should match /Products")
	}

	// Case insensitive
	routerInsensitive := Router{
		CaseSensitivity: CaseInsensitive,
		Routes: []ScreenRoute{
			{Path: "/Products", Screen: stubScreen},
		},
	}

	stateInsensitive := &routerState{router: routerInsensitive}
	stateInsensitive.routeIndex = stateInsensitive.buildRouteIndex()

	ir, _ = stateInsensitive.findRoute("/products")
	if ir == nil {
		t.Error("Case insensitive: should match /products")
	}
}

func TestRouter_ApplyRedirect_GlobalRedirect(t *testing.T) {
	router := Router{
		Redirect: func(ctx RedirectContext) RedirectResult {
			if ctx.ToPath == "/protected" {
				return RedirectTo("/login")
			}
			return NoRedirect()
		},
		Routes: []ScreenRoute{
			{Path: "/", Screen: stubScreen},
			{Path: "/login", Screen: stubScreen},
			{Path: "/protected", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Protected route should redirect
	result := state.applyRedirect(RedirectContext{ToPath: "/protected"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	// Non-protected route should not redirect
	result = state.applyRedirect(RedirectContext{ToPath: "/"})
	if result.Path != "" {
		t.Errorf("Expected no redirect, got %q", result.Path)
	}
}

func TestRouter_ApplyRedirect_RouteLevel(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{Path: "/", Screen: stubScreen},
			{
				Path:   "/admin",
				Screen: stubScreen,
				Redirect: func(ctx RedirectContext) RedirectResult {
					return RedirectTo("/login")
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// /admin has route-level redirect
	result := state.applyRedirect(RedirectContext{ToPath: "/admin"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	// / has no redirect
	result = state.applyRedirect(RedirectContext{ToPath: "/"})
	if result.Path != "" {
		t.Errorf("Expected no redirect, got %q", result.Path)
	}
}

func TestRouter_ApplyRedirect_GlobalOverridesRoute(t *testing.T) {
	router := Router{
		Redirect: func(ctx RedirectContext) RedirectResult {
			// Global redirect always sends to /maintenance
			return RedirectTo("/maintenance")
		},
		Routes: []ScreenRoute{
			{
				Path:   "/admin",
				Screen: stubScreen,
				Redirect: func(ctx RedirectContext) RedirectResult {
					return RedirectTo("/login")
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Global redirect takes precedence
	result := state.applyRedirect(RedirectContext{ToPath: "/admin"})
	if result.Path != "/maintenance" {
		t.Errorf("Expected global redirect to /maintenance, got %q", result.Path)
	}
}

func TestRouter_ApplyRedirect_GroupRedirect(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{
				// Group-level redirect guards all children
				Redirect: func(ctx RedirectContext) RedirectResult {
					return RedirectTo("/login")
				},
				Children: []ScreenRoute{
					{Path: "/admin/users", Screen: stubScreen},
					{Path: "/admin/settings", Screen: stubScreen},
				},
			},
			{Path: "/public", Screen: stubScreen},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Children of the guarded group should redirect
	result := state.applyRedirect(RedirectContext{ToPath: "/admin/users"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	result = state.applyRedirect(RedirectContext{ToPath: "/admin/settings"})
	if result.Path != "/login" {
		t.Errorf("Expected redirect to /login, got %q", result.Path)
	}

	// Route outside the group should not redirect
	result = state.applyRedirect(RedirectContext{ToPath: "/public"})
	if result.Path != "" {
		t.Errorf("Expected no redirect, got %q", result.Path)
	}
}

func TestRouter_DeeplyNestedRoutes(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{
				Path:   "/api",
				Screen: stubScreen,
				Children: []ScreenRoute{
					{
						Path:   "/v1",
						Screen: stubScreen,
						Children: []ScreenRoute{
							{Path: "/users", Screen: stubScreen},
							{Path: "/users/:id", Screen: stubScreen},
						},
					},
					{Path: "/v2", Screen: stubScreen},
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	tests := []struct {
		path      string
		wantMatch bool
	}{
		{"/api", true},
		{"/api/v1", true},
		{"/api/v1/users", true},
		{"/api/v1/users/42", true},
		{"/api/v2", true},
		{"/api/v3", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ir, _ := state.findRoute(tt.path)
			gotMatch := ir != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("findRoute(%q) = %v, want %v", tt.path, gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRouter_Wrap_WrapsTracked(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{
				Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child // Simplified for test
				},
				Children: []ScreenRoute{
					{Path: "/home", Screen: stubScreen},
					{Path: "/profile", Screen: stubScreen},
				},
			},
			{Path: "/login", Screen: stubScreen}, // Outside wrap
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Routes inside wrap should have wrap tracked
	ir, _ := state.findRoute("/home")
	if ir == nil {
		t.Fatal("Should find /home")
	}
	if len(ir.wraps) != 1 {
		t.Errorf("/home should have 1 wrap, got %d", len(ir.wraps))
	}

	ir, _ = state.findRoute("/profile")
	if ir == nil {
		t.Fatal("Should find /profile")
	}
	if len(ir.wraps) != 1 {
		t.Errorf("/profile should have 1 wrap, got %d", len(ir.wraps))
	}

	// Route outside wrap should have no wraps
	ir, _ = state.findRoute("/login")
	if ir == nil {
		t.Fatal("Should find /login")
	}
	if len(ir.wraps) != 0 {
		t.Errorf("/login should have 0 wraps, got %d", len(ir.wraps))
	}
}

func TestRouter_Wrap_NestedWraps(t *testing.T) {
	router := Router{
		Routes: []ScreenRoute{
			{
				Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child // Outer wrap
				},
				Children: []ScreenRoute{
					{
						Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
							return child // Inner wrap
						},
						Children: []ScreenRoute{
							{Path: "/nested", Screen: stubScreen},
						},
					},
					{Path: "/single-wrap", Screen: stubScreen},
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// Route in nested wraps should have both wraps
	ir, _ := state.findRoute("/nested")
	if ir == nil {
		t.Fatal("Should find /nested")
	}
	if len(ir.wraps) != 2 {
		t.Errorf("/nested should have 2 wraps, got %d", len(ir.wraps))
	}

	// Route in single wrap should have 1 wrap
	ir, _ = state.findRoute("/single-wrap")
	if ir == nil {
		t.Fatal("Should find /single-wrap")
	}
	if len(ir.wraps) != 1 {
		t.Errorf("/single-wrap should have 1 wrap, got %d", len(ir.wraps))
	}
}

func TestRouter_Wrap_ScreenAndWrapCombined(t *testing.T) {
	// A route with both Screen and Wrap: the Screen is indexed with the
	// parent's wraps (not its own), while Children get this route's Wrap.
	router := Router{
		Routes: []ScreenRoute{
			{
				Path:   "/dashboard",
				Screen: stubScreen,
				Wrap: func(ctx core.BuildContext, child core.Widget) core.Widget {
					return child
				},
				Children: []ScreenRoute{
					{Path: "/stats", Screen: stubScreen},
				},
			},
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// /dashboard itself should have no wraps (Wrap applies to children only)
	ir, _ := state.findRoute("/dashboard")
	if ir == nil {
		t.Fatal("Should find /dashboard")
	}
	if len(ir.wraps) != 0 {
		t.Errorf("/dashboard should have 0 wraps, got %d", len(ir.wraps))
	}

	// /dashboard/stats should be wrapped
	ir, _ = state.findRoute("/dashboard/stats")
	if ir == nil {
		t.Fatal("Should find /dashboard/stats")
	}
	if len(ir.wraps) != 1 {
		t.Errorf("/dashboard/stats should have 1 wrap, got %d", len(ir.wraps))
	}
}

func TestRouter_TrailingSlash_RequiresTrailingSlash(t *testing.T) {
	// In strict mode, pattern ending with / should require trailing slash
	router := Router{
		TrailingSlashBehavior: TrailingSlashStrict,
		Routes: []ScreenRoute{
			{Path: "/products/", Screen: stubScreen}, // Requires trailing slash
			{Path: "/users", Screen: stubScreen},     // No trailing slash
		},
	}

	state := &routerState{router: router}
	state.routeIndex = state.buildRouteIndex()

	// /products/ pattern should match /products/ but not /products
	ir, _ := state.findRoute("/products/")
	if ir == nil {
		t.Error("Should match /products/ with trailing slash")
	}

	ir, _ = state.findRoute("/products")
	if ir != nil {
		t.Error("Should NOT match /products without trailing slash when pattern has it")
	}

	// /users pattern should match /users but not /users/
	ir, _ = state.findRoute("/users")
	if ir == nil {
		t.Error("Should match /users without trailing slash")
	}

	ir, _ = state.findRoute("/users/")
	if ir != nil {
		t.Error("Should NOT match /users/ with trailing slash when pattern doesn't have it")
	}
}

// stubScreen is a minimal Screen builder for tests that only need route indexing/matching.
func stubScreen(_ core.BuildContext, _ RouteSettings) core.Widget { return nil }
