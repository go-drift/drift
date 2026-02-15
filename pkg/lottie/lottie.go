//go:build android || ios

// Package lottie provides Lottie animation loading and rendering using Skia's Skottie module.
package lottie

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/skia"
)

// Animation represents a loaded Lottie animation backed by Skia's Skottie player.
//
// # Lifetime Rules
//
// Animations must not be destroyed while any display list that references them
// might still be replayed. In practice, keep animations alive for the widget's
// lifetime. If unsure, don't call Destroy.
//
// # Thread Safety
//
// Animations must only be rendered from the UI thread.
type Animation struct {
	skottie  *skia.Skottie
	duration time.Duration
	size     graphics.Size
}

// Load parses a Lottie animation from the provided reader.
func Load(r io.Reader) (*Animation, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

// LoadBytes parses a Lottie animation from byte data.
func LoadBytes(data []byte) (*Animation, error) {
	s := skia.NewSkottie(data)
	if s == nil {
		return nil, errors.New("lottie: failed to parse animation data")
	}
	dur := s.Duration()
	w, h := s.Size()
	return &Animation{
		skottie:  s,
		duration: time.Duration(dur * float64(time.Second)),
		size:     graphics.Size{Width: w, Height: h},
	}, nil
}

// LoadFile parses a Lottie animation from a file path.
func LoadFile(path string) (*Animation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

// Duration returns the total duration of the animation.
func (a *Animation) Duration() time.Duration {
	if a == nil {
		return 0
	}
	return a.duration
}

// Size returns the intrinsic size of the animation.
func (a *Animation) Size() graphics.Size {
	if a == nil {
		return graphics.Size{}
	}
	return a.size
}

// Draw renders the animation at normalized time t (0.0 to 1.0) within bounds.
func (a *Animation) Draw(canvas graphics.Canvas, bounds graphics.Rect, t float64) {
	if a == nil || a.skottie == nil {
		return
	}
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return
	}
	canvas.DrawLottie(a.skottie.Ptr(), bounds, t)
}

// Destroy releases the animation resources.
func (a *Animation) Destroy() {
	if a != nil && a.skottie != nil {
		a.skottie.Destroy()
		a.skottie = nil
	}
}
