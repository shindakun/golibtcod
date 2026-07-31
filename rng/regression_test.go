package rng

import (
	"testing"
	"time"
)

// atoiPrefix parsed leading digits only. C's atoi also skips whitespace,
// accepts a sign, and returns a 32-bit int; without that, " 3d6" silently
// rolled nothing and "99999999999d6" produced a 64-bit roll count.
func TestParseDiceMatchesAtoiSemantics(t *testing.T) {
	cases := []struct {
		in           string
		rolls, faces int
	}{
		{"3d6", 3, 6},
		{" 3d6", 3, 6},
		{"\t3d6", 3, 6},
		{"  12d6", 12, 6},
		{"-2d6", -2, 6},
		{"+5d6", 5, 6},
		{"1d20", 1, 20},
		{"abc", 0, 0},
		{"", 0, 0},
		// C's atoi is 32-bit: 99999999999 wraps to 1215752191.
		{"99999999999d6", 1215752191, 6},
	}
	for _, c := range cases {
		d := ParseDice(c.in)
		if d.Rolls != c.rolls || d.Faces != c.faces {
			t.Errorf("ParseDice(%q) = {Rolls:%d Faces:%d}, want {Rolls:%d Faces:%d}",
				c.in, d.Rolls, d.Faces, c.rolls, c.faces)
		}
	}
}

func TestParseDiceModifiers(t *testing.T) {
	if d := ParseDice("1d20+5"); d.Rolls != 1 || d.Faces != 20 || d.AddSub != 5 {
		t.Errorf("1d20+5 -> %+v", d)
	}
	if d := ParseDice("2d10-3"); d.Rolls != 2 || d.Faces != 10 || d.AddSub != -3 {
		t.Errorf("2d10-3 -> %+v", d)
	}
}

// A huge but C-faithful roll count must not hang the process.
func TestRollClampsAbsurdCounts(t *testing.T) {
	d := ParseDice("99999999999d6")
	if d.Rolls != 1215752191 {
		t.Fatalf("expected C's 32-bit truncation, got %d", d.Rolls)
	}
	r := New(MT, 42)
	done := make(chan int, 1)
	go func() { done <- r.Roll(d) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Roll did not terminate; MaxRolls clamp is not working")
	}
}

// Ordinary dice must be untouched by the clamp.
func TestRollNormalDiceInRange(t *testing.T) {
	r := New(MT, 7)
	for i := 0; i < 200; i++ {
		if got := r.Roll(ParseDice("3d6")); got < 3 || got > 18 {
			t.Fatalf("3d6 rolled %d, outside [3,18]", got)
		}
		if got := r.Roll(ParseDice("1d20+5")); got < 6 || got > 25 {
			t.Fatalf("1d20+5 rolled %d, outside [6,25]", got)
		}
	}
}
