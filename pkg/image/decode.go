package image

import (
	"bytes"
	"image"
	"io"

	// Register the formats Decode supports.
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Decode decodes an image in any registered format, as the standard
// library's [image.Decode] does, and returns it upright: a JPEG's EXIF
// orientation (set by cameras, which store pixels as the sensor saw them)
// is applied, as browsers do. The returned image's bounds are its
// displayed size. format is the format name, such as "jpeg".
//
// Use Decode, not [image.Decode], for photos: from an image picker, the
// camera, a file or an asset. [Loader] decodes with it.
func Decode(r io.Reader) (img image.Image, format string, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	return decodeBytes(data)
}

// decodeBytes is [Decode] for an encoded image already in memory.
func decodeBytes(data []byte) (image.Image, string, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	if format == "jpeg" {
		img = jpegOrientation(data).apply(img)
	}
	return img, format, nil
}
