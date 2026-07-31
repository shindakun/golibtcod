package rexpaint

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"
	"time"
)

func TestHostileDimsRejectedFast(t *testing.T) {
	for _, d := range [][2]int32{{40000, 40000}, {65536, 65536}, {1 << 20, 1 << 20}} {
		var raw bytes.Buffer
		gz := gzip.NewWriter(&raw)
		binary.Write(gz, binary.LittleEndian, header{Version: -1, LayerCount: 1})
		binary.Write(gz, binary.LittleEndian, layerChunk{Width: d[0], Height: d[1]})
		gz.Close()

		start := time.Now()
		_, err := ReadLayers(bytes.NewReader(raw.Bytes()))
		el := time.Since(start)
		if err == nil {
			t.Errorf("%dx%d: expected rejection, got nil error", d[0], d[1])
		}
		if el > 2*time.Second {
			t.Errorf("%dx%d: took %v, expected immediate rejection", d[0], d[1], el)
		}
		t.Logf("%dx%d rejected in %v: %v", d[0], d[1], el, err)
	}
}
