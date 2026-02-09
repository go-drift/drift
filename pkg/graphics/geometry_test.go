package graphics

import "testing"

func TestRect_Intersects_OverlappingRects(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)
	b := RectFromLTWH(5, 5, 10, 10)
	if !a.Intersects(b) {
		t.Error("partially overlapping rects should intersect")
	}
}

func TestRect_Intersects_IdenticalRects(t *testing.T) {
	a := RectFromLTWH(10, 20, 30, 40)
	if !a.Intersects(a) {
		t.Error("identical rects should intersect")
	}
}

func TestRect_Intersects_ContainedRect(t *testing.T) {
	outer := RectFromLTWH(0, 0, 100, 100)
	inner := RectFromLTWH(10, 10, 20, 20)
	if !outer.Intersects(inner) {
		t.Error("contained rect should intersect with outer")
	}
	if !inner.Intersects(outer) {
		t.Error("outer rect should intersect with contained")
	}
}

func TestRect_Intersects_DisjointRects(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)
	b := RectFromLTWH(20, 20, 10, 10)
	if a.Intersects(b) {
		t.Error("disjoint rects should not intersect")
	}
}

func TestRect_Intersects_DisjointHorizontally(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)
	b := RectFromLTWH(15, 0, 10, 10) // same Y band, separated X
	if a.Intersects(b) {
		t.Error("horizontally disjoint rects should not intersect")
	}
}

func TestRect_Intersects_DisjointVertically(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)
	b := RectFromLTWH(0, 15, 10, 10) // same X band, separated Y
	if a.Intersects(b) {
		t.Error("vertically disjoint rects should not intersect")
	}
}

func TestRect_Intersects_TouchingEdges(t *testing.T) {
	// Touching edges (no overlap) — strict < comparison means false
	a := RectFromLTWH(0, 0, 10, 10)  // right=10
	b := RectFromLTWH(10, 0, 10, 10) // left=10
	if a.Intersects(b) {
		t.Error("rects touching at edge should not intersect (strict < comparison)")
	}
}

func TestRect_Intersects_TouchingCorners(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)   // bottom-right = (10, 10)
	b := RectFromLTWH(10, 10, 10, 10) // top-left = (10, 10)
	if a.Intersects(b) {
		t.Error("rects touching at corner should not intersect")
	}
}

func TestRect_Intersects_ZeroWidthRect(t *testing.T) {
	// Zero-width rect has Left==Right. Strict < comparison means
	// b.Left < a.Right is false when they're equal, so zero-width
	// rects on the boundary don't intersect.
	a := RectFromLTWH(10, 0, 0, 10)  // Left=10, Right=10
	b := RectFromLTWH(10, 0, 10, 10) // Left=10
	if a.Intersects(b) {
		t.Error("zero-width rect touching at edge should not intersect")
	}

	// But a zero-width rect INSIDE another rect does intersect
	// because Left < Right of the outer, and outer.Left < Right of inner.
	c := RectFromLTWH(5, 0, 0, 10)  // Left=5, Right=5
	d := RectFromLTWH(0, 0, 10, 10) // Left=0, Right=10
	// c: 5 < 10 (true) && 0 < 5 (true) → intersects
	if !c.Intersects(d) {
		t.Error("zero-width rect inside another should intersect (strict < passes)")
	}
}

func TestRect_Intersects_ZeroHeightRect(t *testing.T) {
	a := RectFromLTWH(0, 10, 10, 0)  // Top=10, Bottom=10
	b := RectFromLTWH(0, 10, 10, 10) // Top=10
	if a.Intersects(b) {
		t.Error("zero-height rect touching at edge should not intersect")
	}

	c := RectFromLTWH(0, 5, 10, 0)  // Top=5, Bottom=5
	d := RectFromLTWH(0, 0, 10, 10) // Top=0, Bottom=10
	if !c.Intersects(d) {
		t.Error("zero-height rect inside another should intersect (strict < passes)")
	}
}

func TestRect_Intersects_NegativeCoordinates(t *testing.T) {
	a := RectFromLTWH(-20, -20, 30, 30) // (-20,-20) to (10,10)
	b := RectFromLTWH(-5, -5, 20, 20)   // (-5,-5) to (15,15)
	if !a.Intersects(b) {
		t.Error("overlapping rects with negative coords should intersect")
	}
}

func TestRect_Intersects_Symmetric(t *testing.T) {
	a := RectFromLTWH(0, 0, 10, 10)
	b := RectFromLTWH(5, 5, 10, 10)
	if a.Intersects(b) != b.Intersects(a) {
		t.Error("Intersects should be symmetric")
	}

	c := RectFromLTWH(20, 20, 10, 10)
	if a.Intersects(c) != c.Intersects(a) {
		t.Error("Intersects should be symmetric for disjoint rects")
	}
}
