package testing

import (
	"fmt"
	"image"
	"math"
	"sort"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

// DisplayOp represents a serialized canvas drawing operation.
type DisplayOp struct {
	Op     string         `json:"op"`
	Params map[string]any `json:"params,omitempty"`
}

// serializingCanvas implements graphics.Canvas and records ops as DisplayOp.
type serializingCanvas struct {
	ops  []DisplayOp
	size graphics.Size
}

func (c *serializingCanvas) Save() {
	c.ops = append(c.ops, DisplayOp{Op: "save"})
}

func (c *serializingCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "saveLayerAlpha",
		Params: sortedMap("bounds", serializeRect(bounds), "alpha", round2(alpha)),
	})
}

func (c *serializingCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint) {
	params := sortedMap("bounds", serializeRect(bounds))
	if paint != nil {
		params["color"] = serializeColor(paint.Color)
	}
	c.ops = append(c.ops, DisplayOp{Op: "saveLayer", Params: params})
}

func (c *serializingCanvas) Restore() {
	c.ops = append(c.ops, DisplayOp{Op: "restore"})
}

func (c *serializingCanvas) Translate(dx, dy float64) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "translate",
		Params: sortedMap("dx", round2(dx), "dy", round2(dy)),
	})
}

func (c *serializingCanvas) Scale(sx, sy float64) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "scale",
		Params: sortedMap("sx", round2(sx), "sy", round2(sy)),
	})
}

func (c *serializingCanvas) Rotate(radians float64) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "rotate",
		Params: sortedMap("radians", round2(radians)),
	})
}

func (c *serializingCanvas) ClipRect(rect graphics.Rect) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "clipRect",
		Params: sortedMap("rect", serializeRect(rect)),
	})
}

func (c *serializingCanvas) ClipRRect(rrect graphics.RRect) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "clipRRect",
		Params: sortedMap("rect", serializeRect(rrect.Rect), "radius", serializeRadius(rrect)),
	})
}

func (c *serializingCanvas) ClipPath(_ *graphics.Path, _ graphics.ClipOp, _ bool) {
	c.ops = append(c.ops, DisplayOp{Op: "clipPath"})
}

func (c *serializingCanvas) Clear(color graphics.Color) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "clear",
		Params: sortedMap("color", serializeColor(color)),
	})
}

func (c *serializingCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawRect",
		Params: sortedMap("rect", serializeRect(rect), "color", serializeColor(paint.Color)),
	})
}

func (c *serializingCanvas) DrawRRect(rrect graphics.RRect, paint graphics.Paint) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawRRect",
		Params: sortedMap(
			"rect", serializeRect(rrect.Rect),
			"radius", serializeRadius(rrect),
			"color", serializeColor(paint.Color),
		),
	})
}

func (c *serializingCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawCircle",
		Params: sortedMap(
			"cx", round2(center.X),
			"cy", round2(center.Y),
			"radius", round2(radius),
			"color", serializeColor(paint.Color),
		),
	})
}

func (c *serializingCanvas) DrawLine(start, end graphics.Offset, paint graphics.Paint) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawLine",
		Params: sortedMap(
			"x1", round2(start.X), "y1", round2(start.Y),
			"x2", round2(end.X), "y2", round2(end.Y),
			"color", serializeColor(paint.Color),
		),
	})
}

func (c *serializingCanvas) DrawPath(_ *graphics.Path, paint graphics.Paint) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawPath",
		Params: sortedMap("color", serializeColor(paint.Color)),
	})
}

func (c *serializingCanvas) DrawText(_ *graphics.TextLayout, position graphics.Offset) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawText",
		Params: sortedMap("x", round2(position.X), "y", round2(position.Y)),
	})
}

func (c *serializingCanvas) DrawImage(_ image.Image, position graphics.Offset) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawImage",
		Params: sortedMap("x", round2(position.X), "y", round2(position.Y)),
	})
}

func (c *serializingCanvas) DrawImageRect(_ image.Image, _, dstRect graphics.Rect, _ graphics.FilterQuality, _ uintptr) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawImageRect",
		Params: sortedMap("dst", serializeRect(dstRect)),
	})
}

func (c *serializingCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawRectShadow",
		Params: sortedMap(
			"rect", serializeRect(rect),
			"color", serializeColor(shadow.Color),
			"blur", round2(shadow.BlurRadius),
		),
	})
}

func (c *serializingCanvas) DrawRRectShadow(rrect graphics.RRect, shadow graphics.BoxShadow) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawRRectShadow",
		Params: sortedMap(
			"rect", serializeRect(rrect.Rect),
			"color", serializeColor(shadow.Color),
			"blur", round2(shadow.BlurRadius),
		),
	})
}

func (c *serializingCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64) {
	c.ops = append(c.ops, DisplayOp{
		Op: "saveLayerBlur",
		Params: sortedMap(
			"bounds", serializeRect(bounds),
			"sigmaX", round2(sigmaX),
			"sigmaY", round2(sigmaY),
		),
	})
}

func (c *serializingCanvas) DrawSVG(_ unsafe.Pointer, bounds graphics.Rect) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "drawSVG",
		Params: sortedMap("bounds", serializeRect(bounds)),
	})
}

func (c *serializingCanvas) DrawSVGTinted(_ unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawSVGTinted",
		Params: sortedMap(
			"bounds", serializeRect(bounds),
			"tintColor", serializeColor(tintColor),
		),
	})
}

func (c *serializingCanvas) DrawLottie(_ unsafe.Pointer, bounds graphics.Rect, t float64) {
	c.ops = append(c.ops, DisplayOp{
		Op: "drawLottie",
		Params: sortedMap(
			"bounds", serializeRect(bounds),
			"t", t,
		),
	})
}

func (c *serializingCanvas) OccludePlatformViews(mask *graphics.Path) {
	c.ops = append(c.ops, DisplayOp{
		Op:     "occludePlatformViews",
		Params: sortedMap("bounds", serializeRect(mask.Bounds())),
	})
}

func (c *serializingCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {
	c.ops = append(c.ops, DisplayOp{
		Op: "embedPlatformView",
		Params: sortedMap(
			"viewId", float64(viewID),
			"width", round2(size.Width),
			"height", round2(size.Height),
		),
	})
}

func (c *serializingCanvas) Size() graphics.Size {
	return c.size
}

// serializeDisplayList replays a DisplayList through the serializing canvas.
func serializeDisplayList(dl *graphics.DisplayList) []DisplayOp {
	canvas := &serializingCanvas{size: dl.Size()}
	dl.Paint(canvas)
	return canvas.ops
}

// --- Serialization helpers ---

func serializeRect(r graphics.Rect) map[string]any {
	return sortedMap(
		"left", round2(r.Left),
		"top", round2(r.Top),
		"right", round2(r.Right),
		"bottom", round2(r.Bottom),
	)
}

func serializeRadius(rr graphics.RRect) map[string]any {
	// If all corners are the same, use a single value
	if rr.TopLeft == rr.TopRight && rr.TopRight == rr.BottomRight && rr.BottomRight == rr.BottomLeft {
		return sortedMap("x", round2(rr.TopLeft.X), "y", round2(rr.TopLeft.Y))
	}
	return sortedMap(
		"topLeft", sortedMap("x", round2(rr.TopLeft.X), "y", round2(rr.TopLeft.Y)),
		"topRight", sortedMap("x", round2(rr.TopRight.X), "y", round2(rr.TopRight.Y)),
		"bottomRight", sortedMap("x", round2(rr.BottomRight.X), "y", round2(rr.BottomRight.Y)),
		"bottomLeft", sortedMap("x", round2(rr.BottomLeft.X), "y", round2(rr.BottomLeft.Y)),
	)
}

func serializeColor(c graphics.Color) string {
	return fmt.Sprintf("0x%08X", uint32(c))
}

// round2 rounds a float64 to 2 decimal places.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// sortedMap creates a map from alternating key-value pairs.
// Keys are sorted alphabetically in the resulting map (Go maps iterate
// in random order, but JSON marshaling sorts keys via our snapshot encoder).
func sortedMap(kvs ...any) map[string]any {
	m := make(map[string]any, len(kvs)/2)
	for i := 0; i+1 < len(kvs); i += 2 {
		m[kvs[i].(string)] = kvs[i+1]
	}
	return m
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
