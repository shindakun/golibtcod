package heightmap

import (
	"testing"

	"golibtcod/noise"
	"golibtcod/rng"
)

func TestAddHillAndNormalize(t *testing.T) {
	hm := New(20, 20)
	hm.AddHill(10, 10, 5, 1.0)
	if hm.At(10, 10) <= 0 {
		t.Fatal("hill center not raised")
	}
	if hm.At(0, 0) != 0 {
		t.Fatal("far corner should be untouched")
	}
	hm.Normalize(0, 1)
	min, max := hm.MinMax()
	if min != 0 || max != 1 {
		t.Fatalf("normalize: min=%f max=%f", min, max)
	}
}

func TestInterpolatedValue(t *testing.T) {
	hm := New(2, 2)
	hm.Set(0, 0, 0)
	hm.Set(1, 0, 1)
	hm.Set(0, 1, 0)
	hm.Set(1, 1, 1)
	if v := hm.InterpolatedValue(0.5, 0.5); v != 0.5 {
		t.Fatalf("interpolated center = %f", v)
	}
}

func TestFbmAndErosion(t *testing.T) {
	r := rng.New(rng.CMWC, 3)
	n := noise.New(2, noise.DefaultHurst, noise.DefaultLacunarity, r)
	hm := New(33, 33)
	hm.AddFbm(n, 3, 3, 0, 0, 5, 0, 1)
	min, max := hm.MinMax()
	if min == max {
		t.Fatal("fbm produced a flat map")
	}
	before := hm.CountCells(-1000, 1000)
	hm.RainErosion(500, 0.05, 0.05, r)
	after := hm.CountCells(-1000, 1000)
	if before != after {
		t.Fatal("erosion changed cell count?!")
	}
}

func TestMidPointDisplacement(t *testing.T) {
	r := rng.New(rng.MT, 11)
	hm := New(33, 33) // 2^5+1
	hm.MidPointDisplacement(r, 0.5)
	min, max := hm.MinMax()
	if min == max {
		t.Fatal("MPD produced a flat map")
	}
}

func TestKernelSmooth(t *testing.T) {
	hm := New(10, 10)
	hm.Set(5, 5, 100)
	dx := []int{-1, 0, 1, 0, 0}
	dy := []int{0, -1, 0, 1, 0}
	w := []float32{1, 1, 1, 1, 4}
	hm.KernelTransform(dx, dy, w, -1e30, 1e30)
	if hm.At(5, 5) >= 100 {
		t.Fatal("smoothing did not reduce the spike")
	}
	if hm.At(5, 4) <= 0 {
		t.Fatal("smoothing did not spread the spike")
	}
}
