//go:build android || darwin || ios
// +build android darwin ios

package widgets

import "github.com/go-drift/drift/pkg/semantics"

func (r *renderText) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	if r.text == "" {
		return false
	}

	// Text elements are readable content
	config.Properties.Label = r.text

	// Check if this looks like a heading (large/bold text)
	if r.style.FontSize >= 24 || r.style.FontWeight >= 600 {
		config.Properties.Flags = config.Properties.Flags.Set(semantics.SemanticsIsHeader)
		config.Properties.HeadingLevel = 1
		if r.style.FontSize < 32 {
			config.Properties.HeadingLevel = 2
		}
		if r.style.FontSize < 24 {
			config.Properties.HeadingLevel = 3
		}
	}

	return true
}
