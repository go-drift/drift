package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

func TestNormalizeSnapPoints_Empty(t *testing.T) {
	result := NormalizeSnapPoints(nil)
	if len(result) != 1 {
		t.Fatalf("Expected 1 snap point, got %d", len(result))
	}
	if result[0].FractionalHeight != 1.0 {
		t.Errorf("Expected default snap point at 1.0, got %f", result[0].FractionalHeight)
	}
}

func TestNormalizeSnapPoints_ClampsValues(t *testing.T) {
	points := []SnapPoint{
		{FractionalHeight: 0.05, Name: "too-low"},
		{FractionalHeight: 1.5, Name: "too-high"},
		{FractionalHeight: 0.5, Name: "valid"},
	}
	result := NormalizeSnapPoints(points)
	if len(result) != 3 {
		t.Fatalf("Expected 3 snap points, got %d", len(result))
	}
	if result[0].FractionalHeight != 0.1 {
		t.Errorf("Expected first snap point at 0.1, got %f", result[0].FractionalHeight)
	}
	if result[1].FractionalHeight != 0.5 {
		t.Errorf("Expected second snap point at 0.5, got %f", result[1].FractionalHeight)
	}
	if result[2].FractionalHeight != 1.0 {
		t.Errorf("Expected third snap point at 1.0, got %f", result[2].FractionalHeight)
	}
}

func TestNormalizeSnapPoints_SortsAndDedups(t *testing.T) {
	points := []SnapPoint{
		{FractionalHeight: 0.9, Name: "full"},
		{FractionalHeight: 0.3, Name: "small"},
		{FractionalHeight: 0.5, Name: "half"},
		{FractionalHeight: 0.5, Name: "duplicate"},
	}
	result := NormalizeSnapPoints(points)
	if len(result) != 3 {
		t.Fatalf("Expected 3 snap points, got %d", len(result))
	}
	if result[0].FractionalHeight != 0.3 {
		t.Errorf("Expected first snap point at 0.3, got %f", result[0].FractionalHeight)
	}
	if result[1].FractionalHeight != 0.5 {
		t.Errorf("Expected second snap point at 0.5, got %f", result[1].FractionalHeight)
	}
	if result[2].FractionalHeight != 0.9 {
		t.Errorf("Expected third snap point at 0.9, got %f", result[2].FractionalHeight)
	}
	if result[1].Name != "half" {
		t.Errorf("Expected first 0.5 snap to be preserved, got %q", result[1].Name)
	}
}

func TestValidateInitialSnap(t *testing.T) {
	points := []SnapPoint{{FractionalHeight: 0.5}, {FractionalHeight: 0.9}}
	if ValidateInitialSnap(0, points) != 0 {
		t.Error("ValidateInitialSnap(0) should return 0")
	}
	if ValidateInitialSnap(1, points) != 1 {
		t.Error("ValidateInitialSnap(1) should return 1")
	}
	if ValidateInitialSnap(-1, points) != 0 {
		t.Error("ValidateInitialSnap(-1) should return 0")
	}
	if ValidateInitialSnap(5, points) != 0 {
		t.Error("ValidateInitialSnap(5) should return 0")
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		value, min, max, expected float64
	}{
		{0.5, 0.0, 1.0, 0.5},
		{-0.5, 0.0, 1.0, 0.0},
		{1.5, 0.0, 1.0, 1.0},
		{0.0, 0.0, 1.0, 0.0},
		{1.0, 0.0, 1.0, 1.0},
	}
	for _, tt := range tests {
		result := clampFloat(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clampFloat(%f, %f, %f) = %f, want %f",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

func TestDefaultBottomSheetTheme(t *testing.T) {
	theme := DefaultBottomSheetTheme()
	if theme.BorderRadius != 16 {
		t.Errorf("Expected BorderRadius 16, got %f", theme.BorderRadius)
	}
	if theme.HandleWidth != 32 {
		t.Errorf("Expected HandleWidth 32, got %f", theme.HandleWidth)
	}
	if theme.HandleHeight != 4 {
		t.Errorf("Expected HandleHeight 4, got %f", theme.HandleHeight)
	}
	if theme.HandleTopPadding != 8 {
		t.Errorf("Expected HandleTopPadding 8, got %f", theme.HandleTopPadding)
	}
	if theme.HandleBottomPadding != 8 {
		t.Errorf("Expected HandleBottomPadding 8, got %f", theme.HandleBottomPadding)
	}
	if theme.BackgroundColor != graphics.ColorWhite {
		t.Errorf("Expected BackgroundColor to be ColorWhite")
	}
}

func TestSnapPointPresets(t *testing.T) {
	if SnapThird.FractionalHeight != 0.33 {
		t.Errorf("SnapThird should be 0.33, got %f", SnapThird.FractionalHeight)
	}
	if SnapThird.Name != "third" {
		t.Errorf("SnapThird.Name should be 'third', got %q", SnapThird.Name)
	}

	if SnapHalf.FractionalHeight != 0.5 {
		t.Errorf("SnapHalf should be 0.5, got %f", SnapHalf.FractionalHeight)
	}
	if SnapHalf.Name != "half" {
		t.Errorf("SnapHalf.Name should be 'half', got %q", SnapHalf.Name)
	}

	if SnapFull.FractionalHeight != 1.0 {
		t.Errorf("SnapFull should be 1.0, got %f", SnapFull.FractionalHeight)
	}
	if SnapFull.Name != "full" {
		t.Errorf("SnapFull.Name should be 'full', got %q", SnapFull.Name)
	}
}

func TestNormalizeSnapBehavior_Defaults(t *testing.T) {
	behavior := normalizeSnapBehavior(SnapBehavior{})
	if behavior.DismissFactor <= 0 {
		t.Errorf("DismissFactor should be > 0")
	}
	if behavior.MinFlingVelocity <= 0 {
		t.Errorf("MinFlingVelocity should be > 0")
	}
	if behavior.SnapVelocityThreshold <= 0 {
		t.Errorf("SnapVelocityThreshold should be > 0")
	}
}

func TestBottomSheetState_FindTargetSnap(t *testing.T) {
	state := &bottomSheetState{
		snapHeights:  []float64{200, 400, 600},
		snapBehavior: normalizeSnapBehavior(SnapBehavior{}),
	}

	result := state.findTargetSnap(450, 0)
	if result != 400 {
		t.Errorf("findTargetSnap(450, 0) = %f, want 400", result)
	}

	result = state.findTargetSnap(450, 800)
	if result != 600 {
		t.Errorf("findTargetSnap(450, 800) = %f, want 600", result)
	}

	result = state.findTargetSnap(250, -800)
	if result != 200 {
		t.Errorf("findTargetSnap(250, -800) = %f, want 200", result)
	}
}
