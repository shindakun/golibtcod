package namegen

import (
	"strings"
	"testing"

	"golibtcod/rng"
)

const cfg = `
name "test" {
    phonemesVocals = "a e i o u"
    phonemesConsonants = "b c d f g"
    syllablesPre = "Old Deep"
    syllablesStart = "Dro Hrim Vint"
    syllablesMiddle = "en ar il"
    syllablesEnd = "heim dal gard"
    illegal = "qq xx"
    rules = "$s$e,$s$m$e"
}
`

func newGen(t *testing.T, seed uint32) *Generator {
	t.Helper()
	reg := NewRegistry(rng.New(rng.CMWC, seed))
	if err := reg.Parse(cfg); err != nil {
		t.Fatal(err)
	}
	g, ok := reg.Get("test")
	if !ok {
		t.Fatal("set not registered")
	}
	return g
}

func TestGenerateUsesSyllables(t *testing.T) {
	g := newGen(t, 1234)
	starts := []string{"Dro", "Hrim", "Vint"}
	ends := []string{"heim", "dal", "gard"}
	for i := 0; i < 200; i++ {
		name, err := g.Generate()
		if err != nil {
			t.Fatal(err)
		}
		okStart := false
		for _, s := range starts {
			if strings.HasPrefix(name, s) {
				okStart = true
			}
		}
		okEnd := false
		for _, e := range ends {
			if strings.HasSuffix(name, e) {
				okEnd = true
			}
		}
		if !okStart || !okEnd {
			t.Fatalf("name %q does not match the rule shape", name)
		}
	}
}

func TestFiltersReject(t *testing.T) {
	g := newGen(t, 99)
	for i := 0; i < 500; i++ {
		name, err := g.Generate()
		if err != nil {
			t.Fatal(err)
		}
		if hasTriples(name) {
			t.Fatalf("triple letters survived: %q", name)
		}
		if g.hasIllegal(name) {
			t.Fatalf("illegal substring survived: %q", name)
		}
		if pruneSyllables(name) {
			t.Fatalf("repeated syllables survived: %q", name)
		}
		if name != strings.TrimSpace(name) || strings.Contains(name, "  ") {
			t.Fatalf("space pruning failed: %q", name)
		}
	}
}

func TestDeterminism(t *testing.T) {
	a, b := newGen(t, 7), newGen(t, 7)
	for i := 0; i < 100; i++ {
		na, _ := a.Generate()
		nb, _ := b.Generate()
		if na != nb {
			t.Fatalf("same seed diverged: %q vs %q", na, nb)
		}
	}
}

func TestCustomWildcards(t *testing.T) {
	g := newGen(t, 3)
	name, err := g.GenerateCustom("$P_$s$e")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(name, " ") {
		t.Fatalf("underscore did not become a space: %q", name)
	}
	if _, err := g.GenerateCustom("$Z"); err == nil {
		t.Fatal("expected an error for an unknown wildcard")
	}
}

func TestPopulateTokenizing(t *testing.T) {
	got := populate("a e i", false)
	if len(got) != 3 || got[0] != "a" {
		t.Fatalf("populate = %v", got)
	}
	// underscore becomes a space without wildcards, is kept with them
	if got := populate("a_b", false); got[0] != "a b" {
		t.Fatalf("underscore no-wildcards = %v", got)
	}
	if got := populate("$s_$e", true); got[0] != "$s_$e" {
		t.Fatalf("wildcard token = %v", got)
	}
}
