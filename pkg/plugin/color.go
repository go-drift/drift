package plugin

import "github.com/go-drift/drift/pkg/plugin/protocol"

// Color is a colour in Drift's hex format, "#RRGGBB" or "#RRGGBBAA" with
// alpha last: the format the `hex` config tag accepts and
// AndroidResourcesScope.Colors takes. Use it to write a colour into a
// platform file a plugin generates itself (a storyboard, a values-night
// XML), which must use that platform's own format.
type Color = protocol.Color

// ParseColor parses Drift's hex format. Six digits mean opaque.
func ParseColor(s string) (Color, error) {
	return protocol.ParseColor(s)
}
