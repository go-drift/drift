package image

import (
	"container/list"
	"image"
	"sync"
)

// CacheOptions configures an in-memory image cache.
type CacheOptions struct {
	// MaxEntries is the maximum number of images to cache. Default: 100.
	MaxEntries int
	// MaxBytes is the maximum total decoded size in bytes. Default: 100 MB.
	MaxBytes int64
}

func (o CacheOptions) withDefaults() CacheOptions {
	if o.MaxEntries <= 0 {
		o.MaxEntries = 100
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 100 << 20 // 100 MB
	}
	return o
}

// Cache is a thread-safe in-memory LRU cache for decoded images.
type Cache struct {
	mu         sync.Mutex
	entries    map[string]*list.Element
	lru        *list.List // front = most recently used
	maxEntries int
	maxBytes   int64
	usedBytes  int64
}

type cacheEntry struct {
	key       string
	image     image.Image
	sizeBytes int64
}

// NewCache creates an in-memory LRU cache with the given options.
func NewCache(opts CacheOptions) *Cache {
	opts = opts.withDefaults()
	return &Cache{
		entries:    make(map[string]*list.Element),
		lru:        list.New(),
		maxEntries: opts.MaxEntries,
		maxBytes:   opts.MaxBytes,
	}
}

// Get retrieves an image from the cache. Returns the image and true on hit,
// or nil and false on miss. A hit promotes the entry to most-recently-used.
func (c *Cache) Get(key string) (image.Image, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(elem)
	return elem.Value.(*cacheEntry).image, true
}

// Put adds an image to the cache, evicting least-recently-used entries as needed.
// If the key already exists, the entry is replaced and promoted.
func (c *Cache) Put(key string, img image.Image) {
	if img == nil {
		return
	}
	size := estimateImageBytes(img)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Replace existing entry.
	if elem, ok := c.entries[key]; ok {
		old := elem.Value.(*cacheEntry)
		c.usedBytes -= old.sizeBytes
		old.image = img
		old.sizeBytes = size
		c.usedBytes += size
		c.lru.MoveToFront(elem)
		c.evictLocked()
		return
	}

	entry := &cacheEntry{key: key, image: img, sizeBytes: size}
	elem := c.lru.PushFront(entry)
	c.entries[key] = elem
	c.usedBytes += size
	c.evictLocked()
}

// Remove removes a single entry from the cache.
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*list.Element)
	c.lru.Init()
	c.usedBytes = 0
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Bytes returns the total estimated decoded size of cached images.
func (c *Cache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usedBytes
}

func (c *Cache) removeLocked(key string) {
	elem, ok := c.entries[key]
	if !ok {
		return
	}
	entry := elem.Value.(*cacheEntry)
	c.usedBytes -= entry.sizeBytes
	c.lru.Remove(elem)
	delete(c.entries, key)
}

// evictLocked removes LRU entries until both maxEntries and maxBytes are satisfied.
// Must be called with c.mu held.
func (c *Cache) evictLocked() {
	for c.lru.Len() > c.maxEntries || (c.usedBytes > c.maxBytes && c.lru.Len() > 0) {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*cacheEntry)
		c.removeLocked(entry.key)
	}
}

// estimateImageBytes returns the estimated decoded size in bytes (RGBA, 4 bytes/pixel).
func estimateImageBytes(img image.Image) int64 {
	b := img.Bounds()
	return int64(b.Dx()) * int64(b.Dy()) * 4
}
