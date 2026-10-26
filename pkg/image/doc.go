// Package image decodes images upright, and loads network images with
// in-memory and disk caching.
//
// # Decoding
//
// [Decode] is the standard library's [image.Decode] plus EXIF orientation:
// cameras store a JPEG's pixels as the sensor saw them and record in its
// EXIF data how to turn them upright, which browsers apply and
// [image.Decode] does not. Decode applies all eight orientations, so a
// portrait photo decodes as portrait, its bounds being its displayed size.
// Use it to decode photos, files and assets for [widgets.Image].
//
// # Network Images
//
// The primary entry point is [Loader], which fetches images over HTTP, decodes
// them into standard [image.Image] values, and caches the results in a two-tier
// cache (in-memory LRU via [Cache], optional filesystem via [DiskCache]).
// Concurrent requests for the same URL are deduplicated into a single HTTP
// request, and callers can cancel individual listeners without affecting others.
//
// # Default Loader
//
// [DefaultLoader] returns a shared process-wide Loader with sensible defaults
// (100-entry / 100 MB memory cache, 500 MB disk cache under the platform cache
// directory). The [widgets.NetworkImage] widget uses DefaultLoader automatically.
//
// # Supported Formats
//
// This package registers decoders for PNG, JPEG, GIF, and WebP on import.
// Additional formats can be registered via the standard [image.RegisterFormat]
// mechanism. EXIF orientation is applied to JPEG only.
//
// # Custom Loader
//
// Applications that need custom cache sizes, HTTP clients, or headers can
// construct their own Loader:
//
//	loader := image.NewLoader(image.LoaderOptions{
//	    MemoryCache: image.NewCache(image.CacheOptions{MaxEntries: 50}),
//	    Client:      myAuthClient,
//	})
//
//	cancel := loader.Load(url, image.LoadOptions{
//	    Headers: map[string]string{"Authorization": "Bearer tok"},
//	}, func(result image.LoadResult) {
//	    // handle result.Image or result.Err
//	})
//	defer cancel()
package image
