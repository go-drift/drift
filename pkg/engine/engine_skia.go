//go:build android || darwin || ios

package engine

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/skia"
)

type skiaStateTracker struct {
	mu      sync.Mutex // protects ctx and backend only
	ctx     *skia.Context
	backend string
	lastErr atomic.Value // stores string; atomic, no mutex needed
}

var skiaState skiaStateTracker

var (
	errInvalidSize = errors.New("skia: invalid surface size")
	errNilBuffer   = errors.New("skia: nil texture buffer")
)

// InitSkiaMetal initializes the Skia Metal context using the provided device/queue.
func InitSkiaMetal(device, queue unsafe.Pointer) error {
	skiaState.mu.Lock()

	if skiaState.ctx != nil {
		if skiaState.backend != "metal" {
			skiaState.mu.Unlock()
			return skiaState.setError(errors.New("skia: context already initialized for " + skiaState.backend))
		}
		skiaState.mu.Unlock()
		return nil
	}

	ctx, err := skia.NewMetalContext(device, queue)
	if err != nil {
		skiaState.mu.Unlock()
		return skiaState.setError(err)
	}
	skiaState.ctx = ctx
	skiaState.backend = "metal"
	skiaState.mu.Unlock()

	// Warmup shaders outside the lock (runs on main thread, logs on failure).
	// This avoids blocking other callers if warmup is slow.
	if err := ctx.WarmupShaders("metal"); err != nil {
		log.Printf("skia: shader warmup failed: %v", err)
	}

	return nil
}

// InitSkiaVulkan initializes the Skia Vulkan context using the provided Vulkan handles.
func InitSkiaVulkan(instance, physDevice, device, queue uintptr, queueFamilyIndex uint32, getInstanceProcAddr uintptr) error {
	skiaState.mu.Lock()

	if skiaState.ctx != nil {
		if skiaState.backend != "vulkan" {
			skiaState.mu.Unlock()
			return skiaState.setError(errors.New("skia: context already initialized for " + skiaState.backend))
		}
		skiaState.ctx.Destroy()
		skiaState.ctx = nil
	}

	ctx, err := skia.NewVulkanContext(instance, physDevice, device, queue, queueFamilyIndex, getInstanceProcAddr)
	if err != nil {
		skiaState.mu.Unlock()
		return skiaState.setError(err)
	}
	skiaState.ctx = ctx
	skiaState.backend = "vulkan"
	skiaState.mu.Unlock()

	if err := ctx.WarmupShaders("vulkan"); err != nil {
		log.Printf("skia: shader warmup failed: %v", err)
	}

	return nil
}

// RenderSkiaVulkanSync renders a frame into the provided VkImage using the
// split pipeline (composite only). Geometry is applied synchronously by the
// Android UI thread between StepAndSnapshot and this call.
func RenderSkiaVulkanSync(width, height int, vkImage uintptr, vkFormat uint32) error {
	if width <= 0 || height <= 0 {
		return skiaState.setError(errInvalidSize)
	}
	ctx, err := currentSkiaContext("vulkan")
	if err != nil {
		return skiaState.setError(err)
	}
	surface, err := ctx.MakeVulkanSurface(width, height, vkImage, vkFormat)
	if err != nil {
		return skiaState.setError(err)
	}
	defer surface.Destroy()

	canvas := graphics.NewSkiaCanvas(surface.Canvas(), graphics.Size{Width: float64(width), Height: float64(height)})
	if err := app.RenderFrame(canvas); err != nil {
		return skiaState.setError(err)
	}
	surface.Flush()
	skiaState.clearError()
	return nil
}

// StepAndSnapshot runs the engine pipeline and returns the platform view
// geometry snapshot as JSON bytes. Called from the Android UI thread via JNI.
func StepAndSnapshot(width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errInvalidSize
	}
	size := graphics.Size{Width: float64(width), Height: float64(height)}
	snapshot, err := app.StepFrame(size)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	return json.Marshal(snapshot)
}

// RenderSkiaMetalSync renders a frame into the provided Metal texture using the
// split pipeline (composite only). Geometry is applied synchronously by the iOS
// main thread between StepAndSnapshot and this call.
func RenderSkiaMetalSync(width, height int, texture unsafe.Pointer) error {
	if width <= 0 || height <= 0 {
		return skiaState.setError(errInvalidSize)
	}
	if texture == nil {
		return skiaState.setError(errNilBuffer)
	}
	ctx, err := currentSkiaContext("metal")
	if err != nil {
		return skiaState.setError(err)
	}
	surface, err := ctx.MakeMetalSurface(texture, width, height)
	if err != nil {
		return skiaState.setError(err)
	}
	defer surface.Destroy()

	canvas := graphics.NewSkiaCanvas(surface.Canvas(), graphics.Size{Width: float64(width), Height: float64(height)})
	if err := app.RenderFrame(canvas); err != nil {
		return skiaState.setError(err)
	}
	surface.Flush()
	skiaState.clearError()
	return nil
}

// PurgeSkiaResources releases all cached GPU resources regardless of backend.
// Call this after events that may invalidate GPU memory (e.g. sleep/wake,
// surface recreation) to force Skia to rebuild its glyph atlas and other GPU
// caches on the next frame.
func PurgeSkiaResources() {
	skiaState.mu.Lock()
	defer skiaState.mu.Unlock()
	if skiaState.ctx != nil {
		skiaState.ctx.PurgeGpuResources()
	}
}

func currentSkiaContext(backend string) (*skia.Context, error) {
	skiaState.mu.Lock()
	defer skiaState.mu.Unlock()

	if skiaState.ctx == nil {
		return nil, errors.New("skia: context not initialized")
	}
	if skiaState.backend != backend {
		return nil, errors.New("skia: context initialized for " + skiaState.backend)
	}
	return skiaState.ctx, nil
}

// LastSkiaError returns the most recent Skia error message, if any.
func LastSkiaError() string {
	if v, ok := skiaState.lastErr.Load().(string); ok {
		return v
	}
	return ""
}

func (s *skiaStateTracker) setError(err error) error {
	if err == nil {
		return nil
	}
	s.lastErr.Store(err.Error())
	return err
}

func (s *skiaStateTracker) clearError() {
	s.lastErr.Store("")
}
