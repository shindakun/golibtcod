package rexpaint

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"
	"time"
)

// A corrupt header must be rejected before allocating: layer dimensions are
// read before the tile data, so an implausible size previously reserved
// tens of gigabytes from a ~40 byte file.
func TestHostileLayerDimensionsRejected(t *testing.T) {
	for _, d := range [][2]int32{
		{40000, 40000}, // ~24 GB
		{65536, 65536}, // ~64 GB
		{1 << 20, 1 << 20},
		{1, 1 << 28}, // lopsided but still over the cap
	} {
		var raw bytes.Buffer
		gz := gzip.NewWriter(&raw)
		if err := binary.Write(gz, binary.LittleEndian, header{Version: -1, LayerCount: 1}); err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(gz, binary.LittleEndian, layerChunk{Width: d[0], Height: d[1]}); err != nil {
			t.Fatal(err)
		}
		gz.Close()

		start := time.Now()
		_, err := ReadLayers(bytes.NewReader(raw.Bytes()))
		elapsed := time.Since(start)

		if err == nil {
			t.Errorf("%dx%d: expected rejection, got nil error", d[0], d[1])
		}
		if elapsed > 2*time.Second {
			t.Errorf("%dx%d: took %v; expected immediate rejection", d[0], d[1], elapsed)
		}
	}
}

// Sizes within the cap must still round-trip, including a non-square
// console (the case that catches a transposed writer).
func TestPlausibleSizesStillRoundTrip(t *testing.T) {
	for _, d := range [][2]int{{1, 1}, {7, 3}, {80, 50}} {
		var buf bytes.Buffer
		if err := Write(&buf, sample(d[0], d[1])); err != nil {
			t.Fatalf("%dx%d write failed: %v", d[0], d[1], err)
		}
		layers, err := ReadLayers(&buf)
		if err != nil {
			t.Fatalf("%dx%d read failed: %v", d[0], d[1], err)
		}
		if len(layers) != 1 {
			t.Fatalf("%dx%d: got %d layers, want 1", d[0], d[1], len(layers))
		}
		if layers[0].W != d[0] || layers[0].H != d[1] {
			t.Errorf("got %dx%d, want %dx%d", layers[0].W, layers[0].H, d[0], d[1])
		}
	}
}
