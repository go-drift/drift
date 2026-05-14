package plugin

import (
	"testing"
)

func TestParseXcodeVersion(t *testing.T) {
	cases := []struct {
		raw       string
		wantMajor int
		wantStr   string
	}{
		{"Xcode 16.2\nBuild version 16C5032a\n", 16, "16.2"},
		{"Xcode 15.4\nBuild version 15F31d\n", 15, "15.4"},
		{"Xcode 17\n", 17, "17"},
	}
	for _, c := range cases {
		gotMajor, gotStr := parseXcodeVersion(c.raw)
		if gotMajor != c.wantMajor {
			t.Errorf("major(%q) = %d, want %d", c.raw, gotMajor, c.wantMajor)
		}
		if gotStr != c.wantStr {
			t.Errorf("str(%q) = %q, want %q", c.raw, gotStr, c.wantStr)
		}
	}
}

func TestParseXcodeVersionFailures(t *testing.T) {
	for _, in := range []string{"", "xcode-select", "garbage"} {
		if major, _ := parseXcodeVersion(in); major >= 0 {
			t.Errorf("parseXcodeVersion(%q) = %d, want negative", in, major)
		}
	}
}
