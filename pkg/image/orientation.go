package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// orientation is an EXIF orientation (TIFF tag 0x0112): which visual sides
// of the upright image the stored pixels' 0th row and 0th column are.
// Valid values are 1 to 8; [jpegOrientation] yields no others.
type orientation uint8

const (
	orientationNormal     orientation = 1 // 0th row top, 0th column left
	orientationFlipH      orientation = 2 // 0th row top, 0th column right
	orientationRotate180  orientation = 3 // 0th row bottom, 0th column right
	orientationFlipV      orientation = 4 // 0th row bottom, 0th column left
	orientationTranspose  orientation = 5 // 0th row left, 0th column top
	orientationRotate90   orientation = 6 // 0th row right, 0th column top
	orientationTransverse orientation = 7 // 0th row right, 0th column bottom
	orientationRotate270  orientation = 8 // 0th row left, 0th column bottom
)

// exifHeader starts the payload of a JPEG APP1 segment holding EXIF data,
// followed by a TIFF structure.
var exifHeader = []byte("Exif\x00\x00")

// jpegOrientation returns the EXIF orientation of the JPEG data: the first
// EXIF APP1 segment's IFD0 orientation tag. Data without one, or with any
// malformed structure on the way, is orientationNormal. Reads stay within
// data, and each step advances, so any input terminates.
func jpegOrientation(data []byte) orientation {
	const (
		markerSOI  = 0xD8
		markerEOI  = 0xD9
		markerSOS  = 0xDA
		markerAPP1 = 0xE1
		markerTEM  = 0x01
		markerRST0 = 0xD0
		markerRST7 = 0xD7
	)
	if len(data) < 2 || data[0] != 0xFF || data[1] != markerSOI {
		return orientationNormal
	}
	i := 2
	for {
		if i >= len(data) || data[i] != 0xFF {
			return orientationNormal
		}
		// A marker is 0xFF and a code; extra 0xFF bytes before it are fill.
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			return orientationNormal
		}
		marker := data[i]
		i++
		switch {
		case marker == markerSOS || marker == markerEOI:
			// Metadata segments all precede the scan.
			return orientationNormal
		case marker == markerTEM || (marker >= markerRST0 && marker <= markerRST7):
			continue // standalone: no length or payload
		}
		// The length counts itself, not the marker.
		if len(data)-i < 2 {
			return orientationNormal
		}
		n := int(binary.BigEndian.Uint16(data[i:]))
		if n < 2 || len(data)-i < n {
			return orientationNormal
		}
		payload := data[i+2 : i+n]
		i += n
		if marker == markerAPP1 && bytes.HasPrefix(payload, exifHeader) {
			return tiffOrientation(payload[len(exifHeader):])
		}
	}
}

// tiffOrientation returns the orientation tag of IFD0 in the TIFF
// structure tiff, or orientationNormal if it is missing or malformed.
func tiffOrientation(tiff []byte) orientation {
	const (
		tagOrientation = 0x0112
		typeShort      = 3
		entrySize      = 12
	)
	if len(tiff) < 8 {
		return orientationNormal
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return orientationNormal
	}
	if order.Uint16(tiff[2:]) != 42 {
		return orientationNormal
	}
	// uint64: the offset is a uint32, which may not fit an int.
	ifd := uint64(order.Uint32(tiff[4:]))
	if ifd+2 > uint64(len(tiff)) {
		return orientationNormal
	}
	count := int(order.Uint16(tiff[ifd:]))
	entries := tiff[ifd+2:]
	for k := 0; k < count && (k+1)*entrySize <= len(entries); k++ {
		entry := entries[k*entrySize : (k+1)*entrySize]
		if order.Uint16(entry) != tagOrientation {
			continue
		}
		// One SHORT, stored in the first two bytes of the value field.
		if order.Uint16(entry[2:]) != typeShort || order.Uint32(entry[4:]) != 1 {
			return orientationNormal
		}
		if v := order.Uint16(entry[8:]); v >= 1 && v <= 8 {
			return orientation(v)
		}
		return orientationNormal
	}
	return orientationNormal
}

// apply returns src upright. For orientationNormal that is src itself;
// otherwise it is a new [*image.RGBA], with width and height swapped for
// orientations 5 to 8, written in one pass over src's pixels.
func (o orientation) apply(src image.Image) image.Image {
	if o == orientationNormal {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= orientationTranspose {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	if w == 0 || h == 0 {
		return dst
	}

	// Source pixel (x, y), relative to b.Min, lands at
	// dst.Pix[origin + x*colStep + y*rowStep].
	const px = 4
	s := dst.Stride
	var origin, colStep, rowStep int
	switch o {
	case orientationFlipH:
		origin, colStep, rowStep = (w-1)*px, -px, s
	case orientationRotate180:
		origin, colStep, rowStep = (h-1)*s+(w-1)*px, -px, -s
	case orientationFlipV:
		origin, colStep, rowStep = (h-1)*s, px, -s
	case orientationTranspose:
		origin, colStep, rowStep = 0, s, px
	case orientationRotate90:
		origin, colStep, rowStep = (h-1)*px, s, -px
	case orientationTransverse:
		origin, colStep, rowStep = (w-1)*s+(h-1)*px, -s, -px
	case orientationRotate270:
		origin, colStep, rowStep = (w-1)*s, -s, px
	default:
		panic(fmt.Sprintf("image: invalid EXIF orientation %d", o))
	}

	pix := dst.Pix
	// The JPEG decoder returns colour photos as YCbCr: read its planes
	// directly, with row offsets hoisted. Shifting indexes chroma as
	// YCbCr.COffset's division does only for non-negative coordinates.
	if src, ok := src.(*image.YCbCr); ok && b.Min.X >= 0 && b.Min.Y >= 0 {
		if hs, vs, ok := chromaShifts(src.SubsampleRatio); ok {
			for y := range h {
				yRow := src.Y[y*src.YStride:]
				c := ((b.Min.Y+y)>>vs - b.Min.Y>>vs) * src.CStride
				cb, cr := src.Cb[c:], src.Cr[c:]
				d := origin + y*rowStep
				for x := range w {
					ci := (b.Min.X+x)>>hs - b.Min.X>>hs
					r, g, bl := color.YCbCrToRGB(yRow[x], cb[ci], cr[ci])
					p := pix[d : d+px : d+px]
					p[0], p[1], p[2], p[3] = r, g, bl, 0xFF
					d += colStep
				}
			}
			return dst
		}
	}
	for y := range h {
		d := origin + y*rowStep
		for x := range w {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			p := pix[d : d+px : d+px]
			p[0], p[1], p[2], p[3] = uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
			d += colStep
		}
	}
	return dst
}

// chromaShifts returns how far to shift a pixel's x and y right to index
// its chroma sample under ratio, and false for an unknown ratio.
func chromaShifts(ratio image.YCbCrSubsampleRatio) (h, v uint, ok bool) {
	switch ratio {
	case image.YCbCrSubsampleRatio444:
		return 0, 0, true
	case image.YCbCrSubsampleRatio422:
		return 1, 0, true
	case image.YCbCrSubsampleRatio420:
		return 1, 1, true
	case image.YCbCrSubsampleRatio440:
		return 0, 1, true
	case image.YCbCrSubsampleRatio411:
		return 2, 0, true
	case image.YCbCrSubsampleRatio410:
		return 2, 1, true
	}
	return 0, 0, false
}
