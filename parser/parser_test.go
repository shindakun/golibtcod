package parser

import "testing"

const sample = `
# a comment
name "Celtic female" {   // trailing comment
    phonemesVocals = "a e i o u"
    weight = 42
    ratio = 1.5
    enabled = true
    rules = ["$s$e", "$s$m$e"]
    fancy
    /* block
       comment */
    item "sword" {
        damage = 7
    }
}

name "Second" {
    phonemesVocals = "y"
}
`

func TestParseBasics(t *testing.T) {
	structs, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(structs) != 2 {
		t.Fatalf("got %d top-level structs, want 2", len(structs))
	}
	s := structs[0]
	if s.Type != "name" || s.Name != "Celtic female" {
		t.Fatalf("type/name = %q/%q", s.Type, s.Name)
	}
	if got := s.PropString("phonemesVocals"); got != "a e i o u" {
		t.Fatalf("vocals = %q", got)
	}
	if v, _ := s.Prop("weight"); func() int { n, _ := v.Int(); return n }() != 42 {
		t.Fatal("weight != 42")
	}
	if v, _ := s.Prop("ratio"); func() float32 { f, _ := v.Float(); return f }() != 1.5 {
		t.Fatal("ratio != 1.5")
	}
	if v, _ := s.Prop("enabled"); func() bool { b, _ := v.Bool(); return b }() != true {
		t.Fatal("enabled != true")
	}
	v, _ := s.Prop("rules")
	if len(v.List) != 2 || v.List[0] != "$s$e" || v.List[1] != "$s$m$e" {
		t.Fatalf("rules list = %v", v.List)
	}
	if !s.HasFlag("fancy") {
		t.Fatal("flag 'fancy' not recorded")
	}
	if len(s.Children) != 1 || s.Children[0].Name != "sword" {
		t.Fatalf("children = %+v", s.Children)
	}
	if structs[1].Name != "Second" {
		t.Fatalf("second struct = %q", structs[1].Name)
	}
}

func TestParseEscapes(t *testing.T) {
	structs, err := Parse(`a "x" { s = "he said \"hi\" and \\ left" }`)
	if err != nil {
		t.Fatal(err)
	}
	if got := structs[0].PropString("s"); got != `he said "hi" and \ left` {
		t.Fatalf("escapes = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, bad := range []string{
		`name "x" {`,          // unterminated struct
		`name "x" { s = "y }`, // unterminated string
		`{ }`,                 // no type
	} {
		if _, err := Parse(bad); err == nil {
			t.Fatalf("expected an error for %q", bad)
		}
	}
}
