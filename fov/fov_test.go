package fov

import "testing"

func mapFrom(rows []string) *Map {
	m := NewMap(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			m.SetProperties(x, y, ch != '#', ch != '#')
		}
	}
	return m
}

var allAlgos = []Algorithm{
	Basic, Diamond, Shadow,
	Permissive0, Permissive4, Permissive8,
	Restrictive, SymmetricShadowcast,
}

func TestPovAlwaysVisible(t *testing.T) {
	m := mapFrom([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	for _, algo := range allAlgos {
		if err := m.ComputeFov(2, 2, 2, true, algo); err != nil {
			t.Fatalf("algo %d: %v", algo, err)
		}
		if !m.InFov(2, 2) {
			t.Fatalf("algo %d: POV not visible", algo)
		}
	}
}

func TestOpenRoomFullyVisible(t *testing.T) {
	m := mapFrom([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	for _, algo := range allAlgos {
		if err := m.ComputeFov(2, 2, 0, true, algo); err != nil {
			t.Fatalf("algo %d: %v", algo, err)
		}
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				if !m.InFov(x, y) {
					t.Fatalf("algo %d: open cell (%d,%d) not visible", algo, x, y)
				}
			}
		}
	}
}

func TestPillarCastsShadow(t *testing.T) {
	// A pillar due east of the POV must hide the cell directly behind it.
	m := mapFrom([]string{
		".......",
		".......",
		"..@#...",
		".......",
		".......",
	})
	for _, algo := range allAlgos {
		if err := m.ComputeFov(2, 2, 0, true, algo); err != nil {
			t.Fatalf("algo %d: %v", algo, err)
		}
		if !m.InFov(3, 2) {
			t.Fatalf("algo %d: pillar itself not lit", algo)
		}
		if m.InFov(6, 2) {
			t.Fatalf("algo %d: cell far behind pillar is visible", algo)
		}
	}
}

func TestLightWallsFalseHidesWalls(t *testing.T) {
	m := mapFrom([]string{
		".....",
		".###.",
		".#.#.",
		".###.",
		".....",
	})
	for _, algo := range allAlgos {
		if err := m.ComputeFov(2, 2, 0, false, algo); err != nil {
			t.Fatalf("algo %d: %v", algo, err)
		}
		if m.InFov(1, 1) || m.InFov(2, 1) || m.InFov(3, 2) {
			t.Fatalf("algo %d: wall lit despite lightWalls=false", algo)
		}
		if !m.InFov(2, 2) {
			t.Fatalf("algo %d: POV lost", algo)
		}
	}
}

func TestRadiusLimits(t *testing.T) {
	m := mapFrom([]string{
		"...........",
		"...........",
		"...........",
		"...........",
		"...........",
	})
	for _, algo := range allAlgos {
		if err := m.ComputeFov(5, 2, 2, true, algo); err != nil {
			t.Fatalf("algo %d: %v", algo, err)
		}
		if m.InFov(10, 2) || m.InFov(0, 2) {
			t.Fatalf("algo %d: cells beyond radius visible", algo)
		}
		if !m.InFov(6, 2) {
			t.Fatalf("algo %d: adjacent cell not visible", algo)
		}
	}
}

// The symmetric shadowcast's core promise: if A sees B then B sees A
// (unlimited radius, floors only).
func TestSymmetricShadowcastSymmetry(t *testing.T) {
	rows := []string{
		"..#....",
		".......",
		"..##...",
		".......",
		"...#...",
		".......",
	}
	m := mapFrom(rows)
	type pt struct{ x, y int }
	var floors []pt
	for y, row := range rows {
		for x, ch := range row {
			if ch != '#' {
				floors = append(floors, pt{x, y})
			}
		}
	}
	sees := func(a, b pt) bool {
		if err := m.ComputeFov(a.x, a.y, 0, true, SymmetricShadowcast); err != nil {
			t.Fatal(err)
		}
		return m.InFov(b.x, b.y)
	}
	for _, a := range floors {
		for _, b := range floors {
			ab := sees(a, b)
			ba := sees(b, a)
			if ab != ba {
				t.Fatalf("asymmetry: %v sees %v = %v but reverse = %v", a, b, ab, ba)
			}
		}
	}
}
