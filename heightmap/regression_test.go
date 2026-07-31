package heightmap

import (
	"testing"

	"github.com/shindakun/golibtcod/rng"
)

// initSz is min(W,H)-1, so the corner seeding indexed Values[-1] when
// min(W,H)==1. min(W,H)==2 is safe (all four seed indices collapse to 0) and
// must still run, matching C.
func TestMidPointDisplacementDegenerateSizes(t *testing.T) {
	for _, d := range [][2]int{{1, 1}, {1, 5}, {5, 1}, {2, 2}, {3, 3}, {33, 33}} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%dx%d panicked: %v", d[0], d[1], r)
				}
			}()
			New(d[0], d[1]).MidPointDisplacement(rng.New(rng.MT, 1), 0.5)
		}()
	}
}

// nbCoef was clamped against nbPoints but not against len(coef), so a short
// coefficient slice indexed past the end.
func TestAddVoronoiShortCoefficientSlice(t *testing.T) {
	cases := []struct {
		nbPoints, nbCoef int
		coef             []float32
	}{
		{3, 3, []float32{1.0}},
		{3, 2, nil},
		{3, 2, []float32{1.0, -0.5}},
		{5, 4, []float32{1.0, -0.5, 0.25}},
	}
	for _, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("nbPoints=%d nbCoef=%d len(coef)=%d panicked: %v",
						c.nbPoints, c.nbCoef, len(c.coef), r)
				}
			}()
			New(8, 8).AddVoronoi(c.nbPoints, c.nbCoef, c.coef, rng.New(rng.MT, 1))
		}()
	}
}

// The guard must reject only min(W,H)==1. A 2x2 map seeds one corner in C,
// so rejecting it too would silently produce an all-zero heightmap.
func TestMidPointDisplacementGuardBoundary(t *testing.T) {
	nonzero := func(hm *HeightMap) int {
		n := 0
		for _, v := range hm.Values {
			if v != 0 {
				n++
			}
		}
		return n
	}
	for _, d := range [][2]int{{1, 1}, {1, 9}, {9, 1}} {
		hm := New(d[0], d[1])
		hm.MidPointDisplacement(rng.New(rng.MT, 1), 0.5)
		if got := nonzero(hm); got != 0 {
			t.Errorf("%dx%d: expected no writes, got %d", d[0], d[1], got)
		}
	}
	for _, d := range [][2]int{{2, 2}, {2, 9}, {9, 2}} {
		hm := New(d[0], d[1])
		hm.MidPointDisplacement(rng.New(rng.MT, 1), 0.5)
		if got := nonzero(hm); got == 0 {
			t.Errorf("%dx%d: guard is too broad, C seeds a corner here", d[0], d[1])
		}
	}
}
