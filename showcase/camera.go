package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildCameraPage creates a stateful widget for camera demos.
func buildCameraPage(ctx core.BuildContext) core.Widget {
	return cameraPage{}
}

type cameraPage struct{}

func (c cameraPage) CreateElement() core.Element {
	return core.NewStatefulElement(c, nil)
}

func (c cameraPage) Key() any {
	return nil
}

func (c cameraPage) CreateState() core.State {
	return &cameraState{}
}

type cameraState struct {
	core.StateBase
	statusText *core.ManagedState[string]
	imagePath  *core.ManagedState[string]
	imageInfo  *core.ManagedState[string]
}

func (s *cameraState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Tap a button to capture or pick an image.")
	s.imagePath = core.NewManagedState(&s.StateBase, "")
	s.imageInfo = core.NewManagedState(&s.StateBase, "")

	// Listen for camera results
	go func() {
		for result := range platform.CameraResults() {
			drift.Dispatch(func() {
				if result.Cancelled {
					s.statusText.Set("Operation cancelled")
					return
				}

				if result.Type == "capture" && result.Media != nil {
					s.imagePath.Set(result.Media.Path)
					s.imageInfo.Set(formatMediaInfo(result.Media))
					s.statusText.Set("Photo captured")
				} else if result.Type == "gallery" && len(result.MediaList) > 0 {
					media := &result.MediaList[0]
					s.imagePath.Set(media.Path)
					s.imageInfo.Set(formatMediaInfo(media))
					s.statusText.Set("Image selected from gallery")
				}
			})
		}
	}()
}

func formatMediaInfo(media *platform.CapturedMedia) string {
	info := media.MimeType
	if media.Width > 0 && media.Height > 0 {
		info += " | " + itoa(media.Width) + "x" + itoa(media.Height)
	}
	if media.Size > 0 {
		info += " | " + formatSize(media.Size)
	}
	return info
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return itoa(int(bytes)) + " B"
	} else if bytes < 1024*1024 {
		return itoa(int(bytes/1024)) + " KB"
	} else {
		return itoa(int(bytes/(1024*1024))) + " MB"
	}
}

func (s *cameraState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)
	imagePath := s.imagePath.Get()

	return demoPage(ctx, "Camera",
		sectionTitle("Capture Photo", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Take a photo using the device camera:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			ChildrenWidgets: []core.Widget{
				theme.ButtonOf(ctx, "Take Photo", func() {
					s.takePhoto(false)
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Take Selfie", func() {
					s.takePhoto(true)
				}).WithColor(colors.Secondary, colors.OnSecondary),
			},
		},
		widgets.VSpace(24),

		sectionTitle("Gallery", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Pick an image from the photo library:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		theme.ButtonOf(ctx, "Pick from Gallery", func() {
			s.pickFromGallery()
		}).WithColor(colors.Tertiary, colors.OnTertiary),
		widgets.VSpace(24),

		sectionTitle("Preview", colors),
		widgets.VSpace(12),
		s.imagePreview(imagePath, colors),
		widgets.VSpace(16),

		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(40),
	)
}

func (s *cameraState) imagePreview(path string, colors theme.ColorScheme) core.Widget {
	if path == "" {
		return widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Padding:      layout.EdgeInsetsAll(24),
			Alignment:    layout.AlignmentCenter,
			ChildWidget: widgets.Text{Content: "No image captured yet", Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}},
		}
	}

	imageInfo := s.imageInfo.Get()
	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		ChildWidget: widgets.PaddingAll(16,
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				ChildrenWidgets: []core.Widget{
					widgets.Text{Content: "Captured Image", Style: graphics.TextStyle{
						Color:      colors.OnSurface,
						FontSize:   16,
						FontWeight: graphics.FontWeightSemibold,
					}},
					widgets.VSpace(8),
					widgets.Text{Content: imageInfo, Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 13,
					}},
					widgets.VSpace(12),
					widgets.Text{Content: path, Style: graphics.TextStyle{
						Color:    colors.OnSurfaceVariant,
						FontSize: 11,
					}, Wrap: true},
				},
			},
		),
	}
}

func (s *cameraState) takePhoto(useFrontCamera bool) {
	s.statusText.Set("Opening camera...")

	err := platform.CapturePhoto(platform.CapturePhotoOptions{
		Quality:        80,
		UseFrontCamera: useFrontCamera,
		SaveToGallery:  false,
	})
	if err != nil {
		s.statusText.Set("Error: " + err.Error())
	}
}

func (s *cameraState) pickFromGallery() {
	s.statusText.Set("Opening gallery...")

	err := platform.PickFromGallery(platform.PickFromGalleryOptions{
		AllowMultiple: false,
		MediaType:     platform.MediaTypeImage,
	})
	if err != nil {
		s.statusText.Set("Error: " + err.Error())
	}
}
