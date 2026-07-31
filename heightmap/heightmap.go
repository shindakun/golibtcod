// Package heightmap is a faithful Go port of libtcod's heightmap_c.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package heightmap

import (
	"math"

	"golibtcod/noise"
	"golibtcod/rng"
)

const (
	fltMax = math.MaxFloat32
	fltEps = 1.1920929e-07
)

// HeightMap mirrors TCOD_heightmap_t.
type HeightMap struct {
	W, H   int
	Values []float32
}

func New(w, h int) *HeightMap { return &HeightMap{W: w, H: h, Values: make([]float32, w*h)} }

func (hm *HeightMap) valid() bool { return hm != nil && hm.Values != nil && hm.W > 0 && hm.H > 0 }

func sameSize(a, b *HeightMap) bool { return a.valid() && b.valid() && a.W == b.W && a.H == b.H }

func (hm *HeightMap) InBounds(x, y int) bool { return 0 <= x && x < hm.W && 0 <= y && y < hm.H }

func (hm *HeightMap) At(x, y int) float32     { return hm.Values[x+y*hm.W] }
func (hm *HeightMap) Set(x, y int, v float32) { hm.Values[x+y*hm.W] = v }
func (hm *HeightMap) add(x, y int, v float32) { hm.Values[x+y*hm.W] += v }

func (hm *HeightMap) Clear() {
	for i := range hm.Values {
		hm.Values[i] = 0
	}
}

// MinMax is TCOD_heightmap_get_minmax.
func (hm *HeightMap) MinMax() (min, max float32) {
	min, max = fltMax, -fltMax
	for _, v := range hm.Values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// Normalize is TCOD_heightmap_normalize.
func (hm *HeightMap) Normalize(min, max float32) {
	if !hm.valid() {
		return
	}
	curMin, curMax := hm.MinMax()
	if curMax-curMin < fltEps {
		for i := range hm.Values {
			hm.Values[i] = min
		}
	} else {
		scale := (max - min) / (curMax - curMin)
		for i := range hm.Values {
			hm.Values[i] = min + (hm.Values[i]-curMin)*scale
		}
	}
}

// AddHill is TCOD_heightmap_add_hill.
func (hm *HeightMap) AddHill(hx, hy, radius, height float32) {
	if !hm.valid() {
		return
	}
	radius2 := radius * radius
	coef := height / radius2
	minX := maxI(int(hx-radius), 0)
	minY := maxI(int(hy-radius), 0)
	maxX := int(minF(float32(math.Ceil(float64(hx+radius))), float32(hm.W)))
	maxY := int(minF(float32(math.Ceil(float64(hy+radius))), float32(hm.H)))
	for y := minY; y < maxY; y++ {
		yDist := (float32(y) - hy) * (float32(y) - hy)
		for x := minX; x < maxX; x++ {
			xDist := (float32(x) - hx) * (float32(x) - hx)
			z := radius2 - xDist - yDist
			if z > 0 {
				hm.add(x, y, z*coef)
			}
		}
	}
}

// DigHill is TCOD_heightmap_dig_hill.
func (hm *HeightMap) DigHill(hx, hy, radius, height float32) {
	if !hm.valid() {
		return
	}
	radius2 := radius * radius
	coef := height / radius2
	minX := maxI(int(hx-radius), 0)
	minY := maxI(int(hy-radius), 0)
	maxX := int(minF(float32(math.Ceil(float64(hx+radius))), float32(hm.W)))
	maxY := int(minF(float32(math.Ceil(float64(hy+radius))), float32(hm.H)))
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			xDist := (float32(x) - hx) * (float32(x) - hx)
			yDist := (float32(y) - hy) * (float32(y) - hy)
			dist := xDist + yDist
			if dist < radius2 {
				z := (radius2 - dist) * coef
				if height > 0 {
					if hm.At(x, y) < z {
						hm.Set(x, y, z)
					}
				} else {
					if hm.At(x, y) > z {
						hm.Set(x, y, z)
					}
				}
			}
		}
	}
}

// Copy is TCOD_heightmap_copy (source method form: hm.CopyTo(dest)).
func (hm *HeightMap) CopyTo(dest *HeightMap) {
	if !sameSize(hm, dest) {
		return
	}
	copy(dest.Values, hm.Values)
}

// AddFbm is TCOD_heightmap_add_fbm.
func (hm *HeightMap) AddFbm(n *noise.Noise, mulX, mulY, addX, addY, octaves, delta, scale float32) {
	if !hm.valid() {
		return
	}
	xc := mulX / float32(hm.W)
	yc := mulY / float32(hm.H)
	for y := 0; y < hm.H; y++ {
		for x := 0; x < hm.W; x++ {
			f := []float32{(float32(x) + addX) * xc, (float32(y) + addY) * yc}
			hm.add(x, y, delta+n.GetFbm(f, octaves)*scale)
		}
	}
}

// ScaleFbm is TCOD_heightmap_scale_fbm.
func (hm *HeightMap) ScaleFbm(n *noise.Noise, mulX, mulY, addX, addY, octaves, delta, scale float32) {
	if !hm.valid() {
		return
	}
	xc := mulX / float32(hm.W)
	yc := mulY / float32(hm.H)
	for y := 0; y < hm.H; y++ {
		for x := 0; x < hm.W; x++ {
			f := []float32{(float32(x) + addX) * xc, (float32(y) + addY) * yc}
			hm.Values[x+y*hm.W] *= delta + n.GetFbm(f, octaves)*scale
		}
	}
}

// InterpolatedValue is TCOD_heightmap_get_interpolated_value.
func (hm *HeightMap) InterpolatedValue(x, y float32) float32 {
	if !hm.valid() {
		return 0.0
	}
	x = clampF(0.0, float32(hm.W-1), x)
	y = clampF(0.0, float32(hm.H-1), y)
	fix, fx := modf(x)
	fiy, fy := modf(y)
	ix, iy := int(fix), int(fiy)
	if ix >= hm.W-1 {
		ix = hm.W - 2
		fx = 1.0
	}
	if iy >= hm.H-1 {
		iy = hm.H - 2
		fy = 1.0
	}
	c1 := hm.At(ix, iy)
	c2 := hm.At(ix+1, iy)
	c3 := hm.At(ix, iy+1)
	c4 := hm.At(ix+1, iy+1)
	top := lerp(c1, c2, fx)
	bottom := lerp(c3, c4, fx)
	return lerp(top, bottom, fy)
}

// Normal is TCOD_heightmap_get_normal.
func (hm *HeightMap) Normal(x, y, waterLevel float32) (n [3]float32) {
	n = [3]float32{0, 0, 1}
	if !hm.valid() {
		return n
	}
	if x >= float32(hm.W-1) || y >= float32(hm.H-1) {
		return n
	}
	h0 := maxF(hm.InterpolatedValue(x, y), waterLevel)
	hx := maxF(hm.InterpolatedValue(x+1, y), waterLevel)
	hy := maxF(hm.InterpolatedValue(x, y+1), waterLevel)
	n[0] = 255 * (h0 - hx)
	n[1] = 255 * (h0 - hy)
	n[2] = 16.0
	invLen := 1.0 / float32(math.Sqrt(float64(n[0]*n[0]+n[1]*n[1]+n[2]*n[2])))
	n[0] *= invLen
	n[1] *= invLen
	n[2] *= invLen
	return n
}

// DigBezier is TCOD_heightmap_dig_bezier.
func (hm *HeightMap) DigBezier(px, py [4]int, startRadius, startDepth, endRadius, endDepth float32) {
	if !hm.valid() {
		return
	}
	xFrom, yFrom := px[0], py[0]
	for i := 0; i <= 1000; i++ {
		t := float32(i) / 1000.0
		it := 1.0 - t
		xTo := int(float32(px[0])*it*it*it + 3*float32(px[1])*t*it*it + 3*float32(px[2])*t*t*it + float32(px[3])*t*t*t)
		yTo := int(float32(py[0])*it*it*it + 3*float32(py[1])*t*it*it + 3*float32(py[2])*t*t*it + float32(py[3])*t*t*t)
		if xTo != xFrom || yTo != yFrom {
			radius := startRadius + (endRadius-startRadius)*t
			depth := startDepth + (endDepth-startDepth)*t
			hm.DigHill(float32(xTo), float32(yTo), radius, depth)
			xFrom, yFrom = xTo, yTo
		}
	}
}

// HasLandOnBorder is TCOD_heightmap_has_land_on_border.
func (hm *HeightMap) HasLandOnBorder(waterLevel float32) bool {
	if !hm.valid() {
		return false
	}
	for x := 0; x < hm.W; x++ {
		if hm.At(x, 0) > waterLevel || hm.At(x, hm.H-1) > waterLevel {
			return true
		}
	}
	for y := 0; y < hm.H; y++ {
		if hm.At(0, y) > waterLevel || hm.At(hm.W-1, y) > waterLevel {
			return true
		}
	}
	return false
}

// Add is TCOD_heightmap_add.
func (hm *HeightMap) Add(value float32) {
	for i := range hm.Values {
		hm.Values[i] += value
	}
}

// CountCells is TCOD_heightmap_count_cells.
func (hm *HeightMap) CountCells(min, max float32) int {
	count := 0
	for _, v := range hm.Values {
		if v >= min && v <= max {
			count++
		}
	}
	return count
}

// Scale is TCOD_heightmap_scale.
func (hm *HeightMap) Scale(value float32) {
	for i := range hm.Values {
		hm.Values[i] *= value
	}
}

// Clamp is TCOD_heightmap_clamp.
func (hm *HeightMap) Clamp(min, max float32) {
	for i := range hm.Values {
		hm.Values[i] = clampF(min, max, hm.Values[i])
	}
}

// Lerp is TCOD_heightmap_lerp_hm: out = lerp(hm1, hm2, coef).
func Lerp(hm1, hm2, out *HeightMap, coef float32) {
	if !sameSize(hm1, hm2) || !sameSize(hm1, out) {
		return
	}
	for i := range hm1.Values {
		out.Values[i] = lerp(hm1.Values[i], hm2.Values[i], coef)
	}
}

// AddHm is TCOD_heightmap_add_hm.
func AddHm(hm1, hm2, out *HeightMap) {
	if !sameSize(hm1, hm2) || !sameSize(hm1, out) {
		return
	}
	for i := range hm1.Values {
		out.Values[i] = hm1.Values[i] + hm2.Values[i]
	}
}

// MultiplyHm is TCOD_heightmap_multiply_hm.
func MultiplyHm(hm1, hm2, out *HeightMap) {
	if !sameSize(hm1, hm2) || !sameSize(hm1, out) {
		return
	}
	for i := range hm1.Values {
		out.Values[i] = hm1.Values[i] * hm2.Values[i]
	}
}

var moore8x = [8]int{-1, 0, 1, -1, 1, -1, 0, 1}
var moore8y = [8]int{-1, -1, -1, 0, 0, 1, 1, 1}

// Slope is TCOD_heightmap_get_slope.
func (hm *HeightMap) Slope(x, y int) float32 {
	if !hm.InBounds(x, y) {
		return 0
	}
	var minDy, maxDy float32
	v := hm.At(x, y)
	for i := 0; i < 8; i++ {
		nx, ny := x+moore8x[i], y+moore8y[i]
		if hm.InBounds(nx, ny) {
			nSlope := hm.At(nx, ny) - v
			minDy = minF(minDy, nSlope)
			maxDy = maxF(maxDy, nSlope)
		}
	}
	return float32(math.Atan2(float64(maxDy+minDy), 1.0))
}

// RainErosion is TCOD_heightmap_rain_erosion.
func (hm *HeightMap) RainErosion(nbDrops int, erosionCoef, aggregationCoef float32, r *rng.Random) {
	if !hm.valid() {
		return
	}
	for ; nbDrops > 0; nbDrops-- {
		curX := r.GetInt(0, hm.W-1)
		curY := r.GetInt(0, hm.H-1)
		var sediment float32
		for {
			nextX, nextY := 0, 0
			v := hm.At(curX, curY)
			slope := float32(math.Inf(-1))
			for i := 0; i < 8; i++ {
				nx, ny := curX+moore8x[i], curY+moore8y[i]
				if !hm.InBounds(nx, ny) {
					continue
				}
				nSlope := v - hm.At(nx, ny)
				if nSlope > slope {
					slope = nSlope
					nextX, nextY = nx, ny
				}
			}
			if slope > 0.0 {
				hm.add(curX, curY, -erosionCoef*slope)
				curX, curY = nextX, nextY
				sediment += slope
			} else {
				hm.add(curX, curY, aggregationCoef*sediment)
				break
			}
		}
	}
}

// ThresholdMask is TCOD_heightmap_threshold_mask.
func (hm *HeightMap) ThresholdMask(minLevel, maxLevel float32) []bool {
	mask := make([]bool, hm.W*hm.H)
	for i, v := range hm.Values {
		mask[i] = v >= minLevel && v <= maxLevel
	}
	return mask
}

// KernelTransformOut is TCOD_heightmap_kernel_transform_out.
func KernelTransformOut(src, dst *HeightMap, dx, dy []int, weight []float32, mask []bool) {
	if !sameSize(src, dst) {
		return
	}
	kernelSize := len(dx)
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			idx := x + y*src.W
			if mask == nil || mask[idx] {
				var val, totalWeight float32
				for i := 0; i < kernelSize; i++ {
					nx, ny := x+dx[i], y+dy[i]
					if src.InBounds(nx, ny) {
						val += weight[i] * src.At(nx, ny)
						totalWeight += weight[i]
					}
				}
				dst.Values[idx] = val / totalWeight
			}
		}
	}
}

// KernelTransform is TCOD_heightmap_kernel_transform.
func (hm *HeightMap) KernelTransform(dx, dy []int, weight []float32, minLevel, maxLevel float32) {
	if !hm.valid() {
		return
	}
	hmCopy := New(hm.W, hm.H)
	hm.CopyTo(hmCopy)
	var mask []bool
	if !(minLevel <= -fltMax && maxLevel >= fltMax) {
		mask = hmCopy.ThresholdMask(minLevel, maxLevel)
	}
	KernelTransformOut(hmCopy, hm, dx, dy, weight, mask)
}

// AddVoronoi is TCOD_heightmap_add_voronoi.
func (hm *HeightMap) AddVoronoi(nbPoints, nbCoef int, coef []float32, r *rng.Random) {
	if !hm.valid() || nbPoints <= 0 {
		return
	}
	type point struct {
		x, y int
		dist float32
	}
	pt := make([]point, nbPoints)
	// C clamps against nbPoints only and then indexes coef[i], reading past
	// the caller's array when fewer coefficients were supplied. Clamp against
	// the slice too.
	nbCoef = minI(nbCoef, nbPoints)
	nbCoef = minI(nbCoef, len(coef))
	if nbCoef <= 0 {
		return
	}
	for i := range pt {
		pt[i].x = r.GetInt(0, hm.W-1)
		pt[i].y = r.GetInt(0, hm.H-1)
	}
	for y := 0; y < hm.H; y++ {
		for x := 0; x < hm.W; x++ {
			for i := range pt {
				dx := pt[i].x - x
				dy := pt[i].y - y
				pt[i].dist = float32(dx*dx + dy*dy)
			}
			for i := 0; i < nbCoef; i++ {
				minDist := float32(1e8)
				idx := -1
				for j := range pt {
					if pt[j].dist < minDist {
						idx = j
						minDist = pt[j].dist
					}
				}
				if idx == -1 {
					break
				}
				hm.add(x, y, coef[i]*pt[idx].dist)
				pt[idx].dist = 1e8
			}
		}
	}
}

func (hm *HeightMap) setMPDHeight(r *rng.Random, x, y int, z, offset float32) {
	z += r.GetFloat(-offset, offset)
	hm.Set(x, y, z)
}

func (hm *HeightMap) setMDPHeightSquare(r *rng.Random, x, y, initSz, sz int, offset float32) {
	var z float32
	count := 0
	if y >= sz {
		z += hm.At(x, y-sz)
		count++
	}
	if x >= sz {
		z += hm.At(x-sz, y)
		count++
	}
	if y+sz < initSz {
		z += hm.At(x, y+sz)
		count++
	}
	if x+sz < initSz {
		z += hm.At(x+sz, y)
		count++
	}
	z /= float32(count)
	hm.setMPDHeight(r, x, y, z, offset)
}

// MidPointDisplacement is TCOD_heightmap_mid_point_displacement.
func (hm *HeightMap) MidPointDisplacement(r *rng.Random, roughness float32) {
	if !hm.valid() {
		return
	}
	step := 1
	offset := float32(1.0)
	initSz := minI(hm.W, hm.H) - 1
	// The corner seeding below indexes Values[sz-1], which is Values[-1] on a
	// 1x1 map (C writes before the array there). Displacement needs at least a
	// 2x2 grid to have a midpoint at all.
	if initSz < 2 {
		return
	}
	sz := initSz
	hm.Values[0] = r.GetFloat(0.0, 1.0)
	hm.Values[sz-1] = r.GetFloat(0.0, 1.0)
	hm.Values[(sz-1)*sz] = r.GetFloat(0.0, 1.0)
	hm.Values[sz*sz-1] = r.GetFloat(0.0, 1.0)
	for sz > 0 {
		// diamond step
		for y := 0; y < step; y++ {
			for x := 0; x < step; x++ {
				diamondX := sz/2 + x*sz
				diamondY := sz/2 + y*sz
				z := hm.At(x*sz, y*sz)
				z += hm.At((x+1)*sz, y*sz)
				z += hm.At((x+1)*sz, (y+1)*sz)
				z += hm.At(x*sz, (y+1)*sz)
				z *= 0.25
				hm.setMPDHeight(r, diamondX, diamondY, z, offset)
			}
		}
		offset *= roughness
		// square step
		for y := 0; y < step; y++ {
			for x := 0; x < step; x++ {
				diamondX := sz/2 + x*sz
				diamondY := sz/2 + y*sz
				hm.setMDPHeightSquare(r, diamondX, diamondY-sz/2, initSz, sz/2, offset) // north
				hm.setMDPHeightSquare(r, diamondX, diamondY+sz/2, initSz, sz/2, offset) // south
				hm.setMDPHeightSquare(r, diamondX-sz/2, diamondY, initSz, sz/2, offset) // west
				hm.setMDPHeightSquare(r, diamondX+sz/2, diamondY, initSz, sz/2, offset) // east
			}
		}
		sz /= 2
		step *= 2
	}
}

/* --- helpers --- */

func lerp(a, b, x float32) float32 { return a + x*(b-a) }

func modf(v float32) (integer, frac float32) {
	i, f := math.Modf(float64(v))
	return float32(i), float32(f)
}

func clampF(min, max, v float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
