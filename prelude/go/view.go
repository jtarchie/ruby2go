//go:build rb2go_prelude

package prelude

import (
	"fmt"
	"sort"
	"strings"
)

// rbView: six accessors per Array instantiation let rbViewCall's one body answer whole-array dynamic calls (decision 236); _At is the raw E, rbUnbox gives the Ruby value.
type rbView interface {
	_Len() Integer
	_At(i Integer) any
	_Set(i Integer, v any)
	_Append(v any)
	_Trunc(n Integer)
	_Make() any
	rbCheckFrozen()
}

// rbHashView is the same for a Hash: keys in insertion order, values by key.
type rbHashView interface {
	_Len() Integer
	_KeyAt(i Integer) any
	_Has(k any) Boolean
	_Get(k any) any
	_Store(k, v any)
	_Delete(k any)
	_Make() any
	rbCheckFrozen()
}

func rbViewElems(s rbView) []any {
	n := int(s._Len())
	out := make([]any, n)
	for i := range n {
		out[i] = s._At(Integer(i))
	}
	return out
}

func rbViewSorted(s rbView) []any {
	xs := rbViewElems(s)
	sort.SliceStable(xs, func(i, j int) bool { return rbCmp(rbUnbox(xs[i]), rbUnbox(xs[j])) < 0 })
	return xs
}

func rbViewFrom(s rbView, xs []any) any {
	out := s._Make()
	for _, x := range xs {
		out.(rbView)._Append(x)
	}
	return out
}

// rbViewReplace makes s hold xs (raw elements), in place.
func rbViewReplace(s rbView, xs []any) {
	s.rbCheckFrozen()
	s._Trunc(0)
	for _, x := range xs {
		s._Append(x)
	}
}

// rbViewArg is an Array argument's raw elements (a view's keep their E, so a T? value round-trips), else as untyped code sees them.
func rbViewArg(v any, what string) []any {
	switch a := rbUnbox(v).(type) {
	case rbView:
		return rbViewElems(a)
	case Array_Any:
		return a._ToAny().s
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into " + what))))
}

// rbViewUniq keeps the first of each element by eql?/hash, as Array#uniq.
func rbViewUniq(xs []any) []any {
	seen := rbNewKeySet[any](nil)
	var out []any
	for _, x := range xs {
		u := rbUnbox(x)
		if !seen.has(u) {
			seen.put(u)
			out = append(out, x)
		}
	}
	return out
}

func rbViewIndex(s rbView, v any, fromEnd bool) any {
	xs := rbViewElems(s)
	for n := range xs {
		i := n
		if fromEnd {
			i = len(xs) - 1 - n
		}
		if rbEq[any](rbUnbox(xs[i]), v) {
			return Integer(i)
		}
	}
	return nil
}

func rbViewAssoc(s rbView, key any, at int) any {
	for _, x := range rbViewElems(s) {
		if a, ok := rbUnbox(x).(Array_Any); ok {
			if inner := a._ToAny().s; len(inner) > at && rbEq[any](inner[at], key) {
				return rbUnbox(x)
			}
		}
	}
	return nil
}

func rbViewTakeDrop(s rbView, args []any, take bool) any {
	rbArity(len(args), 1, 1)
	n := int(rbAs[Integer](args[0], "Integer"))
	what := "drop"
	if take {
		what = "take"
	}
	if n < 0 {
		panic(NewArgumentError(Ref(String("attempt to " + what + " negative size"))))
	}
	xs := rbViewElems(s)
	n = min(n, len(xs))
	if take {
		return rbViewFrom(s, xs[:n])
	}
	return rbViewFrom(s, xs[n:])
}

// rbViewCall answers a dynamic call of name on the view, or false when name is not one of its names.
func rbViewCall(name string, s rbView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo,maintidx // one switch per name, as the dispatchers are
	switch name {
	case "sort":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewSorted(s)), true
	case "sort!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		for i, x := range rbViewSorted(s) {
			s._Set(Integer(i), x)
		}
		return s, true
	case "reverse", "reverse!":
		rbArity(len(args), 0, 0)
		xs := rbViewElems(s)
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		if name == "reverse" {
			return rbViewFrom(s, xs), true
		}
		s.rbCheckFrozen()
		for i, x := range xs {
			s._Set(Integer(i), x)
		}
		return s, true
	case "compact":
		rbArity(len(args), 0, 0)
		var xs []any
		for _, x := range rbViewElems(s) {
			if rbUnbox(x) != nil {
				xs = append(xs, x)
			}
		}
		return rbViewFrom(s, xs), true
	case "include?":
		rbArity(len(args), 1, 1)
		return Boolean(rbViewIndex(s, args[0], false) != nil), true
	case "index", "find_index":
		rbArity(len(args), 1, 1)
		return rbViewIndex(s, args[0], false), true
	case "rindex":
		rbArity(len(args), 1, 1)
		return rbViewIndex(s, args[0], true), true
	case "count":
		rbArity(len(args), 0, 1)
		if len(args) == 0 {
			return s._Len(), true
		}
		n := 0
		for _, x := range rbViewElems(s) {
			if rbEq[any](rbUnbox(x), args[0]) {
				n++
			}
		}
		return Integer(n), true
	case "max", "min":
		rbArity(len(args), 0, 0)
		var best any
		for i, x := range rbViewElems(s) {
			x = rbUnbox(x)
			if i == 0 || (name == "max") == (rbCmp(x, best) > 0) && rbCmp(x, best) != 0 {
				best = x
			}
		}
		return best, true
	case "join":
		rbArity(len(args), 0, 1)
		sep := String("")
		if len(args) == 1 && args[0] != nil {
			sep = rbAs[String](args[0], "String")
		}
		parts := make([]string, int(s._Len()))
		for i := range parts {
			x := rbUnbox(s._At(Integer(i)))
			if a, ok := x.(Array_Any); ok {
				parts[i] = string(Array_Join(a._ToAny(), sep))
				continue
			}
			parts[i] = string(rbToS(x))
		}
		return rbStrClone(String(strings.Join(parts, string(sep)))), true
	case "uniq":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewUniq(rbViewElems(s))), true
	case "uniq!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		u := rbViewUniq(xs)
		if len(u) == len(xs) {
			return nil, true
		}
		rbViewReplace(s, u)
		return s, true
	case "+":
		rbArity(len(args), 1, 1)
		return rbViewFrom(s, append(rbViewElems(s), rbViewArg(args[0], "Array")...)), true
	case "-", "difference":
		rbArity(len(args), 1, 1)
		drop := rbNewKeySet(rbViewArg(args[0], "Array"))
		var xs []any
		for _, x := range rbViewElems(s) {
			if !drop.has(rbUnbox(x)) {
				xs = append(xs, x)
			}
		}
		return rbViewFrom(s, xs), true
	case "&", "intersection", "intersect?":
		rbArity(len(args), 1, 1)
		keep := rbNewKeySet(rbViewArg(args[0], "Array"))
		var xs []any
		for _, x := range rbViewElems(s) {
			if keep.has(rbUnbox(x)) {
				xs = append(xs, x)
			}
		}
		xs = rbViewUniq(xs)
		if name == "intersect?" {
			return Boolean(len(xs) > 0), true
		}
		return rbViewFrom(s, xs), true
	case "|", "union":
		rbArity(len(args), 1, 1)
		return rbViewFrom(s, rbViewUniq(append(rbViewElems(s), rbViewArg(args[0], "Array")...))), true
	case "*":
		rbArity(len(args), 1, 1)
		if sep, ok := rbUnbox(args[0]).(String); ok {
			return rbViewCall("join", s, []any{sep})
		}
		n := int(rbAs[Integer](args[0], "Integer"))
		if n < 0 {
			panic(NewArgumentError(Ref(String("negative argument"))))
		}
		xs := rbViewElems(s)
		var out []any
		for range n {
			out = append(out, xs...)
		}
		return rbViewFrom(s, out), true
	case "delete":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		var found any
		var xs []any
		hit := false
		for _, x := range rbViewElems(s) {
			if rbEq[any](rbUnbox(x), args[0]) {
				found, hit = rbUnbox(x), true
				continue
			}
			xs = append(xs, x)
		}
		if hit {
			rbViewReplace(s, xs)
		}
		return found, true
	case "delete_at":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		i := int(rbAs[Integer](args[0], "Integer"))
		if i < 0 {
			i += len(xs)
		}
		if i < 0 || i >= len(xs) {
			return nil, true
		}
		x := xs[i]
		rbViewReplace(s, append(xs[:i:i], xs[i+1:]...))
		return rbUnbox(x), true
	case "concat":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		for _, x := range rbViewArg(args[0], "Array") {
			s._Append(x)
		}
		return s, true
	case "replace":
		rbArity(len(args), 1, 1)
		rbViewReplace(s, rbViewArg(args[0], "Array"))
		return s, true
	case "fill":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		for i := range int(s._Len()) {
			s._Set(Integer(i), args[0])
		}
		return s, true
	case "insert":
		rbArity(len(args), 1, -1)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		i := int(rbAs[Integer](args[0], "Integer"))
		if i < 0 {
			i += len(xs) + 1
		}
		if i < 0 {
			panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", int(rbAs[Integer](args[0], "Integer")), len(xs)+1)))))
		}
		for len(xs) < i {
			xs = append(xs, nil)
		}
		out := append(append(append([]any{}, xs[:i]...), args[1:]...), xs[i:]...)
		rbViewReplace(s, out)
		return s, true
	case "rotate":
		rbArity(len(args), 0, 1)
		xs := rbViewElems(s)
		if len(xs) == 0 {
			return rbViewFrom(s, nil), true
		}
		n := 1
		if len(args) == 1 {
			n = int(rbAs[Integer](args[0], "Integer"))
		}
		k := ((n % len(xs)) + len(xs)) % len(xs)
		return rbViewFrom(s, append(append([]any{}, xs[k:]...), xs[:k]...)), true
	case "take":
		return rbViewTakeDrop(s, args, true), true
	case "drop":
		return rbViewTakeDrop(s, args, false), true
	case "assoc":
		rbArity(len(args), 1, 1)
		return rbViewAssoc(s, args[0], 0), true
	case "rassoc":
		rbArity(len(args), 1, 1)
		return rbViewAssoc(s, args[0], 1), true
	case "flatten", "flatten!":
		rbArity(len(args), 0, 1)
		depth := -1
		if len(args) == 1 {
			depth = int(rbAs[Integer](args[0], "Integer"))
		}
		xs := rbViewElems(s)
		nested := false
		for i, x := range xs {
			xs[i] = rbUnbox(x)
			if _, isArr := xs[i].(Array_Any); isArr {
				nested = true
			}
		}
		if name == "flatten!" {
			s.rbCheckFrozen()
			a, untyped := s.(*Array[any]) // a typed E cannot hold what its nested Arrays held (decision 92): nil, as the typed path
			if !nested || !untyped {
				return nil, true
			}
			out := &Array[any]{}
			rbFlattenInto(out, a.s, depth)
			a.s = out.s
			return s, true
		}
		if !nested {
			return rbViewFrom(s, rbViewElems(s)), true
		}
		out := &Array[any]{}
		rbFlattenInto(out, xs, depth)
		return out, true
	default:
	}
	return nil, false
}

// rbHashViewArg is a Hash argument as a view (raw values round-trip), else its untyped copy, which is one too.
func rbHashViewArg(v any) rbHashView {
	switch h := rbUnbox(v).(type) {
	case rbHashView:
		return h
	case Hash_Any:
		return h._ToAny()
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Hash"))))
}

func rbHashViewKeys(s rbHashView) []any {
	n := int(s._Len())
	out := make([]any, n)
	for i := range n {
		out[i] = s._KeyAt(Integer(i))
	}
	return out
}

func rbHashViewCopy(s rbHashView) rbHashView {
	out := s._Make().(rbHashView)
	for _, k := range rbHashViewKeys(s) {
		out._Store(k, s._Get(k))
	}
	return out
}

// rbHashViewCall answers a dynamic call of name on a Hash's view, or false when name is not one of its names.
func rbHashViewCall(name string, s rbHashView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo // one switch per name
	switch name {
	case "merge":
		rbArity(len(args), 1, 1)
		out := rbHashViewCopy(s)
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(other) {
			out._Store(k, other._Get(k))
		}
		return out, true
	case "merge!", "update":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(other) {
			s._Store(k, other._Get(k))
		}
		return s, true
	case "except", "slice":
		ks := rbNewKeySet(args)
		out := s._Make().(rbHashView)
		if name == "slice" {
			for _, k := range args {
				if s._Has(k) {
					out._Store(k, s._Get(k))
				}
			}
			return out, true
		}
		for _, k := range rbHashViewKeys(s) {
			if !ks.has(rbUnbox(k)) {
				out._Store(k, s._Get(k))
			}
		}
		return out, true
	case "to_h":
		rbArity(len(args), 0, 0)
		return s, true // MRI's Hash#to_h without a block is self
	case "compact":
		rbArity(len(args), 0, 0)
		out := s._Make().(rbHashView)
		for _, k := range rbHashViewKeys(s) {
			if v := s._Get(k); rbUnbox(v) != nil {
				out._Store(k, v)
			}
		}
		return out, true
	case "compact!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		changed := false
		for _, k := range rbHashViewKeys(s) {
			if rbUnbox(s._Get(k)) == nil {
				s._Delete(k)
				changed = true
			}
		}
		if !changed {
			return nil, true
		}
		return s, true
	case "flatten":
		rbArity(len(args), 0, 1)
		out := &Array[any]{}
		for _, k := range rbHashViewKeys(s) {
			out.s = append(out.s, rbUnbox(k), rbUnbox(s._Get(k)))
		}
		if len(args) == 1 {
			if depth := rbAs[Integer](args[0], "Integer"); depth > 1 {
				return Array_Flatten(out, depth-1), true
			}
		}
		return out, true
	case "value?", "has_value?":
		rbArity(len(args), 1, 1)
		for _, k := range rbHashViewKeys(s) {
			if rbEq[any](rbUnbox(s._Get(k)), args[0]) {
				return Boolean(true), true
			}
		}
		return Boolean(false), true
	case "key":
		rbArity(len(args), 1, 1)
		for _, k := range rbHashViewKeys(s) {
			if rbEq[any](rbUnbox(s._Get(k)), args[0]) {
				return rbUnbox(k), true
			}
		}
		return nil, true
	case "replace":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(s) {
			s._Delete(k)
		}
		for _, k := range rbHashViewKeys(other) {
			s._Store(k, other._Get(k))
		}
		return s, true
	default:
	}
	return nil, false
}
