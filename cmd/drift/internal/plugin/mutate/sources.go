package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// WriteIOSAssets writes one Contents.json + image bundle per OpIOSAssetsAddImageSet
// under the supplied Assets.xcassets directory. Each set lives at
// <Assets.xcassets>/<Name>.imageset/.
func WriteIOSAssets(assetsRoot string, ops []*driftplugin.OpIOSAssetsAddImageSet) ([]string, error) {
	var changed []string
	if err := ensureAssetsRoot(assetsRoot); err != nil {
		return changed, err
	}
	for _, op := range ops {
		setDir := filepath.Join(assetsRoot, op.Name+".imageset")
		if err := os.MkdirAll(setDir, 0o755); err != nil {
			return changed, fmt.Errorf("mkdir imageset: %w", err)
		}
		content, err := driftplugin.DecodeContent(op.Image)
		if err != nil {
			return changed, fmt.Errorf("decode image %s: %w", op.Name, err)
		}
		imageFile := filepath.Join(setDir, op.Name+".png")
		ch, err := writeIfDifferent(imageFile, content)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, imageFile)
		}

		contentsJSON, err := imagesetContentsJSON(op.Name + ".png")
		if err != nil {
			return changed, err
		}
		manifest := filepath.Join(setDir, "Contents.json")
		ch2, err := writeIfDifferent(manifest, contentsJSON)
		if err != nil {
			return changed, err
		}
		if ch2 {
			changed = append(changed, manifest)
		}
	}
	return changed, nil
}

func ensureAssetsRoot(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir Assets.xcassets: %w", err)
	}
	rootContents := filepath.Join(dir, "Contents.json")
	if _, err := os.Stat(rootContents); err == nil {
		return nil
	}
	root := []byte(`{
  "info" : {
    "author" : "drift",
    "version" : 1
  }
}
`)
	return os.WriteFile(rootContents, root, 0o644)
}

func imagesetContentsJSON(filename string) ([]byte, error) {
	desc := map[string]any{
		"images": []map[string]any{
			{"idiom": "universal", "filename": filename, "scale": "1x"},
			{"idiom": "universal", "scale": "2x"},
			{"idiom": "universal", "scale": "3x"},
		},
		"info": map[string]any{"author": "drift", "version": 1},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(desc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReplaceLaunchScreen writes a new LaunchScreen.storyboard at path. Returns
// the path and a changed flag.
func ReplaceLaunchScreen(path string, op *driftplugin.OpIOSReplaceLaunchScreen) (string, bool, error) {
	ch, err := writeIfDifferent(path, []byte(op.Content))
	if err != nil {
		return path, false, fmt.Errorf("write LaunchScreen: %w", err)
	}
	return path, ch, nil
}

// WriteIOSSources writes Swift sources under <pluginsRoot>/<group>/<rel>.
func WriteIOSSources(pluginsRoot string, ops []*driftplugin.OpAddIOSSource) ([]string, error) {
	var changed []string
	for _, op := range ops {
		dest := filepath.Join(pluginsRoot, sanitizePath(op.Group), filepath.FromSlash(op.RelPath))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return changed, fmt.Errorf("mkdir iOS source: %w", err)
		}
		content, err := driftplugin.DecodeContent(op.Content)
		if err != nil {
			return changed, fmt.Errorf("decode iOS source %s: %w", op.RelPath, err)
		}
		ch, err := writeIfDifferent(dest, content)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

// WriteKotlinSources writes Kotlin sources under
// <javaRoot>/<packagePath>/<rel> where packagePath = pkg with dots to slashes.
func WriteKotlinSources(javaRoot string, ops []*driftplugin.OpAddKotlinSource) ([]string, error) {
	var changed []string
	for _, op := range ops {
		pkgSegments := strings.Split(op.Package, ".")
		parts := append([]string{javaRoot}, pkgSegments...)
		parts = append(parts, filepath.FromSlash(op.RelPath))
		dest := filepath.Join(parts...)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return changed, fmt.Errorf("mkdir Kotlin source: %w", err)
		}
		content, err := driftplugin.DecodeContent(op.Content)
		if err != nil {
			return changed, fmt.Errorf("decode Kotlin source %s: %w", op.RelPath, err)
		}
		ch, err := writeIfDifferent(dest, content)
		if err != nil {
			return changed, err
		}
		if ch {
			changed = append(changed, dest)
		}
	}
	return changed, nil
}

func sanitizePath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
