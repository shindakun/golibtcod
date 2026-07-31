package rexpaint

import (
	"bytes"
	"path/filepath"
	"testing"

	"golibtcod/color"
	"golibtcod/console"
)

func sample(w, h int) *console.Console {
	c := console.New(w, h)
	c.SetDefaultForeground(color.White)
	c.SetDefaultBackground(color.Black)
	c.Clear()
	c.PutCharEx(0, 0, '@', color.White, color.RGB{R: 20, G: 20, B: 40})
	c.PutCharEx(w-1, h-1, '#', color.Red, color.Blue)
	c.PutCharEx(1, 2, '≈', color.Cyan, color.Black)
	return c
}

func TestRoundTripSingleLayer(t *testing.T) {
	src := sample(7, 5)
	var buf bytes.Buffer
	if err := Write(&buf, src); err != nil {
		t.Fatal(err)
	}
	layers, err := ReadLayers(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 {
		t.Fatalf("got %d layers, want 1", len(layers))
	}
	got := layers[0]
	if got.W != src.W || got.H != src.H {
		t.Fatalf("size %dx%d != %dx%d", got.W, got.H, src.W, src.H)
	}
	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			a, b := src.Tiles[y*src.W+x], got.Tiles[y*got.W+x]
			if a.Ch != b.Ch {
				t.Fatalf("(%d,%d) glyph %U != %U", x, y, rune(a.Ch), rune(b.Ch))
			}
			if a.Fg != b.Fg || a.Bg != b.Bg {
				t.Fatalf("(%d,%d) colour %+v/%+v != %+v/%+v", x, y, a.Fg, a.Bg, b.Fg, b.Bg)
			}
		}
	}
}

// Non-square consoles are the classic way to get column/row major wrong:
// a transposed writer round-trips cleanly on a square and corrupts here.
func TestRoundTripNonSquare(t *testing.T) {
	src := sample(11, 3)
	var buf bytes.Buffer
	if err := Write(&buf, src); err != nil {
		t.Fatal(err)
	}
	layers, err := ReadLayers(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got := layers[0]
	if got.W != 11 || got.H != 3 {
		t.Fatalf("dimensions transposed: %dx%d", got.W, got.H)
	}
	if got.Char(10, 2) != '#' {
		t.Fatalf("far corner glyph = %U, want '#'", rune(got.Char(10, 2)))
	}
	if got.Char(1, 2) != '≈' {
		t.Fatalf("(1,2) = %U, want '≈'", rune(got.Char(1, 2)))
	}
}

func TestMultiLayerAndCombine(t *testing.T) {
	base := console.New(4, 4)
	base.SetDefaultBackground(color.Blue)
	base.Clear()

	overlay := console.New(4, 4)
	overlay.SetDefaultBackground(KeyColor) // fuchsia = transparent
	overlay.Clear()
	overlay.PutCharEx(2, 2, '@', color.White, color.Black)

	var buf bytes.Buffer
	if err := Write(&buf, base, overlay); err != nil {
		t.Fatal(err)
	}
	layers, err := ReadLayers(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("got %d layers, want 2", len(layers))
	}

	flat := Combine(layers)
	if flat.Char(2, 2) != '@' {
		t.Errorf("overlay glyph lost in combine: %U", rune(flat.Char(2, 2)))
	}
	// where the overlay was fuchsia, the base must show through
	if got := flat.CharBackground(0, 0); got != color.Blue {
		t.Errorf("key colour was not treated as transparent: %+v", got)
	}
}

func TestFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "map.xp")
	src := sample(6, 6)
	if err := Save(path, src); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Char(0, 0) != '@' {
		t.Fatalf("glyph lost through file: %U", rune(got.Char(0, 0)))
	}
}

func TestRejectsGarbage(t *testing.T) {
	if _, err := ReadLayers(bytes.NewReader([]byte("not gzip"))); err == nil {
		t.Fatal("expected an error on non-gzip input")
	}
	// valid gzip, nonsense contents
	var buf bytes.Buffer
	if err := Write(&buf, console.New(2, 2)); err != nil {
		t.Fatal(err)
	}
	corrupt := buf.Bytes()[:len(buf.Bytes())/2]
	if _, err := ReadLayers(bytes.NewReader(corrupt)); err == nil {
		t.Fatal("expected an error on truncated input")
	}
}
