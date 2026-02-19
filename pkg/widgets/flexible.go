package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
)

// Flexible allows its child to participate in flex space distribution within
// a [Row] or [Column] without requiring the child to fill all allocated space.
//
// # Fit Behavior
//
// By default (zero value), Flexible uses [FlexFitLoose], which allows the child
// to be smaller than its allocated space. The child receives loose constraints
// where MinWidth/MinHeight is 0 and MaxWidth/MaxHeight is the allocated space.
//
// Set Fit to [FlexFitTight] for behavior equivalent to [Expanded], where the
// child must fill exactly its allocated space.
//
// # When to Use Flexible vs Expanded
//
// Use [Flexible] when the child should participate in flex distribution but
// may not need all allocated space (e.g., text that might be short).
//
// Use [Expanded] when the child must fill all remaining space (e.g., a panel
// or container that should stretch).
//
// # Example
//
// A row where text takes only the space it needs, while a panel fills the rest:
//
//	Row{
//	    Children: []core.Widget{
//	        Flexible{Child: Text{Content: "Short"}},  // Uses only needed width
//	        Expanded{Child: panel},                   // Fills remaining space
//	    },
//	}
//
// # Example with Flex Factors
//
// Distribute space proportionally while allowing children to be smaller:
//
//	Row{
//	    Children: []core.Widget{
//	        Flexible{Flex: 1, Child: smallWidget},  // Gets up to 1/3 of space
//	        Flexible{Flex: 2, Child: largeWidget},  // Gets up to 2/3 of space
//	    },
//	}
type Flexible struct {
	// Child is the widget to display within the flexible space.
	Child core.Widget

	// Flex determines the ratio of space allocated to this child relative to
	// other flexible children. Defaults to 1 if not set or <= 0.
	//
	// For example, in a Row with two Flexible children where one has Flex: 1
	// and the other has Flex: 2, the remaining space is split 1:2.
	Flex int

	// Fit controls whether the child must fill its allocated space.
	// The zero value is [FlexFitLoose], allowing the child to be smaller.
	// Set to [FlexFitTight] for behavior equivalent to [Expanded].
	Fit FlexFit
}

// CreateElement returns a RenderObjectElement for this Flexible.
func (f Flexible) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

// Key returns nil (no key).
func (f Flexible) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (f Flexible) ChildWidget() core.Widget {
	return f.Child
}

// CreateRenderObject creates the renderFlexChild.
func (f Flexible) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderFlexChild{
		flex: f.effectiveFlex(),
		fit:  f.Fit, // Zero value is FlexFitLoose
	}
	r.SetSelf(r)
	return r
}

// UpdateRenderObject updates the renderFlexChild.
func (f Flexible) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderFlexChild); ok {
		r.flex = f.effectiveFlex()
		r.fit = f.Fit
		r.MarkNeedsLayout()
	}
}

// effectiveFlex returns the flex factor, defaulting to 1 if not set.
func (f Flexible) effectiveFlex() int {
	if f.Flex <= 0 {
		return 1
	}
	return f.Flex
}
