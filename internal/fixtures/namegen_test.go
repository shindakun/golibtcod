package fixtures

import (
	_ "embed"
	"strconv"
	"strings"
	"testing"

	"golibtcod/namegen"
	"golibtcod/rng"
)

//go:embed fixtures_namegen.txt
var namegenRaw string

//go:embed mkname.cfg
var mknameCfg string

func TestNamegenFixtures(t *testing.T) {
	var secs []section
	for _, line := range strings.Split(namegenRaw, "\n") {
		if strings.HasPrefix(line, "== ") {
			secs = append(secs, section{header: strings.TrimPrefix(line, "== ")})
		} else if line != "" && len(secs) > 0 {
			secs[len(secs)-1].lines = append(secs[len(secs)-1].lines, line)
		}
	}
	if len(secs) == 0 {
		t.Fatal("no namegen fixtures")
	}

	gens := map[uint32]*namegen.Generator{}
	getGen := func(seed uint32) *namegen.Generator {
		if g, ok := gens[seed]; ok {
			return g
		}
		reg := namegen.NewRegistry(rng.New(rng.CMWC, seed))
		if err := reg.Parse(mknameCfg); err != nil {
			t.Fatalf("parse mkname.cfg: %v", err)
		}
		g, ok := reg.Get("syllables")
		if !ok {
			t.Fatal(`set "syllables" not registered`)
		}
		gens[seed] = g
		return g
	}

	n := 0
	for _, sec := range secs {
		f := strings.Fields(sec.header) // namegen <kind> <seed>
		seed64, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			t.Fatalf("bad seed %q", f[2])
		}
		g := getGen(uint32(seed64))
		for i, want := range sec.lines {
			var got string
			var err error
			switch f[1] {
			case "generate":
				got, err = g.Generate()
			case "custom":
				got, err = g.GenerateCustom("$s$m$e")
			case "wildcards":
				got, err = g.GenerateCustom("$v$c$?_$60P$s$e")
			}
			if err != nil {
				t.Fatalf("%s[%d]: %v", sec.header, i, err)
			}
			if got != want {
				t.Fatalf("%s[%d]: got %q want %q", sec.header, i, got, want)
			}
			n++
		}
	}
	t.Logf("verified %d generated names against C", n)
}
