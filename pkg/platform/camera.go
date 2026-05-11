package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	drifterrors "github.com/go-drift/drift/pkg/errors"
)

// CapturedMedia represents a captured or selected media file.
type CapturedMedia struct {
	// Path is the absolute file path to the media file in the app's temp directory.
	Path string

	// MimeType is the media type (e.g., "image/jpeg").
	MimeType string

	// Width is the image width in pixels.
	Width int

	// Height is the image height in pixels.
	Height int

	// Size is the file size in bytes.
	Size int64
}

// CameraResult represents the result of a camera or gallery operation.
type CameraResult struct {
	requestID string

	// Type indicates the operation: "capture" for camera, "gallery" for picker.
	Type string

	// Media contains the captured/selected media for single-selection operations.
	Media *CapturedMedia

	// MediaList contains multiple selected media when AllowMultiple is true.
	MediaList []CapturedMedia

	// Cancelled is true if the user dismissed the camera/picker without selecting.
	Cancelled bool

	// Error contains the error message if the operation failed.
	Error string
}

// CapturePhotoOptions configures photo capture behavior.
type CapturePhotoOptions struct {
	// UseFrontCamera opens the front-facing camera when true.
	// Note: On Android, this is a hint that not all camera apps honor.
	UseFrontCamera bool
}

// PickFromGalleryOptions configures gallery picker behavior.
type PickFromGalleryOptions struct {
	// AllowMultiple enables selecting multiple images from the gallery.
	// When true on Android, results are returned in CameraResult.MediaList.
	// Note: iOS does not support multi-select; only a single image is returned.
	AllowMultiple bool
}

// ErrCameraBusy is returned when a camera operation is already in progress.
// Native camera handlers only support one operation at a time.
var ErrCameraBusy = errors.New("camera operation already in progress")

// CameraService provides camera capture and photo library access.
type CameraService struct {
	// Permission for camera access. Request before capturing photos.
	Permission Permission

	state *cameraServiceState
	mu    sync.Mutex // serializes camera operations
}

// Camera is the singleton camera service.
var Camera *CameraService

func init() {
	Camera = &CameraService{
		Permission: newPerm("camera"),
		state:      newCameraService(),
	}
}

type cameraServiceState struct {
	channel *MethodChannel
	events  *EventChannel
}

func newCameraService() *cameraServiceState {
	return &cameraServiceState{
		channel: NewMethodChannel("drift/camera"),
		events:  NewEventChannel("drift/camera/result"),
	}
}

// CapturePhoto opens the native camera to capture a photo.
// Blocks until the user captures a photo, cancels, or the context expires.
// This method should be called from a goroutine, not the main/render thread.
//
// Returns ErrCameraBusy if another camera operation is already in progress.
//
// Important: Always pass a context with a deadline or timeout. If the native
// layer fails to send a result and the context has no deadline, this method
// blocks forever and holds the camera mutex, preventing further operations.
//
// Note: If the context is canceled, the native camera UI remains open.
// The user must dismiss it manually; there's no programmatic close.
func (c *CameraService) CapturePhoto(ctx context.Context, opts CapturePhotoOptions) (CameraResult, error) {
	// Serialize camera operations - native handlers only support one at a time
	if !c.mu.TryLock() {
		return CameraResult{}, ErrCameraBusy
	}
	defer c.mu.Unlock()

	requestID := generateRequestID()

	// Subscribe to results BEFORE triggering native
	resultChan := make(chan CameraResult, 1)
	errChan := make(chan error, 1)
	sub := c.state.events.Listen(EventHandler{
		OnEvent: func(data any) {
			result, err := parseCameraResultWithID(data)
			if err != nil {
				drifterrors.Report(&drifterrors.DriftError{
					Op:      "camera.parse",
					Kind:    drifterrors.KindParsing,
					Channel: "drift/camera/result",
					Err:     err,
				})
				// If we got a parse error but can't match requestID, we can't route it
				// to a specific caller. Report and continue listening.
				return
			}
			if result.requestID == requestID {
				select {
				case resultChan <- result:
				default:
				}
			}
		},
		OnError: func(err error) {
			drifterrors.Report(&drifterrors.DriftError{
				Op:      "camera.streamError",
				Kind:    drifterrors.KindPlatform,
				Channel: "drift/camera/result",
				Err:     err,
			})
			select {
			case errChan <- err:
			default:
			}
		},
	})
	defer sub.Cancel()

	// Trigger native camera with request ID
	_, err := c.state.channel.Invoke("capturePhoto", map[string]any{
		"useFrontCamera": opts.UseFrontCamera,
		"requestId":      requestID,
	})
	if err != nil {
		return CameraResult{}, err
	}

	// Wait for result or context cancellation
	select {
	case result := <-resultChan:
		if result.Error != "" {
			return result, errors.New(result.Error)
		}
		return result, nil
	case err := <-errChan:
		return CameraResult{}, err
	case <-ctx.Done():
		return CameraResult{}, ctx.Err()
	}
}

// PickFromGallery opens the photo picker.
// Blocks until the user selects images, cancels, or the context expires.
// This method should be called from a goroutine, not the main/render thread.
//
// Returns ErrCameraBusy if another camera operation is already in progress.
//
// Important: Always pass a context with a deadline or timeout. If the native
// layer fails to send a result and the context has no deadline, this method
// blocks forever and holds the camera mutex, preventing further operations.
//
// Note: If the context is canceled, the native picker UI remains open.
func (c *CameraService) PickFromGallery(ctx context.Context, opts PickFromGalleryOptions) (CameraResult, error) {
	// Serialize camera operations - native handlers only support one at a time
	if !c.mu.TryLock() {
		return CameraResult{}, ErrCameraBusy
	}
	defer c.mu.Unlock()

	requestID := generateRequestID()

	resultChan := make(chan CameraResult, 1)
	errChan := make(chan error, 1)
	sub := c.state.events.Listen(EventHandler{
		OnEvent: func(data any) {
			result, err := parseCameraResultWithID(data)
			if err != nil {
				drifterrors.Report(&drifterrors.DriftError{
					Op:      "camera.parse",
					Kind:    drifterrors.KindParsing,
					Channel: "drift/camera/result",
					Err:     err,
				})
				return
			}
			if result.requestID == requestID {
				select {
				case resultChan <- result:
				default:
				}
			}
		},
		OnError: func(err error) {
			drifterrors.Report(&drifterrors.DriftError{
				Op:      "camera.streamError",
				Kind:    drifterrors.KindPlatform,
				Channel: "drift/camera/result",
				Err:     err,
			})
			select {
			case errChan <- err:
			default:
			}
		},
	})
	defer sub.Cancel()

	_, err := c.state.channel.Invoke("pickFromGallery", map[string]any{
		"allowMultiple": opts.AllowMultiple,
		"requestId":     requestID,
	})
	if err != nil {
		return CameraResult{}, err
	}

	select {
	case result := <-resultChan:
		if result.Error != "" {
			return result, errors.New(result.Error)
		}
		return result, nil
	case err := <-errChan:
		return CameraResult{}, err
	case <-ctx.Done():
		return CameraResult{}, ctx.Err()
	}
}

// generateRequestID creates a unique ID for request correlation.
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// parseCameraResultWithID parses camera result including request ID.
// Returns an error if requestId is missing or empty, since we cannot
// correlate the result with a waiting caller.
func parseCameraResultWithID(data any) (CameraResult, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return CameraResult{}, errors.New("invalid result format: expected map")
	}

	requestID := parseString(m["requestId"])
	if requestID == "" {
		return CameraResult{}, errors.New("missing or empty requestId in camera result")
	}

	result := CameraResult{
		requestID: requestID,
		Type:      parseString(m["type"]),
		Cancelled: parseBool(m["cancelled"]),
		Error:     parseString(m["error"]),
	}

	if mediaData, ok := m["media"].(map[string]any); ok {
		result.Media = parseCapturedMedia(mediaData)
	}

	if mediaList, ok := m["mediaList"].([]any); ok {
		for _, item := range mediaList {
			if itemMap, ok := item.(map[string]any); ok {
				if media := parseCapturedMedia(itemMap); media != nil {
					result.MediaList = append(result.MediaList, *media)
				}
			}
		}
	}

	return result, nil
}

func parseCapturedMedia(m map[string]any) *CapturedMedia {
	path := parseString(m["path"])
	if path == "" {
		return nil
	}
	return &CapturedMedia{
		Path:     path,
		MimeType: parseString(m["mimeType"]),
		Width:    parseCameraInt(m["width"]),
		Height:   parseCameraInt(m["height"]),
		Size:     parseInt64(m["size"]),
	}
}

func parseCameraInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
