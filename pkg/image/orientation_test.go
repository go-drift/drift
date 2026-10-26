package image

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// side is a visual side of the upright image.
type side int

const (
	top side = iota
	bottom
	left
	right
)

// exifSpec is the EXIF 2.3 definition of each orientation: the visual
// sides that the stored 0th row and 0th column are. Tests derive expected
// pixel placement from it, independently of the implementation.
var exifSpec = map[orientation]struct{ row, col side }{
	1: {top, left},
	2: {top, right},
	3: {bottom, right},
	4: {bottom, left},
	5: {left, top},
	6: {right, top},
	7: {right, bottom},
	8: {left, bottom},
}

// displayed returns where stored pixel (x, y) of a w by h image appears
// in the upright image under orientation o, per exifSpec.
func displayed(o orientation, x, y, w, h int) (dx, dy int) {
	spec := exifSpec[o]
	dw, dh := w, h
	if spec.row == left || spec.row == right {
		dw, dh = h, w
	}
	// fromEdge positions index i measured from side s.
	fromEdge := func(s side, i int) int {
		switch s {
		case bottom:
			return dh - 1 - i
		case right:
			return dw - 1 - i
		}
		return i
	}
	// Stored row y lies along the row side; stored column x along the
	// column side.
	if spec.row == top || spec.row == bottom {
		return fromEdge(spec.col, x), fromEdge(spec.row, y)
	}
	return fromEdge(spec.row, y), fromEdge(spec.col, x)
}

func TestOrientationApply_PixelExact(t *testing.T) {
	// 3x2 with a unique colour per pixel, offset so Bounds().Min != 0.
	const w, h = 3, 2
	full := image.NewRGBA(image.Rect(0, 0, w+5, h+7))
	src := full.SubImage(image.Rect(5, 7, 5+w, 7+h)).(*image.RGBA)
	pixel := func(x, y int) color.RGBA { return color.RGBA{uint8(10 + x), uint8(20 + y), 30, 255} }
	for y := range h {
		for x := range w {
			src.SetRGBA(5+x, 7+y, pixel(x, y))
		}
	}

	for o := orientation(1); o <= 8; o++ {
		got := o.apply(src)
		wantW, wantH := w, h
		if o >= orientationTranspose {
			wantW, wantH = h, w
		}
		if b := got.Bounds(); b.Dx() != wantW || b.Dy() != wantH {
			t.Fatalf("orientation %d: size %dx%d, want %dx%d", o, b.Dx(), b.Dy(), wantW, wantH)
		}
		for y := range h {
			for x := range w {
				dx, dy := displayed(o, x, y, w, h)
				b := got.Bounds()
				if c := color.RGBAModel.Convert(got.At(b.Min.X+dx, b.Min.Y+dy)); c != pixel(x, y) {
					t.Errorf("orientation %d: stored (%d,%d) at displayed (%d,%d) is %v, want %v", o, x, y, dx, dy, c, pixel(x, y))
				}
			}
		}
	}
}

func TestOrientationApply_YCbCrMatchesAt(t *testing.T) {
	ratios := []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422,
		image.YCbCrSubsampleRatio420, image.YCbCrSubsampleRatio440,
		image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	}
	for _, ratio := range ratios {
		full := image.NewYCbCr(image.Rect(0, 0, 32, 32), ratio)
		for i := range full.Y {
			full.Y[i] = uint8(i * 7)
		}
		for i := range full.Cb {
			full.Cb[i], full.Cr[i] = uint8(i*13), uint8(255-i*5)
		}
		// Odd offset and size, so chroma rows and columns straddle bounds.
		src := full.SubImage(image.Rect(3, 5, 3+13, 5+7)).(*image.YCbCr)
		b := src.Bounds()
		for o := orientation(2); o <= 8; o++ {
			got := o.apply(src)
			for y := range b.Dy() {
				for x := range b.Dx() {
					dx, dy := displayed(o, x, y, b.Dx(), b.Dy())
					want := color.RGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y))
					if c := got.At(dx, dy); c != want {
						t.Fatalf("%v orientation %d: stored (%d,%d) at (%d,%d) is %v, want %v", ratio, o, x, y, dx, dy, c, want)
					}
				}
			}
		}
	}
}

func TestOrientationApply_NormalReturnsSource(t *testing.T) {
	src := image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio420)
	if got := orientationNormal.apply(src); got != image.Image(src) {
		t.Fatalf("apply(normal) = %T, want the source itself", got)
	}
}

func TestOrientationApply_Empty(t *testing.T) {
	got := orientationRotate90.apply(image.NewRGBA(image.Rect(0, 0, 0, 5)))
	if b := got.Bounds(); b.Dx() != 5 || b.Dy() != 0 {
		t.Fatalf("size %v, want 5x0", b)
	}
}

// quadrantColors are the stored image's quadrants: top-left, top-right,
// bottom-left, bottom-right.
var quadrantColors = [4]color.RGBA{
	{255, 0, 0, 255},
	{0, 255, 0, 255},
	{0, 0, 255, 255},
	{255, 255, 255, 255},
}

// Quadrant edges fall on 16-pixel MCU boundaries so JPEG keeps each
// quadrant's colour clean at its centre.
const quadW, quadH = 64, 32

func quadrantImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, quadW, quadH))
	for y := range quadH {
		for x := range quadW {
			q := 0
			if x >= quadW/2 {
				q++
			}
			if y >= quadH/2 {
				q += 2
			}
			img.SetRGBA(x, y, quadrantColors[q])
		}
	}
	return img
}

// encodeJPEG encodes img and inserts segments right after SOI.
func encodeJPEG(t *testing.T, img image.Image, segments ...[]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	out := append([]byte{}, data[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, data[2:]...)
}

// segment builds a JPEG marker segment.
func segment(marker byte, payload []byte) []byte {
	out := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(out[2:], uint16(len(payload)+2))
	return append(out, payload...)
}

// exifSegment builds an APP1 EXIF segment around tiff.
func exifSegment(tiff []byte) []byte {
	return segment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

// tiffEntry is one IFD entry; value holds a SHORT in its first two bytes.
type tiffEntry struct {
	tag, typ uint16
	count    uint32
	value    uint16
}

// byteOrder is binary.LittleEndian or binary.BigEndian.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// tiffIFD0 builds a TIFF structure whose IFD0 holds entries.
func tiffIFD0(order byteOrder, entries ...tiffEntry) []byte {
	out := make([]byte, 8, 8+2+12*len(entries)+4)
	if order == binary.LittleEndian {
		copy(out, "II")
	} else {
		copy(out, "MM")
	}
	order.PutUint16(out[2:], 42)
	order.PutUint32(out[4:], 8)
	out = order.AppendUint16(out, uint16(len(entries)))
	for _, e := range entries {
		out = order.AppendUint16(out, e.tag)
		out = order.AppendUint16(out, e.typ)
		out = order.AppendUint32(out, e.count)
		out = order.AppendUint16(out, e.value)
		out = append(out, 0, 0)
	}
	return order.AppendUint32(out, 0) // no next IFD
}

func orientationTIFF(order byteOrder, v uint16) []byte {
	return tiffIFD0(order,
		tiffEntry{tag: 0x010F, typ: 2, count: 1}, // Make, to skip past
		tiffEntry{tag: 0x0112, typ: 3, count: 1, value: v},
	)
}

func near(a, b color.Color) bool {
	const tolerance = 24 << 8
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) bool { return x-y < tolerance || y-x < tolerance }
	return d(ar, br) && d(ag, bg) && d(ab, bb)
}

func TestDecode_AppliesJPEGOrientation(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, quadW, quadH))
	rgba := quadrantImage()
	for y := range quadH {
		for x := range quadW {
			gray.Set(x, y, rgba.At(x, y))
		}
	}
	sources := map[string]image.Image{"colour": rgba, "gray": gray}

	for name, src := range sources {
		for _, order := range []byteOrder{binary.LittleEndian, binary.BigEndian} {
			for o := orientation(1); o <= 8; o++ {
				data := encodeJPEG(t, src, exifSegment(orientationTIFF(order, uint16(o))))
				img, format, err := Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatalf("%s %v orientation %d: %v", name, order, o, err)
				}
				if format != "jpeg" {
					t.Fatalf("format %q, want jpeg", format)
				}
				wantW, wantH := quadW, quadH
				if o >= orientationTranspose {
					wantW, wantH = quadH, quadW
				}
				b := img.Bounds()
				if b.Dx() != wantW || b.Dy() != wantH {
					t.Fatalf("%s %v orientation %d: size %dx%d, want %dx%d", name, order, o, b.Dx(), b.Dy(), wantW, wantH)
				}
				// Each stored quadrant centre must land where the spec says.
				for q, centre := range []image.Point{
					{quadW / 4, quadH / 4}, {3 * quadW / 4, quadH / 4},
					{quadW / 4, 3 * quadH / 4}, {3 * quadW / 4, 3 * quadH / 4},
				} {
					dx, dy := displayed(o, centre.X, centre.Y, quadW, quadH)
					want := src.At(centre.X, centre.Y)
					if got := img.At(b.Min.X+dx, b.Min.Y+dy); !near(got, want) {
						t.Errorf("%s %v orientation %d: quadrant %d at (%d,%d) is %v, want %v", name, order, o, q, dx, dy, got, want)
					}
				}
			}
		}
	}
}

func TestDecode_UntaggedJPEGUnchanged(t *testing.T) {
	data := encodeJPEG(t, quadrantImage())
	img, _, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.YCbCr); !ok {
		t.Fatalf("decoded %T, want the decoder's *image.YCbCr", img)
	}
	if b := img.Bounds(); b.Dx() != quadW || b.Dy() != quadH {
		t.Fatalf("size %v", b)
	}
}

func TestDecode_PNG(t *testing.T) {
	img, format, err := Decode(bytes.NewReader(testPNG(3, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" || img.Bounds().Dx() != 3 || img.Bounds().Dy() != 2 {
		t.Fatalf("format %q size %v", format, img.Bounds())
	}
}

func TestDecode_MalformedEXIFDecodesUnrotated(t *testing.T) {
	data := encodeJPEG(t, quadrantImage(), exifSegment([]byte("MM\x00\x2a\xff\xff\xff\xff")))
	img, _, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != quadW || b.Dy() != quadH {
		t.Fatalf("size %v, want %dx%d", b, quadW, quadH)
	}
}

func TestDecode_Invalid(t *testing.T) {
	if _, _, err := Decode(bytes.NewReader([]byte("not an image"))); err == nil {
		t.Fatal("expected an error")
	}
}

// jpegHeader is SOI, segments, then SOS: what jpegOrientation scans.
func jpegHeader(segments ...[]byte) []byte {
	out := []byte{0xFF, 0xD8}
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, 0xFF, 0xDA, 0x00, 0x02)
}

func TestJPEGOrientation(t *testing.T) {
	le, be := binary.LittleEndian, binary.BigEndian
	rotate90 := exifSegment(orientationTIFF(be, 6))
	cases := []struct {
		name string
		data []byte
		want orientation
	}{
		{"empty", nil, orientationNormal},
		{"not a JPEG", testPNG(1, 1), orientationNormal},
		{"SOI only", []byte{0xFF, 0xD8}, orientationNormal},
		{"no EXIF", jpegHeader(segment(0xE0, []byte("JFIF\x00"))), orientationNormal},
		{"little endian", jpegHeader(exifSegment(orientationTIFF(le, 8))), orientationRotate270},
		{"big endian", jpegHeader(rotate90), orientationRotate90},
		{"after APP0 and XMP APP1", jpegHeader(
			segment(0xE0, []byte("JFIF\x00")),
			segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00")),
			exifSegment(orientationTIFF(le, 3)),
		), orientationRotate180},
		{"fill bytes before marker", append([]byte{0xFF, 0xD8, 0xFF, 0xFF}, jpegHeader(rotate90)[3:]...), orientationRotate90},
		{"standalone marker skipped", jpegHeader([]byte{0xFF, 0xD0}, rotate90), orientationRotate90},
		{"after SOS ignored", append(jpegHeader(), rotate90...), orientationNormal},
		{"garbage between segments", jpegHeader([]byte{0x00}, rotate90), orientationNormal},
		{"segment length below 2", jpegHeader([]byte{0xFF, 0xE1, 0x00, 0x01}, rotate90), orientationNormal},
		{"segment length past end", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x'}, orientationNormal},
		{"length truncated", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00}, orientationNormal},
		{"TIFF too short", jpegHeader(exifSegment([]byte("MM\x00\x2a"))), orientationNormal},
		{"bad byte order", jpegHeader(exifSegment(append([]byte("XX"), orientationTIFF(be, 6)[2:]...))), orientationNormal},
		{"bad magic", jpegHeader(exifSegment(append([]byte("MM\x00\x2b"), orientationTIFF(be, 6)[4:]...))), orientationNormal},
		{"IFD offset past end", jpegHeader(exifSegment([]byte("MM\x00\x2a\xff\xff\xff\xff"))), orientationNormal},
		{"IFD offset at end", jpegHeader(exifSegment([]byte("MM\x00\x2a\x00\x00\x00\x08"))), orientationNormal},
		{"entry count past end", jpegHeader(exifSegment([]byte("MM\x00\x2a\x00\x00\x00\x08\xff\xff"))), orientationNormal},
		{"no orientation tag", jpegHeader(exifSegment(tiffIFD0(be, tiffEntry{tag: 0x010F, typ: 2, count: 1}))), orientationNormal},
		{"wrong type", jpegHeader(exifSegment(tiffIFD0(be, tiffEntry{tag: 0x0112, typ: 4, count: 1, value: 6}))), orientationNormal},
		{"wrong count", jpegHeader(exifSegment(tiffIFD0(be, tiffEntry{tag: 0x0112, typ: 3, count: 2, value: 6}))), orientationNormal},
		{"value 0", jpegHeader(exifSegment(orientationTIFF(be, 0))), orientationNormal},
		{"value 9", jpegHeader(exifSegment(orientationTIFF(be, 9))), orientationNormal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jpegOrientation(tc.data); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestJPEGOrientation_EveryTruncation(t *testing.T) {
	data := encodeJPEG(t, quadrantImage(), exifSegment(orientationTIFF(binary.BigEndian, 6)))
	for n := range len(data) {
		if got := jpegOrientation(data[:n]); got != orientationNormal && got != orientationRotate90 {
			t.Fatalf("truncated to %d bytes: got %d", n, got)
		}
	}
}

func FuzzJPEGOrientation(f *testing.F) {
	f.Add(jpegHeader(exifSegment(orientationTIFF(binary.BigEndian, 6))))
	f.Add(jpegHeader(exifSegment(orientationTIFF(binary.LittleEndian, 8))))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, data []byte) {
		if o := jpegOrientation(data); o < 1 || o > 8 {
			t.Fatalf("orientation %d out of range", o)
		}
	})
}

func BenchmarkOrientationApply(b *testing.B) {
	// A 12 MP camera frame.
	src := image.NewYCbCr(image.Rect(0, 0, 4000, 3000), image.YCbCrSubsampleRatio420)
	b.ResetTimer()
	for range b.N {
		orientationRotate90.apply(src)
	}
}
