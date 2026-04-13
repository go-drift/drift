package graphics

import "testing"

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Color
		wantErr bool
	}{
		{name: "rgb hash", input: "#6750A4", want: Color(0xFF6750A4)},
		{name: "rgb plain", input: "6750A4", want: Color(0xFF6750A4)},
		{name: "argb hash", input: "#806750A4", want: Color(0x806750A4)},
		{name: "argb plain", input: "806750A4", want: Color(0x806750A4)},
		{name: "trim whitespace", input: "  #FFFFFF  ", want: ColorWhite},
		{name: "bad length", input: "12345", wantErr: true},
		{name: "bad chars", input: "#XYZXYZ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHexColor(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHexColor() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseHexColor() = 0x%08X, want 0x%08X", uint32(got), uint32(tt.want))
			}
		})
	}
}
