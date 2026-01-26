//go:build android || darwin || ios

package widgets

import "github.com/go-drift/drift/pkg/semantics"

func (r *renderSVGIcon) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	if r.excludeFromSemantics {
		return false
	}

	// Icons are images
	config.Properties.Role = semantics.SemanticsRoleImage
	config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsIsImage)

	if r.semanticLabel != "" {
		config.Properties.Label = r.semanticLabel
	}

	return true
}
