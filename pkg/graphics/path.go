package graphics

import (
	"fmt"
	"math"
)

// PathOp represents a path drawing operation type.
type PathOp int

const (
	PathOpMoveTo  PathOp = iota // Start new subpath at point (x, y)
	PathOpLineTo                // Draw line to point (x, y)
	PathOpQuadTo                // Draw quadratic curve to (x2, y2) via control (x1, y1)
	PathOpCubicTo               // Draw cubic curve to (x3, y3) via controls (x1, y1), (x2, y2)
	PathOpClose                 // Close subpath with line to start point
)

// String returns a human-readable representation of the path operation.
func (o PathOp) String() string {
	switch o {
	case PathOpMoveTo:
		return "move_to"
	case PathOpLineTo:
		return "line_to"
	case PathOpQuadTo:
		return "quad_to"
	case PathOpCubicTo:
		return "cubic_to"
	case PathOpClose:
		return "close"
	default:
		return fmt.Sprintf("PathOp(%d)", int(o))
	}
}

// PathFillRule determines how path interiors are calculated for filling.
type PathFillRule int

const (
	// FillRuleNonZero fills regions with nonzero winding count.
	// A point is inside if a ray from it crosses more left-to-right edges
	// than right-to-left edges (or vice versa).
	FillRuleNonZero PathFillRule = iota

	// FillRuleEvenOdd fills regions crossed an odd number of times.
	// Useful for creating holes: nested shapes alternate between filled/unfilled.
	FillRuleEvenOdd
)

// String returns a human-readable representation of the path fill rule.
func (r PathFillRule) String() string {
	switch r {
	case FillRuleNonZero:
		return "nonzero"
	case FillRuleEvenOdd:
		return "evenodd"
	default:
		return fmt.Sprintf("PathFillRule(%d)", int(r))
	}
}

// PathCommand represents a single path operation with its coordinate arguments.
type PathCommand struct {
	Op   PathOp    // The operation type
	Args []float64 // Coordinates: MoveTo/LineTo=[x,y], QuadTo=[x1,y1,x2,y2], CubicTo=[x1,y1,x2,y2,x3,y3]
}

// Path represents a vector path for drawing or clipping arbitrary shapes.
//
// Build paths using MoveTo, LineTo, QuadTo, CubicTo, and Close methods.
// Use with Canvas.DrawPath to stroke/fill, or Canvas.ClipPath to clip.
type Path struct {
	Commands []PathCommand
	FillRule PathFillRule
}

// NewPath creates a new empty path with nonzero fill rule.
func NewPath() *Path {
	return &Path{FillRule: FillRuleNonZero}
}

// NewPathWithFillRule creates a new empty path with the specified fill rule.
func NewPathWithFillRule(fillRule PathFillRule) *Path {
	return &Path{FillRule: fillRule}
}

// MoveTo starts a new subpath at the given point.
func (p *Path) MoveTo(x, y float64) {
	p.Commands = append(p.Commands, PathCommand{
		Op:   PathOpMoveTo,
		Args: []float64{x, y},
	})
}

// LineTo adds a line segment from the current point to (x, y).
func (p *Path) LineTo(x, y float64) {
	p.Commands = append(p.Commands, PathCommand{
		Op:   PathOpLineTo,
		Args: []float64{x, y},
	})
}

// QuadTo adds a quadratic bezier curve from the current point to (x2, y2)
// with control point (x1, y1).
func (p *Path) QuadTo(x1, y1, x2, y2 float64) {
	p.Commands = append(p.Commands, PathCommand{
		Op:   PathOpQuadTo,
		Args: []float64{x1, y1, x2, y2},
	})
}

// CubicTo adds a cubic bezier curve from the current point to (x3, y3)
// with control points (x1, y1) and (x2, y2).
func (p *Path) CubicTo(x1, y1, x2, y2, x3, y3 float64) {
	p.Commands = append(p.Commands, PathCommand{
		Op:   PathOpCubicTo,
		Args: []float64{x1, y1, x2, y2, x3, y3},
	})
}

// Close closes the current subpath by drawing a line to the starting point.
func (p *Path) Close() {
	p.Commands = append(p.Commands, PathCommand{
		Op: PathOpClose,
	})
}

// IsEmpty returns true if the path has no commands.
func (p *Path) IsEmpty() bool {
	return len(p.Commands) == 0
}

// Clear removes all commands from the path.
func (p *Path) Clear() {
	p.Commands = p.Commands[:0]
}

// Bounds returns the bounding rectangle of the path.
// Returns an empty Rect if the path has no points.
func (p *Path) Bounds() Rect {
	if p == nil || len(p.Commands) == 0 {
		return Rect{}
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, cmd := range p.Commands {
		for i := 0; i+1 < len(cmd.Args); i += 2 {
			x, y := cmd.Args[i], cmd.Args[i+1]
			minX = math.Min(minX, x)
			minY = math.Min(minY, y)
			maxX = math.Max(maxX, x)
			maxY = math.Max(maxY, y)
		}
	}
	if math.IsInf(minX, 1) {
		return Rect{}
	}
	return Rect{Left: minX, Top: minY, Right: maxX, Bottom: maxY}
}
