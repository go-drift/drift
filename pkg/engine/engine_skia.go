//go:build android || darwin || ios

package engine

import (
	"errors"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/skia"
)

type skiaStateTracker struct {
	mu      sync.Mutex
	ctx     *skia.Context
	backend string
	lastErr string
}

var skiaState skiaStateTracker

// platformGeometryTimeout is the maximum time the render thread waits for
// native to confirm platform view geometry has been applied. Half a frame
// at 60fps — long enough for the main thread to run a posted closure, short
// enough to avoid visible stalls if the signal is lost.
const platformGeometryTimeout = 8 * time.Millisecond

var (
	errInvalidSize = errors.New("skia: invalid surface size")
	errNilBuffer   = errors.New("skia: nil texture buffer")
)

// InitSkiaGL initializes the Skia GL context using the current OpenGL context.
func InitSkiaGL() error {
	skiaState.mu.Lock()

	if skiaState.ctx != nil {
		if skiaState.backend != "gl" {
			err := skiaState.setError(errors.New("skia: context already initialized for " + skiaState.backend))
			skiaState.mu.Unlock()
			return err
		}
		skiaState.ctx.Destroy()
		skiaState.ctx = nil
	}

	ctx, err := skia.NewGLContext()
	if err != nil {
		err = skiaState.setError(err)
		skiaState.mu.Unlock()
		return err
	}
	skiaState.ctx = ctx
	skiaState.backend = "gl"
	skiaState.mu.Unlock()

	// Warmup shaders outside the lock (runs on GL thread, logs on failure).
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
			err := skiaState.setError(errors.New("skia: context already initialized for " + skiaState.backend))
			skiaState.mu.Unlock()
			return err
		}
		skiaState.mu.Unlock()
		return nil
	}

	ctx, err := skia.NewMetalContext(device, queue)
	if err != nil {
		err = skiaState.setError(err)
		skiaState.mu.Unlock()
		return err
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

// RenderSkiaGL draws a frame into the currently bound OpenGL framebuffer.
func RenderSkiaGL(width, height int) error {
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
	if err := app.Paint(canvas, graphics.Size{Width: float64(width), Height: float64(height)}); err != nil {
		return skiaState.setError(err)
	}
	surface.Flush()
	skiaState.clearError()
	return nil
}

// RenderSkiaMetal draws a frame into the provided Metal texture.
func RenderSkiaMetal(width, height int, texture unsafe.Pointer) error {
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
	if err := app.Paint(canvas, graphics.Size{Width: float64(width), Height: float64(height)}); err != nil {
		return skiaState.setError(err)
	}
	surface.Flush()
	// Wait for native to confirm geometry applied (no-op if no platform views).
	// GPU work is already submitted above and runs in parallel with this wait.
	platform.GetPlatformViewRegistry().WaitGeometryApplied(platformGeometryTimeout)
	skiaState.clearError()
	return nil
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
	skiaState.mu.Lock()
	defer skiaState.mu.Unlock()
	return skiaState.lastErr
}

func (s *skiaStateTracker) setError(err error) error {
	if err == nil {
		return nil
	}
	s.lastErr = err.Error()
	return err
}

func (s *skiaStateTracker) clearError() {
	s.lastErr = ""
}
