package rng

import "testing"

// The MT path is canonical MT19937 (init 1812433253, standard twist and
// tempering), so seed 5489 must yield the textbook first outputs.
func TestMT19937KnownStream(t *testing.T) {
	r := New(MT, 5489)
	want := []uint32{3499211612, 581869302, 3890346734, 3586334585, 545404204}
	for i, w := range want {
		if got := r.U32(); got != w {
			t.Fatalf("MT output %d = %d, want %d", i, got, w)
		}
	}
}

func TestDeterminismAndSaveRestore(t *testing.T) {
	for _, algo := range []Algorithm{MT, CMWC} {
		a := New(algo, 1234)
		b := New(algo, 1234)
		for i := 0; i < 1000; i++ {
			if a.U32() != b.U32() {
				t.Fatalf("algo %d: same seed diverged at %d", algo, i)
			}
		}
		snap := a.Save()
		x := []uint32{a.U32(), a.U32(), a.U32()}
		a.Restore(snap)
		for i, w := range x {
			if got := a.U32(); got != w {
				t.Fatalf("algo %d: restore mismatch at %d: %d != %d", algo, i, got, w)
			}
		}
	}
}

func TestGetIBounds(t *testing.T) {
	r := New(CMWC, 42)
	for i := 0; i < 10000; i++ {
		v := r.GetI(3, 9)
		if v < 3 || v > 9 {
			t.Fatalf("GetI out of range: %d", v)
		}
	}
	if r.GetI(5, 5) != 5 {
		t.Fatal("GetI(5,5) != 5")
	}
	// swapped bounds
	v := r.GetI(9, 3)
	if v < 3 || v > 9 {
		t.Fatalf("GetI swapped out of range: %d", v)
	}
}

func TestDice(t *testing.T) {
	d := ParseDice("3d6+2")
	if d.Rolls != 3 || d.Faces != 6 || d.AddSub != 2 || d.Multiplier != 1 {
		t.Fatalf("ParseDice 3d6+2 = %+v", d)
	}
	d = ParseDice("1.5x2d10-1")
	if d.Rolls != 2 || d.Faces != 10 || d.AddSub != -1 || d.Multiplier != 1.5 {
		t.Fatalf("ParseDice 1.5x2d10-1 = %+v", d)
	}
	r := New(MT, 7)
	for i := 0; i < 1000; i++ {
		v := r.RollS("3d6")
		if v < 3 || v > 18 {
			t.Fatalf("3d6 rolled %d", v)
		}
	}
}

func TestGaussianRangeStaysInRange(t *testing.T) {
	r := New(CMWC, 99)
	r.SetDistribution(GaussianRange)
	for i := 0; i < 5000; i++ {
		v := r.GetInt(-10, 10)
		if v < -10 || v > 10 {
			t.Fatalf("gaussian range out of bounds: %d", v)
		}
	}
}
