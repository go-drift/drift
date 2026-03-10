package engine

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

func TestMarshalBinaryEmpty(t *testing.T) {
	snap := &FrameSnapshot{Views: nil}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	if len(data) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(data))
	}
	version := binary.LittleEndian.Uint32(data[0:4])
	viewCount := binary.LittleEndian.Uint32(data[4:8])
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}
	if viewCount != 0 {
		t.Fatalf("expected viewCount 0, got %d", viewCount)
	}
}

func TestMarshalBinaryGolden(t *testing.T) {
	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{
				ViewID:        42,
				X:             10,
				Y:             20,
				Width:         300,
				Height:        400,
				Visible:       true,
				VisibleLeft:   10,
				VisibleTop:    20,
				VisibleRight:  310,
				VisibleBottom: 420,
			},
		},
	}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	// Header (8) + fixed view (60) = 68 bytes
	if len(data) != 68 {
		t.Fatalf("expected 68 bytes, got %d", len(data))
	}

	off := 0
	// Header
	if binary.LittleEndian.Uint32(data[off:]) != 1 {
		t.Fatal("version mismatch")
	}
	off += 4
	if binary.LittleEndian.Uint32(data[off:]) != 1 {
		t.Fatal("viewCount mismatch")
	}
	off += 4

	// viewId
	viewID := int64(binary.LittleEndian.Uint64(data[off:]))
	off += 8
	if viewID != 42 {
		t.Fatalf("viewId: expected 42, got %d", viewID)
	}

	// x, y, width, height
	readF32 := func() float32 {
		v := math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))
		off += 4
		return v
	}
	if v := readF32(); v != 10 {
		t.Fatalf("x: expected 10, got %v", v)
	}
	if v := readF32(); v != 20 {
		t.Fatalf("y: expected 20, got %v", v)
	}
	if v := readF32(); v != 300 {
		t.Fatalf("width: expected 300, got %v", v)
	}
	if v := readF32(); v != 400 {
		t.Fatalf("height: expected 400, got %v", v)
	}

	// clip (all zero since no clip)
	for i := range 4 {
		if v := readF32(); v != 0 {
			t.Fatalf("clip[%d]: expected 0, got %v", i, v)
		}
	}

	// visible rect
	if v := readF32(); v != 10 {
		t.Fatalf("visibleLeft: expected 10, got %v", v)
	}
	if v := readF32(); v != 20 {
		t.Fatalf("visibleTop: expected 20, got %v", v)
	}
	if v := readF32(); v != 310 {
		t.Fatalf("visibleRight: expected 310, got %v", v)
	}
	if v := readF32(); v != 420 {
		t.Fatalf("visibleBottom: expected 420, got %v", v)
	}

	// flags: bit1 (visible) set, bit0 (hasClip) unset
	flags := data[off]
	off++
	if flags != 2 {
		t.Fatalf("flags: expected 2 (visible), got %d", flags)
	}

	// reserved
	off++

	// pathCount
	pathCount := binary.LittleEndian.Uint16(data[off:])
	if pathCount != 0 {
		t.Fatalf("pathCount: expected 0, got %d", pathCount)
	}
}

func TestMarshalBinaryWithClip(t *testing.T) {
	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{
				ViewID:        1,
				HasClip:       true,
				ClipLeft:      5,
				ClipTop:       10,
				ClipRight:     100,
				ClipBottom:    200,
				Visible:       true,
				VisibleLeft:   5,
				VisibleTop:    10,
				VisibleRight:  100,
				VisibleBottom: 200,
			},
		},
	}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	off := 8 + 8 + 16 // header + viewId + pos/size
	readF32At := func(offset int) float32 {
		return math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
	}

	// clip values
	if v := readF32At(off); v != 5 {
		t.Fatalf("clipLeft: expected 5, got %v", v)
	}
	if v := readF32At(off + 4); v != 10 {
		t.Fatalf("clipTop: expected 10, got %v", v)
	}
	if v := readF32At(off + 8); v != 100 {
		t.Fatalf("clipRight: expected 100, got %v", v)
	}
	if v := readF32At(off + 12); v != 200 {
		t.Fatalf("clipBottom: expected 200, got %v", v)
	}

	// flags: bit0 (hasClip) + bit1 (visible) = 3
	flagsOff := 8 + 8 + 48 // header + viewId + 12*float32
	flags := data[flagsOff]
	if flags != 3 {
		t.Fatalf("flags: expected 3, got %d", flags)
	}
}

func TestMarshalBinaryWithOcclusionPaths(t *testing.T) {
	p1 := graphics.NewPath()
	p1.MoveTo(10, 20)
	p1.LineTo(30, 40)
	p1.Close()

	p2 := graphics.NewPath()
	p2.MoveTo(0, 0)
	p2.QuadTo(10, 20, 30, 40)
	p2.CubicTo(1, 2, 3, 4, 5, 6)
	p2.Close()

	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{
				ViewID:         1,
				Visible:        true,
				OcclusionPaths: []*graphics.Path{p1, p2},
			},
		},
	}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	off := 8 + 60 // header + fixed view part

	// pathCount should be at offset 8+56 (within fixed part)
	pathCountOff := 8 + 58 // header(8) + viewId(8) + 12*float32(48) + flags(1) + reserved(1)
	pathCount := binary.LittleEndian.Uint16(data[pathCountOff:])
	if pathCount != 2 {
		t.Fatalf("pathCount: expected 2, got %d", pathCount)
	}

	readU16 := func() uint16 {
		v := binary.LittleEndian.Uint16(data[off:])
		off += 2
		return v
	}
	readF32 := func() float32 {
		v := math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))
		off += 4
		return v
	}

	// Path 1: MoveTo(10,20), LineTo(30,40), Close
	cmdCount1 := readU16()
	if cmdCount1 != 3 {
		t.Fatalf("path1 cmdCount: expected 3, got %d", cmdCount1)
	}

	// MoveTo
	if data[off] != 0 || data[off+1] != 2 {
		t.Fatalf("path1 cmd0: expected op=0 argCount=2, got %d %d", data[off], data[off+1])
	}
	off += 2
	if v := readF32(); v != 10 {
		t.Fatalf("path1 MoveTo x: expected 10, got %v", v)
	}
	if v := readF32(); v != 20 {
		t.Fatalf("path1 MoveTo y: expected 20, got %v", v)
	}

	// LineTo
	if data[off] != 1 || data[off+1] != 2 {
		t.Fatalf("path1 cmd1: expected op=1 argCount=2, got %d %d", data[off], data[off+1])
	}
	off += 2
	if v := readF32(); v != 30 {
		t.Fatalf("path1 LineTo x: expected 30, got %v", v)
	}
	if v := readF32(); v != 40 {
		t.Fatalf("path1 LineTo y: expected 40, got %v", v)
	}

	// Close
	if data[off] != 4 || data[off+1] != 0 {
		t.Fatalf("path1 cmd2: expected op=4 argCount=0, got %d %d", data[off], data[off+1])
	}
	off += 2

	// Path 2: MoveTo(0,0), QuadTo(10,20,30,40), CubicTo(1,2,3,4,5,6), Close
	cmdCount2 := readU16()
	if cmdCount2 != 4 {
		t.Fatalf("path2 cmdCount: expected 4, got %d", cmdCount2)
	}

	// MoveTo
	if data[off] != 0 || data[off+1] != 2 {
		t.Fatalf("path2 cmd0: expected op=0 argCount=2")
	}
	off += 2
	readF32() // x=0
	readF32() // y=0

	// QuadTo
	if data[off] != 2 || data[off+1] != 4 {
		t.Fatalf("path2 cmd1: expected op=2 argCount=4, got %d %d", data[off], data[off+1])
	}
	off += 2
	for _, expected := range []float32{10, 20, 30, 40} {
		if v := readF32(); v != expected {
			t.Fatalf("path2 QuadTo arg: expected %v, got %v", expected, v)
		}
	}

	// CubicTo
	if data[off] != 3 || data[off+1] != 6 {
		t.Fatalf("path2 cmd2: expected op=3 argCount=6, got %d %d", data[off], data[off+1])
	}
	off += 2
	for _, expected := range []float32{1, 2, 3, 4, 5, 6} {
		if v := readF32(); v != expected {
			t.Fatalf("path2 CubicTo arg: expected %v, got %v", expected, v)
		}
	}

	// Close
	if data[off] != 4 || data[off+1] != 0 {
		t.Fatalf("path2 cmd3: expected op=4 argCount=0")
	}
	off += 2

	// Verify we consumed all bytes
	if off != len(data) {
		t.Fatalf("expected to consume %d bytes, consumed %d", len(data), off)
	}
}

func TestMarshalBinaryMultipleViews(t *testing.T) {
	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{ViewID: 1, X: 10, Y: 20, Width: 100, Height: 200, Visible: true},
			{ViewID: 2, X: 30, Y: 40, Width: 150, Height: 250, Visible: false},
			{ViewID: 3, X: 50, Y: 60, Width: 200, Height: 300, Visible: true, HasClip: true, ClipLeft: 5},
		},
	}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	// Header
	viewCount := binary.LittleEndian.Uint32(data[4:8])
	if viewCount != 3 {
		t.Fatalf("viewCount: expected 3, got %d", viewCount)
	}

	// Verify each view's viewID at the correct offset
	off := 8
	for _, expectedID := range []int64{1, 2, 3} {
		viewID := int64(binary.LittleEndian.Uint64(data[off:]))
		if viewID != expectedID {
			t.Fatalf("viewId at offset %d: expected %d, got %d", off, expectedID, viewID)
		}
		off += 60 // skip fixed part
	}

	// Total: 8 + 3*60 = 188
	if len(data) != 188 {
		t.Fatalf("expected 188 bytes, got %d", len(data))
	}
}

func TestMarshalBinaryFloat32Precision(t *testing.T) {
	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{
				ViewID: 1,
				X:      math.Pi,
				Y:      math.E,
			},
		},
	}
	bp := MarshalBinary(snap)
	defer PutSnapshotBuffer(bp)
	data := *bp

	off := 8 + 8 // header + viewId
	xBits := binary.LittleEndian.Uint32(data[off:])
	yBits := binary.LittleEndian.Uint32(data[off+4:])

	expectedX := math.Float32bits(float32(math.Pi))
	expectedY := math.Float32bits(float32(math.E))

	if xBits != expectedX {
		t.Fatalf("X bits: expected %08x, got %08x", expectedX, xBits)
	}
	if yBits != expectedY {
		t.Fatalf("Y bits: expected %08x, got %08x", expectedY, yBits)
	}
}

func TestMarshalBinaryPoolReuse(t *testing.T) {
	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{ViewID: 1, X: 100, Visible: true},
		},
	}

	// First call
	bp1 := MarshalBinary(snap)
	data1 := append([]byte(nil), (*bp1)...)
	PutSnapshotBuffer(bp1)

	// Second call (reuses pooled buffer)
	snap.Views[0].X = 200
	bp2 := MarshalBinary(snap)
	data2 := append([]byte(nil), (*bp2)...)
	PutSnapshotBuffer(bp2)

	// Verify both are correctly encoded (not corrupted by reuse)
	off := 8 + 8 // header + viewId
	x1 := math.Float32frombits(binary.LittleEndian.Uint32(data1[off:]))
	x2 := math.Float32frombits(binary.LittleEndian.Uint32(data2[off:]))

	if x1 != 100 {
		t.Fatalf("first encode X: expected 100, got %v", x1)
	}
	if x2 != 200 {
		t.Fatalf("second encode X: expected 200, got %v", x2)
	}
}

func BenchmarkMarshalBinary(b *testing.B) {
	p := graphics.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(100, 0)
	p.LineTo(100, 100)
	p.LineTo(0, 100)
	p.Close()

	snap := &FrameSnapshot{
		Views: []ViewSnapshot{
			{
				ViewID: 1, X: 10, Y: 20, Width: 300, Height: 400,
				HasClip: true, ClipLeft: 10, ClipTop: 20, ClipRight: 310, ClipBottom: 420,
				Visible: true, VisibleLeft: 10, VisibleTop: 20, VisibleRight: 310, VisibleBottom: 420,
				OcclusionPaths: []*graphics.Path{p, p},
			},
			{
				ViewID: 2, X: 50, Y: 60, Width: 200, Height: 150,
				Visible: true, VisibleLeft: 50, VisibleTop: 60, VisibleRight: 250, VisibleBottom: 210,
				OcclusionPaths: []*graphics.Path{p, p},
			},
			{
				ViewID: 3, X: 100, Y: 100, Width: 400, Height: 300,
				Visible: true, VisibleLeft: 100, VisibleTop: 100, VisibleRight: 500, VisibleBottom: 400,
				OcclusionPaths: []*graphics.Path{p, p},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bp := MarshalBinary(snap)
		PutSnapshotBuffer(bp)
	}
}
