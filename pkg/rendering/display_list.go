package rendering

import "image"

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

func (c *recordingCanvas) DrawPath(path *Path, paint Paint) {
	c.recorder.append(opPath{path: path, paint: paint})
}

func (c *recordingCanvas) Size() Size {
	return c.size
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

type opPath struct {
	path  *Path
	paint Paint
}

func (op opPath) execute(canvas Canvas) {
	canvas.DrawPath(op.path, op.paint)
}
