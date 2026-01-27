//go:build android || ios

// Package svg provides SVG loading and rendering using Skia's native SVG DOM.
package svg

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/skia"
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
	dom     *skia.SVGDOM
	viewBox rendering.Rect
}

// Load parses an SVG from the provided reader.
func Load(r io.Reader) (*Icon, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

// LoadBytes parses an SVG from byte data.
func LoadBytes(data []byte) (*Icon, error) {
	dom := skia.NewSVGDOM(data)
	if dom == nil {
		return nil, errors.New("svg: failed to parse SVG data")
	}
	w, h := dom.Size()
	if w == 0 || h == 0 {
		w, h = 24, 24 // default size
	}
	return &Icon{
		dom:     dom,
		viewBox: rendering.Rect{Right: w, Bottom: h},
	}, nil
}

// LoadFile parses an SVG from a file path.
// Relative resource references (e.g., <image href="./foo.png">) will be resolved
// relative to the file's directory.
func LoadFile(path string) (*Icon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use file's directory as base path for relative resource resolution
	basePath := filepath.Dir(path)
	dom := skia.NewSVGDOMWithBase(data, basePath)
	if dom == nil {
		return nil, errors.New("svg: failed to parse SVG data")
	}

	w, h := dom.Size()
	if w == 0 || h == 0 {
		w, h = 24, 24
	}
	return &Icon{
		dom:     dom,
		viewBox: rendering.Rect{Right: w, Bottom: h},
	}, nil
}

// ViewBox returns the viewBox of the SVG.
func (i *Icon) ViewBox() rendering.Rect {
	if i == nil {
		return rendering.Rect{}
	}
	return i.viewBox
}

// Draw renders the SVG onto a canvas within the specified bounds.
// The SVG scales to fill the bounds while respecting preserveAspectRatio
// (default: contain, centered) unless overridden via SetPreserveAspectRatio.
// Content is clipped to the provided bounds.
// If tintColor is non-zero, all SVG colors are replaced with the tint
// while preserving alpha (useful for icons). Note: tinting affects all
// content including gradients and embedded images.
func (i *Icon) Draw(canvas rendering.Canvas, bounds rendering.Rect, tintColor rendering.Color) {
	if i == nil || i.dom == nil {
		return
	}
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return
	}

	// Set root width/height to 100% so SVG scales to container bounds.
	// This is needed for SVGs with explicit pixel dimensions.
	i.dom.SetSizeToContainer()

	canvas.DrawSVGTinted(i.dom.Ptr(), bounds, tintColor)
}

// SetPreserveAspectRatio overrides the SVG's preserveAspectRatio attribute.
// This controls how the viewBox scales and aligns within the render bounds.
// By default, SVGs use AlignXMidYMid + ScaleMeet (contain, centered).
//
// Note: This mutates the Icon. If the same Icon is used by multiple widgets
// with different settings, last write wins.
func (i *Icon) SetPreserveAspectRatio(par PreserveAspectRatio) {
	if i == nil || i.dom == nil {
		return
	}
	i.dom.SetPreserveAspectRatio(int(par.Align), int(par.Scale))
}

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
func (i *Icon) Destroy() {
	if i != nil && i.dom != nil {
		rendering.SVGDebugCheckDestroy(i.dom.Ptr())
		i.dom.Destroy()
		i.dom = nil
	}
}
