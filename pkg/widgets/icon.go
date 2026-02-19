package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
)

// Icon renders a single glyph with icon-friendly defaults.
//
// # Styling Model
//
// Icon is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - Size: 0 means zero size (not rendered)
//   - Color: 0 means transparent (invisible)
//
// For theme-styled icons, use [theme.IconOf] which sets Size to 24 and Color
// to the theme's OnSurface color.
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Icon{
//	    Glyph: "★",
//	    Size:  32,
//	    Color: graphics.RGB(255, 193, 7),
//	}
//
// Themed (reads from current theme):
//
//	theme.IconOf(ctx, "✓")
//	// Pre-filled with standard size (24) and theme color
//
// Icon renders the glyph as a Text widget with MaxLines: 1 and the specified
// size and color. Use Weight to control font weight if needed.
type Icon struct {
	core.StatelessBase

	// Glyph is the text glyph to render.
	Glyph string
	// Size is the font size for the icon. Zero means zero size (not rendered).
	Size float64
	// Color is the icon color. Zero means transparent.
	Color graphics.Color
	// Weight sets the font weight if non-zero.
	Weight graphics.FontWeight
}

func (i Icon) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero
	style := graphics.TextStyle{
		FontSize:   i.Size,
		Color:      i.Color,
		FontWeight: i.Weight,
	}

	return Text{
		Content:  i.Glyph,
		Style:    style,
		MaxLines: 1,
	}
}
