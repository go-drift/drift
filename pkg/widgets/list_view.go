package widgets

import (
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
)

// ListView displays a scrollable list of widgets.
//
// ListView wraps its children in a [ScrollView] with either a [Row] or [Column]
// depending on ScrollDirection. All children are built immediately, making it
// suitable for small lists with a known number of items.
//
// For large lists or dynamic content, use [ListViewBuilder] which builds children
// on demand and supports virtualization for better performance.
//
// Example:
//
//	ListView{
//	    Padding: layout.EdgeInsetsAll(16),
//	    Children: []core.Widget{
//	        ListTile{Title: "Item 1"},
//	        ListTile{Title: "Item 2"},
//	        ListTile{Title: "Item 3"},
//	    },
//	}
type ListView struct {
	core.StatelessBase

	// Children are the widgets to display in the list.
	Children []core.Widget
	// ScrollDirection is the axis along which the list scrolls. Defaults to vertical.
	ScrollDirection Axis
	// Controller manages scroll position and provides scroll notifications.
	Controller *ScrollController
	// Physics determines how the scroll view responds to user input.
	Physics ScrollPhysics
	// Padding is applied around the list content.
	Padding layout.EdgeInsets
	// MainAxisAlignment controls how children are positioned along the scroll axis.
	MainAxisAlignment MainAxisAlignment
	// MainAxisSize determines how much space the list takes along the scroll axis.
	MainAxisSize MainAxisSize
}

// ListViewBuilder builds list items on demand for efficient scrolling of large lists.
//
// Unlike [ListView] which builds all children upfront, ListViewBuilder only builds
// widgets for visible items plus a cache region, making it suitable for lists with
// hundreds or thousands of items.
//
// # Virtualization
//
// For virtualization to work, ItemExtent must be set to a fixed height (or width
// for horizontal lists). This allows the list to calculate which items are visible
// without building all items. If ItemExtent is 0, all items are built immediately.
//
// CacheExtent controls how many pixels beyond the visible area are pre-built,
// reducing flicker during fast scrolling.
//
// Example:
//
//	ListViewBuilder{
//	    ItemCount:   1000,
//	    ItemExtent:  56, // Fixed height per item enables virtualization
//	    CacheExtent: 200, // Pre-build 200px beyond visible area
//	    ItemBuilder: func(ctx core.BuildContext, index int) core.Widget {
//	        return ListTile{Title: fmt.Sprintf("Item %d", index)}
//	    },
//	}
type ListViewBuilder struct {
	core.StatefulBase

	// ItemCount is the total number of items in the list.
	ItemCount int
	// ItemBuilder creates widgets for visible items. Called with the build context and item index.
	ItemBuilder func(ctx core.BuildContext, index int) core.Widget
	// ItemExtent is the fixed extent of each item along the scroll axis. Required for virtualization.
	ItemExtent float64
	// CacheExtent is the number of pixels to render beyond the visible area.
	CacheExtent float64
	// ScrollDirection is the axis along which the list scrolls. Defaults to vertical.
	ScrollDirection Axis
	// Controller manages scroll position and provides scroll notifications.
	Controller *ScrollController
	// Physics determines how the scroll view responds to user input.
	Physics ScrollPhysics
	// Padding is applied around the list content.
	Padding layout.EdgeInsets
	// MainAxisAlignment controls how children are positioned along the scroll axis.
	MainAxisAlignment MainAxisAlignment
	// MainAxisSize determines how much space the list takes along the scroll axis.
	MainAxisSize MainAxisSize
}

func (l ListView) Build(ctx core.BuildContext) core.Widget {
	content := l.buildContent()
	if l.Padding != (layout.EdgeInsets{}) {
		content = Padding{Padding: l.Padding, Child: content}
	}

	return ScrollView{
		Child:           content,
		ScrollDirection: l.ScrollDirection,
		Controller:      l.Controller,
		Physics:         l.Physics,
	}
}

func (l ListViewBuilder) CreateState() core.State {
	return &listViewBuilderState{}
}

func (l ListView) buildContent() core.Widget {
	if l.ScrollDirection == AxisHorizontal {
		return Row{
			Children:          l.Children,
			MainAxisAlignment: l.MainAxisAlignment,
			MainAxisSize:      l.MainAxisSize,
		}
	}
	return Column{
		Children:          l.Children,
		MainAxisAlignment: l.MainAxisAlignment,
		MainAxisSize:      l.MainAxisSize,
	}
}

type listViewBuilderState struct {
	element        *core.StatefulElement
	controller     *ScrollController
	removeListener func()
	visibleStart   int
	visibleEnd     int
}

func (s *listViewBuilderState) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *listViewBuilderState) InitState() {
	widgetValue, ok := s.currentWidget()
	if !ok {
		return
	}
	s.controller = widgetValue.Controller
	if s.controller == nil {
		s.controller = &ScrollController{}
	}
	s.attachListener(widgetValue)
	s.updateVisibleRange(widgetValue)
}

func (s *listViewBuilderState) Build(ctx core.BuildContext) core.Widget {
	widgetValue, ok := s.currentWidget()
	if !ok {
		return nil
	}
	s.attachListener(widgetValue)
	s.updateVisibleRange(widgetValue)
	children := widgetValue.buildChildren(ctx, s.controller, s.visibleStart, s.visibleEnd)
	return ListView{
		Children:          children,
		ScrollDirection:   widgetValue.ScrollDirection,
		Controller:        s.controller,
		Physics:           widgetValue.Physics,
		Padding:           widgetValue.Padding,
		MainAxisAlignment: widgetValue.MainAxisAlignment,
		MainAxisSize:      widgetValue.MainAxisSize,
	}
}

func (s *listViewBuilderState) SetState(fn func()) {
	if fn != nil {
		fn()
	}
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *listViewBuilderState) Dispose() {
	if s.removeListener != nil {
		s.removeListener()
		s.removeListener = nil
	}
}

func (s *listViewBuilderState) DidChangeDependencies() {}

func (s *listViewBuilderState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	oldList, ok := oldWidget.(ListViewBuilder)
	if !ok {
		return
	}
	current, ok := s.currentWidget()
	if !ok {
		return
	}
	if oldList.Controller != current.Controller {
		if s.removeListener != nil {
			s.removeListener()
			s.removeListener = nil
		}
		s.controller = current.Controller
		if s.controller == nil {
			s.controller = &ScrollController{}
		}
		s.attachListener(current)
	}
	s.updateVisibleRange(current)
}

func (s *listViewBuilderState) currentWidget() (ListViewBuilder, bool) {
	if s.element == nil {
		return ListViewBuilder{}, false
	}
	widgetValue, ok := s.element.Widget().(ListViewBuilder)
	return widgetValue, ok
}

func (s *listViewBuilderState) attachListener(_ ListViewBuilder) {
	if s.controller == nil || s.removeListener != nil {
		return
	}
	s.removeListener = s.controller.AddListener(func() {
		s.onScroll()
	})
}

func (s *listViewBuilderState) onScroll() {
	widgetValue, ok := s.currentWidget()
	if !ok {
		return
	}
	if s.updateVisibleRange(widgetValue) {
		if s.element != nil {
			s.element.MarkNeedsBuild()
		}
	}
}

func (s *listViewBuilderState) updateVisibleRange(widgetValue ListViewBuilder) bool {
	start, end := widgetValue.visibleRange(s.controller)
	if start == s.visibleStart && end == s.visibleEnd {
		return false
	}
	s.visibleStart = start
	s.visibleEnd = end
	return true
}

func (l ListViewBuilder) buildChildren(ctx core.BuildContext, controller *ScrollController, start, end int) []core.Widget {
	if l.ItemBuilder == nil || l.ItemCount <= 0 {
		return nil
	}
	if l.ItemExtent <= 0 || controller == nil || controller.ViewportExtent() <= 0 {
		return l.buildAllChildren(ctx)
	}
	children := make([]core.Widget, 0, end-start+2)
	if start > 0 {
		children = append(children, l.buildSpacer(float64(start)*l.ItemExtent))
	}
	for i := start; i < end; i++ {
		child := l.ItemBuilder(ctx, i)
		children = append(children, l.wrapItem(child))
	}
	trailing := l.ItemCount - end
	if trailing > 0 {
		children = append(children, l.buildSpacer(float64(trailing)*l.ItemExtent))
	}
	return children
}

func (l ListViewBuilder) buildAllChildren(ctx core.BuildContext) []core.Widget {
	children := make([]core.Widget, 0, l.ItemCount)
	for i := 0; i < l.ItemCount; i++ {
		child := l.ItemBuilder(ctx, i)
		if l.ItemExtent > 0 {
			children = append(children, l.wrapItem(child))
			continue
		}
		if child != nil {
			children = append(children, child)
		}
	}
	return children
}

func (l ListViewBuilder) wrapItem(child core.Widget) core.Widget {
	if l.ItemExtent <= 0 {
		return child
	}
	if child == nil {
		return l.buildSpacer(l.ItemExtent)
	}
	if l.ScrollDirection == AxisHorizontal {
		return SizedBox{Width: l.ItemExtent, Child: child}
	}
	return SizedBox{Height: l.ItemExtent, Child: child}
}

func (l ListViewBuilder) buildSpacer(extent float64) core.Widget {
	if extent <= 0 {
		return nil
	}
	if l.ScrollDirection == AxisHorizontal {
		return SizedBox{Width: extent}
	}
	return SizedBox{Height: extent}
}

func (l ListViewBuilder) visibleRange(controller *ScrollController) (int, int) {
	if l.ItemCount <= 0 || l.ItemExtent <= 0 || controller == nil {
		return 0, 0
	}
	viewport := controller.ViewportExtent()
	if viewport <= 0 {
		return 0, l.ItemCount
	}
	cache := l.CacheExtent
	if cache < 0 {
		cache = 0
	}
	paddingLeading := l.paddingLeading()
	offset := controller.Offset()
	visibleStart := offset - paddingLeading - cache
	visibleEnd := offset + viewport - paddingLeading + cache
	startIndex := int(math.Floor(visibleStart / l.ItemExtent))
	endIndex := int(math.Ceil(visibleEnd / l.ItemExtent))
	if startIndex < 0 {
		startIndex = 0
	}
	if endIndex > l.ItemCount {
		endIndex = l.ItemCount
	}
	if endIndex < startIndex {
		endIndex = startIndex
	}
	return startIndex, endIndex
}

func (l ListViewBuilder) paddingLeading() float64 {
	if l.ScrollDirection == AxisHorizontal {
		return l.Padding.Left
	}
	return l.Padding.Top
}
