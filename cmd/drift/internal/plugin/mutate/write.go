package mutate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// rename is os.Rename; tests replace it to simulate a crash between writing
// the temporary file and renaming it into place.
var rename = os.Rename

// WriteFileAtomic replaces path with data. It writes a temporary file in the
// same directory and renames it over path, so an interrupted build leaves
// the old file or the new one, never a torn one. In ejected projects the
// files Drift edits are the user's. The file keeps its permissions (0644
// when new), and a symlink is written through to its target.
func WriteFileAtomic(path string, data []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return rename(tmp.Name(), path)
}

// WriteIfDifferent writes data to path with WriteFileAtomic unless path
// already holds exactly data. Reports whether it wrote.
func WriteIfDifferent(path string, data []byte) (bool, error) {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err := WriteFileAtomic(path, data); err != nil {
		return false, err
	}
	return true, nil
}
