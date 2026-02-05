package graphics

import "testing"

func TestRect_Intersects(t *testing.T) {
	tests := []struct {
		name     string
		r1       Rect
		r2       Rect
		expected bool
	}{
		{
			name:     "overlapping rectangles",
			r1:       RectFromLTWH(0, 0, 100, 100),
			r2:       RectFromLTWH(50, 50, 100, 100),
			expected: true,
		},
		{
			name:     "one contains the other",
			r1:       RectFromLTWH(0, 0, 100, 100),
			r2:       RectFromLTWH(25, 25, 50, 50),
			expected: true,
		},
		{
			name:     "identical rectangles",
			r1:       RectFromLTWH(10, 10, 50, 50),
			r2:       RectFromLTWH(10, 10, 50, 50),
			expected: true,
		},
		{
			name:     "non-overlapping horizontally",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(100, 0, 50, 50),
			expected: false,
		},
		{
			name:     "non-overlapping vertically",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(0, 100, 50, 50),
			expected: false,
		},
		{
			name:     "adjacent horizontally (share edge)",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(50, 0, 50, 50),
			expected: false, // Adjacent rects don't intersect (no interior overlap)
		},
		{
			name:     "adjacent vertically (share edge)",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(0, 50, 50, 50),
			expected: false, // Adjacent rects don't intersect (no interior overlap)
		},
		{
			name:     "share single corner point",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(50, 50, 50, 50),
			expected: false, // Corner touch doesn't count as intersection
		},
		{
			name:     "empty rect with normal rect",
			r1:       Rect{Left: 10, Top: 10, Right: 10, Bottom: 10}, // Zero area
			r2:       RectFromLTWH(0, 0, 100, 100),
			expected: false, // Empty rects don't intersect
		},
		{
			name:     "negative dimension rect",
			r1:       Rect{Left: 50, Top: 50, Right: 0, Bottom: 0}, // Inverted
			r2:       RectFromLTWH(0, 0, 100, 100),
			expected: false, // Inverted rects don't intersect
		},
		{
			name:     "both empty rects",
			r1:       Rect{},
			r2:       Rect{},
			expected: false,
		},
		{
			name:     "partial overlap top-left corner",
			r1:       RectFromLTWH(0, 0, 100, 100),
			r2:       RectFromLTWH(-50, -50, 100, 100),
			expected: true,
		},
		{
			name:     "minimal overlap (1 pixel)",
			r1:       RectFromLTWH(0, 0, 50, 50),
			r2:       RectFromLTWH(49, 49, 50, 50),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test both directions (intersection should be symmetric)
			if got := tt.r1.Intersects(tt.r2); got != tt.expected {
				t.Errorf("r1.Intersects(r2) = %v, want %v", got, tt.expected)
			}
			if got := tt.r2.Intersects(tt.r1); got != tt.expected {
				t.Errorf("r2.Intersects(r1) = %v, want %v (should be symmetric)", got, tt.expected)
			}
		})
	}
}
