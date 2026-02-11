// Package icongen generates app icons for Android and iOS from a single source image.
package icongen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// IconSource holds a decoded source image for icon generation.
type IconSource struct {
	img image.Image
}

// LoadSource loads the icon source image. If iconPath is empty, the embedded
// default icon is used. Otherwise the image is read from projectRoot/iconPath.
// The source image must be at least 1024x1024.
func LoadSource(projectRoot, iconPath string) (*IconSource, error) {
	var img image.Image
	var err error

	if iconPath == "" {
		img, err = png.Decode(bytes.NewReader(defaultIconPNG))
		if err != nil {
			return nil, fmt.Errorf("failed to decode embedded default icon: %w", err)
		}
	} else {
		path := filepath.Join(projectRoot, iconPath)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open icon %s: %w", path, err)
		}
		defer f.Close()

		img, _, err = image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("failed to decode icon %s: %w", path, err)
		}
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 1024 || h < 1024 {
		return nil, fmt.Errorf("icon must be at least 1024x1024 (got %dx%d)", w, h)
	}
	if w != h {
		return nil, fmt.Errorf("icon must be square (got %dx%d)", w, h)
	}

	return &IconSource{img: img}, nil
}

// androidDensity describes a single Android mipmap density bucket.
type androidDensity struct {
	name         string
	legacySize   int
	adaptiveSize int
}

var androidDensities = []androidDensity{
	{"mipmap-mdpi", 48, 108},
	{"mipmap-hdpi", 72, 162},
	{"mipmap-xhdpi", 96, 216},
	{"mipmap-xxhdpi", 144, 324},
	{"mipmap-xxxhdpi", 192, 432},
}

const adaptiveIconXML = `<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
  <background android:drawable="@mipmap/ic_launcher_adaptive_back"/>
  <foreground android:drawable="@mipmap/ic_launcher_adaptive_fore"/>
</adaptive-icon>`

// GenerateAndroid generates all mipmap directories and the adaptive icon XML
// into resDir. bgColor sets the adaptive icon background (default "#FFFFFF").
func (s *IconSource) GenerateAndroid(resDir, bgColor string) error {
	if bgColor == "" {
		bgColor = "#FFFFFF"
	}
	bg, err := parseHexColor(bgColor)
	if err != nil {
		return fmt.Errorf("invalid icon_background color %q: %w", bgColor, err)
	}

	for _, d := range androidDensities {
		dir := filepath.Join(resDir, d.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}

		// Legacy launcher icon
		legacy := resizeImage(s.img, d.legacySize)
		if err := writePNG(filepath.Join(dir, "ic_launcher.png"), legacy); err != nil {
			return err
		}

		// Adaptive foreground (icon centered in inner 2/3 of canvas for safe zone,
		// canvas filled with bg color to avoid interpolation fringe artifacts)
		fore := adaptiveForeground(s.img, d.adaptiveSize, bg)
		if err := writePNG(filepath.Join(dir, "ic_launcher_adaptive_fore.png"), fore); err != nil {
			return err
		}

		// Adaptive background (solid color)
		back := solidColorImage(d.adaptiveSize, bg)
		if err := writePNG(filepath.Join(dir, "ic_launcher_adaptive_back.png"), back); err != nil {
			return err
		}
	}

	// Write adaptive-icon XML for API 26+
	anydpiDir := filepath.Join(resDir, "mipmap-anydpi-v26")
	if err := os.MkdirAll(anydpiDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", anydpiDir, err)
	}
	if err := os.WriteFile(filepath.Join(anydpiDir, "ic_launcher.xml"), []byte(adaptiveIconXML), 0o644); err != nil {
		return fmt.Errorf("failed to write adaptive icon XML: %w", err)
	}

	return nil
}

const assetCatalogContentsJSON = `{
  "info": { "version": 1, "author": "xcode" }
}`

const appIconContentsJSON = `{
  "images": [
    {
      "filename": "icon.png",
      "idiom": "universal",
      "platform": "ios",
      "size": "1024x1024"
    }
  ],
  "info": { "version": 1, "author": "xcode" }
}`

const launchImageContentsJSON = `{
  "images": [
    {
      "filename": "launch_icon.png",
      "idiom": "universal",
      "scale": "1x"
    },
    {
      "filename": "launch_icon@2x.png",
      "idiom": "universal",
      "scale": "2x"
    },
    {
      "filename": "launch_icon@3x.png",
      "idiom": "universal",
      "scale": "3x"
    }
  ],
  "info": { "version": 1, "author": "xcode" }
}`

// GenerateIOS generates the Assets.xcassets structure into assetDir,
// including both AppIcon.appiconset and LaunchImage.imageset.
func (s *IconSource) GenerateIOS(assetDir string) error {
	// Contents.json for the asset catalog root
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", assetDir, err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "Contents.json"), []byte(assetCatalogContentsJSON+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write asset catalog Contents.json: %w", err)
	}

	// AppIcon.appiconset
	appIconDir := filepath.Join(assetDir, "AppIcon.appiconset")
	if err := os.MkdirAll(appIconDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", appIconDir, err)
	}
	if err := os.WriteFile(filepath.Join(appIconDir, "Contents.json"), []byte(appIconContentsJSON+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write AppIcon Contents.json: %w", err)
	}
	if err := writePNG(filepath.Join(appIconDir, "icon.png"), resizeImage(s.img, 1024)); err != nil {
		return err
	}

	// LaunchImage.imageset (used by LaunchScreen.storyboard)
	if err := s.generateLaunchImageSet(assetDir); err != nil {
		return err
	}

	return nil
}

// generateLaunchImageSet writes a LaunchImage.imageset into the given asset
// catalog directory with 1x/2x/3x variants for the launch screen storyboard.
func (s *IconSource) generateLaunchImageSet(assetDir string) error {
	launchDir := filepath.Join(assetDir, "LaunchImage.imageset")
	if err := os.MkdirAll(launchDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", launchDir, err)
	}
	if err := os.WriteFile(filepath.Join(launchDir, "Contents.json"), []byte(launchImageContentsJSON+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write LaunchImage Contents.json: %w", err)
	}

	// 120pt icon: 1x=120, 2x=240, 3x=360
	for _, s2 := range []struct {
		name string
		size int
	}{
		{"launch_icon.png", 120},
		{"launch_icon@2x.png", 240},
		{"launch_icon@3x.png", 360},
	} {
		if err := writePNG(filepath.Join(launchDir, s2.name), resizeImage(s.img, s2.size)); err != nil {
			return err
		}
	}

	return nil
}

// GenerateIconPNG writes a single 1024x1024 icon PNG to the given path.
// Used by xtool, which takes a single PNG via iconPath in xtool.yml.
func (s *IconSource) GenerateIconPNG(path string) error {
	icon := resizeImage(s.img, 1024)
	return writePNG(path, icon)
}

// resizeImage scales src to size x size using high-quality CatmullRom interpolation.
func resizeImage(src image.Image, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// adaptiveForeground creates an adaptive icon foreground: the source image
// centered within the inner 2/3 of the canvas. Android adaptive icons use a
// 108dp canvas where the safe zone is the inner 72dp (66.67%). Content outside
// the safe zone may be clipped by the device's icon mask. The canvas is filled
// with bgColor to avoid interpolation fringe artifacts at the image boundary.
func adaptiveForeground(src image.Image, canvasSize int, bgColor color.NRGBA) *image.NRGBA {
	dst := solidColorImage(canvasSize, bgColor)
	iconSize := canvasSize * 2 / 3
	offset := (canvasSize - iconSize) / 2
	dstRect := image.Rect(offset, offset, offset+iconSize, offset+iconSize)
	draw.CatmullRom.Scale(dst, dstRect, src, src.Bounds(), draw.Over, nil)
	return dst
}

// solidColorImage creates a size x size image filled with the given color.
func solidColorImage(size int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	r, g, b, a := c.RGBA()
	nrgba := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	for y := range size {
		for x := range size {
			img.SetNRGBA(x, y, nrgba)
		}
	}
	return img
}

// parseHexColor parses a hex color string (#RGB or #RRGGBB) into a color.NRGBA.
func parseHexColor(hex string) (color.NRGBA, error) {
	if len(hex) == 0 || hex[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("color must start with '#'")
	}
	hex = hex[1:]

	switch len(hex) {
	case 3:
		r, g, b := hex[0], hex[1], hex[2]
		rr, err := parseHexByte(r, r)
		if err != nil {
			return color.NRGBA{}, err
		}
		gg, err := parseHexByte(g, g)
		if err != nil {
			return color.NRGBA{}, err
		}
		bb, err := parseHexByte(b, b)
		if err != nil {
			return color.NRGBA{}, err
		}
		return color.NRGBA{R: rr, G: gg, B: bb, A: 255}, nil

	case 6:
		rr, err := parseHexByte(hex[0], hex[1])
		if err != nil {
			return color.NRGBA{}, err
		}
		gg, err := parseHexByte(hex[2], hex[3])
		if err != nil {
			return color.NRGBA{}, err
		}
		bb, err := parseHexByte(hex[4], hex[5])
		if err != nil {
			return color.NRGBA{}, err
		}
		return color.NRGBA{R: rr, G: gg, B: bb, A: 255}, nil

	default:
		return color.NRGBA{}, fmt.Errorf("invalid hex color length (expected 3 or 6 hex digits)")
	}
}

func parseHexByte(hi, lo byte) (uint8, error) {
	h, ok1 := hexVal(hi)
	l, ok2 := hexVal(lo)
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("invalid hex digit")
	}
	return h<<4 | l, nil
}

func hexVal(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func writePNG(path string, img image.Image) (retErr error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close %s: %w", path, cerr)
		}
	}()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("failed to encode %s: %w", path, err)
	}
	return nil
}
