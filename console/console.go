// Package console is a faithful Go port of libtcod's console core:
// console.c (tiles, background blend flags, blit) and the essentials of
// console_drawing.c / console_printing.c (rect, lines, frames, print).
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package console

import "github.com/shindakun/golibtcod/color"

// BkgndFlag mirrors TCOD_bkgnd_flag_t. For AddAlpha/Alpha use the
// AddAlpha()/Alpha() constructors, which encode the alpha in the flag as
// the C API does.
type BkgndFlag int

const (
	BkgndNone BkgndFlag = iota
	BkgndSet
	BkgndMultiply
	BkgndLighten
	BkgndDarken
	BkgndScreen
	BkgndColorDodge
	BkgndColorBurn
	BkgndAdd
	BkgndAddA
	BkgndBurn
	BkgndOverlay
	BkgndAlph
	BkgndDefault
)

// AddAlpha is TCOD_BKGND_ADDALPHA(alpha) with alpha in [0,1].
func AddAlpha(alpha float32) BkgndFlag {
	return BkgndAddA | BkgndFlag(int(alpha*255))<<8
}

// Alpha is TCOD_BKGND_ALPHA(alpha) with alpha in [0,1].
func Alpha(alpha float32) BkgndFlag {
	return BkgndAlph | BkgndFlag(int(alpha*255))<<8
}

// Tile mirrors TCOD_ConsoleTile.
type Tile struct {
	Ch int // unicode codepoint
	Fg color.RGBA
	Bg color.RGBA
}

// Console mirrors TCOD_Console.
type Console struct {
	W, H  int
	Tiles []Tile

	// default colors/flag used by PutChar/Print* (con->fore, con->back)
	fore      color.RGB
	back      color.RGB
	bkgndFlag BkgndFlag

	hasKeyColor bool
	keyColor    color.RGB
}

func New(w, h int) *Console {
	if w <= 0 || h <= 0 {
		return nil
	}
	c := &Console{
		W: w, H: h,
		Tiles:     make([]Tile, w*h),
		fore:      color.White,
		back:      color.Black,
		bkgndFlag: BkgndNone,
	}
	c.Clear()
	return c
}

func (c *Console) In(x, y int) bool { return 0 <= x && x < c.W && 0 <= y && y < c.H }

func (c *Console) SetDefaultForeground(col color.RGB) { c.fore = col }
func (c *Console) SetDefaultBackground(col color.RGB) { c.back = col }
func (c *Console) DefaultForeground() color.RGB       { return c.fore }
func (c *Console) DefaultBackground() color.RGB       { return c.back }
func (c *Console) SetBackgroundFlag(flag BkgndFlag)   { c.bkgndFlag = flag }
func (c *Console) BackgroundFlag() BkgndFlag          { return c.bkgndFlag }

// SetKeyColor is TCOD_console_set_key_color: bg pixels of this color are
// treated as transparent during Blit.
func (c *Console) SetKeyColor(col color.RGB) {
	c.hasKeyColor = true
	c.keyColor = col
}

// Clear is TCOD_console_clear: fills with spaces in the default colors.
func (c *Console) Clear() {
	fill := Tile{
		Ch: ' ',
		Fg: color.RGBA{R: c.fore.R, G: c.fore.G, B: c.fore.B, A: 255},
		Bg: color.RGBA{R: c.back.R, G: c.back.G, B: c.back.B, A: 255},
	}
	for i := range c.Tiles {
		c.Tiles[i] = fill
	}
}

/* --- per-cell accessors --- */

// SetChar is TCOD_console_set_char.
func (c *Console) SetChar(x, y int, ch int) {
	if !c.In(x, y) {
		return
	}
	c.Tiles[y*c.W+x].Ch = ch
}

// Char is TCOD_console_get_char.
func (c *Console) Char(x, y int) int {
	if !c.In(x, y) {
		return 0
	}
	return c.Tiles[y*c.W+x].Ch
}

// SetCharForeground is TCOD_console_set_char_foreground.
func (c *Console) SetCharForeground(x, y int, col color.RGB) {
	if !c.In(x, y) {
		return
	}
	c.Tiles[y*c.W+x].Fg = color.RGBA{R: col.R, G: col.G, B: col.B, A: 255}
}

// CharForeground is TCOD_console_get_char_foreground.
func (c *Console) CharForeground(x, y int) color.RGB {
	if !c.In(x, y) {
		return color.White
	}
	f := c.Tiles[y*c.W+x].Fg
	return color.RGB{R: f.R, G: f.G, B: f.B}
}

// CharBackground is TCOD_console_get_char_background.
func (c *Console) CharBackground(x, y int) color.RGB {
	if !c.In(x, y) {
		return color.Black
	}
	b := c.Tiles[y*c.W+x].Bg
	return color.RGB{R: b.R, G: b.G, B: b.B}
}

/* --- background blending (TCOD_console_set_char_background) --- */

func clampColor(c int) uint8 {
	if c < 0 {
		return 0
	}
	if c > 255 {
		return 255
	}
	return uint8(c)
}

func blendColor(bg *color.RGBA, fg color.RGB, lambda func(dst, src uint8) int) color.RGBA {
	return color.RGBA{
		R: clampColor(lambda(bg.R, fg.R)),
		G: clampColor(lambda(bg.G, fg.G)),
		B: clampColor(lambda(bg.B, fg.B)),
		A: bg.A,
	}
}

func channelMultiply(dst, src uint8) int { return int(dst) * int(src) / 255 }
func channelLighten(dst, src uint8) int {
	if dst > src {
		return int(dst)
	}
	return int(src)
}
func channelDarken(dst, src uint8) int {
	if dst < src {
		return int(dst)
	}
	return int(src)
}
func channelScreen(dst, src uint8) int { return 255 - (255-int(dst))*(255-int(src))/255 }
func channelColorDodge(dst, src uint8) int {
	if dst == 255 {
		return 255
	}
	return 255 * int(src) / (255 - int(dst))
}
func channelColorBurn(dst, src uint8) int {
	if src == 0 {
		return 0
	}
	return 255 - (255*(255-int(dst)))/int(src)
}
func channelAdd(dst, src uint8) int  { return int(dst) + int(src) }
func channelBurn(dst, src uint8) int { return int(dst) + int(src) - 255 }
func channelOverlay(dst, src uint8) int {
	if int(src) <= 128 {
		return 2 * int(src) * int(dst) / 255
	}
	return 255 - 2*(255-int(src))*(255-int(dst))/255
}

// SetCharBackground is TCOD_console_set_char_background.
func (c *Console) SetCharBackground(x, y int, col color.RGB, flag BkgndFlag) {
	if !c.In(x, y) {
		return
	}
	bg := &c.Tiles[y*c.W+x].Bg
	// C compares the whole flag, not just the low byte: an alpha-carrying
	// flag whose low byte happens to be BkgndDefault must fall through to the
	// switch's default (no change) rather than substituting the console's.
	if flag == BkgndDefault {
		flag = c.bkgndFlag
	}
	alpha := uint8((flag >> 8) & 0xFF)
	switch flag & 0xff {
	case BkgndSet:
		bg.R, bg.G, bg.B = col.R, col.G, col.B
	case BkgndMultiply:
		*bg = blendColor(bg, col, channelMultiply)
	case BkgndLighten:
		*bg = blendColor(bg, col, channelLighten)
	case BkgndDarken:
		*bg = blendColor(bg, col, channelDarken)
	case BkgndScreen:
		*bg = blendColor(bg, col, channelScreen)
	case BkgndColorDodge:
		*bg = blendColor(bg, col, channelColorDodge)
	case BkgndColorBurn:
		*bg = blendColor(bg, col, channelColorBurn)
	case BkgndAdd:
		*bg = blendColor(bg, col, channelAdd)
	case BkgndAddA:
		bg.R = clampColor(int(bg.R) + int(alpha)*int(col.R)/255)
		bg.G = clampColor(int(bg.G) + int(alpha)*int(col.G)/255)
		bg.B = clampColor(int(bg.B) + int(alpha)*int(col.B)/255)
	case BkgndBurn:
		*bg = blendColor(bg, col, channelBurn)
	case BkgndOverlay:
		*bg = blendColor(bg, col, channelOverlay)
	case BkgndAlph:
		*bg = blitLerp(*bg, color.RGBA{R: col.R, G: col.G, B: col.B, A: alpha}, 1.0)
	}
}

// PutChar is TCOD_console_put_char (uses the default colors).
func (c *Console) PutChar(x, y int, ch int, flag BkgndFlag) {
	if !c.In(x, y) {
		return
	}
	c.Tiles[y*c.W+x].Ch = ch
	c.SetCharForeground(x, y, c.fore)
	c.SetCharBackground(x, y, c.back, flag)
}

// PutCharEx is TCOD_console_put_char_ex.
func (c *Console) PutCharEx(x, y int, ch int, fore, back color.RGB) {
	if !c.In(x, y) {
		return
	}
	c.Tiles[y*c.W+x].Ch = ch
	c.SetCharForeground(x, y, fore)
	c.SetCharBackground(x, y, back, BkgndSet)
}

/* --- blit (TCOD_console_blit) --- */

func alphaBlend(srcC, srcA, dstC, dstA, outA int) uint8 {
	return uint8(((srcC * srcA) + (dstC * dstA * (255 - srcA) / 255)) / outA)
}

// blitLerp is TCOD_console_blit_lerp_.
func blitLerp(dst, src color.RGBA, interp float32) color.RGBA {
	outA := uint8(int(src.A) + int(dst.A)*(255-int(src.A))/255)
	if outA == 0 {
		return dst
	}
	srcA := uint8(float32(src.A) * interp)
	return color.RGBA{
		R: alphaBlend(int(src.R), int(srcA), int(dst.R), int(dst.A), int(outA)),
		G: alphaBlend(int(src.G), int(srcA), int(dst.G), int(dst.A), int(outA)),
		B: alphaBlend(int(src.B), int(srcA), int(dst.B), int(dst.A), int(outA)),
		A: outA,
	}
}

// blitCell is TCOD_console_blit_cell_.
func blitCell(src, dst *Tile, fgAlpha, bgAlpha float32, keyColor *color.RGB) Tile {
	if keyColor != nil && keyColor.R == src.Bg.R && keyColor.G == src.Bg.G && keyColor.B == src.Bg.B {
		return *dst // source pixel is transparent
	}
	fgAlpha *= float32(src.Fg.A) / 255.0
	bgAlpha *= float32(src.Bg.A) / 255.0
	if fgAlpha > 254.5/255.0 && bgAlpha > 254.5/255.0 {
		return *src // no alpha; plain copy
	}
	out := *dst
	out.Bg = blitLerp(out.Bg, src.Bg, bgAlpha)
	switch {
	case src.Ch == ' ':
		// source is space, keep the current glyph
		out.Fg = blitLerp(out.Fg, src.Bg, bgAlpha)
	case out.Ch == ' ':
		// destination is space, use the glyph from source
		out.Ch = src.Ch
		out.Fg = blitLerp(out.Bg, src.Fg, fgAlpha)
	case out.Ch == src.Ch:
		out.Fg = blitLerp(out.Fg, src.Fg, fgAlpha)
	default:
		// pick the glyph based on fgAlpha
		if fgAlpha < 0.5 {
			out.Fg = blitLerp(out.Fg, out.Bg, fgAlpha*2)
		} else {
			out.Ch = src.Ch
			out.Fg = blitLerp(out.Bg, src.Fg, (fgAlpha-0.5)*2)
		}
	}
	return out
}

// Blit is TCOD_console_blit. wSrc/hSrc of 0 mean the whole source.
func Blit(src *Console, xSrc, ySrc, wSrc, hSrc int, dst *Console, xDst, yDst int, foregroundAlpha, backgroundAlpha float32) {
	var key *color.RGB
	if src.hasKeyColor {
		k := src.keyColor
		key = &k
	}
	BlitKeyColor(src, xSrc, ySrc, wSrc, hSrc, dst, xDst, yDst, foregroundAlpha, backgroundAlpha, key)
}

// BlitKeyColor is TCOD_console_blit_key_color.
func BlitKeyColor(src *Console, xSrc, ySrc, wSrc, hSrc int, dst *Console, xDst, yDst int, foregroundAlpha, backgroundAlpha float32, keyColor *color.RGB) {
	if src == nil || dst == nil {
		return
	}
	if wSrc == 0 {
		wSrc = src.W
	}
	if hSrc == 0 {
		hSrc = src.H
	}
	if wSrc <= 0 || hSrc <= 0 {
		return
	}
	if xDst+wSrc < 0 || yDst+hSrc < 0 || xDst >= dst.W || yDst >= dst.H {
		return
	}
	for cx := xSrc; cx < xSrc+wSrc; cx++ {
		for cy := ySrc; cy < ySrc+hSrc; cy++ {
			dx := cx - xSrc + xDst
			dy := cy - ySrc + yDst
			if !src.In(cx, cy) || !dst.In(dx, dy) {
				continue
			}
			dst.Tiles[dy*dst.W+dx] = blitCell(
				&src.Tiles[cy*src.W+cx], &dst.Tiles[dy*dst.W+dx],
				foregroundAlpha, backgroundAlpha, keyColor)
		}
	}
}

/* --- drawing (console_drawing.c essentials) --- */

// Rect is TCOD_console_rect: fills a region with the default background;
// clear also resets glyphs to space.
func (c *Console) Rect(x, y, rw, rh int, clear bool, flag BkgndFlag) {
	for cx := x; cx < x+rw; cx++ {
		for cy := y; cy < y+rh; cy++ {
			if !c.In(cx, cy) {
				continue
			}
			c.SetCharBackground(cx, cy, c.back, flag)
			if clear {
				c.Tiles[cy*c.W+cx].Ch = ' '
			}
		}
	}
}

// HLine is TCOD_console_hline (glyph U+2500 '─').
func (c *Console) HLine(x, y, l int, flag BkgndFlag) {
	for i := x; i < x+l; i++ {
		c.PutChar(i, y, '─', flag)
	}
}

// VLine is TCOD_console_vline (glyph U+2502 '│').
func (c *Console) VLine(x, y, l int, flag BkgndFlag) {
	for i := y; i < y+l; i++ {
		c.PutChar(x, i, '│', flag)
	}
}

// Frame draws a single-line box frame (TCOD_console_print_frame's border,
// without the legacy title banner).
func (c *Console) Frame(x, y, w, h int, clear bool, flag BkgndFlag) {
	c.PutChar(x, y, '┌', flag)
	c.PutChar(x+w-1, y, '┐', flag)
	c.PutChar(x, y+h-1, '└', flag)
	c.PutChar(x+w-1, y+h-1, '┘', flag)
	c.HLine(x+1, y, w-2, flag)
	c.HLine(x+1, y+h-1, w-2, flag)
	if h > 2 {
		c.VLine(x, y+1, h-2, flag)
		c.VLine(x+w-1, y+1, h-2, flag)
	}
	if clear {
		c.Rect(x+1, y+1, w-2, h-2, true, flag)
	}
}

/* --- printing (console_printing.c essentials) --- */

// Alignment mirrors TCOD_alignment_t.
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignRight
	AlignCenter
)

// PrintEx prints s at x,y with explicit colors, alignment, and blend flag.
// This is the modern TCOD_console_printn core without the format markup.
func (c *Console) PrintEx(x, y int, s string, fg, bg *color.RGB, alignment Alignment, flag BkgndFlag) {
	runes := []rune(s)
	switch alignment {
	case AlignRight:
		x -= len(runes) - 1
	case AlignCenter:
		x -= len(runes) / 2
	}
	for i, r := range runes {
		cx := x + i
		if !c.In(cx, y) {
			continue
		}
		if r != ' ' || bg != nil { // spaces only paint if bg requested
			c.Tiles[y*c.W+cx].Ch = int(r)
		}
		if fg != nil {
			c.SetCharForeground(cx, y, *fg)
		}
		if bg != nil {
			c.SetCharBackground(cx, y, *bg, flag)
		}
	}
}

// Print prints with the console's default colors, left-aligned, BkgndSet.
func (c *Console) Print(x, y int, s string) {
	fg, bg := c.fore, c.back
	c.PrintEx(x, y, s, &fg, &bg, AlignLeft, BkgndSet)
}
