package mutate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// WriteIOSAssets plans one Contents.json + image bundle per OpIOSAssetsAddImageSet
// under the supplied Assets.xcassets directory. Each set lives at
// <Assets.xcassets>/<Name>.imageset/.
func WriteIOSAssets(cs *Changeset, assetsRoot string, ops []*protocol.OpIOSAssetsAddImageSet) ([]string, error) {
	var changed []string
	if err := ensureAssetsRoot(cs, assetsRoot); err != nil {
		return changed, err
	}
	for _, op := range ops {
		files, err := ImageSetFiles(assetsRoot, op)
		if err != nil {
			return changed, err
		}
		paths, err := writeOwned(cs, files...)
		changed = append(changed, paths...)
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// ensureAssetsRoot plans the catalog's root Contents.json unless one
// exists.
func ensureAssetsRoot(cs *Changeset, dir string) error {
	rootContents := filepath.Join(dir, "Contents.json")
	if _, err := cs.ReadFile(rootContents); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", rootContents, err)
	}
	root := []byte(`{
  "info" : {
    "author" : "drift",
    "version" : 1
  }
}
`)
	_, err := cs.WriteFile(rootContents, root)
	return err
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

// ReplaceLaunchScreen plans a new LaunchScreen.storyboard at path. Returns
// the path and a changed flag.
func ReplaceLaunchScreen(cs *Changeset, path string, op *protocol.OpIOSReplaceLaunchScreen) (string, bool, error) {
	ch, err := cs.WriteFile(path, []byte(op.Content))
	if err != nil {
		return path, false, fmt.Errorf("LaunchScreen: %w", err)
	}
	return path, ch, nil
}

// WriteKotlinSources plans Kotlin sources under
// <javaRoot>/<packagePath>/<rel> where packagePath = pkg with dots to slashes.
func WriteKotlinSources(cs *Changeset, javaRoot string, ops []*protocol.OpAddKotlinSource) ([]string, error) {
	return writeEach(cs, ops, func(op *protocol.OpAddKotlinSource) (OwnedFile, error) {
		return KotlinSourceFile(javaRoot, op)
	})
}
