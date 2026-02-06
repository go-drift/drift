// Package templates provides embedded template files for project creation.
package templates

import (
	"embed"
	"io/fs"
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

// ReadFile reads a file from the embedded filesystem.
func ReadFile(path string) ([]byte, error) {
	return FS.ReadFile(path)
}

// GetAndroidJavaFiles returns the list of Java/Kotlin template files.
func GetAndroidJavaFiles() ([]string, error) {
	return ListFiles("android/java")
}

// GetAndroidCPPFiles returns the list of C/C++ template files.
func GetAndroidCPPFiles() ([]string, error) {
	return ListFiles("android/cpp")
}

// GetIOSFiles returns the list of iOS Swift template files.
func GetIOSFiles() ([]string, error) {
	return ListFiles("ios")
}

// GetBridgeFiles returns the list of bridge template files.
func GetBridgeFiles() ([]string, error) {
	return ListFiles("bridge")
}

// GetXcodeProjectFiles returns the list of Xcode project template files.
func GetXcodeProjectFiles() ([]string, error) {
	return ListFiles("xcodeproj")
}

// GetXtoolFiles returns the list of xtool SwiftPM template files.
func GetXtoolFiles() ([]string, error) {
	return ListFiles("xtool")
}

// GetInitFiles returns the list of init template files.
func GetInitFiles() ([]string, error) {
	return ListFiles("init")
}

// FileName returns just the filename from a path.
func FileName(path string) string {
	return filepath.Base(path)
}
