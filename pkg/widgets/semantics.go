//go:build android || darwin || ios

package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// Semantics is a widget that annotates the widget tree with semantics information
// for accessibility services.
type Semantics struct {
	// ChildWidget is the child widget to annotate.
	ChildWidget core.Widget

	// Label is the primary accessibility label for this node.
	Label string

	// Value is the current value (e.g., slider position, text content).
	Value string

	// Hint provides guidance on the action that will occur.
	Hint string

	// Tooltip provides additional information shown on hover/long press.
	Tooltip string

	// Role defines the semantic role of the node.
	Role semantics.SemanticsRole

	// Flags contains boolean state flags.
	Flags semantics.SemanticsFlag

	// Container indicates this node creates a semantic boundary.
	Container bool

	// MergeDescendants merges labels from descendant nodes into this node.
	// Use this when a widget contains multiple text elements that should be
	// announced as a single unit (e.g., a card with title and subtitle).
	MergeDescendants bool

	// ExplicitChildNodes indicates whether child nodes should be explicit.
	ExplicitChildNodes bool

	// CurrentValue for slider-type controls.
	CurrentValue *float64

	// MinValue for slider-type controls.
	MinValue *float64

	// MaxValue for slider-type controls.
	MaxValue *float64

	// HeadingLevel indicates heading level (1-6, 0 for none).
	HeadingLevel int

	// OnTap is the handler for tap/click actions.
	OnTap func()

	// OnLongPress is the handler for long press actions.
	OnLongPress func()

	// OnScrollLeft is the handler for scroll left actions.
	OnScrollLeft func()

	// OnScrollRight is the handler for scroll right actions.
	OnScrollRight func()

	// OnScrollUp is the handler for scroll up actions.
	OnScrollUp func()

	// OnScrollDown is the handler for scroll down actions.
	OnScrollDown func()

	// OnIncrease is the handler for increase actions.
	OnIncrease func()

	// OnDecrease is the handler for decrease actions.
	OnDecrease func()

	// OnDismiss is the handler for dismiss actions.
	OnDismiss func()

	// CustomActions is a list of custom accessibility actions.
	CustomActions []semantics.CustomSemanticsAction

	// CustomActionHandlers maps custom action IDs to handlers.
	CustomActionHandlers map[int64]func()
}

func (s Semantics) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s Semantics) Key() any {
	return nil
}

func (s Semantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderSemantics{}
	r.SetSelf(r)
	r.update(s)
	return r
}

func (s Semantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderSemantics); ok {
		r.update(s)
		r.MarkNeedsSemanticsUpdate()
	}
}

// Child returns the child widget.
func (s Semantics) Child() core.Widget {
	return s.ChildWidget
}

type renderSemantics struct {
	layout.RenderBoxBase
	child              layout.RenderObject
	label              string
	value              string
	hint               string
	tooltip            string
	role               semantics.SemanticsRole
	flags              semantics.SemanticsFlag
	container          bool
	mergeDescendants   bool
	explicitChildNodes bool
	currentValue       *float64
	minValue           *float64
	maxValue           *float64
	headingLevel       int
	customActions      []semantics.CustomSemanticsAction
	actions            *semantics.SemanticsActions
}

func (r *renderSemantics) update(s Semantics) {
	r.label = s.Label
	r.value = s.Value
	r.hint = s.Hint
	r.tooltip = s.Tooltip
	r.role = s.Role
	r.flags = s.Flags
	r.container = s.Container
	r.mergeDescendants = s.MergeDescendants
	r.explicitChildNodes = s.ExplicitChildNodes
	r.currentValue = s.CurrentValue
	r.minValue = s.MinValue
	r.maxValue = s.MaxValue
	r.headingLevel = s.HeadingLevel
	r.customActions = s.CustomActions

	// Build actions
	r.actions = semantics.NewSemanticsActions()
	if s.OnTap != nil {
		r.actions.SetHandler(semantics.SemanticsActionTap, func(args any) { s.OnTap() })
	}
	if s.OnLongPress != nil {
		r.actions.SetHandler(semantics.SemanticsActionLongPress, func(args any) { s.OnLongPress() })
	}
	if s.OnScrollLeft != nil {
		r.actions.SetHandler(semantics.SemanticsActionScrollLeft, func(args any) { s.OnScrollLeft() })
	}
	if s.OnScrollRight != nil {
		r.actions.SetHandler(semantics.SemanticsActionScrollRight, func(args any) { s.OnScrollRight() })
	}
	if s.OnScrollUp != nil {
		r.actions.SetHandler(semantics.SemanticsActionScrollUp, func(args any) { s.OnScrollUp() })
	}
	if s.OnScrollDown != nil {
		r.actions.SetHandler(semantics.SemanticsActionScrollDown, func(args any) { s.OnScrollDown() })
	}
	if s.OnIncrease != nil {
		r.actions.SetHandler(semantics.SemanticsActionIncrease, func(args any) { s.OnIncrease() })
	}
	if s.OnDecrease != nil {
		r.actions.SetHandler(semantics.SemanticsActionDecrease, func(args any) { s.OnDecrease() })
	}
	if s.OnDismiss != nil {
		r.actions.SetHandler(semantics.SemanticsActionDismiss, func(args any) { s.OnDismiss() })
	}
	if len(s.CustomActionHandlers) > 0 {
		r.actions.SetHandler(semantics.SemanticsActionCustomAction, func(args any) {
			if argMap, ok := args.(map[string]any); ok {
				if actionID, ok := argMap["actionId"].(int64); ok {
					if handler, exists := s.CustomActionHandlers[actionID]; exists {
						handler()
					}
				}
			}
		})
	}
}

func (r *renderSemantics) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderSemantics) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderSemantics) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderSemantics) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child.(layout.RenderBox), rendering.Offset{})
	}
}

func (r *renderSemantics) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}

// DescribeSemanticsConfiguration implements SemanticsDescriber.
func (r *renderSemantics) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = r.container
	config.IsMergingSemanticsOfDescendants = r.mergeDescendants
	config.ExplicitChildNodes = r.explicitChildNodes
	config.Properties = semantics.SemanticsProperties{
		Label:         r.label,
		Value:         r.value,
		Hint:          r.hint,
		Tooltip:       r.tooltip,
		Role:          r.role,
		Flags:         r.flags,
		CurrentValue:  r.currentValue,
		MinValue:      r.minValue,
		MaxValue:      r.maxValue,
		HeadingLevel:  r.headingLevel,
		CustomActions: r.customActions,
	}
	config.Actions = r.actions

	return r.label != "" || r.value != "" || r.hint != "" ||
		r.role != semantics.SemanticsRoleNone || r.flags != 0 ||
		!r.actions.IsEmpty()
}

// ExcludeSemantics is a widget that excludes its child from the semantics tree.
// Use this to hide decorative elements from accessibility services.
type ExcludeSemantics struct {
	// ChildWidget is the child widget to exclude.
	ChildWidget core.Widget

	// Excluding controls whether to exclude the child from semantics.
	// Set to true to exclude, false to include. Defaults to false (Go zero value).
	// For the common case of excluding, use: ExcludeSemantics{Excluding: true, ChildWidget: child}
	Excluding bool
}

// NewExcludeSemantics creates an ExcludeSemantics widget that excludes the child from accessibility.
func NewExcludeSemantics(child core.Widget) ExcludeSemantics {
	return ExcludeSemantics{Excluding: true, ChildWidget: child}
}

func (e ExcludeSemantics) CreateElement() core.Element {
	return core.NewRenderObjectElement(e, nil)
}

func (e ExcludeSemantics) Key() any {
	return nil
}

func (e ExcludeSemantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderExcludeSemantics{excluding: e.Excluding}
	r.SetSelf(r)
	return r
}

func (e ExcludeSemantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderExcludeSemantics); ok {
		r.excluding = e.Excluding
		r.MarkNeedsSemanticsUpdate()
	}
}

// Child returns the child widget.
func (e ExcludeSemantics) Child() core.Widget {
	return e.ChildWidget
}

type renderExcludeSemantics struct {
	layout.RenderBoxBase
	child     layout.RenderObject
	excluding bool
}

func (r *renderExcludeSemantics) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderExcludeSemantics) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderExcludeSemantics) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderExcludeSemantics) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child.(layout.RenderBox), rendering.Offset{})
	}
}

func (r *renderExcludeSemantics) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}

// DescribeSemanticsConfiguration implements SemanticsDescriber.
func (r *renderExcludeSemantics) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	if r.excluding {
		config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsIsHidden)
	}
	return false
}

// MergeSemantics is a widget that merges the semantics of its descendants.
type MergeSemantics struct {
	// ChildWidget is the child widget whose semantics will be merged.
	ChildWidget core.Widget
}

func (m MergeSemantics) CreateElement() core.Element {
	return core.NewRenderObjectElement(m, nil)
}

func (m MergeSemantics) Key() any {
	return nil
}

func (m MergeSemantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderMergeSemantics{}
	r.SetSelf(r)
	return r
}

func (m MergeSemantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderMergeSemantics); ok {
		r.MarkNeedsSemanticsUpdate()
	}
}

// Child returns the child widget.
func (m MergeSemantics) Child() core.Widget {
	return m.ChildWidget
}

type renderMergeSemantics struct {
	layout.RenderBoxBase
	child layout.RenderObject
}

func (r *renderMergeSemantics) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderMergeSemantics) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderMergeSemantics) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderMergeSemantics) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child.(layout.RenderBox), rendering.Offset{})
	}
}

func (r *renderMergeSemantics) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}

// DescribeSemanticsConfiguration implements SemanticsDescriber.
func (r *renderMergeSemantics) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsMergingSemanticsOfDescendants = true
	return true
}

// SemanticsGestureHandler wraps gestures with accessibility semantics.
// This is used internally by widgets to connect gestures with semantics actions.
type SemanticsGestureHandler struct {
	TapHandler  *gestures.TapGestureRecognizer
	OnTap       func()
	OnLongPress func()
}

// BuildSemanticsActions creates semantics actions from the gesture handlers.
func (h *SemanticsGestureHandler) BuildSemanticsActions() *semantics.SemanticsActions {
	actions := semantics.NewSemanticsActions()
	if h.OnTap != nil {
		actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
			h.OnTap()
		})
	}
	if h.OnLongPress != nil {
		actions.SetHandler(semantics.SemanticsActionLongPress, func(args any) {
			h.OnLongPress()
		})
	}
	return actions
}
