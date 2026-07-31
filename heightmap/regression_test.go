package heightmap

import (
	"testing"

	"golibtcod/rng"
)

// initSz is min(W,H)-1, so the corner seeding indexed Values[-1] on a 1x1
// map. Displacement needs at least a 2x2 grid to have a midpoint.
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
