package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateDirectory(t *testing.T) {
	type tc struct {
		name    string
		dir     string
		wantErr bool
	}
	tests := []tc{
		{"simple name", "myapp", false},
		{"relative path", "projects/myapp", false},
		{"dot-slash relative", "./projects/myapp", false},
		{"deep relative", "a/b/c/myapp", false},

		// Dangerous paths (cross-platform)
		{"empty", "", true},
		{"root slash", "/", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests,
			tc{"drive root", `C:\`, true},
			tc{"bare backslash root", `\`, true},
			tc{"root-level C:\\Users", `C:\Users`, true},
			tc{"root-level C:\\Windows", `C:\Windows`, true},
			tc{"nested windows path", `C:\Users\me\projects\myapp`, false},
		)
	} else {
		tests = append(tests,
			tc{"absolute nested", "/home/user/projects/myapp", false},
			tc{"root-level /etc", "/etc", true},
			tc{"root-level /home", "/home", true},
			tc{"root-level /tmp", "/tmp", true},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDirectory(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDirectory(%q) error = %v, wantErr %v", tt.dir, err, tt.wantErr)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "myapp", false},
		{"with hyphen", "my-app", false},
		{"with underscore", "my_app", false},
		{"with numbers", "app2", false},
		{"uppercase", "MyApp", false},

		{"empty", "", true},
		{"starts with dot", ".hidden", true},
		{"starts with hyphen", "-bad", true},
		{"starts with number", "1app", true},
		{"has spaces", "my app", true},
		{"has slash", "my/app", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSafeRemoveAll(t *testing.T) {
	// safeRemoveAll should remove a normal directory
	t.Run("removes normal directory", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "myapp")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		safeRemoveAll(target)
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("expected directory to be removed, but it still exists")
		}
	})

	// safeRemoveAll should refuse to remove dangerous paths.
	// We can't directly observe a no-op on paths that don't exist,
	// but we verify it doesn't panic.
	t.Run("no-ops on dangerous paths", func(t *testing.T) {
		dangerous := []string{"", "/", ".", ".."}
		if runtime.GOOS == "windows" {
			dangerous = append(dangerous, `C:\`, `\`)
		}
		for _, d := range dangerous {
			safeRemoveAll(d) // must not panic
		}
	})
}

func TestScaffoldProject_ProjectNameFromBasename(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "projects", "myapp")

	err := scaffoldProject(dir, "myapp")
	if err != nil {
		t.Fatalf("scaffoldProject(%q) unexpected error: %v", dir, err)
	}

	// Verify go.mod exists and uses basename as module path
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if got := string(gomod); !strings.Contains(got, "module myapp") {
		t.Errorf("go.mod should contain 'module myapp', got:\n%s", got)
	}

	// Verify main.go exists
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Errorf("main.go should exist: %v", err)
	}
}

func TestScaffoldProject_ModulePathOverride(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "myapp")

	err := scaffoldProject(dir, "github.com/user/myapp")
	if err != nil {
		t.Fatalf("scaffoldProject unexpected error: %v", err)
	}

	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if got := string(gomod); !strings.Contains(got, "module github.com/user/myapp") {
		t.Errorf("go.mod should contain overridden module path, got:\n%s", got)
	}
}

func TestScaffoldProject_RejectsExistingDirectory(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "myapp")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := scaffoldProject(dir, "myapp")
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
}

func TestRunInit_RejectsDangerousDirectory(t *testing.T) {
	// Note: "" is not included here because filepath.Clean converts it to ".",
	// making it redundant with the "." case. The "" case is tested directly
	// in TestValidateDirectory for direct callers.
	for _, dir := range []string{"/", ".", ".."} {
		err := runInit([]string{dir})
		if err == nil {
			t.Errorf("expected error for dangerous directory %q, got nil", dir)
		}
	}
}

func TestRunInit_RejectsTilde(t *testing.T) {
	for _, dir := range []string{"~/myapp", "~/projects/myapp"} {
		err := runInit([]string{dir})
		if err == nil {
			t.Errorf("expected error for tilde path %q, got nil", dir)
		}
		if err != nil && !strings.Contains(err.Error(), "tilde") {
			t.Errorf("expected tilde-specific error for %q, got: %v", dir, err)
		}
	}
}

func TestRunInit_RejectsEmptyModulePath(t *testing.T) {
	err := runInit([]string{"myapp", ""})
	if err == nil {
		t.Fatal("expected error for empty module path, got nil")
	}
}

func TestRunInit_NoArgs(t *testing.T) {
	err := runInit(nil)
	if err == nil {
		t.Fatal("expected error for no args, got nil")
	}
}

