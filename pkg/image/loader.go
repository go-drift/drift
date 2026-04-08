package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	// Register additional image decoders beyond the stdlib defaults.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
)

// LoadResult is the outcome of an image load operation.
type LoadResult struct {
	// Image is the decoded image on success, nil on failure.
	Image image.Image
	// Err is the error on failure, nil on success.
	Err error
}

// LoadOptions configures a single image load request.
type LoadOptions struct {
	// Headers are added to the HTTP request (e.g., authorization tokens).
	Headers map[string]string
}

// LoaderOptions configures a Loader.
type LoaderOptions struct {
	// MemoryCache is the in-memory LRU cache. Required.
	MemoryCache *Cache
	// DiskCache is the optional filesystem cache. Nil disables disk caching.
	DiskCache *DiskCache
	// Client is the HTTP client. Defaults to a client with 30s timeout.
	Client *http.Client
}

// Loader coordinates fetching, decoding, and caching of network images.
// It deduplicates concurrent requests for the same URL.
// All methods are safe for concurrent use.
type Loader struct {
	memCache  *Cache
	diskCache *DiskCache
	client    *http.Client
	inflight  map[string]*inflightRequest
	variants  map[string]map[string]struct{}
	mu        sync.Mutex
}

type inflightRequest struct {
	listeners []*loadListener
	cancel    context.CancelFunc
}

type loadListener struct {
	callback func(LoadResult)
	canceled bool
}

// NewLoader creates a Loader with the given options.
func NewLoader(opts LoaderOptions) *Loader {
	if opts.MemoryCache == nil {
		opts.MemoryCache = NewCache(CacheOptions{})
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Loader{
		memCache:  opts.MemoryCache,
		diskCache: opts.DiskCache,
		client:    client,
		inflight:  make(map[string]*inflightRequest),
		variants:  make(map[string]map[string]struct{}),
	}
}

// Load fetches, decodes, and caches an image from url. The callback is invoked
// from a background goroutine with either the decoded image or an error.
// Callers that need to update UI state should use drift.Dispatch inside the
// callback to marshal onto the UI thread.
//
// If the image is already in the memory cache, the callback is called
// synchronously before Load returns and cancel is a no-op.
//
// The returned function cancels this listener. If all listeners for a URL
// cancel, the underlying HTTP request is also canceled.
func (l *Loader) Load(url string, opts LoadOptions, callback func(LoadResult)) (cancel func()) {
	key := requestKey(url, opts.Headers)

	// Memory cache hit: call synchronously.
	if img, ok := l.memCache.Get(key); ok {
		l.registerVariant(url, key)
		callback(LoadResult{Image: img})
		return func() {}
	}

	listener := &loadListener{callback: callback}

	l.mu.Lock()
	if req, ok := l.inflight[key]; ok {
		// Join existing in-flight request.
		req.listeners = append(req.listeners, listener)
		l.mu.Unlock()
		return l.cancelFunc(key, listener)
	}

	// Start new request.
	ctx, cancelCtx := context.WithCancel(context.Background())
	req := &inflightRequest{
		listeners: []*loadListener{listener},
		cancel:    cancelCtx,
	}
	l.inflight[key] = req
	l.mu.Unlock()

	go l.fetch(ctx, url, key, opts)

	return l.cancelFunc(key, listener)
}

// Evict removes a URL from both memory and disk caches, including all
// header-variant entries that were cached for different request headers.
func (l *Loader) Evict(url string) {
	keys := []string{requestKey(url, nil)}

	l.mu.Lock()
	if variants := l.variants[url]; len(variants) > 0 {
		keys = keys[:0]
		for key := range variants {
			keys = append(keys, key)
		}
		delete(l.variants, url)
	}
	l.mu.Unlock()

	for _, key := range keys {
		l.memCache.Remove(key)
		if l.diskCache != nil {
			l.diskCache.Remove(key)
		}
	}
}

// ClearCache clears both memory and disk caches.
// Memory cache clearing is always attempted; disk cache errors are returned.
func (l *Loader) ClearCache() error {
	l.memCache.Clear()
	l.mu.Lock()
	l.variants = make(map[string]map[string]struct{})
	l.mu.Unlock()
	if l.diskCache != nil {
		return l.diskCache.Clear()
	}
	return nil
}

func (l *Loader) cancelFunc(key string, listener *loadListener) func() {
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()

		listener.canceled = true

		req, ok := l.inflight[key]
		if !ok {
			return
		}

		// If all listeners have canceled, cancel the HTTP request.
		allCanceled := true
		for _, ln := range req.listeners {
			if !ln.canceled {
				allCanceled = false
				break
			}
		}
		if allCanceled {
			req.cancel()
			delete(l.inflight, key)
		}
	}
}

func (l *Loader) fetch(ctx context.Context, url, key string, opts LoadOptions) {
	result := l.fetchOnce(ctx, url, key, opts)

	// Retry once on transient errors.
	if result.Err != nil && isTransient(result.Err) && ctx.Err() == nil {
		result = l.fetchOnce(ctx, url, key, opts)
	}

	l.deliver(key, result)
}

func (l *Loader) fetchOnce(ctx context.Context, url, key string, opts LoadOptions) LoadResult {
	// Check disk cache before network.
	if l.diskCache != nil {
		if rc, ok := l.diskCache.Get(key); ok {
			img, _, err := image.Decode(rc)
			rc.Close()
			if err == nil {
				l.memCache.Put(key, img)
				l.registerVariant(url, key)
				return LoadResult{Image: img}
			}
			// Corrupt cache entry: remove and fall through to network.
			l.diskCache.Remove(key)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LoadResult{Err: fmt.Errorf("image: invalid URL %q: %w", url, err)}
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return LoadResult{Err: &networkError{err: fmt.Errorf("image: fetch %q: %w", url, err)}}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LoadResult{Err: &HTTPError{URL: url, StatusCode: resp.StatusCode}}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LoadResult{Err: &networkError{err: fmt.Errorf("image: read %q: %w", url, err)}}
	}

	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return LoadResult{Err: fmt.Errorf("image: decode %q: %w", url, err)}
	}

	// Disk cache writes are best-effort; a cache write failure should not fail
	// an otherwise successful image load.
	if l.diskCache != nil {
		_ = l.diskCache.Put(key, body)
	}

	l.memCache.Put(key, img)
	l.registerVariant(url, key)
	return LoadResult{Image: img}
}

func (l *Loader) deliver(key string, result LoadResult) {
	l.mu.Lock()
	req, ok := l.inflight[key]
	if ok {
		delete(l.inflight, key)
	}
	l.mu.Unlock()

	if !ok {
		return
	}

	req.cancel() // release context resources

	for _, ln := range req.listeners {
		if !ln.canceled {
			ln.callback(result)
		}
	}
}

func (l *Loader) registerVariant(url, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	variants := l.variants[url]
	if variants == nil {
		variants = make(map[string]struct{})
		l.variants[url] = variants
	}
	variants[key] = struct{}{}
}

func requestKey(url string, headers map[string]string) string {
	if len(headers) == 0 {
		return url
	}

	keys := make([]string, 0, len(headers))
	normalized := make(map[string]string, len(headers))
	for k, v := range headers {
		name := http.CanonicalHeaderKey(k)
		keys = append(keys, name)
		normalized[name] = v
	}
	slices.Sort(keys)

	var b strings.Builder
	b.Grow(len(url) + len(keys)*32)
	b.WriteString(url)
	for _, key := range keys {
		b.WriteByte('\n')
		b.WriteString(key)
		b.WriteByte(':')
		b.WriteString(normalized[key])
	}
	return b.String()
}

// HTTPError represents a non-2xx HTTP response.
type HTTPError struct {
	URL        string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("image: HTTP %d for %q", e.StatusCode, e.URL)
}

// networkError wraps errors from HTTP transport (timeout, connection refused, etc.)
// to distinguish them from decode or URL errors during retry decisions.
type networkError struct {
	err error
}

func (e *networkError) Error() string { return e.err.Error() }
func (e *networkError) Unwrap() error { return e.err }

// isTransient returns true for errors that may succeed on retry:
// server errors (5xx) and network transport failures.
func isTransient(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}
	var netErr *networkError
	return errors.As(err, &netErr)
}
