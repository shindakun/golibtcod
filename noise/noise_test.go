package noise

import (
	"math"
	"testing"

	"golibtcod/rng"
)

func TestRangeAndDeterminism(t *testing.T) {
	for _, typ := range []Type{Perlin, Simplex, Wavelet} {
		a := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 42))
		b := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 42))
		a.SetType(typ)
		b.SetType(typ)
		for i := 0; i < 500; i++ {
			f := []float32{float32(i) * 0.13, float32(i) * 0.07}
			va := a.Get(f...)
			vb := b.Get(f...)
			if va != vb {
				t.Fatalf("type %d: nondeterministic at %d: %f != %f", typ, i, va, vb)
			}
			if va < -1 || va > 1 || math.IsNaN(float64(va)) {
				t.Fatalf("type %d: out of range: %f", typ, va)
			}
		}
	}
}

func TestSeedsDiffer(t *testing.T) {
	a := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 1))
	b := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 2))
	same := 0
	for i := 0; i < 100; i++ {
		f := []float32{float32(i) * 0.31, 0.5}
		if a.Get(f...) == b.Get(f...) {
			same++
		}
	}
	if same > 5 {
		t.Fatalf("different seeds produced %d/100 identical samples", same)
	}
}

func TestAllDimensions(t *testing.T) {
	for ndim := 1; ndim <= 4; ndim++ {
		n := New(ndim, DefaultHurst, DefaultLacunarity, rng.New(rng.MT, 9))
		f := make([]float32, ndim)
		for i := range f {
			f[i] = 0.7 * float32(i+1)
		}
		for _, typ := range []Type{Perlin, Simplex} {
			v := n.GetEx(f, typ)
			if v < -1 || v > 1 || math.IsNaN(float64(v)) {
				t.Fatalf("ndim %d type %d: bad value %f", ndim, typ, v)
			}
		}
	}
}

func TestFbmTurbulence(t *testing.T) {
	n := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 5))
	for i := 0; i < 200; i++ {
		f := []float32{float32(i) * 0.17, float32(i) * 0.11}
		v1 := n.GetFbm(f, 4.5)
		v2 := n.GetTurbulence(f, 4.5)
		if v1 < -1 || v1 > 1 || v2 < -1 || v2 > 1 {
			t.Fatalf("fractal out of range: fbm=%f turb=%f", v1, v2)
		}
		if v2 < -1 { // turbulence sums abs values then clamps
			t.Fatalf("turbulence negative below clamp: %f", v2)
		}
	}
}

func TestContinuity(t *testing.T) {
	// Noise must be continuous: close inputs -> close outputs.
	n := New(2, DefaultHurst, DefaultLacunarity, rng.New(rng.CMWC, 77))
	for i := 0; i < 100; i++ {
		x := float32(i) * 0.23
		a := n.Get(x, 1.5)
		b := n.Get(x+1e-4, 1.5)
		if diff := math.Abs(float64(a - b)); diff > 0.01 {
			t.Fatalf("discontinuity at %f: |%f-%f| = %f", x, a, b, diff)
		}
	}
}
