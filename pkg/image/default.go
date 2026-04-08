package image

import (
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-drift/drift/pkg/platform"
)

var (
	defaultLoader *Loader
	defaultOnce   sync.Once
)

// DefaultLoader returns the process-wide shared Loader. It uses a 100-entry /
// 100 MB in-memory cache and a 500 MB disk cache under the platform's cache
// directory. The loader is created lazily on first call.
func DefaultLoader() *Loader {
	defaultOnce.Do(func() {
		var diskCache *DiskCache
		if dir, err := platform.Storage.GetAppDirectory(platform.AppDirectoryCache); err == nil {
			diskCache, _ = NewDiskCache(filepath.Join(dir, "drift_images"), 500<<20)
		}
		defaultLoader = NewLoader(LoaderOptions{
			MemoryCache: NewCache(CacheOptions{}),
			DiskCache:   diskCache,
			Client:      &http.Client{Timeout: 30 * time.Second},
		})
	})
	return defaultLoader
}
