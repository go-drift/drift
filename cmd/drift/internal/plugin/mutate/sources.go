package mutate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// EnsureAssetCatalog plans the root Contents.json of the asset catalog
// at dir unless one exists, so the image sets plugins write into it
// (ImageSetFiles) compile. Returns the path if it plans one.
func EnsureAssetCatalog(cs *Changeset, dir string) ([]string, error) {
	rootContents := filepath.Join(dir, "Contents.json")
	if _, err := cs.ReadFile(rootContents); err == nil {
		return nil, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", rootContents, err)
	}
	root := []byte(`{
  "info" : {
    "author" : "drift",
    "version" : 1
  }
}
`)
	if _, err := cs.WriteFile(rootContents, root); err != nil {
		return nil, err
	}
	return []string{rootContents}, nil
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
