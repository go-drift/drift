package graphics

import (
	"image"
	"unsafe"
)

// DisplayList is an immutable list of drawing operations.
// It can be replayed onto any Canvas implementation.
type DisplayList struct {
	ops  []displayOp
	size Size
}

// Paint replays the recorded operations onto the provided canvas.
func (d *DisplayList) Paint(canvas Canvas) {
	for _, op := range d.ops {
		op.execute(canvas)
	}
}

// Size returns the size recorded when the display list was created.
func (d *DisplayList) Size() Size {
	return d.size
}

// Dispose releases debug tracking for SVG pointers in this display list.
// In release builds this is a no-op. In debug builds (-tags svgdebug) it
// decrements the refcount for tracked SVG pointers.
// Call this when a cached display list is being replaced or discarded.
func (d *DisplayList) Dispose() {
	for _, op := range d.ops {
		switch svg := op.(type) {
		case opSVG:
			svgDebugUntrack(svg.svgPtr)
		case opSVGTinted:
			svgDebugUntrack(svg.svgPtr)
		}
	}
}

// PictureRecorder records drawing commands into a display list.
type PictureRecorder struct {
	ops       []displayOp
	recording bool
	size      Size
}

// BeginRecording starts a new recording session.
func (r *PictureRecorder) BeginRecording(size Size) Canvas {
	r.ops = r.ops[:0]
	r.recording = true
	r.size = size
	return &recordingCanvas{recorder: r, size: size}
}

// EndRecording finishes the recording and returns a display list.
func (r *PictureRecorder) EndRecording() *DisplayList {
	if !r.recording {
		return &DisplayList{size: r.size}
	}
	r.recording = false
	ops := make([]displayOp, len(r.ops))
	copy(ops, r.ops)
	return &DisplayList{
		ops:  ops,
		size: r.size,
	}
}

func (r *PictureRecorder) append(op displayOp) {
	if !r.recording {
		return
	}
	r.ops = append(r.ops, op)
}

// DrawChildLayer records a reference to a child repaint boundary's layer.
// When the display list is replayed during compositing, the child layer's
// content is composited at the current canvas transform and clip state.
func (r *PictureRecorder) DrawChildLayer(layer *Layer) {
	r.append(opDrawChildLayer{layer: layer})
}

type displayOp interface {
	execute(canvas Canvas)
}

type recordingCanvas struct {
	recorder *PictureRecorder
	size     Size
}

func (c *recordingCanvas) Save() {
	c.recorder.append(opSave{})
}

func (c *recordingCanvas) SaveLayerAlpha(bounds Rect, alpha float64) {
	c.recorder.append(opSaveLayerAlpha{bounds: bounds, alpha: alpha})
}

func (c *recordingCanvas) SaveLayer(bounds Rect, paint *Paint) {
	var paintCopy *Paint
	if paint != nil {
		p := *paint
		// Deep copy filter chains to ensure immutability
		p.ColorFilter = paint.ColorFilter.clone()
		p.ImageFilter = paint.ImageFilter.clone()
		paintCopy = &p
	}
	c.recorder.append(opSaveLayer{bounds: bounds, paint: paintCopy})
}

func (c *recordingCanvas) Restore() {
	c.recorder.append(opRestore{})
}

func (c *recordingCanvas) Translate(dx, dy float64) {
	c.recorder.append(opTranslate{dx: dx, dy: dy})
}

func (c *recordingCanvas) Scale(sx, sy float64) {
	c.recorder.append(opScale{sx: sx, sy: sy})
}

func (c *recordingCanvas) Rotate(radians float64) {
	c.recorder.append(opRotate{radians: radians})
}

func (c *recordingCanvas) ClipRect(rect Rect) {
	c.recorder.append(opClipRect{rect: rect})
}

func (c *recordingCanvas) ClipRRect(rrect RRect) {
	c.recorder.append(opClipRRect{rrect: rrect})
}

func (c *recordingCanvas) ClipPath(path *Path, op ClipOp, antialias bool) {
	c.recorder.append(opClipPath{path: deepCopyPath(path), op: op, antialias: antialias})
}

func (c *recordingCanvas) Clear(color Color) {
	c.recorder.append(opClear{color: color})
}

func (c *recordingCanvas) DrawRect(rect Rect, paint Paint) {
	c.recorder.append(opRect{rect: rect, paint: paint})
}

func (c *recordingCanvas) DrawRRect(rrect RRect, paint Paint) {
	c.recorder.append(opRRect{rrect: rrect, paint: paint})
}

func (c *recordingCanvas) DrawCircle(center Offset, radius float64, paint Paint) {
	c.recorder.append(opCircle{center: center, radius: radius, paint: paint})
}

func (c *recordingCanvas) DrawLine(start, end Offset, paint Paint) {
	c.recorder.append(opLine{start: start, end: end, paint: paint})
}

func (c *recordingCanvas) DrawText(layout *TextLayout, position Offset) {
	c.recorder.append(opText{layout: layout, position: position})
}

func (c *recordingCanvas) DrawImage(image image.Image, position Offset) {
	c.recorder.append(opImage{image: image, position: position})
}

func (c *recordingCanvas) DrawImageRect(img image.Image, srcRect, dstRect Rect, quality FilterQuality, cacheKey uintptr) {
	c.recorder.append(opImageRect{image: img, srcRect: srcRect, dstRect: dstRect, quality: quality, cacheKey: cacheKey})
}

func (c *recordingCanvas) DrawPath(path *Path, paint Paint) {
	c.recorder.append(opPath{path: deepCopyPath(path), paint: paint})
}

func (c *recordingCanvas) DrawRectShadow(rect Rect, shadow BoxShadow) {
	c.recorder.append(opRectShadow{rect: rect, shadow: shadow})
}

func (c *recordingCanvas) DrawRRectShadow(rrect RRect, shadow BoxShadow) {
	c.recorder.append(opRRectShadow{rrect: rrect, shadow: shadow})
}

func (c *recordingCanvas) SaveLayerBlur(bounds Rect, sigmaX, sigmaY float64) {
	c.recorder.append(opSaveLayerBlur{bounds: bounds, sigmaX: sigmaX, sigmaY: sigmaY})
}

func (c *recordingCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds Rect) {
	if svgPtr != nil {
		svgDebugTrack(svgPtr) // no-op in release builds
	}
	c.recorder.append(opSVG{svgPtr: svgPtr, bounds: bounds})
}

func (c *recordingCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds Rect, tintColor Color) {
	if svgPtr != nil {
		svgDebugTrack(svgPtr) // no-op in release builds
	}
	c.recorder.append(opSVGTinted{svgPtr: svgPtr, bounds: bounds, tintColor: tintColor})
}

func (c *recordingCanvas) EmbedPlatformView(viewID int64, size Size) {
	c.recorder.append(opEmbedPlatformView{viewID: viewID, size: size})
}

// DrawChildLayer records a child layer reference at current canvas state.
func (c *recordingCanvas) DrawChildLayer(layer *Layer) {
	c.recorder.DrawChildLayer(layer)
}

func (c *recordingCanvas) Size() Size {
	return c.size
}

// opEmbedPlatformView records a platform view embedding.
// On execute, calls canvas.EmbedPlatformView() which, for GeometryCanvas,
// resolves the current transform+clip and captures native view geometry.
type opEmbedPlatformView struct {
	viewID int64
	size   Size
}

func (op opEmbedPlatformView) execute(canvas Canvas) {
	canvas.EmbedPlatformView(op.viewID, op.size)
}

// opDrawChildLayer references a child layer to composite at current canvas state.
//
// Future optimization: the compositing canvas could expose its clip bounds,
// allowing this op to skip compositing layers entirely outside the visible
// region. If added, ensure platform views in culled layers are still hidden
// (the viewsSeenThisFrame tracking in PlatformViewRegistry handles this).
type opDrawChildLayer struct {
	layer *Layer
}

func (op opDrawChildLayer) execute(canvas Canvas) {
	if op.layer == nil {
		return
	}
	op.layer.Composite(canvas)
}

type opSave struct{}

func (opSave) execute(canvas Canvas) {
	canvas.Save()
}

type opSaveLayerAlpha struct {
	bounds Rect
	alpha  float64
}

func (op opSaveLayerAlpha) execute(canvas Canvas) {
	canvas.SaveLayerAlpha(op.bounds, op.alpha)
}

type opSaveLayer struct {
	bounds Rect
	paint  *Paint
}

func (op opSaveLayer) execute(canvas Canvas) {
	canvas.SaveLayer(op.bounds, op.paint)
}

type opRestore struct{}

func (opRestore) execute(canvas Canvas) {
	canvas.Restore()
}

type opTranslate struct {
	dx, dy float64
}

func (op opTranslate) execute(canvas Canvas) {
	canvas.Translate(op.dx, op.dy)
}

type opScale struct {
	sx, sy float64
}

func (op opScale) execute(canvas Canvas) {
	canvas.Scale(op.sx, op.sy)
}

type opRotate struct {
	radians float64
}

func (op opRotate) execute(canvas Canvas) {
	canvas.Rotate(op.radians)
}

type opClipRect struct {
	rect Rect
}

func (op opClipRect) execute(canvas Canvas) {
	canvas.ClipRect(op.rect)
}

type opClipRRect struct {
	rrect RRect
}

func (op opClipRRect) execute(canvas Canvas) {
	canvas.ClipRRect(op.rrect)
}

type opClipPath struct {
	path      *Path
	op        ClipOp
	antialias bool
}

func (op opClipPath) execute(canvas Canvas) {
	canvas.ClipPath(op.path, op.op, op.antialias)
}

type opClear struct {
	color Color
}

func (op opClear) execute(canvas Canvas) {
	canvas.Clear(op.color)
}

type opRect struct {
	rect  Rect
	paint Paint
}

func (op opRect) execute(canvas Canvas) {
	canvas.DrawRect(op.rect, op.paint)
}

type opRRect struct {
	rrect RRect
	paint Paint
}

func (op opRRect) execute(canvas Canvas) {
	canvas.DrawRRect(op.rrect, op.paint)
}

type opCircle struct {
	center Offset
	radius float64
	paint  Paint
}

func (op opCircle) execute(canvas Canvas) {
	canvas.DrawCircle(op.center, op.radius, op.paint)
}

type opLine struct {
	start, end Offset
	paint      Paint
}

func (op opLine) execute(canvas Canvas) {
	canvas.DrawLine(op.start, op.end, op.paint)
}

type opText struct {
	layout   *TextLayout
	position Offset
}

func (op opText) execute(canvas Canvas) {
	canvas.DrawText(op.layout, op.position)
}

type opImage struct {
	image    image.Image
	position Offset
}

func (op opImage) execute(canvas Canvas) {
	canvas.DrawImage(op.image, op.position)
}

type opImageRect struct {
	image    image.Image
	srcRect  Rect
	dstRect  Rect
	quality  FilterQuality
	cacheKey uintptr
}

func (op opImageRect) execute(canvas Canvas) {
	canvas.DrawImageRect(op.image, op.srcRect, op.dstRect, op.quality, op.cacheKey)
}

type opPath struct {
	path  *Path
	paint Paint
}

func (op opPath) execute(canvas Canvas) {
	canvas.DrawPath(op.path, op.paint)
}

type opRectShadow struct {
	rect   Rect
	shadow BoxShadow
}

func (op opRectShadow) execute(canvas Canvas) {
	canvas.DrawRectShadow(op.rect, op.shadow)
}

type opRRectShadow struct {
	rrect  RRect
	shadow BoxShadow
}

func (op opRRectShadow) execute(canvas Canvas) {
	canvas.DrawRRectShadow(op.rrect, op.shadow)
}

type opSaveLayerBlur struct {
	bounds Rect
	sigmaX float64
	sigmaY float64
}

func (op opSaveLayerBlur) execute(canvas Canvas) {
	canvas.SaveLayerBlur(op.bounds, op.sigmaX, op.sigmaY)
}

type opSVG struct {
	svgPtr unsafe.Pointer // C handle from SVGDOM.Ptr() - stable, not Go heap
	bounds Rect
}

func (op opSVG) execute(canvas Canvas) {
	canvas.DrawSVG(op.svgPtr, op.bounds)
}

type opSVGTinted struct {
	svgPtr    unsafe.Pointer
	bounds    Rect
	tintColor Color
}

func (op opSVGTinted) execute(canvas Canvas) {
	canvas.DrawSVGTinted(op.svgPtr, op.bounds, op.tintColor)
}

// deepCopyPath creates a fully independent copy of a Path, including all
// command arguments. Returns nil if path is nil.
func deepCopyPath(path *Path) *Path {
	if path == nil {
		return nil
	}
	pathCopy := &Path{
		Commands: make([]PathCommand, len(path.Commands)),
		FillRule: path.FillRule,
	}
	for i, cmd := range path.Commands {
		pathCopy.Commands[i] = PathCommand{
			Op:   cmd.Op,
			Args: append([]float64(nil), cmd.Args...),
		}
	}
	return pathCopy
}
