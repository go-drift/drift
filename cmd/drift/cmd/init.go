package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
)

func init() {
	RegisterCommand(&Command{
		Name:  "init",
		Short: "Create a new Drift project",
		Long: `Create a new Drift project in a new directory.

This command creates:
  - A new directory at the specified path
  - go.mod with the specified module path
  - main.go with a starter application

The project name is derived from the directory basename.
The module path defaults to the project name if not specified.

Examples:
  drift init myapp
  drift init myapp github.com/username/myapp
  drift init ./projects/myapp
  drift init ./projects/myapp github.com/username/myapp`,
		Usage: "drift init <directory> [module-path]",
		Run:   runInit,
	})
}

// initTemplateData contains the data for init template substitution.
type initTemplateData struct {
	ModulePath string
}

// runInit creates a new Drift project. The first argument is the directory path
// to create (which may be relative or absolute). The project name is derived from
// the directory's basename. An optional second argument overrides the Go module path,
// which otherwise defaults to the project name.
func runInit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("directory is required\n\nUsage: drift init <directory> [module-path]")
	}

	raw := args[0]
	if strings.HasPrefix(raw, "~") {
		return fmt.Errorf("tilde (~) is not expanded by drift; use an absolute path or $HOME instead")
	}

	dir := filepath.Clean(raw)

	// Validate directory path before deriving anything from it
	if err := validateDirectory(dir); err != nil {
		return err
	}

	projectName := filepath.Base(dir)
	modulePath := projectName
	if len(args) > 1 {
		modulePath = args[1]
	}

	if modulePath == "" {
		return fmt.Errorf("module path cannot be empty")
	}

	// Validate project name
	if err := validateProjectName(projectName); err != nil {
		return fmt.Errorf("invalid project name %q (derived from directory basename): %w", projectName, err)
	}

	// Scaffold the project directory and template files
	if err := scaffoldProject(dir, modulePath); err != nil {
		return err
	}

	// Resolve Go dependencies
	fmt.Println("  Adding drift dependency...")
	getCmd := exec.Command("go", "get", "github.com/go-drift/drift@latest")
	getCmd.Dir = dir
	getCmd.Stdout = os.Stdout
	getCmd.Stderr = os.Stderr
	if err := getCmd.Run(); err != nil {
		fmt.Println("  Warning: go get failed (this is expected if drift is not yet published)")
	}

	fmt.Println("  Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		fmt.Println("  Warning: go mod tidy failed")
	}

	fmt.Println()
	fmt.Printf("Project created successfully!\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", dir)
	fmt.Printf("  drift run android    # Run on Android\n")
	fmt.Printf("  drift run ios        # Run on iOS (macOS only)\n")

	return nil
}

// scaffoldProject creates the project directory and writes the template files.
// This is the portion of init that has no side effects beyond the filesystem,
// making it safe to call from tests without network access.
func scaffoldProject(dir, modulePath string) error {
	// Check if directory already exists
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %q already exists", dir)
	}

	fmt.Printf("Creating new Drift project: %s\n", filepath.Base(dir))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data := initTemplateData{
		ModulePath: modulePath,
	}

	initFiles := []struct {
		templatePath string
		destName     string
	}{
		{"init/go.mod.tmpl", "go.mod"},
		{"init/main.go.tmpl", "main.go"},
	}

	for _, f := range initFiles {
		if err := writeInitTemplate(dir, f.templatePath, f.destName, data); err != nil {
			safeRemoveAll(dir)
			return err
		}
		fmt.Printf("  Created %s\n", f.destName)
	}

	return nil
}

func writeInitTemplate(projectDir, templatePath, destName string, data initTemplateData) error {
	content, err := templates.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Process template
	tmpl, err := template.New(destName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	destPath := filepath.Join(projectDir, destName)
	if err := os.WriteFile(destPath, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", destName, err)
	}

	return nil
}

// validateDirectory rejects directory paths that would be dangerous to create or
// clean up. This includes filesystem roots (/, C:\), the current/parent directory,
// and root-level absolute paths (e.g. /etc, C:\Users).
func validateDirectory(dir string) error {
	// The "" case is not reachable via runInit (filepath.Clean converts it to
	// "."), but is included for direct callers of validateDirectory.
	// "/" is kept explicitly because isVolumeRoot won't match "/" on Windows
	// (where the separator is \), yet "/" still refers to the current drive root.
	switch dir {
	case "", "/", ".", "..":
		return fmt.Errorf("directory %q is not a valid project location", dir)
	}
	// Reject filesystem roots (\, C:\, etc.)
	if isVolumeRoot(dir) {
		return fmt.Errorf("directory %q is not a valid project location", dir)
	}
	// Reject root-level absolute paths (e.g. /etc, /home, C:\Users)
	if filepath.IsAbs(dir) && isVolumeRoot(filepath.Dir(dir)) {
		return fmt.Errorf("refusing to create project at root-level path %q", dir)
	}
	return nil
}

// isVolumeRoot reports whether dir is a filesystem root. On Unix this is "/",
// on Windows this covers drive roots like "C:\" and the bare root "\".
func isVolumeRoot(dir string) bool {
	return dir == filepath.VolumeName(dir)+string(filepath.Separator)
}

// safeRemoveAll removes a directory only if the path passes validateDirectory.
// It silently no-ops for dangerous paths rather than returning an error, since
// it is called on cleanup paths where the original error should not be masked.
func safeRemoveAll(dir string) {
	if validateDirectory(dir) != nil {
		return
	}
	os.RemoveAll(dir)
}

var validProjectName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// validateProjectName checks that a project name (derived from the directory
// basename) is a valid identifier: starts with a letter, contains only letters,
// digits, underscores, and hyphens.
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	// These prefix checks are redundant with the regex below, but produce
	// more actionable error messages for common mistakes (hidden dirs, flags).
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("project name cannot start with a dot")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("project name cannot start with a hyphen")
	}
	if !validProjectName.MatchString(name) {
		return fmt.Errorf("project name must start with a letter and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}
