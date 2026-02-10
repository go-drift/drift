package widgets

import (
	"fmt"
	"math"
	"reflect"
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// DropdownItem represents a selectable value for a dropdown.
type DropdownItem[T any] struct {
	// Value is the item value.
	Value T
	// Label is the text shown for the item.
	Label string
	// Child overrides the label when provided.
	Child core.Widget
	// Disabled disables selection when true.
	Disabled bool
}

// Dropdown displays a button that opens a menu of selectable items.
//
// # Styling Model
//
// Dropdown is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - BackgroundColor: 0 means transparent background
//   - BorderRadius: 0 means sharp corners
//   - Height: 0 means zero height (not rendered)
//
// For theme-styled dropdowns, use [theme.DropdownOf] which pre-fills visual
// properties from the current theme's [theme.DropdownThemeData].
//
// # Creation Patterns
//
// Explicit with struct literal (full control):
//
//	widgets.Dropdown[string]{
//	    Value:           selectedCountry,
//	    Items:           countryItems,
//	    OnChanged:       func(v string) { s.SetState(func() { s.selectedCountry = v }) },
//	    BackgroundColor: graphics.ColorWhite,
//	    BorderColor:     graphics.RGB(200, 200, 200),
//	    BorderRadius:    8,
//	    Height:          48,
//	}
//
// Themed (reads from current theme):
//
//	theme.DropdownOf(ctx, selectedCountry, countryItems, onChanged)
//	// Pre-filled with theme colors, border radius, height, item padding
//
// Dropdown is a generic widget where T is the type of the selection value.
// When an item is selected, OnChanged is called with the selected item's Value.
//
// Each [DropdownItem] can have a custom Child instead of a text Label.
// Items can be individually disabled by setting Disabled: true.
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
	// Height sets a fixed height. Zero means zero height (not rendered).
	Height float64
	// BorderRadius sets the corner radius. Zero means sharp corners.
	BorderRadius float64
	// BackgroundColor sets the trigger background. Zero means transparent.
	BackgroundColor graphics.Color
	// BorderColor sets the trigger border color. Zero means no border.
	BorderColor graphics.Color
	// MenuBackgroundColor sets the menu background. Zero means transparent.
	MenuBackgroundColor graphics.Color
	// MenuBorderColor sets the menu border color. Zero means no border.
	MenuBorderColor graphics.Color
	// TextStyle sets the text style for labels.
	TextStyle graphics.TextStyle
	// ItemPadding sets padding for each menu item. Zero means no padding.
	ItemPadding layout.EdgeInsets
	// SelectedItemColor is the background for the currently selected item.
	SelectedItemColor graphics.Color

	// DisabledTextColor is the text color when disabled.
	// If zero, falls back to 0.5 opacity on the normal styling.
	DisabledTextColor graphics.Color
}

// WithBackgroundColor returns a copy with the specified trigger background color.
func (d Dropdown[T]) WithBackgroundColor(c graphics.Color) Dropdown[T] {
	d.BackgroundColor = c
	return d
}

// WithBorderColor returns a copy with the specified trigger border color.
func (d Dropdown[T]) WithBorderColor(c graphics.Color) Dropdown[T] {
	d.BorderColor = c
	return d
}

// WithMenuBackgroundColor returns a copy with the specified menu panel background color.
func (d Dropdown[T]) WithMenuBackgroundColor(c graphics.Color) Dropdown[T] {
	d.MenuBackgroundColor = c
	return d
}

// WithMenuBorderColor returns a copy with the specified menu panel border color.
func (d Dropdown[T]) WithMenuBorderColor(c graphics.Color) Dropdown[T] {
	d.MenuBorderColor = c
	return d
}

// WithBorderRadius returns a copy with the specified corner radius.
func (d Dropdown[T]) WithBorderRadius(radius float64) Dropdown[T] {
	d.BorderRadius = radius
	return d
}

// WithHeight returns a copy with the specified item row height.
func (d Dropdown[T]) WithHeight(height float64) Dropdown[T] {
	d.Height = height
	return d
}

// WithItemPadding returns a copy with the specified menu item padding.
func (d Dropdown[T]) WithItemPadding(padding layout.EdgeInsets) Dropdown[T] {
	d.ItemPadding = padding
	return d
}

// WithHint returns a copy with the specified hint text shown when no selection matches.
func (d Dropdown[T]) WithHint(hint string) Dropdown[T] {
	d.Hint = hint
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

	// Use field values directly — zero means zero
	textStyle := w.TextStyle
	backgroundColor := w.BackgroundColor
	borderColor := w.BorderColor
	menuBackgroundColor := w.MenuBackgroundColor
	menuBorderColor := w.MenuBorderColor
	borderRadius := w.BorderRadius
	itemPadding := w.ItemPadding
	itemHeight := w.Height
	selectedItemColor := w.SelectedItemColor

	enabled := !w.Disabled && w.OnChanged != nil

	// Apply disabled styling when not enabled (either Disabled=true or OnChanged=nil).
	// This ensures widgets with nil handlers also appear disabled.
	useOpacityFallback := false
	if !enabled {
		if w.DisabledTextColor != 0 {
			textStyle.Color = w.DisabledTextColor
		} else {
			useOpacityFallback = true
		}
	}

	selectedLabel := ""
	var selectedChild core.Widget
	for _, item := range w.Items {
		if reflect.DeepEqual(item.Value, w.Value) {
			selectedLabel = item.Label
			selectedChild = item.Child
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
		Padding{Padding: layout.EdgeInsetsOnly(0, 0, 8, 0), Child: displayChild},
		SizedBox{Width: iconSize + 8, Child: dropdownChevron{size: iconSize * 0.6, color: textStyle.Color}},
	)

	trigger := GestureDetector{
		OnTap: toggle,
		Child: Container{
			Width:   width,
			Height:  itemHeight,
			Padding: contentPadding,
			Child:   triggerContent,
		},
	}

	var triggerBox core.Widget = DecoratedBox{
		Color:        backgroundColor,
		BorderColor:  borderColor,
		BorderWidth:  1,
		BorderRadius: borderRadius,
		Child:        trigger,
	}

	// Fall back to opacity if no disabled text color provided
	if useOpacityFallback {
		triggerBox = Opacity{Opacity: 0.5, Child: triggerBox}
	}

	// Wrap trigger with semantics
	triggerFlags := semantics.SemanticsHasEnabledState | semantics.SemanticsHasExpandedState
	if enabled {
		triggerFlags = triggerFlags.Set(semantics.SemanticsIsEnabled)
	}
	if s.expanded {
		triggerFlags = triggerFlags.Set(semantics.SemanticsIsExpanded)
	}
	triggerValue := selectedLabel
	if triggerValue == "" {
		triggerValue = w.Hint
	}
	triggerHint := "Double tap to open"
	if s.expanded {
		triggerHint = "Double tap to close"
	}
	triggerBox = Semantics{
		Role:             semantics.SemanticsRolePopup,
		Flags:            triggerFlags,
		Value:            triggerValue,
		Hint:             triggerHint,
		Container:        true,
		MergeDescendants: true,
		OnTap:            toggle,
		Child:            triggerBox,
	}

	if !s.expanded {
		return dropdownScope{owner: s, child: triggerBox}
	}

	menuItems := make([]core.Widget, 0, len(w.Items))
	itemCount := len(w.Items)
	for i, item := range w.Items {
		itemEnabled := enabled && !item.Disabled
		itemLabel := item.Label
		itemChild := item.Child
		if itemChild == nil {
			itemChild = Text{Content: itemLabel, Style: textStyle}
		}
		itemBackground := graphics.ColorTransparent
		isSelected := reflect.DeepEqual(item.Value, w.Value)
		if isSelected {
			itemBackground = selectedItemColor
		}
		onItemTap := func(value T, enabled bool) func() {
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
		}(item.Value, itemEnabled)

		itemFlags := semantics.SemanticsHasSelectedState | semantics.SemanticsHasEnabledState
		if isSelected {
			itemFlags = itemFlags.Set(semantics.SemanticsIsSelected)
		}
		if itemEnabled {
			itemFlags = itemFlags.Set(semantics.SemanticsIsEnabled)
		}

		menuItems = append(menuItems, Semantics{
			Role:             semantics.SemanticsRoleMenuItem,
			Flags:            itemFlags,
			Hint:             fmt.Sprintf("Item %d of %d", i+1, itemCount),
			Container:        true,
			MergeDescendants: true,
			OnTap:            onItemTap,
			Child: GestureDetector{
				OnTap: onItemTap,
				Child: Container{
					Color: itemBackground,
					Child: SizedBox{
						Width:  width,
						Height: itemHeight,
						Child: RowOf(
							MainAxisAlignmentStart,
							CrossAxisAlignmentCenter,
							MainAxisSizeMax,
							Padding{Padding: contentPadding, Child: itemChild},
						),
					},
				},
			},
		})
	}

	menu := DecoratedBox{
		Color:        menuBackgroundColor,
		BorderColor:  menuBorderColor,
		BorderWidth:  1,
		BorderRadius: borderRadius,
		Child:        ColumnOf(MainAxisAlignmentStart, CrossAxisAlignmentStretch, MainAxisSizeMin, menuItems...),
	}

	content := ColumnOf(
		MainAxisAlignmentStart,
		CrossAxisAlignmentStretch,
		MainAxisSizeMin,
		triggerBox,
		VSpace(6),
		SizedBox{Width: width, Child: menu},
	)

	return dropdownScope{owner: s, child: content}
}

type dropdownScope struct {
	child core.Widget
	owner dropdownCloser
}

func (d dropdownScope) CreateElement() core.Element {
	return core.NewRenderObjectElement(d, nil)
}

func (d dropdownScope) Key() any {
	return nil
}

func (d dropdownScope) ChildWidget() core.Widget {
	return d.child
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

func (r *renderDropdownScope) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
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
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
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
