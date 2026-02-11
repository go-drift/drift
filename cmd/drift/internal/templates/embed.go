// Package templates provides embedded template files for project creation.
package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed android ios bridge/* xcodeproj/* xtool/* init/* driftw driftw.bat
var FS embed.FS

// TemplateInput holds the caller-provided values for template rendering.
type TemplateInput struct {
	AppName        string
	AndroidPackage string
	IOSBundleID    string
	Orientation    string
	AllowHTTP      bool
}

// TemplateData contains the data for template substitution.
type TemplateData struct {
	AppName     string // e.g., "my_app"
	PackageName string // e.g., "com.example.my_app"
	JNIPackage  string // e.g., "com_example_my_app"
	PackagePath string // e.g., "com/example/my_app"
	BundleID    string // e.g., "com.example.my_app"
	URLScheme   string // e.g., "my-app"
	Orientation string // "portrait", "landscape", or "all"
	AllowHTTP   bool   // allow cleartext HTTP traffic
}

// NewTemplateData creates template data from the given input, deriving
// JNI-safe names, package paths, and URL schemes automatically.
func NewTemplateData(in TemplateInput) *TemplateData {
	return &TemplateData{
		AppName:     in.AppName,
		PackageName: in.AndroidPackage,
		JNIPackage:  strings.ReplaceAll(strings.ReplaceAll(in.AndroidPackage, "_", "_1"), ".", "_"),
		PackagePath: strings.ReplaceAll(in.AndroidPackage, ".", "/"),
		BundleID:    in.IOSBundleID,
		URLScheme:   sanitizeURLScheme(in.AppName),
		Orientation: in.Orientation,
		AllowHTTP:   in.AllowHTTP,
	}
}

func sanitizeURLScheme(appName string) string {
	lower := strings.ToLower(strings.TrimSpace(appName))
	if lower == "" {
		return "app"
	}
	var b strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	scheme := b.String()
	if scheme == "" {
		return "app"
	}
	if scheme[0] < 'a' || scheme[0] > 'z' {
		return "app-" + scheme
	}
	return scheme
}

// ProcessTemplate processes a template string with the given data.
func ProcessTemplate(content string, data *TemplateData) (string, error) {
	tmpl, err := template.New("").Parse(content)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ListFiles returns all files in the embedded filesystem under the given path.
func ListFiles(path string) ([]string, error) {
	var files []string

	err := fs.WalkDir(FS, path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})

	return files, err
}

// CopyTree copies all files from srcDir in the embedded filesystem to destDir,
// processing each as a Go template with the given data. Files ending in .tmpl
// have that suffix stripped from the destination filename. Subdirectory
// structure under srcDir is preserved in destDir.
//
// If filter is non-nil, only files where filter returns true for the base
// filename are copied. Pass nil to copy all files.
func CopyTree(srcDir, destDir string, data *TemplateData, filter func(name string) bool) error {
	return fs.WalkDir(FS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		name := filepath.Base(path)
		if filter != nil && !filter(name) {
			return nil
		}

		content, err := FS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		processed, err := ProcessTemplate(string(content), data)
		if err != nil {
			return fmt.Errorf("failed to process %s: %w", path, err)
		}

		destName := name
		if strings.HasSuffix(destName, ".tmpl") {
			destName = strings.TrimSuffix(destName, ".tmpl")
		}

		// Preserve subdirectory structure relative to srcDir
		rel, _ := filepath.Rel(srcDir, filepath.Dir(path))
		dest := filepath.Join(destDir, rel, destName)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", dest, err)
		}

		if err := os.WriteFile(dest, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dest, err)
		}

		return nil
	})
}

// ReadFile reads a file from the embedded filesystem.
func ReadFile(path string) ([]byte, error) {
	return FS.ReadFile(path)
}

// GetBridgeFiles returns the list of bridge template files.
func GetBridgeFiles() ([]string, error) {
	return ListFiles("bridge")
}

// FileName returns just the filename from a path.
func FileName(path string) string {
	return filepath.Base(path)
}
