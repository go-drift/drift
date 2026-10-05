package platform

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrStorageBusy is returned when a storage picker is already open.
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
//
// Every method except GetAppDirectory waits for the platform (a picker, or
// file I/O off the main thread), so call it from a goroutine: from the UI
// thread it fails with [ErrBlocksUIThread]. File contents cross the
// channel as base64.
type StorageService struct {
	channel *MethodChannel
}

// Storage is the singleton storage service.
var Storage = &StorageService{channel: NewMethodChannel("drift/storage")}

// PickFile opens the system file picker and returns the chosen files, or
// none if the user canceled. Returns [ErrStorageBusy] if a picker is open.
func (s *StorageService) PickFile(ctx context.Context, opts PickFileOptions) ([]PickedFile, error) {
	result, err := s.channel.Invoke(ctx, "pickFile", map[string]any{
		"allowMultiple": opts.AllowMultiple,
		"allowedTypes":  opts.AllowedTypes,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
	if err != nil {
		return nil, err
	}
	return parsePickedFiles(result)
}

// PickDirectory opens the system directory picker and returns the chosen
// directory's path (a content URI on Android), or "" if the user canceled.
// Returns [ErrStorageBusy] if a picker is open.
func (s *StorageService) PickDirectory(ctx context.Context) (string, error) {
	return s.invokePath(ctx, "pickDirectory", nil)
}

// SaveFile asks the user where to save data and writes it there, returning
// the saved file's path (a content URI on Android), or "" if the user
// canceled. Returns [ErrStorageBusy] if a picker is open.
func (s *StorageService) SaveFile(ctx context.Context, data []byte, opts SaveFileOptions) (string, error) {
	return s.invokePath(ctx, "saveFile", map[string]any{
		"data":          data,
		"suggestedName": opts.SuggestedName,
		"mimeType":      opts.MimeType,
		"initialDir":    opts.InitialDir,
		"dialogTitle":   opts.DialogTitle,
	})
}

// ReadFile reads the contents of a file path or content URI.
func (s *StorageService) ReadFile(ctx context.Context, pathOrURI string) ([]byte, error) {
	result, err := s.channel.Invoke(ctx, "readFile", map[string]any{"path": pathOrURI})
	if err != nil {
		return nil, err
	}
	m, _ := result.(map[string]any)
	encoded, ok := m["data"].(string)
	if !ok {
		return nil, fmt.Errorf("storage readFile: want base64 data, got %v", result)
	}
	return base64.StdEncoding.DecodeString(encoded)
}

// WriteFile writes data to a file path or content URI, replacing its
// contents.
func (s *StorageService) WriteFile(ctx context.Context, pathOrURI string, data []byte) error {
	_, err := s.channel.Invoke(ctx, "writeFile", map[string]any{"path": pathOrURI, "data": data})
	return err
}

// DeleteFile deletes a file path or content URI.
func (s *StorageService) DeleteFile(ctx context.Context, pathOrURI string) error {
	_, err := s.channel.Invoke(ctx, "deleteFile", map[string]any{"path": pathOrURI})
	return err
}

// GetFileInfo returns metadata about a file path or content URI.
func (s *StorageService) GetFileInfo(ctx context.Context, pathOrURI string) (FileInfo, error) {
	result, err := s.channel.Invoke(ctx, "getFileInfo", map[string]any{"path": pathOrURI})
	if err != nil {
		return FileInfo{}, err
	}
	info, ok := parseFileInfo(result)
	if !ok {
		return FileInfo{}, fmt.Errorf("storage getFileInfo: want a map, got %v", result)
	}
	return info, nil
}

// GetAppDirectory returns the path to a standard app directory. It does
// not wait, so the UI thread may call it.
func (s *StorageService) GetAppDirectory(dir AppDirectory) (string, error) {
	result, err := s.channel.Invoke(context.Background(), "getAppDirectory", map[string]any{
		"directory": string(dir),
	})
	if err != nil {
		return "", err
	}
	m, _ := result.(map[string]any)
	path := parseString(m["path"])
	if path == "" {
		return "", fmt.Errorf("storage getAppDirectory: no %s directory", dir)
	}
	return path, nil
}

// invokePath calls a picker that replies with {"path": ...}, or nil when
// the user canceled.
func (s *StorageService) invokePath(ctx context.Context, method string, args map[string]any) (string, error) {
	result, err := s.channel.Invoke(ctx, method, args)
	if err != nil || result == nil {
		return "", err
	}
	m, ok := result.(map[string]any)
	if !ok {
		return "", fmt.Errorf("storage %s: want a map, got %v", method, result)
	}
	return parseString(m["path"]), nil
}

// parsePickedFiles parses pickFile's reply: {"files": [...]}, or nil when
// the user canceled.
func parsePickedFiles(result any) ([]PickedFile, error) {
	if result == nil {
		return nil, nil
	}
	m, _ := result.(map[string]any)
	list, ok := m["files"].([]any)
	if !ok {
		return nil, fmt.Errorf("storage pickFile: want a file list, got %v", result)
	}
	files := make([]PickedFile, 0, len(list))
	for _, f := range list {
		fm, ok := f.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("storage pickFile: want a file, got %v", f)
		}
		files = append(files, PickedFile{
			Name:     parseString(fm["name"]),
			Path:     parseString(fm["path"]),
			URI:      parseString(fm["uri"]),
			MimeType: parseString(fm["mimeType"]),
			Size:     parseInt64(fm["size"]),
		})
	}
	return files, nil
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
