// Package parser reads libtcod's config file format (the format consumed
// by TCOD_parser_run, used for namegen syllable sets and game data).
//
// Unlike the rest of golibtcod, this is a clean-room Go implementation of the
// documented grammar rather than a line-by-line port of lex_c.c/parser_c.c
// (those are built around C's global lexer state and a listener callback
// ABI that would be actively unpleasant in Go). The accepted syntax and
// value semantics match libtcod; the API is idiomatic Go. See
// docs/BUILDLOG.md for the rationale.
//
// Grammar:
//
//	structType "optional name" {
//	    property = value
//	    flag
//	    subStruct { ... }
//	}
//
// Values: quoted strings ("a b c", with \" and \\ escapes), integers,
// floats, booleans (true/false), chars ('x'), colors ("#rrggbb" or
// "r,g,b"), and bracketed lists ([1,2,3] or ["a","b"]).
//
// Comments: // to end of line, /* ... */ blocks, and (as a deliberate
// EXTENSION) # to end of line.
//
// # DIFFERENCES FROM LIBTCOD, verified against the C implementation
//
// These were measured by running a corpus through both parsers; the C
// harness, inputs and reference output are in internal/fixtures/parser.
//
//  1. '#' comments are a golibtcod extension. libtcod's lexer knows only
//     '//' and '/* */' and reports a syntax error on '#'. golibtcod
//     therefore accepts a superset: any file libtcod parses, golibtcod
//     parses, but a golibtcod .cfg using '#' will NOT load in C libtcod.
//  2. libtcod is schema-first: it validates struct types, property
//     names and value types while parsing, and rejects anything
//     undeclared. golibtcod parses first and validates separately; see
//     Schema and Validate in schema.go for the equivalent checking.
//  3. On a malformed file libtcod reports the error and keeps parsing,
//     salvaging what it can. golibtcod stops at the first error.
//  4. On a type mismatch libtcod substitutes a zero value; golibtcod keeps
//     the original text and reports at conversion or validation time.
package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// Struct is one parsed block.
type Struct struct {
	Type       string // the struct type keyword, e.g. "name"
	Name       string // optional quoted name after the type
	Line       int    // 1-based line the struct opens on
	Properties map[string]Value
	PropOrder  []string // property names in source order
	Flags      []string
	Children   []*Struct
}

// Value is a parsed property value.
//
// Values are stored as text and converted on demand. libtcod converts at
// parse time against a declared schema and substitutes a zero on failure;
// deferring means the error arrives with the offending text attached. Use
// the schema layer (schema.go) to check types up front.
type Value struct {
	Raw  string   // original text (unquoted for strings)
	List []string // non-nil for bracketed lists
	Line int      // 1-based line the value appears on
}

// String returns the value as a string.
func (v Value) String() string { return v.Raw }

// Int parses the value as an integer.
func (v Value) Int() (int, error) { return strconv.Atoi(strings.TrimSpace(v.Raw)) }

// Float parses the value as a float32.
func (v Value) Float() (float32, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v.Raw), 32)
	return float32(f), err
}

// Bool parses the value as a boolean.
func (v Value) Bool() (bool, error) { return strconv.ParseBool(strings.TrimSpace(v.Raw)) }

// Prop returns a property by name.
func (s *Struct) Prop(name string) (Value, bool) {
	v, ok := s.Properties[name]
	return v, ok
}

// PropString returns a string property or "" if absent.
func (s *Struct) PropString(name string) string {
	if v, ok := s.Properties[name]; ok {
		return v.Raw
	}
	return ""
}

// HasFlag reports whether a bare flag was set.
func (s *Struct) HasFlag(name string) bool {
	for _, f := range s.Flags {
		if f == name {
			return true
		}
	}
	return false
}

type lexer struct {
	src  string
	pos  int
	line int
}

func (l *lexer) errf(format string, args ...any) error {
	return fmt.Errorf("parser: line %d: %s", l.line, fmt.Sprintf(format, args...))
}

func (l *lexer) skipSpaceAndComments() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*':
			l.pos += 2
			for l.pos+1 < len(l.src) && !(l.src[l.pos] == '*' && l.src[l.pos+1] == '/') {
				if l.src[l.pos] == '\n' {
					l.line++
				}
				l.pos++
			}
			l.pos += 2
		default:
			return
		}
	}
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '-' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (l *lexer) ident() string {
	start := l.pos
	for l.pos < len(l.src) && isIdentChar(l.src[l.pos]) {
		l.pos++
	}
	return l.src[start:l.pos]
}

// quoted reads a "..." string, honoring \" and \\ escapes.
func (l *lexer) quoted() (string, error) {
	if l.src[l.pos] != '"' {
		return "", l.errf("expected a quoted string")
	}
	l.pos++
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '\\':
			if l.pos+1 < len(l.src) {
				l.pos++
				b.WriteByte(l.src[l.pos])
				l.pos++
				continue
			}
			l.pos++
		case '"':
			l.pos++
			return b.String(), nil
		case '\n':
			return "", l.errf("unterminated string")
		default:
			b.WriteByte(c)
			l.pos++
		}
	}
	return "", l.errf("unterminated string")
}

// bareValue reads a single unquoted value token.
//
// This used to read to end of line, which silently swallowed anything that
// followed on the same line: `cost = 1 legendary` produced the value
// "1 legendary" and lost the flag, and `cost = 1 sublist { ... }` ate the
// nested struct. libtcod's lexer is token-based, so an unquoted value is
// exactly one token there; text with spaces must be quoted. Reading one
// token matches that and fixes both bugs.
func (l *lexer) bareValue() string {
	start := l.pos
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\n' || c == '#' || c == '}' || c == '{' ||
			c == ' ' || c == '\t' || c == '\r' {
			break
		}
		if c == '/' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == '/' || l.src[l.pos+1] == '*') {
			break
		}
		l.pos++
	}
	return strings.TrimSpace(l.src[start:l.pos])
}

func (l *lexer) list() (Value, error) {
	l.pos++ // consume '['
	var items []string
	for {
		l.skipSpaceAndComments()
		if l.pos >= len(l.src) {
			return Value{}, l.errf("unterminated list")
		}
		if l.src[l.pos] == ']' {
			l.pos++
			break
		}
		if l.src[l.pos] == ',' {
			l.pos++
			continue
		}
		if l.src[l.pos] == '"' {
			s, err := l.quoted()
			if err != nil {
				return Value{}, err
			}
			items = append(items, s)
			continue
		}
		start := l.pos
		for l.pos < len(l.src) && l.src[l.pos] != ',' && l.src[l.pos] != ']' {
			l.pos++
		}
		items = append(items, strings.TrimSpace(l.src[start:l.pos]))
	}
	return Value{Raw: strings.Join(items, ","), List: items}, nil
}

// Parse reads all top-level structs from src.
func Parse(src string) ([]*Struct, error) {
	l := &lexer{src: src, line: 1}
	var out []*Struct
	for {
		l.skipSpaceAndComments()
		if l.pos >= len(l.src) {
			return out, nil
		}
		s, err := l.parseStruct()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
}

func (l *lexer) parseStruct() (*Struct, error) {
	typ := l.ident()
	if typ == "" {
		return nil, l.errf("expected a struct type, found %q", string(l.src[l.pos]))
	}
	s := &Struct{Type: typ, Line: l.line, Properties: map[string]Value{}}
	l.skipSpaceAndComments()
	if l.pos < len(l.src) && l.src[l.pos] == '"' {
		name, err := l.quoted()
		if err != nil {
			return nil, err
		}
		s.Name = name
		l.skipSpaceAndComments()
	}
	if l.pos >= len(l.src) || l.src[l.pos] != '{' {
		return nil, l.errf("expected '{' after struct %q", typ)
	}
	l.pos++
	for {
		l.skipSpaceAndComments()
		if l.pos >= len(l.src) {
			return nil, l.errf("unterminated struct %q", typ)
		}
		if l.src[l.pos] == '}' {
			l.pos++
			return s, nil
		}
		propLine := l.line
		name := l.ident()
		if name == "" {
			return nil, l.errf("expected a property name in struct %q", typ)
		}
		save := l.pos
		saveLine := l.line
		l.skipSpaceAndComments()
		if l.pos >= len(l.src) {
			return nil, l.errf("unexpected end of file")
		}
		switch l.src[l.pos] {
		case '=':
			l.pos++
			l.skipSpaceAndComments()
			var v Value
			switch {
			case l.src[l.pos] == '"':
				str, err := l.quoted()
				if err != nil {
					return nil, err
				}
				v = Value{Raw: str}
			case l.src[l.pos] == '[':
				lv, err := l.list()
				if err != nil {
					return nil, err
				}
				v = lv
			default:
				v = Value{Raw: l.bareValue()}
			}
			v.Line = propLine
			if _, seen := s.Properties[name]; !seen {
				s.PropOrder = append(s.PropOrder, name)
			}
			s.Properties[name] = v
		case '{', '"':
			// a sub-struct: rewind so parseStruct sees the type name again
			l.pos = save - len(name)
			l.line = saveLine
			child, err := l.parseStruct()
			if err != nil {
				return nil, err
			}
			s.Children = append(s.Children, child)
		default:
			// bare flag
			l.pos = save
			l.line = saveLine
			s.Flags = append(s.Flags, name)
		}
	}
}
