// Package rbs parses the subset of RBS type syntax that rb2go understands.
package rbs

import (
	"errors"
	"fmt"
	"strings"
)

// Type is an RBS type. Names are unresolved: a bare identifier may be a
// class, a type variable or an alias; the compiler decides.
type Type interface{ String() string }

type (
	// Name is `Foo` or `Foo[A, B]`.
	Name struct {
		Name string
		Args []Type
	}
	// Optional is `T?`.
	Optional struct{ Elem Type }
	// Tuple is `[A, B]`.
	Tuple struct{ Elems []Type }
	// Union is `A | B`. Unions with nil are normalised to Optional.
	Union struct{ Elems []Type }
	// Self is `self`.
	Self struct{}
	// Void is `void`.
	Void struct{}
	Bot  struct{} // never returns: raises or throws
	// Nil is `nil`.
	Nil struct{}
	// Untyped is `untyped`.
	Untyped struct{}
	// Bool is `bool`.
	Bool struct{}
	// Singleton is `singleton(Foo)`: the class object Foo.
	Singleton struct{ Name string }
	// Proc is `^(A, B) -> R`.
	Proc struct {
		Params []Type
		Ret    Type
	}
)

func (t Name) String() string {
	if len(t.Args) == 0 {
		return t.Name
	}
	return t.Name + "[" + join(t.Args) + "]"
}
func (t Optional) String() string { return t.Elem.String() + "?" }
func (t Tuple) String() string    { return "[" + join(t.Elems) + "]" }
func (t Union) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return strings.Join(parts, " | ")
}
func (Self) String() string        { return "self" }
func (Void) String() string        { return "void" }
func (Bot) String() string         { return "bot" }
func (Nil) String() string         { return "nil" }
func (Untyped) String() string     { return "untyped" }
func (Bool) String() string        { return "bool" }
func (t Singleton) String() string { return "singleton(" + t.Name + ")" }

func join(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}

// Param is one positional parameter of a method or block type.
type Param struct {
	Type     Type
	Name     string
	Optional bool // `?T`
	Rest     bool // `*T`
}

// Block is `{ (A) -> R }`.
type Block struct {
	Params   []Param
	Return   Type
	Optional bool // `?{ ... }`
}

// MethodType is `[X] (A, B) { (C) -> D } -> R`.
type MethodType struct {
	TypeParams []string
	Params     []Param
	Block      *Block
	Return     Type
}

func (m *MethodType) String() string {
	var b strings.Builder
	if len(m.TypeParams) > 0 {
		b.WriteString("[" + strings.Join(m.TypeParams, ", ") + "] ")
	}
	b.WriteString("(")
	for i, p := range m.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		if p.Rest {
			b.WriteString("*")
		}
		if p.Optional {
			b.WriteString("?")
		}
		b.WriteString(p.Type.String())
		if p.Name != "" {
			b.WriteString(" " + p.Name)
		}
	}
	b.WriteString(")")
	if m.Block != nil {
		b.WriteString(" { (")
		for i, p := range m.Block.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Type.String())
		}
		b.WriteString(") -> " + m.Block.Return.String() + " }")
	}
	b.WriteString(" -> " + m.Return.String())
	return b.String()
}

type parser struct {
	toks []string
	pos  int
}

func tokenize(s string) ([]string, error) {
	var toks []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '-' && i+1 < len(s) && s[i+1] == '>':
			toks = append(toks, "->")
			i += 2
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			toks = append(toks, "**")
			i += 2
		case c == ':' && i+1 < len(s) && s[i+1] == ':':
			// leading `::Foo`
			toks = append(toks, "::")
			i += 2
		case strings.IndexByte("()[]{},|?*:&^", c) >= 0:
			toks = append(toks, string(c))
			i++
		case isIdentStart(c):
			j := i
			for j < len(s) && (isIdentStart(s[j]) || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			// A::B::C is one name token.
			for j+1 < len(s) && s[j] == ':' && s[j+1] == ':' {
				k := j + 2
				if k == len(s) || !isIdentStart(s[k]) {
					return nil, fmt.Errorf("rbs: expected a name after \"::\" in %q", s)
				}
				for k < len(s) && (isIdentStart(s[k]) || (s[k] >= '0' && s[k] <= '9')) {
					k++
				}
				j = k
			}
			toks = append(toks, s[i:j])
			i = j
		default:
			return nil, fmt.Errorf("rbs: unexpected %q in %q", string(c), s)
		}
	}
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (p *parser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}
func (p *parser) next() string {
	t := p.peek()
	p.pos++
	return t
}
func (p *parser) expect(t string) error {
	if got := p.next(); got != t {
		return fmt.Errorf("rbs: expected %q, got %q", t, got)
	}
	return nil
}

// sep consumes the `,` after a list element, or stops before `closer`.
func (p *parser) sep(closer string) error {
	switch p.peek() {
	case ",":
		p.next()
		return nil
	case closer:
		return nil
	case "":
		return fmt.Errorf("rbs: unterminated list, expected %q", closer)
	}
	return fmt.Errorf("rbs: expected \",\" or %q, got %q", closer, p.peek())
}

// ParseType parses a standalone type, e.g. `Array[String]?`.
func ParseType(s string) (Type, error) {
	toks, err := tokenize(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	t, err := p.parseType()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("rbs: trailing %q in %q", p.peek(), s)
	}
	return t, nil
}

// ParseMethodType parses a method type, e.g. `[X] (A) { (B) -> X } -> C`.
func ParseMethodType(s string) (*MethodType, error) {
	toks, err := tokenize(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	m := &MethodType{}
	if p.peek() == "[" {
		p.next()
		for {
			tp := p.next()
			if tp == "" || !isIdentStart(tp[0]) {
				return nil, fmt.Errorf("rbs: expected a type parameter, got %q", tp)
			}
			m.TypeParams = append(m.TypeParams, tp)
			if p.peek() != "," {
				break
			}
			p.next()
		}
		err = p.expect("]")
		if err != nil {
			return nil, err
		}
	}
	err = p.expect("(")
	if err != nil {
		return nil, err
	}
	m.Params, err = p.parseParams()
	if err != nil {
		return nil, err
	}
	if p.peek() == "?" || p.peek() == "{" {
		blk := &Block{Optional: p.peek() == "?"}
		if blk.Optional {
			p.next()
		}
		err = p.expect("{")
		if err != nil {
			return nil, err
		}
		err = p.expect("(")
		if err != nil {
			return nil, err
		}
		blk.Params, err = p.parseParams()
		if err != nil {
			return nil, err
		}
		err = p.expect("->")
		if err != nil {
			return nil, err
		}
		blk.Return, err = p.parseType()
		if err != nil {
			return nil, err
		}
		err = p.expect("}")
		if err != nil {
			return nil, err
		}
		m.Block = blk
	}
	err = p.expect("->")
	if err != nil {
		return nil, err
	}
	m.Return, err = p.parseType()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("rbs: trailing %q in %q", p.peek(), s)
	}
	return m, nil
}

// parseParams parses after `(` through `)`.
func (p *parser) parseParams() ([]Param, error) {
	var params []Param
	for p.peek() != ")" {
		if p.peek() == "" {
			return nil, errors.New("rbs: unterminated parameter list")
		}
		var prm Param
		switch p.peek() {
		case "*":
			p.next()
			prm.Rest = true
		case "?":
			p.next()
			prm.Optional = true
		case "**", "&":
			return nil, fmt.Errorf("rbs: %q parameters are not supported", p.peek())
		}
		if p.pos+1 < len(p.toks) && p.toks[p.pos+1] == ":" {
			return nil, errors.New("rbs: keyword parameters are not supported")
		}
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		prm.Type = t
		if tok := p.peek(); tok != "" && tok != "," && tok != ")" && isIdentStart(tok[0]) {
			prm.Name = p.next()
			if p.peek() == ":" {
				return nil, errors.New("rbs: keyword parameters are not supported")
			}
		}
		params = append(params, prm)
		err = p.sep(")")
		if err != nil {
			return nil, err
		}
	}
	p.next() // )
	return params, nil
}

func (p *parser) parseType() (Type, error) {
	first, err := p.parseSuffixed()
	if err != nil {
		return nil, err
	}
	if p.peek() != "|" {
		return first, nil
	}
	elems := []Type{first}
	for p.peek() == "|" {
		p.next()
		t, err := p.parseSuffixed()
		if err != nil {
			return nil, err
		}
		elems = append(elems, t)
	}
	// `T | nil` → T?
	var nonNil []Type
	hasNil := false
	for _, e := range elems {
		if _, ok := e.(Nil); ok {
			hasNil = true
		} else {
			nonNil = append(nonNil, e)
		}
	}
	if hasNil && len(nonNil) == 1 {
		return Optional{Elem: nonNil[0]}, nil
	}
	return Union{Elems: elems}, nil
}

func (p *parser) parseSuffixed() (Type, error) {
	t, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peek() == "?" {
		p.next()
		t = Optional{Elem: t}
	}
	return t, nil
}

func (p *parser) parsePrimary() (Type, error) {
	tok := p.next()
	switch tok {
	case "":
		return nil, errors.New("rbs: unexpected end of type")
	case "(":
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		return t, p.expect(")")
	case "[":
		var elems []Type
		for p.peek() != "]" {
			t, err := p.parseType()
			if err != nil {
				return nil, err
			}
			elems = append(elems, t)
			err = p.sep("]")
			if err != nil {
				return nil, err
			}
		}
		p.next()
		return Tuple{Elems: elems}, nil
	case "::":
		return p.parsePrimary()
	case "self":
		return Self{}, nil
	case "void":
		return Void{}, nil
	case "bot":
		return Bot{}, nil
	case "nil":
		return Nil{}, nil
	case "untyped", "top":
		return Untyped{}, nil
	case "bool", "boolish":
		return Bool{}, nil
	case "^":
		return p.parseProc()
	case "singleton":
		if p.peek() == "(" {
			p.next()
			name := p.next()
			if name == "::" {
				name = p.next()
			}
			if name == "" || !isIdentStart(name[0]) {
				return nil, fmt.Errorf("rbs: bad singleton type near %q", name)
			}
			return Singleton{Name: name}, p.expect(")")
		}
		return nil, fmt.Errorf("rbs: expected \"(\" after singleton, got %q", p.peek())
	}
	if !isIdentStart(tok[0]) {
		return nil, fmt.Errorf("rbs: unexpected %q", tok)
	}
	n := Name{Name: tok}
	if p.peek() == "[" {
		p.next()
		for p.peek() != "]" {
			t, err := p.parseType()
			if err != nil {
				return nil, err
			}
			n.Args = append(n.Args, t)
			err = p.sep("]")
			if err != nil {
				return nil, err
			}
		}
		p.next()
	}
	return n, nil
}

func (t Proc) String() string { return "^(" + join(t.Params) + ") -> " + t.Ret.String() }

// parseProc reads a proc type after its `^`: `(A, B) -> R`.
func (p *parser) parseProc() (Type, error) {
	err := p.expect("(")
	if err != nil {
		return nil, err
	}
	var out Proc
	for p.peek() != ")" {
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		out.Params = append(out.Params, t)
		err = p.sep(")")
		if err != nil {
			return nil, err
		}
	}
	p.next()
	err = p.expect("->")
	if err != nil {
		return nil, err
	}
	ret, err := p.parseType()
	if err != nil {
		return nil, err
	}
	out.Ret = ret
	return out, nil
}
