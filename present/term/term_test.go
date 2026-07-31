package term

import (
	"strings"
	"testing"

	"github.com/shindakun/golibtcod/color"
	"github.com/shindakun/golibtcod/console"
)

func TestRenderEmitsTruecolor(t *testing.T) {
	c := console.New(2, 1)
	c.PutCharEx(0, 0, '@', color.White, color.Black)
	c.PutCharEx(1, 0, '#', color.Red, color.Blue)
	var b strings.Builder
	if err := Render(c, &b, Options{}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "\x1b[38;2;255;255;255;48;2;0;0;0m@") {
		t.Fatalf("missing white-on-black cell: %q", out)
	}
	if !strings.Contains(out, "\x1b[38;2;255;0;0;48;2;0;0;255m#") {
		t.Fatalf("missing red-on-blue cell: %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected 1 line, got %q", out)
	}
}

func TestRunLengthCollapse(t *testing.T) {
	c := console.New(10, 1) // all cells identical after Clear
	var b strings.Builder
	if err := Render(c, &b, Options{}); err != nil {
		t.Fatal(err)
	}
	// one SGR set for the row plus the reset
	if n := strings.Count(b.String(), "38;2;"); n != 1 {
		t.Fatalf("expected 1 SGR run for a uniform row, got %d", n)
	}
}
