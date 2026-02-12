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
	mu      sync.Mutex   // protects ctx and backend only
	ctx     *skia.Context
	backend string
	lastErr atomic.Value // stores string; atomic, no mutex needed
}

var skiaState skiaStateTracker

var (
	errInvalidSize = errors.New("skia: invalid surface size")
	errNilBuffer   = errors.New("skia: nil texture buffer")
)

// InitSkiaGL initializes the Skia GL context using the current OpenGL context.
func InitSkiaGL() error {
	skiaState.mu.Lock()

	if skiaState.ctx != nil {
		if skiaState.backend != "gl" {
			skiaState.mu.Unlock()
			return skiaState.setError(errors.New("skia: context already initialized for " + skiaState.backend))
		}
		skiaState.ctx.Destroy()
		skiaState.ctx = nil
	}

	ctx, err := skia.NewGLContext()
	if err != nil {
		skiaState.mu.Unlock()
		return skiaState.setError(err)
	}
	skiaState.ctx = ctx
	skiaState.backend = "gl"
	skiaState.mu.Unlock()

	// Warmup shaders outside the lock (runs on init thread, logs on failure).
	// This avoids blocking other callers if warmup is slow.
	if err := ctx.WarmupShaders("gl"); err != nil {
		log.Printf("skia: shader warmup failed: %v", err)
	}

	return nil
}

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

// RenderSkiaGLSync renders a frame into the currently bound framebuffer using
// the split pipeline (RenderFrame only, composite phase). Geometry is applied
// synchronously by the Android UI thread between StepAndSnapshot and this call.
//
// Y-flip is only applied when the current FBO is 0 (default framebuffer).
// HardwareBuffer FBOs have top-left origin matching the Skia coordinate system.
func RenderSkiaGLSync(width, height int) error {
	if width <= 0 || height <= 0 {
		return skiaState.setError(errInvalidSize)
	}
	ctx, err := currentSkiaContext("gl")
	if err != nil {
		return skiaState.setError(err)
	}
	surface, err := ctx.MakeGLSurface(width, height)
	if err != nil {
		return skiaState.setError(err)
	}
	defer surface.Destroy()

	canvas := graphics.NewSkiaCanvas(surface.Canvas(), graphics.Size{Width: float64(width), Height: float64(height)})

	// Only flip for default framebuffer (FBO 0). HardwareBuffer FBOs
	// have top-left origin and don't need flipping.
	fbo := skia.GLGetFramebufferBinding()
	if fbo == 0 {
		canvas.Translate(0, float64(height))
		canvas.Scale(1, -1)
	}

	if err := app.RenderFrame(canvas); err != nil {
		return skiaState.setError(err)
	}
	surface.Flush()
	skiaState.clearError()
	return nil
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

// PurgeSkiaGLResources resets GL state tracking and releases all cached GPU
// resources. Call this after events that may invalidate GPU memory (e.g.
// sleep/wake, surface recreation) to force Skia to rebuild its glyph atlas
// and other GPU caches on the next frame.
func PurgeSkiaGLResources() {
	skiaState.mu.Lock()
	defer skiaState.mu.Unlock()
	if skiaState.ctx != nil && skiaState.backend == "gl" {
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
