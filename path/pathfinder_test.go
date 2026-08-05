package path

import "testing"

func uniform(w, h int, c int32) []int32 {
	g := make([]int32, w*h)
	for i := range g {
		g[i] = c
	}
	return g
}

func TestPathfinderUniformField(t *testing.T) {
	p := NewPathfinder(4, 3, uniform(4, 3, 1), 1, 0)
	p.AddRoot(0, 0)
	p.Compute()
	want := [][]int32{{0, 1, 2, 3}, {1, 2, 3, 4}, {2, 3, 4, 5}}
	for y := range want {
		for x := range want[y] {
			if got := p.Distance(x, y); got != want[y][x] {
				t.Errorf("(%d,%d) = %d, want %d", x, y, got, want[y][x])
			}
		}
	}
}

// A cost of 0 or less is impassable, so a wall column splits the grid.
func TestPathfinderWallsBlock(t *testing.T) {
	cost := uniform(5, 3, 1)
	for y := 0; y < 3; y++ {
		cost[y*5+2] = 0
	}
	p := NewPathfinder(5, 3, cost, 1, 0)
	p.AddRoot(0, 0)
	p.Compute()
	if d := p.Distance(1, 0); d == PathfinderUnreachable {
		t.Error("near side should be reachable")
	}
	for y := 0; y < 3; y++ {
		if d := p.Distance(4, y); d != PathfinderUnreachable {
			t.Errorf("(4,%d) = %d, want unreachable behind the wall", y, d)
		}
	}
}

func TestPathfinderTerrainCosts(t *testing.T) {
	cost := uniform(3, 1, 1)
	cost[1] = 5 // expensive middle cell
	p := NewPathfinder(3, 1, cost, 1, 0)
	p.AddRoot(0, 0)
	p.Compute()
	if got := p.Distance(1, 0); got != 5 {
		t.Errorf("cost-5 cell = %d, want 5", got)
	}
	if got := p.Distance(2, 0); got != 6 {
		t.Errorf("beyond it = %d, want 6", got)
	}
}

func TestPathfinderDiagonal(t *testing.T) {
	p := NewPathfinder(3, 3, uniform(3, 3, 1), 2, 3)
	p.AddRoot(0, 0)
	p.Compute()
	if got := p.Distance(1, 1); got != 3 {
		t.Errorf("diagonal step = %d, want 3", got)
	}
	// Diagonal disabled: the corner costs two cardinal steps.
	q := NewPathfinder(3, 3, uniform(3, 3, 1), 2, 0)
	q.AddRoot(0, 0)
	q.Compute()
	if got := q.Distance(1, 1); got != 4 {
		t.Errorf("no-diagonal corner = %d, want 4", got)
	}
}

func TestPathfinderMultipleRoots(t *testing.T) {
	p := NewPathfinder(5, 1, uniform(5, 1, 1), 1, 0)
	p.AddRoot(0, 0)
	p.AddRoot(4, 0)
	p.Compute()
	// The midpoint is equidistant from both seeds.
	if got := p.Distance(2, 0); got != 2 {
		t.Errorf("midpoint = %d, want 2", got)
	}
}

func TestPathfinderPathTo(t *testing.T) {
	p := NewPathfinder(4, 1, uniform(4, 1, 1), 1, 0)
	p.AddRoot(0, 0)
	p.Compute()
	got := p.PathTo(3, 0)
	want := []Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}}
	if len(got) != len(want) {
		t.Fatalf("path = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d = %v, want %v", i, got[i], want[i])
		}
	}
	if p.PathTo(0, 0) == nil {
		t.Error("path to the seed itself should not be nil")
	}
}

func TestPathfinderUnreachableCells(t *testing.T) {
	p := NewPathfinder(3, 1, []int32{1, 0, 1}, 1, 0)
	p.AddRoot(0, 0)
	p.Compute()
	if d := p.Distance(2, 0); d != PathfinderUnreachable {
		t.Errorf("walled-off cell = %d, want unreachable", d)
	}
	if p.PathTo(2, 0) != nil {
		t.Error("PathTo an unreachable cell should be nil")
	}
}

// ComputeStep drives the search incrementally.
func TestPathfinderIncremental(t *testing.T) {
	p := NewPathfinder(4, 4, uniform(4, 4, 1), 1, 0)
	p.AddRoot(0, 0)
	steps := 0
	for p.ComputeStep() {
		steps++
		if steps > 1000 {
			t.Fatal("ComputeStep did not terminate")
		}
	}
	if p.Distance(3, 3) != 6 {
		t.Errorf("far corner = %d, want 6", p.Distance(3, 3))
	}
}

func TestPathfinderRecompile(t *testing.T) {
	p := NewPathfinder(4, 1, uniform(4, 1, 1), 1, 0)
	d := p.Distances()
	d[0] = 0 // seed by writing the field directly
	p.Recompile()
	p.Compute()
	if got := p.Distance(3, 0); got != 3 {
		t.Errorf("after Recompile = %d, want 3", got)
	}
}

func TestPathfinderDegenerate(t *testing.T) {
	if NewPathfinder(0, 4, nil, 1, 0) != nil {
		t.Error("zero width should return nil")
	}
	if NewPathfinder(4, 4, uniform(2, 2, 1), 1, 0) != nil {
		t.Error("mismatched cost length should return nil")
	}
	p := NewPathfinder(1, 1, uniform(1, 1, 1), 1, 1)
	p.AddRoot(0, 0)
	p.Compute()
	if got := p.Distance(0, 0); got != 0 {
		t.Errorf("1x1 seed = %d, want 0", got)
	}
	// Out-of-range queries must not panic.
	if p.Distance(-1, 0) != PathfinderUnreachable || p.Distance(9, 9) != PathfinderUnreachable {
		t.Error("out-of-range Distance should be unreachable")
	}
	p.AddRoot(-5, -5) // must be ignored
}
