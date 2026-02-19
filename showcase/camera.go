package main

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// cameraPage is the camera demo widget.
type cameraPage struct{ core.StatefulBase }

func (cameraPage) CreateState() core.State { return &cameraState{} }

func buildCameraPage(_ core.BuildContext) core.Widget { return cameraPage{} }

type cameraState struct {
	core.StateBase
	status           *core.Managed[string]
	image            *core.Managed[image.Image]
	permissionStatus *core.Managed[platform.PermissionStatus]
	unsubscribe      func()
}

func (s *cameraState) InitState() {
	s.status = core.NewManaged(s, "Tap a button to capture or select an image.")
	s.image = core.NewManaged[image.Image](s, nil)
	s.permissionStatus = core.NewManaged(s, platform.PermissionNotDetermined)

	ctx := context.Background()

	// Check initial permission status
	go func() {
		status, _ := platform.Camera.Permission.Status(ctx)
		drift.Dispatch(func() {
			s.permissionStatus.Set(status)
		})
	}()

	// Listen for permission changes
	s.unsubscribe = platform.Camera.Permission.Listen(func(status platform.PermissionStatus) {
		drift.Dispatch(func() {
			s.permissionStatus.Set(status)
		})
	})

	s.OnDispose(s.unsubscribe)
}

func (s *cameraState) handleResult(result platform.CameraResult) {
	if result.Cancelled {
		s.status.Set("Cancelled")
		return
	}

	if result.Error != "" {
		s.status.Set("Error: " + result.Error)
		return
	}

	var media *platform.CapturedMedia
	if result.Media != nil {
		media = result.Media
	} else if len(result.MediaList) > 0 {
		media = &result.MediaList[0]
	}

	if media == nil {
		s.status.Set("No media received")
		return
	}

	img, err := loadImageFromPath(media.Path)
	if err != nil {
		s.status.Set("Failed to load image: " + err.Error())
		return
	}

	s.image.Set(img)
	s.status.Set("Image loaded: " + itoa(media.Width) + "x" + itoa(media.Height))
}

func loadImageFromPath(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (s *cameraState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Camera",
		sectionTitle("Permission", colors),
		widgets.VSpace(8),
		widgets.Row{
			MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
			CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
			Children: []core.Widget{
				widgets.Text{Content: "Camera access:", Style: labelStyle(colors)},
				permissionBadge(s.permissionStatus.Value(), colors),
			},
		},
		widgets.VSpace(8),
		theme.ButtonOf(ctx, "Request Permission", func() {
			go func() {
				ctx := context.Background()
				status, _ := platform.Camera.Permission.Request(ctx)
				drift.Dispatch(func() {
					s.permissionStatus.Set(status)
				})
			}()
		}),
		widgets.VSpace(24),

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
		// Use timeout to prevent blocking forever if native layer fails
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := platform.Camera.CapturePhoto(ctx, platform.CapturePhotoOptions{
			UseFrontCamera: useFront,
		})
		drift.Dispatch(func() {
			if err != nil {
				s.status.Set("Error: " + err.Error())
				return
			}
			s.handleResult(result)
		})
	}()
}

func (s *cameraState) pickFromGallery() {
	s.status.Set("Opening gallery...")
	go func() {
		// Use timeout to prevent blocking forever if native layer fails
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := platform.Camera.PickFromGallery(ctx, platform.PickFromGalleryOptions{})
		drift.Dispatch(func() {
			if err != nil {
				s.status.Set("Error: " + err.Error())
				return
			}
			s.handleResult(result)
		})
	}()
}
