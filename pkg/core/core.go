package core

import "reflect"

// Widget is an immutable description of part of the user interface.
//
// Widgets are the building blocks of the UI. They describe what the interface
// should look like given the current configuration and state. Widgets themselves
// are immutable - when the UI needs to change, new widget instances are created.
//
// There are three main types of widgets:
//   - [StatelessWidget]: Builds UI from configuration alone, no mutable state
//   - [StatefulWidget]: Has mutable [State] that can change over time
//   - [InheritedWidget]: Provides data to descendants without explicit passing
//
// Widgets create [Element] instances that manage the actual UI lifecycle.
type Widget interface {
	// CreateElement creates the element that will manage this widget's lifecycle.
	CreateElement() Element
	// Key returns an optional key for widget identity. When non-nil, the framework
	// uses the key to match widgets across rebuilds, preserving state and avoiding
	// unnecessary recreation. Use keys for widgets in lists or when widget identity
	// matters across tree restructuring.
	Key() any
}

// StatelessWidget builds UI without mutable state.
//
// Use StatelessWidget when the widget's appearance depends only on its configuration
// (the struct fields) and inherited data from ancestors. The widget rebuilds when
// its parent rebuilds with different configuration or when an inherited dependency
// changes.
//
// Example:
//
//	type Greeting struct {
//	    Name string
//	}
//
//	func (g Greeting) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: "Hello, " + g.Name}
//	}
//
//	func (g Greeting) CreateElement() core.Element {
//	    return core.NewStatelessElement(g, nil)
//	}
//
//	func (g Greeting) Key() any { return nil }
type StatelessWidget interface {
	Widget
	// Build creates the widget tree for this widget's portion of the UI.
	Build(ctx BuildContext) Widget
}

// StatefulWidget has mutable state that persists across rebuilds.
//
// Use StatefulWidget when the widget needs to maintain state that can change
// during its lifetime, such as user input, animation values, or data fetched
// asynchronously.
//
// StatefulWidget creates a [State] object that holds the mutable data. The State
// persists even when the widget is rebuilt with new configuration, allowing it
// to maintain continuity.
//
// Example:
//
//	type Counter struct{}
//
//	func (c Counter) CreateState() core.State {
//	    return &counterState{}
//	}
//
//	type counterState struct {
//	    element *core.StatefulElement
//	    count   int
//	}
//
//	func (s *counterState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Button{
//	        Label: fmt.Sprintf("Count: %d", s.count),
//	        OnTap: func() {
//	            s.SetState(func() { s.count++ })
//	        },
//	    }
//	}
type StatefulWidget interface {
	Widget
	// CreateState creates the mutable state for this widget.
	CreateState() State
}

// State holds mutable state for a [StatefulWidget].
//
// State objects are long-lived - they persist across widget rebuilds as long as
// the widget remains in the tree at the same location. This allows state like
// user input, scroll position, or animation values to be preserved.
//
// # Lifecycle
//
// State lifecycle methods are called in this order:
//  1. SetElement - Framework sets the element reference
//  2. InitState - Called once when state is first created
//  3. DidChangeDependencies - Called when inherited dependencies change
//  4. Build - Called to create the widget tree (may be called many times)
//  5. DidUpdateWidget - Called when parent rebuilds with new widget configuration
//  6. Dispose - Called when the widget is permanently removed from the tree
//
// # Triggering Rebuilds
//
// Call SetState with a function that modifies state to schedule a rebuild:
//
//	s.SetState(func() {
//	    s.count++
//	})
type State interface {
	// InitState is called once when the state is first created.
	// Use this to initialize state that depends on the widget configuration.
	InitState()
	// Build creates the widget tree. Called whenever the state changes or
	// the parent rebuilds with new widget configuration.
	Build(ctx BuildContext) Widget
	// SetState schedules a rebuild after executing the provided function.
	// The function should modify state variables that affect the Build output.
	SetState(fn func())
	// Dispose is called when the widget is permanently removed from the tree.
	// Use this to release resources like stream subscriptions or controllers.
	Dispose()
	// DidChangeDependencies is called when an inherited dependency changes.
	// Called after InitState and whenever an InheritedWidget ancestor notifies.
	DidChangeDependencies()
	// DidUpdateWidget is called when the parent rebuilds with a new widget
	// configuration. The old widget is passed for comparison.
	DidUpdateWidget(oldWidget StatefulWidget)
}

// InheritedWidget provides data to descendant widgets without explicit parameter
// passing through the widget tree.
//
// InheritedWidget is the foundation for dependency injection in the widget tree.
// Descendant widgets can access the inherited data via [BuildContext.DependOnInherited],
// and they automatically rebuild when the inherited data changes.
//
// # When to Use
//
// Use InheritedWidget when data needs to be accessed by many widgets at different
// levels of the tree. Common examples include:
//   - Theme data (colors, typography)
//   - Localization strings
//   - User authentication state
//   - Application configuration
//
// # Simple Usage with InheritedProvider
//
// For simple cases, use the generic [InheritedProvider] which eliminates boilerplate:
//
//	// Provide data
//	core.InheritedProvider[*UserState]{
//	    Value: userState,
//	    Child: MyApp{},
//	}
//
//	// Access data (in a descendant's Build method)
//	if user, ok := core.ProviderOf[*UserState](ctx); ok {
//	    // use user
//	}
//
// # Custom Implementation
//
// For advanced cases like aspect-based selective rebuilds, implement InheritedWidget
// directly:
//
//	type MyTheme struct {
//	    Colors     ColorScheme
//	    Typography TextTheme
//	    Child      core.Widget
//	}
//
//	func (t MyTheme) UpdateShouldNotify(old core.InheritedWidget) bool {
//	    return true // Check any aspect changed
//	}
//
//	func (t MyTheme) UpdateShouldNotifyDependent(old core.InheritedWidget, aspects map[any]struct{}) bool {
//	    oldTheme := old.(MyTheme)
//	    if _, ok := aspects["colors"]; ok && t.Colors != oldTheme.Colors {
//	        return true
//	    }
//	    if _, ok := aspects["typography"]; ok && t.Typography != oldTheme.Typography {
//	        return true
//	    }
//	    return false
//	}
//
// Dependents register aspects via [BuildContext.DependOnInherited] with a non-nil aspect.
type InheritedWidget interface {
	Widget
	// ChildWidget returns the child widget tree.
	ChildWidget() Widget
	// UpdateShouldNotify returns true if dependents should potentially rebuild
	// when this widget is updated. This is a coarse-grained gate - if it returns
	// false, no dependents are notified. Return true when any aspect might have
	// changed, then use UpdateShouldNotifyDependent for fine-grained filtering.
	UpdateShouldNotify(oldWidget InheritedWidget) bool
	// UpdateShouldNotifyDependent returns true if a specific dependent should rebuild
	// based on the aspects it registered via DependOnInherited. This enables granular
	// rebuild optimization where dependents only rebuild when their specific aspects
	// change. The aspects map contains all aspects the dependent registered.
	UpdateShouldNotifyDependent(oldWidget InheritedWidget, aspects map[any]struct{}) bool
}

// BuildContext provides access to the widget tree during the build phase.
//
// BuildContext is passed to [StatelessWidget.Build] and [State.Build] methods,
// providing access to inherited data and ancestor widgets.
//
// The most common use is accessing [InheritedWidget] data:
//
//	func (w MyWidget) Build(ctx core.BuildContext) core.Widget {
//	    theme := theme.ThemeOf(ctx)  // Uses DependOnInherited internally
//	    return Text{Content: "Hello", Style: theme.TextTheme.BodyLarge}
//	}
type BuildContext interface {
	// Widget returns the widget that created this context.
	Widget() Widget
	// FindAncestor walks up the tree and returns the first element matching
	// the predicate, or nil if none is found.
	FindAncestor(predicate func(Element) bool) Element
	// DependOnInherited finds the nearest ancestor [InheritedWidget] of the given
	// type and registers this context as a dependent. When the inherited widget
	// updates and notifies dependents, this widget will rebuild.
	//
	// The aspect parameter enables granular dependency tracking: when non-nil,
	// only changes affecting that aspect trigger rebuilds. Pass nil to rebuild
	// on any change. Use [reflect.TypeOf] to get the inherited widget's type:
	//
	//	themeType := reflect.TypeOf(MyTheme{})
	//	widget := ctx.DependOnInherited(themeType, "colors")
	//
	// For simpler access, use [InheritedProvider] with [ProviderOf].
	DependOnInherited(inheritedType reflect.Type, aspect any) any
	// DependOnInheritedWithAspects is like DependOnInherited but registers multiple
	// aspects in a single tree walk. More efficient when depending on several aspects
	// of the same inherited widget.
	DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any
}

// Element is the instantiation of a [Widget] at a specific location in the tree.
//
// While widgets are immutable configuration objects, elements are the mutable
// counterparts that manage the widget's lifecycle in the tree. Each widget creates
// an element when mounted, and the element persists across widget rebuilds as long
// as the widget type and key match.
//
// Elements form a tree parallel to the widget tree. When a parent widget rebuilds,
// the framework compares new widgets against existing elements to determine whether
// to update, replace, or remove elements.
//
// Most developers don't interact with elements directly - the framework manages
// them automatically. Custom elements are only needed for advanced use cases like
// building new layout primitives.
type Element interface {
	// Widget returns the current widget configuration for this element.
	Widget() Widget
	// Mount attaches this element to the tree under the given parent.
	Mount(parent Element, slot any)
	// Update reconfigures this element with a new widget of the same type.
	Update(newWidget Widget)
	// Unmount removes this element from the tree permanently.
	Unmount()
	// MarkNeedsBuild schedules this element for rebuild in the next frame.
	MarkNeedsBuild()
	// RebuildIfNeeded performs the rebuild if this element is marked dirty.
	RebuildIfNeeded()
	// VisitChildren calls the visitor for each child element.
	VisitChildren(visitor func(Element) bool)
	// Depth returns this element's depth in the tree (root is 0).
	Depth() int
	// Slot returns the slot identifier for this element in its parent.
	Slot() any
	// UpdateSlot changes the slot identifier for this element.
	UpdateSlot(newSlot any)
}
