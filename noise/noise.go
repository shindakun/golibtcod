// Package noise is a faithful Go port of libtcod's noise_c.c: Perlin,
// simplex (Gustavson-style), and wavelet (Cook & DeRose) noise in 1-4
// dimensions, with fBm and turbulence fractal variants.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// float32 is used throughout to mirror C float behavior. Initialization
// consumes the rng stream in the same order as C, so a seeded generator
// yields the same gradient tables.
package noise

import (
	"math"

	"golibtcod/rng"
)

type Type int

const (
	Default Type = 0 // resolves to Simplex, as in C
	Perlin  Type = 1
	Simplex Type = 2
	Wavelet Type = 4
)

const (
	MaxOctaves    = 128
	MaxDimensions = 4

	DefaultHurst      = 0.5
	DefaultLacunarity = 2.0

	waveletTileSize = 32
	waveletARad     = 16

	simplexScale = 0.5
	waveletScale = 2.0

	delta = 1e-6
)

// Noise mirrors TCOD_Noise.
type Noise struct {
	ndim            int
	buffer          [256][MaxDimensions]float32 // random gradients
	m               [256]uint8                  // randomized map (C: map)
	h               float32                     // hurst
	lacunarity      float32
	exponent        [MaxOctaves]float32
	waveletTileData []float32
	noiseType       Type
	rand            *rng.Random
}

// New is TCOD_noise_new. r must not be nil (no global generator in golibtcod).
func New(ndim int, hurst, lacunarity float32, r *rng.Random) *Noise {
	n := &Noise{ndim: ndim, rand: r, h: hurst, lacunarity: lacunarity, noiseType: Default}
	for i := 0; i < 256; i++ {
		n.m[i] = uint8(i)
		for j := 0; j < ndim; j++ {
			n.buffer[i][j] = r.GetFloat(-0.5, 0.5)
		}
		n.normalize(n.buffer[i][:])
	}
	for i := 255; i >= 0; i-- {
		j := r.GetInt(0, 255)
		n.m[i], n.m[j] = n.m[j], n.m[i]
	}
	f := float32(1)
	for i := 0; i < MaxOctaves; i++ {
		n.exponent[i] = 1.0 / f
		f *= lacunarity
	}
	return n
}

func (n *Noise) SetType(t Type) { n.noiseType = t }

func (n *Noise) normalize(f []float32) {
	var magnitude float32
	for i := 0; i < n.ndim; i++ {
		magnitude += f[i] * f[i]
	}
	magnitude = 1.0 / float32(math.Sqrt(float64(magnitude)))
	for i := 0; i < n.ndim; i++ {
		f[i] *= magnitude
	}
}

const f32Eps = 1.1920929e-07 // FLT_EPSILON

// clampSignedF excludes -1 and 1 to avoid rounding issues, as in C.
func clampSignedF(v float32) float32 {
	const low = -1.0 + f32Eps
	const high = 1.0 - f32Eps
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func floorI(a float32) int {
	if a > 0 {
		return int(a)
	}
	return int(a) - 1
}

func cubic(a float32) float32 { return a * a * (3 - 2*a) }

func lerp(a, b, x float32) float32 { return a + x*(b-a) }

func absmod(x, n int) int {
	m := x % n
	if m < 0 {
		return m + n
	}
	return m
}

/* --- Perlin --- */

func (d *Noise) lattice(ix int, fx float32, iy int, fy float32, iz int, fz float32, iw int, fw float32) float32 {
	n := [4]int{ix, iy, iz, iw}
	f := [4]float32{fx, fy, fz, fw}
	nIndex := 0
	for i := 0; i < d.ndim; i++ {
		nIndex = int(d.m[(nIndex+n[i])&0xFF])
	}
	var value float32
	for i := 0; i < d.ndim; i++ {
		value += d.buffer[nIndex][i] * f[i]
	}
	return value
}

func (d *Noise) perlin(f []float32) float32 {
	var n [MaxDimensions]int
	var r, w [MaxDimensions]float32
	for i := 0; i < d.ndim; i++ {
		n[i] = floorI(f[i])
		r[i] = f[i] - float32(n[i])
		w[i] = cubic(r[i])
	}
	var value float32
	switch d.ndim {
	case 1:
		value = lerp(
			d.lattice(n[0], r[0], 0, 0, 0, 0, 0, 0),
			d.lattice(n[0]+1, r[0]-1, 0, 0, 0, 0, 0, 0), w[0])
	case 2:
		value = lerp(
			lerp(
				d.lattice(n[0], r[0], n[1], r[1], 0, 0, 0, 0),
				d.lattice(n[0]+1, r[0]-1, n[1], r[1], 0, 0, 0, 0), w[0]),
			lerp(
				d.lattice(n[0], r[0], n[1]+1, r[1]-1, 0, 0, 0, 0),
				d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, 0, 0, 0, 0), w[0]),
			w[1])
	case 3:
		value = lerp(
			lerp(
				lerp(
					d.lattice(n[0], r[0], n[1], r[1], n[2], r[2], 0, 0),
					d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2], r[2], 0, 0), w[0]),
				lerp(
					d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2], r[2], 0, 0),
					d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2], r[2], 0, 0), w[0]),
				w[1]),
			lerp(
				lerp(
					d.lattice(n[0], r[0], n[1], r[1], n[2]+1, r[2]-1, 0, 0),
					d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2]+1, r[2]-1, 0, 0), w[0]),
				lerp(
					d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2]+1, r[2]-1, 0, 0),
					d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2]+1, r[2]-1, 0, 0), w[0]),
				w[1]),
			w[2])
	case 4:
		value = lerp(
			lerp(
				lerp(
					lerp(
						d.lattice(n[0], r[0], n[1], r[1], n[2], r[2], n[3], r[3]),
						d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2], r[2], n[3], r[3]), w[0]),
					lerp(
						d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2], r[2], n[3], r[3]),
						d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2], r[2], n[3], r[3]), w[0]),
					w[1]),
				lerp(
					lerp(
						d.lattice(n[0], r[0], n[1], r[1], n[2]+1, r[2]-1, n[3], r[3]),
						d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2]+1, r[2]-1, n[3], r[3]), w[0]),
					lerp(
						// C quirk preserved: this lattice call passes trailing 0,0
						d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2]+1, r[2]-1, 0, 0),
						d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2]+1, r[2]-1, n[3], r[3]), w[0]),
					w[1]),
				w[2]),
			lerp(
				lerp(
					lerp(
						d.lattice(n[0], r[0], n[1], r[1], n[2], r[2], n[3]+1, r[3]-1),
						d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2], r[2], n[3]+1, r[3]-1), w[0]),
					lerp(
						d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2], r[2], n[3]+1, r[3]-1),
						d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2], r[2], n[3]+1, r[3]-1), w[0]),
					w[1]),
				lerp(
					lerp(
						d.lattice(n[0], r[0], n[1], r[1], n[2]+1, r[2]-1, n[3]+1, r[3]-1),
						d.lattice(n[0]+1, r[0]-1, n[1], r[1], n[2]+1, r[2]-1, n[3]+1, r[3]-1), w[0]),
					lerp(
						// C quirk preserved: trailing 0,0 here too
						d.lattice(n[0], r[0], n[1]+1, r[1]-1, n[2]+1, r[2]-1, 0, 0),
						d.lattice(n[0]+1, r[0]-1, n[1]+1, r[1]-1, n[2]+1, r[2]-1, n[3]+1, r[3]-1), w[0]),
					w[1]),
				w[2]),
			w[3])
	default:
		return float32(math.NaN())
	}
	return clampSignedF(value)
}

/* --- simplex --- */

func gradient1D(h int, x float32) float32 {
	h &= 0xF
	grad := 1.0 + float32(h&7)
	if h&8 != 0 {
		grad = -grad
	}
	return grad * x
}

func gradient2D(h int, x, y float32) float32 {
	var u, v float32
	h &= 0x7
	if h < 4 {
		u, v = x, 2.0*y
	} else {
		u, v = y, 2.0*x
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
}

func gradient3D(h int, x, y, z float32) float32 {
	h &= 0xF
	u := x
	if h >= 8 {
		u = y
	}
	var v float32
	if h < 4 {
		v = y
	} else if h == 12 || h == 14 {
		v = x
	} else {
		v = z
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
}

func gradient4D(h int, x, y, z, t float32) float32 {
	h &= 0x1F
	u := x
	if h >= 24 {
		u = y
	}
	v := y
	if h >= 16 {
		v = z
	}
	w := z
	if h >= 8 {
		w = t
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	if h&4 != 0 {
		w = -w
	}
	return u + v + w
}

var simplexTable = [64][4]float32{
	{0, 1, 2, 3}, {0, 1, 3, 2}, {0, 0, 0, 0}, {0, 2, 3, 1}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {1, 2, 3, 0},
	{0, 2, 1, 3}, {0, 0, 0, 0}, {0, 3, 1, 2}, {0, 3, 2, 1}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {1, 3, 2, 0},
	{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0},
	{1, 2, 0, 3}, {0, 0, 0, 0}, {1, 3, 0, 2}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {2, 3, 0, 1}, {2, 3, 1, 0},
	{1, 0, 2, 3}, {1, 0, 3, 2}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {2, 0, 3, 1}, {0, 0, 0, 0}, {2, 1, 3, 0},
	{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0},
	{2, 0, 1, 3}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {3, 0, 1, 2}, {3, 0, 2, 1}, {0, 0, 0, 0}, {3, 1, 2, 0},
	{2, 1, 0, 3}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {3, 1, 0, 2}, {0, 0, 0, 0}, {3, 2, 0, 1}, {3, 2, 1, 0},
}

func (d *Noise) simplex(f []float32) float32 {
	switch d.ndim {
	case 1:
		i0 := floorI(f[0] * simplexScale)
		i1 := i0 + 1
		x0 := f[0]*simplexScale - float32(i0)
		x1 := x0 - 1.0
		t0 := 1.0 - x0*x0
		t1 := 1.0 - x1*x1
		t0 = t0 * t0
		t1 = t1 * t1
		n0 := gradient1D(int(d.m[i0&0xFF]), x0) * t0 * t0
		n1 := gradient1D(int(d.m[i1&0xFF]), x1) * t1 * t1
		return clampSignedF(0.25 * (n0 + n1))
	case 2:
		const F2 = 0.366025403 // 0.5*(sqrt(3)-1)
		const G2 = 0.211324865 // (3-sqrt(3))/6
		s := (f[0] + f[1]) * F2 * simplexScale
		xs := f[0]*simplexScale + s
		ys := f[1]*simplexScale + s
		i := floorI(xs)
		j := floorI(ys)
		t := float32(i+j) * G2
		xo := float32(i) - t
		yo := float32(j) - t
		x0 := f[0]*simplexScale - xo
		y0 := f[1]*simplexScale - yo
		ii := absmod(i, 256)
		jj := absmod(j, 256)
		var i1, j1 int
		if x0 > y0 {
			i1, j1 = 1, 0
		} else {
			i1, j1 = 0, 1
		}
		x1 := x0 - float32(i1) + G2
		y1 := y0 - float32(j1) + G2
		x2 := x0 - 1.0 + 2.0*G2
		y2 := y0 - 1.0 + 2.0*G2
		var n0, n1, n2 float32
		if t0 := 0.5 - x0*x0 - y0*y0; t0 >= 0 {
			idx := (ii + int(d.m[jj])) & 0xFF
			t0 *= t0
			n0 = gradient2D(int(d.m[idx]), x0, y0) * t0 * t0
		}
		if t1 := 0.5 - x1*x1 - y1*y1; t1 >= 0 {
			idx := (ii + i1 + int(d.m[(jj+j1)&0xFF])) & 0xFF
			t1 *= t1
			n1 = gradient2D(int(d.m[idx]), x1, y1) * t1 * t1
		}
		if t2 := 0.5 - x2*x2 - y2*y2; t2 >= 0 {
			idx := (ii + 1 + int(d.m[(jj+1)&0xFF])) & 0xFF
			t2 *= t2
			n2 = gradient2D(int(d.m[idx]), x2, y2) * t2 * t2
		}
		return clampSignedF(40.0 * (n0 + n1 + n2))
	case 3:
		const F3 = 0.333333333
		const G3 = 0.166666667
		s := (f[0] + f[1] + f[2]) * F3 * simplexScale
		xs := f[0]*simplexScale + s
		ys := f[1]*simplexScale + s
		zs := f[2]*simplexScale + s
		i := floorI(xs)
		j := floorI(ys)
		k := floorI(zs)
		t := float32(i+j+k) * G3
		xo := float32(i) - t
		yo := float32(j) - t
		zo := float32(k) - t
		x0 := f[0]*simplexScale - xo
		y0 := f[1]*simplexScale - yo
		z0 := f[2]*simplexScale - zo
		var i1, j1, k1, i2, j2, k2 int
		if x0 >= y0 {
			if y0 >= z0 {
				i1, j1, k1, i2, j2, k2 = 1, 0, 0, 1, 1, 0
			} else if x0 >= z0 {
				i1, j1, k1, i2, j2, k2 = 1, 0, 0, 1, 0, 1
			} else {
				i1, j1, k1, i2, j2, k2 = 0, 0, 1, 1, 0, 1
			}
		} else {
			if y0 < z0 {
				i1, j1, k1, i2, j2, k2 = 0, 0, 1, 0, 1, 1
			} else if x0 < z0 {
				i1, j1, k1, i2, j2, k2 = 0, 1, 0, 0, 1, 1
			} else {
				i1, j1, k1, i2, j2, k2 = 0, 1, 0, 1, 1, 0
			}
		}
		x1 := x0 - float32(i1) + G3
		y1 := y0 - float32(j1) + G3
		z1 := z0 - float32(k1) + G3
		x2 := x0 - float32(i2) + 2.0*G3
		y2 := y0 - float32(j2) + 2.0*G3
		z2 := z0 - float32(k2) + 2.0*G3
		x3 := x0 - 1.0 + 3.0*G3
		y3 := y0 - 1.0 + 3.0*G3
		z3 := z0 - 1.0 + 3.0*G3
		ii := absmod(i, 256)
		jj := absmod(j, 256)
		kk := absmod(k, 256)
		var n0, n1, n2, n3 float32
		if t0 := 0.6 - x0*x0 - y0*y0 - z0*z0; t0 >= 0 {
			idx := int(d.m[(ii+int(d.m[(jj+int(d.m[kk]))&0xFF]))&0xFF])
			t0 *= t0
			n0 = gradient3D(idx, x0, y0, z0) * t0 * t0
		}
		if t1 := 0.6 - x1*x1 - y1*y1 - z1*z1; t1 >= 0 {
			idx := int(d.m[(ii+i1+int(d.m[(jj+j1+int(d.m[(kk+k1)&0xFF]))&0xFF]))&0xFF])
			t1 *= t1
			n1 = gradient3D(idx, x1, y1, z1) * t1 * t1
		}
		if t2 := 0.6 - x2*x2 - y2*y2 - z2*z2; t2 >= 0 {
			idx := int(d.m[(ii+i2+int(d.m[(jj+j2+int(d.m[(kk+k2)&0xFF]))&0xFF]))&0xFF])
			t2 *= t2
			n2 = gradient3D(idx, x2, y2, z2) * t2 * t2
		}
		if t3 := 0.6 - x3*x3 - y3*y3 - z3*z3; t3 >= 0 {
			idx := int(d.m[(ii+1+int(d.m[(jj+1+int(d.m[(kk+1)&0xFF]))&0xFF]))&0xFF])
			t3 *= t3
			n3 = gradient3D(idx, x3, y3, z3) * t3 * t3
		}
		return clampSignedF(32.0 * (n0 + n1 + n2 + n3))
	case 4:
		const F4 = 0.309016994 // (sqrt(5)-1)/4
		const G4 = 0.138196601 // (5-sqrt(5))/20
		s := (f[0] + f[1] + f[2] + f[3]) * F4 * simplexScale
		xs := f[0]*simplexScale + s
		ys := f[1]*simplexScale + s
		zs := f[2]*simplexScale + s
		ws := f[3]*simplexScale + s
		i := floorI(xs)
		j := floorI(ys)
		k := floorI(zs)
		l := floorI(ws)
		t := float32(i+j+k+l) * G4
		xo := float32(i) - t
		yo := float32(j) - t
		zo := float32(k) - t
		wo := float32(l) - t
		x0 := f[0]*simplexScale - xo
		y0 := f[1]*simplexScale - yo
		z0 := f[2]*simplexScale - zo
		w0 := f[3]*simplexScale - wo
		c := 0
		if x0 > y0 {
			c += 32
		}
		if x0 > z0 {
			c += 16
		}
		if y0 > z0 {
			c += 8
		}
		if x0 > w0 {
			c += 4
		}
		if y0 > w0 {
			c += 2
		}
		if z0 > w0 {
			c += 1
		}
		b := func(v float32, threshold float32) int {
			if v >= threshold {
				return 1
			}
			return 0
		}
		i1 := b(simplexTable[c][0], 3)
		j1 := b(simplexTable[c][1], 3)
		k1 := b(simplexTable[c][2], 3)
		l1 := b(simplexTable[c][3], 3)
		i2 := b(simplexTable[c][0], 2)
		j2 := b(simplexTable[c][1], 2)
		k2 := b(simplexTable[c][2], 2)
		l2 := b(simplexTable[c][3], 2)
		i3 := b(simplexTable[c][0], 1)
		j3 := b(simplexTable[c][1], 1)
		k3 := b(simplexTable[c][2], 1)
		l3 := b(simplexTable[c][3], 1)
		x1 := x0 - float32(i1) + G4
		y1 := y0 - float32(j1) + G4
		z1 := z0 - float32(k1) + G4
		w1 := w0 - float32(l1) + G4
		x2 := x0 - float32(i2) + 2.0*G4
		y2 := y0 - float32(j2) + 2.0*G4
		z2 := z0 - float32(k2) + 2.0*G4
		w2 := w0 - float32(l2) + 2.0*G4
		x3 := x0 - float32(i3) + 3.0*G4
		y3 := y0 - float32(j3) + 3.0*G4
		z3 := z0 - float32(k3) + 3.0*G4
		w3 := w0 - float32(l3) + 3.0*G4
		x4 := x0 - 1.0 + 4.0*G4
		y4 := y0 - 1.0 + 4.0*G4
		z4 := z0 - 1.0 + 4.0*G4
		w4 := w0 - 1.0 + 4.0*G4
		ii := absmod(i, 256)
		jj := absmod(j, 256)
		kk := absmod(k, 256)
		ll := absmod(l, 256)
		var n0, n1, n2, n3, n4 float32
		if t0 := 0.6 - x0*x0 - y0*y0 - z0*z0 - w0*w0; t0 >= 0 {
			idx := int(d.m[(ii+int(d.m[(jj+int(d.m[(kk+int(d.m[ll]))&0xFF]))&0xFF]))&0xFF])
			t0 *= t0
			n0 = gradient4D(idx, x0, y0, z0, w0) * t0 * t0
		}
		if t1 := 0.6 - x1*x1 - y1*y1 - z1*z1 - w1*w1; t1 >= 0 {
			idx := int(d.m[(ii+i1+int(d.m[(jj+j1+int(d.m[(kk+k1+int(d.m[(ll+l1)&0xFF]))&0xFF]))&0xFF]))&0xFF])
			t1 *= t1
			n1 = gradient4D(idx, x1, y1, z1, w1) * t1 * t1
		}
		if t2 := 0.6 - x2*x2 - y2*y2 - z2*z2 - w2*w2; t2 >= 0 {
			idx := int(d.m[(ii+i2+int(d.m[(jj+j2+int(d.m[(kk+k2+int(d.m[(ll+l2)&0xFF]))&0xFF]))&0xFF]))&0xFF])
			t2 *= t2
			n2 = gradient4D(idx, x2, y2, z2, w2) * t2 * t2
		}
		if t3 := 0.6 - x3*x3 - y3*y3 - z3*z3 - w3*w3; t3 >= 0 {
			idx := int(d.m[(ii+i3+int(d.m[(jj+j3+int(d.m[(kk+k3+int(d.m[(ll+l3)&0xFF]))&0xFF]))&0xFF]))&0xFF])
			t3 *= t3
			n3 = gradient4D(idx, x3, y3, z3, w3) * t3 * t3
		}
		if t4 := 0.6 - x4*x4 - y4*y4 - z4*z4 - w4*w4; t4 >= 0 {
			idx := int(d.m[(ii+1+int(d.m[(jj+1+int(d.m[(kk+1+int(d.m[(ll+1)&0xFF]))&0xFF]))&0xFF]))&0xFF])
			t4 *= t4
			n4 = gradient4D(idx, x4, y4, z4, w4) * t4 * t4
		}
		return clampSignedF(27.0 * (n0 + n1 + n2 + n3 + n4))
	default:
		return float32(math.NaN())
	}
}

/* --- wavelet --- */

var aCoefficients = [2 * waveletARad]float32{
	0.000334, -0.001528, 0.000410, 0.003545, -0.000938, -0.008233, 0.002172, 0.019120,
	-0.005040, -0.044412, 0.011655, 0.103311, -0.025936, -0.243780, 0.033979, 0.655340,
	0.655340, 0.033979, -0.243780, -0.025936, 0.103311, 0.011655, -0.044412, -0.005040,
	0.019120, 0.002172, -0.008233, -0.000938, 0.003546, 0.000410, -0.001528, 0.000334,
}

func waveletDownsample(from, to []float32, stride int) {
	a := func(i int) float32 { return aCoefficients[i+waveletARad] }
	for i := 0; i < waveletTileSize/2; i++ {
		to[i*stride] = 0
		for k := 2*i - waveletARad; k < 2*i+waveletARad; k++ {
			to[i*stride] += a(k-2*i) * from[absmod(k, waveletTileSize)*stride]
		}
	}
}

func waveletUpsample(from, to []float32, stride int) {
	p := [4]float32{0.25, 0.75, 0.75, 0.25}
	pAt := func(i int) float32 { return p[i+2] }
	for i := 0; i < waveletTileSize; i++ {
		to[i*stride] = 0
		for k := i / 2; k < i/2+1; k++ {
			to[i*stride] += pAt(i-2*k) * from[absmod(k, waveletTileSize/2)*stride]
		}
	}
}

func (d *Noise) waveletInit() {
	const n3 = waveletTileSize * waveletTileSize * waveletTileSize
	temp1 := make([]float32, n3)
	temp2 := make([]float32, n3)
	noiseData := make([]float32, n3)
	for i := range noiseData {
		noiseData[i] = d.rand.GetFloat(-1.0, 1.0)
	}
	for iy := 0; iy < waveletTileSize; iy++ {
		for iz := 0; iz < waveletTileSize; iz++ {
			i := iy*waveletTileSize + iz*waveletTileSize*waveletTileSize
			waveletDownsample(noiseData[i:], temp1[i:], 1)
			waveletUpsample(temp1[i:], temp2[i:], 1)
		}
	}
	for ix := 0; ix < waveletTileSize; ix++ {
		for iz := 0; iz < waveletTileSize; iz++ {
			i := ix + iz*waveletTileSize*waveletTileSize
			waveletDownsample(temp2[i:], temp1[i:], waveletTileSize)
			waveletUpsample(temp1[i:], temp2[i:], waveletTileSize)
		}
	}
	for ix := 0; ix < waveletTileSize; ix++ {
		for iy := 0; iy < waveletTileSize; iy++ {
			i := ix + iy*waveletTileSize
			waveletDownsample(temp2[i:], temp1[i:], waveletTileSize*waveletTileSize)
			waveletUpsample(temp1[i:], temp2[i:], waveletTileSize*waveletTileSize)
		}
	}
	for i := 0; i < n3; i++ {
		noiseData[i] -= temp2[i]
	}
	offset := waveletTileSize / 2
	if offset&1 == 0 {
		offset++
	}
	i := 0
	for ix := 0; ix < waveletTileSize; ix++ {
		for iy := 0; iy < waveletTileSize; iy++ {
			for iz := 0; iz < waveletTileSize; iz++ {
				temp1[i] = noiseData[absmod(ix+offset, waveletTileSize)+
					absmod(iy+offset, waveletTileSize)*waveletTileSize+
					absmod(iz+offset, waveletTileSize)*waveletTileSize*waveletTileSize]
				i++
			}
		}
	}
	for i := 0; i < n3; i++ {
		noiseData[i] += temp1[i]
	}
	d.waveletTileData = noiseData
}

func (d *Noise) wavelet(f []float32) float32 {
	const n = waveletTileSize
	if d.ndim <= 0 || d.ndim > 3 {
		return float32(math.NaN())
	}
	if d.waveletTileData == nil {
		d.waveletInit()
	}
	var pf [3]float32
	for i := 0; i < d.ndim; i++ {
		pf[i] = f[i] * waveletScale
	}
	var mid [3]int
	var w [3][3]float32
	for i := 0; i < 3; i++ {
		mid[i] = int(math.Ceil(float64(pf[i] - 0.5)))
		t := float32(mid[i]) - (pf[i] - 0.5)
		w[i][0] = t * t * 0.5
		w[i][2] = (1.0 - t) * (1.0 - t) * 0.5
		w[i][1] = 1.0 - w[i][0] - w[i][2]
	}
	var result float32
	var p, c [3]int
	for p[2] = -1; p[2] <= 1; p[2]++ {
		for p[1] = -1; p[1] <= 1; p[1]++ {
			for p[0] = -1; p[0] <= 1; p[0]++ {
				weight := float32(1.0)
				for i := 0; i < 3; i++ {
					c[i] = absmod(mid[i]+p[i], n)
					weight *= w[i][p[i]+1]
				}
				result += weight * d.waveletTileData[c[2]*n*n+c[1]*n+c[0]]
			}
		}
	}
	return clampSignedF(result)
}

/* --- fractal variants and public getters --- */

func (d *Noise) fbmInt(f []float32, octaves float32, fn func([]float32) float32) float32 {
	var tf [MaxDimensions]float32
	for i := 0; i < d.ndim; i++ {
		tf[i] = f[i]
	}
	var value float32
	i := 0
	for ; i < int(octaves); i++ {
		value += fn(tf[:]) * d.exponent[i]
		for j := 0; j < d.ndim; j++ {
			tf[j] *= d.lacunarity
		}
	}
	octaves -= float32(int(octaves))
	if octaves > delta {
		value += octaves * fn(tf[:]) * d.exponent[i]
	}
	return clampSignedF(value)
}

func (d *Noise) turbulenceInt(f []float32, octaves float32, fn func([]float32) float32) float32 {
	var tf [MaxDimensions]float32
	for i := 0; i < d.ndim; i++ {
		tf[i] = f[i]
	}
	var value float32
	i := 0
	for ; i < int(octaves); i++ {
		nv := fn(tf[:])
		value += abs32(nv) * d.exponent[i]
		for j := 0; j < d.ndim; j++ {
			tf[j] *= d.lacunarity
		}
	}
	octaves -= float32(int(octaves))
	if octaves > delta {
		nv := fn(tf[:])
		value += octaves * abs32(nv) * d.exponent[i]
	}
	return clampSignedF(value)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (d *Noise) fnFor(t Type) func([]float32) float32 {
	if t == 0 {
		t = d.noiseType
	}
	switch t {
	case Perlin:
		return d.perlin
	case Wavelet:
		return d.wavelet
	default: // Default and Simplex
		return d.simplex
	}
}

// GetEx is TCOD_noise_get_ex.
func (d *Noise) GetEx(f []float32, t Type) float32 { return d.fnFor(t)(f) }

// Get is TCOD_noise_get.
func (d *Noise) Get(f ...float32) float32 { return d.GetEx(f, d.noiseType) }

// GetFbmEx is TCOD_noise_get_fbm_ex.
func (d *Noise) GetFbmEx(f []float32, octaves float32, t Type) float32 {
	return d.fbmInt(f, octaves, d.fnFor(t))
}

// GetFbm is TCOD_noise_get_fbm.
func (d *Noise) GetFbm(f []float32, octaves float32) float32 {
	return d.GetFbmEx(f, octaves, d.noiseType)
}

// GetTurbulenceEx is TCOD_noise_get_turbulence_ex.
func (d *Noise) GetTurbulenceEx(f []float32, octaves float32, t Type) float32 {
	return d.turbulenceInt(f, octaves, d.fnFor(t))
}

// GetTurbulence is TCOD_noise_get_turbulence.
func (d *Noise) GetTurbulence(f []float32, octaves float32) float32 {
	return d.GetTurbulenceEx(f, octaves, d.noiseType)
}
