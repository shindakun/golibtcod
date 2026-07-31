// Package pngout is a software Presenter for golibtcod: renders a Console
// to a PNG with an embedded hand-authored 8x8 string-art font. It proves
// the presenter contract with zero third-party dependencies, and makes
// headless rendering (batch worldgen, CI, replay) possible.
package pngout

import (
	"image"
	col "image/color"
	"image/png"
	"math/rand"
	"os"

	"github.com/shindakun/golibtcod/color"
	"github.com/shindakun/golibtcod/console"
	"github.com/shindakun/golibtcod/tileset"
)

const CellPx = 8 // glyph cell is 8x8 source pixels

// missingGlyph keys the box drawn for a codepoint the font has no art for.
// It is a private sentinel rather than a printable rune so the marker can
// never collide with real content: '#' was previously used, which is also
// what callers draw walls with, making unmapped glyphs invisible as bugs.
const missingGlyph = rune(-1)

// glyphs: 8 rows of 8 chars; non-space = foreground pixel.
var glyphs = map[rune][8]string{
	' ':  {},
	'A':  {" XXX    ", "X   X   ", "X   X   ", "XXXXX   ", "X   X   ", "X   X   ", "X   X   ", ""},
	'B':  {"XXXX    ", "X   X   ", "X   X   ", "XXXX    ", "X   X   ", "X   X   ", "XXXX    ", ""},
	'C':  {" XXX    ", "X   X   ", "X       ", "X       ", "X       ", "X   X   ", " XXX    ", ""},
	'D':  {"XXXX    ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", "XXXX    ", ""},
	'E':  {"XXXXX   ", "X       ", "X       ", "XXXX    ", "X       ", "X       ", "XXXXX   ", ""},
	'F':  {"XXXXX   ", "X       ", "X       ", "XXXX    ", "X       ", "X       ", "X       ", ""},
	'G':  {" XXX    ", "X   X   ", "X       ", "X  XX   ", "X   X   ", "X   X   ", " XXX    ", ""},
	'H':  {"X   X   ", "X   X   ", "X   X   ", "XXXXX   ", "X   X   ", "X   X   ", "X   X   ", ""},
	'I':  {"XXXXX   ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "XXXXX   ", ""},
	'J':  {"XXXXX   ", "   X    ", "   X    ", "   X    ", "   X    ", "X  X    ", " XX     ", ""},
	'K':  {"X   X   ", "X  X    ", "X X     ", "XX      ", "X X     ", "X  X    ", "X   X   ", ""},
	'L':  {"X       ", "X       ", "X       ", "X       ", "X       ", "X       ", "XXXXX   ", ""},
	'M':  {"X   X   ", "XX XX   ", "X X X   ", "X X X   ", "X   X   ", "X   X   ", "X   X   ", ""},
	'N':  {"X   X   ", "XX  X   ", "XX  X   ", "X X X   ", "X  XX   ", "X  XX   ", "X   X   ", ""},
	'O':  {" XXX    ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", " XXX    ", ""},
	'P':  {"XXXX    ", "X   X   ", "X   X   ", "XXXX    ", "X       ", "X       ", "X       ", ""},
	'Q':  {" XXX    ", "X   X   ", "X   X   ", "X   X   ", "X X X   ", "X  X    ", " XX X   ", ""},
	'R':  {"XXXX    ", "X   X   ", "X   X   ", "XXXX    ", "X X     ", "X  X    ", "X   X   ", ""},
	'S':  {" XXXX   ", "X       ", "X       ", " XXX    ", "    X   ", "    X   ", "XXXX    ", ""},
	'T':  {"XXXXX   ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", ""},
	'U':  {"X   X   ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", " XXX    ", ""},
	'V':  {"X   X   ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", " X X    ", "  X     ", ""},
	'W':  {"X   X   ", "X   X   ", "X   X   ", "X X X   ", "X X X   ", "XX XX   ", "X   X   ", ""},
	'X':  {"X   X   ", "X   X   ", " X X    ", "  X     ", " X X    ", "X   X   ", "X   X   ", ""},
	'Y':  {"X   X   ", "X   X   ", " X X    ", "  X     ", "  X     ", "  X     ", "  X     ", ""},
	'Z':  {"XXXXX   ", "    X   ", "   X    ", "  X     ", " X      ", "X       ", "XXXXX   ", ""},
	'0':  {" XXX    ", "X   X   ", "X  XX   ", "X X X   ", "XX  X   ", "X   X   ", " XXX    ", ""},
	'1':  {"  X     ", " XX     ", "  X     ", "  X     ", "  X     ", "  X     ", "XXXXX   ", ""},
	'2':  {" XXX    ", "X   X   ", "    X   ", "   X    ", "  X     ", " X      ", "XXXXX   ", ""},
	'3':  {"XXXX    ", "    X   ", "    X   ", " XXX    ", "    X   ", "    X   ", "XXXX    ", ""},
	'4':  {"X   X   ", "X   X   ", "X   X   ", "XXXXX   ", "    X   ", "    X   ", "    X   ", ""},
	'5':  {"XXXXX   ", "X       ", "XXXX    ", "    X   ", "    X   ", "X   X   ", " XXX    ", ""},
	'6':  {" XXX    ", "X       ", "XXXX    ", "X   X   ", "X   X   ", "X   X   ", " XXX    ", ""},
	'7':  {"XXXXX   ", "    X   ", "   X    ", "   X    ", "  X     ", "  X     ", "  X     ", ""},
	'8':  {" XXX    ", "X   X   ", "X   X   ", " XXX    ", "X   X   ", "X   X   ", " XXX    ", ""},
	'9':  {" XXX    ", "X   X   ", "X   X   ", " XXXX   ", "    X   ", "    X   ", " XXX    ", ""},
	'.':  {"", "", "", "", "", " XX     ", " XX     ", ""},
	',':  {"", "", "", "", "", " XX     ", " XX     ", " X      "},
	'\'': {" X      ", " X      ", "", "", "", "", "", ""},
	':':  {"", " XX     ", " XX     ", "", " XX     ", " XX     ", "", ""},
	'-':  {"", "", "", "XXXX    ", "", "", "", ""},
	'/':  {"     X  ", "    X   ", "   X    ", "  X     ", " X      ", "X       ", "X       ", ""},
	'%':  {"XX   X  ", "XX  X   ", "   X    ", "  X     ", " X      ", "X   XX  ", "    XX  ", ""},
	'[':  {" XXX    ", " X      ", " X      ", " X      ", " X      ", " X      ", " XXX    ", ""},
	']':  {" XXX    ", "   X    ", "   X    ", "   X    ", "   X    ", "   X    ", " XXX    ", ""},
	'>':  {"X       ", " X      ", "  X     ", "   X    ", "  X     ", " X      ", "X       ", ""},
	'<':  {"   X    ", "  X     ", " X      ", "X       ", " X      ", "  X     ", "   X    ", ""},
	'@':  {" XXXX   ", "X    X  ", "X XX X  ", "X X XX  ", "X XXX   ", "X       ", " XXXX   ", ""},
	'd':  {"    X   ", "    X   ", " XXXX   ", "X   X   ", "X   X   ", "X   X   ", " XXXX   ", ""},
	'z':  {"", "", "XXXXX   ", "   X    ", "  X     ", " X      ", "XXXXX   ", ""},
	'#':  {" X X    ", "XXXXX   ", " X X    ", " X X    ", " X X    ", "XXXXX   ", " X X    ", ""},
	'*':  {"", " X X X  ", "  XXX   ", "XXXXXXX ", "  XXX   ", " X X X  ", "", ""},
	'+':  {"", "  X     ", "  X     ", "XXXXX   ", "  X     ", "  X     ", "", ""},
	'!':  {"  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "", "  X     ", ""},
	'?':  {" XXX    ", "X   X   ", "    X   ", "   X    ", "  X     ", "", "  X     ", ""},
	'"':  {" X X    ", " X X    ", "", "", "", "", "", ""},
	';':  {"", " XX     ", " XX     ", "", " XX     ", " XX     ", " X      ", ""},
	'(':  {"   X    ", "  X     ", " X      ", " X      ", " X      ", "  X     ", "   X    ", ""},
	')':  {" X      ", "  X     ", "   X    ", "   X    ", "   X    ", "  X     ", " X      ", ""},
	'=':  {"", "", "XXXXX   ", "", "XXXXX   ", "", "", ""},
	'_':  {"", "", "", "", "", "", "", "XXXXX   "},
	'$':  {"  X     ", " XXXX   ", "X X     ", " XXX    ", "  X X   ", "XXXX    ", "  X     ", ""},
	'&':  {" XX     ", "X  X    ", "X X     ", " X      ", "X X X   ", "X  X    ", " XX X   ", ""},
	'\\': {"X       ", "X       ", " X      ", "  X     ", "   X    ", "    X   ", "     X  ", ""},
	'^':  {"  X     ", " X X    ", "X   X   ", "", "", "", "", ""},
	'`':  {" X      ", "  X     ", "", "", "", "", "", ""},
	'{':  {"   XX   ", "  X     ", "  X     ", " X      ", "  X     ", "  X     ", "   XX   ", ""},
	'|':  {"  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     "},
	'}':  {" XX     ", "   X    ", "   X    ", "    X   ", "   X    ", "   X    ", " XX     ", ""},
	// lowercase
	'a': {"", "", " XXX    ", "    X   ", " XXXX   ", "X   X   ", " XXXX   ", ""},
	'b': {"X       ", "X       ", "XXXX    ", "X   X   ", "X   X   ", "X   X   ", "XXXX    ", ""},
	'c': {"", "", " XXX    ", "X       ", "X       ", "X       ", " XXX    ", ""},
	'e': {"", "", " XXX    ", "X   X   ", "XXXXX   ", "X       ", " XXX    ", ""},
	'f': {"  XX    ", " X      ", "XXXX    ", " X      ", " X      ", " X      ", " X      ", ""},
	'g': {"", "", " XXXX   ", "X   X   ", " XXXX   ", "    X   ", " XXX    ", ""},
	'h': {"X       ", "X       ", "XXXX    ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", ""},
	'i': {"  X     ", "        ", " XX     ", "  X     ", "  X     ", "  X     ", " XXX    ", ""},
	'j': {"   X    ", "        ", "  XX    ", "   X    ", "   X    ", "X  X    ", " XX     ", ""},
	'k': {"X       ", "X       ", "X  X    ", "X X     ", "XX      ", "X X     ", "X  X    ", ""},
	'l': {" XX     ", "  X     ", "  X     ", "  X     ", "  X     ", "  X     ", " XXX    ", ""},
	'm': {"", "", "XX X    ", "X X X   ", "X X X   ", "X X X   ", "X X X   ", ""},
	'n': {"", "", "XXXX    ", "X   X   ", "X   X   ", "X   X   ", "X   X   ", ""},
	'o': {"", "", " XXX    ", "X   X   ", "X   X   ", "X   X   ", " XXX    ", ""},
	'p': {"", "", "XXXX    ", "X   X   ", "XXXX    ", "X       ", "X       ", ""},
	'q': {"", "", " XXXX   ", "X   X   ", " XXXX   ", "    X   ", "    X   ", ""},
	'r': {"", "", "X XX    ", "XX  X   ", "X       ", "X       ", "X       ", ""},
	's': {"", "", " XXXX   ", "X       ", " XXX    ", "    X   ", "XXXX    ", ""},
	't': {" X      ", " X      ", "XXXX    ", " X      ", " X      ", " X  X   ", "  XX    ", ""},
	'u': {"", "", "X   X   ", "X   X   ", "X   X   ", "X   X   ", " XXXX   ", ""},
	'v': {"", "", "X   X   ", "X   X   ", "X   X   ", " X X    ", "  X     ", ""},
	'w': {"", "", "X X X   ", "X X X   ", "X X X   ", "X X X   ", " X X    ", ""},
	'x': {"", "", "X   X   ", " X X    ", "  X     ", " X X    ", "X   X   ", ""},
	'y': {"", "", "X   X   ", "X   X   ", " XXXX   ", "    X   ", " XXX    ", ""},
	// missing-glyph marker: deliberately NOT '#', which is a real wall tile
	missingGlyph: {"XXXXXXXX", "X      X", "X      X", "X      X", "X      X", "X      X", "X      X", "XXXXXXXX"},
	// terrain & UI specials
	'─': {"", "", "", "XXXXXXXX", "XXXXXXXX", "", "", ""},
	'│': {"   XX   ", "   XX   ", "   XX   ", "   XX   ", "   XX   ", "   XX   ", "   XX   ", "   XX   "},
	'┌': {"", "", "", "   XXXXX", "   XXXXX", "   XX   ", "   XX   ", "   XX   "},
	'┐': {"", "", "", "XXXXX   ", "XXXXX   ", "   XX   ", "   XX   ", "   XX   "},
	'└': {"   XX   ", "   XX   ", "   XX   ", "   XXXXX", "   XXXXX", "", "", ""},
	'┘': {"   XX   ", "   XX   ", "   XX   ", "XXXXX   ", "XXXXX   ", "", "", ""},
	'├': {"   XX   ", "   XX   ", "   XX   ", "   XXXXX", "   XXXXX", "   XX   ", "   XX   ", "   XX   "},
	'┤': {"   XX   ", "   XX   ", "   XX   ", "XXXXX   ", "XXXXX   ", "   XX   ", "   XX   ", "   XX   "},
	'┬': {"", "", "", "XXXXXXXX", "XXXXXXXX", "   XX   ", "   XX   ", "   XX   "},
	'┴': {"   XX   ", "   XX   ", "   XX   ", "XXXXXXXX", "XXXXXXXX", "", "", ""},
	'┼': {"   XX   ", "   XX   ", "   XX   ", "XXXXXXXX", "XXXXXXXX", "   XX   ", "   XX   ", "   XX   "},
	'█': {"XXXXXXXX", "XXXXXXXX", "XXXXXXXX", "XXXXXXXX", "XXXXXXXX", "XXXXXXXX", "XXXXXXXX", "XXXXXXXX"},
	'▓': {"XX XX XX", "X XX XX ", "XX XX XX", "X XX XX ", "XX XX XX", "X XX XX ", "XX XX XX", "X XX XX "},
	'░': {"X   X   ", "        ", "  X   X ", "        ", "X   X   ", "        ", "  X   X ", "        "},
	'≈': {"", " XX   X ", "X  XXX  ", "", " XX   X ", "X  XXX  ", "", ""},
	'♠': {"   X    ", "  XXX   ", " XXXXX  ", "XXXXXXX ", "XXXXXXX ", "  XXX   ", "   X    ", "  XXX   "},
	'†': {"   X    ", "   X    ", " XXXXX  ", "   X    ", "   X    ", "   X    ", "   X    ", "   X    "},
	'☼': {"   X    ", " X X X  ", "  XXX   ", "XXX XXX ", "  XXX   ", " X X X  ", "   X    ", ""},
	'∙': {"", "", "", "   XX   ", "   XX   ", "", "", ""},
	'~': {"", "", "", " XX  X  ", "X  XX   ", "", "", ""},
}

// Options for the lo-fi post pass: a software approximation of film grain
// and corner darkening.
type Options struct {
	Scale    int     // integer upscale of the glyph cells
	Grain    float64 // 0..1 film-grain amplitude
	Vignette float64 // 0..1 corner darkening
	Seed     int64

	// Tileset supplies the glyph bitmaps. Nil uses the built-in 8x8 font,
	// which keeps the zero-configuration path working; a loaded BDF font
	// (see the tileset package) replaces it and sets the cell size.
	Tileset *tileset.Tileset
}

// cellSize reports the glyph cell dimensions for these options.
func (o Options) cellSize() (w, h int) {
	if o.Tileset != nil {
		return o.Tileset.TileWidth, o.Tileset.TileHeight
	}
	return CellPx, CellPx
}

// coverage reports, for one glyph cell, which pixels are foreground. The
// returned slice is cw*ch long in row-major order.
func (o Options) coverage(ch int, cw, chh int) []bool {
	out := make([]bool, cw*chh)
	if o.Tileset != nil {
		cov := o.Tileset.Coverage(ch)
		if cov == nil {
			// No glyph for this codepoint: draw a hollow box, the same
			// marker the built-in font uses, so a missing glyph stays
			// visibly distinct from real content.
			for y := 0; y < chh; y++ {
				for x := 0; x < cw; x++ {
					if x == 0 || y == 0 || x == cw-1 || y == chh-1 {
						out[y*cw+x] = true
					}
				}
			}
			return out
		}
		for i := range out {
			if i < len(cov) {
				out[i] = cov[i] >= 128
			}
		}
		return out
	}

	g, ok := glyphs[rune(ch)]
	if !ok {
		g = glyphs[missingGlyph]
	}
	for y := 0; y < chh; y++ {
		row := ""
		if y < len(g) {
			row = g[y]
		}
		for x := 0; x < cw; x++ {
			out[y*cw+x] = x < len(row) && row[x] != ' '
		}
	}
	return out
}

// Render draws the console and writes a PNG. This is the whole presenter.
func Render(c *console.Console, path string, o Options) (err error) {
	if o.Scale < 1 {
		o.Scale = 1
	}
	cw, ch := o.cellSize()
	w, h := c.W*cw*o.Scale, c.H*ch*o.Scale
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(o.Seed))

	for cy := 0; cy < c.H; cy++ {
		for cx := 0; cx < c.W; cx++ {
			cell := c.Tiles[cy*c.W+cx]
			on := o.coverage(cell.Ch, cw, ch)
			for py := 0; py < ch; py++ {
				for px := 0; px < cw; px++ {
					col2 := color.RGB{R: cell.Bg.R, G: cell.Bg.G, B: cell.Bg.B}
					if on[py*cw+px] {
						col2 = color.RGB{R: cell.Fg.R, G: cell.Fg.G, B: cell.Fg.B}
					}
					fill(img, (cx*cw+px)*o.Scale, (cy*ch+py)*o.Scale, o.Scale, col2)
				}
			}
		}
	}

	post(img, w, h, o, rng)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// Checked rather than deferred: a failed close on a write path means a
	// truncated PNG, which must not be reported as success.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return png.Encode(f, img)
}

func fill(img *image.RGBA, x, y, s int, c color.RGB) {
	for dy := 0; dy < s; dy++ {
		for dx := 0; dx < s; dx++ {
			img.SetRGBA(x+dx, y+dy, col.RGBA{c.R, c.G, c.B, 255})
		}
	}
}

// post applies grain + vignette: the "photocopied demo cover" pass.
func post(img *image.RGBA, w, h int, o Options, rng *rand.Rand) {
	cxf, cyf := float64(w)/2, float64(h)/2
	maxd := cxf*cxf + cyf*cyf
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := img.RGBAAt(x, y)
			f := 1.0
			if o.Vignette > 0 {
				dx, dy := float64(x)-cxf, float64(y)-cyf
				f -= o.Vignette * (dx*dx + dy*dy) / maxd
			}
			if o.Grain > 0 {
				f += (rng.Float64() - 0.5) * o.Grain
			}
			p.R = clamp(float64(p.R) * f)
			p.G = clamp(float64(p.G) * f)
			p.B = clamp(float64(p.B) * f)
			img.SetRGBA(x, y, p)
		}
	}
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
