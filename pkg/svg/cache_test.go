package svg

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestIconCache_Hit(t *testing.T) {
	cache := NewIconCache()
	icon := &Icon{}
	callCount := 0

	loader := func() (*Icon, error) {
		callCount++
		return icon, nil
	}

	got1, err := cache.Get("key", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got2, err := cache.Get("key", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got1 != got2 {
		t.Error("cache hit should return the same icon pointer")
	}
	if callCount != 1 {
		t.Errorf("loader should be called once, called %d times", callCount)
	}
}

func TestIconCache_Miss(t *testing.T) {
	cache := NewIconCache()
	icon := &Icon{}
	called := false

	got, err := cache.Get("key", func() (*Icon, error) {
		called = true
		return icon, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("loader should be called on cache miss")
	}
	if got != icon {
		t.Error("should return the icon from loader")
	}
}

func TestIconCache_NilCache(t *testing.T) {
	var cache *IconCache
	icon := &Icon{}

	got, err := cache.Get("key", func() (*Icon, error) {
		return icon, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != icon {
		t.Error("nil cache should pass through to loader")
	}
}

func TestIconCache_NilLoader(t *testing.T) {
	cache := NewIconCache()

	_, err := cache.Get("key", nil)
	if err == nil {
		t.Error("nil loader should return an error")
	}
}

func TestIconCache_LoaderError(t *testing.T) {
	cache := NewIconCache()
	loadErr := errors.New("load failed")
	callCount := 0

	loader := func() (*Icon, error) {
		callCount++
		return nil, loadErr
	}

	_, err := cache.Get("key", loader)
	if !errors.Is(err, loadErr) {
		t.Errorf("expected load error, got %v", err)
	}

	// Error should not be cached; loader should be called again
	_, _ = cache.Get("key", loader)
	if callCount != 2 {
		t.Errorf("loader should be called again after error, called %d times", callCount)
	}
}

func TestIconCache_Concurrent(t *testing.T) {
	cache := NewIconCache()
	icon := &Icon{}
	var calls atomic.Int64

	loader := func() (*Icon, error) {
		calls.Add(1)
		return icon, nil
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := cache.Get("key", loader)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != icon {
				t.Errorf("got different icon pointer")
			}
		}()
	}
	wg.Wait()
}
