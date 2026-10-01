package protocol

import "testing"

func TestParseColor(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Color
		android string
	}{
		{"#112233", Color{0x11, 0x22, 0x33, 0xFF}, "#112233"},
		{"#112233ff", Color{0x11, 0x22, 0x33, 0xFF}, "#112233"},
		{"#11223380", Color{0x11, 0x22, 0x33, 0x80}, "#80112233"},
		{"#aabbcc00", Color{0xAA, 0xBB, 0xCC, 0x00}, "#00AABBCC"},
	} {
		got, err := ParseColor(tc.in)
		if err != nil {
			t.Fatalf("ParseColor(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseColor(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
		if a := got.AndroidHex(); a != tc.android {
			t.Errorf("ParseColor(%q).AndroidHex() = %q, want %q", tc.in, a, tc.android)
		}
	}
}

func TestParseColorRejects(t *testing.T) {
	for _, in := range []string{"", "#fff", "112233", "#11223", "#1122334", "#GG0000", "#112233FF00"} {
		if _, err := ParseColor(in); err == nil {
			t.Errorf("ParseColor(%q) accepted", in)
		}
	}
}
