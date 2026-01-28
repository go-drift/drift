package widgets

import (
	"math"
	"reflect"
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
)

// DropdownItem represents a selectable value for a dropdown.
type DropdownItem[T any] struct {
	// Value is the item value.
	Value T
	// Label is the text shown for the item.
	Label string
	// ChildWidget overrides the label when provided.
	ChildWidget core.Widget
	// Disabled disables selection when true.
	Disabled bool
}

// Dropdown displays a button that opens a menu of selectable items.
//
// Dropdown is a generic widget where T is the type of the selection value.
// When an item is selected, OnChanged is called with the selected item's Value.
//
// Example:
//
//	Dropdown[string]{
//	    Value: selectedCountry,
//	    Hint:  "Select a country",
//	    Items: []widgets.DropdownItem[string]{
//	        {Value: "us", Label: "United States"},
//	        {Value: "ca", Label: "Canada"},
//	        {Value: "mx", Label: "Mexico"},
//	    },
//	    OnChanged: func(value string) {
//	        s.SetState(func() { s.selectedCountry = value })
//	    },
//	}
//
// Each [DropdownItem] can have a custom ChildWidget instead of a text Label.
// Items can be individually disabled by setting Disabled: true.
//
// The dropdown uses colors from the current [theme.DropdownTheme] by default.
// Visual properties fall back to theme defaults when their value is zero and
// they have not been explicitly set via a WithX method. Use the WithX methods
// to set a value that should be used even when it equals zero (e.g.,
// [Dropdown.WithBorderRadius](0) for sharp corners).
type Dropdown[T any] struct {
	// Value is the current selected value.
	Value T
	// Items are the available selections.
	Items []DropdownItem[T]
	// OnChanged is called when a new value is selected.
	OnChanged func(T)
	// Hint is shown when no selection matches.
	Hint string
	// Disabled disables the dropdown when true.
	Disabled bool
	// Width sets a fixed width (0 uses layout constraints).
	Width float64
	// Height sets a fixed height (0 uses default).
	Height float64
	// BorderRadius sets the corner radius.
	BorderRadius float64
	// BackgroundColor sets the trigger background.
	BackgroundColor graphics.Color
	// BorderColor sets the trigger border color.
	BorderColor graphics.Color
	// MenuBackgroundColor sets the menu background.
	MenuBackgroundColor graphics.Color
	// MenuBorderColor sets the menu border color.
	MenuBorderColor graphics.Color
	// TextStyle sets the text style for labels.
	TextStyle graphics.TextStyle
	// ItemPadding sets padding for each menu item.
	ItemPadding layout.EdgeInsets
	// overrides tracks which fields were explicitly set via WithX methods.
	overrides dropdownOverrides
}

type dropdownOverrides uint16

const (
	dropdownOverrideBackgroundColor     dropdownOverrides = 1 << iota
	dropdownOverrideBorderColor
	dropdownOverrideMenuBackgroundColor
	dropdownOverrideMenuBorderColor
	dropdownOverrideBorderRadius
	dropdownOverrideHeight
	dropdownOverrideItemPadding
)

// WithBackgroundColor returns a copy with the specified trigger background color.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (d Dropdown[T]) WithBackgroundColor(c graphics.Color) Dropdown[T] {
	d.BackgroundColor = c
	d.overrides |= dropdownOverrideBackgroundColor
	return d
}

// WithBorderColor returns a copy with the specified trigger border color.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (d Dropdown[T]) WithBorderColor(c graphics.Color) Dropdown[T] {
	d.BorderColor = c
	d.overrides |= dropdownOverrideBorderColor
	return d
}

// WithMenuBackgroundColor returns a copy with the specified menu panel background color.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (d Dropdown[T]) WithMenuBackgroundColor(c graphics.Color) Dropdown[T] {
	d.MenuBackgroundColor = c
	d.overrides |= dropdownOverrideMenuBackgroundColor
	return d
}

// WithMenuBorderColor returns a copy with the specified menu panel border color.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (d Dropdown[T]) WithMenuBorderColor(c graphics.Color) Dropdown[T] {
	d.MenuBorderColor = c
	d.overrides |= dropdownOverrideMenuBorderColor
	return d
}

// WithBorderRadius returns a copy with the specified corner radius.
// The value is marked as explicitly set, so even zero (sharp corners)
// will be used instead of falling back to the theme default.
func (d Dropdown[T]) WithBorderRadius(radius float64) Dropdown[T] {
	d.BorderRadius = radius
	d.overrides |= dropdownOverrideBorderRadius
	return d
}

// WithHeight returns a copy with the specified item row height.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (d Dropdown[T]) WithHeight(height float64) Dropdown[T] {
	d.Height = height
	d.overrides |= dropdownOverrideHeight
	return d
}

// WithItemPadding returns a copy with the specified menu item padding.
// The value is marked as explicitly set, so even a zero [layout.EdgeInsets]
// will be used instead of falling back to the theme default.
func (d Dropdown[T]) WithItemPadding(padding layout.EdgeInsets) Dropdown[T] {
	d.ItemPadding = padding
	d.overrides |= dropdownOverrideItemPadding
	return d
}

func (d Dropdown[T]) CreateElement() core.Element {
	return core.NewStatefulElement(d, nil)
}

func (d Dropdown[T]) Key() any {
	return nil
}

func (d Dropdown[T]) CreateState() core.State {
	return &dropdownState[T]{}
}

type dropdownState[T any] struct {
	element  *core.StatefulElement
	expanded bool
}

type dropdownCloser interface {
	closeFromOutside()
	isExpanded() bool
}

var dropdownRegistry = struct {
	items map[dropdownCloser]struct{}
	mu    sync.Mutex
}{
	items: make(map[dropdownCloser]struct{}),
}

func registerDropdown(closer dropdownCloser) {
	dropdownRegistry.mu.Lock()
	dropdownRegistry.items[closer] = struct{}{}
	dropdownRegistry.mu.Unlock()
}

func unregisterDropdown(closer dropdownCloser) {
	dropdownRegistry.mu.Lock()
	delete(dropdownRegistry.items, closer)
	dropdownRegistry.mu.Unlock()
}

// HandleDropdownPointerDown dismisses open dropdowns on outside taps.
func HandleDropdownPointerDown(entries []layout.RenderObject) {
	dropdownRegistry.mu.Lock()
	if len(dropdownRegistry.items) == 0 {
		dropdownRegistry.mu.Unlock()
		return
	}
	hasExpanded := false
	for _, entry := range entries {
		if scope, ok := entry.(*renderDropdownScope); ok {
			if scope.owner != nil && scope.owner.isExpanded() {
				hasExpanded = true
				break
			}
		}
	}
	if hasExpanded {
		dropdownRegistry.mu.Unlock()
		return
	}
	closers := make([]dropdownCloser, 0, len(dropdownRegistry.items))
	for closer := range dropdownRegistry.items {
		closers = append(closers, closer)
	}
	dropdownRegistry.mu.Unlock()
	for _, closer := range closers {
		closer.closeFromOutside()
	}
}

func (s *dropdownState[T]) SetElement(element *core.StatefulElement) {
	s.element = element
}

func (s *dropdownState[T]) InitState() {}

func (s *dropdownState[T]) setExpanded(expanded bool) {
	if s.expanded == expanded {
		return
	}
	s.expanded = expanded
	if expanded {
		registerDropdown(s)
		return
	}
	unregisterDropdown(s)
}

func (s *dropdownState[T]) closeFromOutside() {
	if !s.expanded {
		return
	}
	s.SetState(func() {
		s.setExpanded(false)
	})
	s.requestParentLayout()
}

func (s *dropdownState[T]) isExpanded() bool {
	return s.expanded
}

func (s *dropdownState[T]) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(Dropdown[T])
	themeData, colors, textTheme := theme.UseTheme(ctx)
	dropdownTheme := themeData.DropdownThemeOf()

	textStyle := w.TextStyle
	if textStyle.FontSize == 0 {
		textStyle = textTheme.BodyMedium
	}
	if textStyle.Color == 0 {
		textStyle.Color = dropdownTheme.TextColor
	}

	backgroundColor := w.BackgroundColor
	if w.overrides&dropdownOverrideBackgroundColor == 0 && backgroundColor == 0 {
		backgroundColor = dropdownTheme.BackgroundColor
	}
	borderColor := w.BorderColor
	if w.overrides&dropdownOverrideBorderColor == 0 && borderColor == 0 {
		borderColor = dropdownTheme.BorderColor
	}
	menuBackgroundColor := w.MenuBackgroundColor
	if w.overrides&dropdownOverrideMenuBackgroundColor == 0 && menuBackgroundColor == 0 {
		menuBackgroundColor = dropdownTheme.MenuBackgroundColor
	}
	menuBorderColor := w.MenuBorderColor
	if w.overrides&dropdownOverrideMenuBorderColor == 0 && menuBorderColor == 0 {
		menuBorderColor = dropdownTheme.MenuBorderColor
	}
	borderRadius := w.BorderRadius
	if w.overrides&dropdownOverrideBorderRadius == 0 && borderRadius == 0 {
		borderRadius = dropdownTheme.BorderRadius
	}
	itemPadding := w.ItemPadding
	if w.overrides&dropdownOverrideItemPadding == 0 && itemPadding == (layout.EdgeInsets{}) {
		itemPadding = dropdownTheme.ItemPadding
	}
	itemHeight := w.Height
	if w.overrides&dropdownOverrideHeight == 0 && itemHeight == 0 {
		itemHeight = dropdownTheme.Height
	}

	enabled := !w.Disabled && w.OnChanged != nil
	if !enabled {
		textStyle.Color = dropdownTheme.DisabledTextColor
		backgroundColor = colors.SurfaceVariant
		borderColor = dropdownTheme.BorderColor
	}

	selectedLabel := ""
	var selectedChild core.Widget
	for _, item := range w.Items {
		if reflect.DeepEqual(item.Value, w.Value) {
			selectedLabel = item.Label
			selectedChild = item.ChildWidget
			break
		}
	}

	displayChild := selectedChild
	if displayChild == nil {
		displayLabel := selectedLabel
		if displayLabel == "" {
			displayLabel = w.Hint
		}
		displayChild = Text{Content: displayLabel, Style: textStyle}
	}

	width := w.Width
	if width == 0 {
		width = math.MaxFloat64
	}
	iconSize := textStyle.FontSize
	if iconSize == 0 {
		iconSize = textTheme.BodyMedium.FontSize
	}
	contentPadding := layout.EdgeInsetsOnly(itemPadding.Left, 0, itemPadding.Right, 0)

	toggle := func() {
		if !enabled {
			return
		}
		s.SetState(func() {
			s.setExpanded(!s.expanded)
		})
		s.requestParentLayout()
	}

	triggerContent := RowOf(
		MainAxisAlignmentSpaceBetween,
		CrossAxisAlignmentCenter,
		MainAxisSizeMax,
		Padding{Padding: layout.EdgeInsetsOnly(0, 0, 8, 0), ChildWidget: displayChild},
		SizedBox{Width: iconSize + 8, ChildWidget: dropdownChevron{size: iconSize * 0.6, color: textStyle.Color}},
	)

	trigger := GestureDetector{
		OnTap: toggle,
		ChildWidget: Container{
			Width:       width,
			Height:      itemHeight,
			Padding:     contentPadding,
			ChildWidget: triggerContent,
		},
	}

	triggerBox := DecoratedBox{
		Color:        backgroundColor,
		BorderColor:  borderColor,
		BorderWidth:  1,
		BorderRadius: borderRadius,
		ChildWidget:  trigger,
	}

	if !s.expanded {
		return dropdownScope{owner: s, childWidget: triggerBox}
	}

	menuItems := make([]core.Widget, 0, len(w.Items))
	for _, item := range w.Items {
		itemEnabled := enabled && !item.Disabled
		itemLabel := item.Label
		itemChild := item.ChildWidget
		if itemChild == nil {
			itemChild = Text{Content: itemLabel, Style: textStyle}
		}
		itemBackground := graphics.ColorTransparent
		if reflect.DeepEqual(item.Value, w.Value) {
			itemBackground = dropdownTheme.SelectedItemColor
		}
		menuItems = append(menuItems, GestureDetector{
			OnTap: func(value T, enabled bool) func() {
				return func() {
					if !enabled {
						return
					}
					s.SetState(func() {
						s.setExpanded(false)
					})
					s.requestParentLayout()
					if w.OnChanged != nil {
						w.OnChanged(value)
					}
				}
			}(item.Value, itemEnabled),
			ChildWidget: Container{
				Color: itemBackground,
				ChildWidget: SizedBox{
					Width:  width,
					Height: itemHeight,
					ChildWidget: RowOf(
						MainAxisAlignmentStart,
						CrossAxisAlignmentCenter,
						MainAxisSizeMax,
						Padding{Padding: contentPadding, ChildWidget: itemChild},
					),
				},
			},
		})
	}

	menu := DecoratedBox{
		Color:        menuBackgroundColor,
		BorderColor:  menuBorderColor,
		BorderWidth:  1,
		BorderRadius: borderRadius,
		ChildWidget:  ColumnOf(MainAxisAlignmentStart, CrossAxisAlignmentStretch, MainAxisSizeMin, menuItems...),
	}

	content := ColumnOf(
		MainAxisAlignmentStart,
		CrossAxisAlignmentStretch,
		MainAxisSizeMin,
		triggerBox,
		VSpace(6),
		SizedBox{Width: width, ChildWidget: menu},
	)

	return dropdownScope{owner: s, childWidget: content}
}

type dropdownScope struct {
	childWidget core.Widget
	owner       dropdownCloser
}

func (d dropdownScope) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d dropdownScope) Key() any {
	return nil
}

func (d dropdownScope) Child() core.Widget {
	return d.childWidget
}

func (d dropdownScope) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	scope := &renderDropdownScope{owner: d.owner}
	scope.SetSelf(scope)
	return scope
}

func (d dropdownScope) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if scope, ok := renderObject.(*renderDropdownScope); ok {
		scope.owner = d.owner
		scope.MarkNeedsLayout()
		scope.MarkNeedsPaint()
	}
}

type renderDropdownScope struct {
	layout.RenderBoxBase
	child layout.RenderBox
	owner dropdownCloser
}

type dropdownChevron struct {
	size  float64
	color graphics.Color
}

func (d dropdownChevron) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d dropdownChevron) Key() any {
	return nil
}

func (d dropdownChevron) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	chevron := &renderDropdownChevron{size: d.size, color: d.color}
	chevron.SetSelf(chevron)
	return chevron
}

func (d dropdownChevron) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if chevron, ok := renderObject.(*renderDropdownChevron); ok {
		chevron.size = d.size
		chevron.color = d.color
		chevron.MarkNeedsLayout()
		chevron.MarkNeedsPaint()
	}
}

type renderDropdownChevron struct {
	layout.RenderBoxBase
	size  float64
	color graphics.Color
}

func (r *renderDropdownChevron) PerformLayout() {
	constraints := r.Constraints()
	size := r.size
	if size == 0 {
		size = 10
	}
	finalSize := constraints.Constrain(graphics.Size{Width: size, Height: size})
	r.SetSize(finalSize)
}

func (r *renderDropdownChevron) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	path := graphics.NewPath()
	path.MoveTo(0, size.Height*0.3)
	path.LineTo(size.Width/2, size.Height*0.75)
	path.LineTo(size.Width, size.Height*0.3)
	paint := graphics.DefaultPaint()
	paint.Color = r.color
	paint.Style = graphics.PaintStyleStroke
	paint.StrokeWidth = max(size.Width*0.12, 1.5)
	ctx.Canvas.DrawPath(path, paint)
}

func (r *renderDropdownChevron) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderDropdownScope) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderDropdownScope) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	r.child.Layout(constraints, true) // true: we read child.Size()
	r.child.SetParentData(&layout.BoxParentData{})
	r.SetSize(constraints.Constrain(r.child.Size()))
}

func (r *renderDropdownScope) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, graphics.Offset{})
	}
}

func (r *renderDropdownScope) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil {
		r.child.HitTest(position, result)
	}
	result.Add(r)
	return true
}

func (s *dropdownState[T]) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *dropdownState[T]) Dispose() {
	unregisterDropdown(s)
}

func (s *dropdownState[T]) DidChangeDependencies() {}

func (s *dropdownState[T]) DidUpdateWidget(oldWidget core.StatefulWidget) {}

func (s *dropdownState[T]) requestParentLayout() {
	if s.element == nil {
		return
	}
	ancestor := s.element.FindAncestor(func(element core.Element) bool {
		_, ok := element.(interface{ RenderObject() layout.RenderObject })
		return ok
	})
	if ancestor == nil {
		return
	}
	if renderElement, ok := ancestor.(interface{ RenderObject() layout.RenderObject }); ok {
		renderObject := renderElement.RenderObject()
		if renderObject != nil {
			renderObject.MarkNeedsLayout()
			renderObject.MarkNeedsPaint()
		}
	}
}
