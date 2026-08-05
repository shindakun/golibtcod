package console

// Word-wrapped printing, ported from libtcod's console_printing.c
// (`next_split_` and the `printn_internal_` driver loop).
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
//
// C classifies characters with utf8proc; the standard library's unicode
// package answers the same questions, so this needs no new dependency.
// One simplification is deliberate: C's get_character_width consults
// utf8proc's charwidth but then collapses double-width characters to 1
// unless TCOD_double_width_print_mode is set, and that flag is a compile-
// time constant 0. So every printable character is one cell wide here too,
// which is what the C build actually does.

import (
	"unicode"

	"github.com/shindakun/golibtcod/color"
)

// isNewline is the C is_newline: line and paragraph separators, plus CR
// and LF among the control characters.
func isNewline(r rune) bool {
	switch r {
	case '\n', '\r', ' ', ' ':
		return true
	}
	return false
}

// isParagraphSep reports U+2029, which advances two lines rather than one.
func isParagraphSep(r rune) bool { return r == ' ' }

// isSpaceSep is utf8proc's category Zs (separator, space). Note this is
// deliberately narrower than unicode.IsSpace, which also covers \n and \t.
func isSpaceSep(r rune) bool { return unicode.Is(unicode.Zs, r) }

// isDash is utf8proc's category Pd (punctuation, dash): a break is allowed
// *after* one of these, keeping the dash on the current line.
func isDash(r rune) bool { return unicode.Is(unicode.Pd, r) }

// runeWidth is the C get_character_width. See the package note above for
// why this is always 1 for printable characters.
//
// Zero-width cases matter beyond rendering: nextSplit adds this to the
// running line width, so a control character reporting 1 would falsely
// overflow the line and force a break. utf8proc gives control characters,
// combining marks and format characters a charwidth of 0, and C relies on
// that when a newline is examined by the width logic before the
// is_newline check.
func runeWidth(r rune) int {
	switch {
	case unicode.Is(unicode.Cc, r), // control, including \n and \r
		unicode.Is(unicode.Mn, r), // non-spacing mark
		unicode.Is(unicode.Me, r), // enclosing mark
		unicode.Is(unicode.Cf, r), // format
		unicode.Is(unicode.Zl, r), // line separator
		unicode.Is(unicode.Zp, r): // paragraph separator
		return 0
	}
	return 1
}

// nextSplit is the C next_split_: find where the next line ends.
//
// It returns the index into runes at which the line breaks, the printed
// width of that line, and whether the break was introduced by wrapping (as
// opposed to running out of input or hitting an explicit newline).
func nextSplit(runes []rune, maxWidth int, canSplit bool) (breakAt, width int, addBreak bool) {
	breakAt = len(runes)
	charWidth := 0
	separating := false

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if canSplit && charWidth > 0 {
			switch {
			case isDash(r):
				if charWidth+runeWidth(r) > maxWidth {
					return i, charWidth, true
				}
				// The dash fits, so allow a break just after it.
				charWidth += runeWidth(r)
				breakAt = i + 1
				width = charWidth
				separating = true
				continue
			case isSpaceSep(r):
				if !separating {
					breakAt = i
					width = charWidth
					separating = true
				}
			default:
				if charWidth+runeWidth(r) > maxWidth {
					if breakAt != len(runes) {
						return breakAt, width, true // use the last good break
					}
					// No break opportunity: split mid-word.
					return i, charWidth, true
				}
				separating = false
			}
		}
		if isNewline(r) {
			return i, charWidth, false
		}
		charWidth += runeWidth(r)
	}
	return len(runes), charWidth, false
}

// printRect is the shared body of PrintRect and HeightRect: the C
// printn_internal_ driver loop. When countOnly is set nothing is drawn and
// only the height is computed, which is how C implements get_height_rect.
func (c *Console) printRect(x, y, w, h int, s string, fg, bg *color.RGB,
	alignment Alignment, flag BkgndFlag, countOnly bool) int {
	if c == nil {
		return 0
	}
	// C returns before any layout for an empty string, so the height is 0
	// rather than the 1 the bounding-box arithmetic would otherwise give.
	if len(s) == 0 {
		return 0
	}
	// A zero width or height extends to the console edge, as in C.
	if w <= 0 {
		w = c.W - x
	}
	if h <= 0 {
		h = c.H - y
	}
	if w <= 0 || h <= 0 {
		return 0 // the bounding box is invalid
	}

	left, right := x, x+w
	top, bottom := y, y+h
	runes := []rune(s)
	pos := 0

	for pos < len(runes) && top < bottom && top < c.H {
		r := runes[pos]
		// Explicit newlines advance without consuming a line slot.
		if isNewline(r) {
			if isParagraphSep(r) {
				top += 2
			} else {
				top++
			}
			pos++
			continue
		}

		breakAt, lineWidth, addBreak := nextSplit(runes[pos:], w, true)
		breakAt += pos

		cursorX := left
		switch alignment {
		case AlignRight:
			cursorX = right - lineWidth
		case AlignCenter:
			cursorX = left + (w-lineWidth)/2
		}

		for pos < breakAt {
			r := runes[pos]
			pos++
			if countOnly || runeWidth(r) == 0 {
				continue
			}
			if left <= cursorX && cursorX < right {
				c.putRGB(cursorX, top, int(r), fg, bg, flag)
			}
			cursorX += runeWidth(r)
		}

		// Swallow the run of spaces the break landed on.
		for pos < len(runes) && isSpaceSep(runes[pos]) {
			pos++
		}
		if addBreak {
			top++
		}
	}

	if top > bottom {
		top = bottom
	}
	return top - y + 1
}

// putRGB is TCOD_console_put_rgb: write a glyph, leaving fg or bg
// untouched when the corresponding pointer is nil.
func (c *Console) putRGB(x, y, ch int, fg, bg *color.RGB, flag BkgndFlag) {
	if !c.In(x, y) {
		return
	}
	c.Tiles[y*c.W+x].Ch = ch
	if fg != nil {
		c.SetCharForeground(x, y, *fg)
	}
	if bg != nil {
		c.SetCharBackground(x, y, *bg, flag)
	}
}

// PrintRect is TCOD_console_printn_rect: print s inside the rectangle at
// (x,y), wrapping on word boundaries. It returns the number of rows the
// text occupied.
//
// A width or height of 0 extends the rectangle to the console edge. Text
// that would fall below the rectangle is not drawn.
func (c *Console) PrintRect(x, y, w, h int, s string, fg, bg *color.RGB,
	alignment Alignment, flag BkgndFlag) int {
	return c.printRect(x, y, w, h, s, fg, bg, alignment, flag, false)
}

// HeightRect is TCOD_console_get_height_rect_n: the number of rows s would
// occupy if wrapped to the given width, without drawing anything. Use it to
// size a panel before printing into it.
//
// A height of 0 lifts the rectangle's own limit, but the result is still
// bounded by the console height, as in C: the driver loop stops at the
// bottom of the console whether or not it is drawing. Measure against a
// console at least as tall as the text to get the full layout height.
func (c *Console) HeightRect(x, y, w, h int, s string) int {
	if h <= 0 {
		h = maxHeightRect
	}
	return c.printRect(x, y, w, h, s, nil, nil, AlignLeft, BkgndNone, true)
}

// maxHeightRect stands in for C's INT_MAX when measuring an unbounded
// height. It is large enough for any console yet small enough that
// `top < bottom` arithmetic cannot overflow.
const maxHeightRect = 1 << 24
