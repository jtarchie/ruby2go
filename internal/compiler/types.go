package compiler

import (
	"strings"
)

// Type is a resolved Ruby type.
type Type interface{ String() string }

type (
	// TClass is an instance of a class or module, possibly with type args.
	TClass struct {
		C    *Class
		Args []Type
	}
	// TOpt is `T?`.
	TOpt struct{ Elem Type }
	// TTuple is `[A, B]`.
	TTuple struct{ Elems []Type }
	// TVar is a type variable: a class/module type param, a method type
	// param, or Self.
	TVar struct{ Name string }
	// TFunc is the type of a block/proc value.
	TFunc struct {
		Params []Type
		Ret    Type
	}
	// TAny is `untyped`.
	TAny struct{}
	// TNil is the type of `nil`.
	TNil struct{}
	// TVoid is `void`: no value.
	TVoid struct{}
)

func (t TClass) String() string {
	if t.C.metaOf != nil {
		return "singleton(" + t.C.metaOf.RubyName + ")"
	}
	if len(t.Args) == 0 {
		return t.C.RubyName
	}
	return t.C.RubyName + "[" + joinTypes(t.Args) + "]"
}
func (t TOpt) String() string   { return t.Elem.String() + "?" }
func (t TTuple) String() string { return "[" + joinTypes(t.Elems) + "]" }
func (t TVar) String() string   { return t.Name }
func (t TFunc) String() string  { return "^(" + joinTypes(t.Params) + ") -> " + t.Ret.String() }
func (TAny) String() string     { return "untyped" }
func (TNil) String() string     { return "nil" }
func (TVoid) String() string    { return "void" }

func joinTypes(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}

func isVoid(t Type) bool {
	switch t.(type) {
	case TVoid, TNil, nil:
		return true
	}
	return false
}

func isAny(t Type) bool { _, ok := t.(TAny); return ok }
func isOpt(t Type) bool { _, ok := t.(TOpt); return ok }
func isNil(t Type) bool { _, ok := t.(TNil); return ok }

func classOf(t Type) *Class {
	if c, ok := t.(TClass); ok {
		return c.C
	}
	return nil
}

func isClass(t Type, name string) bool {
	c := classOf(t)
	return c != nil && c.Name == name
}

// stripOpt returns the non-optional part of t.
func stripOpt(t Type) Type {
	if o, ok := t.(TOpt); ok {
		return o.Elem
	}
	return t
}

func typeEq(a, b Type) bool {
	switch a := a.(type) {
	case TClass:
		b, ok := b.(TClass)
		if !ok || a.C != b.C || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !typeEq(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case TOpt:
		b, ok := b.(TOpt)
		return ok && typeEq(a.Elem, b.Elem)
	case TTuple:
		b, ok := b.(TTuple)
		if !ok || len(a.Elems) != len(b.Elems) {
			return false
		}
		for i := range a.Elems {
			if !typeEq(a.Elems[i], b.Elems[i]) {
				return false
			}
		}
		return true
	case TVar:
		b, ok := b.(TVar)
		return ok && a.Name == b.Name
	case TFunc:
		b, ok := b.(TFunc)
		if !ok || len(a.Params) != len(b.Params) || !typeEq(a.Ret, b.Ret) {
			return false
		}
		for i := range a.Params {
			if !typeEq(a.Params[i], b.Params[i]) {
				return false
			}
		}
		return true
	case TAny:
		_, ok := b.(TAny)
		return ok
	case TNil:
		_, ok := b.(TNil)
		return ok
	case TVoid:
		_, ok := b.(TVoid)
		return ok
	}
	return false
}

// subst replaces type variables according to env.
func subst(t Type, env map[string]Type) Type {
	if len(env) == 0 {
		return t
	}
	switch t := t.(type) {
	case TVar:
		if r, ok := env[t.Name]; ok {
			return r
		}
		return t
	case TClass:
		if len(t.Args) == 0 {
			return t
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = subst(a, env)
		}
		return TClass{C: t.C, Args: args}
	case TOpt:
		return TOpt{Elem: subst(t.Elem, env)}
	case TTuple:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = subst(e, env)
		}
		return TTuple{Elems: elems}
	case TFunc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = subst(p, env)
		}
		return TFunc{Params: ps, Ret: subst(t.Ret, env)}
	}
	return t
}

// unify binds type variables in pattern so that it matches actual.
// Returns false on a structural mismatch (never an error: Go decides).
func unify(pattern, actual Type, env map[string]Type) bool {
	switch p := pattern.(type) {
	case TVar:
		if bound, ok := env[p.Name]; ok {
			// Prefer the more specific binding: nil upgrades to T? later.
			if isNil(bound) && !isNil(actual) {
				env[p.Name] = actual
			}
			return true
		}
		if _, ok := actual.(TVoid); ok {
			return false
		}
		env[p.Name] = actual
		return true
	case TClass:
		return unifyClass(p, actual, env)
	case TOpt:
		switch a := actual.(type) {
		case TOpt:
			return unify(p.Elem, a.Elem, env)
		case TNil:
			return true
		default:
			return unify(p.Elem, actual, env)
		}
	case TTuple:
		a, ok := actual.(TTuple)
		if !ok {
			return false
		}
		return unifyAll(p.Elems, a.Elems, env)
	case TFunc:
		a, ok := actual.(TFunc)
		if !ok || !unifyAll(p.Params, a.Params, env) {
			return false
		}
		return unify(p.Ret, a.Ret, env)
	}
	return true
}

func unifyClass(p TClass, actual Type, env map[string]Type) bool {
	a, ok := actual.(TClass)
	if !ok {
		return false
	}
	if a.C != p.C {
		// Allow a subclass to match its ancestor's pattern.
		return a.C.isSubclassOf(p.C)
	}
	return unifyAll(p.Args, a.Args, env)
}

func unifyAll(ps, as []Type, env map[string]Type) bool {
	if len(ps) != len(as) {
		return false
	}
	ok := true
	for i := range ps {
		ok = unify(ps[i], as[i], env) && ok
	}
	return ok
}

// freeVars lists the type variables mentioned in t, in order of appearance.
func freeVars(t Type, out *[]string) {
	switch t := t.(type) {
	case TVar:
		for _, n := range *out {
			if n == t.Name {
				return
			}
		}
		*out = append(*out, t.Name)
	case TClass:
		for _, a := range t.Args {
			freeVars(a, out)
		}
	case TOpt:
		freeVars(t.Elem, out)
	case TTuple:
		for _, e := range t.Elems {
			freeVars(e, out)
		}
	case TFunc:
		for _, p := range t.Params {
			freeVars(p, out)
		}
		freeVars(t.Ret, out)
	}
}

// join computes the type of `cond ? a : b`.
func join(a, b Type) (Type, bool) {
	switch {
	case isNil(a) && isNil(b):
		return TNil{}, true
	case isNil(a):
		if isOpt(b) || isAny(b) {
			return b, true
		}
		return TOpt{Elem: b}, true
	case isNil(b):
		if isOpt(a) || isAny(a) {
			return a, true
		}
		return TOpt{Elem: a}, true
	case typeEq(a, b):
		return a, true
	case isOpt(a) && typeEq(a.(TOpt).Elem, b):
		return a, true
	case isOpt(b) && typeEq(b.(TOpt).Elem, a):
		return b, true
	case isAny(a) || isAny(b):
		return TAny{}, true
	}
	// Subclass / superclass: pick the ancestor.
	if ca, cb := classOf(a), classOf(b); ca != nil && cb != nil && ca != cb {
		if ca.isSubclassOf(cb) {
			return b, true
		}
		if cb.isSubclassOf(ca) {
			return a, true
		}
	}
	return nil, false
}
