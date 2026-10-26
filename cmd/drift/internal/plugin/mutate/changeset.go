package mutate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Changeset is a set of file writes and removals planned against a project
// and committed together. The mutators plan into one and never touch disk,
// so a build whose ops fail anywhere writes nothing: an ejected project is
// never left holding plugin output its lock does not record.
//
// Reads go through the changeset (ReadFile), so a mutator sees what an
// earlier one planned for the same file (several ops edit Info.plist, the
// manifest and build.gradle). Every path is cleaned (filepath.Clean), so
// two spellings of one file, such as a/./b and a/b, share one entry. A
// changeset is not safe for concurrent use.
type Changeset struct {
	writes  map[string][]byte
	order   []string // write paths, in the order first planned
	removed []string // paths to remove, files or trees
	gone    map[string]bool
	empties []string // directories to remove once empty
}

// NewChangeset returns an empty changeset.
func NewChangeset() *Changeset {
	return &Changeset{writes: map[string][]byte{}, gone: map[string]bool{}}
}

// ReadFile returns path's content as the changeset leaves it: the planned
// bytes, fs.ErrNotExist once it or a parent directory is planned for
// removal, otherwise what is on disk.
func (c *Changeset) ReadFile(path string) ([]byte, error) {
	path = filepath.Clean(path)
	if data, ok := c.writes[path]; ok {
		return bytes.Clone(data), nil
	}
	if c.removedPath(path) {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return os.ReadFile(path)
}

// WriteFile plans path to hold data. Reports whether that differs from what
// ReadFile returns now, i.e. whether the build changes the file. Another
// error reading path is returned.
func (c *Changeset) WriteFile(path string, data []byte) (bool, error) {
	path = filepath.Clean(path)
	existing, err := c.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if _, planned := c.writes[path]; !planned {
		c.order = append(c.order, path)
	}
	c.writes[path] = bytes.Clone(data)
	return err != nil || !bytes.Equal(existing, data), nil
}

// Remove plans the removal of path, a file or a directory tree, dropping
// writes planned at or under it. Reports whether it exists now (on disk or
// planned). Writes planned after Remove land after the removal.
func (c *Changeset) Remove(path string) (bool, error) {
	path = filepath.Clean(path)
	exists := false
	if !c.removedPath(path) {
		_, err := os.Lstat(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
		exists = err == nil
	}
	prefix := path + string(filepath.Separator)
	kept := c.order[:0]
	for _, p := range c.order {
		if p == path || strings.HasPrefix(p, prefix) {
			delete(c.writes, p)
			exists = true
			continue
		}
		kept = append(kept, p)
	}
	c.order = kept
	if !c.gone[path] {
		c.gone[path] = true
		c.removed = append(c.removed, path)
	}
	return exists, nil
}

// RemoveIfEmpty plans the removal of directory dir if it is empty once
// everything else is committed. Directories are tried in the order
// planned, so plan children before their parents.
func (c *Changeset) RemoveIfEmpty(dir string) {
	c.empties = append(c.empties, filepath.Clean(dir))
}

// removedPath reports whether path or a parent of it is planned for
// removal.
func (c *Changeset) removedPath(path string) bool {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		if c.gone[p] {
			return true
		}
		if parent := filepath.Dir(p); parent == p {
			return false
		}
	}
}

// Commit applies the changeset: removals first, then each write whose bytes
// differ from disk (WriteIfDifferent) in the order planned, then the empty
// directories, skipping one that is gone or not empty. A write planned
// last, such as an ejected project's lock, lands last. Commit fails on the
// first other I/O error.
func (c *Changeset) Commit() error {
	for _, p := range c.removed {
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	for _, p := range c.order {
		if _, err := WriteIfDifferent(p, c.writes[p]); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	for _, d := range c.empties {
		if err := os.Remove(d); err != nil && !errors.Is(err, fs.ErrNotExist) && !dirNotEmpty(err) {
			return fmt.Errorf("remove %s: %w", d, err)
		}
	}
	return nil
}

// dirNotEmpty reports whether err is os.Remove's failure on a directory
// that still has entries (ENOTEMPTY; EEXIST on some Unix systems).
func dirNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}
