package rbs

import (
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

func (Name) isType()         {}
func (Optional) isType()     {}
func (Tuple) isType()        {}
func (Union) isType()        {}
func (Intersection) isType() {}
func (Self) isType()         {}
func (Instance) isType()     {}
func (Void) isType()         {}
func (Bot) isType()          {}
func (Nil) isType()          {}
func (Untyped) isType()      {}
func (Bool) isType()         {}
func (Singleton) isType()    {}
func (Proc) isType()         {}
func (Literal) isType()      {}
func (Record) isType()       {}

type (
	// Name is `Foo` or `Foo[A, B]`.
	Name struct {
		Name     string
		Args     []Type
		Absolute bool // written `::Foo`: resolved from the top level, not lexically
	}
	// Optional is `T?`.
	Optional struct{ Elem Type }
	// Tuple is `[A, B]`.
	Tuple struct{ Elems []Type }
	// Union is `A | B`. Unions with nil are normalised to Optional.
	Union struct{ Elems []Type }
	// Intersection is `A & B`, binding tighter than `|` (decision 315).
	Intersection struct{ Elems []Type }
	// Self is `self`.
	Self struct{}
	// Instance is `instance`: an instance of the class whose singleton method names it.
	Instance struct{}
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
	// class the value is a member of (LitString, LitSymbol or LitInteger); Value
	// is the decoded text. Used to select a signature by a literal argument
	// (decision 12).
	Literal struct {
		Kind  LitKind
		Value string
	}
	// Record is `{ "a" => T, b: U }`: a Hash-shaped type whose keys are known.
	Record struct{ Fields []RecordField }
)

// LitKind is the base class a Literal is a member of.
type LitKind string

const (
	LitString  LitKind = "String"
	LitSymbol  LitKind = "Symbol"
	LitInteger LitKind = "Integer"
)

// RecordField is one field of a Record.
type RecordField struct {
	Key    string
	Symbol bool // `b: U` (a symbol key) rather than `"b" => U`
	Value  Type
}

func (t Name) String() string {
	name := t.Name
	if t.Absolute {
		name = "::" + name
	}
	if len(t.Args) == 0 {
		return name
	}
	return name + "[" + join(t.Args) + "]"
}
func (t Optional) String() string {
	if needsParens(t.Elem) { // unparenthesised, the `?` would bind to the last member
		return "(" + t.Elem.String() + ")?"
	}
	return t.Elem.String() + "?"
}

// needsParens: a union, intersection or proc nested in a union, intersection or optional reparses differently without them.
func needsParens(t Type) bool {
	_, union := t.(Union)
	_, inter := t.(Intersection)
	_, proc := t.(Proc)
	return union || inter || proc
}
func (t Tuple) String() string        { return "[" + join(t.Elems) + "]" }
func (t Union) String() string        { return joinOp(t.Elems, " | ") }
func (t Intersection) String() string { return joinOp(t.Elems, " & ") }

func joinOp(elems []Type, op string) string {
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = e.String()
		if needsParens(e) {
			parts[i] = "(" + parts[i] + ")"
		}
	}
	return strings.Join(parts, op)
}
func (Self) String() string        { return "self" }
func (Instance) String() string    { return "instance" }
func (Void) String() string        { return "void" }
func (Bot) String() string         { return "bot" }
func (Nil) String() string         { return "nil" }
func (Untyped) String() string     { return "untyped" }
func (Bool) String() string        { return "bool" }
func (t Singleton) String() string { return "singleton(" + t.Name + ")" }

func (t Literal) String() string {
	switch t.Kind {
	case LitString:
		return strconv.Quote(t.Value)
	case LitSymbol:
		return ":" + t.Value
	case LitInteger:
		return t.Value
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
	Optional bool      // `?T`, or `?name: T` for a keyword
	Kind     ParamKind // positional, `*T`, `name: T` or `**T`: exactly one
}

// ParamKind is what shape of argument a Param takes. A parameter is exactly
// one of these, so a rest keyword (or any other mix) cannot be built.
type ParamKind uint8

const (
	ParamPositional ParamKind = iota // `T`
	ParamRest                        // `*T`
	ParamKeyword                     // `name: T`: Name is the keyword
	ParamKwRest                      // `**T`: the other keywords, a Hash[Symbol, T]
)

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

func writeParams(b *strings.Builder, params []Param) {
	b.WriteString("(")
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		switch p.Kind {
		case ParamRest:
			b.WriteString("*")
		case ParamKwRest:
			b.WriteString("**")
		case ParamPositional, ParamKeyword:
		}
		if p.Optional {
			b.WriteString("?")
		}
		if p.Kind == ParamKeyword {
			b.WriteString(p.Name + ": " + p.Type.String())
			continue
		}
		b.WriteString(p.Type.String())
		if p.Name != "" {
			b.WriteString(" " + paramName(p.Name))
		}
	}
	b.WriteString(")")
}

func (m *MethodType) String() string {
	var b strings.Builder
	if len(m.TypeParams) > 0 {
		b.WriteString("[" + strings.Join(m.TypeParams, ", ") + "] ")
	}
	writeParams(&b, m.Params)
	if m.Block != nil {
		b.WriteString(" ")
		if m.Block.Optional {
			b.WriteString("?")
		}
		b.WriteString("{ ")
		writeParams(&b, m.Block.Params)
		if m.Block.Self != nil {
			b.WriteString(" [self: " + m.Block.Self.String() + "]")
		}
		b.WriteString(" -> " + m.Block.Return.String() + " }")
	}
	b.WriteString(" -> " + m.Return.String())
	return b.String()
}

func (t Proc) String() string {
	self := ""
	if t.Self != nil {
		self = " [self: " + t.Self.String() + "]"
	}
	return "^(" + join(t.Params) + ")" + self + " -> " + t.Ret.String()
}
