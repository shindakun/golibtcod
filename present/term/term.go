// Package term is a terminal Presenter for golibtcod: renders a Console to
// any ANSI truecolor terminal using 24-bit SGR sequences. Alongside
// present/pngout it fulfills the presenter contract: the same console,
// rendered two different ways, with no third-party dependencies.
package term

import (
	"fmt"
	"io"
	"strings"

	"github.com/shindakun/golibtcod/console"
)

// Options controls terminal output.
type Options struct {
	// ClearFirst emits a home+clear before drawing (for animation loops).
	ClearFirst bool
	// TrailingReset emits SGR reset + newline after the frame (default on
	// via Render; disable only for manual cursor control).
	NoTrailingReset bool
}

// Render writes the console to w as ANSI truecolor text, one line per
// console row. Runs of identical fg/bg share one SGR sequence.
func Render(c *console.Console, w io.Writer, o Options) error {
	var b strings.Builder
	b.Grow(c.W * c.H * 8)
	if o.ClearFirst {
		b.WriteString("\x1b[H\x1b[2J")
	}
	for y := 0; y < c.H; y++ {
		var lastFg, lastBg [3]uint8
		first := true
		for x := 0; x < c.W; x++ {
			t := c.Tiles[y*c.W+x]
			fg := [3]uint8{t.Fg.R, t.Fg.G, t.Fg.B}
			bg := [3]uint8{t.Bg.R, t.Bg.G, t.Bg.B}
			if first || fg != lastFg || bg != lastBg {
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm",
					fg[0], fg[1], fg[2], bg[0], bg[1], bg[2])
				lastFg, lastBg = fg, bg
				first = false
			}
			ch := t.Ch
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(rune(ch))
		}
		b.WriteString("\x1b[0m\n")
	}
	if !o.NoTrailingReset {
		b.WriteString("\x1b[0m")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
