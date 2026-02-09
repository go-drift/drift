package config

import "testing"

// --- defaultAppName ---

func TestDefaultAppName(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		dir        string
		want       string
	}{
		{
			name:       "simple module path",
			modulePath: "github.com/user/myapp",
			dir:        "/home/user/myapp",
			want:       "myapp",
		},
		{
			name:       "module with version suffix",
			modulePath: "github.com/user/myapp/v2",
			dir:        "/home/user/myapp",
			want:       "myapp",
		},
		{
			name:       "deep module path",
			modulePath: "github.com/org/repo/cmd/tool",
			dir:        "/home/user/tool",
			want:       "tool",
		},
		{
			name:       "empty module path falls back to drift_app",
			modulePath: "",
			dir:        "/home/user/fallback",
			want:       "drift_app",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultAppName(tt.modulePath, tt.dir)
			if got != tt.want {
				t.Errorf("defaultAppName(%q, %q) = %q, want %q", tt.modulePath, tt.dir, got, tt.want)
			}
		})
	}
}

// --- defaultAppID ---

func TestDefaultAppID(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		appName    string
		want       string
	}{
		{
			name:       "github module",
			modulePath: "github.com/user/myapp",
			appName:    "myapp",
			want:       "com.github.user.myapp",
		},
		{
			name:       "custom domain",
			modulePath: "example.org/tools/drift",
			appName:    "drift",
			want:       "org.example.tools.drift",
		},
		{
			name:       "single segment host falls back",
			modulePath: "myapp",
			appName:    "myapp",
			want:       "com.example.myapp",
		},
		{
			name:       "no dots in host falls back",
			modulePath: "internal/myapp",
			appName:    "myapp",
			want:       "com.example.myapp",
		},
		{
			name:       "deep nested path",
			modulePath: "github.com/org/mono/apps/frontend",
			appName:    "frontend",
			want:       "com.github.org.mono.apps.frontend",
		},
		{
			name:       "hyphens and underscores stripped",
			modulePath: "github.com/my-org/my_app",
			appName:    "my_app",
			want:       "com.github.myorg.myapp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultAppID(tt.modulePath, tt.appName)
			if got != tt.want {
				t.Errorf("defaultAppID(%q, %q) = %q, want %q", tt.modulePath, tt.appName, got, tt.want)
			}
		})
	}
}

// --- sanitizeSegment ---

func TestSanitizeSegment(t *testing.T) {
	tests := []struct {
		name              string
		segment           string
		allowLeadingDigit bool
		want              string
	}{
		{"lowercase passthrough", "hello", true, "hello"},
		{"uppercase lowered", "Hello", true, "hello"},
		{"digits allowed", "app1", true, "app1"},
		{"hyphens stripped", "my-app", true, "myapp"},
		{"underscores stripped", "my_app", true, "myapp"},
		{"special chars stripped", "my@app!", true, "myapp"},
		{"empty becomes app", "", true, "app"},
		{"whitespace only becomes app", "   ", true, "app"},
		{"all invalid becomes app", "---", true, "app"},

		// Leading digit handling
		{"leading digit allowed", "1foo", true, "1foo"},
		{"leading digit prefixed", "1foo", false, "a1foo"},
		{"all digits prefixed", "123", false, "a123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSegment(tt.segment, tt.allowLeadingDigit)
			if got != tt.want {
				t.Errorf("sanitizeSegment(%q, %v) = %q, want %q", tt.segment, tt.allowLeadingDigit, got, tt.want)
			}
		})
	}
}

// --- validateOrientation ---

func TestValidateOrientation(t *testing.T) {
	// Valid orientations
	for _, valid := range []string{"portrait", "landscape", "all"} {
		if err := validateOrientation(valid); err != nil {
			t.Errorf("validateOrientation(%q) returned unexpected error: %v", valid, err)
		}
	}

	// Invalid orientations
	for _, invalid := range []string{"", "auto", "Portrait", "LANDSCAPE", "reverse"} {
		if err := validateOrientation(invalid); err == nil {
			t.Errorf("validateOrientation(%q) should return error", invalid)
		}
	}
}

// --- validateAppID ---

func TestValidateAppID_Valid(t *testing.T) {
	valid := []string{
		"com.example.app",
		"org.drift.myapp",
		"com.github.user.repo",
		"io.drift.a1",
		"a.b",
		"com.example.app_name",
	}
	for _, id := range valid {
		if err := validateAppID(id); err != nil {
			t.Errorf("validateAppID(%q) returned unexpected error: %v", id, err)
		}
	}
}

func TestValidateAppID_NoDot(t *testing.T) {
	err := validateAppID("nodot")
	if err == nil {
		t.Error("expected error for ID without dots")
	}
}

func TestValidateAppID_EmptySegment(t *testing.T) {
	err := validateAppID("com..app")
	if err == nil {
		t.Error("expected error for empty segment")
	}
}

func TestValidateAppID_LeadingDigit(t *testing.T) {
	err := validateAppID("com.1bad.app")
	if err == nil {
		t.Error("expected error for segment starting with digit")
	}
}

func TestValidateAppID_LeadingUnderscore(t *testing.T) {
	err := validateAppID("com._bad.app")
	if err == nil {
		t.Error("expected error for segment starting with underscore")
	}
}

func TestValidateAppID_InvalidChars(t *testing.T) {
	invalid := []string{
		"com.UPPER.app",
		"com.my-app.id",
		"com.app!.id",
		"com.foo bar.id",
	}
	for _, id := range invalid {
		if err := validateAppID(id); err == nil {
			t.Errorf("validateAppID(%q) should return error for invalid characters", id)
		}
	}
}
