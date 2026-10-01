package mutate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// WriteIOSAssets writes one Contents.json + image bundle per OpIOSAssetsAddImageSet
// under the supplied Assets.xcassets directory. Each set lives at
// <Assets.xcassets>/<Name>.imageset/.
func WriteIOSAssets(assetsRoot string, ops []*protocol.OpIOSAssetsAddImageSet) ([]string, error) {
	var changed []string
	if err := ensureAssetsRoot(assetsRoot); err != nil {
		return changed, err
	}
	for _, op := range ops {
		files, err := ImageSetFiles(assetsRoot, op)
		if err != nil {
			return changed, err
		}
		paths, err := writeOwned(files...)
		changed = append(changed, paths...)
		if err != nil {
			return changed, err
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
func ReplaceLaunchScreen(path string, op *protocol.OpIOSReplaceLaunchScreen) (string, bool, error) {
	ch, err := writeIfDifferent(path, []byte(op.Content))
	if err != nil {
		return path, false, fmt.Errorf("write LaunchScreen: %w", err)
	}
	return path, ch, nil
}

// WriteIOSSources writes Swift sources under <pluginsRoot>/<group>/<rel>.
func WriteIOSSources(pluginsRoot string, ops []*protocol.OpAddIOSSource) ([]string, error) {
	return writeEach(ops, func(op *protocol.OpAddIOSSource) (OwnedFile, error) {
		return IOSSourceFile(pluginsRoot, op)
	})
}

// WriteKotlinSources writes Kotlin sources under
// <javaRoot>/<packagePath>/<rel> where packagePath = pkg with dots to slashes.
func WriteKotlinSources(javaRoot string, ops []*protocol.OpAddKotlinSource) ([]string, error) {
	return writeEach(ops, func(op *protocol.OpAddKotlinSource) (OwnedFile, error) {
		return KotlinSourceFile(javaRoot, op)
	})
}
