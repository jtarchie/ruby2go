package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// --int-overflow=promote (decision 372): Bignum exists only in that mode, so the default build never reaches these.

// bignumConv: past 64 bits the Integer side raises RangeError, as MRI's conversion to a C long does.
func bignumConv(e expr, to TClass) (string, bool) {
	switch {
	case to.C.Name == "Bignum" && isClass(e.typ, "Integer"):
		return "Bignum{i: " + e.code + "}", true
	case to.C.Name == "Integer" && isClass(e.typ, "Bignum"):
		return "rbBignumToInt(" + e.code + ")", true
	}
	return "", false
}

// bignumFallback: Integer methods Bignum leaves out (times, chr, ...) run on the value as an Integer.
func (f *fctx) bignumFallback(recv expr, name string) (expr, bool) {
	if !isClass(recv.typ, "Bignum") || !f.bignumLacks(name) {
		return expr{}, false
	}
	if ie := f.c.classes["Integer"].lookup(name); ie == nil || ie.Owner.universal {
		return expr{}, false
	}
	return expr{code: "rbBignumToInt(" + recv.code + ")", typ: f.cls("Integer")}, true
}

// bignumLacks reports a method Bignum does not define itself (Object's does not count).
func (f *fctx) bignumLacks(name string) bool {
	e := f.c.classes["Bignum"].lookup(name)
	return e == nil || e.Owner.universal
}

// bignumLocals feed the next run, so `a = b` after `b = a + b` carries the Bignum on.
func (f *fctx) bignumLocals() map[string]bool {
	if f.c.classes["Bignum"] == nil {
		return nil
	}
	var out map[string]bool
	for k, li := range f.locals {
		if !li.param && !f.promoted[k.name] && isClass(stripOpt(li.typ), "Bignum") {
			if out == nil {
				out = map[string]bool{}
			}
			out[k.name] = true
		}
	}
	return out
}

// promotedType is Bignum for an unannotated write of an Integer or Bignum to a promoted local, else annotated.
func (f *fctx) promotedType(name string, val expr, annotated Type) Type {
	if annotated != nil || !f.promoted[name] || !isClass(val.typ, "Integer") && !isClass(val.typ, "Bignum") {
		return annotated
	}
	return f.cls("Bignum")
}

// growthLocals is spinel's loop-growth promotion (analyze_pass.c): `f = f * i`, `f *= i`, `c = a + b; a = b; b = c`.
func growthLocals(body parser.Node) map[string]bool {
	out := map[string]bool{}
	walkNode(body, func(n parser.Node) (bool, bool) {
		if lb := loopBody(n); lb != nil {
			scanGrowth(lb, out)
		}
		return false, !opensScope(n)
	})
	return out
}

func loopBody(n parser.Node) parser.Node {
	switch n := n.(type) {
	case *parser.WhileNode:
		if n.Statements != nil { // a typed nil would reach walkNode as a non-nil Node
			return n.Statements
		}
	case *parser.UntilNode:
		if n.Statements != nil {
			return n.Statements
		}
	case *parser.CallNode:
		if b, ok := n.Block.(*parser.BlockNode); ok && blockLoops[n.Name] {
			return b.Body
		}
	}
	return nil
}

var blockLoops = map[string]bool{"times": true, "each": true, "upto": true, "downto": true, "step": true, "loop": true, "each_with_index": true}

type growthPair struct{ dst, src string }

type growthWrite struct {
	name string
	val  parser.Node
}

func scanGrowth(body parser.Node, out map[string]bool) {
	pairs, writes := growthAssigns(body, out)
	for _, w := range writes {
		call, ok := w.val.(*parser.CallNode)
		if !ok {
			continue
		}
		recv := localName(call.Receiver)
		var arg string
		if args := callArgs(call); len(args) == 1 {
			arg = localName(args[0])
		}
		switch call.Name {
		case "*", "**":
			if recv != "" && flowsTo(pairs, w.name, recv) || arg != "" && flowsTo(pairs, w.name, arg) {
				out[w.name] = true
			}
		case "+": // both operands: `i = i + 1` is linear
			if recv != "" && arg != "" && flowsTo(pairs, w.name, recv) && flowsTo(pairs, w.name, arg) {
				out[w.name] = true
			}
		}
	}
}

// growthAssigns are a loop body's local writes and its `a = b` copies; `x *= y` promotes x outright.
func growthAssigns(body parser.Node, out map[string]bool) ([]growthPair, []growthWrite) {
	var pairs []growthPair
	var writes []growthWrite
	write := func(name string, val parser.Node) {
		writes = append(writes, growthWrite{name, val})
		if r := localName(val); r != "" {
			pairs = append(pairs, growthPair{name, r})
		}
	}
	walkNode(body, func(n parser.Node) (bool, bool) {
		switch n := n.(type) {
		case *parser.LocalVariableWriteNode:
			write(n.Name, n.Value)
		case *parser.LocalVariableOperatorWriteNode:
			if n.BinaryOperator == "*" || n.BinaryOperator == "**" {
				out[n.Name] = true
			}
		case *parser.MultiWriteNode:
			if arr, ok := n.Value.(*parser.ArrayNode); ok && n.Rest == nil && len(n.Rights) == 0 {
				for i, l := range n.Lefts {
					if t, ok := l.(*parser.LocalVariableTargetNode); ok && i < len(arr.Elements) {
						write(t.Name, arr.Elements[i])
					}
				}
			}
		}
		return false, !opensScope(n)
	})
	return pairs, writes
}

func localName(n parser.Node) string {
	if r, ok := n.(*parser.LocalVariableReadNode); ok {
		return r.Name
	}
	return ""
}

// flowsTo reports whether from's value reaches local to through the loop's `to = from` copies.
func flowsTo(pairs []growthPair, from, to string) bool {
	seen := map[string]bool{from: true}
	frontier := []string{from}
	for len(frontier) > 0 {
		if seen[to] {
			return true
		}
		var next []string
		for _, v := range frontier {
			for _, p := range pairs {
				if p.src == v && !seen[p.dst] {
					seen[p.dst] = true
					next = append(next, p.dst)
				}
			}
		}
		frontier = next
	}
	return seen[to]
}

// takesInteger reports an Integer? or a union with an Integer member, where a Bignum converts down.
func takesInteger(t Type) bool {
	if u, ok := stripOpt(t).(TUnion); ok {
		return slices.ContainsFunc(u.Members, func(m Type) bool { return isClass(m, "Integer") })
	}
	return isOpt(t) && isClass(stripOpt(t), "Integer")
}
