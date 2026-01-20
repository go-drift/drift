package platform

var storageService = newStorageService()

// PickedFile represents a file selected by the user.
type PickedFile struct {
	Name     string
	Path     string
	URI      string
	MimeType string
	Size     int64
}

// FileInfo contains metadata about a file.
type FileInfo struct {
	Name         string
	Path         string
	Size         int64
	MimeType     string
	IsDirectory  bool
	LastModified int64
}

// StorageResult represents a result from storage picker operations.
type StorageResult struct {
	Type      string       // "pickFile", "pickDirectory", or "saveFile"
	Files     []PickedFile // For pickFile results
	Path      string       // For pickDirectory or saveFile results
	Cancelled bool
	Error     string
}

// PickFileOptions configures file picker behavior.
type PickFileOptions struct {
	AllowMultiple bool
	AllowedTypes  []string
	InitialDir    string
	DialogTitle   string
}

// SaveFileOptions configures save file dialog behavior.
type SaveFileOptions struct {
	SuggestedName string
	MimeType      string
	InitialDir    string
	DialogTitle   string
}

// AppDirectory represents standard app directories.
type AppDirectory string

const (
	AppDirectoryDocuments AppDirectory = "documents"
	AppDirectoryCache     AppDirectory = "cache"
	AppDirectoryTemp      AppDirectory = "temp"
	AppDirectorySupport   AppDirectory = "support"
)

// PickFile opens a file picker dialog.
// Results are delivered asynchronously via StorageResults().
func PickFile(opts PickFileOptions) error {
	return storageService.pickFile(opts)
}

// PickDirectory opens a directory picker dialog.
// Results are delivered asynchronously via StorageResults().
func PickDirectory() error {
	return storageService.pickDirectory()
}

// SaveFile saves data to a file chosen by the user.
// Results are delivered asynchronously via StorageResults().
func SaveFile(data []byte, opts SaveFileOptions) error {
	return storageService.saveFile(data, opts)
}

// StorageResults returns a channel that receives storage operation results.
func StorageResults() <-chan StorageResult {
	return storageService.resultChannel()
}

// ReadFile reads the contents of a file.
func ReadFile(pathOrURI string) ([]byte, error) {
	return storageService.readFile(pathOrURI)
}

// WriteFile writes data to a file.
func WriteFile(pathOrURI string, data []byte) error {
	return storageService.writeFile(pathOrURI, data)
}

// DeleteFile deletes a file.
func DeleteFile(pathOrURI string) error {
	return storageService.deleteFile(pathOrURI)
}

// GetFileInfo returns metadata about a file.
func GetFileInfo(pathOrURI string) (*FileInfo, error) {
	return storageService.getFileInfo(pathOrURI)
}

// GetAppDirectory returns the path to a standard app directory.
func GetAppDirectory(dir AppDirectory) (string, error) {
	return storageService.getAppDirectory(dir)
}

type storageServiceState struct {
	channel  *MethodChannel
	results  *EventChannel
	resultCh chan StorageResult
}

func newStorageService() *storageServiceState {
	service := &storageServiceState{
		channel:  NewMethodChannel("drift/storage"),
		results:  NewEventChannel("drift/storage/result"),
		resultCh: make(chan StorageResult, 4),
	}

	service.results.Listen(EventHandler{OnEvent: func(data any) {
		if result, ok := parseStorageResult(data); ok {
			service.resultCh <- result
		}
	}})

	return service
}

func (s *storageServiceState) pickFile(opts PickFileOptions) error {
	_, err := s.channel.Invoke("pickFile", map[string]any{
		"allowMultiple": opts.AllowMultiple,
		"allowedTypes":  opts.AllowedTypes,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
	return err
}

func (s *storageServiceState) pickDirectory() error {
	_, err := s.channel.Invoke("pickDirectory", nil)
	return err
}

func (s *storageServiceState) saveFile(data []byte, opts SaveFileOptions) error {
	_, err := s.channel.Invoke("saveFile", map[string]any{
		"data":          data,
		"suggestedName": opts.SuggestedName,
		"mimeType":      opts.MimeType,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
	return err
}

func (s *storageServiceState) resultChannel() <-chan StorageResult {
	return s.resultCh
}

func (s *storageServiceState) readFile(pathOrURI string) ([]byte, error) {
	result, err := s.channel.Invoke("readFile", map[string]any{
		"path": pathOrURI,
	})
	if err != nil {
		return nil, err
	}
	if m, ok := result.(map[string]any); ok {
		if data, ok := m["data"].([]byte); ok {
			return data, nil
		}
		if data, ok := m["data"].(string); ok {
			return []byte(data), nil
		}
	}
	return nil, nil
}

func (s *storageServiceState) writeFile(pathOrURI string, data []byte) error {
	_, err := s.channel.Invoke("writeFile", map[string]any{
		"path": pathOrURI,
		"data": data,
	})
	return err
}

func (s *storageServiceState) deleteFile(pathOrURI string) error {
	_, err := s.channel.Invoke("deleteFile", map[string]any{
		"path": pathOrURI,
	})
	return err
}

func (s *storageServiceState) getFileInfo(pathOrURI string) (*FileInfo, error) {
	result, err := s.channel.Invoke("getFileInfo", map[string]any{
		"path": pathOrURI,
	})
	if err != nil {
		return nil, err
	}
	if info, ok := parseFileInfo(result); ok {
		return &info, nil
	}
	return nil, nil
}

func (s *storageServiceState) getAppDirectory(dir AppDirectory) (string, error) {
	result, err := s.channel.Invoke("getAppDirectory", map[string]any{
		"directory": string(dir),
	})
	if err != nil {
		return "", err
	}
	if m, ok := result.(map[string]any); ok {
		return parseString(m["path"]), nil
	}
	return "", nil
}

func parseStorageResult(data any) (StorageResult, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return StorageResult{}, false
	}

	result := StorageResult{
		Type:      parseString(m["type"]),
		Path:      parseString(m["path"]),
		Cancelled: parseBool(m["cancelled"]),
		Error:     parseString(m["error"]),
	}

	if files, ok := m["files"].([]any); ok {
		for _, f := range files {
			if fm, ok := f.(map[string]any); ok {
				result.Files = append(result.Files, PickedFile{
					Name:     parseString(fm["name"]),
					Path:     parseString(fm["path"]),
					URI:      parseString(fm["uri"]),
					MimeType: parseString(fm["mimeType"]),
					Size:     parseInt64(fm["size"]),
				})
			}
		}
	}

	return result, true
}

func parseFileInfo(result any) (FileInfo, bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return FileInfo{}, false
	}
	return FileInfo{
		Name:         parseString(m["name"]),
		Path:         parseString(m["path"]),
		Size:         parseInt64(m["size"]),
		MimeType:     parseString(m["mimeType"]),
		IsDirectory:  parseBool(m["isDirectory"]),
		LastModified: parseInt64(m["lastModified"]),
	}, true
}

func parseInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}
