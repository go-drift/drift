package rendering

// PathOp represents a path operation type.
type PathOp int

const (
	PathOpMoveTo PathOp = iota
	PathOpLineTo
	PathOpQuadTo
	PathOpCubicTo
	PathOpClose
)

// PathFillRule represents the fill rule for a path.
type PathFillRule int

const (
	// FillRuleNonZero uses the nonzero winding rule.
	FillRuleNonZero PathFillRule = iota
	// FillRuleEvenOdd uses the even-odd rule (for paths with holes).
	FillRuleEvenOdd
)

// PathCommand represents a single path command with its arguments.
type PathCommand struct {
	Op   PathOp
	Args []float64
}

// Path represents a vector path consisting of move, line, and curve commands.
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
