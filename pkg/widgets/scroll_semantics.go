//go:build android || darwin || ios

package widgets

import "github.com/go-drift/drift/pkg/semantics"

func (r *renderScrollView) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.Properties.Role = semantics.SemanticsRoleScrollView
	config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsHasImplicitScrolling)

	// Get scroll position info
	pos := r.position
	if pos != nil {
		scrollPos := pos.Offset()
		config.Properties.ScrollPosition = &scrollPos

		minExtent := pos.min
		config.Properties.ScrollExtentMin = &minExtent

		maxExtent := pos.max
		config.Properties.ScrollExtentMax = &maxExtent
	}

	// Add scroll actions
	config.Actions = semantics.NewSemanticsActions()

	if r.direction == AxisVertical {
		config.Actions.SetHandler(semantics.SemanticsActionScrollUp, func(args any) {
			if r.position != nil {
				r.position.SetOffset(r.position.Offset() - 100)
			}
		})
		config.Actions.SetHandler(semantics.SemanticsActionScrollDown, func(args any) {
			if r.position != nil {
				r.position.SetOffset(r.position.Offset() + 100)
			}
		})
	} else {
		config.Actions.SetHandler(semantics.SemanticsActionScrollLeft, func(args any) {
			if r.position != nil {
				r.position.SetOffset(r.position.Offset() - 100)
			}
		})
		config.Actions.SetHandler(semantics.SemanticsActionScrollRight, func(args any) {
			if r.position != nil {
				r.position.SetOffset(r.position.Offset() + 100)
			}
		})
	}

	return true
}
