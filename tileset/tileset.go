// Package tileset is a faithful Go port of libtcod's tileset.c: a tile
// atlas mapping Unicode codepoints to RGBA glyph bitmaps.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// # Scope
//
// libtcod's tileset module answers one question: which pixels does
// codepoint N draw? Answering "where do those pixels go" is the renderer's
// job, and in golibtcod that is the presenter interface (present/*).
//
// So the two C files that need external libraries are deliberately absent:
// tileset_render.c requires SDL, and tileset_truetype.c requires
// stb_truetype. Both are rendering concerns already covered by the
// presenters, and either would break the zero-dependency rule. The atlas
// itself (this file) and the BDF font loader (bdf.go) need only libc in C
// and only the standard library here, so they port cleanly.
//
// A Tileset is the general form of a presenter's glyph table: present/pngout
// carries a built-in one and accepts a Tileset to override it.
package tileset

import (
	"fmt"

	"github.com/shindakun/golibtcod/color"
)

const (
	defaultTilesLength   = 256
	defaultCharmapLength = 256
)

// Tileset mirrors TCOD_Tileset: a run of fixed-size RGBA tiles plus a
// codepoint-to-tile-id map.
//
// The observer list and ref_count from the C struct are omitted: both exist
// to invalidate SDL texture atlases when a tile changes, and there is no
// such cache here. Go's garbage collector covers ref_count.
type Tileset struct {
	TileWidth  int
	TileHeight int
	tileLength int // TileWidth * TileHeight

	// pixels holds tilesCount tiles back to back, each tileLength long.
	pixels     []color.RGBA
	tilesCount int

	// characterMap[codepoint] is a tile id; 0 means "not assigned", which
	// is why tile 0 is kept blank.
	characterMap []int

	// VirtualColumns is the atlas width used by codepoint-less lookups.
	VirtualColumns int
}

// maxTileDimension bounds a single glyph cell. Real fonts are tens of
// pixels per side, so this rejects only nonsense. Without it, dimensions
// read straight out of a BDF header reach the allocator: tileWidth *
// tileHeight overflows int (2^28 squared makes capacity * tileLength wrap
// to exactly 0, so reserve reports success having allocated nothing), and a
// merely large value asks for gigabytes per glyph.
const maxTileDimension = 1 << 12 // 4096 px per side

// New is TCOD_tileset_new. Tile dimensions must be positive and no larger
// than maxTileDimension; anything else returns nil.
func New(tileWidth, tileHeight int) *Tileset {
	if tileWidth <= 0 || tileHeight <= 0 {
		return nil
	}
	if tileWidth > maxTileDimension || tileHeight > maxTileDimension {
		return nil
	}
	return &Tileset{
		TileWidth:      tileWidth,
		TileHeight:     tileHeight,
		tileLength:     tileWidth * tileHeight,
		VirtualColumns: 1,
	}
}

// TilesCount reports how many tiles are allocated, including the blank
// tile 0.
func (t *Tileset) TilesCount() int {
	if t == nil {
		return 0
	}
	return t.tilesCount
}

// reserve is TCOD_tileset_reserve: grow the tile buffer to hold want tiles.
func (t *Tileset) reserve(want int) error {
	if t == nil {
		return fmt.Errorf("tileset: nil tileset")
	}
	if t.tileLength == 0 {
		return nil // tiles have zero size
	}
	if want < 0 {
		return fmt.Errorf("tileset: negative tile count %d", want)
	}
	capacity := len(t.pixels) / t.tileLength
	if want <= capacity {
		if t.tilesCount == 0 {
			t.tilesCount = 1 // keep tile zero blank
		}
		return nil
	}
	newCapacity := capacity * 2
	if newCapacity == 0 {
		newCapacity = defaultTilesLength
	}
	if newCapacity < want {
		newCapacity = want
	}
	grown := make([]color.RGBA, newCapacity*t.tileLength)
	copy(grown, t.pixels)
	// Go zeroes the tail, which matches C clearing new tiles to {0,0,0,0}.
	t.pixels = grown
	if t.tilesCount == 0 {
		t.tilesCount = 1 // keep tile zero blank
	}
	return nil
}

// maxCodepoint bounds the character map. Unicode's last codepoint is
// U+10FFFF, so nothing legitimate sits above this; the ceiling stops the
// doubling loop below from overflowing on a hostile or nonsense value.
const maxCodepoint = 0x10FFFF

// charmapReserve is TCOD_tileset_charmap_reserve.
func (t *Tileset) charmapReserve(want int) error {
	if want < 0 {
		return fmt.Errorf("tileset: negative codepoint count %d", want)
	}
	if want > maxCodepoint+1 {
		return fmt.Errorf("tileset: codepoint count %d exceeds the Unicode maximum", want)
	}
	if want <= len(t.characterMap) {
		return nil
	}
	newLength := len(t.characterMap)
	if newLength == 0 {
		newLength = defaultCharmapLength
	}
	// Guard the doubling: newLength*2 overflows to negative past MaxInt/2,
	// which would make this loop spin forever rather than terminate.
	for want > newLength {
		newLength *= 2
		if newLength > maxCodepoint+1 || newLength < 0 {
			newLength = maxCodepoint + 1
			break
		}
	}
	grown := make([]int, newLength)
	copy(grown, t.characterMap)
	t.characterMap = grown
	return nil
}

// newTileID is TCOD_tileset_new_tile_id_: allocate the next blank tile.
func (t *Tileset) newTileID() (int, error) {
	if err := t.reserve(t.tilesCount + 1); err != nil {
		return -1, err
	}
	id := t.tilesCount
	t.tilesCount++
	return id, nil
}

// AssignTile is TCOD_tileset_assign_tile: point a codepoint at a tile id.
func (t *Tileset) AssignTile(tileID, codepoint int) error {
	if t == nil {
		return fmt.Errorf("tileset: nil tileset")
	}
	if tileID < 0 || tileID >= t.tilesCount {
		return fmt.Errorf("tileset: tile id %d out of bounds", tileID)
	}
	if codepoint < 0 {
		return fmt.Errorf("tileset: negative codepoint %d", codepoint)
	}
	if err := t.charmapReserve(codepoint + 1); err != nil {
		return err
	}
	t.characterMap[codepoint] = tileID
	return nil
}

// generateCodepoint is TCOD_tileset_generate_codepoint: return the tile id
// for a codepoint, allocating a blank tile the first time it is seen.
func (t *Tileset) generateCodepoint(codepoint int) (int, error) {
	if codepoint < 0 {
		return -1, fmt.Errorf("tileset: negative codepoint %d", codepoint)
	}
	if codepoint < len(t.characterMap) {
		if id := t.characterMap[codepoint]; id > 0 {
			return id, nil
		}
	}
	id, err := t.newTileID()
	if err != nil {
		return -1, err
	}
	if err := t.AssignTile(id, codepoint); err != nil {
		return -1, err
	}
	return id, nil
}

// SetTile is TCOD_tileset_set_tile_: upload a glyph bitmap for a codepoint.
// The pixel slice must be TileWidth*TileHeight long, in row-major order.
func (t *Tileset) SetTile(codepoint int, pixels []color.RGBA) error {
	if t == nil {
		return fmt.Errorf("tileset: nil tileset")
	}
	if len(pixels) < t.tileLength {
		return fmt.Errorf("tileset: need %d pixels for a tile, got %d", t.tileLength, len(pixels))
	}
	id, err := t.generateCodepoint(codepoint)
	if err != nil {
		return err
	}
	copy(t.pixels[id*t.tileLength:(id+1)*t.tileLength], pixels[:t.tileLength])
	return nil
}

// Tile is TCOD_tileset_get_tile: the glyph bitmap for a codepoint, or nil
// if the codepoint has no tile.
//
// The result aliases the tileset's internal buffer, matching C, which
// returns a const pointer into the same storage. Treat it as read-only, do
// not re-slice it past its length (the underlying array continues into the
// next glyph), and do not retain it: a later SetTile can reallocate the
// buffer, after which writes through an older slice are silently lost. Use
// Coverage, which copies, when you need a value you can keep or modify.
//
// DIVERGENCE (deliberate): C's TCOD_tileset_get_tile_id returns 0 for an
// unmapped codepoint, and TCOD_tileset_get_tile only rejects a negative id,
// so C hands back tile 0 (the reserved blank tile) and reports success. An
// unmapped codepoint is therefore indistinguishable from one deliberately
// assigned a blank glyph. Verified against the C build: loading
// Tamzen5x9r.bdf and asking for U+00B1, U+03B4, U+2588 or U+2591 (none of
// which that font defines) yields an all-zero tile rather than an error.
//
// Reporting nil instead lets a caller tell "no glyph" from "blank glyph",
// which is what a presenter needs to draw a missing-glyph marker. Every
// codepoint the font actually defines behaves identically in both.
func (t *Tileset) Tile(codepoint int) []color.RGBA {
	if t == nil || codepoint < 0 || codepoint >= len(t.characterMap) {
		return nil
	}
	id := t.characterMap[codepoint]
	if id <= 0 || id >= t.tilesCount {
		return nil // tile 0 is the reserved blank tile: treat as unassigned
	}
	return t.pixels[id*t.tileLength : (id+1)*t.tileLength]
}

// HasTile reports whether a codepoint has a glyph assigned.
func (t *Tileset) HasTile(codepoint int) bool { return t.Tile(codepoint) != nil }

// Codepoints returns every codepoint with a tile assigned, ascending.
func (t *Tileset) Codepoints() []int {
	if t == nil {
		return nil
	}
	var out []int
	for cp, id := range t.characterMap {
		if id > 0 && id < t.tilesCount {
			out = append(out, cp)
		}
	}
	return out
}

// Coverage reports the alpha of each pixel in a codepoint's tile as a
// row-major bitmap, which is what a presenter needs to decide whether to
// paint foreground or background. Nil if the codepoint has no tile.
//
// BDF glyphs are monochrome and stored as white-with-alpha, so alpha is the
// whole signal; anti-aliased sources give intermediate values.
func (t *Tileset) Coverage(codepoint int) []uint8 {
	tile := t.Tile(codepoint)
	if tile == nil {
		return nil
	}
	out := make([]uint8, len(tile))
	for i, p := range tile {
		out[i] = p.A
	}
	return out
}
