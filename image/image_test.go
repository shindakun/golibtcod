package image

import (
	"os"
	"path/filepath"
	"testing"

	"golibtcod/color"
	"golibtcod/console"
)

func TestNewAndPixels(t *testing.T) {
	img := New(8, 6)
	w, h := img.Size()
	if w != 8 || h != 6 {
		t.Fatalf("size = %dx%d", w, h)
	}
	if got := img.Pixel(0, 0); got != (color.RGB{}) {
		t.Fatalf("new image should be black, got %+v", got)
	}
	img.PutPixel(3, 2, color.Red)
	if got := img.Pixel(3, 2); got != color.Red {
		t.Fatalf("put/get mismatch: %+v", got)
	}
	// out of bounds reads return black and writes are ignored, as in C
	if got := img.Pixel(-1, 0); got != (color.RGB{}) {
		t.Fatal("out-of-bounds read should be black")
	}
	img.PutPixel(100, 100, color.Red) // must not panic
}

func TestMipmapLevels(t *testing.T) {
	// levels halve until a dimension hits zero
	if got := mipmapLevels(8, 8); got != 4 { // 8,4,2,1
		t.Errorf("mipmapLevels(8,8) = %d, want 4", got)
	}
	if got := mipmapLevels(1, 1); got != 1 {
		t.Errorf("mipmapLevels(1,1) = %d, want 1", got)
	}
	if got := mipmapLevels(16, 4); got != 3 { // limited by the smaller side
		t.Errorf("mipmapLevels(16,4) = %d, want 3", got)
	}
}

func TestMipmapAveraging(t *testing.T) {
	// A per-pixel black/white checker: every 2x2 block averages to mid grey,
	// so mipmap level 1 must be uniformly grey.
	img := New(4, 4)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if (x+y)%2 == 0 {
				img.PutPixel(x, y, color.White)
			} else {
				img.PutPixel(x, y, color.Black)
			}
		}
	}
	img.generateMip(1)
	for i, c := range img.mipmaps[1].buf {
		if c.R < 120 || c.R > 135 {
			t.Fatalf("mip level 1 texel %d = %+v, want ~127 grey", i, c)
		}
	}

	// MipmapPixel picks a level from the requested texel footprint, then
	// steps back one; C does this deliberately (`if mip > 0 { mip-- }`),
	// trading sharpness for aliasing. A 4-pixel footprint therefore lands
	// on level 1, not level 2.
	if got := img.MipmapPixel(0, 0, 4, 4); got.R < 120 || got.R > 135 {
		t.Fatalf("MipmapPixel over a 4px footprint = %+v, want grey", got)
	}
	// A 1-pixel footprint stays at level 0: the original pixel, not a blend.
	if got := img.MipmapPixel(0, 0, 1, 1); got != color.White {
		t.Fatalf("MipmapPixel over a 1px footprint = %+v, want the exact pixel", got)
	}
}

func TestMipmapInvalidatedOnWrite(t *testing.T) {
	img := New(4, 4)
	img.Clear(color.White)
	_ = img.MipmapPixel(0, 0, 4, 4) // force generation
	img.Clear(color.Black)
	if got := img.MipmapPixel(0, 0, 4, 4); got.R > 10 {
		t.Fatalf("stale mipmap after write: %+v", got)
	}
}

func TestScaleDownAveragesAndUpNearest(t *testing.T) {
	img := New(4, 4)
	img.Clear(color.RGB{R: 200, G: 100, B: 50})
	img.Scale(2, 2)
	w, h := img.Size()
	if w != 2 || h != 2 {
		t.Fatalf("scaled size = %dx%d", w, h)
	}
	if got := img.Pixel(0, 0); got.R < 195 || got.R > 205 {
		t.Fatalf("uniform image should survive downscale: %+v", got)
	}
	img.Scale(6, 6)
	if w, h = img.Size(); w != 6 || h != 6 {
		t.Fatalf("upscaled size = %dx%d", w, h)
	}
}

func TestFlipsAndRotate(t *testing.T) {
	img := New(3, 2)
	img.PutPixel(0, 0, color.Red)
	img.HFlip()
	if img.Pixel(2, 0) != color.Red {
		t.Error("hflip did not move the pixel to the right edge")
	}
	img.HFlip()
	img.VFlip()
	if img.Pixel(0, 1) != color.Red {
		t.Error("vflip did not move the pixel to the bottom")
	}
	img.VFlip()

	img.Rotate90(1)
	if w, h := img.Size(); w != 2 || h != 3 {
		t.Fatalf("rotate90 should transpose dimensions, got %dx%d", w, h)
	}
	img.Rotate90(3) // back to start
	if w, h := img.Size(); w != 3 || h != 2 {
		t.Fatalf("four quarter turns should restore, got %dx%d", w, h)
	}
	if img.Pixel(0, 0) != color.Red {
		t.Error("four quarter turns should restore the pixel")
	}
}

func TestKeyColorSkipsBlit(t *testing.T) {
	img := New(2, 2)
	img.Clear(color.RGB{R: 255, G: 0, B: 255})
	img.SetKeyColor(color.RGB{R: 255, G: 0, B: 255})
	if !img.IsPixelTransparent(0, 0) {
		t.Fatal("key-coloured pixel should be transparent")
	}
	c := console.New(4, 4)
	c.SetDefaultBackground(color.Blue)
	c.Clear()
	img.BlitRect(c, 0, 0, 2, 2, console.BkgndSet)
	if got := c.CharBackground(0, 0); got != color.Blue {
		t.Fatalf("key colour was blitted anyway: %+v", got)
	}
}

func TestBlitRectFillsConsole(t *testing.T) {
	img := New(4, 4)
	img.Clear(color.Red)
	c := console.New(8, 8)
	img.BlitRect(c, 2, 2, 4, 4, console.BkgndSet)
	if got := c.CharBackground(3, 3); got != color.Red {
		t.Fatalf("blit did not land: %+v", got)
	}
	if got := c.CharBackground(0, 0); got == color.Red {
		t.Fatal("blit leaked outside its rect")
	}
}

// Subcell rendering is the reason this module is worth having: a solid
// colour must produce a space, and a two-colour split must pick a block
// glyph with the right fg/bg pair.
func TestSubcellQuadrants(t *testing.T) {
	solid := generateQuadrantGraphic([4]color.RGB{color.Red, color.Red, color.Red, color.Red})
	if solid.Ch != ' ' {
		t.Errorf("uniform quadrants should be a space, got %U", rune(solid.Ch))
	}
	if solid.Bg.R != 255 {
		t.Errorf("uniform quadrant colour lost: %+v", solid.Bg)
	}

	// top half black, bottom half white -> upper-half block with swapped pair
	half := generateQuadrantGraphic([4]color.RGB{
		color.Black, color.Black, color.White, color.White,
	})
	if half.Ch != 0x2580 {
		t.Errorf("expected upper half block U+2580, got %U", rune(half.Ch))
	}
	if half.Fg == half.Bg {
		t.Error("two-colour cell must have distinct fg/bg")
	}
}

func TestBlit2xDoublesResolution(t *testing.T) {
	img := New(4, 4)
	img.Clear(color.Black)
	for x := 0; x < 4; x++ { // top two rows white
		img.PutPixel(x, 0, color.White)
		img.PutPixel(x, 1, color.White)
	}
	c := console.New(2, 2)
	img.Blit2x(c, 0, 0, 0, 0, -1, -1)
	// the first console row covers image rows 0-1: all white, so a space
	if c.Char(0, 0) != ' ' {
		t.Errorf("uniform top row should be a space, got %U", rune(c.Char(0, 0)))
	}
	if c.CharBackground(0, 0).R != 255 {
		t.Errorf("top row should be white, got %+v", c.CharBackground(0, 0))
	}
	if c.CharBackground(0, 1).R != 0 {
		t.Errorf("bottom row should be black, got %+v", c.CharBackground(0, 1))
	}
}

func TestFromConsoleRoundTrip(t *testing.T) {
	c := console.New(3, 3)
	c.SetDefaultBackground(color.RGB{R: 10, G: 200, B: 30})
	c.Clear()
	img := FromConsole(c)
	if w, h := img.Size(); w != 6 || h != 6 {
		t.Fatalf("image from console should be 2x: %dx%d", w, h)
	}
	if got := img.Pixel(1, 1); got.G != 200 {
		t.Fatalf("console background not captured: %+v", got)
	}
}

func TestSaveLoadPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")

	img := New(5, 4)
	img.Clear(color.RGB{R: 12, G: 34, B: 56})
	img.PutPixel(2, 1, color.RGB{R: 200, G: 100, B: 0})
	if err := img.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	w, h := loaded.Size()
	if w != 5 || h != 4 {
		t.Fatalf("round-trip size = %dx%d", w, h)
	}
	if got := loaded.Pixel(2, 1); got != (color.RGB{R: 200, G: 100, B: 0}) {
		t.Fatalf("round-trip pixel = %+v", got)
	}
	if got := loaded.Pixel(0, 0); got != (color.RGB{R: 12, G: 34, B: 56}) {
		t.Fatalf("round-trip background = %+v", got)
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.png")
	os.WriteFile(path, []byte("not a png"), 0o644)
	if _, err := Load(path); err == nil {
		t.Fatal("expected an error loading garbage")
	}
}
