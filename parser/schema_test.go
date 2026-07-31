package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// itemSchema mirrors the schema declared in the C harness
// (internal/fixtures/parser/gen_parser.c.txt), so these tests check the
// same declarations libtcod was given.
func itemSchema() Schema {
	s := NewSchema()
	s.Declare("sublist").Prop("bonus", TypeInt, false)
	s.Declare("item_type").
		Prop("cost", TypeInt, true).
		Prop("weight", TypeFloat, false).
		Prop("deal_damage", TypeBool, false).
		Prop("damages", TypeDice, false).
		Prop("col", TypeColor, false).
		Prop("initial", TypeChar, false).
		Prop("description", TypeString, false).
		Flag("abstract").
		Child("sublist")
	return s
}

func parseFile(t *testing.T, name string) []*Struct {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "internal", "fixtures", "parser", name))
	if err != nil {
		t.Fatal(err)
	}
	structs, err := Parse(string(data))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return structs
}

// case1 is the well-formed file both parsers agreed on. It must validate
// cleanly, or the schema layer is stricter than libtcod.
func TestValidCaseAccepted(t *testing.T) {
	if errs := Validate(parseFile(t, "case1.cfg"), itemSchema()); errs != nil {
		t.Fatalf("well-formed file rejected: %v", errs)
	}
}

// case3: libtcod reported "entity type item_type does not contain
// undeclared_property". Without the schema layer golibtcod accepted it
// silently; that was divergence #2.
func TestUndeclaredPropertyRejected(t *testing.T) {
	errs := Validate(parseFile(t, "case3.cfg"), itemSchema())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	e := errs[0]
	if e.Property != "undeclared_property" {
		t.Errorf("error names %q, want undeclared_property", e.Property)
	}
	if e.Line != 3 {
		t.Errorf("error at line %d, want 3 (libtcod also says line 3)", e.Line)
	}
}

// case7: libtcod reported "unknown entity type monster".
func TestUnknownStructTypeRejected(t *testing.T) {
	errs := Validate(parseFile(t, "case7.cfg"), itemSchema())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if !strings.Contains(errs[0].Msg, "unknown struct type") {
		t.Errorf("message = %q", errs[0].Msg)
	}
}

// case6: libtcod reported a type error and substituted 0. We report the
// error and keep the original text, which is the more useful half.
func TestTypeMismatchRejected(t *testing.T) {
	errs := Validate(parseFile(t, "case6.cfg"), itemSchema())
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if !strings.Contains(errs[0].Msg, "expected an integer") {
		t.Errorf("message = %q", errs[0].Msg)
	}
	if !strings.Contains(errs[0].Msg, "not a number") {
		t.Errorf("message should quote the offending text, got %q", errs[0].Msg)
	}
}

func TestMandatoryPropertyMissing(t *testing.T) {
	structs, err := Parse(`item_type "no_cost" { weight = 1.0 }`)
	if err != nil {
		t.Fatal(err)
	}
	errs := Validate(structs, itemSchema())
	if len(errs) != 1 || errs[0].Property != "cost" {
		t.Fatalf("expected a missing-cost error, got %v", errs)
	}
	if !strings.Contains(errs[0].Msg, "mandatory") {
		t.Errorf("message = %q", errs[0].Msg)
	}
}

func TestUndeclaredFlagRejected(t *testing.T) {
	structs, err := Parse(`item_type "x" { cost = 1 legendary }`)
	if err != nil {
		t.Fatal(err)
	}
	errs := Validate(structs, itemSchema())
	if len(errs) != 1 || errs[0].Property != "legendary" {
		t.Fatalf("expected a flag error, got %v", errs)
	}
}

func TestNestingRules(t *testing.T) {
	// sublist is permitted inside item_type
	ok, err := Parse(`item_type "x" { cost = 1 sublist { bonus = 2 } }`)
	if err != nil {
		t.Fatal(err)
	}
	if errs := Validate(ok, itemSchema()); errs != nil {
		t.Fatalf("declared nesting rejected: %v", errs)
	}
	// item_type inside item_type is not
	bad, err := Parse(`item_type "x" { cost = 1 item_type "y" { cost = 2 } }`)
	if err != nil {
		t.Fatal(err)
	}
	errs := Validate(bad, itemSchema())
	if len(errs) != 1 || !strings.Contains(errs[0].Msg, "not permitted inside") {
		t.Fatalf("expected a nesting error, got %v", errs)
	}
}

// libtcod stops at its first fatal error. Reporting everything is more
// useful for a config file, so this is a deliberate divergence; pin it.
func TestAllErrorsReportedNotJustTheFirst(t *testing.T) {
	structs, err := Parse(`
item_type "a" {
	bogus_one = 1
	bogus_two = 2
	cost = "nope"
}`)
	if err != nil {
		t.Fatal(err)
	}
	errs := Validate(structs, itemSchema())
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
	// and in source order
	for i := 1; i < len(errs); i++ {
		if errs[i].Line < errs[i-1].Line {
			t.Fatalf("errors out of source order: %v", errs)
		}
	}
}

func TestAllowUnknownRelaxesTheCheck(t *testing.T) {
	s := itemSchema()
	s.Declare("item_type").AllowUnknown()
	if errs := Validate(parseFile(t, "case3.cfg"), s); errs != nil {
		t.Fatalf("AllowUnknown should permit extra properties: %v", errs)
	}
}

/* ------------------------------------------------- typed value getters */

func TestValueTypes(t *testing.T) {
	structs := parseFile(t, "case1.cfg")
	item := structs[0]

	if v, _ := item.Prop("cost"); mustInt(t, v) != 300 {
		t.Error("cost")
	}
	if v, _ := item.Prop("weight"); mustFloat(t, v) != 3.5 {
		t.Error("weight")
	}
	if v, _ := item.Prop("deal_damage"); !mustBool(t, v) {
		t.Error("deal_damage")
	}
	v, _ := item.Prop("initial")
	c, err := v.Char()
	if err != nil || c != 'S' {
		t.Errorf("initial = %q, %v", c, err)
	}
	v, _ = item.Prop("col")
	col, err := v.Color()
	if err != nil || col.R != 255 || col.G != 0 || col.B != 0 {
		t.Errorf("col = %+v, %v", col, err)
	}
	v, _ = item.Prop("damages")
	d, err := v.Dice()
	if err != nil || d.Rolls != 3 || d.Faces != 6 || d.AddSub != 2 {
		t.Errorf("damages = %+v, %v", d, err)
	}
}

func TestColorFormats(t *testing.T) {
	for _, tc := range []struct {
		in         string
		r, g, b    uint8
		shouldFail bool
	}{
		{"255,0,0", 255, 0, 0, false},
		{"#ff8000", 255, 128, 0, false},
		{" 10 , 20 , 30 ", 10, 20, 30, false},
		{"300,0,0", 0, 0, 0, true},
		{"#ff80", 0, 0, 0, true},
		{"red", 0, 0, 0, true},
	} {
		got, err := Value{Raw: tc.in}.Color()
		if tc.shouldFail {
			if err == nil {
				t.Errorf("%q should have failed", tc.in)
			}
			continue
		}
		if err != nil || got.R != tc.r || got.G != tc.g || got.B != tc.b {
			t.Errorf("%q = %+v, %v", tc.in, got, err)
		}
	}
}

func mustInt(t *testing.T, v Value) int {
	t.Helper()
	n, err := v.Int()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustFloat(t *testing.T, v Value) float32 {
	t.Helper()
	f, err := v.Float()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func mustBool(t *testing.T, v Value) bool {
	t.Helper()
	b, err := v.Bool()
	if err != nil {
		t.Fatal(err)
	}
	return b
}
