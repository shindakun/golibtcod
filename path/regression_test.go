package path

import (
	"math/rand"
	"testing"

	"github.com/shindakun/golibtcod/fov"
)

// The Dijkstra pending queue could be driven one slot past the end of
// nodes[], panicking on ordinary varied-cost maps (~14% of random 8x8 maps
// with every cell walkable). C hides the same logic error behind a 4x
// over-allocation in TCOD_dijkstra_new_using_function.
func TestDijkstraQueueDoesNotOverflow(t *testing.T) {
	for seed := int64(0); seed < 2000; seed++ {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %d panicked: %v", seed, r)
				}
			}()
			rr := rand.New(rand.NewSource(seed))
			w, h := 2+rr.Intn(12), 2+rr.Intn(12)
			costs := make([]float32, w*h)
			for i := range costs {
				costs[i] = float32(rr.Intn(40)) / 10.0
			}
			d := NewDijkstraUsingFunc(w, h, func(a, b, cx, cy int) float32 {
				return costs[cy*w+cx]
			}, float32(rr.Intn(300))/100.0)
			d.Compute(rr.Intn(w), rr.Intn(h))
		}()
	}
}

// The specific 7x5 map that first exposed the overflow.
func TestDijkstraKnownOverflowCase(t *testing.T) {
	costs := []float32{
		0.7, 1.9, 0.1, 3.8, 2.5, 2.0, 1.6,
		2.0, 1.4, 3.1, 0.2, 0.9, 0.8, 3.4,
		1.1, 0.5, 3.7, 2.6, 1.5, 2.6, 0.8,
		1.8, 0.7, 2.7, 0.7, 0.8, 3.0, 1.5,
		2.1, 0.8, 2.7, 3.1, 2.9, 3.6, 1.7,
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	d := NewDijkstraUsingFunc(7, 5, func(a, b, cx, cy int) float32 {
		return costs[cy*7+cx]
	}, 0.31)
	d.Compute(5, 1)
	if got := d.Distance(0, 0); got < 0 {
		t.Errorf("root-adjacent cell unreachable: %v", got)
	}
}

// Compute succeeds with Size()==0 when origin equals destination, so Get(0)
// must not index path[-1]. Out-of-range indices return the origin.
func TestAStarGetOutOfRange(t *testing.T) {
	m := fov.NewMap(8, 8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			m.SetProperties(x, y, true, true)
		}
	}
	p := NewUsingMap(m, 1.41)

	if !p.Compute(2, 2, 2, 2) {
		t.Fatal("trivial Compute failed")
	}
	if x, y := p.Get(0); x != 2 || y != 2 {
		t.Errorf("empty-path Get(0) = (%d,%d), want origin (2,2)", x, y)
	}

	p.Compute(0, 0, 3, 0)
	size := p.Size()
	if size == 0 {
		t.Fatal("expected a non-empty path")
	}
	// Valid indices still walk the path.
	if x, y := p.Get(size - 1); x != 3 || y != 0 {
		t.Errorf("Get(last) = (%d,%d), want destination (3,0)", x, y)
	}
	for _, idx := range []int{-1, size, size + 50} {
		if x, y := p.Get(idx); x != 0 || y != 0 {
			t.Errorf("Get(%d) = (%d,%d), want origin (0,0)", idx, x, y)
		}
	}
}

func TestDijkstraGetOutOfRange(t *testing.T) {
	d := NewDijkstraUsingFunc(6, 6, func(a, b, cx, cy int) float32 { return 1.0 }, 1.41)
	d.Compute(0, 0)
	d.PathSet(3, 3)
	for _, idx := range []int{-1, d.Size(), d.Size() + 50} {
		if x, y := d.Get(idx); x != -1 || y != -1 {
			t.Errorf("Get(%d) = (%d,%d), want (-1,-1)", idx, x, y)
		}
	}
}
