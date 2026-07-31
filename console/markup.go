package console

// Legacy colour-control markup, ported from libtcod's console_printing.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
//
// libtcod's older printing API lets a string carry inline colour changes
// using control characters in the 1..8 range:
//
//	ColCtrl1..ColCtrl5   switch to a preset fg/bg pair (see SetColorControl)
//	ColCtrlForeRGB       followed by three bytes: set foreground to that RGB
//	ColCtrlBackRGB       followed by three bytes: set background to that RGB
//	ColCtrlStop          revert to the console's default colours
//
// Because the RGB forms embed raw bytes in the string, a channel value of
// zero would terminate a C string, so libtcod's convention is that
// callers offset each component by 1. That quirk is preserved in
// MarkupForeRGB/MarkupBackRGB, which do the offsetting for you.
//
// This is legacy surface, kept for porting existing libtcod content. New
// code should use PrintEx and change colours between calls.

import (
	"strings"
	"unicode/utf8"

	"golibtcod/color"
)

// ColCtrl values are the in-string control characters.
const (
	ColCtrl1       = 1
	ColCtrl2       = 2
	ColCtrl3       = 3
	ColCtrl4       = 4
	ColCtrl5       = 5
	ColCtrlNumber  = 5
	ColCtrlForeRGB = 6
	ColCtrlBackRGB = 7
	ColCtrlStop    = 8
)

var (
	colorControlFore [ColCtrlNumber]color.RGB
	colorControlBack [ColCtrlNumber]color.RGB
)

// SetColorControl is TCOD_console_set_color_control: bind a fg/bg pair to
// one of the five preset indices. Indices are 1-based (ColCtrl1..5).
//
// The C version stores these in process globals, and so does this: the
// markup codes are a property of the string format rather than of any one
// console, and diverging here would silently change the meaning of ported
// content.
func SetColorControl(index int, fore, back color.RGB) {
	if index < ColCtrl1 || index > ColCtrlNumber {
		return
	}
	colorControlFore[index-1] = fore
	colorControlBack[index-1] = back
}

// MarkupFore returns the control string for a preset index (1..5).
func MarkupFore(index int) string {
	if index < ColCtrl1 || index > ColCtrlNumber {
		return ""
	}
	return string(rune(index))
}

// MarkupStop returns the control string that reverts to default colours.
func MarkupStop() string { return string(rune(ColCtrlStop)) }

// MarkupForeRGB builds an inline foreground-colour control sequence.
// Channel values are offset by 1 so no embedded byte is zero, matching
// libtcod's convention.
func MarkupForeRGB(c color.RGB) string {
	return string([]rune{ColCtrlForeRGB, rune(c.R) + 1, rune(c.G) + 1, rune(c.B) + 1})
}

// MarkupBackRGB builds an inline background-colour control sequence.
func MarkupBackRGB(c color.RGB) string {
	return string([]rune{ColCtrlBackRGB, rune(c.R) + 1, rune(c.G) + 1, rune(c.B) + 1})
}

// StripMarkup removes all colour control sequences from a string,
// returning the text as it will actually appear. Useful for measuring
// widths before printing.
func StripMarkup(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == ColCtrlForeRGB || runes[i] == ColCtrlBackRGB:
			i += 3 // skip the three colour bytes
		case runes[i] > 0 && runes[i] <= ColCtrlStop:
			// a bare control code: skip it
		default:
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

// MarkupWidth returns the printed width of a string with markup removed.
func MarkupWidth(s string) int { return utf8.RuneCountInString(StripMarkup(s)) }

// PrintMarkup prints a string containing legacy colour-control codes,
// honouring alignment and the console's default colours.
//
// This is TCOD_console_print_internal_'s colour handling: control codes
// change the pen mid-string, ColCtrlStop restores the console defaults,
// and the codes themselves occupy no cells.
func (c *Console) PrintMarkup(x, y int, s string, alignment Alignment, flag BkgndFlag) {
	width := MarkupWidth(s)
	switch alignment {
	case AlignRight:
		x -= width - 1
	case AlignCenter:
		x -= width / 2
	}

	fg, bg := c.fore, c.back
	runes := []rune(s)
	col := x
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r >= ColCtrl1 && r <= ColCtrlNumber:
			fg = colorControlFore[r-1]
			bg = colorControlBack[r-1]
		case r == ColCtrlForeRGB:
			if i+3 < len(runes) {
				fg = color.RGB{
					R: uint8(runes[i+1] - 1),
					G: uint8(runes[i+2] - 1),
					B: uint8(runes[i+3] - 1),
				}
				i += 3
			}
		case r == ColCtrlBackRGB:
			if i+3 < len(runes) {
				bg = color.RGB{
					R: uint8(runes[i+1] - 1),
					G: uint8(runes[i+2] - 1),
					B: uint8(runes[i+3] - 1),
				}
				i += 3
			}
		case r == ColCtrlStop:
			fg, bg = c.fore, c.back
		default:
			if c.In(col, y) {
				c.Tiles[y*c.W+col].Ch = int(r)
				c.SetCharForeground(col, y, fg)
				c.SetCharBackground(col, y, bg, flag)
			}
			col++
		}
	}
}
