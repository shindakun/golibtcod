package tileset

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shindakun/golibtcod/color"
)

func solidTile(t *Tileset) []color.RGBA {
	px := make([]color.RGBA, t.TileWidth*t.TileHeight)
	for i := range px {
		px[i] = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return px
}

func TestNewRejectsBadSizes(t *testing.T) {
	for _, d := range [][2]int{{0, 8}, {8, 0}, {-1, 8}, {8, -1}, {0, 0}} {
		if ts := New(d[0], d[1]); ts != nil {
			t.Errorf("New(%d,%d) = non-nil, want nil", d[0], d[1])
		}
	}
	if ts := New(4, 6); ts == nil {
		t.Error("New(4,6) returned nil")
	}
}

func TestSetAndGetTile(t *testing.T) {
	ts := New(2, 2)
	if err := ts.SetTile('A', solidTile(ts)); err != nil {
		t.Fatal(err)
	}
	got := ts.Tile('A')
	if got == nil {
		t.Fatal("no tile after SetTile")
	}
	if len(got) != 4 {
		t.Fatalf("tile length %d, want 4", len(got))
	}
	for i, p := range got {
		if p.A != 255 {
			t.Errorf("pixel %d alpha = %d, want 255", i, p.A)
		}
	}
	if !ts.HasTile('A') {
		t.Error("HasTile false for an assigned codepoint")
	}
}

// Tile 0 is reserved blank, so an unassigned codepoint reports missing
// rather than handing back a blank tile the way C does.
func TestUnassignedCodepointIsMissing(t *testing.T) {
	ts := New(2, 2)
	if err := ts.SetTile('A', solidTile(ts)); err != nil {
		t.Fatal(err)
	}
	for _, cp := range []int{'B', 0, -1, 99999} {
		if ts.Tile(cp) != nil {
			t.Errorf("codepoint %d reported a tile, want nil", cp)
		}
		if ts.HasTile(cp) {
			t.Errorf("HasTile(%d) = true, want false", cp)
		}
	}
}

func TestSetTileRejectsShortBuffer(t *testing.T) {
	ts := New(4, 4)
	if err := ts.SetTile('A', make([]color.RGBA, 3)); err == nil {
		t.Error("expected an error for a short pixel buffer")
	}
}

func TestAssignTileBounds(t *testing.T) {
	ts := New(2, 2)
	if err := ts.SetTile('A', solidTile(ts)); err != nil {
		t.Fatal(err)
	}
	if err := ts.AssignTile(999, 'B'); err == nil {
		t.Error("expected an error for an out-of-range tile id")
	}
	if err := ts.AssignTile(1, -5); err == nil {
		t.Error("expected an error for a negative codepoint")
	}
	// Aliasing one tile to a second codepoint is legal.
	if err := ts.AssignTile(1, 'B'); err != nil {
		t.Fatalf("aliasing failed: %v", err)
	}
	if !ts.HasTile('B') {
		t.Error("alias codepoint has no tile")
	}
}

// Codepoints past the initial charmap must grow it rather than panic.
func TestHighCodepointsGrowCharmap(t *testing.T) {
	ts := New(2, 2)
	for _, cp := range []int{0x41, 0x2588, 0x10FFFF} {
		if err := ts.SetTile(cp, solidTile(ts)); err != nil {
			t.Fatalf("SetTile(%#x): %v", cp, err)
		}
	}
	for _, cp := range []int{0x41, 0x2588, 0x10FFFF} {
		if !ts.HasTile(cp) {
			t.Errorf("codepoint %#x lost after growth", cp)
		}
	}
}

func TestNilTilesetSafe(t *testing.T) {
	var ts *Tileset
	if ts.TilesCount() != 0 {
		t.Error("TilesCount on nil should be 0")
	}
	if ts.Tile('A') != nil || ts.HasTile('A') || ts.Coverage('A') != nil {
		t.Error("nil tileset should report no tiles")
	}
	if ts.Codepoints() != nil {
		t.Error("nil tileset should have no codepoints")
	}
	if err := ts.SetTile('A', nil); err == nil {
		t.Error("SetTile on nil should error")
	}
}

func TestCoverageMatchesAlpha(t *testing.T) {
	ts := New(2, 1)
	px := []color.RGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 0},
	}
	if err := ts.SetTile('X', px); err != nil {
		t.Fatal(err)
	}
	cov := ts.Coverage('X')
	if len(cov) != 2 || cov[0] != 255 || cov[1] != 0 {
		t.Errorf("Coverage = %v, want [255 0]", cov)
	}
}

/* --- BDF parsing --- */

// A minimal well-formed BDF: one 2x2 glyph with the top row set.
const miniBDF = `STARTFONT 2.1
FONT test
SIZE 2 75 75
FONTBOUNDINGBOX 2 2 0 0
STARTPROPERTIES 1
FAMILY_NAME "test"
ENDPROPERTIES
CHARS 1
STARTCHAR uniAAAA
ENCODING 65
SWIDTH 500 0
DWIDTH 2 0
BBX 2 2 0 0
BITMAP
C0
00
ENDCHAR
ENDFONT
`

func TestReadBDFMinimal(t *testing.T) {
	ts, err := ReadBDF([]byte(miniBDF))
	if err != nil {
		t.Fatal(err)
	}
	if ts.TileWidth != 2 || ts.TileHeight != 2 {
		t.Fatalf("cell = %dx%d, want 2x2", ts.TileWidth, ts.TileHeight)
	}
	cov := ts.Coverage('A')
	if cov == nil {
		t.Fatal("no glyph for 'A'")
	}
	// 0xC0 = 1100_0000: both pixels of a 2-wide row are set.
	want := []uint8{255, 255, 0, 0}
	for i := range want {
		if cov[i] != want[i] {
			t.Errorf("coverage = %v, want %v", cov, want)
			break
		}
	}
}

func TestReadBDFHandlesCRLF(t *testing.T) {
	ts, err := ReadBDF([]byte(strings.ReplaceAll(miniBDF, "\n", "\r\n")))
	if err != nil {
		t.Fatalf("CRLF input failed: %v", err)
	}
	if !ts.HasTile('A') {
		t.Error("glyph lost with CRLF line endings")
	}
}

// ENCODING -1 marks an unencoded glyph; C skips it and so must this.
func TestReadBDFSkipsUnencodedGlyph(t *testing.T) {
	src := strings.Replace(miniBDF, "ENCODING 65", "ENCODING -1", 1)
	ts, err := ReadBDF([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(ts.Codepoints()) != 0 {
		t.Errorf("unencoded glyph was stored: %v", ts.Codepoints())
	}
}

func TestReadBDFErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"no STARTFONT", "FONTBOUNDINGBOX 2 2 0 0\n"},
		{"missing FONTBOUNDINGBOX", "STARTFONT 2.1\nCHARS 1\nENDFONT\n"},
		{"truncated", "STARTFONT 2.1\nFONTBOUNDINGBOX 2 2 0 0\n"},
		{"glyph count mismatch", strings.Replace(miniBDF, "CHARS 1", "CHARS 2", 1)},
		{"bad hex", strings.Replace(miniBDF, "C0", "ZZ", 1)},
		{"duplicate bbox", strings.Replace(miniBDF, "SIZE 2 75 75", "FONTBOUNDINGBOX 2 2 0 0", 1)},
	}
	for _, c := range cases {
		if _, err := ReadBDF([]byte(c.src)); err == nil {
			t.Errorf("%s: expected an error, got nil", c.name)
		}
	}
}

func TestLoadBDFMissingFile(t *testing.T) {
	if _, err := LoadBDF("does-not-exist.bdf"); err == nil {
		t.Error("expected an error for a missing file")
	}
}

// charmapReserve doubles its length until it covers the requested codepoint.
// Past MaxInt/2 that doubling overflowed to negative and the loop spun
// forever, reachable straight through SetTile.
func TestCharmapReserveTerminates(t *testing.T) {
	const maxInt = int(^uint(0) >> 1)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic: %v", r)
			}
		}()
		done <- New(2, 2).SetTile(maxInt-1, make([]color.RGBA, 4))
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected an error for an absurd codepoint")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetTile did not return: charmapReserve is looping")
	}
}

// The whole Unicode range must still be assignable.
func TestFullUnicodeRangeAssignable(t *testing.T) {
	ts := New(2, 2)
	px := make([]color.RGBA, 4)
	for _, cp := range []int{0x41, 0x2588, 0xFFFF, 0x10000, 0x10FFFF} {
		if err := ts.SetTile(cp, px); err != nil {
			t.Errorf("SetTile(%#x): %v", cp, err)
		} else if !ts.HasTile(cp) {
			t.Errorf("codepoint %#x lost", cp)
		}
	}
	if err := ts.SetTile(0x110000, px); err == nil {
		t.Error("expected rejection past U+10FFFF")
	}
}

// FONTBOUNDINGBOX goes straight to New, so an unbounded cell size let a
// tiny hostile file reach the allocator: 2^28 squared overflows int and
// made reserve report success with zero storage, and merely large values
// asked for gigabytes per glyph. A panic must never escape ReadBDF, which
// returns an error.
func TestHostileFontBoundingBoxRejected(t *testing.T) {
	for _, bbox := range []string{
		"268435456 268435456", // overflows tileLength * capacity to 0
		"65536 65536",         // ~16 GB per glyph
		"1000000000 1000000000",
		"1 1000000000",
	} {
		src := "STARTFONT 2.1\nFONTBOUNDINGBOX " + bbox +
			" 0 0\nCHARS 1\nSTARTCHAR a\nENCODING 65\nBBX 1 1 0 0\nBITMAP\n80\nENDCHAR\nENDFONT\n"
		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("panic: %v", r)
				}
			}()
			_, err := ReadBDF([]byte(src))
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("FONTBOUNDINGBOX %q was accepted", bbox)
			} else if strings.HasPrefix(err.Error(), "panic:") {
				t.Errorf("FONTBOUNDINGBOX %q: %v", bbox, err)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("FONTBOUNDINGBOX %q: did not return", bbox)
		}
	}
}

// reserve must never report success while allocating nothing.
func TestNewRejectsOverflowingDimensions(t *testing.T) {
	for _, d := range [][2]int{{1 << 28, 1 << 28}, {1 << 30, 1 << 30}, {maxTileDimension + 1, 1}} {
		if ts := New(d[0], d[1]); ts != nil {
			t.Errorf("New(%d,%d) should be nil", d[0], d[1])
		}
	}
	// The largest allowed cell still works.
	if ts := New(maxTileDimension, 1); ts == nil {
		t.Errorf("New(%d,1) should be allowed", maxTileDimension)
	}
}
