package layout

// EdgeInsets represents padding/margin on four sides.
type EdgeInsets struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// EdgeInsetsAll creates uniform padding on all sides.
func EdgeInsetsAll(value float64) EdgeInsets {
	return EdgeInsets{Left: value, Top: value, Right: value, Bottom: value}
}

// EdgeInsetsSymmetric creates symmetric padding.
func EdgeInsetsSymmetric(horizontal, vertical float64) EdgeInsets {
	return EdgeInsets{Left: horizontal, Right: horizontal, Top: vertical, Bottom: vertical}
}

// EdgeInsetsOnly creates padding with explicit values.
func EdgeInsetsOnly(left, top, right, bottom float64) EdgeInsets {
	return EdgeInsets{Left: left, Top: top, Right: right, Bottom: bottom}
}

// Horizontal returns the sum of left and right padding.
func (e EdgeInsets) Horizontal() float64 {
	return e.Left + e.Right
}

// Vertical returns the sum of top and bottom padding.
func (e EdgeInsets) Vertical() float64 {
	return e.Top + e.Bottom
}

// Add returns a new EdgeInsets with uniform padding added to all sides.
func (e EdgeInsets) Add(all float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left + all,
		Top:    e.Top + all,
		Right:  e.Right + all,
		Bottom: e.Bottom + all,
	}
}

// AddHorizontal returns a new EdgeInsets with padding added to left and right.
func (e EdgeInsets) AddHorizontal(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left + value,
		Top:    e.Top,
		Right:  e.Right + value,
		Bottom: e.Bottom,
	}
}

// AddVertical returns a new EdgeInsets with padding added to top and bottom.
func (e EdgeInsets) AddVertical(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left,
		Top:    e.Top + value,
		Right:  e.Right,
		Bottom: e.Bottom + value,
	}
}

// AddTop returns a new EdgeInsets with padding added to top.
func (e EdgeInsets) AddTop(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left,
		Top:    e.Top + value,
		Right:  e.Right,
		Bottom: e.Bottom,
	}
}

// AddBottom returns a new EdgeInsets with padding added to bottom.
func (e EdgeInsets) AddBottom(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left,
		Top:    e.Top,
		Right:  e.Right,
		Bottom: e.Bottom + value,
	}
}

// AddLeft returns a new EdgeInsets with padding added to left.
func (e EdgeInsets) AddLeft(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left + value,
		Top:    e.Top,
		Right:  e.Right,
		Bottom: e.Bottom,
	}
}

// AddRight returns a new EdgeInsets with padding added to right.
func (e EdgeInsets) AddRight(value float64) EdgeInsets {
	return EdgeInsets{
		Left:   e.Left,
		Top:    e.Top,
		Right:  e.Right + value,
		Bottom: e.Bottom,
	}
}

// OnlyTop returns a new EdgeInsets with only the top value preserved.
func (e EdgeInsets) OnlyTop() EdgeInsets {
	return EdgeInsets{Top: e.Top}
}

// OnlyBottom returns a new EdgeInsets with only the bottom value preserved.
func (e EdgeInsets) OnlyBottom() EdgeInsets {
	return EdgeInsets{Bottom: e.Bottom}
}

// OnlyLeft returns a new EdgeInsets with only the left value preserved.
func (e EdgeInsets) OnlyLeft() EdgeInsets {
	return EdgeInsets{Left: e.Left}
}

// OnlyRight returns a new EdgeInsets with only the right value preserved.
func (e EdgeInsets) OnlyRight() EdgeInsets {
	return EdgeInsets{Right: e.Right}
}

// OnlyHorizontal returns a new EdgeInsets with only left and right preserved.
func (e EdgeInsets) OnlyHorizontal() EdgeInsets {
	return EdgeInsets{Left: e.Left, Right: e.Right}
}

// OnlyVertical returns a new EdgeInsets with only top and bottom preserved.
func (e EdgeInsets) OnlyVertical() EdgeInsets {
	return EdgeInsets{Top: e.Top, Bottom: e.Bottom}
}
