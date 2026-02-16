package navigation

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// Tab configures a single tab in a [TabScaffold].
//
// For simple tabs with a single screen, use [NewTab]. For tabs with their own
// navigation stack, configure OnGenerateRoute.
type Tab struct {
	// Item defines the tab's appearance in the tab bar.
	Item widgets.TabItem

	// Builder creates the tab's root widget.
	// Used when OnGenerateRoute is nil to create a simple single-screen tab.
	Builder func(ctx core.BuildContext) core.Widget

	// InitialRoute is the starting route for this tab's navigator.
	// Defaults to "/" if not specified.
	InitialRoute string

	// OnGenerateRoute creates routes for this tab's navigation stack.
	// If nil, a simple navigator is created using Builder for the initial route.
	OnGenerateRoute func(settings RouteSettings) Route

	// OnUnknownRoute handles navigation to undefined routes within this tab.
	OnUnknownRoute func(settings RouteSettings) Route

	// Observers receive navigation events for this tab's navigator.
	Observers []NavigatorObserver
}

// NewTab creates a Tab with a simple root builder.
//
// Use this for tabs that don't need their own navigation stack. For tabs with
// multiple screens, create a Tab with OnGenerateRoute instead.
//
//	navigation.NewTab(
//	    widgets.TabItem{Label: "Home", Icon: homeIcon},
//	    buildHomeScreen,
//	)
func NewTab(item widgets.TabItem, builder func(ctx core.BuildContext) core.Widget) Tab {
	return Tab{
		Item:    item,
		Builder: builder,
	}
}

// TabScaffold provides bottom tab navigation with separate navigation stacks
// per tab.
//
// Each tab has its own [Navigator], allowing independent navigation within tabs.
// When the user switches tabs, the tab's navigation state is preserved.
// TabScaffold automatically manages which tab's navigator is "active" for
// back button handling via [NavigationScope].
//
// Basic usage:
//
//	navigation.TabScaffold{
//	    Tabs: []navigation.Tab{
//	        navigation.NewTab(
//	            widgets.TabItem{Label: "Home"},
//	            buildHomeScreen,
//	        ),
//	        navigation.NewTab(
//	            widgets.TabItem{Label: "Profile"},
//	            buildProfileScreen,
//	        ),
//	    },
//	}
//
// With navigation stacks per tab:
//
//	navigation.Tab{
//	    Item:         widgets.TabItem{Label: "Products"},
//	    InitialRoute: "/products",
//	    OnGenerateRoute: func(settings navigation.RouteSettings) navigation.Route {
//	        switch settings.Name {
//	        case "/products":
//	            return navigation.NewMaterialPageRoute(buildProductList, settings)
//	        case "/products/detail":
//	            return navigation.NewMaterialPageRoute(buildProductDetail, settings)
//	        }
//	        return nil
//	    },
//	}
//
// Accessibility: Inactive tabs are automatically excluded from the accessibility
// tree using [widgets.ExcludeSemantics].
type TabScaffold struct {
	// Tabs defines the tab configuration. At least one tab is required.
	Tabs []Tab

	// Controller optionally provides programmatic control over tab selection.
	// If nil, a default controller starting at index 0 is created.
	Controller *TabController
}

func (t TabScaffold) CreateElement() core.Element {
	return core.NewStatefulElement(t, nil)
}

func (t TabScaffold) Key() any {
	return nil
}

func (t TabScaffold) CreateState() core.State {
	return &tabScaffoldState{}
}

type tabScaffoldState struct {
	element               *core.StatefulElement
	scaffold              TabScaffold
	controller            *TabController
	unsubscribeController func()
	tabNavigators         []NavigatorState // Store each tab's navigator
	currentIndex          int              // Track current tab for active navigator
}

func (s *tabScaffoldState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *tabScaffoldState) InitState() {
	s.scaffold = s.element.Widget().(TabScaffold)
	s.tabNavigators = make([]NavigatorState, len(s.scaffold.Tabs))
	s.configureController()
}

func (s *tabScaffoldState) Build(ctx core.BuildContext) core.Widget {
	if len(s.scaffold.Tabs) == 0 {
		return widgets.SizedBox{}
	}

	index := s.validatedIndex()
	s.currentIndex = index
	tabItems := make([]widgets.TabItem, len(s.scaffold.Tabs))
	bodies := make([]core.Widget, len(s.scaffold.Tabs))

	for i, tab := range s.scaffold.Tabs {
		tabItems[i] = tab.Item
		isActive := i == index

		// Wrap each tab's navigator with registration callback and accessibility handling
		bodies[i] = widgets.ExcludeSemantics{
			Excluding: !isActive,
			Child: widgets.Offstage{
				Offstage: !isActive,
				Child: tabNavigatorScope{
					scaffoldState: s,
					tabIndex:      i,
					child:         s.buildTabNavigator(tab),
				},
			},
		}
	}

	return widgets.Column{
		Children: []core.Widget{
			widgets.Expanded{
				Child: widgets.IndexedStack{
					Children:  bodies,
					Alignment: layout.AlignmentTopLeft,
					Fit:       widgets.StackFitExpand,
					Index:     index,
				},
			},
			widgets.SafeArea{
				Bottom: true,
				Child:  theme.TabBarOf(ctx, tabItems, index, func(tabIndex int) { s.controller.SetIndex(tabIndex) }),
			},
		},
		MainAxisAlignment:  widgets.MainAxisAlignmentStart,
		MainAxisSize:       widgets.MainAxisSizeMax,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStretch,
	}
}

// validatedIndex returns the current tab index, clamping to valid range.
func (s *tabScaffoldState) validatedIndex() int {
	index := s.controller.Index()
	if index < 0 || index >= len(s.scaffold.Tabs) {
		s.controller.SetIndex(0)
		return 0
	}
	return index
}

// buildTabNavigator creates a Navigator for the given tab configuration.
func (s *tabScaffoldState) buildTabNavigator(tab Tab) Navigator {
	initialRoute := tab.InitialRoute
	if initialRoute == "" {
		initialRoute = "/"
	}

	onGenerate := tab.OnGenerateRoute
	if onGenerate == nil && tab.Builder != nil {
		builder := tab.Builder
		initial := initialRoute
		onGenerate = func(settings RouteSettings) Route {
			if settings.Name == initial {
				return NewMaterialPageRoute(builder, settings)
			}
			return nil
		}
	}

	return Navigator{
		InitialRoute:    initialRoute,
		OnGenerateRoute: onGenerate,
		OnUnknownRoute:  tab.OnUnknownRoute,
		Observers:       tab.Observers,
	}
}

func (s *tabScaffoldState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *tabScaffoldState) Dispose() {
	s.detachController()
}

func (s *tabScaffoldState) DidChangeDependencies() {}

func (s *tabScaffoldState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	oldScaffold := s.scaffold
	s.scaffold = s.element.Widget().(TabScaffold)

	// Resize tabNavigators if tab count changed
	if len(s.scaffold.Tabs) != len(oldScaffold.Tabs) {
		newNavigators := make([]NavigatorState, len(s.scaffold.Tabs))
		// Preserve existing navigators where possible
		for i := 0; i < len(newNavigators) && i < len(s.tabNavigators); i++ {
			newNavigators[i] = s.tabNavigators[i]
		}
		s.tabNavigators = newNavigators
	}

	s.configureController()
}

func (s *tabScaffoldState) configureController() {
	s.detachController()

	controller := s.scaffold.Controller
	if controller == nil {
		controller = NewTabController(0)
	}

	s.controller = controller
	s.unsubscribeController = s.controller.AddListener(func(index int) {
		s.onTabChanged(index)
		s.SetState(func() {})
	})
}

func (s *tabScaffoldState) detachController() {
	if s.unsubscribeController != nil {
		s.unsubscribeController()
		s.unsubscribeController = nil
	}
	s.controller = nil
}

// registerTabNavigator stores a tab's navigator and sets it as active if needed.
func (s *tabScaffoldState) registerTabNavigator(index int, nav NavigatorState) {
	if index < 0 || index >= len(s.tabNavigators) {
		return
	}
	s.tabNavigators[index] = nav

	// If this is the active tab, set it as the active navigator
	if index == s.currentIndex {
		globalScope.SetActiveNavigator(nav)
	}
}

// onTabChanged updates the active navigator when tabs change.
func (s *tabScaffoldState) onTabChanged(index int) {
	s.currentIndex = index
	// Set the active tab's navigator as the focused one
	if index >= 0 && index < len(s.tabNavigators) && s.tabNavigators[index] != nil {
		globalScope.SetActiveNavigator(s.tabNavigators[index])
	}
}

// tabNavigatorScope provides a way for child navigators to register with TabScaffold.
type tabNavigatorScope struct {
	scaffoldState *tabScaffoldState
	tabIndex      int
	child         core.Widget
}

func (t tabNavigatorScope) CreateElement() core.Element {
	return core.NewInheritedElement(t, nil)
}

func (t tabNavigatorScope) Key() any {
	return t.tabIndex
}

func (t tabNavigatorScope) ChildWidget() core.Widget {
	return t.child
}

func (t tabNavigatorScope) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(tabNavigatorScope); ok {
		return t.tabIndex != old.tabIndex || t.scaffoldState != old.scaffoldState
	}
	return true
}

func (t tabNavigatorScope) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return t.UpdateShouldNotify(oldWidget)
}

var tabNavigatorScopeType = reflect.TypeFor[tabNavigatorScope]()

// RegisterTabNavigator registers a navigator with its enclosing [TabScaffold].
//
// This is called automatically by [Navigator] during Build when inside a
// TabScaffold. You typically don't need to call this directly.
//
// Registration enables TabScaffold to track which navigator is active and
// should receive back button events.
func RegisterTabNavigator(ctx core.BuildContext, nav NavigatorState) {
	tryRegisterTabNavigator(ctx, nav)
}

// tryRegisterTabNavigator attempts to register a navigator with its enclosing
// TabScaffold. Returns true if inside a TabScaffold and registration occurred,
// false otherwise.
func tryRegisterTabNavigator(ctx core.BuildContext, nav NavigatorState) bool {
	inherited := ctx.DependOnInherited(tabNavigatorScopeType, nil)
	if inherited == nil {
		return false
	}
	if scope, ok := inherited.(tabNavigatorScope); ok {
		scope.scaffoldState.registerTabNavigator(scope.tabIndex, nav)
		return true
	}
	return false
}
