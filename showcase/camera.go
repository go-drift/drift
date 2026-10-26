package main

import (
	"context"
	"image"
	"os"
	"strconv"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	driftimage "github.com/go-drift/drift/pkg/image"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
	imagepicker "github.com/go-drift/drift/plugins/imagepicker/runtime"
)

// cameraPage is the camera demo widget.
type cameraPage struct{ core.StatefulBase }

func (cameraPage) CreateState() core.State { return &cameraState{} }

func buildCameraPage(_ core.BuildContext) core.Widget { return cameraPage{} }

type cameraState struct {
	core.StateBase
	status *core.Signal[string]
	image  *core.Signal[image.Image]
}

func (s *cameraState) InitState() {
	s.status = core.NewSignal("Tap a button to capture or select an image.")
	s.image = core.NewSignal[image.Image](nil)
	core.UseListenable(s, s.status)
	core.UseListenable(s, s.image)
}

// show loads img into the preview; nil means the user canceled.
func (s *cameraState) show(img *imagepicker.Image) {
	if img == nil {
		s.status.Set("Cancelled")
		return
	}
	decoded, err := loadImageFromPath(img.Path)
	if err != nil {
		s.status.Set("Failed to load image: " + err.Error())
		return
	}
	s.image.Set(decoded)
	s.status.Set("Image loaded: " + strconv.Itoa(img.Width) + "x" + strconv.Itoa(img.Height))
}

func loadImageFromPath(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// driftimage.Decode turns camera photos upright (EXIF orientation).
	img, _, err := driftimage.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (s *cameraState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Camera",
		sectionTitle("Camera Capture", colors),
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Take Photo", func() {
					s.capturePhoto(false)
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Selfie", func() {
					s.capturePhoto(true)
				}),
			},
		},
		widgets.VSpace(24),

		sectionTitle("Photo Library", colors),
		widgets.VSpace(12),
		theme.ButtonOf(ctx, "Pick from Gallery", func() {
			s.pickFromGallery()
		}),
		widgets.VSpace(24),

		sectionTitle("Preview", colors),
		widgets.VSpace(12),
		s.imagePreview(colors),
		widgets.VSpace(16),

		statusCard(s.status.Value(), colors),
		widgets.VSpace(40),
	)
}

func (s *cameraState) imagePreview(colors theme.ColorScheme) core.Widget {
	img := s.image.Value()

	if img == nil {
		return widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Width:        280,
			Height:       280,
			Alignment:    layout.AlignmentCenter,
			Child: widgets.Text{
				Content: "No image",
				Style: graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				},
			},
		}
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Overflow:     widgets.OverflowClip,
		Child: widgets.Image{
			Source:    img,
			Width:     280,
			Height:    280,
			Fit:       widgets.ImageFitCover,
			Alignment: layout.AlignmentCenter,
		},
	}
}

func (s *cameraState) capturePhoto(useFront bool) {
	s.status.Set("Opening camera...")
	go func() {
		img, err := imagepicker.Capture(context.Background(), imagepicker.CaptureOptions{FrontCamera: useFront})
		drift.Dispatch(func() {
			if err != nil {
				s.status.Set("Error: " + err.Error())
				return
			}
			s.show(img)
		})
	}()
}

func (s *cameraState) pickFromGallery() {
	s.status.Set("Opening gallery...")
	go func() {
		images, err := imagepicker.Pick(context.Background(), imagepicker.PickOptions{})
		drift.Dispatch(func() {
			if err != nil {
				s.status.Set("Error: " + err.Error())
				return
			}
			if len(images) == 0 {
				s.show(nil)
				return
			}
			s.show(&images[0])
		})
	}()
}
