package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
)

// Icon renders a single glyph with icon-friendly defaults.
type Icon struct {
	// Glyph is the text glyph to render.
	Glyph string
	// Size is the font size for the icon. Uses theme defaults if zero.
	Size float64
	// Color is the icon color. Uses theme defaults if zero.
	Color rendering.Color
	// Weight sets the font weight if non-zero.
	Weight rendering.FontWeight
}

func (i Icon) CreateElement() core.Element {
	return core.NewStatelessElement(i, nil)
}

func (i Icon) Key() any {
	return nil
}

func (i Icon) Build(ctx core.BuildContext) core.Widget {
	style := theme.TextThemeOf(ctx).LabelLarge
	if i.Size > 0 {
		style.FontSize = i.Size
	}
	if i.Color != rendering.ColorTransparent {
		style.Color = i.Color
	}
	if i.Weight != 0 {
		style.FontWeight = i.Weight
	}

	return Text{
		Content:  i.Glyph,
		Style:    style,
		MaxLines: 1,
	}
}
