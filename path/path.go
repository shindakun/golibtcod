// Package path is a faithful Go port of libtcod's path_c.c: the classic
// A* pathfinder and Mingos' Dijkstra flood-fill pathfinder.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package path

import (
	"math"

	"golibtcod/fov"
)

// CostFunc mirrors TCOD_path_func_t: returns the cost to move from
// (xFrom,yFrom) to (xTo,yTo); <= 0 means the move is blocked.
type CostFunc func(xFrom, yFrom, xTo, yTo int) float32

type dir uint8

const (
	dirNW dir = iota
	dirN
	dirNE
	dirW
	dirNone
	dirE
	dirSW
	dirS
	dirSE
)

var dirX = [9]int{-1, 0, 1, -1, 0, 1, -1, 0, 1}
var dirY = [9]int{-1, -1, -1, 0, 0, 0, 1, 1, 1}
var invertDir = [9]dir{dirSE, dirS, dirSW, dirE, dirNone, dirW, dirNE, dirN, dirNW}

/* --- classic A* (TCOD_Path) --- */

// AStar mirrors TCOD_Path.
type AStar struct {
	ox, oy, dx, dy int
	path           []dir // stack of directions (walked back-to-front)
	w, h           int
	grid           []float32 // covered distance
	heuristic      []float32 // A* score
	prev           []dir
	diagonalCost   float32
	heap           []int // min-heap of cell offsets keyed by heuristic
	m              *fov.Map
	fn             CostFunc
}

func newIntern(w, h int) *AStar {
	return &AStar{
		w: w, h: h,
		grid:      make([]float32, w*h),
		heuristic: make([]float32, w*h),
		prev:      make([]dir, w*h),
	}
}

// NewUsingMap is TCOD_path_new_using_map; walkability comes from m.
func NewUsingMap(m *fov.Map, diagonalCost float32) *AStar {
	if m == nil {
		return nil
	}
	p := newIntern(m.Width(), m.Height())
	p.m = m
	p.diagonalCost = diagonalCost
	return p
}

// NewUsingFunc is TCOD_path_new_using_function.
func NewUsingFunc(w, h int, fn CostFunc, diagonalCost float32) *AStar {
	if fn == nil || w <= 0 || h <= 0 {
		return nil
	}
	p := newIntern(w, h)
	p.fn = fn
	p.diagonalCost = diagonalCost
	return p
}

func (p *AStar) walkCost(xFrom, yFrom, xTo, yTo int) float32 {
	if p.m != nil {
		if p.m.IsWalkable(xTo, yTo) {
			return 1.0
		}
		return 0.0
	}
	return p.fn(xFrom, yFrom, xTo, yTo)
}

/* heap: min-heap over p.heap keyed by p.heuristic, exact port */

func (p *AStar) heapSiftDown() {
	end := len(p.heap) - 1
	cur := 0
	child := 1
	for child <= end {
		curDist := p.heuristic[p.heap[cur]]
		childDist := p.heuristic[p.heap[child]]
		toSwap := cur
		swapValue := curDist
		if childDist < curDist {
			toSwap = child
			swapValue = childDist
		}
		if child < end {
			child2Dist := p.heuristic[p.heap[child+1]]
			if swapValue > child2Dist {
				toSwap = child + 1
			}
		}
		if toSwap != cur {
			p.heap[toSwap], p.heap[cur] = p.heap[cur], p.heap[toSwap]
			cur = toSwap
		} else {
			return
		}
		child = cur*2 + 1
	}
}

func (p *AStar) heapSiftUp() {
	child := len(p.heap) - 1
	for child > 0 {
		childDist := p.heuristic[p.heap[child]]
		parent := (child - 1) / 2
		parentDist := p.heuristic[p.heap[parent]]
		if parentDist > childDist {
			p.heap[child], p.heap[parent] = p.heap[parent], p.heap[child]
			child = parent
		} else {
			return
		}
	}
}

func (p *AStar) heapAdd(x, y int) {
	p.heap = append(p.heap, x+y*p.w)
	p.heapSiftUp()
}

func (p *AStar) heapGet() int {
	end := len(p.heap) - 1
	off := p.heap[0]
	p.heap[0] = p.heap[end]
	p.heap = p.heap[:end]
	p.heapSiftDown()
	return off
}

func (p *AStar) heapReorder(offset int) {
	idx := -1
	for i, v := range p.heap {
		if v == offset {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	heapSize := len(p.heap)
	offIdx := p.heap[idx]
	value := p.heuristic[offIdx]
	if idx > 0 {
		parent := (idx - 1) / 2
		offParent := p.heap[parent]
		parentValue := p.heuristic[offParent]
		if value < parentValue {
			for idx > 0 && value < parentValue {
				p.heap[parent] = offIdx
				p.heap[idx] = offParent
				idx = parent
				if idx > 0 {
					parent = (idx - 1) / 2
					offParent = p.heap[parent]
					parentValue = p.heuristic[offParent]
				}
			}
			return
		}
	}
	for idx*2+1 < heapSize {
		child := idx*2 + 1
		toSwap := idx
		swapValue := value
		if p.heuristic[p.heap[child]] < value {
			toSwap = child
			swapValue = p.heuristic[p.heap[child]]
		}
		child2 := child + 1
		if child2 < heapSize {
			if p.heuristic[p.heap[child2]] < swapValue {
				toSwap = child2
			}
		}
		if toSwap != idx {
			p.heap[toSwap], p.heap[idx] = p.heap[idx], p.heap[toSwap]
			idx = toSwap
		} else {
			return
		}
	}
}

/* A* proper */

var iDirX = [8]int{0, -1, 1, 0, -1, 1, -1, 1}
var iDirY = [8]int{-1, 0, 0, 1, -1, -1, 1, 1}
var previousDirs = [8]dir{dirN, dirW, dirE, dirS, dirNW, dirNE, dirSW, dirSE}

func (p *AStar) setCells() {
	for p.grid[p.dx+p.dy*p.w] == 0 && len(p.heap) > 0 {
		off := p.heapGet()
		x, y := off%p.w, off/p.w
		distance := p.grid[off]
		iMax := 8
		if p.diagonalCost == 0.0 {
			iMax = 4
		}
		for i := 0; i < iMax; i++ {
			cx := x + iDirX[i]
			cy := y + iDirY[i]
			if cx >= 0 && cy >= 0 && cx < p.w && cy < p.h {
				walkCost := p.walkCost(x, y, cx, cy)
				if walkCost > 0.0 {
					mult := float32(1.0)
					if i >= 4 {
						mult = p.diagonalCost
					}
					covered := distance + walkCost*mult
					previousCovered := p.grid[cx+cy*p.w]
					if previousCovered == 0 {
						offset := cx + cy*p.w
						remaining := float32(math.Sqrt(float64((cx-p.dx)*(cx-p.dx) + (cy-p.dy)*(cy-p.dy))))
						p.grid[offset] = covered
						p.heuristic[offset] = covered + remaining
						p.prev[offset] = previousDirs[i]
						p.heapAdd(cx, cy)
					} else if previousCovered > covered {
						offset := cx + cy*p.w
						p.grid[offset] = covered
						p.heuristic[offset] -= previousCovered - covered
						p.prev[offset] = previousDirs[i]
						p.heapReorder(offset)
					}
				}
			}
		}
	}
}

// Compute is TCOD_path_compute.
func (p *AStar) Compute(ox, oy, dx, dy int) bool {
	p.ox, p.oy, p.dx, p.dy = ox, oy, dx, dy
	p.path = p.path[:0]
	p.heap = p.heap[:0]
	if ox == dx && oy == dy {
		return true
	}
	if !(uint(ox) < uint(p.w) && uint(oy) < uint(p.h)) {
		return false
	}
	if !(uint(dx) < uint(p.w) && uint(dy) < uint(p.h)) {
		return false
	}
	for i := range p.grid {
		p.grid[i] = 0
		p.prev[i] = dirNone
	}
	p.heuristic[ox+oy*p.w] = 1.0
	p.heapAdd(ox, oy)
	p.setCells()
	if p.grid[dx+dy*p.w] == 0 {
		return false
	}
	for dx != ox || dy != oy {
		step := p.prev[dx+dy*p.w]
		p.path = append(p.path, step)
		dx -= dirX[step]
		dy -= dirY[step]
	}
	return true
}

// Reverse is TCOD_path_reverse.
func (p *AStar) Reverse() {
	p.ox, p.dx = p.dx, p.ox
	p.oy, p.dy = p.dy, p.oy
	for i := range p.path {
		p.path[i] = invertDir[p.path[i]]
	}
}

// Walk is TCOD_path_walk: pops the next step and returns the new position.
func (p *AStar) Walk(recalculateWhenNeeded bool) (x, y int, ok bool) {
	if p.IsEmpty() {
		return 0, 0, false
	}
	d := p.path[len(p.path)-1]
	p.path = p.path[:len(p.path)-1]
	newX := p.ox + dirX[d]
	newY := p.oy + dirY[d]
	if p.walkCost(p.ox, p.oy, newX, newY) <= 0.0 {
		if !recalculateWhenNeeded {
			return 0, 0, false
		}
		if !p.Compute(p.ox, p.oy, p.dx, p.dy) {
			return 0, 0, false
		}
		return p.Walk(true)
	}
	p.ox, p.oy = newX, newY
	return newX, newY, true
}

func (p *AStar) IsEmpty() bool { return len(p.path) == 0 }
func (p *AStar) Size() int     { return len(p.path) }

// Get is TCOD_path_get: the coordinates of the index-th step.
//
// An out-of-range index returns the origin rather than panicking. C runs a
// do/while here, so an empty path reads TCOD_list_get(list, -1) before the
// array; Compute succeeds with Size()==0 when origin equals destination, so
// Get(0) on a valid path object must not crash.
func (p *AStar) Get(index int) (x, y int) {
	x, y = p.ox, p.oy
	if index < 0 || index >= len(p.path) {
		return x, y
	}
	pos := len(p.path) - 1
	for {
		step := p.path[pos]
		x += dirX[step]
		y += dirY[step]
		pos--
		index--
		if index < 0 {
			return x, y
		}
	}
}

func (p *AStar) Origin() (x, y int)      { return p.ox, p.oy }
func (p *AStar) Destination() (x, y int) { return p.dx, p.dy }

/* --- classic Dijkstra (TCOD_Dijkstra, by Mingos) --- */

const dijkstraInfinity = 0xFFFFFFFF

// Dijkstra mirrors TCOD_Dijkstra.
type Dijkstra struct {
	w, h         int
	diagonalCost int // cost * 100, as in C
	distances    []uint32
	nodes        []uint32
	path         []uint32
	m            *fov.Map
	fn           CostFunc
}

// NewDijkstra is TCOD_dijkstra_new.
func NewDijkstra(m *fov.Map, diagonalCost float32) *Dijkstra {
	if m == nil {
		return nil
	}
	n := m.Width() * m.Height()
	return &Dijkstra{
		w: m.Width(), h: m.Height(), m: m,
		diagonalCost: int(diagonalCost*100.0 + 0.1), // (int)(1.41f*100.0f)==140 quirk
		distances:    make([]uint32, n),
		nodes:        make([]uint32, n),
	}
}

// NewDijkstraUsingFunc is TCOD_dijkstra_new_using_function.
func NewDijkstraUsingFunc(w, h int, fn CostFunc, diagonalCost float32) *Dijkstra {
	if fn == nil || w <= 0 || h <= 0 {
		return nil
	}
	n := w * h
	return &Dijkstra{
		w: w, h: h, fn: fn,
		diagonalCost: int(diagonalCost*100.0 + 0.1),
		distances:    make([]uint32, n),
		nodes:        make([]uint32, n),
	}
}

// Compute is TCOD_dijkstra_compute: fills the whole distance grid from root.
func (d *Dijkstra) Compute(rootX, rootY int) {
	mx, my := d.w, d.h
	mMax := mx * my
	if !(uint(rootX) < uint(mx) && uint(rootY) < uint(my)) {
		return
	}
	root := uint32(rootY*mx + rootX)
	index := 0
	lastIndex := 1
	// node processing order: W, S, E, N, NW, NE, SE, SW (C comment; table below is authoritative)
	dx := [8]int{-1, 0, 1, 0, -1, 1, 1, -1}
	dy := [8]int{0, -1, 0, 1, -1, -1, 1, 1}
	dd := [8]int{100, 100, 100, 100, d.diagonalCost, d.diagonalCost, d.diagonalCost, d.diagonalCost}
	iMax := 8
	if d.diagonalCost == 0 {
		iMax = 4
	}
	for i := range d.distances {
		d.distances[i] = dijkstraInfinity
	}
	for i := range d.nodes {
		d.nodes[i] = dijkstraInfinity
	}
	d.distances[root] = 0
	d.nodes[index] = root
	for {
		if d.nodes[index] != dijkstraInfinity {
			x := int(d.nodes[index]) % mx
			y := int(d.nodes[index]) / mx
			for i := 0; i < iMax; i++ {
				tx := x + dx[i]
				ty := y + dy[i]
				if uint(tx) < uint(mx) && uint(ty) < uint(my) {
					dt := d.distances[d.nodes[index]]
					var userDist float32
					if d.m != nil {
						dt += uint32(dd[i])
					} else {
						userDist = d.fn(x, y, tx, ty)
						dt += uint32(userDist * float32(dd[i]))
					}
					newNode := uint32(ty*mx + tx)
					if d.distances[newNode] > dt {
						if d.m != nil && !d.m.IsWalkable(tx, ty) {
							continue
						} else if d.fn != nil && userDist <= 0.0 {
							continue
						}
						d.distances[newNode] = dt
						// Insertion into the pending queue, following the C walk.
						//
						// DIVERGENCE (deliberate): C lets lastIndex grow without
						// bound, so nodes[j+1] can be written one past the end of
						// the array. Upstream absorbs this by over-allocating 4x in
						// TCOD_dijkstra_new_using_function (path_c.c:473-474) while
						// leaving nodes_max at w*h; the map-based constructor gets no
						// such padding. Rather than reproduce a heap overflow, the
						// queue is clamped to its capacity. See docs/BUILDLOG.md.
						if lastIndex > len(d.nodes) {
							lastIndex = len(d.nodes)
						}
						j := lastIndex - 1
						for d.distances[d.nodes[j]] >= d.distances[newNode] {
							if d.nodes[j] == newNode {
								for k := j; k <= lastIndex && k+1 < len(d.nodes); k++ {
									d.nodes[k] = d.nodes[k+1]
								}
								lastIndex--
							} else if j+1 < len(d.nodes) {
								d.nodes[j+1] = d.nodes[j]
							}
							j--
							if j < 0 {
								break
							}
						}
						if lastIndex < len(d.nodes) {
							lastIndex++
						}
						if j+1 < len(d.nodes) {
							d.nodes[j+1] = newNode
						}
					}
				}
			}
		}
		index++
		if index >= mMax {
			break
		}
	}
}

// Distance is TCOD_dijkstra_get_distance; -1 if unreachable.
func (d *Dijkstra) Distance(x, y int) float32 {
	if !(uint(x) < uint(d.w) && uint(y) < uint(d.h)) {
		return -1.0
	}
	v := d.distances[y*d.w+x]
	if v == dijkstraInfinity {
		return -1.0
	}
	return float32(v) * 0.01
}

func (d *Dijkstra) intDistance(x, y int) uint32 { return d.distances[y*d.w+x] }

// PathSet is TCOD_dijkstra_path_set: builds a path from the computed root
// to (x, y). Returns false if unreachable.
func (d *Dijkstra) PathSet(x, y int) bool {
	dx := [9]int{-1, 0, 1, 0, -1, 1, 1, -1, 0}
	dy := [9]int{0, -1, 0, 1, -1, -1, 1, 1, 0}
	iMax := 8
	if d.diagonalCost == 0 {
		iMax = 4
	}
	if !(uint(x) < uint(d.w) && uint(y) < uint(d.h)) {
		return false
	}
	if d.intDistance(x, y) == dijkstraInfinity {
		return false
	}
	d.path = d.path[:0]
	px, py := x, y
	var distances [8]uint32
	var lowestIndex int
	for {
		d.path = append(d.path, uint32(py*d.w+px))
		for i := 0; i < iMax; i++ {
			cx := px + dx[i]
			cy := py + dy[i]
			if uint(cx) < uint(d.w) && uint(cy) < uint(d.h) {
				distances[i] = d.intDistance(cx, cy)
			} else {
				distances[i] = dijkstraInfinity
			}
		}
		lowest := d.intDistance(px, py)
		lowestIndex = 8
		for i := 0; i < iMax; i++ {
			if distances[i] < lowest {
				lowest = distances[i]
				lowestIndex = i
			}
		}
		px += dx[lowestIndex]
		py += dy[lowestIndex]
		if lowestIndex == 8 {
			break
		}
	}
	d.path = d.path[:len(d.path)-1] // remove the last step
	return true
}

// Reverse is TCOD_dijkstra_reverse.
func (d *Dijkstra) Reverse() {
	for i, j := 0, len(d.path)-1; i < j; i, j = i+1, j-1 {
		d.path[i], d.path[j] = d.path[j], d.path[i]
	}
}

// PathWalk is TCOD_dijkstra_path_walk.
func (d *Dijkstra) PathWalk() (x, y int, ok bool) {
	if len(d.path) == 0 {
		return 0, 0, false
	}
	node := d.path[len(d.path)-1]
	d.path = d.path[:len(d.path)-1]
	return int(node) % d.w, int(node) / d.w, true
}

func (d *Dijkstra) IsEmpty() bool { return len(d.path) == 0 }
func (d *Dijkstra) Size() int     { return len(d.path) }

// Get is TCOD_dijkstra_get. An out-of-range index returns (-1,-1) rather
// than panicking; see AStar.Get for the rationale.
func (d *Dijkstra) Get(index int) (x, y int) {
	if index < 0 || index >= len(d.path) {
		return -1, -1
	}
	node := d.path[len(d.path)-index-1]
	return int(node) % d.w, int(node) / d.w
}
