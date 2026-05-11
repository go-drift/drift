package platform

import (
	"context"
	"errors"
	"sync"

	drifterrors "github.com/go-drift/drift/pkg/errors"
)

// ErrStorageBusy is returned when a storage picker operation is already in progress.
var ErrStorageBusy = errors.New("storage picker operation already in progress")

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
	requestID string
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

// StorageService provides file picking, saving, and file system access.
type StorageService struct {
	state *storageServiceState
	mu    sync.Mutex // serializes picker operations
}

// Storage is the singleton storage service.
var Storage *StorageService

func init() {
	Storage = &StorageService{
		state: newStorageService(),
	}
}

type storageServiceState struct {
	channel *MethodChannel
	results *EventChannel
}

func newStorageService() *storageServiceState {
	return &storageServiceState{
		channel: NewMethodChannel("drift/storage"),
		results: NewEventChannel("drift/storage/result"),
	}
}

// PickFile opens a file picker dialog and blocks until the user selects files or cancels.
// Returns ErrStorageBusy if another picker operation is already in progress.
func (s *StorageService) PickFile(ctx context.Context, opts PickFileOptions) (StorageResult, error) {
	return s.invokePicker(ctx, "pickFile", map[string]any{
		"allowMultiple": opts.AllowMultiple,
		"allowedTypes":  opts.AllowedTypes,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
}

// PickDirectory opens a directory picker dialog and blocks until the user selects a directory or cancels.
// Returns ErrStorageBusy if another picker operation is already in progress.
func (s *StorageService) PickDirectory(ctx context.Context) (StorageResult, error) {
	return s.invokePicker(ctx, "pickDirectory", nil)
}

// SaveFile saves data to a file chosen by the user and blocks until complete.
// Returns ErrStorageBusy if another picker operation is already in progress.
func (s *StorageService) SaveFile(ctx context.Context, data []byte, opts SaveFileOptions) (StorageResult, error) {
	return s.invokePicker(ctx, "saveFile", map[string]any{
		"data":          data,
		"suggestedName": opts.SuggestedName,
		"mimeType":      opts.MimeType,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
}

// invokePicker serializes picker operations, subscribes to the result event
// channel filtered by a generated request ID, invokes the native method,
// and blocks until a matching result arrives or the context is canceled.
func (s *StorageService) invokePicker(ctx context.Context, method string, args map[string]any) (StorageResult, error) {
	if !s.mu.TryLock() {
		return StorageResult{}, ErrStorageBusy
	}
	defer s.mu.Unlock()

	requestID := generateRequestID()

	resultChan := make(chan StorageResult, 1)
	errChan := make(chan error, 1)
	sub := s.state.results.Listen(EventHandler{
		OnEvent: func(data any) {
			result, err := parseStorageResultWithID(data)
			if err != nil {
				drifterrors.Report(&drifterrors.DriftError{
					Op:      "storage.parse",
					Kind:    drifterrors.KindParsing,
					Channel: "drift/storage/result",
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
			select {
			case errChan <- err:
			default:
			}
		},
	})
	defer sub.Cancel()

	if args == nil {
		args = make(map[string]any)
	}
	args["requestId"] = requestID

	_, err := s.state.channel.Invoke(ctx, method, args)
	if err != nil {
		return StorageResult{}, err
	}

	select {
	case result := <-resultChan:
		if result.Error != "" {
			return result, errors.New(result.Error)
		}
		return result, nil
	case err := <-errChan:
		return StorageResult{}, err
	case <-ctx.Done():
		return StorageResult{}, ctx.Err()
	}
}

// ReadFile reads the contents of a file.
func (s *StorageService) ReadFile(pathOrURI string) ([]byte, error) {
	result, err := s.state.channel.Invoke(context.Background(), "readFile", map[string]any{
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

// WriteFile writes data to a file.
func (s *StorageService) WriteFile(pathOrURI string, data []byte) error {
	_, err := s.state.channel.Invoke(context.Background(), "writeFile", map[string]any{
		"path": pathOrURI,
		"data": data,
	})
	return err
}

// DeleteFile deletes a file.
func (s *StorageService) DeleteFile(pathOrURI string) error {
	_, err := s.state.channel.Invoke(context.Background(), "deleteFile", map[string]any{
		"path": pathOrURI,
	})
	return err
}

// GetFileInfo returns metadata about a file.
func (s *StorageService) GetFileInfo(pathOrURI string) (*FileInfo, error) {
	result, err := s.state.channel.Invoke(context.Background(), "getFileInfo", map[string]any{
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

// GetAppDirectory returns the path to a standard app directory.
func (s *StorageService) GetAppDirectory(dir AppDirectory) (string, error) {
	result, err := s.state.channel.Invoke(context.Background(), "getAppDirectory", map[string]any{
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

func parseStorageResultWithID(data any) (StorageResult, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return StorageResult{}, &drifterrors.ParseError{
			Channel:  "drift/storage/result",
			DataType: "StorageResult",
			Got:      data,
		}
	}

	requestID := parseString(m["requestId"])
	if requestID == "" {
		return StorageResult{}, &drifterrors.ParseError{
			Channel:  "drift/storage/result",
			DataType: "StorageResult",
			Got:      data,
		}
	}

	result := StorageResult{
		requestID: requestID,
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

	return result, nil
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
