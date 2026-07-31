// Package fov is a faithful Go port of libtcod's field-of-view module:
// fov_c.c, fov_circular_raycasting.c, fov_recursive_shadowcasting.c,
// fov_diamond_raycasting.c, fov_restrictive.c, fov_symmetric_shadowcast.c,
// and fov_permissive2.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package fov

import (
	"fmt"
	"math"

	"golibtcod/bresenham"
)

// Algorithm mirrors TCOD_fov_algorithm_t.
type Algorithm int

const (
	Basic Algorithm = iota // circular raycasting
	Diamond
	Shadow // Bergström recursive shadowcasting
	Permissive0
	Permissive1
	Permissive2
	Permissive3
	Permissive4
	Permissive5
	Permissive6
	Permissive7
	Permissive8
	Restrictive // Mingos' MRPAS
	SymmetricShadowcast
)

// Permissive returns the algorithm with permissiveness p in [0,8].
//
// Out-of-range values return AlgorithmInvalid, which ComputeFov rejects.
// Unvalidated arithmetic here would silently alias a neighbouring member
// (Permissive(-1) would be Shadow, Permissive(9) would be Restrictive), so
// a bad grade must not look like a valid algorithm. C validates the same
// range and returns TCOD_E_INVALID_ARGUMENT.
func Permissive(p int) Algorithm {
	if p < 0 || p > 8 {
		return AlgorithmInvalid
	}
	return Permissive0 + Algorithm(p)
}

// AlgorithmInvalid is returned by Permissive for an out-of-range grade.
const AlgorithmInvalid Algorithm = -1

type cell struct {
	transparent, walkable, fov bool
}

// Map mirrors struct TCOD_Map: transparent/walkable properties plus the
// last-computed FOV flag per cell.
type Map struct {
	w, h  int
	cells []cell
}

func NewMap(width, height int) *Map {
	if width <= 0 || height <= 0 {
		return nil
	}
	return &Map{w: width, h: height, cells: make([]cell, width*height)}
}

// The accessors below tolerate a nil receiver, returning zero values as
// TCOD_map_get_width and friends do. NewMap signals failure by returning
// nil, so an unchecked result would otherwise panic on the next call.

func (m *Map) Width() int {
	if m == nil {
		return 0
	}
	return m.w
}

func (m *Map) Height() int {
	if m == nil {
		return 0
	}
	return m.h
}

func (m *Map) InBounds(x, y int) bool {
	return m != nil && 0 <= x && x < m.w && 0 <= y && y < m.h
}

// Clear is TCOD_map_clear.
func (m *Map) Clear(transparent, walkable bool) {
	if m == nil {
		return
	}
	for i := range m.cells {
		m.cells[i] = cell{transparent: transparent, walkable: walkable}
	}
}

// SetProperties is TCOD_map_set_properties.
func (m *Map) SetProperties(x, y int, transparent, walkable bool) {
	if !m.InBounds(x, y) {
		return
	}
	m.cells[x+y*m.w].transparent = transparent
	m.cells[x+y*m.w].walkable = walkable
}

func (m *Map) IsTransparent(x, y int) bool { return m.InBounds(x, y) && m.cells[x+y*m.w].transparent }
func (m *Map) IsWalkable(x, y int) bool    { return m.InBounds(x, y) && m.cells[x+y*m.w].walkable }

// InFov reports the FOV flag from the last ComputeFov.
func (m *Map) InFov(x, y int) bool { return m.InBounds(x, y) && m.cells[x+y*m.w].fov }

func (m *Map) clearFov() {
	for i := range m.cells {
		m.cells[i].fov = false
	}
}

// ComputeFov is TCOD_map_compute_fov. maxRadius <= 0 means unlimited.
func (m *Map) ComputeFov(povX, povY, maxRadius int, lightWalls bool, algo Algorithm) error {
	if m == nil {
		return fmt.Errorf("fov: map must not be nil")
	}
	if !m.InBounds(povX, povY) {
		return fmt.Errorf("fov: point of view {%d, %d} is out of bounds", povX, povY)
	}
	m.clearFov()
	switch {
	case algo == Basic:
		return m.circularRaycasting(povX, povY, maxRadius, lightWalls)
	case algo == Diamond:
		return m.diamondRaycasting(povX, povY, maxRadius, lightWalls)
	case algo == Shadow:
		return m.recursiveShadowcasting(povX, povY, maxRadius, lightWalls)
	case algo >= Permissive0 && algo <= Permissive8:
		return m.permissive(povX, povY, maxRadius, lightWalls, int(algo-Permissive0))
	case algo == Restrictive:
		return m.restrictiveShadowcasting(povX, povY, maxRadius, lightWalls)
	case algo == SymmetricShadowcast:
		return m.symmetricShadowcast(povX, povY, maxRadius, lightWalls)
	default:
		return fmt.Errorf("fov: unknown algorithm %d", algo)
	}
}

/* --- postprocess (TCOD_map_postprocess): spread light to walls --- */

func (m *Map) postprocessQuadrant(x0, y0, x1, y1, dx, dy int) {
	if abs(dx) != 1 || abs(dy) != 1 {
		return
	}
	for cx := x0; cx <= x1; cx++ {
		for cy := y0; cy <= y1; cy++ {
			x2, y2 := cx+dx, cy+dy
			offset := cx + cy*m.w
			if offset < len(m.cells) && m.cells[offset].fov && m.cells[offset].transparent {
				if x2 >= x0 && x2 <= x1 {
					o2 := x2 + cy*m.w
					if o2 < len(m.cells) && !m.cells[o2].transparent {
						m.cells[o2].fov = true
					}
				}
				if y2 >= y0 && y2 <= y1 {
					o2 := cx + y2*m.w
					if o2 < len(m.cells) && !m.cells[o2].transparent {
						m.cells[o2].fov = true
					}
				}
				if x2 >= x0 && x2 <= x1 && y2 >= y0 && y2 <= y1 {
					o2 := x2 + y2*m.w
					if o2 < len(m.cells) && !m.cells[o2].transparent {
						m.cells[o2].fov = true
					}
				}
			}
		}
	}
}

// Postprocess is TCOD_map_postprocess.
func (m *Map) Postprocess(povX, povY, radius int) {
	xMin, yMin, xMax, yMax := 0, 0, m.w, m.h
	if radius > 0 {
		xMin = max(xMin, povX-radius)
		yMin = max(yMin, povY-radius)
		xMax = min(xMax, povX+radius+1)
		yMax = min(yMax, povY+radius+1)
	}
	m.postprocessQuadrant(xMin, yMin, povX, povY, -1, -1)
	m.postprocessQuadrant(povX, yMin, xMax-1, povY, 1, -1)
	m.postprocessQuadrant(xMin, povY, povX, yMax-1, -1, 1)
	m.postprocessQuadrant(povX, povY, xMax-1, yMax-1, 1, 1)
}

/* --- FOV_BASIC: circular raycasting --- */

func (m *Map) castRay(xOrigin, yOrigin, xDest, yDest, radiusSquared int, lightWalls bool) {
	d := bresenham.Init(xOrigin, yOrigin, xDest, yDest)
	for {
		cx, cy, done := d.Step()
		if done {
			return
		}
		if !m.InBounds(cx, cy) {
			return
		}
		if radiusSquared > 0 {
			r := (cx-xOrigin)*(cx-xOrigin) + (cy-yOrigin)*(cy-yOrigin)
			if r > radiusSquared {
				return
			}
		}
		i := cx + cy*m.w
		if !m.cells[i].transparent {
			if lightWalls {
				m.cells[i].fov = true
			}
			return
		}
		m.cells[i].fov = true
	}
}

func (m *Map) circularRaycasting(povX, povY, maxRadius int, lightWalls bool) error {
	xMin, yMin, xMax, yMax := 0, 0, m.w, m.h
	if maxRadius > 0 {
		xMin = max(xMin, povX-maxRadius)
		yMin = max(yMin, povY-maxRadius)
		xMax = min(xMax, povX+maxRadius+1)
		yMax = min(yMax, povY+maxRadius+1)
	}
	m.cells[povX+povY*m.w].fov = true
	rs := maxRadius * maxRadius
	for x := xMin; x < xMax; x++ {
		m.castRay(povX, povY, x, yMin, rs, lightWalls)
	}
	for y := yMin + 1; y < yMax; y++ {
		m.castRay(povX, povY, xMax-1, y, rs, lightWalls)
	}
	for x := xMax - 2; x >= xMin; x-- {
		m.castRay(povX, povY, x, yMax-1, rs, lightWalls)
	}
	for y := yMax - 2; y > yMin; y-- {
		m.castRay(povX, povY, xMin, y, rs, lightWalls)
	}
	if lightWalls {
		m.Postprocess(povX, povY, maxRadius)
	}
	return nil
}

/* --- FOV_SHADOW: recursive shadowcasting --- */

var shadowMatrix = [8][4]int{
	{1, 0, 0, 1}, {0, 1, 1, 0}, {0, -1, 1, 0}, {-1, 0, 0, 1},
	{-1, 0, 0, -1}, {0, -1, -1, 0}, {0, 1, -1, 0}, {1, 0, 0, -1},
}

func (m *Map) castLight(povX, povY, distance int, viewSlopeHigh, viewSlopeLow float32, maxRadius, octant int, lightWalls bool) {
	xx, xy := shadowMatrix[octant][0], shadowMatrix[octant][1]
	yx, yy := shadowMatrix[octant][2], shadowMatrix[octant][3]
	radiusSquared := maxRadius * maxRadius
	if viewSlopeHigh < viewSlopeLow {
		return
	}
	if distance > maxRadius {
		return
	}
	if !m.InBounds(povX+distance*xy, povY+distance*yy) {
		return
	}
	prevTileBlocked := false
	for angle := distance; angle >= 0; angle-- {
		tileSlopeHigh := (float32(angle) + 0.5) / (float32(distance) - 0.5)
		tileSlopeLow := (float32(angle) - 0.5) / (float32(distance) + 0.5)
		prevTileSlopeLow := (float32(angle) + 0.5) / (float32(distance) + 0.5)
		if tileSlopeLow > viewSlopeHigh {
			continue
		} else if tileSlopeHigh < viewSlopeLow {
			break
		}
		mapX := povX + angle*xx + distance*xy
		mapY := povY + angle*yx + distance*yy
		if !m.InBounds(mapX, mapY) {
			continue
		}
		mi := mapX + mapY*m.w
		if angle*angle+distance*distance <= radiusSquared && (lightWalls || m.cells[mi].transparent) {
			m.cells[mi].fov = true
		}
		if prevTileBlocked && m.cells[mi].transparent { // wall -> floor
			viewSlopeHigh = prevTileSlopeLow
		}
		if !prevTileBlocked && !m.cells[mi].transparent { // floor -> wall
			m.castLight(povX, povY, distance+1, viewSlopeHigh, tileSlopeHigh, maxRadius, octant, lightWalls)
		}
		prevTileBlocked = !m.cells[mi].transparent
	}
	if !prevTileBlocked {
		m.castLight(povX, povY, distance+1, viewSlopeHigh, viewSlopeLow, maxRadius, octant, lightWalls)
	}
}

func (m *Map) recursiveShadowcasting(povX, povY, maxRadius int, lightWalls bool) error {
	if maxRadius <= 0 {
		maxRadiusX := max(m.w-povX, povX)
		maxRadiusY := max(m.h-povY, povY)
		maxRadius = int(math.Sqrt(float64(maxRadiusX*maxRadiusX+maxRadiusY*maxRadiusY))) + 1
	}
	for octant := 0; octant < 8; octant++ {
		m.castLight(povX, povY, 1, 1.0, 0.0, maxRadius, octant, lightWalls)
	}
	m.cells[povX+povY*m.w].fov = true
	return nil
}

/* --- FOV_SYMMETRIC_SHADOWCAST --- */

var quadrantTable = [4][4]int{
	{1, 0, 0, 1}, {0, 1, 1, 0}, {0, -1, -1, 0}, {-1, 0, 0, -1},
}

type symRow struct {
	povX, povY int
	quadrant   int
	depth      int
	slopeLow   float32
	slopeHigh  float32
}

func symSlope(rowDepth, column int) float32 {
	return (2.0*float32(column) - 1.0) / (2.0 * float32(rowDepth))
}

const f32Epsilon = 1.1920929e-07 // FLT_EPSILON

func roundHalfUp(n float32) int   { return int(roundf(n * (1 + f32Epsilon))) }
func roundHalfDown(n float32) int { return int(roundf(n * (1 - f32Epsilon))) }

// roundf matches C roundf: round half away from zero.
func roundf(n float32) float32 { return float32(math.Round(float64(n))) }

func (r *symRow) isSymmetric(column int) bool {
	return float32(column) >= float32(r.depth)*r.slopeLow &&
		float32(column) <= float32(r.depth)*r.slopeHigh
}

func (m *Map) symScan(row *symRow) {
	xx, xy := quadrantTable[row.quadrant][0], quadrantTable[row.quadrant][1]
	yx, yy := quadrantTable[row.quadrant][2], quadrantTable[row.quadrant][3]
	if !m.InBounds(row.povX+row.depth*xx, row.povY+row.depth*yx) {
		return
	}
	columnMin := roundHalfUp(float32(row.depth) * row.slopeLow)
	columnMax := roundHalfDown(float32(row.depth) * row.slopeHigh)
	prevTileIsWall := false
	for column := columnMin; column <= columnMax; column++ {
		mapX := row.povX + row.depth*xx + column*xy
		mapY := row.povY + row.depth*yx + column*yy
		if !m.InBounds(mapX, mapY) {
			continue
		}
		c := &m.cells[mapX+mapY*m.w]
		isWall := !c.transparent
		if isWall || row.isSymmetric(column) {
			c.fov = true
		}
		if prevTileIsWall && !isWall { // wall -> floor
			row.slopeLow = symSlope(row.depth, column)
		}
		if column != columnMin && !prevTileIsWall && isWall { // floor -> wall
			next := symRow{
				povX: row.povX, povY: row.povY, quadrant: row.quadrant,
				depth:    row.depth + 1,
				slopeLow: row.slopeLow, slopeHigh: symSlope(row.depth, column),
			}
			m.symScan(&next)
		}
		prevTileIsWall = isWall
	}
	if !prevTileIsWall {
		row.depth++
		m.symScan(row)
	}
}

func (m *Map) symmetricShadowcast(povX, povY, maxRadius int, lightWalls bool) error {
	m.cells[povX+povY*m.w].fov = true
	for quadrant := 0; quadrant < 4; quadrant++ {
		row := symRow{povX: povX, povY: povY, quadrant: quadrant, depth: 1, slopeLow: -1.0, slopeHigh: 1.0}
		m.symScan(&row)
	}
	radiusSquared := maxRadius * maxRadius
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			i := x + y*m.w
			if !lightWalls && !m.cells[i].transparent {
				m.cells[i].fov = false
			}
			if maxRadius > 0 {
				dx, dy := x-povX, y-povY
				if dx*dx+dy*dy >= radiusSquared {
					m.cells[i].fov = false
				}
			}
		}
	}
	return nil
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
