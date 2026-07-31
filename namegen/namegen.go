// Package namegen is a faithful Go port of libtcod's namegen_c.c: the
// syllable-set name generator, including its rule wildcards and the
// "rubbish pruning" rejection filters.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
//
// The C version keeps a process-global generator registry and reads files
// through the global parser; golibtcod scopes that to an explicit *Registry
// so multiple sets can coexist. The generation algorithm itself,
// tokenizing, wildcard evaluation, chance rolls, and the reject-and-retry
// loop, is ported exactly, including rng call order.
package namegen

import (
	"fmt"
	"strings"

	"golibtcod/parser"
	"golibtcod/rng"
)

// Generator is one syllable set (namegen_t).
type Generator struct {
	Name string

	vocals     []string
	consonants []string
	pre        []string
	start      []string
	middle     []string
	end        []string
	post       []string
	illegal    []string
	rules      []string

	rand *rng.Random
}

// Registry holds parsed generators by name (the C global list).
type Registry struct {
	gens map[string]*Generator
	rand *rng.Random
}

func NewRegistry(r *rng.Random) *Registry {
	return &Registry{gens: map[string]*Generator{}, rand: r}
}

// Parse reads syllable-set definitions from libtcod .cfg content and
// registers them (TCOD_namegen_parse). Existing names are not replaced,
// matching the C behavior.
func (reg *Registry) Parse(src string) error {
	structs, err := parser.Parse(src)
	if err != nil {
		return err
	}
	for _, s := range structs {
		if s.Type != "name" {
			continue
		}
		if s.Name == "" {
			return fmt.Errorf("namegen: a name struct is missing its quoted set name")
		}
		if _, exists := reg.gens[s.Name]; exists {
			continue // C: only the first definition wins
		}
		g := &Generator{Name: s.Name, rand: reg.rand}
		g.vocals = populate(s.PropString("phonemesVocals"), false)
		g.consonants = populate(s.PropString("phonemesConsonants"), false)
		g.pre = populate(s.PropString("syllablesPre"), false)
		g.start = populate(s.PropString("syllablesStart"), false)
		g.middle = populate(s.PropString("syllablesMiddle"), false)
		g.end = populate(s.PropString("syllablesEnd"), false)
		g.post = populate(s.PropString("syllablesPost"), false)
		// illegal strings are lowercased in C
		g.illegal = populate(strings.ToLower(s.PropString("illegal")), false)
		if v, ok := s.Prop("rules"); ok {
			if v.List != nil {
				for _, item := range v.List {
					g.rules = append(g.rules, populate(item, true)...)
				}
			} else {
				g.rules = populate(v.Raw, true)
			}
		}
		reg.gens[s.Name] = g
	}
	return nil
}

// Get returns a registered generator.
func (reg *Registry) Get(name string) (*Generator, bool) {
	g, ok := reg.gens[name]
	return g, ok
}

// Sets lists registered set names (TCOD_namegen_get_sets).
func (reg *Registry) Sets() []string {
	out := make([]string, 0, len(reg.gens))
	for name := range reg.gens {
		out = append(out, name)
	}
	return out
}

// populate is namegen_populate_list: tokenizes a space/punctuation
// separated source string. Letters, apostrophe and hyphen accumulate;
// '/' escapes the next char (keeping the slash when wildcards are on);
// '_' becomes a space (or is kept as a wildcard marker); '$', '%' and
// digits are kept only when wildcards are allowed; anything else ends
// the current token.
func populate(source string, wildcards bool) []string {
	var list []string
	var token strings.Builder
	flush := func() {
		if token.Len() > 0 {
			list = append(list, token.String())
			token.Reset()
		}
	}
	for i := 0; i < len(source); i++ {
		c := source[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '\'' || c == '-':
			token.WriteByte(c)
		case c == '/':
			if wildcards {
				token.WriteByte(c)
				if i+1 < len(source) {
					i++
					token.WriteByte(source[i])
				}
			} else if i+1 < len(source) {
				i++
				token.WriteByte(source[i])
			}
		case c == '_':
			if wildcards {
				token.WriteByte(c)
			} else {
				token.WriteByte(' ')
			}
		case wildcards && (c == '$' || c == '%' || (c >= '0' && c <= '9')):
			token.WriteByte(c)
		default:
			flush()
		}
	}
	flush()
	return list
}

/* --- rubbish pruning (exact ports) --- */

// hasTriples is namegen_word_has_triples: three identical letters in a row.
func hasTriples(s string) bool {
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	c := lower[0]
	cnt := 1
	for i := 1; i < len(lower); i++ {
		if lower[i] == c {
			cnt++
		} else {
			cnt = 1
			c = lower[i]
		}
		if cnt >= 3 {
			return true
		}
	}
	return false
}

// hasIllegal is namegen_word_has_illegal.
func (g *Generator) hasIllegal(s string) bool {
	haystack := strings.ToLower(s)
	for _, bad := range g.illegal {
		if strings.Contains(haystack, bad) {
			return true
		}
	}
	return false
}

// pruneSpaces is namegen_word_prune_spaces: strips leading/trailing
// spaces and collapses doubles.
func pruneSpaces(s string) string {
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.Trim(s, " ")
}

// pruneSyllables is namegen_word_prune_syllables: rejects 2-char direct
// repetitions and any 3-char repetition later in the word ("Arnarn").
func pruneSyllables(s string) bool {
	data := strings.ToLower(s)
	n := len(data)
	for i := 0; i < n-4; i++ {
		check := data[i:i+2] + data[i:i+2]
		if strings.Contains(data, check) {
			return true
		}
	}
	for i := 0; i < n-6; i++ {
		check := data[i : i+3]
		if strings.Contains(data[i+3:], check) {
			return true
		}
	}
	return false
}

// wordIsOK is namegen_word_is_ok (note: C prunes spaces in place first).
func (g *Generator) wordIsOK(s string) (string, bool) {
	s = pruneSpaces(s)
	return s, len(s) > 0 && !hasTriples(s) && !g.hasIllegal(s) && !pruneSyllables(s)
}

/* --- generation --- */

// GenerateCustom is TCOD_namegen_generate_custom: builds a name from an
// explicit rule string, retrying until the result passes the filters.
func (g *Generator) GenerateCustom(rule string) (string, error) {
	for attempts := 0; ; attempts++ {
		if attempts > 10000 {
			return "", fmt.Errorf("namegen: rule %q never produced an acceptable name", rule)
		}
		var buf strings.Builder
		for i := 0; i <= len(rule); i++ {
			if i == len(rule) {
				break
			}
			c := rule[i]
			switch {
			case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '\'' || c == '-':
				buf.WriteByte(c)
			case c == '/':
				if i+1 < len(rule) {
					i++
					buf.WriteByte(rule[i])
				}
			case c == '_':
				buf.WriteByte(' ')
			case c == '$':
				i++
				chance := 100
				if i < len(rule) && rule[i] >= '0' && rule[i] <= '9' {
					chance = 0
					for i < len(rule) && rule[i] >= '0' && rule[i] <= '9' {
						chance = chance*10 + int(rule[i]-'0')
						i++
					}
				}
				if i >= len(rule) {
					return "", fmt.Errorf("namegen: rule %q ends with a bare wildcard", rule)
				}
				// the chance roll happens before the list lookup, as in C
				if chance >= g.rand.GetInt(0, 100) {
					var lst []string
					switch rule[i] {
					case 'P':
						lst = g.pre
					case 's':
						lst = g.start
					case 'm':
						lst = g.middle
					case 'e':
						lst = g.end
					case 'p':
						lst = g.post
					case 'v':
						lst = g.vocals
					case 'c':
						lst = g.consonants
					case '?':
						if g.rand.GetInt(0, 1) == 0 {
							lst = g.vocals
						} else {
							lst = g.consonants
						}
					default:
						return "", fmt.Errorf("namegen: bad wildcard %q in rule %q", string(rule[i]), rule)
					}
					if len(lst) > 0 {
						buf.WriteString(lst[g.rand.GetInt(0, len(lst)-1)])
					}
				}
			}
		}
		if s, ok := g.wordIsOK(buf.String()); ok {
			return s, nil
		}
	}
}

// Generate is TCOD_namegen_generate: picks one of the set's rules
// (honoring "%NN" chance prefixes) and runs it.
func (g *Generator) Generate() (string, error) {
	if len(g.rules) == 0 {
		return "", fmt.Errorf("namegen: set %q has no rules", g.Name)
	}
	var rule string
	for {
		ruleRolled := g.rules[g.rand.GetInt(0, len(g.rules)-1)]
		chance := 100
		truncation := 0
		if len(ruleRolled) > 0 && ruleRolled[0] == '%' {
			truncation = 1
			chance = 0
			for truncation < len(ruleRolled) && ruleRolled[truncation] >= '0' && ruleRolled[truncation] <= '9' {
				chance = chance*10 + int(ruleRolled[truncation]-'0')
				truncation++
			}
		}
		if g.rand.GetInt(0, 100) <= chance {
			rule = ruleRolled[truncation:]
			break
		}
	}
	return g.GenerateCustom(rule)
}
