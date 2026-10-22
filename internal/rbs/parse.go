package rbs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type parser struct {
	toks  []string
	pos   int
	depth int
	wraps int // `?` suffixes so far: tree height is at most nesting + wraps (<= 2*maxDepth)
}

// maxDepth bounds type nesting: the parser and every consumer recurse on it, and a Go stack overflow is fatal.
const maxDepth = 200

var errTooDeep = fmt.Errorf("rbs: type nested too deeply (limit %d)", maxDepth)

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
			prm.Kind = ParamRest
		case "**":
			p.next()
			prm.Kind = ParamKwRest
		case "?":
			p.next()
			prm.Optional = true
		case "&":
			return nil, fmt.Errorf("rbs: %q parameters are not supported", p.peek())
		}
		if tok := p.peek(); prm.Kind == ParamPositional && tok != "" && isIdentStart(tok[0]) && p.pos+1 < len(p.toks) && p.toks[p.pos+1] == ":" {
			prm.Kind, prm.Name = ParamKeyword, p.next()
			p.next() // :
		}
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		prm.Type = t
		if name := varName(p.peek()); prm.Kind == ParamKeyword && name != "" {
			p.next() // `name: T var`: the variable name is documentation
		} else if name != "" {
			prm.Name = name
			p.next()
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
	first, err := p.parseIntersection()
	if err != nil {
		return nil, err
	}
	if p.peek() != "|" {
		return first, nil
	}
	elems := []Type{first}
	for p.peek() == "|" {
		p.next()
		t, err := p.parseIntersection()
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

// parseIntersection reads `A & B`, which binds tighter than `|`.
func (p *parser) parseIntersection() (Type, error) {
	first, err := p.parseSuffixed()
	if err != nil || p.peek() != "&" {
		return first, err
	}
	elems := []Type{first}
	for p.peek() == "&" {
		p.next()
		t, err := p.parseSuffixed()
		if err != nil {
			return nil, err
		}
		elems = append(elems, t)
	}
	return Intersection{Elems: elems}, nil
}

func (p *parser) parseSuffixed() (Type, error) {
	t, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.peek() == "?" { // each wrap is a tree level, found after the nested type returned: so bound them in total, not per level
		p.wraps++
		if p.wraps > maxDepth {
			return nil, errTooDeep
		}
		p.next()
		t = Optional{Elem: t}
	}
	return t, nil
}

func (p *parser) parsePrimary() (Type, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return nil, errTooDeep
	}
	return p.parseAtom()
}

func (p *parser) parseAtom() (Type, error) {
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
		return p.parseAbsolute()
	case "self":
		return Self{}, nil
	case "instance":
		return Instance{}, nil
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
		return Literal{Kind: LitSymbol, Value: name}, nil
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
		return Literal{Kind: LitString, Value: v}, nil
	}
	if tok[0] >= '0' && tok[0] <= '9' || tok[0] == '-' {
		return Literal{Kind: LitInteger, Value: tok}, nil
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

// parseAbsolute reads the type after a leading `::`: a name in it resolves from the top level.
func (p *parser) parseAbsolute() (Type, error) {
	t, err := p.parsePrimary()
	if n, ok := t.(Name); ok {
		n.Absolute = true
		return n, err
	}
	return t, err
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
		if varName(p.peek()) != "" {
			p.next() // `^(String chunk) -> void`: the name is documentation
		}
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
