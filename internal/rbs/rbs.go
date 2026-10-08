// Package rbs parses the subset of RBS type syntax that rb2go understands.
package rbs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Type is an RBS type. Names are unresolved: a bare identifier may be a
// class, a type variable or an alias; the compiler decides.
// Type is a sealed sum type (decision 119): only the members below
// implement isType, and gochecksumtype makes every type switch on it list
// all of them.
//
//sumtype:decl
type Type interface {
	String() string
	isType()
}

func (Name) isType()      {}
func (Optional) isType()  {}
func (Tuple) isType()     {}
func (Union) isType()     {}
func (Self) isType()      {}
func (Void) isType()      {}
func (Bot) isType()       {}
func (Nil) isType()       {}
func (Untyped) isType()   {}
func (Bool) isType()      {}
func (Singleton) isType() {}
func (Proc) isType()      {}
func (Literal) isType()   {}
func (Record) isType()    {}

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
		Self   Type // `^() [self: C] -> R`: what `self` is inside it, or nil
	}
	// Literal is a literal type: `"foo"`, `:foo` or `42`. Kind is the base
	// class the value is a member of ("String", "Symbol" or "Integer"); Value
	// is the decoded text. Used to select a signature by a literal argument
	// (decision 12).
	Literal struct {
		Kind  string
		Value string
	}
	// Record is `{ "a" => T, b: U }`: a Hash-shaped type whose keys are known.
	Record struct{ Fields []RecordField }
)

// RecordField is one field of a Record.
type RecordField struct {
	Key    string
	Symbol bool // `b: U` (a symbol key) rather than `"b" => U`
	Value  Type
}

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

func (t Literal) String() string {
	switch t.Kind {
	case "String":
		return strconv.Quote(t.Value)
	case "Symbol":
		return ":" + t.Value
	default:
		return t.Value
	}
}

func (t Record) String() string {
	parts := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		if f.Symbol {
			parts[i] = f.Key + ": " + f.Value.String()
		} else {
			parts[i] = strconv.Quote(f.Key) + " => " + f.Value.String()
		}
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

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
	Optional bool // `?T`, or `?name: T` for a keyword
	Rest     bool // `*T`
	Keyword  bool // `name: T`: Name is the keyword
	KwRest   bool // `**T`: the other keywords, a Hash[Symbol, T]
}

// Block is `{ (A) -> R }`.
type Block struct {
	Params   []Param
	Return   Type
	Optional bool // `?{ ... }`
	Self     Type // `[self: C]`: what `self` is inside the block, or nil
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
		switch {
		case p.Rest:
			b.WriteString("*")
		case p.KwRest:
			b.WriteString("**")
		}
		if p.Optional {
			b.WriteString("?")
		}
		if p.Keyword {
			b.WriteString(p.Name + ": " + p.Type.String())
			continue
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
		b.WriteString(")")
		if m.Block.Self != nil {
			b.WriteString(" [self: " + m.Block.Self.String() + "]")
		}
		b.WriteString(" -> " + m.Block.Return.String() + " }")
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
		case c == '"':
			tok, ni, serr := scanString(s, i)
			if serr != nil {
				return nil, serr
			}
			toks = append(toks, tok)
			i = ni
		case isNumberStart(s, i):
			tok, ni := scanNumber(s, i)
			toks = append(toks, tok)
			i = ni
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			toks = append(toks, "**")
			i += 2
		case c == ':' && i+1 < len(s) && s[i+1] == ':':
			// leading `::Foo`
			toks = append(toks, "::")
			i += 2
		case c == '=' && i+1 < len(s) && s[i+1] == '>':
			toks = append(toks, "=>")
			i += 2
		case strings.IndexByte("()[]{},|?*:&^", c) >= 0:
			toks = append(toks, string(c))
			i++
		case isIdentStart(c):
			tok, ni, ierr := scanIdent(s, i)
			if ierr != nil {
				return nil, ierr
			}
			toks = append(toks, tok)
			i = ni
		default:
			return nil, fmt.Errorf("rbs: unexpected %q in %q", string(c), s)
		}
	}
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isNumberStart reports whether an integer literal starts at i.
func isNumberStart(s string, i int) bool {
	c := s[i]
	return c >= '0' && c <= '9' || c == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'
}

// scanIdent reads one identifier or `A::B::C` name token.
func scanIdent(s string, i int) (string, int, error) {
	j := i
	for j < len(s) && (isIdentStart(s[j]) || s[j] >= '0' && s[j] <= '9') {
		j++
	}
	for j+1 < len(s) && s[j] == ':' && s[j+1] == ':' {
		k := j + 2
		if k == len(s) || !isIdentStart(s[k]) {
			return "", 0, fmt.Errorf("rbs: expected a name after \"::\" in %q", s)
		}
		for k < len(s) && (isIdentStart(s[k]) || s[k] >= '0' && s[k] <= '9') {
			k++
		}
		j = k
	}
	return s[i:j], j, nil
}

// scanString reads a `"..."` literal (with backslash escapes) as one token.
func scanString(s string, i int) (string, int, error) {
	j := i + 1
	for j < len(s) && s[j] != '"' {
		if s[j] == '\\' {
			j++
		}
		j++
	}
	if j >= len(s) {
		return "", 0, fmt.Errorf("rbs: unterminated string in %q", s)
	}
	return s[i : j+1], j + 1, nil
}

// scanNumber reads an integer literal (with an optional sign and `_`).
func scanNumber(s string, i int) (string, int) {
	j := i
	if s[j] == '-' {
		j++
	}
	for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '_') {
		j++
	}
	return s[i:j], j
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
		m.Block, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
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
		case "**":
			p.next()
			prm.KwRest = true
		case "?":
			p.next()
			prm.Optional = true
		case "&":
			return nil, fmt.Errorf("rbs: %q parameters are not supported", p.peek())
		}
		if tok := p.peek(); !prm.Rest && !prm.KwRest && tok != "" && isIdentStart(tok[0]) && p.pos+1 < len(p.toks) && p.toks[p.pos+1] == ":" {
			prm.Keyword, prm.Name = true, p.next()
			p.next() // :
		}
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		prm.Type = t
		if tok := p.peek(); !prm.Keyword && tok != "" && tok != "," && tok != ")" && isIdentStart(tok[0]) {
			prm.Name = p.next()
			if p.peek() == ":" {
				return nil, errors.New("rbs: a keyword parameter is spelt `name: T`")
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
		return p.parseTuple()
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
	case "true", "false":
		return Bool{}, nil
	case ":":
		name := p.next()
		if name == "" || !isIdentStart(name[0]) {
			return nil, fmt.Errorf("rbs: bad symbol literal near %q", name)
		}
		return Literal{Kind: "Symbol", Value: name}, nil
	case "{":
		return p.parseRecord()
	case "untyped", "top":
		return Untyped{}, nil
	case "bool", "boolish":
		return Bool{}, nil
	case "^":
		return p.parseProc()
	case "singleton":
		return p.parseSingleton()
	}
	if strings.HasPrefix(tok, `"`) {
		v, err := strconv.Unquote(tok)
		if err != nil {
			return nil, fmt.Errorf("rbs: bad string literal %q", tok)
		}
		return Literal{Kind: "String", Value: v}, nil
	}
	if tok[0] >= '0' && tok[0] <= '9' || tok[0] == '-' {
		return Literal{Kind: "Integer", Value: tok}, nil
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

// parseTuple reads a tuple type after its `[`.
func (p *parser) parseTuple() (Type, error) {
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
	p.next() // ]
	return Tuple{Elems: elems}, nil
}

// parseSingleton reads `singleton(Foo)` after its `singleton` token.
func (p *parser) parseSingleton() (Type, error) {
	if p.peek() != "(" {
		return nil, fmt.Errorf("rbs: expected \"(\" after singleton, got %q", p.peek())
	}
	p.next() // (
	name := p.next()
	if name == "::" {
		name = p.next()
	}
	if name == "" || !isIdentStart(name[0]) {
		return nil, fmt.Errorf("rbs: bad singleton type near %q", name)
	}
	err := p.expect(")")
	if err != nil {
		return nil, err
	}
	return Singleton{Name: name}, nil
}

func (t Proc) String() string {
	self := ""
	if t.Self != nil {
		self = " [self: " + t.Self.String() + "]"
	}
	return "^(" + join(t.Params) + ")" + self + " -> " + t.Ret.String()
}

// parseBlock reads a block type after the `?`/`{` that starts it.
func (p *parser) parseBlock() (*Block, error) {
	blk := &Block{Optional: p.peek() == "?"}
	if blk.Optional {
		p.next()
	}
	err := p.expect("{")
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
	blk.Self, err = p.parseSelfBracket()
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
	return blk, nil
}

// parseSelfBracket reads an optional `[self: C]` (a block's self type).
func (p *parser) parseSelfBracket() (Type, error) {
	if p.peek() != "[" {
		return nil, nil //nolint:nilnil // no bracket: the block or proc keeps its lexical self
	}
	p.next() // [
	tok := p.next()
	if tok != "self" {
		return nil, fmt.Errorf("rbs: expected `self: Type`, got %q", tok)
	}
	err := p.expect(":")
	if err != nil {
		return nil, err
	}
	t, err := p.parseType()
	if err != nil {
		return nil, err
	}
	err = p.expect("]")
	if err != nil {
		return nil, err
	}
	return t, nil
}

// parseRecord reads a record type after its `{`: `{ "a" => T, b: U }`.
func (p *parser) parseRecord() (Type, error) {
	var out Record
	for p.peek() != "}" {
		if p.peek() == "" {
			return nil, errors.New("rbs: unterminated record type")
		}
		var f RecordField
		switch {
		case strings.HasPrefix(p.peek(), `"`):
			tok := p.next()
			v, err := strconv.Unquote(tok)
			if err != nil {
				return nil, fmt.Errorf("rbs: bad record key %q", tok)
			}
			f.Key = v
			err = p.expect("=>")
			if err != nil {
				return nil, err
			}
		case p.pos+1 < len(p.toks) && p.toks[p.pos+1] == ":":
			f.Key, f.Symbol = p.next(), true
			p.next() // :
		default:
			return nil, fmt.Errorf("rbs: bad record key %q", p.peek())
		}
		val, err := p.parseType()
		if err != nil {
			return nil, err
		}
		f.Value = val
		out.Fields = append(out.Fields, f)
		err = p.sep("}")
		if err != nil {
			return nil, err
		}
	}
	p.next() // }
	return out, nil
}

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
	self, err := p.parseSelfBracket()
	if err != nil {
		return nil, err
	}
	out.Self = self
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
