package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// testPNG returns a valid PNG as raw bytes.
func testPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestFetchOnce_SuccessFromNetwork(t *testing.T) {
	pngData := testPNG(4, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngData)
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/img.png"
	key := requestKey(url, nil)
	result := l.fetchOnce(context.Background(), url, key, LoadOptions{})
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Image == nil {
		t.Fatal("expected non-nil image")
	}
	b := result.Image.Bounds()
	if b.Dx() != 4 || b.Dy() != 4 {
		t.Fatalf("expected 4x4, got %dx%d", b.Dx(), b.Dy())
	}

	// Should now be in memory cache.
	if _, ok := l.memCache.Get(key); !ok {
		t.Fatal("expected image in memory cache")
	}
}

func TestFetchOnce_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/missing.png"
	result := l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})
	if result.Err == nil {
		t.Fatal("expected error for 404")
	}
	var httpErr *HTTPError
	if !errors.As(result.Err, &httpErr) {
		t.Fatalf("expected HTTPError, got %T: %v", result.Err, result.Err)
	}
	if httpErr.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", httpErr.StatusCode)
	}
}

func TestFetchOnce_InvalidBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not an image"))
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/bad"
	result := l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})
	if result.Err == nil {
		t.Fatal("expected decode error")
	}
}

func TestFetchOnce_CustomHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		w.Write(testPNG(1, 1))
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/img.png"
	l.fetchOnce(context.Background(), url, requestKey(url, map[string]string{"Authorization": "Bearer tok123"}), LoadOptions{
		Headers: map[string]string{"Authorization": "Bearer tok123"},
	})

	if gotAuth != "Bearer tok123" {
		t.Fatalf("expected auth header, got %q", gotAuth)
	}
}

func TestFetchOnce_PopulatesMemoryCache(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write(testPNG(1, 1))
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/img.png"

	// First fetch: hits network.
	r1 := l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})
	if r1.Err != nil {
		t.Fatal(r1.Err)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected 1 request, got %d", requests.Load())
	}

	// Second fetch: should come from memory cache (fetchOnce checks disk, not memory,
	// but the caller Load() checks memory first). Let's verify it's in cache.
	if _, ok := l.memCache.Get(requestKey(url, nil)); !ok {
		t.Fatal("expected cache hit")
	}
}

func TestFetchOnce_DiskCacheHit(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write(testPNG(2, 2))
	}))
	t.Cleanup(srv.Close)

	dc, err := NewDiskCache(filepath.Join(t.TempDir(), "cache"), 0)
	if err != nil {
		t.Fatal(err)
	}

	l := &Loader{
		memCache:  NewCache(CacheOptions{}),
		diskCache: dc,
		client:    srv.Client(),
		inflight:  make(map[string]*inflightRequest),
		variants:  make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/img.png"

	// First fetch: network + writes to disk.
	r1 := l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})
	if r1.Err != nil {
		t.Fatal(r1.Err)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected 1 request, got %d", requests.Load())
	}

	// Clear memory cache to force disk cache path.
	l.memCache.Clear()

	// Second fetch: should come from disk cache.
	r2 := l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})
	if r2.Err != nil {
		t.Fatal(r2.Err)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected still 1 request (disk hit), got %d", requests.Load())
	}
}

func TestFetchOnce_CanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testPNG(1, 1))
	}))
	t.Cleanup(srv.Close)

	l := &Loader{
		memCache: NewCache(CacheOptions{}),
		client:   srv.Client(),
		inflight: make(map[string]*inflightRequest),
		variants: make(map[string]map[string]struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	url := srv.URL + "/img.png"
	result := l.fetchOnce(ctx, url, requestKey(url, nil), LoadOptions{})
	if result.Err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestEvict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(testPNG(1, 1))
	}))
	t.Cleanup(srv.Close)

	dc, _ := NewDiskCache(filepath.Join(t.TempDir(), "cache"), 0)
	l := &Loader{
		memCache:  NewCache(CacheOptions{}),
		diskCache: dc,
		client:    srv.Client(),
		inflight:  make(map[string]*inflightRequest),
		variants:  make(map[string]map[string]struct{}),
	}

	url := srv.URL + "/img.png"
	l.fetchOnce(context.Background(), url, requestKey(url, nil), LoadOptions{})

	l.Evict(url)

	if _, ok := l.memCache.Get(requestKey(url, nil)); ok {
		t.Fatal("expected memory cache miss after evict")
	}
	if _, ok := dc.Get(requestKey(url, nil)); ok {
		t.Fatal("expected disk cache miss after evict")
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{"500", &HTTPError{StatusCode: 500}, true},
		{"503", &HTTPError{StatusCode: 503}, true},
		{"404", &HTTPError{StatusCode: 404}, false},
		{"canceled", context.Canceled, false},
		{"network", &networkError{err: fmt.Errorf("connection refused")}, true},
		{"decode", fmt.Errorf("image: unknown format"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.transient {
				t.Fatalf("isTransient(%v) = %v, want %v", tt.err, got, tt.transient)
			}
		})
	}
}

func TestRequestKey_IncludesHeaders(t *testing.T) {
	url := "https://example.com/image.png"
	k1 := requestKey(url, map[string]string{"Authorization": "Bearer one"})
	k2 := requestKey(url, map[string]string{"Authorization": "Bearer two"})
	if k1 == k2 {
		t.Fatal("expected different keys for different header values")
	}
}

func TestRequestKey_NormalizesHeaderOrderAndCase(t *testing.T) {
	url := "https://example.com/image.png"
	k1 := requestKey(url, map[string]string{
		"authorization": "Bearer one",
		"X-Tenant":      "acme",
	})
	k2 := requestKey(url, map[string]string{
		"X-Tenant":      "acme",
		"Authorization": "Bearer one",
	})
	if k1 != k2 {
		t.Fatal("expected stable request key regardless of header order or case")
	}
}

func TestLoad_DoesNotShareCacheAcrossDifferentHeaders(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.Header.Get("Authorization") {
		case "Bearer one":
			w.Write(testPNG(1, 1))
		case "Bearer two":
			w.Write(testPNG(2, 2))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)

	l := NewLoader(LoaderOptions{
		MemoryCache: NewCache(CacheOptions{}),
		Client:      srv.Client(),
	})

	type result struct {
		img image.Image
		err error
	}
	load := func(token string) result {
		ch := make(chan result, 1)
		cancel := l.Load(srv.URL+"/img.png", LoadOptions{
			Headers: map[string]string{"Authorization": token},
		}, func(res LoadResult) {
			ch <- result{img: res.Image, err: res.Err}
		})
		defer cancel()
		return <-ch
	}

	r1 := load("Bearer one")
	if r1.err != nil {
		t.Fatalf("first load failed: %v", r1.err)
	}
	r2 := load("Bearer two")
	if r2.err != nil {
		t.Fatalf("second load failed: %v", r2.err)
	}

	if requests.Load() != 2 {
		t.Fatalf("expected 2 network requests, got %d", requests.Load())
	}
	if r1.img.Bounds().Dx() == r2.img.Bounds().Dx() && r1.img.Bounds().Dy() == r2.img.Bounds().Dy() {
		t.Fatal("expected different images for different authorization headers")
	}
}

func TestClearCache_ReturnsDiskCacheError(t *testing.T) {
	l := NewLoader(LoaderOptions{
		MemoryCache: NewCache(CacheOptions{}),
		DiskCache:   &DiskCache{dir: filepath.Join(t.TempDir(), "missing")},
	})

	if err := l.ClearCache(); err == nil {
		t.Fatal("expected disk cache clear error")
	}
	if l.memCache.Len() != 0 {
		t.Fatal("expected memory cache to be cleared even when disk cache clear fails")
	}
}
