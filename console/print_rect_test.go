package console

import (
	"strings"
	"testing"

	"github.com/shindakun/golibtcod/color"
)

func rows(c *Console) []string {
	out := make([]string, c.H)
	for y := 0; y < c.H; y++ {
		var b strings.Builder
		for x := 0; x < c.W; x++ {
			ch := c.Char(x, y)
			if ch == 0 || ch == ' ' {
				b.WriteByte(' ')
			} else {
				b.WriteRune(rune(ch))
			}
		}
		out[y] = strings.TrimRight(b.String(), " ")
	}
	return out
}

func TestPrintRectWraps(t *testing.T) {
	c := New(20, 4)
	c.Clear()
	h := c.PrintRect(0, 0, 20, 4, "the quick brown fox jumps over the lazy dog",
		nil, nil, AlignLeft, BkgndNone)
	want := []string{"the quick brown fox", "jumps over the lazy", "dog", ""}
	got := rows(c)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
	if h != 3 {
		t.Errorf("height = %d, want 3", h)
	}
}

// Print silently truncated at the console edge; PrintRect must not.
func TestPrintRectDoesNotTruncate(t *testing.T) {
	const text = "the quick brown fox jumps over the lazy dog"
	c := New(20, 6)
	c.Clear()
	c.PrintRect(0, 0, 20, 6, text, nil, nil, AlignLeft, BkgndNone)
	var n int
	for _, r := range rows(c) {
		n += len(strings.ReplaceAll(r, " ", ""))
	}
	if want := len(strings.ReplaceAll(text, " ", "")); n != want {
		t.Errorf("rendered %d non-space chars, want %d", n, want)
	}
}

func TestPrintRectAlignment(t *testing.T) {
	for _, tc := range []struct {
		align Alignment
		want  string
	}{
		{AlignLeft, "abc"},
		{AlignRight, "       abc"},
		{AlignCenter, "   abc"},
	} {
		c := New(10, 2)
		c.Clear()
		c.PrintRect(0, 0, 10, 2, "abc", nil, nil, tc.align, BkgndNone)
		if got := rows(c)[0]; got != tc.want {
			t.Errorf("align %v = %q, want %q", tc.align, got, tc.want)
		}
	}
}

// A word longer than the line is split rather than dropped.
func TestPrintRectSplitsLongWord(t *testing.T) {
	c := New(6, 5)
	c.Clear()
	c.PrintRect(0, 0, 6, 5, "supercalifragilistic", nil, nil, AlignLeft, BkgndNone)
	joined := strings.Join(rows(c), "")
	if joined != "supercalifragilistic" {
		t.Errorf("got %q", joined)
	}
}

// A break is allowed after a dash, keeping the dash on the first line.
func TestPrintRectBreaksAfterDash(t *testing.T) {
	c := New(6, 4)
	c.Clear()
	c.PrintRect(0, 0, 6, 4, "well-known", nil, nil, AlignLeft, BkgndNone)
	if got := rows(c); got[0] != "well-" || got[1] != "known" {
		t.Errorf("got %q / %q, want %q / %q", got[0], got[1], "well-", "known")
	}
}

func TestPrintRectExplicitNewlines(t *testing.T) {
	c := New(20, 5)
	c.Clear()
	h := c.PrintRect(0, 0, 20, 5, "one\ntwo\nthree", nil, nil, AlignLeft, BkgndNone)
	got := rows(c)
	for i, w := range []string{"one", "two", "three"} {
		if got[i] != w {
			t.Errorf("row %d = %q, want %q", i, got[i], w)
		}
	}
	if h != 3 {
		t.Errorf("height = %d, want 3", h)
	}
}

// Text below the rectangle is clipped, not drawn outside it.
func TestPrintRectClipsToHeight(t *testing.T) {
	c := New(6, 6)
	c.Clear()
	c.PrintRect(0, 0, 6, 2, "one two three four five", nil, nil, AlignLeft, BkgndNone)
	for i, r := range rows(c) {
		if i >= 2 && r != "" {
			t.Errorf("row %d outside the rect was drawn: %q", i, r)
		}
	}
}

func TestHeightRectMeasuresWithoutDrawing(t *testing.T) {
	c := New(20, 10)
	c.Clear()
	before := rows(c)
	h := c.HeightRect(0, 0, 20, 0, "the quick brown fox jumps over the lazy dog")
	if h != 3 {
		t.Errorf("HeightRect = %d, want 3", h)
	}
	for i, r := range rows(c) {
		if r != before[i] {
			t.Errorf("HeightRect drew to row %d: %q", i, r)
		}
	}
}

// Measuring IS bounded by the console height, matching C: its driver loop
// carries the same `top < console->h` condition, so a tall block measured
// against a short console reports what would fit, not the full layout.
// Verified against the C build: a 6x2 console measuring the text below
// returns 3 there too.
func TestHeightRectBoundedByConsole(t *testing.T) {
	c := New(6, 2)
	if got := c.HeightRect(0, 0, 6, 0, "one two three four five six seven"); got != 3 {
		t.Errorf("HeightRect = %d, want 3 (C returns 3 for this case)", got)
	}
	// Given room, the full wrapped height is reported.
	tall := New(6, 40)
	if got := tall.HeightRect(0, 0, 6, 0, "one two three four five six seven"); got < 6 {
		t.Errorf("HeightRect on a tall console = %d, want the full height", got)
	}
}

func TestPrintRectEmptyString(t *testing.T) {
	c := New(10, 3)
	c.Clear()
	if h := c.PrintRect(0, 0, 10, 3, "", nil, nil, AlignLeft, BkgndNone); h != 0 {
		t.Errorf("empty string height = %d, want 0", h)
	}
}

func TestPrintRectAppliesColors(t *testing.T) {
	fg := color.RGB{R: 200, G: 100, B: 50}
	bg := color.RGB{R: 10, G: 20, B: 30}
	c := New(10, 2)
	c.Clear()
	c.PrintRect(0, 0, 10, 2, "hi", &fg, &bg, AlignLeft, BkgndSet)
	if got := c.CharForeground(0, 0); got != fg {
		t.Errorf("fg = %v, want %v", got, fg)
	}
	if got := c.CharBackground(0, 0); got != bg {
		t.Errorf("bg = %v, want %v", got, bg)
	}
}

// Degenerate rectangles must not panic.
func TestPrintRectDegenerate(t *testing.T) {
	c := New(8, 4)
	for _, d := range [][4]int{{0, 0, 1, 1}, {7, 3, 1, 1}, {0, 0, -1, -1}, {20, 20, 4, 4}} {
		c.PrintRect(d[0], d[1], d[2], d[3], "hello world", nil, nil, AlignLeft, BkgndNone)
	}
	var nilCon *Console
	if h := nilCon.PrintRect(0, 0, 4, 4, "x", nil, nil, AlignLeft, BkgndNone); h != 0 {
		t.Errorf("nil console height = %d, want 0", h)
	}
}
