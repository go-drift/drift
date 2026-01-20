package platform

import "time"

var cameraService = newCameraService()

// CapturedMedia represents a captured photo or video.
type CapturedMedia struct {
	Path     string
	MimeType string
	Width    int
	Height   int
	Size     int64
	Duration time.Duration
}

// CameraResult represents a result from camera/gallery operations.
type CameraResult struct {
	Type      string           // "capture" or "gallery"
	Media     *CapturedMedia   // For capture results
	MediaList []CapturedMedia  // For gallery results
	Cancelled bool
}

// CapturePhotoOptions configures photo capture behavior.
type CapturePhotoOptions struct {
	Quality        int
	MaxWidth       int
	MaxHeight      int
	UseFrontCamera bool
	SaveToGallery  bool
}

// CaptureVideoOptions configures video capture behavior.
type CaptureVideoOptions struct {
	Quality        int
	MaxDuration    time.Duration
	UseFrontCamera bool
	SaveToGallery  bool
}

// PickFromGalleryOptions configures gallery picker behavior.
type PickFromGalleryOptions struct {
	AllowMultiple bool
	MediaType     MediaType
	MaxSelection  int
}

// MediaType specifies the type of media to pick.
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeAll   MediaType = "all"
)

// CapturePhoto captures a photo using the device camera.
// Results are delivered asynchronously via CameraResults().
func CapturePhoto(opts CapturePhotoOptions) error {
	return cameraService.capturePhoto(opts)
}

// CaptureVideo captures a video using the device camera.
// Results are delivered asynchronously via CameraResults().
func CaptureVideo(opts CaptureVideoOptions) error {
	return cameraService.captureVideo(opts)
}

// PickFromGallery opens the device gallery to pick media.
// Results are delivered asynchronously via CameraResults().
func PickFromGallery(opts PickFromGalleryOptions) error {
	return cameraService.pickFromGallery(opts)
}

// CameraResults returns a channel that receives camera operation results.
func CameraResults() <-chan CameraResult {
	return cameraService.resultChannel()
}

type cameraServiceState struct {
	channel  *MethodChannel
	results  *EventChannel
	resultCh chan CameraResult
}

func newCameraService() *cameraServiceState {
	service := &cameraServiceState{
		channel:  NewMethodChannel("drift/camera"),
		results:  NewEventChannel("drift/camera/result"),
		resultCh: make(chan CameraResult, 4),
	}

	service.results.Listen(EventHandler{OnEvent: func(data any) {
		if result, ok := parseCameraResult(data); ok {
			service.resultCh <- result
		}
	}})

	return service
}

func (s *cameraServiceState) capturePhoto(opts CapturePhotoOptions) error {
	_, err := s.channel.Invoke("capturePhoto", map[string]any{
		"quality":        opts.Quality,
		"maxWidth":       opts.MaxWidth,
		"maxHeight":      opts.MaxHeight,
		"useFrontCamera": opts.UseFrontCamera,
		"saveToGallery":  opts.SaveToGallery,
	})
	return err
}

func (s *cameraServiceState) captureVideo(opts CaptureVideoOptions) error {
	_, err := s.channel.Invoke("captureVideo", map[string]any{
		"quality":        opts.Quality,
		"maxDurationMs":  opts.MaxDuration.Milliseconds(),
		"useFrontCamera": opts.UseFrontCamera,
		"saveToGallery":  opts.SaveToGallery,
	})
	return err
}

func (s *cameraServiceState) pickFromGallery(opts PickFromGalleryOptions) error {
	mediaType := string(opts.MediaType)
	if mediaType == "" {
		mediaType = string(MediaTypeAll)
	}

	_, err := s.channel.Invoke("pickFromGallery", map[string]any{
		"allowMultiple": opts.AllowMultiple,
		"mediaType":     mediaType,
		"maxSelection":  opts.MaxSelection,
	})
	return err
}

func (s *cameraServiceState) resultChannel() <-chan CameraResult {
	return s.resultCh
}

func parseCameraResult(data any) (CameraResult, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return CameraResult{}, false
	}

	result := CameraResult{
		Type:      parseString(m["type"]),
		Cancelled: parseBool(m["cancelled"]),
	}

	if result.Type == "gallery" {
		if mediaList, ok := m["media"].([]any); ok {
			for _, item := range mediaList {
				if cm, ok := parseCapturedMedia(item); ok {
					result.MediaList = append(result.MediaList, cm)
				}
			}
		}
	} else if result.Type == "capture" {
		if cm, ok := parseCapturedMedia(m); ok {
			result.Media = &cm
		}
	}

	return result, true
}

func parseCapturedMedia(result any) (CapturedMedia, bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return CapturedMedia{}, false
	}
	return CapturedMedia{
		Path:     parseString(m["path"]),
		MimeType: parseString(m["mimeType"]),
		Width:    int(parseInt64(m["width"])),
		Height:   int(parseInt64(m["height"])),
		Size:     parseInt64(m["size"]),
		Duration: time.Duration(parseInt64(m["durationMs"])) * time.Millisecond,
	}, true
}
