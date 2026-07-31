// Package color is a faithful Go port of libtcod's color.c: RGB/RGBA
// types, arithmetic, HSV manipulation, gradients, and alpha blending.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package color

import "math"

// RGB mirrors TCOD_color_t / TCOD_ColorRGB.
type RGB struct{ R, G, B uint8 }

// RGBA mirrors TCOD_ColorRGBA.
type RGBA struct{ R, G, B, A uint8 }

func New(r, g, b uint8) RGB { return RGB{r, g, b} }

// HSV is TCOD_color_HSV.
func HSV(hue, saturation, value float32) RGB {
	var c RGB
	c.SetHSV(hue, saturation, value)
	return c
}

func (c RGB) Equals(other RGB) bool { return c == other }

func clampI(c int) uint8 {
	if c < 0 {
		return 0
	}
	if c > 255 {
		return 255
	}
	return uint8(c)
}

// Add is TCOD_color_add (saturating).
func Add(c1, c2 RGB) RGB {
	return RGB{clampI(int(c1.R) + int(c2.R)), clampI(int(c1.G) + int(c2.G)), clampI(int(c1.B) + int(c2.B))}
}

// Subtract is TCOD_color_subtract (saturating).
func Subtract(c1, c2 RGB) RGB {
	return RGB{clampI(int(c1.R) - int(c2.R)), clampI(int(c1.G) - int(c2.G)), clampI(int(c1.B) - int(c2.B))}
}

// Multiply is TCOD_color_multiply.
func Multiply(c1, c2 RGB) RGB {
	return RGB{
		uint8(int(c1.R) * int(c2.R) / 255),
		uint8(int(c1.G) * int(c2.G) / 255),
		uint8(int(c1.B) * int(c2.B) / 255),
	}
}

// MultiplyScalar is TCOD_color_multiply_scalar.
func MultiplyScalar(c RGB, value float32) RGB {
	cl := func(v float32) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	return RGB{cl(float32(c.R) * value), cl(float32(c.G) * value), cl(float32(c.B) * value)}
}

// Lerp is TCOD_color_lerp.
func Lerp(c1, c2 RGB, coef float32) RGB {
	return RGB{
		uint8(float32(c1.R) + float32(int(c2.R)-int(c1.R))*coef),
		uint8(float32(c1.G) + float32(int(c2.G)-int(c1.G))*coef),
		uint8(float32(c1.B) + float32(int(c2.B)-int(c1.B))*coef),
	}
}

// fabsmod is floor modulo for float values, as in C.
func fabsmod(x, n float32) float32 {
	m := float32(math.Mod(float64(x), float64(n)))
	if m < 0 {
		return m + n
	}
	return m
}

// SetHSV is TCOD_color_set_HSV. hue in degrees; s, v in [0,1].
func (c *RGB) SetHSV(hue, saturation, value float32) {
	clamp01 := func(v float32) float32 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	saturation = clamp01(saturation)
	value = clamp01(value)
	if saturation == 0.0 { // achromatic (grey)
		g := uint8(value*255.0 + 0.5)
		c.R, c.G, c.B = g, g, g
		return
	}
	hue = fabsmod(hue, 360.0)
	hue /= 60.0 // sector 0..5
	hueSection := int(math.Floor(float64(hue)))
	hueFraction := hue - float32(hueSection)
	p := value * (1 - saturation)
	q := value * (1 - saturation*hueFraction)
	t := value * (1 - saturation*(1-hueFraction))
	b := func(v float32) uint8 { return uint8(v*255.0 + 0.5) }
	switch hueSection {
	case 1: // yellow/green
		c.R, c.G, c.B = b(q), b(value), b(p)
	case 2: // green/cyan
		c.R, c.G, c.B = b(p), b(value), b(t)
	case 3: // cyan/blue
		c.R, c.G, c.B = b(p), b(q), b(value)
	case 4: // blue/purple
		c.R, c.G, c.B = b(t), b(p), b(value)
	case 5: // purple/red
		c.R, c.G, c.B = b(value), b(p), b(q)
	default: // 0: red/yellow
		c.R, c.G, c.B = b(value), b(t), b(p)
	}
}

// Hue is TCOD_color_get_hue (degrees).
func (c RGB) Hue() float32 {
	mx := maxU8(c.R, maxU8(c.G, c.B))
	mn := minU8(c.R, minU8(c.G, c.B))
	delta := float32(mx) - float32(mn)
	if delta == 0.0 {
		return 0.0 // achromatic, including black
	}
	var hue float32
	switch {
	case c.R == mx:
		hue = float32(int(c.G)-int(c.B)) / delta
	case c.G == mx:
		hue = 2.0 + float32(int(c.B)-int(c.R))/delta
	default:
		hue = 4.0 + float32(int(c.R)-int(c.G))/delta
	}
	hue *= 60.0
	return fabsmod(hue, 360.0)
}

// Saturation is TCOD_color_get_saturation.
func (c RGB) Saturation() float32 {
	mx := float32(maxU8(c.R, maxU8(c.G, c.B))) / 255.0
	mn := float32(minU8(c.R, minU8(c.G, c.B))) / 255.0
	if mx == 0.0 {
		return 0.0
	}
	return (mx - mn) / mx
}

// Value is TCOD_color_get_value.
func (c RGB) Value() float32 { return float32(maxU8(c.R, maxU8(c.G, c.B))) / 255.0 }

// GetHSV is TCOD_color_get_HSV.
func (c RGB) GetHSV() (hue, saturation, value float32) {
	return c.Hue(), c.Saturation(), c.Value()
}

func (c *RGB) SetHue(hue float32)      { c.SetHSV(hue, c.Saturation(), c.Value()) }
func (c *RGB) SetSaturation(s float32) { c.SetHSV(c.Hue(), s, c.Value()) }
func (c *RGB) SetValue(v float32)      { c.SetHSV(c.Hue(), c.Saturation(), v) }

// ShiftHue is TCOD_color_shift_hue.
func (c *RGB) ShiftHue(shift float32) {
	if shift == 0.0 {
		return
	}
	c.SetHSV(c.Hue()+shift, c.Saturation(), c.Value())
}

// ScaleHSV is TCOD_color_scale_HSV.
func (c *RGB) ScaleHSV(saturationCoef, valueCoef float32) {
	c.SetHSV(c.Hue(), c.Saturation()*saturationCoef, c.Value()*valueCoef)
}

// GenMap is TCOD_color_gen_map: fills m by interpolating keyColor stops
// placed at keyIndex positions.
func GenMap(m []RGB, keyColor []RGB, keyIndex []int) {
	for segment := 0; segment < len(keyColor)-1; segment++ {
		idxStart := keyIndex[segment]
		idxEnd := keyIndex[segment+1]
		for idx := idxStart; idx <= idxEnd; idx++ {
			m[idx] = Lerp(keyColor[segment], keyColor[segment+1], float32(idx-idxStart)/float32(idxEnd-idxStart))
		}
	}
}

func alphaBlendChannel(dstC, dstA, srcC, srcA, outA int) uint8 {
	return uint8(((srcC * srcA) + (dstC * dstA * (255 - srcA) / 255)) / outA)
}

// AlphaBlend is TCOD_color_alpha_blend: blends src over dst in place.
func AlphaBlend(dst *RGBA, src RGBA) {
	outA := uint8(int(src.A) + int(dst.A)*(255-int(src.A))/255)
	dst.R = alphaBlendChannel(int(dst.R), int(dst.A), int(src.R), int(src.A), int(outA))
	dst.G = alphaBlendChannel(int(dst.G), int(dst.A), int(src.G), int(src.A), int(outA))
	dst.B = alphaBlendChannel(int(dst.B), int(dst.A), int(src.B), int(src.A), int(outA))
	dst.A = outA
}

func minU8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

func maxU8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

// Grey levels and base named colors from libtcod's classic palette
// (color.h). The full generated light/dark/desaturated table is large;
// this covers the standard hues at normal level plus greys. More can be
// derived with Lerp/ScaleHSV.
var (
	Black         = RGB{0, 0, 0}
	DarkestGrey   = RGB{31, 31, 31}
	DarkerGrey    = RGB{63, 63, 63}
	DarkGrey      = RGB{95, 95, 95}
	Grey          = RGB{127, 127, 127}
	LightGrey     = RGB{159, 159, 159}
	LighterGrey   = RGB{191, 191, 191}
	LightestGrey  = RGB{223, 223, 223}
	White         = RGB{255, 255, 255}
	DarkestSepia  = RGB{31, 24, 15}
	DarkerSepia   = RGB{63, 50, 31}
	DarkSepia     = RGB{94, 75, 47}
	Sepia         = RGB{127, 101, 63}
	LightSepia    = RGB{158, 134, 100}
	LighterSepia  = RGB{191, 171, 143}
	LightestSepia = RGB{222, 211, 195}
	Red           = RGB{255, 0, 0}
	Flame         = RGB{255, 63, 0}
	Orange        = RGB{255, 127, 0}
	Amber         = RGB{255, 191, 0}
	Yellow        = RGB{255, 255, 0}
	Lime          = RGB{191, 255, 0}
	Chartreuse    = RGB{127, 255, 0}
	Green         = RGB{0, 255, 0}
	Sea           = RGB{0, 255, 127}
	Turquoise     = RGB{0, 255, 191}
	Cyan          = RGB{0, 255, 255}
	Sky           = RGB{0, 191, 255}
	Azure         = RGB{0, 127, 255}
	Blue          = RGB{0, 0, 255}
	Han           = RGB{63, 0, 255}
	Violet        = RGB{127, 0, 255}
	Purple        = RGB{191, 0, 255}
	Fuchsia       = RGB{255, 0, 255}
	Magenta       = RGB{255, 0, 191}
	Pink          = RGB{255, 0, 127}
	Crimson       = RGB{255, 0, 63}
	Brass         = RGB{191, 151, 96}
	Copper        = RGB{197, 136, 124}
	Gold          = RGB{229, 191, 0}
	Silver        = RGB{203, 203, 203}
	Celadon       = RGB{172, 255, 175}
	Peach         = RGB{255, 159, 127}
)
