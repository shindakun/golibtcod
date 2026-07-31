// Package rexpaint reads and writes REXPaint `.xp` files: a faithful Go
// port of libtcod's console_rexpaint.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// The C version links zlib; this uses Go's compress/gzip, so the package
// keeps golibtcod's zero-dependency rule. The file format is unchanged:
//
//	gzip stream:
//	  int32 version
//	  int32 layerCount
//	  per layer:
//	    int32 width, int32 height
//	    width*height tiles, COLUMN-MAJOR:
//	      int32 codepoint, RGB fg (3 bytes), RGB bg (3 bytes)
//
// Layers are combined with fuchsia (255,0,255) as the transparent key,
// matching REXPaint's own rule.
package rexpaint

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/shindakun/golibtcod/color"
	"github.com/shindakun/golibtcod/console"
)

// KeyColor is the transparency key REXPaint uses between layers.
var KeyColor = color.RGB{R: 255, G: 0, B: 255}

// maxLayerCells bounds the tile count of a single layer read from a file.
// REXPaint's own editor tops out far below this, so it rejects only corrupt
// or hostile headers. Without it a ~40 byte file claiming a 65536x65536
// layer reserves ~64 GB before the first tile read fails.
const maxLayerCells = 1 << 24 // 16.7M cells, e.g. 4096x4096

type header struct {
	Version    int32
	LayerCount int32
}

type layerChunk struct {
	Width  int32
	Height int32
}

type tile struct {
	Ch         int32
	Fr, Fg, Fb uint8
	Br, Bg, Bb uint8
}

// LoadLayers reads every layer of an .xp file as a separate console.
func LoadLayers(path string) ([]*console.Console, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadLayers(f)
}

// ReadLayers decodes .xp data from any reader.
func ReadLayers(r io.Reader) ([]*console.Console, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("rexpaint: opening gzip stream: %w", err)
	}
	defer gz.Close()

	var h header
	if err := binary.Read(gz, binary.LittleEndian, &h); err != nil {
		return nil, fmt.Errorf("rexpaint: reading header: %w", err)
	}
	if h.LayerCount <= 0 || h.LayerCount > 64 {
		return nil, fmt.Errorf("rexpaint: implausible layer count %d", h.LayerCount)
	}

	layers := make([]*console.Console, 0, h.LayerCount)
	for i := int32(0); i < h.LayerCount; i++ {
		var lc layerChunk
		if err := binary.Read(gz, binary.LittleEndian, &lc); err != nil {
			return nil, fmt.Errorf("rexpaint: reading layer %d header: %w", i, err)
		}
		if lc.Width <= 0 || lc.Height <= 0 {
			return nil, fmt.Errorf("rexpaint: layer %d has bad size %dx%d", i, lc.Width, lc.Height)
		}
		// A corrupt or hostile header must not drive a huge allocation: the
		// tile data is read after this point, so an implausible size would
		// otherwise reserve gigabytes before hitting EOF.
		if int64(lc.Width)*int64(lc.Height) > maxLayerCells {
			return nil, fmt.Errorf("rexpaint: layer %d size %dx%d exceeds the %d-cell limit",
				i, lc.Width, lc.Height, maxLayerCells)
		}
		c := console.New(int(lc.Width), int(lc.Height))
		total := int(lc.Width) * int(lc.Height)
		for k := 0; k < total; k++ {
			var t tile
			if err := binary.Read(gz, binary.LittleEndian, &t); err != nil {
				return nil, fmt.Errorf("rexpaint: reading layer %d tile %d: %w", i, k, err)
			}
			// REXPaint stores tiles column-major
			x := k / int(lc.Height)
			y := k % int(lc.Height)
			c.Tiles[x+y*c.W] = console.Tile{
				Ch: int(t.Ch),
				Fg: color.RGBA{R: t.Fr, G: t.Fg, B: t.Fb, A: 255},
				Bg: color.RGBA{R: t.Br, G: t.Bg, B: t.Bb, A: 255},
			}
		}
		layers = append(layers, c)
	}
	return layers, nil
}

// Load reads an .xp file and flattens its layers into one console,
// following REXPaint's transparency rule (fuchsia is the key color).
func Load(path string) (*console.Console, error) {
	layers, err := LoadLayers(path)
	if err != nil {
		return nil, err
	}
	return Combine(layers), nil
}

// Combine flattens layers bottom-up using the fuchsia key color.
func Combine(layers []*console.Console) *console.Console {
	if len(layers) == 0 {
		return nil
	}
	main := layers[0]
	for _, layer := range layers[1:] {
		layer.SetKeyColor(KeyColor)
		console.Blit(layer, 0, 0, 0, 0, main, 0, 0, 1.0, 1.0)
	}
	return main
}

// Save writes consoles as the layers of an .xp file.
//
// Close is checked rather than deferred: on a write path a failed flush
// means a truncated file, and deferring would report success anyway.
func Save(path string, layers ...*console.Console) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return Write(f, layers...)
}

// Write encodes consoles as .xp data to any writer.
func Write(w io.Writer, layers ...*console.Console) (err error) {
	if len(layers) == 0 {
		return fmt.Errorf("rexpaint: no layers to write")
	}
	gz := gzip.NewWriter(w)
	// The gzip trailer is only written on Close, so a dropped error here
	// yields a file that looks fine until something tries to read it.
	defer func() {
		if cerr := gz.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	h := header{Version: -1, LayerCount: int32(len(layers))}
	if err := binary.Write(gz, binary.LittleEndian, h); err != nil {
		return err
	}
	for _, c := range layers {
		if c == nil {
			return fmt.Errorf("rexpaint: nil layer")
		}
		if err := binary.Write(gz, binary.LittleEndian,
			layerChunk{Width: int32(c.W), Height: int32(c.H)}); err != nil {
			return err
		}
		total := c.W * c.H
		for k := 0; k < total; k++ {
			x := k / c.H
			y := k % c.H
			t := c.Tiles[x+y*c.W]
			ch := t.Ch
			if ch == 0 {
				ch = ' '
			}
			if err := binary.Write(gz, binary.LittleEndian, tile{
				Ch: int32(ch),
				Fr: t.Fg.R, Fg: t.Fg.G, Fb: t.Fg.B,
				Br: t.Bg.R, Bg: t.Bg.G, Bb: t.Bg.B,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
