package widgets

import (
	"fmt"
	"math"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// WrapAlignment controls how children are positioned along the main axis within each run.
type WrapAlignment int

const (
	// WrapAlignmentStart places children at the start of each run.
	WrapAlignmentStart WrapAlignment = iota
	// WrapAlignmentEnd places children at the end of each run.
	WrapAlignmentEnd
	// WrapAlignmentCenter centers children within each run.
	WrapAlignmentCenter
	// WrapAlignmentSpaceBetween distributes free space evenly between children.
	// No space before the first or after the last child in each run.
	WrapAlignmentSpaceBetween
	// WrapAlignmentSpaceAround distributes free space evenly, with half-sized
	// spaces at the start and end of each run.
	WrapAlignmentSpaceAround
	// WrapAlignmentSpaceEvenly distributes free space evenly, including
	// equal space before the first and after the last child in each run.
	WrapAlignmentSpaceEvenly
)

// String returns a human-readable representation of the wrap alignment.
func (a WrapAlignment) String() string {
	switch a {
	case WrapAlignmentStart:
		return "start"
	case WrapAlignmentEnd:
		return "end"
	case WrapAlignmentCenter:
		return "center"
	case WrapAlignmentSpaceBetween:
		return "space_between"
	case WrapAlignmentSpaceAround:
		return "space_around"
	case WrapAlignmentSpaceEvenly:
		return "space_evenly"
	default:
		return fmt.Sprintf("WrapAlignment(%d)", int(a))
	}
}

// WrapCrossAlignment controls how children are positioned along the cross axis within each run.
type WrapCrossAlignment int

const (
	// WrapCrossAlignmentStart places children at the start of the cross axis within each run.
	WrapCrossAlignmentStart WrapCrossAlignment = iota
	// WrapCrossAlignmentEnd places children at the end of the cross axis within each run.
	WrapCrossAlignmentEnd
	// WrapCrossAlignmentCenter centers children along the cross axis within each run.
	WrapCrossAlignmentCenter
)

// String returns a human-readable representation of the wrap cross alignment.
func (a WrapCrossAlignment) String() string {
	switch a {
	case WrapCrossAlignmentStart:
		return "start"
	case WrapCrossAlignmentEnd:
		return "end"
	case WrapCrossAlignmentCenter:
		return "center"
	default:
		return fmt.Sprintf("WrapCrossAlignment(%d)", int(a))
	}
}

// WrapAxis controls the layout direction for Wrap.
//
// WrapAxisHorizontal is the zero value to make horizontal wrapping the default.
// Use WrapAxisVertical for top-to-bottom flow that wraps into new columns.
type WrapAxis int

const (
	WrapAxisHorizontal WrapAxis = iota
	WrapAxisVertical
)

// String returns a human-readable representation of the wrap axis.
func (a WrapAxis) String() string {
	switch a {
	case WrapAxisHorizontal:
		return "horizontal"
	case WrapAxisVertical:
		return "vertical"
	default:
		return fmt.Sprintf("WrapAxis(%d)", int(a))
	}
}

// RunAlignment controls how runs are distributed along the cross axis.
type RunAlignment int

const (
	// RunAlignmentStart places runs at the start of the cross axis.
	RunAlignmentStart RunAlignment = iota
	// RunAlignmentEnd places runs at the end of the cross axis.
	RunAlignmentEnd
	// RunAlignmentCenter centers runs along the cross axis.
	RunAlignmentCenter
	// RunAlignmentSpaceBetween distributes free space evenly between runs.
	// No space before the first or after the last run.
	RunAlignmentSpaceBetween
	// RunAlignmentSpaceAround distributes free space evenly, with half-sized
	// spaces at the start and end.
	RunAlignmentSpaceAround
	// RunAlignmentSpaceEvenly distributes free space evenly, including
	// equal space before the first and after the last run.
	RunAlignmentSpaceEvenly
)

// String returns a human-readable representation of the run alignment.
func (a RunAlignment) String() string {
	switch a {
	case RunAlignmentStart:
		return "start"
	case RunAlignmentEnd:
		return "end"
	case RunAlignmentCenter:
		return "center"
	case RunAlignmentSpaceBetween:
		return "space_between"
	case RunAlignmentSpaceAround:
		return "space_around"
	case RunAlignmentSpaceEvenly:
		return "space_evenly"
	default:
		return fmt.Sprintf("RunAlignment(%d)", int(a))
	}
}

// Wrap lays out children in runs, wrapping to the next line when space runs out.
//
// Wrap is similar to CSS flexbox with flex-wrap: wrap. Children are laid out
// along the main axis until they exceed the available space, at which point
// a new run is started.
//
// # Sizing Behavior
//
// Wrap requires bounded constraints on the main axis (width for horizontal,
// height for vertical). The cross axis can be unbounded - Wrap will size to
// fit all runs. If the main axis is unbounded, Wrap will panic with guidance.
//
// # Spacing
//
// Use Spacing to add gaps between children within each run. Use RunSpacing to
// add gaps between runs.
//
// # Alignment
//
// Wrap provides three alignment properties:
//   - Alignment: Controls main axis positioning within each run (Start, End, Center,
//     SpaceBetween, SpaceAround, SpaceEvenly)
//   - CrossAxisAlignment: Controls cross axis positioning within each run (Start, End, Center)
//   - RunAlignment: Controls distribution of runs along the cross axis (Start, End, Center,
//     SpaceBetween, SpaceAround, SpaceEvenly)
//
// # Direction
//
// Direction defaults to WrapAxisHorizontal (the zero value for WrapAxis).
// For vertical wrapping, set Direction to WrapAxisVertical.
//
// Example:
//
//	Wrap{
//	    Direction:  WrapAxisHorizontal,
//	    Spacing:    8,
//	    RunSpacing: 8,
//	    ChildrenWidgets: []core.Widget{
//	        Chip{Label: "Go"},
//	        Chip{Label: "Rust"},
//	        Chip{Label: "TypeScript"},
//	        Chip{Label: "Python"},
//	    },
//	}
//
// For non-wrapping horizontal layout, use [Row]. For non-wrapping vertical layout,
// use [Column].
type Wrap struct {
	ChildrenWidgets    []core.Widget
	Direction          WrapAxis           // WrapAxisHorizontal (zero value); set WrapAxisVertical for column-style wrapping
	Alignment          WrapAlignment      // Main axis alignment within runs
	CrossAxisAlignment WrapCrossAlignment // Cross axis alignment within runs
	RunAlignment       RunAlignment       // Distribution of runs in cross axis
	Spacing            float64            // Gap between items in a run
	RunSpacing         float64            // Gap between runs
}

// WrapOf creates a Wrap widget with the specified spacing and children.
// This is a convenience helper for the common horizontal wrap case.
func WrapOf(spacing, runSpacing float64, children ...core.Widget) Wrap {
	return Wrap{
		ChildrenWidgets: children,
		Direction:       WrapAxisHorizontal,
		Spacing:         spacing,
		RunSpacing:      runSpacing,
	}
}

func (w Wrap) CreateElement() core.Element {
	return core.NewRenderObjectElement(w, nil)
}

func (w Wrap) Key() any {
	return nil
}

func (w Wrap) Children() []core.Widget {
	return w.ChildrenWidgets
}

func (w Wrap) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	wrap := &renderWrap{
		direction:          w.Direction,
		alignment:          w.Alignment,
		crossAxisAlignment: w.CrossAxisAlignment,
		runAlignment:       w.RunAlignment,
		spacing:            w.Spacing,
		runSpacing:         w.RunSpacing,
	}
	wrap.SetSelf(wrap)
	return wrap
}

func (w Wrap) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if wrap, ok := renderObject.(*renderWrap); ok {
		wrap.direction = w.Direction
		wrap.alignment = w.Alignment
		wrap.crossAxisAlignment = w.CrossAxisAlignment
		wrap.runAlignment = w.RunAlignment
		wrap.spacing = w.Spacing
		wrap.runSpacing = w.RunSpacing
		wrap.MarkNeedsLayout()
		wrap.MarkNeedsPaint()
	}
}

// runMetrics stores layout information for a single run of children.
type runMetrics struct {
	mainAxisExtent  float64 // Total main axis size of children + spacing
	crossAxisExtent float64 // Max cross axis size in this run
	childCount      int
	firstChildIndex int
}

type renderWrap struct {
	layout.RenderBoxBase
	children           []layout.RenderBox
	direction          WrapAxis
	alignment          WrapAlignment
	crossAxisAlignment WrapCrossAlignment
	runAlignment       RunAlignment
	spacing            float64
	runSpacing         float64
}

func (r *renderWrap) SetChildren(children []layout.RenderObject) {
	for _, child := range r.children {
		setParentOnChild(child, nil)
	}
	r.children = r.children[:0]
	for _, child := range children {
		if box, ok := child.(layout.RenderBox); ok {
			r.children = append(r.children, box)
			setParentOnChild(box, r)
		}
	}
}

func (r *renderWrap) VisitChildren(visitor func(layout.RenderObject)) {
	for _, child := range r.children {
		visitor(child)
	}
}

func (r *renderWrap) mainAxis(size graphics.Size) float64 {
	if r.direction == WrapAxisVertical {
		return size.Height
	}
	return size.Width
}

func (r *renderWrap) crossAxis(size graphics.Size) float64 {
	if r.direction == WrapAxisVertical {
		return size.Width
	}
	return size.Height
}

func (r *renderWrap) makeSize(main, cross float64) graphics.Size {
	if r.direction == WrapAxisVertical {
		return graphics.Size{Width: cross, Height: main}
	}
	return graphics.Size{Width: main, Height: cross}
}

func (r *renderWrap) makeOffset(main, cross float64) graphics.Offset {
	if r.direction == WrapAxisVertical {
		return graphics.Offset{X: cross, Y: main}
	}
	return graphics.Offset{X: main, Y: cross}
}

func (r *renderWrap) PerformLayout() {
	constraints := r.Constraints()
	maxSize := graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight}
	maxMain := r.mainAxis(maxSize)
	maxCross := r.crossAxis(maxSize)

	// Validate: main axis must be bounded
	if maxMain == math.MaxFloat64 {
		mainAxisName := "width"
		containerType := "horizontal"
		if r.direction == WrapAxisVertical {
			mainAxisName = "height"
			containerType = "vertical"
		}
		panic(fmt.Sprintf(
			"Wrap (%s) used with unbounded %s.\n\n"+
				"Wrap needs a finite main axis to determine when to wrap to a new run. This happens when:\n"+
				"- The Wrap is inside a ScrollView scrolling in the same direction\n"+
				"- The Wrap is a direct child of another flex without constrained %s\n\n"+
				"Solutions:\n"+
				"- Ensure parent provides bounded %s constraints\n"+
				"- Wrap in a SizedBox or Container with a fixed %s\n"+
				"- Use Row/Column instead if wrapping is not needed",
			containerType, mainAxisName,
			mainAxisName,
			mainAxisName,
			mainAxisName,
		))
	}

	if len(r.children) == 0 {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}

	// Phase 1: Measure children with loose constraints and group into runs
	childConstraints := layout.Loose(maxSize)
	runs := make([]runMetrics, 0)
	var currentRun runMetrics
	currentRun.firstChildIndex = 0

	for i, child := range r.children {
		child.Layout(childConstraints, true)
		childSize := child.Size()
		childMain := r.mainAxis(childSize)
		childCross := r.crossAxis(childSize)

		// Check if we need to start a new run
		spacingToAdd := 0.0
		if currentRun.childCount > 0 {
			spacingToAdd = r.spacing
		}

		if currentRun.childCount > 0 && currentRun.mainAxisExtent+spacingToAdd+childMain > maxMain {
			// Finish current run and start a new one
			runs = append(runs, currentRun)
			currentRun = runMetrics{
				firstChildIndex: i,
			}
			spacingToAdd = 0
		}

		currentRun.mainAxisExtent += spacingToAdd + childMain
		currentRun.crossAxisExtent = math.Max(currentRun.crossAxisExtent, childCross)
		currentRun.childCount++
	}

	// Don't forget the last run
	if currentRun.childCount > 0 {
		runs = append(runs, currentRun)
	}

	// Phase 2: Calculate total cross axis size
	totalCrossExtent := 0.0
	for i, run := range runs {
		totalCrossExtent += run.crossAxisExtent
		if i > 0 {
			totalCrossExtent += r.runSpacing
		}
	}

	// Determine final size
	crossSize := totalCrossExtent
	if maxCross != math.MaxFloat64 {
		if r.direction == WrapAxisVertical {
			crossSize = math.Max(crossSize, constraints.MinWidth)
		} else {
			crossSize = math.Max(crossSize, constraints.MinHeight)
		}
	}

	finalSize := constraints.Constrain(r.makeSize(maxMain, crossSize))
	r.SetSize(finalSize)

	// Phase 3: Position runs and children
	finalCrossSize := r.crossAxis(finalSize)
	freeCrossSpace := math.Max(0, finalCrossSize-totalCrossExtent)
	runSpacing, runOffset := r.computeRunSpacing(freeCrossSpace, len(runs))

	crossCursor := runOffset
	childIndex := 0
	for _, run := range runs {
		// Compute spacing within this run
		freeMainSpace := math.Max(0, maxMain-run.mainAxisExtent)
		itemSpacing, mainOffset := r.computeMainSpacing(freeMainSpace, run.childCount)

		mainCursor := mainOffset
		for i := 0; i < run.childCount; i++ {
			child := r.children[childIndex]
			childSize := child.Size()
			childCross := r.crossAxis(childSize)

			// Cross axis alignment within the run
			crossOffset := r.computeCrossOffset(run.crossAxisExtent, childCross)

			child.SetParentData(&layout.BoxParentData{
				Offset: r.makeOffset(mainCursor, crossCursor+crossOffset),
			})

			mainCursor += r.mainAxis(childSize) + itemSpacing
			if i < run.childCount-1 {
				mainCursor += r.spacing
			}
			childIndex++
		}

		crossCursor += run.crossAxisExtent + runSpacing + r.runSpacing
	}
}

func (r *renderWrap) computeMainSpacing(freeSpace float64, count int) (spacing, offset float64) {
	if count == 0 {
		return 0, 0
	}
	switch r.alignment {
	case WrapAlignmentEnd:
		offset = freeSpace
	case WrapAlignmentCenter:
		offset = freeSpace * 0.5
	case WrapAlignmentSpaceBetween:
		if count > 1 {
			spacing = freeSpace / float64(count-1)
		}
	case WrapAlignmentSpaceAround:
		spacing = freeSpace / float64(count)
		offset = spacing * 0.5
	case WrapAlignmentSpaceEvenly:
		spacing = freeSpace / float64(count+1)
		offset = spacing
	}
	return
}

func (r *renderWrap) computeRunSpacing(freeSpace float64, count int) (spacing, offset float64) {
	if count == 0 {
		return 0, 0
	}
	switch r.runAlignment {
	case RunAlignmentEnd:
		offset = freeSpace
	case RunAlignmentCenter:
		offset = freeSpace * 0.5
	case RunAlignmentSpaceBetween:
		if count > 1 {
			spacing = freeSpace / float64(count-1)
		}
	case RunAlignmentSpaceAround:
		spacing = freeSpace / float64(count)
		offset = spacing * 0.5
	case RunAlignmentSpaceEvenly:
		spacing = freeSpace / float64(count+1)
		offset = spacing
	}
	return
}

func (r *renderWrap) computeCrossOffset(runCrossExtent, childCrossExtent float64) float64 {
	freeSpace := runCrossExtent - childCrossExtent
	if freeSpace <= 0 {
		return 0
	}
	switch r.crossAxisAlignment {
	case WrapCrossAlignmentEnd:
		return freeSpace
	case WrapCrossAlignmentCenter:
		return freeSpace * 0.5
	default:
		return 0
	}
}

func (r *renderWrap) Paint(ctx *layout.PaintContext) {
	for _, child := range r.children {
		ctx.PaintChildWithLayer(child, getChildOffset(child))
	}
}

func (r *renderWrap) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	for i := len(r.children) - 1; i >= 0; i-- {
		child := r.children[i]
		offset := getChildOffset(child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if child.HitTest(local, result) {
			return true
		}
	}
	result.Add(r)
	return true
}
