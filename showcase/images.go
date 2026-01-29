package main

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

var (
	goLogoOnce  sync.Once
	goLogoImage image.Image
)

func loadImageAsset(name string) image.Image {
	file, err := assetFS.Open("assets/" + name)
	if err != nil {
		return nil
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil
	}
	return img
}

func loadGoLogo() image.Image {
	goLogoOnce.Do(func() {
		goLogoImage = loadImageAsset("go-logo.png")
	})
	return goLogoImage
}

func buildImagesPage(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	logo := loadGoLogo()

	return demoPage(ctx, "Images",
		sectionTitle("Raster Images", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Decoded with Go's image package:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Image{
			Source: logo,
			Width:  180,
		},
		widgets.VSpace(24),

		sectionTitle("Fit Modes", colors),
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			ChildrenWidgets: []core.Widget{
				fitPreview("Fill", widgets.ImageFitFill, logo, colors),
				widgets.HSpace(12),
				fitPreview("Contain", widgets.ImageFitContain, logo, colors),
			},
		},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			ChildrenWidgets: []core.Widget{
				fitPreview("Cover", widgets.ImageFitCover, logo, colors),
				widgets.HSpace(12),
				fitPreview("None", widgets.ImageFitNone, logo, colors),
			},
		},
		widgets.VSpace(12),
		fitPreview("ScaleDown", widgets.ImageFitScaleDown, logo, colors),
		widgets.VSpace(40),
	)
}

func fitPreview(label string, fit widgets.ImageFit, logo image.Image, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		MainAxisSize:       widgets.MainAxisSizeMin,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		ChildrenWidgets: []core.Widget{
			widgets.Text{Content: label, Style: labelStyle(colors)},
			widgets.VSpace(4),
			widgets.Container{
				Color:     colors.SurfaceVariant,
				Width:     100,
				Height:    100,
				Alignment: layout.AlignmentCenter,
				ChildWidget: widgets.Image{
					Source:    logo,
					Width:     100,
					Height:    100,
					Fit:       fit,
					Alignment: layout.AlignmentCenter,
				},
			},
		},
	}
}
