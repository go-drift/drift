package navigation

import (
	"testing"

	"github.com/go-drift/drift/pkg/core"
)

// mockNavigatorState implements NavigatorState for testing
type mockNavigatorState struct {
	canPopResult bool
	popCalled    bool
	popResult    any
}

func (m *mockNavigatorState) Push(route Route)                           {}
func (m *mockNavigatorState) PushNamed(name string, args any)            {}
func (m *mockNavigatorState) PushReplacementNamed(name string, args any) {}
func (m *mockNavigatorState) Pop(result any)                             { m.popCalled = true; m.popResult = result }
func (m *mockNavigatorState) PopUntil(predicate func(Route) bool)        {}
func (m *mockNavigatorState) PushReplacement(route Route)                {}
func (m *mockNavigatorState) CanPop() bool                               { return m.canPopResult }
func (m *mockNavigatorState) MaybePop(result any) bool {
	if m.canPopResult {
		m.popCalled = true
		m.popResult = result
		return true
	}
	return false
}

func TestNavigationScope_SetRoot(t *testing.T) {
	scope := &NavigationScope{}
	nav := &mockNavigatorState{}

	scope.SetRoot(nav)

	if scope.root != nav {
		t.Error("SetRoot should set root navigator")
	}
	if scope.activeNavigator != nav {
		t.Error("SetRoot should set activeNavigator when nil")
	}
}

func TestNavigationScope_SetRoot_PreservesActive(t *testing.T) {
	scope := &NavigationScope{}
	nav1 := &mockNavigatorState{}
	nav2 := &mockNavigatorState{}

	scope.SetActiveNavigator(nav1)
	scope.SetRoot(nav2)

	if scope.root != nav2 {
		t.Error("SetRoot should set root navigator")
	}
	if scope.activeNavigator != nav1 {
		t.Error("SetRoot should preserve existing activeNavigator")
	}
}

func TestNavigationScope_SetActiveNavigator(t *testing.T) {
	scope := &NavigationScope{}
	nav := &mockNavigatorState{}

	scope.SetActiveNavigator(nav)

	if scope.activeNavigator != nav {
		t.Error("SetActiveNavigator should set active navigator")
	}
}

func TestNavigationScope_ActiveNavigator(t *testing.T) {
	scope := &NavigationScope{}
	root := &mockNavigatorState{}
	active := &mockNavigatorState{}

	// When nothing is set, returns nil
	if scope.ActiveNavigator() != nil {
		t.Error("ActiveNavigator should return nil when nothing set")
	}

	// When only root is set, returns root
	scope.SetRoot(root)
	if scope.ActiveNavigator() != root {
		t.Error("ActiveNavigator should return root when no explicit active")
	}

	// When active is set, returns active
	scope.SetActiveNavigator(active)
	if scope.ActiveNavigator() != active {
		t.Error("ActiveNavigator should return active navigator")
	}
}

func TestNavigationScope_ClearActiveIf(t *testing.T) {
	scope := &NavigationScope{}
	root := &mockNavigatorState{}
	active := &mockNavigatorState{}
	other := &mockNavigatorState{}

	scope.SetRoot(root)
	scope.SetActiveNavigator(active)

	// Clearing a different navigator does nothing
	scope.ClearActiveIf(other)
	if scope.activeNavigator != active {
		t.Error("ClearActiveIf should not clear when nav doesn't match")
	}

	// Clearing the active navigator falls back to root
	scope.ClearActiveIf(active)
	if scope.activeNavigator != root {
		t.Error("ClearActiveIf should fall back to root")
	}
}

func TestNavigationScope_ClearRootIf(t *testing.T) {
	scope := &NavigationScope{}
	root := &mockNavigatorState{}
	other := &mockNavigatorState{}

	scope.SetRoot(root)

	// Clearing a different navigator does nothing
	scope.ClearRootIf(other)
	if scope.root != root {
		t.Error("ClearRootIf should not clear when nav doesn't match")
	}

	// Clearing the root navigator
	scope.ClearRootIf(root)
	if scope.root != nil {
		t.Error("ClearRootIf should clear root")
	}
}

func TestHandleBackButton_NoNavigator(t *testing.T) {
	// Save and restore global state
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	if HandleBackButton() {
		t.Error("HandleBackButton should return false when no navigator")
	}
}

func TestHandleBackButton_ActiveCanPop(t *testing.T) {
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	active := &mockNavigatorState{canPopResult: true}
	globalScope.SetActiveNavigator(active)

	if !HandleBackButton() {
		t.Error("HandleBackButton should return true when active can pop")
	}
	if !active.popCalled {
		t.Error("HandleBackButton should call MaybePop on active")
	}
}

func TestHandleBackButton_FallbackToRoot(t *testing.T) {
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	root := &mockNavigatorState{canPopResult: true}
	active := &mockNavigatorState{canPopResult: false}
	globalScope.SetRoot(root)
	globalScope.SetActiveNavigator(active)

	if !HandleBackButton() {
		t.Error("HandleBackButton should return true when root can pop")
	}
	if !root.popCalled {
		t.Error("HandleBackButton should fall back to root")
	}
}

func TestHandleBackButton_NoFallbackWhenSame(t *testing.T) {
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	// When active == root, don't try to pop root again
	nav := &mockNavigatorState{canPopResult: false}
	globalScope.SetRoot(nav)
	// active will be set to nav by SetRoot since activeNavigator was nil

	if HandleBackButton() {
		t.Error("HandleBackButton should return false when at root")
	}
}

func TestRootNavigator(t *testing.T) {
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	// Initially nil
	if RootNavigator() != nil {
		t.Error("RootNavigator should be nil when no root set")
	}

	root := &mockNavigatorState{}
	globalScope.SetRoot(root)

	if RootNavigator() != root {
		t.Error("RootNavigator should return the root navigator")
	}
}

func TestRootNavigator_IndependentOfActive(t *testing.T) {
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	root := &mockNavigatorState{}
	active := &mockNavigatorState{}

	globalScope.SetRoot(root)
	globalScope.SetActiveNavigator(active)

	// RootNavigator always returns root, regardless of active
	if RootNavigator() != root {
		t.Error("RootNavigator should return root navigator, not active")
	}
}

func TestRedirectResult_Helpers(t *testing.T) {
	// NoRedirect
	r := NoRedirect()
	if r.Path != "" {
		t.Error("NoRedirect should have empty path")
	}

	// RedirectTo
	r = RedirectTo("/login")
	if r.Path != "/login" {
		t.Errorf("RedirectTo path = %q, want /login", r.Path)
	}
	if !r.Replace {
		t.Error("RedirectTo should set Replace=true")
	}

	// RedirectWithArgs
	r = RedirectWithArgs("/login", map[string]any{"returnTo": "/dashboard"})
	if r.Path != "/login" {
		t.Errorf("RedirectWithArgs path = %q, want /login", r.Path)
	}
	if r.Arguments == nil {
		t.Error("RedirectWithArgs should set arguments")
	}
	if !r.Replace {
		t.Error("RedirectWithArgs should set Replace=true")
	}
}

// mockRoute implements Route for testing
type mockRoute struct {
	settings            RouteSettings
	didPushCalled       bool
	didPopCalled        bool
	didPopResult        any
	didChangeNextCalled bool
	didChangeNextRoute  Route
	didChangePrevCalled bool
	didChangePrevRoute  Route
	willPopResult       bool
}

func newMockRoute(name string) *mockRoute {
	return &mockRoute{
		settings:      RouteSettings{Name: name},
		willPopResult: true,
	}
}

func (m *mockRoute) Build(ctx core.BuildContext) core.Widget { return nil }
func (m *mockRoute) Settings() RouteSettings                 { return m.settings }
func (m *mockRoute) DidPush()                                { m.didPushCalled = true }
func (m *mockRoute) DidPop(result any) {
	m.didPopCalled = true
	m.didPopResult = result
}
func (m *mockRoute) DidChangeNext(nextRoute Route) {
	m.didChangeNextCalled = true
	m.didChangeNextRoute = nextRoute
}
func (m *mockRoute) DidChangePrevious(previousRoute Route) {
	m.didChangePrevCalled = true
	m.didChangePrevRoute = previousRoute
}
func (m *mockRoute) WillPop() bool { return m.willPopResult }

func TestBaseRoute_Defaults(t *testing.T) {
	r := NewBaseRoute(RouteSettings{Name: "/test", Arguments: "args"})

	if r.Settings().Name != "/test" {
		t.Errorf("Settings().Name = %q, want /test", r.Settings().Name)
	}
	if r.Settings().Arguments != "args" {
		t.Errorf("Settings().Arguments = %v, want args", r.Settings().Arguments)
	}

	// Default WillPop returns true
	if !r.WillPop() {
		t.Error("BaseRoute.WillPop() should return true by default")
	}

	// Lifecycle methods are no-ops (shouldn't panic)
	r.DidPush()
	r.DidPop(nil)
	r.DidChangeNext(nil)
	r.DidChangePrevious(nil)
}
