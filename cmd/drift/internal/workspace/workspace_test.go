package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsEjected_iOS(t *testing.T) {
	root := t.TempDir()

	// No platform dir at all
	if IsEjected(root, "ios") {
		t.Error("expected false when platform dir does not exist")
	}

	// Empty platform/ios directory (stray mkdir)
	platformDir := filepath.Join(root, "platform", "ios")
	os.MkdirAll(platformDir, 0o755)
	if IsEjected(root, "ios") {
		t.Error("expected false for empty platform/ios directory")
	}

	// Only Runner directory, no pbxproj
	os.MkdirAll(filepath.Join(platformDir, "Runner"), 0o755)
	if IsEjected(root, "ios") {
		t.Error("expected false with Runner dir but no pbxproj")
	}

	// Only pbxproj file, no Runner directory
	root2 := t.TempDir()
	platformDir2 := filepath.Join(root2, "platform", "ios")
	pbxprojDir := filepath.Join(platformDir2, "Runner.xcodeproj")
	os.MkdirAll(pbxprojDir, 0o755)
	os.WriteFile(filepath.Join(pbxprojDir, "project.pbxproj"), []byte("{}"), 0o644)
	if IsEjected(root2, "ios") {
		t.Error("expected false with pbxproj but no Runner dir")
	}

	// Both Runner dir and pbxproj file present
	os.MkdirAll(filepath.Join(platformDir, "Runner.xcodeproj"), 0o755)
	os.WriteFile(filepath.Join(platformDir, "Runner.xcodeproj", "project.pbxproj"), []byte("{}"), 0o644)
	if !IsEjected(root, "ios") {
		t.Error("expected true with both Runner dir and pbxproj file")
	}
}

func TestIsEjected_Android(t *testing.T) {
	root := t.TempDir()

	// No platform dir
	if IsEjected(root, "android") {
		t.Error("expected false when platform dir does not exist")
	}

	// Empty platform/android directory
	platformDir := filepath.Join(root, "platform", "android")
	os.MkdirAll(platformDir, 0o755)
	if IsEjected(root, "android") {
		t.Error("expected false for empty platform/android directory")
	}

	// Only settings.gradle, no app/build.gradle
	os.WriteFile(filepath.Join(platformDir, "settings.gradle"), []byte(""), 0o644)
	if IsEjected(root, "android") {
		t.Error("expected false with settings.gradle but no app/build.gradle")
	}

	// Both files present
	os.MkdirAll(filepath.Join(platformDir, "app"), 0o755)
	os.WriteFile(filepath.Join(platformDir, "app", "build.gradle"), []byte(""), 0o644)
	if !IsEjected(root, "android") {
		t.Error("expected true with both settings.gradle and app/build.gradle")
	}
}

func TestIsEjected_Xtool(t *testing.T) {
	root := t.TempDir()

	// No platform dir
	if IsEjected(root, "xtool") {
		t.Error("expected false when platform dir does not exist")
	}

	// Only Package.swift
	platformDir := filepath.Join(root, "platform", "xtool")
	os.MkdirAll(platformDir, 0o755)
	os.WriteFile(filepath.Join(platformDir, "Package.swift"), []byte(""), 0o644)
	if IsEjected(root, "xtool") {
		t.Error("expected false with Package.swift but no Sources/Runner")
	}

	// Both Package.swift and Sources/Runner
	os.MkdirAll(filepath.Join(platformDir, "Sources", "Runner"), 0o755)
	if !IsEjected(root, "xtool") {
		t.Error("expected true with Package.swift and Sources/Runner dir")
	}
}

func TestIsEjected_UnknownPlatform(t *testing.T) {
	root := t.TempDir()
	if IsEjected(root, "windows") {
		t.Error("expected false for unknown platform")
	}
}

func TestEjectedBuildDir(t *testing.T) {
	got := EjectedBuildDir("/home/user/myapp", "ios")
	want := filepath.Join("/home/user/myapp", "platform", "ios")
	if got != want {
		t.Errorf("EjectedBuildDir = %q, want %q", got, want)
	}
}

func TestBridgeDir(t *testing.T) {
	got := BridgeDir("/some/build/dir")
	want := filepath.Join("/some/build/dir", "bridge")
	if got != want {
		t.Errorf("BridgeDir = %q, want %q", got, want)
	}
}

func TestWriteDriftEnv(t *testing.T) {
	dir := t.TempDir()

	if err := WriteDriftEnv(dir); err != nil {
		t.Fatalf("WriteDriftEnv failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".drift.env"))
	if err != nil {
		t.Fatalf("failed to read .drift.env: %v", err)
	}

	content := string(data)

	// Should contain DRIFT_BIN assignment
	if !strings.Contains(content, "DRIFT_BIN=") {
		t.Error("expected .drift.env to contain DRIFT_BIN=")
	}

	// Should contain a "do not commit" comment
	if !strings.Contains(content, "Do not commit") {
		t.Error("expected .drift.env to contain a do-not-commit comment")
	}

	// DRIFT_BIN should point to an executable
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "DRIFT_BIN=") {
			// Extract quoted path
			path := strings.TrimPrefix(line, "DRIFT_BIN=")
			path = strings.Trim(path, "\"")
			if _, err := os.Stat(path); err != nil {
				t.Errorf("DRIFT_BIN path %q does not exist: %v", path, err)
			}
			break
		}
	}
}

func TestJniLibsDir(t *testing.T) {
	tests := []struct {
		name     string
		buildDir string
		ejected  bool
		want     string
	}{
		{
			name:     "ejected",
			buildDir: "/home/user/myapp/platform/android",
			ejected:  true,
			want:     filepath.Join("/home/user/myapp/platform/android", "app", "src", "main", "jniLibs"),
		},
		{
			name:     "managed",
			buildDir: "/home/user/.drift/build/mod/android/abc123",
			ejected:  false,
			want:     filepath.Join("/home/user/.drift/build/mod/android/abc123", "android", "app", "src", "main", "jniLibs"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JniLibsDir(tt.buildDir, tt.ejected)
			if got != tt.want {
				t.Errorf("JniLibsDir(%q, %v) = %q, want %q", tt.buildDir, tt.ejected, got, tt.want)
			}
		})
	}
}
