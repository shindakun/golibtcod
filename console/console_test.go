package console

import (
	"testing"

	"github.com/shindakun/golibtcod/color"
)

func TestBlendModes(t *testing.T) {
	c := New(3, 1)
	c.SetCharBackground(0, 0, color.RGB{R: 100, G: 100, B: 100}, BkgndSet)
	c.SetCharBackground(0, 0, color.RGB{R: 128, G: 128, B: 128}, BkgndMultiply)
	if got := c.CharBackground(0, 0); got.R != 50 {
		t.Fatalf("multiply = %+v, want ~50", got)
	}
	c.SetCharBackground(1, 0, color.RGB{R: 200, G: 10, B: 100}, BkgndSet)
	c.SetCharBackground(1, 0, color.RGB{R: 100, G: 100, B: 100}, BkgndLighten)
	if got := c.CharBackground(1, 0); got.R != 200 || got.G != 100 || got.B != 100 {
		t.Fatalf("lighten = %+v", got)
	}
	c.SetCharBackground(2, 0, color.RGB{R: 200, G: 200, B: 200}, BkgndSet)
	c.SetCharBackground(2, 0, color.RGB{R: 100, G: 100, B: 100}, BkgndAdd)
	if got := c.CharBackground(2, 0); got.R != 255 {
		t.Fatalf("add should saturate: %+v", got)
	}
}

func TestBlitKeyColor(t *testing.T) {
	src := New(2, 1)
	src.SetDefaultBackground(color.RGB{R: 255, G: 0, B: 255}) // magenta key
	src.Clear()
	src.PutCharEx(0, 0, '@', color.White, color.RGB{R: 255, G: 0, B: 255})
	src.PutCharEx(1, 0, '#', color.White, color.Black)
	src.SetKeyColor(color.RGB{R: 255, G: 0, B: 255})

	dst := New(2, 1)
	dst.SetDefaultBackground(color.RGB{R: 10, G: 20, B: 30})
	dst.Clear()
	Blit(src, 0, 0, 0, 0, dst, 0, 0, 1.0, 1.0)

	// Cell 0 has key-color bg: dst bg preserved, but glyph... per C the whole
	// cell is skipped, so the dst tile is unchanged.
	if got := dst.CharBackground(0, 0); got.R != 10 {
		t.Fatalf("key color cell overwritten: %+v", got)
	}
	if dst.Char(0, 0) != ' ' {
		t.Fatalf("key color cell glyph = %c", dst.Char(0, 0))
	}
	// Cell 1 is opaque: full copy.
	if dst.Char(1, 0) != '#' {
		t.Fatalf("opaque cell not copied: %c", dst.Char(1, 0))
	}
}

func TestPrintAlignment(t *testing.T) {
	c := New(11, 1)
	fg := color.White
	c.PrintEx(5, 0, "abc", &fg, nil, AlignCenter, BkgndNone)
	if c.Char(4, 0) != 'a' || c.Char(5, 0) != 'b' || c.Char(6, 0) != 'c' {
		t.Fatalf("center alignment wrong: %c%c%c", c.Char(4, 0), c.Char(5, 0), c.Char(6, 0))
	}
	c.Clear()
	c.PrintEx(10, 0, "abc", &fg, nil, AlignRight, BkgndNone)
	if c.Char(8, 0) != 'a' || c.Char(10, 0) != 'c' {
		t.Fatal("right alignment wrong")
	}
}

func TestGenMapGradient(t *testing.T) {
	var m [8]color.RGB
	color.GenMap(m[:], []color.RGB{color.Black, color.White}, []int{0, 7})
	if m[0] != color.Black || m[7] != color.White {
		t.Fatalf("gradient endpoints wrong: %+v %+v", m[0], m[7])
	}
	if m[3].R <= m[1].R {
		t.Fatal("gradient not increasing")
	}
}
