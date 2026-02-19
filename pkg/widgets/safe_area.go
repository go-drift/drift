package widgets

import (
	"reflect"
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// SafeAreaAspect identifies which safe area inset a widget depends on.
type SafeAreaAspect int

const (
	SafeAreaAspectTop SafeAreaAspect = iota
	SafeAreaAspectBottom
	SafeAreaAspectLeft
	SafeAreaAspectRight
)

// SafeAreaData provides safe area insets to descendants via InheritedWidget.
// It implements [core.AspectAwareInheritedWidget] for granular per-edge tracking.
type SafeAreaData struct {
	core.InheritedBase
	Insets layout.EdgeInsets
	Child  core.Widget
}

func (s SafeAreaData) ChildWidget() core.Widget { return s.Child }

func (s SafeAreaData) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(SafeAreaData); ok {
		return s.Insets != old.Insets
	}
	return true
}

func (s SafeAreaData) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	old, ok := oldWidget.(SafeAreaData)
	if !ok {
		return true
	}
	for aspect := range aspects {
		switch aspect.(SafeAreaAspect) {
		case SafeAreaAspectTop:
			if s.Insets.Top != old.Insets.Top {
				return true
			}
		case SafeAreaAspectBottom:
			if s.Insets.Bottom != old.Insets.Bottom {
				return true
			}
		case SafeAreaAspectLeft:
			if s.Insets.Left != old.Insets.Left {
				return true
			}
		case SafeAreaAspectRight:
			if s.Insets.Right != old.Insets.Right {
				return true
			}
		}
	}
	return false
}

// SafeAreaProvider is a StatefulWidget that subscribes to platform safe area changes
// and provides SafeAreaData to descendants. This scopes rebuilds to only the provider
// and widgets that depend on safe area data, instead of rebuilding the entire tree.
type SafeAreaProvider struct {
	core.StatefulBase

	Child core.Widget
}

func (s SafeAreaProvider) CreateState() core.State {
	return &safeAreaProviderState{}
}

type safeAreaProviderState struct {
	core.StateBase
	insets      layout.EdgeInsets
	unsubscribe func()
	mu          sync.Mutex
	pending     layout.EdgeInsets
	hasPending  bool
}

func (s *safeAreaProviderState) InitState() {
	// Read initial insets
	platformInsets := platform.SafeArea.Insets()
	s.insets = layout.EdgeInsets{
		Top:    platformInsets.Top,
		Bottom: platformInsets.Bottom,
		Left:   platformInsets.Left,
		Right:  platformInsets.Right,
	}

	// Subscribe to changes
	s.unsubscribe = platform.SafeArea.AddHandler(s.onPlatformInsetsChanged)
	s.OnDispose(func() {
		if s.unsubscribe != nil {
			s.unsubscribe()
		}
	})
}

func (s *safeAreaProviderState) onPlatformInsetsChanged(insets platform.EdgeInsets) {
	newInsets := layout.EdgeInsets{
		Top:    insets.Top,
		Bottom: insets.Bottom,
		Left:   insets.Left,
		Right:  insets.Right,
	}

	// Batch rapid updates
	s.mu.Lock()
	s.pending = newInsets
	shouldSchedule := !s.hasPending
	s.hasPending = true
	s.mu.Unlock()

	if shouldSchedule {
		if !platform.Dispatch(s.applyPendingInsets) {
			// Dispatch not available - clear hasPending so future updates can retry
			s.mu.Lock()
			s.hasPending = false
			s.mu.Unlock()
		}
	}
}

func (s *safeAreaProviderState) applyPendingInsets() {
	s.mu.Lock()
	newInsets := s.pending
	s.hasPending = false
	s.mu.Unlock()

	if s.insets == newInsets {
		return
	}
	s.SetState(func() { s.insets = newInsets })
}

func (s *safeAreaProviderState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(SafeAreaProvider)
	return SafeAreaData{
		Insets: s.insets,
		Child:  w.Child,
	}
}

var _ core.AspectAwareInheritedWidget = SafeAreaData{}

var safeAreaDataType = reflect.TypeFor[SafeAreaData]()

// SafeAreaOf returns the current safe area insets from context.
// Widgets calling this will rebuild when any inset changes.
func SafeAreaOf(ctx core.BuildContext) layout.EdgeInsets {
	// Register all aspects in a single tree walk for efficiency
	inherited := ctx.DependOnInheritedWithAspects(safeAreaDataType,
		SafeAreaAspectTop, SafeAreaAspectBottom, SafeAreaAspectLeft, SafeAreaAspectRight)
	if sa, ok := inherited.(SafeAreaData); ok {
		return sa.Insets
	}
	return layout.EdgeInsets{}
}

// SafeAreaTopOf returns only the top safe area inset.
// Widgets calling this will only rebuild when the top inset changes.
func SafeAreaTopOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(safeAreaDataType, SafeAreaAspectTop)
	if sa, ok := inherited.(SafeAreaData); ok {
		return sa.Insets.Top
	}
	return 0
}

// SafeAreaBottomOf returns only the bottom safe area inset.
// Widgets calling this will only rebuild when the bottom inset changes.
func SafeAreaBottomOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(safeAreaDataType, SafeAreaAspectBottom)
	if sa, ok := inherited.(SafeAreaData); ok {
		return sa.Insets.Bottom
	}
	return 0
}

// SafeAreaLeftOf returns only the left safe area inset.
// Widgets calling this will only rebuild when the left inset changes.
func SafeAreaLeftOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(safeAreaDataType, SafeAreaAspectLeft)
	if sa, ok := inherited.(SafeAreaData); ok {
		return sa.Insets.Left
	}
	return 0
}

// SafeAreaRightOf returns only the right safe area inset.
// Widgets calling this will only rebuild when the right inset changes.
func SafeAreaRightOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(safeAreaDataType, SafeAreaAspectRight)
	if sa, ok := inherited.(SafeAreaData); ok {
		return sa.Insets.Right
	}
	return 0
}

// SafeAreaPadding returns the safe area insets as EdgeInsets for use with
// ScrollView.Padding or other widgets. The returned EdgeInsets can be modified
// using chainable methods:
//
//	ScrollView{
//	    Padding: widgets.SafeAreaPadding(ctx),              // just safe area
//	    Child: ...,
//	}
//	ScrollView{
//	    Padding: widgets.SafeAreaPadding(ctx).Add(24),      // safe area + 24px all sides
//	    Child: ...,
//	}
//	ScrollView{
//	    Padding: widgets.SafeAreaPadding(ctx).OnlyTop().Add(24), // only top safe area + 24px
//	    Child: ...,
//	}
func SafeAreaPadding(ctx core.BuildContext) layout.EdgeInsets {
	return SafeAreaOf(ctx)
}

// SafeArea is a convenience widget that applies safe area insets as padding.
type SafeArea struct {
	core.StatelessBase

	Top    bool
	Bottom bool
	Left   bool
	Right  bool
	Child  core.Widget
}

func (s SafeArea) Build(ctx core.BuildContext) core.Widget {
	// Default to applying all sides if none specified
	top, bottom, left, right := s.Top, s.Bottom, s.Left, s.Right
	if !top && !bottom && !left && !right {
		top, bottom, left, right = true, true, true, true
	}

	// Use aspect-specific helpers for granular rebuild tracking
	padding := layout.EdgeInsets{}
	if top {
		padding.Top = SafeAreaTopOf(ctx)
	}
	if bottom {
		padding.Bottom = SafeAreaBottomOf(ctx)
	}
	if left {
		padding.Left = SafeAreaLeftOf(ctx)
	}
	if right {
		padding.Right = SafeAreaRightOf(ctx)
	}

	return Padding{
		Padding: padding,
		Child:   s.Child,
	}
}
