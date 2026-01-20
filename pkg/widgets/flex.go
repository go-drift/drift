package widgets

import (
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// Axis represents the layout direction.
type Axis int

const (
	AxisHorizontal Axis = iota
	AxisVertical
)

// MainAxisAlignment controls spacing along the main axis.
type MainAxisAlignment int

const (
	MainAxisAlignmentStart MainAxisAlignment = iota
	MainAxisAlignmentEnd
	MainAxisAlignmentCenter
	MainAxisAlignmentSpaceBetween
	MainAxisAlignmentSpaceAround
	MainAxisAlignmentSpaceEvenly
)

// CrossAxisAlignment controls placement along the cross axis.
type CrossAxisAlignment int

const (
	CrossAxisAlignmentStart CrossAxisAlignment = iota
	CrossAxisAlignmentEnd
	CrossAxisAlignmentCenter
	CrossAxisAlignmentStretch
)

// MainAxisSize controls the size along the main axis.
type MainAxisSize int

const (
	MainAxisSizeMin MainAxisSize = iota
	MainAxisSizeMax
)

// FlexFactor reports the flex value for a render box.
type FlexFactor interface {
	FlexFactor() int
}

// Row lays out children horizontally.
type Row struct {
	ChildrenWidgets    []core.Widget
	MainAxisAlignment  MainAxisAlignment
	CrossAxisAlignment CrossAxisAlignment
	MainAxisSize       MainAxisSize
}

func (r Row) CreateElement() core.Element {
	return core.NewRenderObjectElement(r, nil)
}

func (r Row) Key() any {
	return nil
}

func (r Row) Children() []core.Widget {
	return r.ChildrenWidgets
}

func (r Row) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	flex := &renderFlex{
		direction:      AxisHorizontal,
		alignment:      r.MainAxisAlignment,
		crossAlignment: r.CrossAxisAlignment,
		axisSize:       r.MainAxisSize,
	}
	flex.SetSelf(flex)
	return flex
}

func (r Row) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if flex, ok := renderObject.(*renderFlex); ok {
		flex.direction = AxisHorizontal
		flex.alignment = r.MainAxisAlignment
		flex.crossAlignment = r.CrossAxisAlignment
		flex.axisSize = r.MainAxisSize
		flex.MarkNeedsLayout()
		flex.MarkNeedsPaint()
	}
}

// Column lays out children vertically.
type Column struct {
	ChildrenWidgets    []core.Widget
	MainAxisAlignment  MainAxisAlignment
	CrossAxisAlignment CrossAxisAlignment
	MainAxisSize       MainAxisSize
}

func (c Column) CreateElement() core.Element {
	return core.NewRenderObjectElement(c, nil)
}

func (c Column) Key() any {
	return nil
}

func (c Column) Children() []core.Widget {
	return c.ChildrenWidgets
}

func (c Column) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	flex := &renderFlex{
		direction:      AxisVertical,
		alignment:      c.MainAxisAlignment,
		crossAlignment: c.CrossAxisAlignment,
		axisSize:       c.MainAxisSize,
	}
	flex.SetSelf(flex)
	return flex
}

func (c Column) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if flex, ok := renderObject.(*renderFlex); ok {
		flex.direction = AxisVertical
		flex.alignment = c.MainAxisAlignment
		flex.crossAlignment = c.CrossAxisAlignment
		flex.axisSize = c.MainAxisSize
		flex.MarkNeedsLayout()
		flex.MarkNeedsPaint()
	}
}

type renderFlex struct {
	layout.RenderBoxBase
	children       []layout.RenderBox
	direction      Axis
	alignment      MainAxisAlignment
	crossAlignment CrossAxisAlignment
	axisSize       MainAxisSize
}

func (r *renderFlex) SetChildren(children []layout.RenderObject) {
	r.children = r.children[:0]
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
		}
	}
}

func (r *renderFlex) mainAxis(size rendering.Size) float64 {
	if r.direction == AxisHorizontal {
		return size.Width
	}
	return size.Height
}

func (r *renderFlex) crossAxis(size rendering.Size) float64 {
	if r.direction == AxisHorizontal {
		return size.Height
	}
	return size.Width
}

func (r *renderFlex) makeSize(main, cross float64) rendering.Size {
	if r.direction == AxisHorizontal {
		return rendering.Size{Width: main, Height: cross}
	}
	return rendering.Size{Width: cross, Height: main}
}

func (r *renderFlex) makeOffset(main, cross float64) rendering.Offset {
	if r.direction == AxisHorizontal {
		return rendering.Offset{X: main, Y: cross}
	}
	return rendering.Offset{X: cross, Y: main}
}

func (r *renderFlex) Layout(constraints layout.Constraints) {
	maxSize := rendering.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight}
	maxMain := r.mainAxis(maxSize)

	mainSize := 0.0
	crossSize := 0.0
	totalFlex := 0
	flexChildren := make([]layout.RenderBox, 0)
	flexFactors := make([]int, 0)

	for _, child := range r.children {
		if flex := r.flexFactor(child); flex > 0 {
			flexChildren = append(flexChildren, child)
			flexFactors = append(flexFactors, flex)
			totalFlex += flex
			continue
		}
		child.Layout(r.looseConstraints(maxSize))
		childSize := child.Size()
		mainSize += r.mainAxis(childSize)
		crossSize = math.Max(crossSize, r.crossAxis(childSize))
	}

	remaining := max(maxMain-mainSize, 0)
	if r.axisSize != MainAxisSizeMax {
		remaining = 0
	}

	for i, child := range flexChildren {
		allocated := 0.0
		if totalFlex > 0 {
			allocated = remaining * float64(flexFactors[i]) / float64(totalFlex)
		}
		child.Layout(r.flexConstraints(constraints, allocated))
		childSize := child.Size()
		mainSize += r.mainAxis(childSize)
		crossSize = math.Max(crossSize, r.crossAxis(childSize))
	}

	finalMain := mainSize
	if r.axisSize == MainAxisSizeMax {
		finalMain = maxMain
	}

	size := constraints.Constrain(r.makeSize(finalMain, crossSize))
	r.SetSize(size)

	freeSpace := math.Max(0, r.mainAxis(size)-mainSize)
	spacing, startOffset := r.computeSpacing(freeSpace)

	cursor := startOffset
	for _, child := range r.children {
		crossOffset := r.crossAxisOffset(child.Size())
		child.SetParentData(&layout.BoxParentData{Offset: r.makeOffset(cursor, crossOffset)})
		cursor += r.mainAxis(child.Size()) + spacing
	}
}

func (r *renderFlex) flexFactor(child layout.RenderBox) int {
	if flexChild, ok := child.(FlexFactor); ok {
		return flexChild.FlexFactor()
	}
	return 0
}

func (r *renderFlex) looseConstraints(maxSize rendering.Size) layout.Constraints {
	if r.crossAlignment != CrossAxisAlignmentStretch {
		return layout.Loose(maxSize)
	}
	if r.direction == AxisHorizontal {
		return layout.Constraints{
			MinWidth:  0,
			MaxWidth:  maxSize.Width,
			MinHeight: maxSize.Height,
			MaxHeight: maxSize.Height,
		}
	}
	return layout.Constraints{
		MinWidth:  maxSize.Width,
		MaxWidth:  maxSize.Width,
		MinHeight: 0,
		MaxHeight: maxSize.Height,
	}
}

func (r *renderFlex) crossAxisOffset(childSize rendering.Size) float64 {
	freeSpace := r.crossAxis(r.Size()) - r.crossAxis(childSize)
	if freeSpace <= 0 {
		return 0
	}
	switch r.crossAlignment {
	case CrossAxisAlignmentEnd:
		return freeSpace
	case CrossAxisAlignmentCenter:
		return freeSpace * 0.5
	default:
		return 0
	}
}

func (r *renderFlex) flexConstraints(constraints layout.Constraints, mainSize float64) layout.Constraints {
	if r.direction == AxisHorizontal {
		minHeight := 0.0
		maxHeight := constraints.MaxHeight
		if r.crossAlignment == CrossAxisAlignmentStretch {
			minHeight = maxHeight
		}
		return layout.Constraints{
			MinWidth:  mainSize,
			MaxWidth:  mainSize,
			MinHeight: minHeight,
			MaxHeight: maxHeight,
		}
	}
	minWidth := 0.0
	maxWidth := constraints.MaxWidth
	if r.crossAlignment == CrossAxisAlignmentStretch {
		minWidth = maxWidth
	}
	return layout.Constraints{
		MinWidth:  minWidth,
		MaxWidth:  maxWidth,
		MinHeight: mainSize,
		MaxHeight: mainSize,
	}
}

func (r *renderFlex) computeSpacing(freeSpace float64) (spacing, offset float64) {
	n := len(r.children)
	switch r.alignment {
	case MainAxisAlignmentEnd:
		offset = freeSpace
	case MainAxisAlignmentCenter:
		offset = freeSpace * 0.5
	case MainAxisAlignmentSpaceBetween:
		if n > 1 {
			spacing = freeSpace / float64(n-1)
		}
	case MainAxisAlignmentSpaceAround:
		if n > 0 {
			spacing = freeSpace / float64(n)
			offset = spacing * 0.5
		}
	case MainAxisAlignmentSpaceEvenly:
		if n > 0 {
			spacing = freeSpace / float64(n+1)
			offset = spacing
		}
	}
	return
}

func (r *renderFlex) Paint(ctx *layout.PaintContext) {
	for _, child := range r.children {
		ctx.PaintChild(child, getChildOffset(child))
	}
}

func (r *renderFlex) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	for i := len(r.children) - 1; i >= 0; i-- {
		child := r.children[i]
		offset := getChildOffset(child)
		local := rendering.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if child.HitTest(local, result) {
			return true
		}
	}
	result.Add(r)
	return true
}
