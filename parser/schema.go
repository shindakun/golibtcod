package parser

// Schema validation: the Go equivalent of libtcod's schema-first parsing.
//
// libtcod's parser requires you to declare struct types and property types
// up front (TCOD_parser_new_struct / TCOD_struct_add_property), then
// validates while it parses: an undeclared property or an unknown struct
// type is an error naming the file and line. golibtcod parses first and
// validates second, which keeps the parser simple and lets callers read
// files they don't have a schema for, but without this layer it would
// silently accept a typo like `syllablesStrat`, which libtcod would catch.
//
// Differences from libtcod that are deliberate:
//
//   - Validate reports EVERY problem, not just the first. libtcod stops
//     at its first fatal error.
//   - A type mismatch keeps the original text in the error message.
//     libtcod substitutes a zero value and moves on.
//   - Validation is optional. Reading a schema-less file is legitimate.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golibtcod/color"
	"golibtcod/rng"
)

// Type mirrors TCOD_value_type_t: the value types a property can declare.
type Type int

const (
	TypeAny Type = iota // accept anything (no golibtcod equivalent in C)
	TypeBool
	TypeChar
	TypeInt
	TypeFloat
	TypeString
	TypeColor
	TypeDice
	TypeList // a bracketed list; element type unchecked
)

func (t Type) String() string {
	switch t {
	case TypeBool:
		return "bool"
	case TypeChar:
		return "char"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeString:
		return "string"
	case TypeColor:
		return "color"
	case TypeDice:
		return "dice"
	case TypeList:
		return "list"
	default:
		return "any"
	}
}

// Property declares one property of a struct type.
type Property struct {
	Type      Type
	Mandatory bool
}

// StructSchema declares one struct type: its properties, its permitted
// bare flags, and which struct types may nest inside it.
type StructSchema struct {
	Properties map[string]Property
	Flags      []string
	Children   []string // permitted child struct types
	// AllowUnknownProperties relaxes the strictest check. Off by default,
	// matching libtcod, because catching typos is the point.
	AllowUnknownProperties bool
}

// Schema maps struct type names to their declarations.
type Schema map[string]StructSchema

// NewSchema returns an empty schema.
func NewSchema() Schema { return Schema{} }

// Declare adds a struct type. Chainable:
//
//	s := parser.NewSchema()
//	s.Declare("name").
//		Prop("phonemesVocals", parser.TypeString, true).
//		Prop("rules", parser.TypeList, true).
//		Flag("deprecated").
//		Child("variant")
func (s Schema) Declare(structType string) *Builder {
	if _, ok := s[structType]; !ok {
		s[structType] = StructSchema{Properties: map[string]Property{}}
	}
	return &Builder{schema: s, name: structType}
}

// Builder is the fluent declaration helper returned by Declare.
type Builder struct {
	schema Schema
	name   string
}

// Prop declares a property.
func (b *Builder) Prop(name string, t Type, mandatory bool) *Builder {
	ss := b.schema[b.name]
	if ss.Properties == nil {
		ss.Properties = map[string]Property{}
	}
	ss.Properties[name] = Property{Type: t, Mandatory: mandatory}
	b.schema[b.name] = ss
	return b
}

// Flag declares a permitted bare flag.
func (b *Builder) Flag(name string) *Builder {
	ss := b.schema[b.name]
	ss.Flags = append(ss.Flags, name)
	b.schema[b.name] = ss
	return b
}

// Child declares a permitted nested struct type.
func (b *Builder) Child(structType string) *Builder {
	ss := b.schema[b.name]
	ss.Children = append(ss.Children, structType)
	b.schema[b.name] = ss
	return b
}

// AllowUnknown permits undeclared properties on this struct type.
func (b *Builder) AllowUnknown() *Builder {
	ss := b.schema[b.name]
	ss.AllowUnknownProperties = true
	b.schema[b.name] = ss
	return b
}

// Done returns the schema, for chaining at the end of a declaration.
func (b *Builder) Done() Schema { return b.schema }

/* --------------------------------------------------------------- errors */

// ValidationError is one problem found during validation. It carries the
// line so callers can report it the way libtcod does.
type ValidationError struct {
	Line     int
	Struct   string // struct type
	Name     string // struct's quoted name, if any
	Property string // property or flag involved, if any
	Msg      string
}

func (e ValidationError) Error() string {
	where := e.Struct
	if e.Name != "" {
		where = fmt.Sprintf("%s %q", e.Struct, e.Name)
	}
	if e.Property != "" {
		return fmt.Sprintf("line %d: %s: %s: %s", e.Line, where, e.Property, e.Msg)
	}
	return fmt.Sprintf("line %d: %s: %s", e.Line, where, e.Msg)
}

// ValidationErrors is the full set, in source order.
type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	if len(v) == 0 {
		return "no validation errors"
	}
	parts := make([]string, len(v))
	for i, e := range v {
		parts[i] = e.Error()
	}
	return fmt.Sprintf("%d validation error(s):\n  %s", len(v), strings.Join(parts, "\n  "))
}

/* ----------------------------------------------------------- validation */

// Validate checks parsed structs against a schema, returning every
// problem found rather than stopping at the first. Returns nil when clean.
func Validate(structs []*Struct, schema Schema) ValidationErrors {
	var errs ValidationErrors
	for _, s := range structs {
		errs = append(errs, validateStruct(s, schema, "")...)
	}
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Line < errs[j].Line })
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateStruct(s *Struct, schema Schema, parentType string) ValidationErrors {
	var errs ValidationErrors
	add := func(prop, msg string) {
		errs = append(errs, ValidationError{
			Line: s.Line, Struct: s.Type, Name: s.Name, Property: prop, Msg: msg,
		})
	}

	ss, declared := schema[s.Type]
	if !declared {
		add("", fmt.Sprintf("unknown struct type %q", s.Type))
		return errs // nothing further is checkable
	}

	// nesting: is this type allowed inside its parent?
	if parentType != "" {
		parent := schema[parentType]
		if !contains(parent.Children, s.Type) {
			add("", fmt.Sprintf("struct type %q is not permitted inside %q", s.Type, parentType))
		}
	}

	// properties present
	for _, name := range s.PropOrder {
		v := s.Properties[name]
		decl, known := ss.Properties[name]
		if !known {
			if !ss.AllowUnknownProperties {
				errs = append(errs, ValidationError{
					Line: v.Line, Struct: s.Type, Name: s.Name, Property: name,
					Msg: fmt.Sprintf("struct type %q does not declare this property", s.Type),
				})
			}
			continue
		}
		if err := checkType(v, decl.Type); err != nil {
			errs = append(errs, ValidationError{
				Line: v.Line, Struct: s.Type, Name: s.Name, Property: name, Msg: err.Error(),
			})
		}
	}

	// mandatory properties absent
	var missing []string
	for name, decl := range ss.Properties {
		if !decl.Mandatory {
			continue
		}
		if _, ok := s.Properties[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing) // map iteration order must not leak into output
	for _, name := range missing {
		add(name, "mandatory property is missing")
	}

	// flags
	for _, f := range s.Flags {
		if !contains(ss.Flags, f) {
			add(f, fmt.Sprintf("struct type %q does not declare this flag", s.Type))
		}
	}

	for _, child := range s.Children {
		errs = append(errs, validateStruct(child, schema, s.Type)...)
	}
	return errs
}

// checkType reports whether a value is convertible to the declared type.
func checkType(v Value, t Type) error {
	switch t {
	case TypeAny:
		return nil
	case TypeList:
		if v.List == nil {
			return fmt.Errorf("expected a list, got %q", v.Raw)
		}
		return nil
	case TypeBool:
		if _, err := v.Bool(); err != nil {
			return fmt.Errorf("expected a bool, got %q", v.Raw)
		}
	case TypeInt:
		if _, err := v.Int(); err != nil {
			return fmt.Errorf("expected an integer, got %q", v.Raw)
		}
	case TypeFloat:
		if _, err := v.Float(); err != nil {
			return fmt.Errorf("expected a float, got %q", v.Raw)
		}
	case TypeChar:
		if _, err := v.Char(); err != nil {
			return fmt.Errorf("expected a char, got %q", v.Raw)
		}
	case TypeColor:
		if _, err := v.Color(); err != nil {
			return fmt.Errorf("expected a color, got %q", v.Raw)
		}
	case TypeDice:
		if _, err := v.Dice(); err != nil {
			return fmt.Errorf("expected dice notation, got %q", v.Raw)
		}
	case TypeString:
		return nil // any text is a valid string
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

/* -------------------------------------------------- typed value getters */

// Char parses a value written as 'x' or as a bare character.
func (v Value) Char() (rune, error) {
	s := strings.TrimSpace(v.Raw)
	if len(s) >= 3 && s[0] == '\'' && s[len(s)-1] == '\'' {
		inner := []rune(s[1 : len(s)-1])
		if len(inner) == 1 {
			return inner[0], nil
		}
		return 0, fmt.Errorf("char literal must hold exactly one character")
	}
	r := []rune(s)
	if len(r) == 1 {
		return r[0], nil
	}
	return 0, fmt.Errorf("not a char literal")
}

// Color parses "#rrggbb" or "r,g,b", matching libtcod's accepted forms.
func (v Value) Color() (color.RGB, error) {
	s := strings.TrimSpace(v.Raw)
	if strings.HasPrefix(s, "#") {
		if len(s) != 7 {
			return color.RGB{}, fmt.Errorf("hex color must be #rrggbb")
		}
		n, err := strconv.ParseUint(s[1:], 16, 32)
		if err != nil {
			return color.RGB{}, fmt.Errorf("bad hex color")
		}
		return color.RGB{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n)}, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return color.RGB{}, fmt.Errorf("color must be #rrggbb or r,g,b")
	}
	var out [3]uint8
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 || n > 255 {
			return color.RGB{}, fmt.Errorf("color component %q out of range", p)
		}
		out[i] = uint8(n)
	}
	return color.RGB{R: out[0], G: out[1], B: out[2]}, nil
}

// Dice parses classic dice notation via the rng package, so the parser and
// the roller can never disagree about what "3d6+2" means.
func (v Value) Dice() (rng.Dice, error) {
	s := strings.TrimSpace(v.Raw)
	d := rng.ParseDice(s)
	if d.Rolls <= 0 || d.Faces <= 0 {
		return d, fmt.Errorf("not dice notation")
	}
	return d, nil
}
