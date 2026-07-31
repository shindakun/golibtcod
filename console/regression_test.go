package console

import (
	"testing"

	"golibtcod/color"
)

// C compares the whole flag to TCOD_BKGND_DEFAULT; testing only the low byte
// meant an alpha-carrying flag whose low byte happened to be 13 substituted
// the console's flag instead of falling through to the switch default.
func TestDefaultFlagComparesWholeValue(t *testing.T) {
	c := New(4, 4)
	c.SetBackgroundFlag(BkgndSet)
	c.SetDefaultBackground(color.RGB{R: 1, G: 2, B: 3})

	flag := BkgndDefault | BkgndFlag(255)<<8
	before := c.Tiles[0].Bg
	c.SetCharBackground(0, 0, color.RGB{R: 200, G: 100, B: 50}, flag)
	if c.Tiles[0].Bg != before {
		t.Errorf("flag %d mutated the tile: %v -> %v", flag, before, c.Tiles[0].Bg)
	}

	// A bare BkgndDefault must still substitute the console's flag.
	c.SetCharBackground(1, 1, color.RGB{R: 200, G: 100, B: 50}, BkgndDefault)
	if got := c.Tiles[1*c.W+1].Bg; got.R != 200 || got.G != 100 || got.B != 50 {
		t.Errorf("bare BkgndDefault did not apply the console flag: %v", got)
	}
}

// The alpha-carrying constructors must keep producing usable flags.
func TestAlphaFlagConstructorsUnaffected(t *testing.T) {
	if got := AddAlpha(1.0) & 0xff; got != BkgndAddA {
		t.Errorf("AddAlpha low byte = %d, want %d", got, BkgndAddA)
	}
	if got := Alpha(1.0) & 0xff; got != BkgndAlph {
		t.Errorf("Alpha low byte = %d, want %d", got, BkgndAlph)
	}
	c := New(2, 1)
	c.SetCharBackground(0, 0, color.RGB{R: 100, G: 100, B: 100}, BkgndSet)
	c.SetCharBackground(0, 0, color.RGB{R: 50, G: 50, B: 50}, AddAlpha(1.0))
	if got := c.CharBackground(0, 0); got.R != 150 {
		t.Errorf("AddAlpha blend = %v, want R=150", got)
	}
}
