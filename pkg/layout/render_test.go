package layout

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// boundaryRenderBox is a render object that acts as a repaint boundary.
type boundaryRenderBox struct {
	RenderBoxBase
	paintCalls int
}

func (r *boundaryRenderBox) PerformLayout() {}
func (r *boundaryRenderBox) Paint(ctx *PaintContext) {
	r.paintCalls++
}
func (r *boundaryRenderBox) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}
func (r *boundaryRenderBox) IsRepaintBoundary() bool { return true }
func (r *boundaryRenderBox) EnsureLayer() *graphics.Layer {
	return r.RenderBoxBase.EnsureLayer()
}

// plainRenderBox is a non-boundary render object.
type plainRenderBox struct {
	RenderBoxBase
	paintCalls int
}

func (r *plainRenderBox) PerformLayout() {}
func (r *plainRenderBox) Paint(ctx *PaintContext) {
	r.paintCalls++
}
func (r *plainRenderBox) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

// --- SetSize tests ---

func TestSetSize_NoOpWhenUnchanged(t *testing.T) {
	owner := &PipelineOwner{}
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.SetOwner(owner)
	box.size = graphics.Size{Width: 50, Height: 50}
	box.ClearNeedsPaint()

	// Set same size — should not mark paint dirty
	box.SetSize(graphics.Size{Width: 50, Height: 50})

	if box.NeedsPaint() {
		t.Error("SetSize with same size should not mark needs paint")
	}
}

func TestSetSize_MarksDirtyWhenChanged(t *testing.T) {
	owner := &PipelineOwner{}
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.SetOwner(owner)
	box.size = graphics.Size{Width: 50, Height: 50}
	box.ClearNeedsPaint()

	box.SetSize(graphics.Size{Width: 60, Height: 50})

	if !box.NeedsPaint() {
		t.Error("SetSize with different size should mark needs paint")
	}
}

// --- SetParentData tests ---

func TestSetParentData_MarksParentDirtyOnOffsetChange(t *testing.T) {
	owner := &PipelineOwner{}

	parent := &boundaryRenderBox{}
	parent.SetSelf(parent)
	parent.SetOwner(owner)
	parent.size = graphics.Size{Width: 100, Height: 100}
	parent.ClearNeedsPaint()

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.parent = parent
	child.SetParentData(&BoxParentData{Offset: graphics.Offset{X: 10, Y: 20}})

	// Parent should be marked dirty because child offset changed
	if !parent.NeedsPaint() {
		t.Error("parent should be marked dirty when child offset changes")
	}
}

func TestSetParentData_NoParentDirtyWhenOffsetSame(t *testing.T) {
	owner := &PipelineOwner{}

	parent := &boundaryRenderBox{}
	parent.SetSelf(parent)
	parent.SetOwner(owner)
	parent.size = graphics.Size{Width: 100, Height: 100}

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.parent = parent
	child.SetParentData(&BoxParentData{Offset: graphics.Offset{X: 10, Y: 20}})

	// Clear after initial set
	parent.ClearNeedsPaint()

	// Set same offset again
	child.SetParentData(&BoxParentData{Offset: graphics.Offset{X: 10, Y: 20}})

	if parent.NeedsPaint() {
		t.Error("parent should NOT be marked dirty when child offset is unchanged")
	}
}

func TestSetParentData_NonBoxParentDataDoesNotMarkParent(t *testing.T) {
	owner := &PipelineOwner{}

	parent := &boundaryRenderBox{}
	parent.SetSelf(parent)
	parent.SetOwner(owner)
	parent.size = graphics.Size{Width: 100, Height: 100}
	parent.ClearNeedsPaint()

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.parent = parent

	// Set non-BoxParentData
	child.SetParentData("some string data")

	if parent.NeedsPaint() {
		t.Error("non-BoxParentData should not mark parent dirty")
	}
}

func TestSetParentData_NilParentSafe(t *testing.T) {
	child := &plainRenderBox{}
	child.SetSelf(child)

	// Should not panic with no parent
	child.SetParentData(&BoxParentData{Offset: graphics.Offset{X: 5, Y: 5}})
}

// --- MarkNeedsPaint tests ---

func TestMarkNeedsPaint_StopsAtBoundary(t *testing.T) {
	owner := &PipelineOwner{}

	boundary := &boundaryRenderBox{}
	boundary.SetSelf(boundary)
	boundary.SetOwner(owner)
	boundary.size = graphics.Size{Width: 100, Height: 100}
	boundary.ClearNeedsPaint()

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.parent = boundary

	child.MarkNeedsPaint()

	// Child should be marked
	if !child.NeedsPaint() {
		t.Error("child should be marked dirty")
	}
	// Boundary should be marked (it's the enclosing boundary)
	if !boundary.NeedsPaint() {
		t.Error("boundary should be marked dirty when child marks paint")
	}
}

func TestMarkNeedsPaint_BoundaryEnsuresLayer(t *testing.T) {
	owner := &PipelineOwner{}

	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.SetOwner(owner)
	box.size = graphics.Size{Width: 50, Height: 50}

	box.MarkNeedsPaint()

	layer := box.Layer()
	if layer == nil {
		t.Fatal("MarkNeedsPaint on boundary should ensure layer exists")
	}
	if !layer.Dirty {
		t.Error("layer should be dirty after MarkNeedsPaint")
	}
}

func TestMarkNeedsPaint_SchedulesSelfWithOwner(t *testing.T) {
	owner := &PipelineOwner{}

	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.SetOwner(owner)
	box.size = graphics.Size{Width: 50, Height: 50}

	box.MarkNeedsPaint()

	if !owner.NeedsPaint() {
		t.Error("owner should have paint scheduled")
	}
}

func TestMarkNeedsPaint_NoOwnerStillSetsDirty(t *testing.T) {
	box := &plainRenderBox{}
	box.SetSelf(box)
	box.ClearNeedsPaint()

	box.MarkNeedsPaint()

	if !box.NeedsPaint() {
		t.Error("needsPaint should be set even without owner")
	}
}

// --- SetParent tests ---

func TestSetParent_MarksOldParentDirty(t *testing.T) {
	owner := &PipelineOwner{}

	oldParent := &boundaryRenderBox{}
	oldParent.SetSelf(oldParent)
	oldParent.SetOwner(owner)
	oldParent.size = graphics.Size{Width: 100, Height: 100}
	oldParent.ClearNeedsPaint()

	newParent := &boundaryRenderBox{}
	newParent.SetSelf(newParent)
	newParent.SetOwner(owner)
	newParent.size = graphics.Size{Width: 100, Height: 100}
	newParent.ClearNeedsPaint()

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.SetParent(oldParent)

	// Clear after initial setup
	oldParent.ClearNeedsPaint()
	newParent.ClearNeedsPaint()

	// Reparent to new parent
	child.SetParent(newParent)

	if !oldParent.NeedsPaint() {
		t.Error("old parent should be dirty after child reparented away")
	}
	if !newParent.NeedsPaint() {
		t.Error("new parent should be dirty after gaining child")
	}
}

func TestSetParent_SameParentIsNoOp(t *testing.T) {
	owner := &PipelineOwner{}

	parent := &boundaryRenderBox{}
	parent.SetSelf(parent)
	parent.SetOwner(owner)
	parent.size = graphics.Size{Width: 100, Height: 100}

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.SetParent(parent)

	parent.ClearNeedsPaint()

	// Set same parent again
	child.SetParent(parent)

	if parent.NeedsPaint() {
		t.Error("setting same parent should be no-op, parent should not be dirty")
	}
}

func TestSetParent_PreservesLayerIdentity(t *testing.T) {
	owner := &PipelineOwner{}

	parent1 := &boundaryRenderBox{}
	parent1.SetSelf(parent1)
	parent1.SetOwner(owner)

	parent2 := &boundaryRenderBox{}
	parent2.SetSelf(parent2)
	parent2.SetOwner(owner)

	child := &boundaryRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.size = graphics.Size{Width: 50, Height: 50}
	child.SetParent(parent1)

	// Force layer creation
	layerBefore := child.EnsureLayer()

	// Reparent
	child.SetParent(parent2)

	layerAfter := child.Layer()
	if layerAfter != layerBefore {
		t.Error("reparenting should preserve layer identity (stable pointer)")
	}
	if layerAfter != nil && !layerAfter.Dirty {
		t.Error("layer should be dirty after reparenting")
	}
}

func TestSetParent_ToNilMarksOldParentDirty(t *testing.T) {
	owner := &PipelineOwner{}

	parent := &boundaryRenderBox{}
	parent.SetSelf(parent)
	parent.SetOwner(owner)
	parent.size = graphics.Size{Width: 100, Height: 100}

	child := &plainRenderBox{}
	child.SetSelf(child)
	child.SetOwner(owner)
	child.SetParent(parent)

	parent.ClearNeedsPaint()

	child.SetParent(nil)

	if !parent.NeedsPaint() {
		t.Error("old parent should be dirty when child detached (parent set to nil)")
	}
}

// --- EnsureLayer tests ---

func TestEnsureLayer_CreatesWithDirtyTrue(t *testing.T) {
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.size = graphics.Size{Width: 30, Height: 40}

	layer := box.EnsureLayer()
	if layer == nil {
		t.Fatal("EnsureLayer should create a layer")
	}
	if !layer.Dirty {
		t.Error("newly created layer should be dirty")
	}
	if layer.Size.Width != 30 || layer.Size.Height != 40 {
		t.Errorf("layer size = %v, want {30, 40}", layer.Size)
	}
}

// --- SetLayerContent tests ---

func TestSetLayerContent_CreatesLayerIfNil(t *testing.T) {
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.size = graphics.Size{Width: 20, Height: 20}

	rec := &graphics.PictureRecorder{}
	rec.BeginRecording(graphics.Size{Width: 20, Height: 20})
	dl := rec.EndRecording()

	box.SetLayerContent(dl)

	if box.Layer() == nil {
		t.Fatal("SetLayerContent should create layer if nil")
	}
	if box.Layer().Content != dl {
		t.Error("layer content should be the provided display list")
	}
	if box.Layer().Dirty {
		t.Error("layer should be clean after SetLayerContent (SetContent clears dirty)")
	}
}

func TestSetLayerContent_SyncsSizeFromRenderBox(t *testing.T) {
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.size = graphics.Size{Width: 75, Height: 50}

	rec := &graphics.PictureRecorder{}
	rec.BeginRecording(graphics.Size{Width: 10, Height: 10}) // recorder size differs
	dl := rec.EndRecording()

	box.SetLayerContent(dl)

	// Layer size should match the render box size, not the recorder size
	if box.Layer().Size.Width != 75 || box.Layer().Size.Height != 50 {
		t.Errorf("layer size = %v, want {75, 50}", box.Layer().Size)
	}
}

// --- Dispose tests ---

func TestDispose_ClearsLayer(t *testing.T) {
	box := &boundaryRenderBox{}
	box.SetSelf(box)
	box.size = graphics.Size{Width: 50, Height: 50}
	box.EnsureLayer()

	box.Dispose()

	if box.Layer() != nil {
		t.Error("layer should be nil after dispose")
	}
}

func TestDispose_IdempotentWhenNoLayer(t *testing.T) {
	box := &plainRenderBox{}
	box.SetSelf(box)

	// Should not panic
	box.Dispose()
	box.Dispose()
}

// --- Layer.String tests ---

func TestLayer_String(t *testing.T) {
	l := &graphics.Layer{Dirty: true, Size: graphics.Size{Width: 100, Height: 200}}
	s := l.String()
	if s == "" {
		t.Error("Layer.String() should return non-empty string")
	}

	var nilLayer *graphics.Layer
	if nilLayer.String() != "Layer(nil)" {
		t.Errorf("nil Layer.String() = %q, want %q", nilLayer.String(), "Layer(nil)")
	}
}

// --- Layer.Dispose idempotency ---

func TestLayer_Dispose_Idempotent(t *testing.T) {
	layer := &graphics.Layer{Dirty: true, Size: graphics.Size{Width: 10, Height: 10}}

	rec := &graphics.PictureRecorder{}
	rec.BeginRecording(graphics.Size{Width: 10, Height: 10})
	layer.SetContent(rec.EndRecording())

	layer.Dispose()
	layer.Dispose() // should not panic
	if layer.Content != nil {
		t.Error("Content should be nil after dispose")
	}
}
