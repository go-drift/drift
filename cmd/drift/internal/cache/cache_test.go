package cache

import "testing"

// --- NormalizeVersion ---

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Valid releases
		{"v0.1.0", "v0.1.0"},
		{"0.1.0", "v0.1.0"},
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},

		// Prefix stripping
		{"drift-v0.1.0", "v0.1.0"},
		{"drift-0.1.0", "v0.1.0"},

		// Prerelease allowed
		{"v0.2.0-rc1", "v0.2.0-rc1"},
		{"v1.0.0-beta2", "v1.0.0-beta2"},

		// Dev builds rejected
		{"0.1.0-dev", ""},
		{"v0.1.0-dev", ""},
		{"drift-v0.1.0-dev", ""},

		// Pseudo-versions rejected
		{"v0.2.1-0.20260122153045-abc123", ""},

		// Invalid formats
		{"", ""},
		{"v1.2", ""},
		{"v1", ""},
		{"not-a-version", ""},

		// NormalizeVersion only checks dot count, not numeric content.
		// parseSemver handles numeric validation downstream.
		{"vx.y.z", "vx.y.z"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeVersion(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseSemver ---

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input               string
		major, minor, patch int
		valid               bool
	}{
		{"v1.2.3", 1, 2, 3, true},
		{"0.1.0", 0, 1, 0, true},
		{"v10.20.30", 10, 20, 30, true},

		// Prerelease: Sscanf parses the numeric prefix of "3-rc1" as 3
		{"v1.2.3-rc1", 1, 2, 3, true},

		// Bad formats
		{"v1.2", 0, 0, 0, false},
		{"v1", 0, 0, 0, false},
		{"", 0, 0, 0, false},
		{"vx.y.z", 0, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := parseSemver(tt.input)
			if s.valid != tt.valid {
				t.Errorf("parseSemver(%q).valid = %v, want %v", tt.input, s.valid, tt.valid)
			}
			if s.valid {
				if s.major != tt.major || s.minor != tt.minor || s.patch != tt.patch {
					t.Errorf("parseSemver(%q) = %d.%d.%d, want %d.%d.%d",
						tt.input, s.major, s.minor, s.patch, tt.major, tt.minor, tt.patch)
				}
			}
		})
	}
}

// --- semverCompare ---

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// Equal
		{"v1.0.0", "v1.0.0", 0},

		// Major difference
		{"v2.0.0", "v1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},

		// Minor difference
		{"v1.2.0", "v1.1.0", 1},
		{"v1.1.0", "v1.2.0", -1},

		// Patch difference
		{"v1.0.2", "v1.0.1", 1},
		{"v1.0.1", "v1.0.2", -1},

		// Prerelease parses as same numeric version (Sscanf stops at "-"),
		// so v1.0.0-rc1 and v1.0.0 compare equal.
		{"v1.0.0-rc1", "v1.0.0", 0},

		// Different major versions still distinguish correctly
		{"v2.0.0-rc1", "v1.0.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := semverCompare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("semverCompare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// --- intCompare ---

func TestIntCompare(t *testing.T) {
	tests := []struct {
		a, b int
		want int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{0, 1, -1},
		{-1, 1, -1},
		{100, 100, 0},
	}
	for _, tt := range tests {
		got := intCompare(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("intCompare(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
