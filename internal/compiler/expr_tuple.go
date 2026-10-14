package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// tupleLiteral turns a literal with mixed element types into a tuple.
func (f *fctx) tupleLiteral(n parser.Node, elems []expr) (expr, bool) {
	same := true
	for _, e := range elems {
		if !typeEq(e.typ, elems[0].typ) {
			same = false
		}
	}
	if same {
		return expr{}, false
	}
	if len(elems) < 2 || len(elems) > 3 {
		return expr{}, false
	}
	for _, e := range elems {
		if isNil(e.typ) || isAny(e.typ) {
			return expr{}, false
		}
	}
	ts := make([]Type, len(elems))
	codes := make([]string, len(elems))
	for i, e := range elems {
		ts[i] = e.typ
		codes[i] = f.coerce(n, e, e.typ)
	}
	tt := TTuple{Elems: ts}
	return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}, true
}

// tupleAsArray reports an Array method a tuple answers on the Array of its fields: any in gem code, else one that
// neither changes the receiver nor iterates (decision 241).
func (f *fctx) tupleAsArray(name string) bool {
	e := f.c.classes["Array"].lookup(name)
	return e != nil && (f.lenient() || !tupleMutators[name] && !strings.HasSuffix(name, "!") && !e.M.Iterator)
}

// tupleArray is a tuple's fields as an Array of their join (untyped when mixed).
func (f *fctx) tupleArray(n parser.Node, recv expr, tt TTuple) expr {
	code := recv.code
	if !isSimpleGo(code) {
		code = f.newTmp()
		f.emit("%s := %s", code, recv.code)
	}
	et := joinOrAny(tt.Elems)
	parts := make([]string, len(tt.Elems))
	for i, t := range tt.Elems {
		parts[i] = f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", code, i), typ: t}, et)
	}
	return expr{code: arrayLit(f.c.goType(et), parts), typ: TClass{C: f.c.classes["Array"], Args: []Type{et}}}
}

// tupleMutators are Array methods that change the receiver: on a tuple they would change a copy of its fields, so only
// gem code (decision 227) gets them; the rest answer on that copy anywhere (decision 241).
var tupleMutators = map[string]bool{"<<": true, "[]=": true, "push": true, "append": true, "pop": true, "shift": true,
	"unshift": true, "prepend": true, "insert": true, "delete": true, "delete_at": true, "delete_if": true, "keep_if": true,
	"clear": true, "concat": true, "replace": true, "fill": true}

func (f *fctx) tupleCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	tt := recv.typ.(TTuple)
	noArgs, oneArg := len(args) == 0 && block == nil, len(args) == 1 && block == nil
	switch name {
	case "[]":
		if len(args) == 1 {
			if in, ok := args[0].(*parser.IntegerNode); ok && in.Value >= 0 && int(in.Value) < len(tt.Elems) {
				return expr{code: fmt.Sprintf("%s.F%d", recv.code, in.Value), typ: tt.Elems[in.Value]}
			}
		}
		f.errorf(n, "tuple index must be a literal in range")
	case "first": // first(n) answers an Array: it falls to the Array of the fields below
		if noArgs {
			return expr{code: recv.code + ".F0", typ: tt.Elems[0]}
		}
	case "last":
		if noArgs {
			return expr{code: fmt.Sprintf("%s.F%d", recv.code, len(tt.Elems)-1), typ: tt.Elems[len(tt.Elems)-1]}
		}
	case "to_s", "inspect":
		return expr{code: recv.code + "." + goMethodName(name) + "()", typ: f.cls("String")}
	case "to_a": // the fields as an Array, joined (decision 241)
		if noArgs {
			return f.tupleArray(n, recv, tt)
		}
	case "to_json":
		return expr{code: recv.code + ".ToJson(" + strings.Join(f.jsonArgs(args), ", ") + ")", typ: f.cls("String")}
	case "<=>":
		if oneArg {
			a := f.genExpr(args[0], recv.typ)
			return expr{code: recv.code + ".Op_cmp(" + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
		}
	case "==":
		if oneArg {
			a := f.genExpr(args[0], nil)
			code := a.code // the same tuple type compares field-wise, without converting
			if !typeEq(a.typ, recv.typ) {
				code = f.coerce(args[0], a, TAny{})
			}
			return expr{code: recv.code + ".Op_eq(" + code + ")", typ: f.cls("Boolean")}
		}
	case "pack": // a mixed literal like [str, n].pack("a4N") is a tuple; rbPack takes its fields as one []any (decision 138)
		if oneArg {
			code := recv.code
			if !isSimpleGo(code) { // the receiver first, as Ruby evaluates it, then the format
				code = f.newTmp()
				f.emit("%s := %s", code, recv.code)
			}
			str := f.cls("String")
			format := f.coerce(args[0], f.genExpr(args[0], str), str)
			fields := make([]string, len(tt.Elems))
			for i := range tt.Elems {
				fields[i] = fmt.Sprintf("%s.F%d", code, i)
			}
			return expr{code: "String(rbPack([]any{" + strings.Join(fields, ", ") + "}, string(" + format + ")))", typ: str}
		}
	}
	if e := f.c.classes["Object"].lookup(name); e != nil && !untypedIntrinsics[name] {
		return f.callEntry(n, e, recv, args, block) // Kernel's, with the tuple as Self
	}
	if f.tupleAsArray(name) {
		// a gem's `[timeout, 0.0].max`, `pair.second`: the Array of the tuple's fields, joined (untyped when mixed), whose
		// method answers as MRI's Array does (decision 227)
		return f.genMethodCall(n, f.tupleArray(n, recv, tt), name, args, block)
	}
	f.errorf(n, "undefined method %s for tuple %s", name, tt)
	return expr{}
}
