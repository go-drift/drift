package graphics

import (
	"testing"
)

func TestBuildGradientPayload_LinearGradient_ZeroHeight(t *testing.T) {
	// Horizontal line: zero height but non-zero width
	gradient := NewLinearGradient(
		AlignCenterLeft,
		AlignCenterRight,
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := Rect{Left: 0, Top: 50, Right: 100, Bottom: 50} // Zero height

	payload, ok := buildGradientPayload(gradient, bounds)
	if !ok {
		t.Fatal("expected linear gradient to work with zero-height bounds (horizontal line)")
	}

	// Start should be at left edge, end at right edge
	if payload.start.X != 0 {
		t.Errorf("expected start.X=0, got %v", payload.start.X)
	}
	if payload.end.X != 100 {
		t.Errorf("expected end.X=100, got %v", payload.end.X)
	}
}

func TestBuildGradientPayload_LinearGradient_ZeroWidth(t *testing.T) {
	// Vertical line: zero width but non-zero height
	gradient := NewLinearGradient(
		AlignTopCenter,
		AlignBottomCenter,
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := Rect{Left: 50, Top: 0, Right: 50, Bottom: 100} // Zero width

	payload, ok := buildGradientPayload(gradient, bounds)
	if !ok {
		t.Fatal("expected linear gradient to work with zero-width bounds (vertical line)")
	}

	// Start should be at top, end at bottom
	if payload.start.Y != 0 {
		t.Errorf("expected start.Y=0, got %v", payload.start.Y)
	}
	if payload.end.Y != 100 {
		t.Errorf("expected end.Y=100, got %v", payload.end.Y)
	}
}

func TestBuildGradientPayload_LinearGradient_BothZero(t *testing.T) {
	// Point: both dimensions zero
	gradient := NewLinearGradient(
		AlignCenterLeft,
		AlignCenterRight,
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := Rect{Left: 50, Top: 50, Right: 50, Bottom: 50} // Both zero

	_, ok := buildGradientPayload(gradient, bounds)
	if ok {
		t.Error("expected linear gradient to be rejected with both dimensions zero")
	}
}

func TestBuildGradientPayload_RadialGradient_ZeroHeight(t *testing.T) {
	// Radial gradient needs non-zero min dimension for radius
	gradient := NewRadialGradient(
		AlignCenter,
		1.0,
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := Rect{Left: 0, Top: 50, Right: 100, Bottom: 50} // Zero height

	_, ok := buildGradientPayload(gradient, bounds)
	if ok {
		t.Error("expected radial gradient to be rejected with zero min dimension")
	}
}

func TestBuildGradientPayload_RadialGradient_ZeroWidth(t *testing.T) {
	gradient := NewRadialGradient(
		AlignCenter,
		1.0,
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := Rect{Left: 50, Top: 0, Right: 50, Bottom: 100} // Zero width

	_, ok := buildGradientPayload(gradient, bounds)
	if ok {
		t.Error("expected radial gradient to be rejected with zero min dimension")
	}
}

func TestBuildGradientPayload_RadialGradient_ValidBounds(t *testing.T) {
	gradient := NewRadialGradient(
		AlignCenter,
		1.0, // radius = half min dimension
		[]GradientStop{
			{Position: 0, Color: RGB(255, 0, 0)},
			{Position: 1, Color: RGB(0, 0, 255)},
		},
	)
	bounds := RectFromLTWH(0, 0, 200, 100) // Min dimension is 100

	payload, ok := buildGradientPayload(gradient, bounds)
	if !ok {
		t.Fatal("expected radial gradient to work with valid bounds")
	}

	// Center should be at (100, 50)
	if payload.center.X != 100 || payload.center.Y != 50 {
		t.Errorf("expected center (100, 50), got (%v, %v)", payload.center.X, payload.center.Y)
	}

	// Radius should be 50 (half of min dimension 100)
	if payload.radius != 50 {
		t.Errorf("expected radius 50, got %v", payload.radius)
	}
}

func TestAlignment_Resolve(t *testing.T) {
	bounds := RectFromLTWH(0, 0, 200, 100)

	tests := []struct {
		name      string
		alignment Alignment
		wantX     float64
		wantY     float64
	}{
		{"TopLeft", AlignTopLeft, 0, 0},
		{"TopCenter", AlignTopCenter, 100, 0},
		{"TopRight", AlignTopRight, 200, 0},
		{"CenterLeft", AlignCenterLeft, 0, 50},
		{"Center", AlignCenter, 100, 50},
		{"CenterRight", AlignCenterRight, 200, 50},
		{"BottomLeft", AlignBottomLeft, 0, 100},
		{"BottomCenter", AlignBottomCenter, 100, 100},
		{"BottomRight", AlignBottomRight, 200, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.alignment.Resolve(bounds)
			if offset.X != tt.wantX || offset.Y != tt.wantY {
				t.Errorf("Resolve() = (%v, %v), want (%v, %v)", offset.X, offset.Y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestResolveRadius(t *testing.T) {
	tests := []struct {
		name     string
		radius   float64
		bounds   Rect
		expected float64
	}{
		{
			name:     "square bounds radius 1.0",
			radius:   1.0,
			bounds:   RectFromLTWH(0, 0, 100, 100),
			expected: 50, // half of min(100, 100)
		},
		{
			name:     "wide bounds radius 1.0",
			radius:   1.0,
			bounds:   RectFromLTWH(0, 0, 200, 100),
			expected: 50, // half of min(200, 100)
		},
		{
			name:     "tall bounds radius 1.0",
			radius:   1.0,
			bounds:   RectFromLTWH(0, 0, 100, 200),
			expected: 50, // half of min(100, 200)
		},
		{
			name:     "radius 2.0",
			radius:   2.0,
			bounds:   RectFromLTWH(0, 0, 100, 100),
			expected: 100, // 2 * half of 100
		},
		{
			name:     "zero height",
			radius:   1.0,
			bounds:   Rect{Left: 0, Top: 0, Right: 100, Bottom: 0},
			expected: 0, // min dimension is 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveRadius(tt.radius, tt.bounds)
			if result != tt.expected {
				t.Errorf("ResolveRadius(%v, %v) = %v, want %v", tt.radius, tt.bounds, result, tt.expected)
			}
		})
	}
}

func TestPath_Bounds(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Path)
		expected Rect
	}{
		{
			name:     "empty path",
			setup:    func(p *Path) {},
			expected: Rect{},
		},
		{
			name: "simple rectangle",
			setup: func(p *Path) {
				p.MoveTo(10, 20)
				p.LineTo(100, 20)
				p.LineTo(100, 80)
				p.LineTo(10, 80)
				p.Close()
			},
			expected: Rect{Left: 10, Top: 20, Right: 100, Bottom: 80},
		},
		{
			name: "with curves (control points included)",
			setup: func(p *Path) {
				p.MoveTo(0, 50)
				p.QuadTo(50, 0, 100, 50) // Control point at (50, 0)
			},
			expected: Rect{Left: 0, Top: 0, Right: 100, Bottom: 50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPath()
			tt.setup(p)
			bounds := p.Bounds()
			if bounds != tt.expected {
				t.Errorf("Bounds() = %v, want %v", bounds, tt.expected)
			}
		})
	}
}
