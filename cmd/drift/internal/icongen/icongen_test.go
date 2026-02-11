package icongen

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSourceDefault(t *testing.T) {
	src, err := LoadSource("", "")
	if err != nil {
		t.Fatalf("LoadSource default: %v", err)
	}
	bounds := src.img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Fatalf("expected 1024x1024, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestLoadSourceCustom(t *testing.T) {
	dir := t.TempDir()
	createTestPNG(t, filepath.Join(dir, "custom.png"), 1024, 1024)

	src, err := LoadSource(dir, "custom.png")
	if err != nil {
		t.Fatalf("LoadSource custom: %v", err)
	}
	bounds := src.img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Fatalf("expected 1024x1024, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestLoadSourceTooSmall(t *testing.T) {
	dir := t.TempDir()
	createTestPNG(t, filepath.Join(dir, "small.png"), 512, 512)

	_, err := LoadSource(dir, "small.png")
	if err == nil {
		t.Fatal("expected error for small image")
	}
}

func TestLoadSourceNonSquare(t *testing.T) {
	dir := t.TempDir()
	createTestPNG(t, filepath.Join(dir, "rect.png"), 1024, 2048)

	_, err := LoadSource(dir, "rect.png")
	if err == nil {
		t.Fatal("expected error for non-square image")
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input   string
		want    color.NRGBA
		wantErr bool
	}{
		{"#FFFFFF", color.NRGBA{255, 255, 255, 255}, false},
		{"#000000", color.NRGBA{0, 0, 0, 255}, false},
		{"#FF0000", color.NRGBA{255, 0, 0, 255}, false},
		{"#FFF", color.NRGBA{255, 255, 255, 255}, false},
		{"#000", color.NRGBA{0, 0, 0, 255}, false},
		{"#f00", color.NRGBA{255, 0, 0, 255}, false},
		{"invalid", color.NRGBA{}, true},
		{"#GG0000", color.NRGBA{}, true},
		{"#12", color.NRGBA{}, true},
		{"", color.NRGBA{}, true},
	}

	for _, tt := range tests {
		got, err := parseHexColor(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseHexColor(%q): expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHexColor(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseHexColor(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGenerateAndroid(t *testing.T) {
	src, err := LoadSource("", "")
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	dir := t.TempDir()
	resDir := filepath.Join(dir, "res")
	if err := src.GenerateAndroid(resDir, ""); err != nil {
		t.Fatalf("GenerateAndroid: %v", err)
	}

	// 5 densities x 3 files each = 15 PNGs + 1 XML = 16 files
	expected := []struct {
		path string
		w, h int
	}{
		{"mipmap-mdpi/ic_launcher.png", 48, 48},
		{"mipmap-mdpi/ic_launcher_adaptive_fore.png", 108, 108},
		{"mipmap-mdpi/ic_launcher_adaptive_back.png", 108, 108},
		{"mipmap-hdpi/ic_launcher.png", 72, 72},
		{"mipmap-hdpi/ic_launcher_adaptive_fore.png", 162, 162},
		{"mipmap-hdpi/ic_launcher_adaptive_back.png", 162, 162},
		{"mipmap-xhdpi/ic_launcher.png", 96, 96},
		{"mipmap-xhdpi/ic_launcher_adaptive_fore.png", 216, 216},
		{"mipmap-xhdpi/ic_launcher_adaptive_back.png", 216, 216},
		{"mipmap-xxhdpi/ic_launcher.png", 144, 144},
		{"mipmap-xxhdpi/ic_launcher_adaptive_fore.png", 324, 324},
		{"mipmap-xxhdpi/ic_launcher_adaptive_back.png", 324, 324},
		{"mipmap-xxxhdpi/ic_launcher.png", 192, 192},
		{"mipmap-xxxhdpi/ic_launcher_adaptive_fore.png", 432, 432},
		{"mipmap-xxxhdpi/ic_launcher_adaptive_back.png", 432, 432},
	}

	for _, e := range expected {
		path := filepath.Join(resDir, e.path)
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("missing %s: %v", e.path, err)
			continue
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Errorf("bad PNG %s: %v", e.path, err)
			continue
		}
		if cfg.Width != e.w || cfg.Height != e.h {
			t.Errorf("%s: expected %dx%d, got %dx%d", e.path, e.w, e.h, cfg.Width, cfg.Height)
		}
	}

	// Check adaptive icon XML exists
	xmlPath := filepath.Join(resDir, "mipmap-anydpi-v26/ic_launcher.xml")
	if _, err := os.Stat(xmlPath); err != nil {
		t.Errorf("missing adaptive icon XML: %v", err)
	}
}

func TestGenerateIOS(t *testing.T) {
	src, err := LoadSource("", "")
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}

	dir := t.TempDir()
	assetDir := filepath.Join(dir, "Assets.xcassets")
	if err := src.GenerateIOS(assetDir); err != nil {
		t.Fatalf("GenerateIOS: %v", err)
	}

	// Check Contents.json
	if _, err := os.Stat(filepath.Join(assetDir, "Contents.json")); err != nil {
		t.Errorf("missing asset catalog Contents.json: %v", err)
	}

	// Check AppIcon Contents.json
	if _, err := os.Stat(filepath.Join(assetDir, "AppIcon.appiconset", "Contents.json")); err != nil {
		t.Errorf("missing AppIcon Contents.json: %v", err)
	}

	// Check icon.png dimensions
	iconPath := filepath.Join(assetDir, "AppIcon.appiconset", "icon.png")
	f, err := os.Open(iconPath)
	if err != nil {
		t.Fatalf("missing icon.png: %v", err)
	}
	defer f.Close()

	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("bad icon.png: %v", err)
	}
	if cfg.Width != 1024 || cfg.Height != 1024 {
		t.Errorf("icon.png: expected 1024x1024, got %dx%d", cfg.Width, cfg.Height)
	}

	// Check LaunchImage.imageset
	launchDir := filepath.Join(assetDir, "LaunchImage.imageset")
	if _, err := os.Stat(filepath.Join(launchDir, "Contents.json")); err != nil {
		t.Errorf("missing LaunchImage Contents.json: %v", err)
	}
	for _, li := range []struct {
		name string
		size int
	}{
		{"launch_icon.png", 120},
		{"launch_icon@2x.png", 240},
		{"launch_icon@3x.png", 360},
	} {
		path := filepath.Join(launchDir, li.name)
		lf, err := os.Open(path)
		if err != nil {
			t.Errorf("missing %s: %v", li.name, err)
			continue
		}
		lcfg, err := png.DecodeConfig(lf)
		lf.Close()
		if err != nil {
			t.Errorf("bad PNG %s: %v", li.name, err)
			continue
		}
		if lcfg.Width != li.size || lcfg.Height != li.size {
			t.Errorf("%s: expected %dx%d, got %dx%d", li.name, li.size, li.size, lcfg.Width, lcfg.Height)
		}
	}
}

func createTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
