package navigation

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/widgets"
)

// Tab configures a single tab in a TabScaffold.
type Tab struct {
	Item            widgets.TabItem
	Builder         func(ctx core.BuildContext) core.Widget
	InitialRoute    string
	OnGenerateRoute func(settings RouteSettings) Route
	OnUnknownRoute  func(settings RouteSettings) Route
	Observers       []NavigatorObserver
}

// NewTab creates a Tab with a simple root builder.
func NewTab(item widgets.TabItem, builder func(ctx core.BuildContext) core.Widget) Tab {
	return Tab{
		Item:    item,
		Builder: builder,
	}
}

// TabScaffold hosts tab navigation with a separate Navigator per tab.
type TabScaffold struct {
	Tabs       []Tab
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
}

func (s *tabScaffoldState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *tabScaffoldState) InitState() {
	s.scaffold = s.element.Widget().(TabScaffold)
	s.configureController()
}

func (s *tabScaffoldState) Build(ctx core.BuildContext) core.Widget {
	if len(s.scaffold.Tabs) == 0 {
		return widgets.SizedBox{}
	}

	index := s.validatedIndex()
	tabItems := make([]widgets.TabItem, len(s.scaffold.Tabs))
	bodies := make([]core.Widget, len(s.scaffold.Tabs))

	for i, tab := range s.scaffold.Tabs {
		tabItems[i] = tab.Item
		bodies[i] = s.buildTabNavigator(tab)
	}

	return widgets.Column{
		ChildrenWidgets: []core.Widget{
			widgets.Expanded{
				ChildWidget: widgets.IndexedStack{
					ChildrenWidgets: bodies,
					Alignment:       layout.AlignmentTopLeft,
					Fit:             widgets.StackFitExpand,
					Index:           index,
				},
			},
			widgets.TabBar{
				Items:        tabItems,
				CurrentIndex: index,
				OnTap: func(tabIndex int) {
					s.controller.SetIndex(tabIndex)
				},
			},
		},
		MainAxisAlignment: widgets.MainAxisAlignmentStart,
		MainAxisSize:      widgets.MainAxisSizeMax,
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
	s.scaffold = s.element.Widget().(TabScaffold)
	s.configureController()
}

func (s *tabScaffoldState) configureController() {
	s.detachController()

	controller := s.scaffold.Controller
	if controller == nil {
		controller = NewTabController(0)
	}

	s.controller = controller
	s.unsubscribeController = s.controller.AddListener(func(_ int) {
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
