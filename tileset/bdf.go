// BDF font loading, a faithful Go port of libtcod's tileset_bdf.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
//
// BDF (Glyph Bitmap Distribution Format) is plain text: a FONTBOUNDINGBOX
// header giving the cell size, then one STARTCHAR block per glyph carrying
// its own bounding box and hex-encoded bitmap rows. The C loader needs only
// libc, so this port needs only the standard library.
package tileset

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golibtcod/color"
)

type bbox struct {
	width, height, xOffset, yOffset int
}

type bdfLoader struct {
	lines []string
	pos   int // index of the current line
	bbox  bbox
	ts    *Tileset
}

// line returns the current line, or "" past the end.
func (l *bdfLoader) line() string {
	if l.pos < 0 || l.pos >= len(l.lines) {
		return ""
	}
	return l.lines[l.pos]
}

func (l *bdfLoader) next() bool {
	l.pos++
	return l.pos < len(l.lines)
}

// keyword reports whether the current line starts with kw, matching C's
// check_keyword (which compares the leading token).
func (l *bdfLoader) keyword(kw string) bool {
	f := strings.Fields(l.line())
	if len(f) == 0 {
		return kw == ""
	}
	return f[0] == kw
}

// args returns the whitespace-separated arguments after the keyword.
func (l *bdfLoader) args() []string {
	f := strings.Fields(l.line())
	if len(f) <= 1 {
		return nil
	}
	return f[1:]
}

// argInt reads the n-th argument as an int, as C's read_next_int does
// (missing or malformed values become 0 there via strtol).
func (l *bdfLoader) argInt(n int) int {
	a := l.args()
	if n >= len(a) {
		return 0
	}
	v, err := strconv.Atoi(a[n])
	if err != nil {
		return 0
	}
	return v
}

// LoadBDF is TCOD_load_bdf: read a BDF font file into a Tileset.
func LoadBDF(path string) (*Tileset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ts, err := ReadBDF(data)
	if err != nil {
		return nil, fmt.Errorf("tileset: %s: %w", path, err)
	}
	return ts, nil
}

// ReadBDFReader loads a BDF font from any reader.
func ReadBDFReader(r io.Reader) (*Tileset, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ReadBDF(data)
}

// ReadBDF is TCOD_load_bdf_memory: parse BDF data into a Tileset.
func ReadBDF(data []byte) (*Tileset, error) {
	// Normalize CRLF and CR so line handling matches C's newline scanning.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	l := &bdfLoader{lines: strings.Split(text, "\n")}
	if err := l.parse(); err != nil {
		return nil, err
	}
	return l.ts, nil
}

// parse is the C parse_bdf: the top-level header scan.
func (l *bdfLoader) parse() error {
	if !l.keyword("STARTFONT") {
		return fmt.Errorf("BDF data must begin with STARTFONT (line 1)")
	}
	for l.next() {
		switch {
		case l.keyword("FONTBOUNDINGBOX"):
			if l.ts != nil {
				return fmt.Errorf("duplicate FONTBOUNDINGBOX on line %d", l.pos+1)
			}
			l.bbox = bbox{l.argInt(0), l.argInt(1), l.argInt(2), l.argInt(3)}
			if l.bbox.width <= 0 || l.bbox.height <= 0 {
				return fmt.Errorf("invalid FONTBOUNDINGBOX %dx%d on line %d",
					l.bbox.width, l.bbox.height, l.pos+1)
			}
			l.ts = New(l.bbox.width, l.bbox.height)
			if l.ts == nil {
				return fmt.Errorf("could not create a tileset for FONTBOUNDINGBOX on line %d", l.pos+1)
			}
		case l.keyword("STARTPROPERTIES"):
			n := l.argInt(0)
			for ; n > 0; n-- {
				if !l.next() {
					return fmt.Errorf("unexpected end of data inside STARTPROPERTIES")
				}
			}
			if !l.next() || !l.keyword("ENDPROPERTIES") {
				return fmt.Errorf("incorrect number of properties near line %d", l.pos+1)
			}
		case l.keyword("CHARS"):
			return l.parseChars()
		case l.keyword("COMMENT"), l.keyword("CONTENTVERSION"), l.keyword("FONT"),
			l.keyword("SIZE"), l.keyword("METRICSSET"), l.keyword("SWIDTH"),
			l.keyword("DWIDTH"), l.keyword("SWIDTH1"), l.keyword("DWIDTH1"),
			l.keyword("VVECTOR"), l.keyword(""):
			// Ignored, as in C.
		default:
			return fmt.Errorf("unknown keyword %q on line %d", firstField(l.line()), l.pos+1)
		}
	}
	return fmt.Errorf("unexpected end of data stream (missing CHARS/ENDFONT)")
}

// parseChars is the C parse_bdf_chars.
func (l *bdfLoader) parseChars() error {
	if l.ts == nil {
		return fmt.Errorf("missing FONTBOUNDINGBOX keyword")
	}
	declared := l.argInt(0)
	processed := 0
	for l.next() {
		switch {
		case l.keyword("ENDFONT"):
			if declared != processed {
				return fmt.Errorf("expected %d glyphs, but processed %d", declared, processed)
			}
			return nil
		case l.keyword("STARTCHAR"):
			if err := l.parseChar(); err != nil {
				return err
			}
			processed++
		case l.keyword(""):
			// Ignore blank lines.
		default:
			return fmt.Errorf("unknown keyword %q on line %d", firstField(l.line()), l.pos+1)
		}
	}
	return fmt.Errorf("unexpected end of data stream (missing ENDFONT)")
}

// parseChar is the C parse_char: one STARTCHAR..ENDCHAR block.
func (l *bdfLoader) parseChar() error {
	codepoint := 0
	var glyph bbox
	for l.next() {
		switch {
		case l.keyword("ENDCHAR"):
			return nil
		case l.keyword("ENCODING"):
			codepoint = l.argInt(0)
		case l.keyword("BBX"):
			glyph = bbox{l.argInt(0), l.argInt(1), l.argInt(2), l.argInt(3)}
		case l.keyword("BITMAP"):
			if err := l.parseBitmap(codepoint, glyph); err != nil {
				return err
			}
		case l.keyword("SWIDTH"), l.keyword("DWIDTH"), l.keyword("SWIDTH1"),
			l.keyword("DWIDTH1"), l.keyword("VVECTOR"), l.keyword(""):
			// Ignored, as in C.
		default:
			return fmt.Errorf("unknown keyword %q on line %d", firstField(l.line()), l.pos+1)
		}
	}
	return fmt.Errorf("unexpected end of data stream inside a glyph")
}

// parseBitmap is the C parse_bitmap: decode hex rows into a tile.
//
// The offsets place a glyph's own bounding box inside the font cell. C
// computes them exactly this way, and the y term flips the origin because
// BDF measures from the baseline upward while tiles are top-down.
func (l *bdfLoader) parseBitmap(codepoint int, glyph bbox) error {
	offsetX := -l.bbox.xOffset + glyph.xOffset
	offsetY := l.bbox.height - glyph.height + l.bbox.yOffset - glyph.yOffset

	// C starts from white-with-zero-alpha and only ever raises alpha.
	pixels := make([]color.RGBA, l.ts.tileLength)
	for i := range pixels {
		pixels[i] = color.RGBA{R: 255, G: 255, B: 255, A: 0}
	}

	for y := 0; y < glyph.height; y++ {
		if !l.next() {
			return fmt.Errorf("unexpected end of data inside a BITMAP")
		}
		row := strings.TrimSpace(l.line())
		cursor := 0
		bitmask := 0
		for x := 0; x < glyph.width; x++ {
			if x%8 == 0 {
				if cursor+2 > len(row) {
					return fmt.Errorf("failed to unpack bitmap on line %d", l.pos+1)
				}
				v, err := strconv.ParseUint(row[cursor:cursor+2], 16, 16)
				if err != nil {
					return fmt.Errorf("failed to unpack bitmap on line %d", l.pos+1)
				}
				bitmask = int(v)
				cursor += 2
			}
			bit := bitmask&(1<<(7-x%8)) != 0
			tx := x + offsetX
			ty := y + offsetY
			if 0 <= tx && tx < l.ts.TileWidth && 0 <= ty && ty < l.ts.TileHeight {
				if bit {
					pixels[ty*l.ts.TileWidth+tx].A = 255
				}
			}
		}
	}

	if codepoint < 0 {
		return nil // "ENCODING -1" means unencoded; C skips it
	}
	return l.ts.SetTile(codepoint, pixels)
}

func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return s
}
