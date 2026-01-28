//go:build svgdebug

package graphics

import (
	"fmt"
	"sync"
	"unsafe"
)

var (
	svgTrackedPtrs = make(map[unsafe.Pointer]int) // refcount per pointer
	svgTrackedMu   sync.Mutex
)

// svgDebugTrack records an SVG pointer as being referenced by a display list.
// Uses refcounting to handle same SVG in multiple display lists.
func svgDebugTrack(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	svgTrackedMu.Lock()
	svgTrackedPtrs[ptr]++
	svgTrackedMu.Unlock()
}

// svgDebugUntrack decrements refcount for an SVG pointer.
// Called when a display list containing the SVG is discarded.
func svgDebugUntrack(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	svgTrackedMu.Lock()
	if count := svgTrackedPtrs[ptr]; count > 1 {
		svgTrackedPtrs[ptr] = count - 1
	} else {
		delete(svgTrackedPtrs, ptr)
	}
	svgTrackedMu.Unlock()
}

// SVGDebugCheckDestroy should be called before destroying an SVG DOM.
// Panics if the pointer is still tracked by any display list.
func SVGDebugCheckDestroy(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	svgTrackedMu.Lock()
	count := svgTrackedPtrs[ptr]
	svgTrackedMu.Unlock()
	if count > 0 {
		panic(fmt.Sprintf("svg: destroying SVGDOM %p while still referenced by %d display list(s)", ptr, count))
	}
}
