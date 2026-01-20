package widgets

import (
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
)

// ListView displays a scrollable list of widgets.
type ListView struct {
	ChildrenWidgets   []core.Widget
	ScrollDirection   *Axis
	Controller        *ScrollController
	Physics           ScrollPhysics
	Padding           layout.EdgeInsets
	MainAxisAlignment MainAxisAlignment
	MainAxisSize      MainAxisSize
}

// ListViewBuilder builds children on demand for the list.
type ListViewBuilder struct {
	ItemCount         int
	ItemBuilder       func(ctx core.BuildContext, index int) core.Widget
	ItemExtent        float64
	CacheExtent       float64
	ScrollDirection   *Axis
	Controller        *ScrollController
	Physics           ScrollPhysics
	Padding           layout.EdgeInsets
	MainAxisAlignment MainAxisAlignment
	MainAxisSize      MainAxisSize
}

func (l ListView) CreateElement() core.Element {
	return core.NewStatelessElement(l, nil)
}

func (l ListView) Key() any {
	return nil
}

func (l ListView) Build(ctx core.BuildContext) core.Widget {
	direction := l.effectiveDirection()
	content := l.buildContent(direction)
	if l.Padding != (layout.EdgeInsets{}) {
		content = Padding{Padding: l.Padding, ChildWidget: content}
	}

	return ScrollView{
		ChildWidget:     content,
		ScrollDirection: direction,
		Controller:      l.Controller,
		Physics:         l.Physics,
	}
}

func (l ListViewBuilder) CreateElement() core.Element {
	return core.NewStatefulElement(l, nil)
}

func (l ListViewBuilder) Key() any {
	return nil
}

func (l ListViewBuilder) CreateState() core.State {
	return &listViewBuilderState{}
}

func (l ListView) effectiveDirection() Axis {
	if l.ScrollDirection != nil {
		return *l.ScrollDirection
	}
	return AxisVertical
}

func (l ListViewBuilder) effectiveDirection() Axis {
	if l.ScrollDirection != nil {
		return *l.ScrollDirection
	}
	return AxisVertical
}

func (l ListView) buildContent(direction Axis) core.Widget {
	if direction == AxisHorizontal {
		return Row{
			ChildrenWidgets:   l.ChildrenWidgets,
			MainAxisAlignment: l.MainAxisAlignment,
			MainAxisSize:      l.MainAxisSize,
		}
	}
	return Column{
		ChildrenWidgets:   l.ChildrenWidgets,
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
		ChildrenWidgets:   children,
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
	if l.effectiveDirection() == AxisHorizontal {
		return SizedBox{Width: l.ItemExtent, ChildWidget: child}
	}
	return SizedBox{Height: l.ItemExtent, ChildWidget: child}
}

func (l ListViewBuilder) buildSpacer(extent float64) core.Widget {
	if extent <= 0 {
		return nil
	}
	if l.effectiveDirection() == AxisHorizontal {
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
	if l.effectiveDirection() == AxisHorizontal {
		return l.Padding.Left
	}
	return l.Padding.Top
}
