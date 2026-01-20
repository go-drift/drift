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
  - A new directory with the given name
  - go.mod with the specified module path
  - main.go with a starter application

The module path defaults to the project name if not specified.

Examples:
  drift init myapp
  drift init myapp github.com/username/myapp`,
		Usage: "drift init <project-name> [module-path]",
		Run:   runInit,
	})
}

// initTemplateData contains the data for init template substitution.
type initTemplateData struct {
	ModulePath string
}

func runInit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project name is required\n\nUsage: drift init <project-name> [module-path]")
	}

	projectName := args[0]
	modulePath := projectName
	if len(args) > 1 {
		modulePath = args[1]
	}

	// Validate project name
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	// Check if directory already exists
	if _, err := os.Stat(projectName); err == nil {
		return fmt.Errorf("directory %q already exists", projectName)
	}

	fmt.Printf("Creating new Drift project: %s\n", projectName)

	// Create project directory
	if err := os.MkdirAll(projectName, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data := initTemplateData{
		ModulePath: modulePath,
	}

	// Write template files
	initFiles := []struct {
		templatePath string
		destName     string
	}{
		{"init/go.mod.tmpl", "go.mod"},
		{"init/main.go.tmpl", "main.go"},
	}

	for _, f := range initFiles {
		if err := writeInitTemplate(projectName, f.templatePath, f.destName, data); err != nil {
			os.RemoveAll(projectName)
			return err
		}
		fmt.Printf("  Created %s\n", f.destName)
	}

	// Add drift dependency with latest version
	fmt.Println("  Adding drift dependency...")
	getCmd := exec.Command("go", "get", "github.com/go-drift/drift@latest")
	getCmd.Dir = projectName
	getCmd.Stdout = os.Stdout
	getCmd.Stderr = os.Stderr
	if err := getCmd.Run(); err != nil {
		fmt.Println("  Warning: go get failed (this is expected if drift is not yet published)")
	}

	// Run go mod tidy to resolve dependencies
	fmt.Println("  Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = projectName
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		fmt.Println("  Warning: go mod tidy failed")
	}

	fmt.Println()
	fmt.Printf("Project created successfully!\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  drift run android    # Run on Android\n")
	fmt.Printf("  drift run ios        # Run on iOS (macOS only)\n")

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

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("project name cannot start with a dot")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("project name cannot start with a hyphen")
	}

	// Allow alphanumeric, underscore, and hyphen
	validName := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("project name must start with a letter and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}
