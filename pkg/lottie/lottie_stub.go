//go:build !android && !ios

// Package lottie provides Lottie animation loading and rendering using Skia's Skottie module.
//
// This is a stub implementation for unsupported platforms. All loading functions
// return an error indicating Lottie is not supported.
package lottie

import (
	"errors"
	"io"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
)

// Animation represents a loaded Lottie animation backed by Skia's Skottie player.
type Animation struct{}

// Load parses a Lottie animation from the provided reader.
func Load(r io.Reader) (*Animation, error) {
	return nil, errors.New("lottie: not supported on this platform")
}

// LoadBytes parses a Lottie animation from byte data.
func LoadBytes(data []byte) (*Animation, error) {
	return nil, errors.New("lottie: not supported on this platform")
}

// LoadFile parses a Lottie animation from a file path.
func LoadFile(path string) (*Animation, error) {
	return nil, errors.New("lottie: not supported on this platform")
}

// Duration returns the total duration of the animation.
func (a *Animation) Duration() time.Duration {
	if a == nil {
		return 0
	}
	return 0
}

// Size returns the intrinsic size of the animation.
func (a *Animation) Size() graphics.Size {
	if a == nil {
		return graphics.Size{}
	}
	return graphics.Size{}
}

// Draw renders the animation at normalized time t (0.0 to 1.0) within bounds.
func (a *Animation) Draw(canvas graphics.Canvas, bounds graphics.Rect, t float64) {}

// Destroy releases the animation resources.
func (a *Animation) Destroy() {}
