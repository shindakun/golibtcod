package pngout

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shindakun/golibtcod/color"
	"github.com/shindakun/golibtcod/console"
)

func renderOne(t *testing.T, ch int, name string) []byte {
	t.Helper()
	c := console.New(1, 1)
	c.PutCharEx(0, 0, ch, color.White, color.Black)
	p := filepath.Join(t.TempDir(), name+".png")
	if err := Render(c, p, Options{Scale: 1}); err != nil {
		t.Fatalf("render %q: %v", name, err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The missing-glyph marker used to be '#', which is also what callers draw
// walls with, so an unmapped codepoint was indistinguishable from real
// content. cmd/sample draws its A* route with '*', which had no glyph.
func TestMissingGlyphIsDistinctFromRealContent(t *testing.T) {
	star := renderOne(t, '*', "star")
	hash := renderOne(t, '#', "hash")
	unmapped := renderOne(t, 0x2620, "unmapped")

	if bytes.Equal(star, hash) {
		t.Error("'*' renders identically to '#'")
	}
	if bytes.Equal(unmapped, hash) {
		t.Error("the missing-glyph marker renders identically to '#'")
	}
	if bytes.Equal(unmapped, star) {
		t.Error("the missing-glyph marker renders identically to '*'")
	}
}

// Ordinary text must render as itself rather than as the marker.
func TestPrintableAsciiHasGlyphs(t *testing.T) {
	marker := renderOne(t, 0x2620, "marker")
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		if _, ok := glyphs[r]; !ok {
			t.Errorf("no glyph for %q", r)
			continue
		}
		if bytes.Equal(renderOne(t, int(r), "g"+string(r)), marker) {
			t.Errorf("%q renders as the missing-glyph marker", r)
		}
	}
}

func TestRenderWritesPng(t *testing.T) {
	c := console.New(8, 3)
	c.Print(0, 0, "hello world")
	p := filepath.Join(t.TempDir(), "out.png")
	if err := Render(c, p, Options{Scale: 2, Grain: 0.1, Vignette: 0.2, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b[:8]), "\x89PNG") {
		t.Error("output is not a PNG")
	}
	if len(b) == 0 {
		t.Error("empty output")
	}
}

// Scale < 1 is normalized to 1 rather than producing a zero-sized image.
func TestRenderScaleNormalized(t *testing.T) {
	c := console.New(2, 2)
	p := filepath.Join(t.TempDir(), "s.png")
	if err := Render(c, p, Options{Scale: 0}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Size() == 0 {
		t.Errorf("expected a non-empty PNG, err=%v", err)
	}
}
