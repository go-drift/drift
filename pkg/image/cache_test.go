package image

import (
	"fmt"
	"image"
	"testing"
)

func makeImage(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w, h))
}

func TestCache_GetMiss(t *testing.T) {
	c := NewCache(CacheOptions{})
	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss on empty cache")
	}
}

func TestCache_PutAndGet(t *testing.T) {
	c := NewCache(CacheOptions{})
	img := makeImage(10, 10)
	c.Put("a", img)

	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected hit")
	}
	if got != img {
		t.Fatal("expected same image instance")
	}
}

func TestCache_PutNilIgnored(t *testing.T) {
	c := NewCache(CacheOptions{})
	c.Put("a", nil)
	if c.Len() != 0 {
		t.Fatal("nil put should be ignored")
	}
}

func TestCache_Replace(t *testing.T) {
	c := NewCache(CacheOptions{})
	img1 := makeImage(10, 10)
	img2 := makeImage(20, 20)
	c.Put("a", img1)
	c.Put("a", img2)

	if c.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", c.Len())
	}
	got, _ := c.Get("a")
	if got != img2 {
		t.Fatal("expected replaced image")
	}
}

func TestCache_EvictsLRUByCount(t *testing.T) {
	c := NewCache(CacheOptions{MaxEntries: 3, MaxBytes: 1 << 30})
	for i := range 5 {
		c.Put(fmt.Sprintf("k%d", i), makeImage(1, 1))
	}
	if c.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", c.Len())
	}
	// k0 and k1 should be evicted (oldest)
	if _, ok := c.Get("k0"); ok {
		t.Fatal("k0 should have been evicted")
	}
	if _, ok := c.Get("k1"); ok {
		t.Fatal("k1 should have been evicted")
	}
	// k2, k3, k4 should remain
	for _, k := range []string{"k2", "k3", "k4"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("%s should still be cached", k)
		}
	}
}

func TestCache_EvictsLRUByBytes(t *testing.T) {
	// Each 10x10 image = 400 bytes. MaxBytes = 1000 allows 2 images.
	c := NewCache(CacheOptions{MaxEntries: 100, MaxBytes: 1000})
	c.Put("a", makeImage(10, 10))
	c.Put("b", makeImage(10, 10))
	c.Put("c", makeImage(10, 10))

	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should have been evicted")
	}
}

func TestCache_GetPromotesEntry(t *testing.T) {
	c := NewCache(CacheOptions{MaxEntries: 2, MaxBytes: 1 << 30})
	c.Put("a", makeImage(1, 1))
	c.Put("b", makeImage(1, 1))

	// Access "a" to promote it; adding "c" should evict "b" instead.
	c.Get("a")
	c.Put("c", makeImage(1, 1))

	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should still be cached after promotion")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
}

func TestCache_Remove(t *testing.T) {
	c := NewCache(CacheOptions{})
	c.Put("a", makeImage(10, 10))
	c.Remove("a")
	if c.Len() != 0 {
		t.Fatal("expected empty cache after remove")
	}
	if c.Bytes() != 0 {
		t.Fatal("expected 0 bytes after remove")
	}
}

func TestCache_RemoveMissing(t *testing.T) {
	c := NewCache(CacheOptions{})
	c.Remove("nonexistent") // should not panic
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(CacheOptions{})
	c.Put("a", makeImage(10, 10))
	c.Put("b", makeImage(10, 10))
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("expected empty cache after clear")
	}
	if c.Bytes() != 0 {
		t.Fatal("expected 0 bytes after clear")
	}
}

func TestCache_BytesTracking(t *testing.T) {
	c := NewCache(CacheOptions{})
	c.Put("a", makeImage(10, 10)) // 400 bytes
	c.Put("b", makeImage(5, 5))   // 100 bytes
	want := int64(400 + 100)
	if got := c.Bytes(); got != want {
		t.Fatalf("expected %d bytes, got %d", want, got)
	}
}

func TestEstimateImageBytes(t *testing.T) {
	img := makeImage(100, 50)
	got := estimateImageBytes(img)
	want := int64(100 * 50 * 4)
	if got != want {
		t.Fatalf("expected %d, got %d", want, got)
	}
}
