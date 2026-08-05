// Dijkstra over a caller-supplied cost grid, a faithful Go port of
// libtcod's pathfinder.c, pathfinder_frontier.c and heapq.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// # Relationship to the rest of this package
//
// AStar and Dijkstra port path_c.c, libtcod's original pathfinders. This
// file ports the newer TCOD_Pathfinder, which is a different thing: it
// computes a distance field over a cost grid the caller owns, and records
// the traversal so a path can be walked back from any cell.
//
// # Deliberate API divergence
//
// The C API is built around raw pointers, per-axis strides and a runtime
// int_type tag selecting uint8 through int64:
//
//	void TCOD_pf_set_graph2d_pointer(path, void* data, int int_type,
//	                                 const size_t* strides, int cardinal, int diagonal);
//
// That is a NumPy array descriptor. It exists so python-tcod can hand a
// NumPy array straight to C without copying, and reproducing it in Go would
// be ceremony around what the type system already provides. A Go caller has
// a []int32 whose length and shape it knows.
//
// So the storage machinery is replaced by ordinary Go slices, keeping the
// seeding rule, the cardinal-then-diagonal edge order and the traversal
// encoding.
//
// C's ndim reaches 4, but TCOD_pf_compute_step calls only
// TCOD_pf_basic2d_edges, which hardcodes origin[0] and origin[1]. The search
// is therefore 2D upstream regardless of the declared dimension count, and
// this port is 2D for the same reason.
//
// # This implements the intended behavior, not the shipped behavior
//
// Upstream pathfinder.c cannot compute a path. Four defects, all confirmed
// against libtcod HEAD c54823e and none covered by an upstream issue:
//
//  1. TCOD_pf_in_bounds tests `0 < index[i]` where it means `0 >`, so every
//     positive coordinate is rejected and only (0,0) is in bounds.
//  2. TCOD_pf_add_edge's relaxation test is inverted; distances start at
//     INT_MAX so an unvisited neighbor always returns early.
//  3. graph.cost is written by the setter and never read, so the caller's
//     cost grid is dead and walls have no effect.
//  4. TCOD_pf_set_traversal_pointer stores a byte stride in a shape field.
//
// Nothing inside libtcod calls TCOD_pf_*, which is why this goes unnoticed
// there; the module is a backend for python-tcod.
//
// This port therefore checks bounds correctly, relaxes toward lower
// distances, and honors the cost grid with non-positive cells impassable,
// which is what the C API documents. It is verified against a C build
// corrected on all four counts: 250 randomized distance fields match
// exactly. See internal/fixtures/pathfinder/README.md.

package path

import "container/heap"

// PathfinderUnreachable marks a cell the search has not reached. It is the
// C sentinel: distance arrays are filled with the maximum value of their
// integer type, and TCOD_pf_recompile_cb skips any cell still holding it.
const PathfinderUnreachable = int32(0x7FFFFFFF)

// Pathfinder mirrors TCOD_Pathfinder: a distance field over a cost grid,
// plus the traversal needed to walk a path back to a seed.
type Pathfinder struct {
	w, h int

	// cost is the per-cell entry cost, row-major, len == w*h. A cost of 0
	// or less makes a cell impassable, matching the C graph semantics.
	cost []int32

	// cardinal and diagonal are the edge multipliers. A value of 0 or less
	// disables that movement class, exactly as C tests `> 0`.
	cardinal, diagonal int

	// distance is the computed field, row-major. PathfinderUnreachable
	// means "not reached".
	distance []int32

	// traversal[i] is the cell stepped from to reach cell i, or -1. C
	// stores this as an ndim+1 dimensional array of per-axis coordinates;
	// a flat predecessor index carries the same information in 2D.
	traversal []int32

	frontier *frontier
}

// NewPathfinder is TCOD_pf_new plus TCOD_pf_set_graph2d_pointer: build a
// pathfinder over a w*h cost grid.
//
// cost is retained, not copied, so a caller can mutate terrain and call
// Recompile. cardinal and diagonal are the movement costs multiplied into
// each step; pass 0 for diagonal to forbid diagonal movement, which is what
// C's `if (path->graph.diagonal > 0)` does.
//
// Returns nil if the dimensions are non-positive or cost is the wrong
// length.
func NewPathfinder(w, h int, cost []int32, cardinal, diagonal int) *Pathfinder {
	if w <= 0 || h <= 0 || len(cost) != w*h {
		return nil
	}
	p := &Pathfinder{
		w: w, h: h,
		cost:      cost,
		cardinal:  cardinal,
		diagonal:  diagonal,
		distance:  make([]int32, w*h),
		traversal: make([]int32, w*h),
		frontier:  &frontier{},
	}
	p.Clear()
	return p
}

// Clear resets every cell to unreachable and empties the frontier.
func (p *Pathfinder) Clear() {
	for i := range p.distance {
		p.distance[i] = PathfinderUnreachable
		p.traversal[i] = -1
	}
	p.frontier.reset()
}

// Width and Height report the grid size.
func (p *Pathfinder) Width() int  { return p.w }
func (p *Pathfinder) Height() int { return p.h }

func (p *Pathfinder) inBounds(x, y int) bool {
	return 0 <= x && x < p.w && 0 <= y && y < p.h
}

// AddRoot seeds a cell at distance 0, the usual starting point. Multiple
// roots give a multi-source field, which is how a Dijkstra map is built.
func (p *Pathfinder) AddRoot(x, y int) { p.SetDistance(x, y, 0) }

// SetDistance seeds a cell at an explicit distance and queues it. C has no
// single function for this: python-tcod writes the distance array directly
// and calls TCOD_pf_recompile, which pushes every non-maximum cell. Doing
// both here keeps that behavior without exposing the raw array.
func (p *Pathfinder) SetDistance(x, y int, dist int32) {
	if !p.inBounds(x, y) {
		return
	}
	i := y*p.w + x
	p.distance[i] = dist
	p.traversal[i] = -1
	p.frontier.push(frontierNode{dist: dist, index: int32(i)})
}

// Recompile is TCOD_pf_recompile: rebuild the frontier from the current
// distance field, queuing every cell that is not unreachable. Call it after
// writing distances through Distances.
func (p *Pathfinder) Recompile() {
	p.frontier.reset()
	for i, d := range p.distance {
		if d == PathfinderUnreachable {
			continue // C: array_is_max -> skip
		}
		p.frontier.push(frontierNode{dist: d, index: int32(i)})
	}
}

// Compute is TCOD_pf_compute: run the search to completion.
func (p *Pathfinder) Compute() {
	for p.frontier.len() > 0 {
		p.ComputeStep()
	}
}

// ComputeStep is TCOD_pf_compute_step: pop one node and relax its edges.
// It reports whether any work was done, so a caller can drive the search
// incrementally.
func (p *Pathfinder) ComputeStep() bool {
	if p.frontier.len() == 0 {
		return false
	}
	n := p.frontier.pop()
	// A stale heap entry: the cell was improved after this was queued.
	if n.dist > p.distance[n.index] {
		return true
	}
	p.edges2D(int(n.index)%p.w, int(n.index)/p.w)
	return true
}

// edges2D is TCOD_pf_basic2d_edges. The neighbor order is C's: west,
// north, south, east, then the four diagonals. Order is not observable in
// the final field, but it is kept so the two can be diffed.
func (p *Pathfinder) edges2D(x, y int) {
	if p.cardinal > 0 {
		p.addEdge(x, y, x-1, y, p.cardinal)
		p.addEdge(x, y, x, y-1, p.cardinal)
		p.addEdge(x, y, x, y+1, p.cardinal)
		p.addEdge(x, y, x+1, y, p.cardinal)
	}
	if p.diagonal > 0 {
		p.addEdge(x, y, x-1, y-1, p.diagonal)
		p.addEdge(x, y, x-1, y+1, p.diagonal)
		p.addEdge(x, y, x+1, y-1, p.diagonal)
		p.addEdge(x, y, x+1, y+1, p.diagonal)
	}
}

// addEdge is TCOD_pf_add_edge: relax one edge.
//
// C multiplies the edge weight by the destination's cost and skips the cell
// when that cost is not positive, so a 0 cost is an impassable wall.
func (p *Pathfinder) addEdge(fromX, fromY, toX, toY, weight int) {
	if !p.inBounds(toX, toY) {
		return
	}
	from := fromY*p.w + fromX
	to := toY*p.w + toX

	c := p.cost[to]
	if c <= 0 {
		return // impassable
	}
	if p.distance[from] == PathfinderUnreachable {
		return
	}
	total := p.distance[from] + int32(weight)*c
	// C: `if (array_get(dest) >= total_dist) return;` so a strictly better
	// distance is required to relax.
	if p.distance[to] <= total {
		return
	}
	p.distance[to] = total
	p.traversal[to] = int32(from)
	p.frontier.push(frontierNode{dist: total, index: int32(to)})
}

// Distance returns the computed distance to a cell, or
// PathfinderUnreachable if the search never reached it.
func (p *Pathfinder) Distance(x, y int) int32 {
	if !p.inBounds(x, y) {
		return PathfinderUnreachable
	}
	return p.distance[y*p.w+x]
}

// Distances exposes the raw distance field, row-major, for callers that
// want to read or seed it in bulk. Writing to it requires a Recompile
// before the next Compute. This is the Go stand-in for C's
// TCOD_pf_set_distance_pointer.
func (p *Pathfinder) Distances() []int32 { return p.distance }

// PathTo walks the traversal back from (x,y) to its seed, returning the
// route as points ordered from the seed to (x,y) inclusive. It returns nil
// if the cell was never reached.
func (p *Pathfinder) PathTo(x, y int) []Point {
	if !p.inBounds(x, y) || p.distance[y*p.w+x] == PathfinderUnreachable {
		return nil
	}
	var rev []Point
	for i := int32(y*p.w + x); i >= 0; i = p.traversal[i] {
		rev = append(rev, Point{X: int(i) % p.w, Y: int(i) / p.w})
		if p.traversal[i] < 0 {
			break
		}
	}
	// Reverse into seed-to-target order.
	for l, r := 0, len(rev)-1; l < r; l, r = l+1, r-1 {
		rev[l], rev[r] = rev[r], rev[l]
	}
	return rev
}

// Point is a grid coordinate.
type Point struct{ X, Y int }

/* --- frontier (pathfinder_frontier.c + heapq.c) --- */

// frontierNode is one queued cell. C's heap stores (priority, index...)
// with the priority first so comparisons are a plain integer test.
type frontierNode struct {
	dist  int32
	index int32
}

// frontier is TCOD_Frontier: a min-heap keyed on distance. C hand-rolls the
// heap in heapq.c; container/heap is the same algorithm and is already a
// dependency-free part of the standard library.
type frontier struct{ nodes []frontierNode }

func (f *frontier) reset()   { f.nodes = f.nodes[:0] }
func (f *frontier) len() int { return len(f.nodes) }

func (f *frontier) push(n frontierNode) { heap.Push((*frontierHeap)(f), n) }

func (f *frontier) pop() frontierNode {
	return heap.Pop((*frontierHeap)(f)).(frontierNode)
}

// frontierHeap adapts frontier to container/heap without exposing the
// interface methods on the friendlier type.
type frontierHeap frontier

func (h frontierHeap) Len() int            { return len(h.nodes) }
func (h frontierHeap) Less(i, j int) bool  { return h.nodes[i].dist < h.nodes[j].dist }
func (h frontierHeap) Swap(i, j int)       { h.nodes[i], h.nodes[j] = h.nodes[j], h.nodes[i] }
func (h *frontierHeap) Push(x interface{}) { h.nodes = append(h.nodes, x.(frontierNode)) }
func (h *frontierHeap) Pop() interface{} {
	old := h.nodes
	n := len(old)
	item := old[n-1]
	h.nodes = old[:n-1]
	return item
}
