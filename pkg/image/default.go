package image

import (
	"net/http"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/go-drift/drift/pkg/platform"
)

var defaultLoader atomic.Pointer[Loader]

// DefaultLoader returns the process-wide shared Loader. It uses a 100-entry /
// 100 MB in-memory cache and a 500 MB disk cache under the platform's cache
// directory. The loader is created lazily on first call.
func DefaultLoader() *Loader {
	if l := defaultLoader.Load(); l != nil {
		return l
	}
	// The directory lookup is a platform call, which from a goroutine waits
	// for the UI thread; no lock is held across it, so a UI thread entering
	// here meanwhile does not wait on that goroutine. Callers racing the
	// first call each build a loader and all return the first one stored.
	var diskCache *DiskCache
	if dir, err := platform.Storage.GetAppDirectory(platform.AppDirectoryCache); err == nil {
		diskCache, _ = NewDiskCache(filepath.Join(dir, "drift_images"), 500<<20)
	}
	defaultLoader.CompareAndSwap(nil, NewLoader(LoaderOptions{
		MemoryCache: NewCache(CacheOptions{}),
		DiskCache:   diskCache,
		Client:      &http.Client{Timeout: 30 * time.Second},
	}))
	return defaultLoader.Load()
}
