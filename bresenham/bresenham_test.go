package bresenham

import "testing"

func TestLineEndpoints(t *testing.T) {
	pts := Points(0, 0, 5, 2)
	if pts[0] != [2]int{0, 0} || pts[len(pts)-1] != [2]int{5, 2} {
		t.Fatalf("line endpoints wrong: %v", pts)
	}
	if len(pts) != 6 { // dominant axis + 1
		t.Fatalf("line length = %d, want 6 (%v)", len(pts), pts)
	}
}

func TestLineSymmetryCount(t *testing.T) {
	// Same number of cells in both directions (paths may differ per C).
	a := Points(0, 0, 7, 3)
	b := Points(7, 3, 0, 0)
	if len(a) != len(b) {
		t.Fatalf("asymmetric lengths %d vs %d", len(a), len(b))
	}
}

func TestVerticalHorizontal(t *testing.T) {
	if got := len(Points(2, 2, 2, 8)); got != 7 {
		t.Fatalf("vertical len = %d", got)
	}
	if got := len(Points(2, 2, 8, 2)); got != 7 {
		t.Fatalf("horizontal len = %d", got)
	}
	if got := len(Points(4, 4, 4, 4)); got != 1 {
		t.Fatalf("degenerate len = %d", got)
	}
}
