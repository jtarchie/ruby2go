//go:build rugaroo_prelude

package prelude

import "sync"

// A Struct's ==, eql? and hash stop at a pair or object already in progress, as MRI's rb_exec_recursive does, so a struct
// that holds itself answers instead of overflowing the Go stack (decision 407).
//
// The common case costs a counter: a comparison nested no deeper than rbRecShallow only bumps its goroutine's depth, and
// only a comparison past that (a cycle always gets there) starts listing the pairs in progress, so ordinary structs pay
// nothing for the list.

const rbRecShallow = 8

// rbRecDepthMain is main's comparison depth (and an untagged goroutine's); a goroutine that runs Ruby keeps its own in its
// tag (rbGoTag), so neither a map lookup nor an atomic is on the fast path.
var rbRecDepthMain, rbRecHashDepthMain int

// rbRecActive maps a goroutine id to the pairs it is comparing past rbRecShallow, MRI's rb_exec_recursive list: only that
// goroutine touches its list, so threads comparing one struct do not see each other's progress.
var rbRecActive sync.Map

type rbRecEntry struct {
	kind Integer // 0 is ==, 1 is eql?, 2 is hash
	a, b any
}

// rbRecThrow unwinds a hash that reached itself to the outermost hash call (MRI's rb_exec_recursive_outer).
type rbRecThrow struct{}

// rbRecHashValue is what a struct that reaches itself hashes to; any constant does, as long as every such struct agrees.
const rbRecHashValue = 0x5ec0de // untyped: a constant of type Integer would be an enum member to the exhaustive linter, flagging every switch over an Integer

func rbRecDepth() *int {
	if !rbSpawned.Load() {
		return &rbRecDepthMain
	}
	if p := rbGetGoTag(); p != nil {
		return &(*rbGoTag)(p).recDepth
	}
	return &rbRecDepthMain
}

func rbRecHashDepth() *int {
	if !rbSpawned.Load() {
		return &rbRecHashDepthMain
	}
	if p := rbGetGoTag(); p != nil {
		return &(*rbGoTag)(p).recHashDepth
	}
	return &rbRecHashDepthMain
}

// rbRecMain is the list while no goroutine but main runs Ruby (rbSpawned false): no map, no allocation.
var rbRecMain []rbRecEntry

func rbRecList() *[]rbRecEntry {
	if !rbSpawned.Load() {
		return &rbRecMain
	}
	id := rbGoID()
	if v, ok := rbRecActive.Load(id); ok {
		if l, ok := v.(*[]rbRecEntry); ok {
			return l
		}
	}
	l := new([]rbRecEntry)
	rbRecActive.Store(id, l)
	return l
}

// rbRecPop drops the innermost entry and forgets the goroutine once its list is empty. A goroutine started between an
// enter and its leave finds the other list empty and leaves it alone.
func rbRecPop(l *[]rbRecEntry) {
	if len(*l) == 0 {
		return
	}
	*l = (*l)[:len(*l)-1]
	if len(*l) == 0 && l != &rbRecMain {
		rbRecActive.Delete(rbGoID())
	}
}

// rbRecSame is one member of a generated Struct#==: a scalar compares directly (same-typed numbers and symbols without
// the generic dispatch), anything else that may lead back to the struct is bracketed by the pairs in progress.
func rbRecSame(a, b any) bool {
	switch x := a.(type) {
	case Integer:
		if y, ok := b.(Integer); ok {
			return x == y
		}
		return bool(rbEq[any](a, b))
	case Symbol:
		if y, ok := b.(Symbol); ok {
			return x == y
		}
		return bool(rbEq[any](a, b))
	case Float, String, Boolean, nil:
		return bool(rbEq[any](a, b))
	}
	if rbRecEnter(0, a, b) {
		return true
	}
	defer rbRecLeave()
	return bool(rbEq[any](a, b))
}

// rbRecEnter reports whether (kind, a, b) is already being compared on this goroutine; if not it counts the comparison in.
func rbRecEnter(kind Integer, a, b any) bool {
	d := rbRecDepth()
	*d++
	if *d <= rbRecShallow {
		return false
	}
	return rbRecEnterDeep(d, kind, a, b)
}

func rbRecEnterDeep(d *int, kind Integer, a, b any) bool {
	l := rbRecList()
	for _, e := range *l {
		if e.kind == kind && e.a == a && e.b == b {
			*d--
			return true
		}
	}
	*l = append(*l, rbRecEntry{kind, a, b})
	return false
}

func rbRecLeave() {
	d := rbRecDepth()
	if *d > rbRecShallow {
		rbRecPop(rbRecList())
	}
	*d--
}

// rbRecHash runs f, the hash of obj; when f reaches obj again the outermost rbRecHash on this goroutine returns rbRecHashValue.
func rbRecHash(obj any, f func() Integer) (h Integer) {
	d := rbRecHashDepth()
	*d++
	outer, deep := *d == 1, *d > rbRecShallow
	var l *[]rbRecEntry
	if deep {
		l = rbRecList()
		for _, e := range *l {
			if e.kind == 2 && e.a == obj {
				*d--
				panic(rbRecThrow{})
			}
		}
		*l = append(*l, rbRecEntry{2, obj, nil})
	}
	defer func() {
		*d--
		if deep {
			rbRecPop(l)
		}
		if outer {
			if r := recover(); r != nil {
				if _, ok := r.(rbRecThrow); !ok {
					panic(r)
				}
				h = Integer(rbRecHashValue)
			}
		}
	}()
	return f()
}
