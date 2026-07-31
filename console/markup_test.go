package console

import (
	"testing"

	"golibtcod/color"
)

func TestStripAndWidth(t *testing.T) {
	s := MarkupFore(1) + "red" + MarkupStop() + "plain"
	if got := StripMarkup(s); got != "redplain" {
		t.Fatalf("strip = %q", got)
	}
	if got := MarkupWidth(s); got != 8 {
		t.Fatalf("width = %d, want 8", got)
	}
	rgb := MarkupForeRGB(color.RGB{R: 10, G: 20, B: 30}) + "abc"
	if got := StripMarkup(rgb); got != "abc" {
		t.Fatalf("rgb strip = %q", got)
	}
}

func TestPresetColorControl(t *testing.T) {
	SetColorControl(1, color.Red, color.Blue)
	c := New(12, 1)
	c.SetDefaultForeground(color.White)
	c.SetDefaultBackground(color.Black)
	c.Clear()
	c.PrintMarkup(0, 0, "a"+MarkupFore(1)+"b"+MarkupStop()+"c", AlignLeft, BkgndSet)

	if c.Char(0, 0) != 'a' || c.Char(1, 0) != 'b' || c.Char(2, 0) != 'c' {
		t.Fatalf("control codes should occupy no cells: %c%c%c",
			c.Char(0, 0), c.Char(1, 0), c.Char(2, 0))
	}
	if got := c.CharForeground(0, 0); got != color.White {
		t.Errorf("cell 0 fg = %+v, want default white", got)
	}
	if got := c.CharForeground(1, 0); got != color.Red {
		t.Errorf("cell 1 fg = %+v, want red from the preset", got)
	}
	if got := c.CharBackground(1, 0); got != color.Blue {
		t.Errorf("cell 1 bg = %+v, want blue from the preset", got)
	}
	if got := c.CharForeground(2, 0); got != color.White {
		t.Errorf("cell 2 fg = %+v, want default restored by Stop", got)
	}
}

// The +1 offset on channel values is libtcod's convention: a zero byte
// would terminate a C string. Round-tripping it is the thing to verify.
func TestInlineRGB(t *testing.T) {
	c := New(8, 1)
	c.SetDefaultForeground(color.White)
	c.Clear()
	want := color.RGB{R: 0, G: 128, B: 255}
	c.PrintMarkup(0, 0, MarkupForeRGB(want)+"x", AlignLeft, BkgndNone)
	if got := c.CharForeground(0, 0); got != want {
		t.Fatalf("inline fg = %+v, want %+v (offset handling)", got, want)
	}

	c.Clear()
	wantBg := color.RGB{R: 9, G: 0, B: 200}
	c.PrintMarkup(0, 0, MarkupBackRGB(wantBg)+"y", AlignLeft, BkgndSet)
	if got := c.CharBackground(0, 0); got != wantBg {
		t.Fatalf("inline bg = %+v, want %+v", got, wantBg)
	}
}

// Alignment must be computed on the visible width, not the raw string:
// otherwise every control code shifts the text.
func TestAlignmentIgnoresControlCodes(t *testing.T) {
	plain := New(11, 1)
	plain.Clear()
	plain.PrintMarkup(5, 0, "abc", AlignCenter, BkgndNone)

	marked := New(11, 1)
	marked.Clear()
	marked.PrintMarkup(5, 0, MarkupFore(2)+"abc"+MarkupStop(), AlignCenter, BkgndNone)

	for x := 0; x < 11; x++ {
		if plain.Char(x, 0) != marked.Char(x, 0) {
			t.Fatalf("markup shifted the text at column %d: %U vs %U",
				x, rune(plain.Char(x, 0)), rune(marked.Char(x, 0)))
		}
	}
}

func TestMalformedMarkupDoesNotPanic(t *testing.T) {
	c := New(6, 1)
	c.Clear()
	// truncated RGB sequence: three bytes promised, none delivered
	c.PrintMarkup(0, 0, string(rune(ColCtrlForeRGB)), AlignLeft, BkgndNone)
	c.PrintMarkup(0, 0, MarkupFore(99), AlignLeft, BkgndNone)
}
