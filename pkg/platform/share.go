package platform

import "github.com/go-drift/drift/pkg/errors"

// ShareService provides access to the system share sheet.
var Share = &ShareService{
	channel: NewMethodChannel("github.com/go-drift/drift/share"),
}

// ShareService manages sharing content with other apps.
type ShareService struct {
	channel *MethodChannel
}

// ShareResult indicates the result of a share operation.
type ShareResult string

const (
	// ShareResultSuccess indicates the content was shared successfully.
	ShareResultSuccess ShareResult = "success"

	// ShareResultDismissed indicates the user dismissed the share sheet.
	ShareResultDismissed ShareResult = "dismissed"

	// ShareResultUnavailable indicates sharing is not available.
	ShareResultUnavailable ShareResult = "unavailable"
)

// ShareText opens the share sheet with the given text.
func (s *ShareService) ShareText(text string) (ShareResult, error) {
	return s.share(map[string]any{
		"text": text,
	})
}

// ShareTextWithSubject opens the share sheet with text and a subject line.
// The subject is used by email apps and similar.
func (s *ShareService) ShareTextWithSubject(text, subject string) (ShareResult, error) {
	return s.share(map[string]any{
		"text":    text,
		"subject": subject,
	})
}

// ShareURL opens the share sheet with a URL.
func (s *ShareService) ShareURL(url string) (ShareResult, error) {
	return s.share(map[string]any{
		"url": url,
	})
}

// ShareURLWithText opens the share sheet with a URL and accompanying text.
func (s *ShareService) ShareURLWithText(url, text string) (ShareResult, error) {
	return s.share(map[string]any{
		"url":  url,
		"text": text,
	})
}

// ShareFile opens the share sheet with a file at the given path.
// The mimeType helps the system determine which apps can handle the file.
func (s *ShareService) ShareFile(filePath, mimeType string) (ShareResult, error) {
	return s.share(map[string]any{
		"file":     filePath,
		"mimeType": mimeType,
	})
}

// ShareFiles opens the share sheet with multiple files.
func (s *ShareService) ShareFiles(files []ShareFile) (ShareResult, error) {
	fileData := make([]map[string]any, len(files))
	for i, f := range files {
		fileData[i] = map[string]any{
			"path":     f.Path,
			"mimeType": f.MimeType,
		}
	}
	return s.share(map[string]any{
		"files": fileData,
	})
}

// ShareFile represents a file to share.
type ShareFile struct {
	Path     string
	MimeType string
}

func (s *ShareService) share(data map[string]any) (ShareResult, error) {
	result, err := s.channel.Invoke("share", data)
	if err != nil {
		return ShareResultUnavailable, err
	}

	if r, ok := result.(string); ok {
		return ShareResult(r), nil
	}

	if m, ok := result.(map[string]any); ok {
		if r, ok := m["result"].(string); ok {
			return ShareResult(r), nil
		}
	}

	// Report unexpected result format (but still return success for compatibility)
	if result != nil {
		errors.Report(&errors.DriftError{
			Op:      "share.parseResult",
			Kind:    errors.KindParsing,
			Channel: "github.com/go-drift/drift/share",
			Err: &errors.ParseError{
				Channel:  "github.com/go-drift/drift/share",
				DataType: "ShareResult",
				Got:      result,
			},
		})
	}

	return ShareResultSuccess, nil
}
