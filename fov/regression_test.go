package fov

import "testing"

// Permissive(p) used to compute Permissive0+p unchecked, so an out-of-range
// grade silently aliased a neighbouring algorithm: Permissive(-1) ran
// recursive shadowcasting and returned a nil error.
func TestPermissiveRejectsOutOfRange(t *testing.T) {
	for _, p := range []int{-100, -9, -1, 9, 12, 100} {
		if got := Permissive(p); got != AlgorithmInvalid {
			t.Errorf("Permissive(%d) = %d, want AlgorithmInvalid", p, got)
		}
	}
	for p := 0; p <= 8; p++ {
		if got, want := Permissive(p), Permissive0+Algorithm(p); got != want {
			t.Errorf("Permissive(%d) = %d, want %d", p, got, want)
		}
	}
}

func TestComputeFovRejectsInvalidAlgorithm(t *testing.T) {
	m := NewMap(9, 9)
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			m.SetProperties(x, y, true, true)
		}
	}
	if err := m.ComputeFov(4, 4, 0, true, Permissive(-1)); err == nil {
		t.Error("ComputeFov accepted an out-of-range permissiveness")
	}
	if err := m.ComputeFov(4, 4, 0, true, Permissive(3)); err != nil {
		t.Errorf("ComputeFov rejected a valid grade: %v", err)
	}
}

// NewMap returns nil for non-positive dimensions, so the accessors must
// tolerate a nil receiver the way TCOD_map_get_width and friends do.
func TestNilMapAccessorsReturnZeroValues(t *testing.T) {
	m := NewMap(0, 10)
	if m != nil {
		t.Fatal("NewMap(0,10) should return nil")
	}
	if got := m.Width(); got != 0 {
		t.Errorf("Width() = %d, want 0", got)
	}
	if got := m.Height(); got != 0 {
		t.Errorf("Height() = %d, want 0", got)
	}
	if m.InBounds(0, 0) {
		t.Error("InBounds should be false on a nil map")
	}
	if m.IsTransparent(0, 0) || m.IsWalkable(0, 0) || m.InFov(0, 0) {
		t.Error("cell queries should be false on a nil map")
	}
	m.SetProperties(0, 0, true, true) // must not panic
	m.Clear(true, true)               // must not panic
}
