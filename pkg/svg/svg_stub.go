//go:build !android && !ios

// Package svg provides SVG loading and rendering using Skia's native SVG DOM.
//
// This is a stub implementation for unsupported platforms. All loading functions
// return an error indicating SVG is not supported.
package svg

import (
	"errors"
	"io"

	"github.com/go-drift/drift/pkg/graphics"
)

// PreserveAspectRatio controls how an SVG scales to fit its container.
type PreserveAspectRatio struct {
	// Align specifies where to position the viewBox within the viewport.
	Align Alignment
	// Scale specifies whether to contain or cover the viewport.
	Scale Scale
}

// Alignment specifies how the viewBox aligns within the viewport.
type Alignment int

const (
	AlignXMidYMid Alignment = iota // Default: center horizontally and vertically
	AlignXMinYMin                  // Top-left
	AlignXMidYMin                  // Top-center
	AlignXMaxYMin                  // Top-right
	AlignXMinYMid                  // Middle-left
	AlignXMaxYMid                  // Middle-right
	AlignXMinYMax                  // Bottom-left
	AlignXMidYMax                  // Bottom-center
	AlignXMaxYMax                  // Bottom-right
	AlignNone                      // Stretch to fill (ignore aspect ratio)
)

// Scale specifies how the viewBox scales to fit the viewport.
type Scale int

const (
	ScaleMeet  Scale = iota // Contain: scale to fit entirely within bounds
	ScaleSlice              // Cover: scale to cover bounds entirely (may clip)
)

// Icon represents a loaded SVG icon backed by Skia's SVG DOM.
//
// # Scaling Behavior
//
// Icons always scale to fill their render bounds. The SVG's viewBox defines the
// aspect ratio, and preserveAspectRatio (default: contain, centered) controls
// how it fits. This means:
//   - SVGs with explicit pixel dimensions (width="400") scale to the requested bounds
//   - Small icons (24x24) rendered at 24x24 are visually unchanged (100% scale)
//   - To render at intrinsic size, pass bounds matching ViewBox() dimensions
//
// # Lifetime Rules
//
// Icons must not be destroyed while any display list that references them might
// still be replayed. Display lists are used by RepaintBoundary and other caching
// mechanisms. In practice:
//   - Icons used in widgets should be kept alive for the widget's lifetime
//   - For globally cached icons (e.g., app icons), call Destroy only at app shutdown
//   - If unsure, don't call Destroy - the memory will be reclaimed at process exit
//
// To detect lifetime violations during development, build with -tags svgdebug.
// This enables runtime checks that panic if Destroy is called while a display
// list still references the Icon.
//
// # Thread Safety
//
// Icons must only be rendered from the UI thread. Do not share Icons between
// goroutines. Rendering the same Icon at two different sizes in the same frame
// will result in last-write-wins for the container size.
type Icon struct {
	viewBox graphics.Rect
}

// Load parses an SVG from the provided reader.
func Load(r io.Reader) (*Icon, error) {
	return nil, errors.New("svg: not supported on this platform")
}

// LoadBytes parses an SVG from byte data.
func LoadBytes(data []byte) (*Icon, error) {
	return nil, errors.New("svg: not supported on this platform")
}

// LoadFile parses an SVG from a file path.
// Relative resource references (e.g., <image href="./foo.png">) will be resolved
// relative to the file's directory.
func LoadFile(path string) (*Icon, error) {
	return nil, errors.New("svg: not supported on this platform")
}

// ViewBox returns the viewBox of the SVG.
func (i *Icon) ViewBox() graphics.Rect {
	return graphics.Rect{}
}

// Draw renders the SVG onto a canvas within the specified bounds.
// The SVG scales to fill the bounds while respecting preserveAspectRatio
// (default: contain, centered) unless overridden via SetPreserveAspectRatio.
// Content is clipped to the provided bounds.
//
// Note: tintColor is currently ignored (known regression from oksvg implementation).
func (i *Icon) Draw(canvas graphics.Canvas, bounds graphics.Rect, tintColor graphics.Color) {}

// SetPreserveAspectRatio overrides the SVG's preserveAspectRatio attribute.
// This controls how the viewBox scales and aligns within the render bounds.
// By default, SVGs use AlignXMidYMid + ScaleMeet (contain, centered).
//
// Note: This mutates the Icon. If the same Icon is used by multiple widgets
// with different settings, last write wins.
func (i *Icon) SetPreserveAspectRatio(par PreserveAspectRatio) {}

// Destroy releases the SVG DOM resources.
//
// WARNING: Do not call while any display list that recorded this Icon might
// still be replayed. This includes display lists cached by RepaintBoundary.
// See Icon type documentation for lifetime rules.
//
// If you're unsure whether display lists might still reference this Icon,
// don't call Destroy - the memory will be reclaimed when the process exits.
//
// To detect lifetime violations, build with -tags svgdebug.
func (i *Icon) Destroy() {}
