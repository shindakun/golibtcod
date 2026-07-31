// Package bresenham is a faithful Go port of libtcod's bresenham_c.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package bresenham

// Data matches TCOD_bresenham_data_t. Zero value is invalid; use Init.
type Data struct {
	stepX, stepY   int
	e              int
	deltaX, deltaY int
	origX, origY   int
	destX, destY   int
}

// Init is TCOD_line_init_mt.
func Init(xFrom, yFrom, xTo, yTo int) *Data {
	d := &Data{
		origX: xFrom, origY: yFrom,
		destX: xTo, destY: yTo,
		deltaX: xTo - xFrom, deltaY: yTo - yFrom,
	}
	switch {
	case d.deltaX > 0:
		d.stepX = 1
	case d.deltaX < 0:
		d.stepX = -1
	}
	switch {
	case d.deltaY > 0:
		d.stepY = 1
	case d.deltaY < 0:
		d.stepY = -1
	}
	if d.stepX*d.deltaX > d.stepY*d.deltaY {
		d.e = d.stepX * d.deltaX
	} else {
		d.e = d.stepY * d.deltaY
	}
	d.deltaX *= 2
	d.deltaY *= 2
	return d
}

// Step is TCOD_line_step_mt: advances one cell; returns the new position and
// done=true once the destination has been passed.
func (d *Data) Step() (x, y int, done bool) {
	if d.stepX*d.deltaX > d.stepY*d.deltaY {
		if d.origX == d.destX {
			return 0, 0, true
		}
		d.origX += d.stepX
		d.e -= d.stepY * d.deltaY
		if d.e < 0 {
			d.origY += d.stepY
			d.e += d.stepX * d.deltaX
		}
	} else {
		if d.origY == d.destY {
			return 0, 0, true
		}
		d.origY += d.stepY
		d.e -= d.stepX * d.deltaX
		if d.e < 0 {
			d.origX += d.stepX
			d.e += d.stepY * d.deltaY
		}
	}
	return d.origX, d.origY, false
}

// Line is TCOD_line: calls listener for every cell from (xo,yo) to (xd,yd)
// inclusive; stops early (returning false) if the listener returns false.
func Line(xo, yo, xd, yd int, listener func(x, y int) bool) bool {
	d := Init(xo, yo, xd, yd)
	x, y := xo, yo
	for {
		if !listener(x, y) {
			return false
		}
		var done bool
		x, y, done = d.Step()
		if done {
			return true
		}
	}
}

// Points returns every cell on the line from (xo,yo) to (xd,yd) inclusive
// (convenience matching tcod.los.bresenham).
func Points(xo, yo, xd, yd int) [][2]int {
	var pts [][2]int
	Line(xo, yo, xd, yd, func(x, y int) bool {
		pts = append(pts, [2]int{x, y})
		return true
	})
	return pts
}
