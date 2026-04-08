package image

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// DiskCache persists raw image bytes to the filesystem. It stores undecoded
// response bodies so that format detection (image.Decode) works on read.
//
// Keys are hashed with SHA-256 to produce safe filenames. When the cache
// exceeds MaxBytes, the oldest files by modification time are evicted.
type DiskCache struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
}

// NewDiskCache creates a disk cache rooted at dir. The directory is created
// if it does not exist. maxBytes controls the total size limit; pass 0 for
// the default of 500 MB.
func NewDiskCache(dir string, maxBytes int64) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		maxBytes = 500 << 20 // 500 MB
	}
	return &DiskCache{dir: dir, maxBytes: maxBytes}, nil
}

// Get returns a reader for the cached data and true on hit, or nil and false
// on miss. The caller must close the returned reader.
func (d *DiskCache) Get(key string) (io.ReadCloser, bool) {
	path := d.path(key)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	return f, true
}

// Put writes data to the cache under key, then evicts old entries if the
// total cache size exceeds MaxBytes.
func (d *DiskCache) Put(key string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := d.path(key)
	// Write to a temp file then rename for atomicity. A concurrent Get on
	// the same key will either see the old file or the new one, never a
	// partially-written file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	d.evictLocked()
	return nil
}

// Remove deletes a single entry from the cache.
func (d *DiskCache) Remove(key string) {
	os.Remove(d.path(key))
}

// Clear removes all cached files.
func (d *DiskCache) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(d.dir, e.Name()))
	}
	return nil
}

func (d *DiskCache) path(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(d.dir, hex.EncodeToString(h[:]))
}

// evictLocked removes the oldest files until total size is within maxBytes.
// Must be called with d.mu held.
func (d *DiskCache) evictLocked() {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}

	type fileEntry struct {
		path    string
		size    int64
		modTime int64 // unix nanos
	}

	files := make([]fileEntry, 0, len(entries))
	var totalSize int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		totalSize += size
		files = append(files, fileEntry{
			path:    filepath.Join(d.dir, e.Name()),
			size:    size,
			modTime: info.ModTime().UnixNano(),
		})
	}

	if totalSize <= d.maxBytes {
		return
	}

	// Sort oldest first.
	slices.SortFunc(files, func(a, b fileEntry) int {
		switch {
		case a.modTime < b.modTime:
			return -1
		case a.modTime > b.modTime:
			return 1
		default:
			return 0
		}
	})

	for _, f := range files {
		if totalSize <= d.maxBytes {
			break
		}
		if os.Remove(f.path) == nil {
			totalSize -= f.size
		}
	}
}
