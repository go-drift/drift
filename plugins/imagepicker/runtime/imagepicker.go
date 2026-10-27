// Package runtime is the Go API of the Drift image picker plugin
// (github.com/go-drift/drift/plugins/imagepicker/plugin in drift.yaml): a
// photo from the camera, or images from the photo library.
//
//	import imagepicker "github.com/go-drift/drift/plugins/imagepicker/runtime"
//
//	go func() {
//	    images, err := imagepicker.Pick(ctx, imagepicker.PickOptions{})
//	    if err != nil || len(images) == 0 {
//	        return // failed, or the user canceled
//	    }
//	    drift.Dispatch(func() { s.showImage(images[0].Path) })
//	}()
//
// Capture and Pick wait for the user, so call them from a goroutine: from
// the UI thread they fail with platform.ErrBlocksUIThread. One picker is
// open at a time.
//
// Decode an Image's file with Decode from github.com/go-drift/drift/pkg/image,
// not the standard library's image.Decode: photos are stored as the camera
// sensor saw them, with an EXIF orientation that turns them upright, which
// only the former applies.
package runtime

import (
	"context"
	"errors"
	"fmt"

	drifterrors "github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/platform"
)

// channelName must match the literal in the plugin's native sources
// (plugin/ios/DriftImagePickerPlugin.swift, plugin/android/*.kt).
const channelName = "drift/imagepicker"

var channel = platform.NewMethodChannel(channelName).WithErrors(errorCodes)

var (
	// ErrBusy indicates a picker is already open.
	ErrBusy = errors.New("an image picker is already open")
	// ErrCameraUnavailable indicates the device has no camera (or no app
	// to take photos), such as the iOS Simulator.
	ErrCameraUnavailable = errors.New("no camera available")
	// ErrCameraNotConfigured indicates Capture on iOS without the plugin's
	// camera: config, which declares the camera usage string.
	ErrCameraNotConfigured = errors.New("taking photos needs the imagepicker plugin's camera: config")
	// ErrPermissionDenied indicates the user denied camera access.
	ErrPermissionDenied = errors.New("camera permission denied")
	// ErrNoActivity indicates the app has no screen to show a picker over
	// (Android, before the first Activity or after the last).
	ErrNoActivity = errors.New("no screen to show the picker over")
)

// errorCodes are the native halves' error codes for the sentinels.
var errorCodes = platform.ErrorCodes{
	"busy":                  ErrBusy,
	"camera_unavailable":    ErrCameraUnavailable,
	"camera_not_configured": ErrCameraNotConfigured,
	"permission_denied":     ErrPermissionDenied,
	"no_activity":           ErrNoActivity,
}

// Image is a picked or captured image, copied to a file the app owns in
// its cache directory. Move it elsewhere to keep it.
type Image struct {
	// Path is the image file's absolute path.
	Path string
	// MimeType is the file's type, always one github.com/go-drift/drift/pkg/image
	// Decode reads: "image/jpeg", "image/png", "image/gif" or "image/webp".
	// iOS converts every image to JPEG; Android converts a pick in any other
	// format (HEIC, AVIF) to JPEG.
	MimeType string
	// Width and Height are the image's size in pixels as displayed, after
	// its EXIF orientation: the bounds github.com/go-drift/drift/pkg/image
	// Decode returns for it.
	Width, Height int
	// Size is the file size in bytes.
	Size int64
}

// CaptureOptions configures Capture.
type CaptureOptions struct {
	// FrontCamera opens the front camera. On Android it is a hint that
	// not every camera app honours.
	FrontCamera bool
}

// PickOptions configures Pick.
type PickOptions struct {
	// Multiple lets the user pick several images.
	Multiple bool
}

// Capture opens the camera and returns the photo taken, or nil if the
// user canceled. It waits for the user, so call it from a goroutine.
func Capture(ctx context.Context, opts CaptureOptions) (*Image, error) {
	result, err := channel.Invoke(ctx, "capture", map[string]any{"frontCamera": opts.FrontCamera})
	if err != nil {
		return nil, fmt.Errorf("imagepicker capture: %w", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		return nil, &drifterrors.ParseError{Channel: channelName, DataType: "capture result", Got: result}
	}
	if m["image"] == nil {
		return nil, nil
	}
	img, err := parseImage(m["image"])
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// Pick opens the photo library and returns the images picked, none if the
// user canceled. It waits for the user, so call it from a goroutine.
func Pick(ctx context.Context, opts PickOptions) ([]Image, error) {
	result, err := channel.Invoke(ctx, "pick", map[string]any{"multiple": opts.Multiple})
	if err != nil {
		return nil, fmt.Errorf("imagepicker pick: %w", err)
	}
	m, _ := result.(map[string]any)
	list, ok := m["images"].([]any)
	if !ok {
		return nil, &drifterrors.ParseError{Channel: channelName, DataType: "pick result", Got: result}
	}
	images := make([]Image, 0, len(list))
	for _, item := range list {
		img, err := parseImage(item)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func parseImage(data any) (Image, error) {
	m, ok := data.(map[string]any)
	path, _ := m["path"].(string)
	mime, _ := m["mimeType"].(string)
	width, okW := m["width"].(float64)
	height, okH := m["height"].(float64)
	size, okS := m["size"].(float64)
	if !ok || path == "" || mime == "" || !okW || !okH || !okS {
		return Image{}, &drifterrors.ParseError{Channel: channelName, DataType: "Image", Got: data}
	}
	return Image{Path: path, MimeType: mime, Width: int(width), Height: int(height), Size: int64(size)}, nil
}
