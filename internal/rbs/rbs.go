// Package rbs parses the subset of RBS type syntax that rugaroo understands.
package rbs

import (
	"fmt"
)

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
			if p.peek() == "<" { // a bound (`[P < Pointer]`) is not modelled
				p.next()
				_, err = p.parseType()
				if err != nil {
					return nil, err
				}
			}
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
	if p.peek() != "?" && p.peek() != "{" && p.peek() != "->" { // `-> T` and `{ ... } -> T` take no parameters
		err = p.expect("(")
		if err != nil {
			return nil, err
		}
		m.Params, err = p.parseParams()
		if err != nil {
			return nil, err
		}
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
