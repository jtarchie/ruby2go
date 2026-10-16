package compiler

import (
	"slices"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

type typeScope struct {
	class     *Class
	lex       []*Class // lexical scope for constant names
	methodTPs []string
	srcPos
}

func (c *Compiler) resolveType(t rbs.Type, sc typeScope) Type {
	switch t := t.(type) {
	case rbs.Name:
		return c.resolveNameType(t, sc)
	case rbs.Bool:
		return TClass{C: c.classes["Boolean"]}
	case rbs.Singleton:
		cls, _ := c.lookupName(sc.lex, t.Name)
		if cls == nil {
			c.errorAt(sc.srcPos, "unknown type %s", t.Name)
		}
		if cls.meta == nil {
			c.errorAt(sc.srcPos, "%s has no class object type", t)
		}
		return TClass{C: cls.meta}
	case rbs.Optional:
		return optOf(c.resolveType(t.Elem, sc))
	case rbs.Tuple:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = c.resolveType(e, sc)
		}
		if (len(elems) < 2 || len(elems) > maxTuple) && c.gemFile(sc.File) {
			return TClass{C: c.classes["Array"], Args: []Type{TAny{}}} // gem code: an Array of untyped
		}
		if len(elems) < 2 || len(elems) > maxTuple {
			c.errorAt(sc.srcPos, "only 2- to %d-tuples are supported", maxTuple)
		}
		return TTuple{Elems: elems}
	case rbs.Self:
		if sc.class == nil {
			c.errorAt(sc.srcPos, "`self` type outside a class")
		}
		return TVar{Name: "Self"}
	case rbs.Instance: // ponytail: the defining class, not the receiver's subclass; make it a Self-like TVar when a subclass call needs it
		if sc.class == nil {
			c.errorAt(sc.srcPos, "`instance` type outside a class")
		}
		if sc.class.metaOf != nil {
			return TClass{C: sc.class.metaOf}
		}
		return TClass{C: sc.class}
	case rbs.Void, rbs.Bot:
		return TVoid{}
	case rbs.Nil:
		return TNil{}
	case rbs.Untyped:
		return TAny{}
	case rbs.Union:
		return c.resolveUnion(t, sc)
	case rbs.Proc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = c.resolveType(p, sc)
		}
		var self Type
		if t.Self != nil { // a Proc instance_eval rebinds (decision 164)
			self = c.resolveType(t.Self, sc)
		}
		return TFunc{Params: ps, Ret: c.resolveType(t.Ret, sc), Proc: true, Self: self}
	case rbs.Literal:
		// A literal is a member of its base class (decision 12); the value
		// matters only to select an overload arm at the call site.
		return TClass{C: c.classes[t.Kind]}
	case rbs.Record:
		return c.resolveRecord(t, sc)
	}
	c.errorAt(sc.srcPos, "unsupported type %s", t)
	return nil
}

// resolveUnion resolves `A | B` (decision 150). Members are what a Go type
// switch can test: classes, tuples, procs and nil.
func (c *Compiler) resolveUnion(t rbs.Union, sc typeScope) Type {
	ms := make([]Type, len(t.Elems))
	for i, e := range t.Elems {
		ms[i] = c.resolveType(e, sc)
		switch m := stripOpt(ms[i]).(type) {
		case TVar, TVoid:
			if c.gemFile(sc.File) {
				return TAny{} // gem code: the union is untyped (decision 172)
			}
			c.errorAt(sc.srcPos, "a union member must be a class, tuple, proc or nil, not %s: %s", m, t)
		case TClass:
			if m.C.IsModule && !m.C.universal { // ponytail: a module member needs an includer check in the type switch; use the module type or untyped until then
				if c.gemFile(sc.File) {
					return TAny{} // gem code: the union is untyped (decision 172)
				}
				c.errorAt(sc.srcPos, "a union member must be a class, not the module %s: %s", m, t)
			}
		case TAny, TFunc, TNil, TOpt, TTuple, TUnion:
		}
	}
	u := unionOf(ms...)
	if tu, ok := u.(TUnion); ok {
		seen := map[string]Type{}
		for _, m := range tu.Members {
			if isNil(m) {
				continue
			}
			g := c.goType(m)
			if o, dup := seen[g]; dup { // a Go type switch cannot tell them apart (Go rejects the duplicate case)
				if c.gemFile(sc.File) {
					return TAny{}
				}
				c.errorAt(sc.srcPos, "union members %s and %s are one Go type (%s): %s", o, m, g, t)
			}
			seen[g] = m
		}
	}
	return u
}

// resolveNameType resolves a `Foo` / `Foo[A]` type: a type variable or a class.
func (c *Compiler) resolveNameType(t rbs.Name, sc typeScope) Type {
	for _, p := range sc.methodTPs {
		if p == t.Name {
			return TVar{Name: p}
		}
	}
	if sc.class != nil {
		for _, p := range sc.class.TypeParams {
			if p == t.Name {
				return TVar{Name: p}
			}
		}
	}
	lex := sc.lex
	if t.Absolute { // `::Pathname` inside a module that has its own Pathname
		lex = nil
	}
	cls, k := c.lookupName(lex, t.Name)
	if cls == nil && k != nil && k.typeAlias != "" && len(t.Args) == 0 {
		at, err := rbs.ParseType(k.typeAlias)
		if err != nil {
			c.errorAt(k.srcPos, "%v", err)
		}
		return c.resolveType(at, typeScope{lex: k.Scope, srcPos: srcPos{k.File, k.Line}})
	}
	if cls == nil && c.gemFile(sc.File) {
		return TAny{} // a lazily-checked sig naming a class outside the closed world: nothing compiled can hold one
	}
	if cls == nil {
		c.errorAt(sc.srcPos, "unknown type %s", t.Name)
	}
	if len(t.Args) == 0 && (cls.RubyName == "Method" || cls.RubyName == "UnboundMethod") {
		return TClass{C: cls, Args: []Type{TAny{}}} // RBS's Method has no signature: called through dyn (decision 141)
	}
	if len(t.Args) > 0 && len(cls.TypeParams) == 0 && c.gemFile(sc.File) {
		t.Args = nil // a Sorbet generic (type_member), erased to untyped (decision 196): its args go too
	}
	if len(t.Args) == 0 && len(cls.TypeParams) > 0 && c.gemFile(sc.File) {
		return TClass{C: cls, Args: slices.Repeat([]Type{TAny{}}, len(cls.TypeParams))} // `Hash` with no args: untyped elements
	}
	if len(t.Args) != len(cls.TypeParams) {
		c.errorAt(sc.srcPos, "%s takes %d type args, got %d", cls.Name, len(cls.TypeParams), len(t.Args))
	}
	args := make([]Type, len(t.Args))
	for i, a := range t.Args {
		args[i] = c.resolveType(a, sc)
	}
	return TClass{C: cls, Args: args}
}

// resolveRecord resolves a record type to a Hash when every key and value is
// uniform, else Hash[untyped, untyped] (decision 160).
func (c *Compiler) resolveRecord(t rbs.Record, sc typeScope) Type {
	hash := c.classes["Hash"]
	anyHash := TClass{C: hash, Args: []Type{TAny{}, TAny{}}}
	if len(t.Fields) == 0 {
		return anyHash
	}
	var kt, vt Type
	for i, f := range t.Fields {
		k := TClass{C: c.classes["String"]}
		if f.Symbol {
			k = TClass{C: c.classes["Symbol"]}
		}
		v := c.resolveType(f.Value, sc)
		if i == 0 {
			kt, vt = k, v
			continue
		}
		if kt.String() != k.String() || vt.String() != v.String() {
			return anyHash
		}
	}
	return TClass{C: hash, Args: []Type{kt, vt}}
}
