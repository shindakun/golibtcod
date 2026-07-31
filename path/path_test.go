package path

import (
	"testing"

	"github.com/shindakun/golibtcod/fov"
)

func mapFrom(rows []string) *fov.Map {
	m := fov.NewMap(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			m.SetProperties(x, y, ch != '#', ch != '#')
		}
	}
	return m
}

func TestAStarStraightCorridor(t *testing.T) {
	m := mapFrom([]string{
		"#######",
		"#.....#",
		"#######",
	})
	p := NewUsingMap(m, 1.41)
	if !p.Compute(1, 1, 5, 1) {
		t.Fatal("no path found in corridor")
	}
	if p.Size() != 4 {
		t.Fatalf("path size = %d, want 4", p.Size())
	}
	// walk it
	steps := 0
	for {
		x, y, ok := p.Walk(false)
		if !ok {
			break
		}
		steps++
		if y != 1 || x < 1 || x > 5 {
			t.Fatalf("walked off corridor: (%d,%d)", x, y)
		}
	}
	if steps != 4 {
		t.Fatalf("walked %d steps, want 4", steps)
	}
}

func TestAStarAroundWall(t *testing.T) {
	m := mapFrom([]string{
		"#######",
		"#..#..#",
		"#..#..#",
		"#.....#",
		"#######",
	})
	p := NewUsingMap(m, 1.41)
	if !p.Compute(1, 1, 5, 1) {
		t.Fatal("no path around wall")
	}
	// must route through the gap at y=3
	sawDetour := false
	for i := 0; i < p.Size(); i++ {
		_, y := p.Get(i)
		if y == 3 {
			sawDetour = true
		}
	}
	if !sawDetour {
		t.Fatal("path did not detour through the gap")
	}
}

func TestAStarNoPath(t *testing.T) {
	m := mapFrom([]string{
		"#####",
		"#.#.#",
		"#####",
	})
	p := NewUsingMap(m, 1.41)
	if p.Compute(1, 1, 3, 1) {
		t.Fatal("found a path through a solid wall")
	}
}

func TestAStarCostFunc(t *testing.T) {
	// 3x3 open grid, but center cell is expensive: path should avoid it.
	fn := func(xFrom, yFrom, xTo, yTo int) float32 {
		if xTo == 1 && yTo == 1 {
			return 100.0
		}
		return 1.0
	}
	p := NewUsingFunc(3, 3, fn, 1.41)
	if !p.Compute(0, 1, 2, 1) {
		t.Fatal("no path with cost func")
	}
	for i := 0; i < p.Size(); i++ {
		x, y := p.Get(i)
		if x == 1 && y == 1 {
			t.Fatal("path went through the expensive cell")
		}
	}
}

func TestDijkstraDistances(t *testing.T) {
	m := mapFrom([]string{
		".....",
		".....",
		".....",
	})
	d := NewDijkstra(m, 1.41)
	d.Compute(0, 0)
	if got := d.Distance(0, 0); got != 0 {
		t.Fatalf("root distance = %f", got)
	}
	if got := d.Distance(4, 0); got != 4.0 {
		t.Fatalf("straight distance = %f, want 4", got)
	}
	// two diagonal steps: 2 * 1.41  (int cost 141 each, *0.01)
	if got := d.Distance(2, 2); got != 2.82 {
		t.Fatalf("diagonal distance = %f, want 2.82", got)
	}
}

func TestDijkstraPath(t *testing.T) {
	m := mapFrom([]string{
		"#######",
		"#..#..#",
		"#..#..#",
		"#.....#",
		"#######",
	})
	d := NewDijkstra(m, 1.41)
	d.Compute(1, 1)
	if !d.PathSet(5, 1) {
		t.Fatal("dijkstra found no path")
	}
	if d.IsEmpty() {
		t.Fatal("dijkstra path empty")
	}
	// walk to destination
	var lx, ly int
	for {
		x, y, ok := d.PathWalk()
		if !ok {
			break
		}
		lx, ly = x, y
	}
	if lx != 5 || ly != 1 {
		t.Fatalf("dijkstra walk ended at (%d,%d)", lx, ly)
	}
	if d.Distance(3, 0) != -1.0 { // wall: unreachable
		t.Fatal("wall should be unreachable")
	}
}
