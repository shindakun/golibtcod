// Package rng is a faithful Go port of libtcod's mersenne_c.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// Both classic generators (Mersenne Twister and CMWC) reproduce the C
// bit streams exactly for a given seed, including libtcod's glibc-LCG
// CMWC seeding and its float scaling (u32 * 1/0xffffffff).
package rng

import (
	"math"
	"strconv"
	"strings"
)

type Algorithm int

const (
	MT Algorithm = iota // Mersenne Twister
	CMWC
)

type Distribution int

const (
	Linear Distribution = iota
	Gaussian
	GaussianRange
	GaussianInverse
	GaussianRangeInverse
)

// Random matches struct TCOD_Random_MT_CMWC. It is copyable: Save/Restore
// are plain value copies, as in C.
type Random struct {
	algo         Algorithm
	distribution Distribution
	// MT state
	mt    [624]uint32
	curMT int
	// CMWC state
	q   [4096]uint32
	c   uint32
	cur int
}

// New creates a generator from a 32-bit seed (TCOD_random_new_from_seed).
func New(algo Algorithm, seed uint32) *Random {
	r := &Random{algo: algo, distribution: Linear}
	if algo == MT {
		r.curMT = 624
		mtInit(seed, &r.mt)
	} else {
		s := seed
		for i := 0; i < 4096; i++ { // glibc LCG, as in C
			s = s*1103515245 + 12345
			r.q[i] = s
		}
		r.c = (s*1103515245 + 12345) % 809430660 // Marsaglia's recommended max
		r.cur = 0
	}
	return r
}

func (r *Random) SetDistribution(d Distribution) { r.distribution = d }

// Save returns a copyable snapshot (TCOD_random_save).
func (r *Random) Save() Random { return *r }

// Restore restores a snapshot (TCOD_random_restore).
func (r *Random) Restore(backup Random) { *r = backup }

/* --- core streams --- */

func mtInit(seed uint32, mt *[624]uint32) {
	mt[0] = seed
	for i := 1; i < 624; i++ {
		mt[i] = 1812433253*(mt[i-1]^(mt[i-1]>>30)) + uint32(i)
	}
}

func (r *Random) mtRand() uint32 {
	const highBit = 0x80000000
	const lowBits = 0x7fffffff
	var y uint32
	if r.curMT == 624 {
		for i := 0; i < 623; i++ {
			y = (r.mt[i] & highBit) | (r.mt[i+1] & lowBits)
			if y&1 != 0 {
				r.mt[i] = r.mt[(i+397)%624] ^ (y >> 1) ^ 2567483615
			} else {
				r.mt[i] = r.mt[(i+397)%624] ^ (y >> 1)
			}
		}
		y = (r.mt[623] & highBit) | (r.mt[0] & lowBits)
		if y&1 != 0 {
			r.mt[623] = r.mt[396] ^ (y >> 1) ^ 2567483615
		} else {
			r.mt[623] = r.mt[396] ^ (y >> 1)
		}
		r.curMT = 0
	}
	y = r.mt[r.curMT]
	r.curMT++
	y ^= y >> 11
	y ^= (y << 7) & 2636928640
	y ^= (y << 15) & 4022730752
	y ^= y >> 18
	return y
}

func (r *Random) cmwcNext() uint32 {
	r.cur = (r.cur + 1) & 4095
	t := 18782*uint64(r.q[r.cur]) + uint64(r.c)
	r.c = uint32(t >> 32)
	x := uint32(t) + r.c
	if x < r.c {
		x++
		r.c++
	}
	if x+1 == 0 {
		r.c++
		x = 0
	}
	r.q[r.cur] = 0xfffffffe - x
	return r.q[r.cur]
}

// U32 returns the next raw 32-bit value (get_random_u32).
func (r *Random) U32() uint32 {
	if r.algo == MT {
		return r.mtRand()
	}
	return r.cmwcNext()
}

func (r *Random) f32() float32 { return float32(r.U32()) * (1.0 / float32(0xffffffff)) }
func (r *Random) f64() float64 { return float64(r.U32()) * (1.0 / float64(0xffffffff)) }

/* --- linear getters (TCOD_random_get_i/f/d) --- */

// GetI returns a uniform int in [min,max] (TCOD_random_get_i).
func (r *Random) GetI(min, max int) int {
	if max == min {
		return min
	}
	if min > max {
		min, max = max, min
	}
	delta := max - min + 1
	return int(r.U32()%uint32(delta)) + min
}

// GetF returns a uniform float32 in [min,max] (TCOD_random_get_f).
func (r *Random) GetF(min, max float32) float32 {
	if max == min {
		return min
	}
	if min > max {
		min, max = max, min
	}
	return min + r.f32()*(max-min)
}

// GetD returns a uniform float64 in [min,max] (TCOD_random_get_d).
func (r *Random) GetD(min, max float64) float64 {
	if max == min {
		return min
	}
	if min > max {
		min, max = max, min
	}
	return min + r.f64()*(max-min)
}

/* --- gaussian family (Box-Muller, exact C structure) --- */

func (r *Random) gaussD(mean, stdDev float64) float64 {
	var x1, x2, w float64
	for {
		x1 = r.f64()*2 - 1
		x2 = r.f64()*2 - 1
		w = x1*x1 + x2*x2
		if w < 1.0 {
			break
		}
	}
	w = math.Sqrt((-2.0 * math.Log(w)) / w)
	return mean + x1*w*stdDev
}

func roundC(num float64) int { // C's (int)(num±0.5) rounding
	if num >= 0 {
		return int(num + 0.5)
	}
	return int(num - 0.5)
}

func clampD(min, max, v float64) float64 { return math.Min(max, math.Max(min, v)) }

func (r *Random) gaussRangeD(min, max float64) float64 {
	if min > max {
		min, max = max, min
	}
	mean := (min + max) / 2
	std := (max - min) / 6.0 // three-sigma rule
	return clampD(min, max, r.gaussD(mean, std))
}

func (r *Random) gaussRangeCustomD(min, max, mean float64) float64 {
	if min > max {
		min, max = max, min
	}
	std := math.Max(max-mean, mean-min) / 3.0
	return clampD(min, max, r.gaussD(mean, std))
}

func (r *Random) gaussInvD(mean, stdDev float64) float64 {
	num := r.gaussD(mean, stdDev)
	if num >= mean {
		return num - 3*stdDev
	}
	return num + 3*stdDev
}

func (r *Random) gaussRangeInvD(min, max float64) float64 {
	if min > max {
		min, max = max, min
	}
	mean := (min + max) / 2
	std := (max - min) / 6.0
	return clampD(min, max, r.gaussInvD(mean, std))
}

func (r *Random) gaussRangeCustomInvD(min, max, mean float64) float64 {
	if min > max {
		min, max = max, min
	}
	std := math.Max(max-mean, mean-min) / 3.0
	return clampD(min, max, r.gaussInvD(mean, std))
}

/* --- distribution-aware getters (TCOD_random_get_int/float/double) --- */

// GetInt returns an int using the generator's current distribution.
func (r *Random) GetInt(min, max int) int {
	switch r.distribution {
	case Gaussian:
		return roundC(r.gaussD(float64(min), float64(max))) // C passes min/max as mean/std here
	case GaussianInverse:
		num := r.gaussInvD(float64(min), float64(max))
		return roundC(num)
	case GaussianRange:
		if min > max {
			min, max = max, min
		}
		v := roundC(r.gaussRangeD(float64(min), float64(max)))
		return clampI(min, max, v)
	case GaussianRangeInverse:
		if min > max {
			min, max = max, min
		}
		v := roundC(r.gaussRangeInvD(float64(min), float64(max)))
		return clampI(min, max, v)
	default:
		return r.GetI(min, max)
	}
}

// GetFloat returns a float32 using the generator's current distribution.
func (r *Random) GetFloat(min, max float32) float32 {
	switch r.distribution {
	case Gaussian:
		return float32(r.gaussD(float64(min), float64(max)))
	case GaussianInverse:
		mean, std := min, max
		num := float32(r.gaussD(float64(mean), float64(std)))
		if num >= mean {
			return num - 3*std
		}
		return num + 3*std
	case GaussianRange:
		if min > max {
			min, max = max, min
		}
		return clampF(min, max, float32(r.gaussRangeD(float64(min), float64(max))))
	case GaussianRangeInverse:
		if min > max {
			min, max = max, min
		}
		return clampF(min, max, float32(r.gaussRangeInvD(float64(min), float64(max))))
	default:
		return r.GetF(min, max)
	}
}

// GetDouble returns a float64 using the generator's current distribution.
func (r *Random) GetDouble(min, max float64) float64 {
	switch r.distribution {
	case Gaussian:
		return r.gaussD(min, max)
	case GaussianInverse:
		return r.gaussInvD(min, max)
	case GaussianRange:
		return r.gaussRangeD(min, max)
	case GaussianRangeInverse:
		return r.gaussRangeInvD(min, max)
	default:
		return r.GetD(min, max)
	}
}

// GetIntMean is TCOD_random_get_int_mean.
func (r *Random) GetIntMean(min, max, mean int) int {
	var num float64
	switch r.distribution {
	case GaussianInverse, GaussianRangeInverse:
		num = r.gaussRangeCustomInvD(float64(min), float64(max), float64(mean))
	default:
		num = r.gaussRangeCustomD(float64(min), float64(max), float64(mean))
	}
	if min > max {
		min, max = max, min
	}
	return clampI(min, max, roundC(num))
}

// GetDoubleMean is TCOD_random_get_double_mean.
func (r *Random) GetDoubleMean(min, max, mean float64) float64 {
	switch r.distribution {
	case GaussianInverse, GaussianRangeInverse:
		return r.gaussRangeCustomInvD(min, max, mean)
	default:
		return r.gaussRangeCustomD(min, max, mean)
	}
}

func clampI(min, max, v int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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

/* --- dice (TCOD_dice_t) --- */

type Dice struct {
	Rolls, Faces       int
	Multiplier, AddSub float32
}

// ParseDice parses classic dice notation, e.g. "3d6+2", "1.5x2d10-1"
// (TCOD_random_dice_new).
func ParseDice(s string) Dice {
	d := Dice{Rolls: 1, Faces: 1, Multiplier: 1}
	if i := strings.IndexAny(s, "*x"); i >= 0 {
		if f, err := strconv.ParseFloat(s[:i], 32); err == nil {
			d.Multiplier = float32(f)
		}
		s = s[i+1:]
	}
	i := strings.IndexAny(s, "dD")
	if i < 0 {
		i = len(s)
	}
	d.Rolls = atoiPrefix(s[:i])
	if i < len(s) {
		s = s[i+1:]
	} else {
		s = ""
	}
	i = strings.IndexAny(s, "+-")
	if i < 0 {
		i = len(s)
	}
	d.Faces = atoiPrefix(s[:i])
	s = s[i:]
	if len(s) > 0 {
		sign := float32(1)
		if s[0] == '-' {
			sign = -1
		}
		if f, err := strconv.ParseFloat(s[1:], 32); err == nil {
			d.AddSub = float32(f) * sign
		}
	}
	return d
}

// atoiPrefix follows C's atoi: skip leading whitespace, accept an optional
// sign, consume digits, and truncate to 32 bits (C's atoi returns int).
// Matching the width matters as well as the grammar: Go's int is 64-bit, so
// without truncation "99999999999d6" yields a roll count that makes Roll
// spin for minutes where C wraps to a merely large number.
func atoiPrefix(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' ||
		s[i] == '\v' || s[i] == '\f' || s[i] == '\r') {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var n int64
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int64(s[i]-'0')
		if n > 1<<62 { // stop before overflowing; the int32 cast bounds it anyway
			break
		}
	}
	if neg {
		n = -n
	}
	return int(int32(n))
}

// MaxRolls bounds Roll's iteration count. ParseDice reproduces C's 32-bit
// atoi exactly, so "99999999999d6" yields 1215752191 rolls in both languages;
// C spends minutes on that, which in Go would let a config string hang the
// process. Counts above this are clamped rather than honored.
const MaxRolls = 1 << 20

// Roll rolls dice (TCOD_random_dice_roll). The roll count is clamped to
// MaxRolls; see that constant for why this diverges from C.
func (r *Random) Roll(d Dice) int {
	rolls := d.Rolls
	if rolls > MaxRolls {
		rolls = MaxRolls
	}
	result := 0
	for i := 0; i < rolls; i++ {
		result += r.GetI(1, d.Faces)
	}
	return int((float32(result) + d.AddSub) * d.Multiplier)
}

// RollS parses and rolls, e.g. r.RollS("3d6") (TCOD_random_dice_roll_s).
func (r *Random) RollS(s string) int { return r.Roll(ParseDice(s)) }
