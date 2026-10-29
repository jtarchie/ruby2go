//go:build rugaroo_prelude

package prelude

import "sync"

// rbRecActive maps a goroutine id to the struct comparisons and hashes it is running, MRI's rb_exec_recursive list: only
// that goroutine touches its list, so threads comparing one struct do not see each other's progress.
var rbRecActive sync.Map

type rbRecEntry struct {
	kind Integer // 0 is ==, 1 is eql?, 2 is hash
	a, b any
}

// rbRecThrow unwinds a hash that reached itself to the outermost hash call (MRI's rb_exec_recursive_outer).
type rbRecThrow struct{}

// rbRecHashValue is what a struct that reaches itself hashes to; any constant does, as long as every such struct agrees.
const rbRecHashValue Integer = 0x5ec0de

// rbRecMain is the list while no goroutine but main runs Ruby (rbSpawned false): no map, no allocation per comparison.
var rbRecMain []rbRecEntry

func rbRecList() *[]rbRecEntry {
	if !rbSpawned.Load() {
		return &rbRecMain
	}
	id := rbGoID()
	if v, ok := rbRecActive.Load(id); ok {
		return v.(*[]rbRecEntry) //nolint:forcetypeassert // only rbRecList stores here
	}
	l := new([]rbRecEntry)
	rbRecActive.Store(id, l)
	return l
}

// rbRecPop drops the innermost entry and forgets the goroutine once its list is empty. A goroutine started between
// an enter and its leave finds the other list empty and leaves it alone.
func rbRecPop(l *[]rbRecEntry) {
	if len(*l) == 0 {
		return
	}
	*l = (*l)[:len(*l)-1]
	if len(*l) == 0 && l != &rbRecMain {
		rbRecActive.Delete(rbGoID())
	}
}

// rbRecEnter reports whether (kind, a, b) is already running on this goroutine; if not it records it.
func rbRecEnter(kind Integer, a, b any) bool {
	l := rbRecList()
	for _, e := range *l {
		if e.kind == kind && e.a == a && e.b == b {
			return true
		}
	}
	*l = append(*l, rbRecEntry{kind, a, b})
	return false
}

func rbRecLeave() { rbRecPop(rbRecList()) }

// rbRecHash runs f, the hash of obj; when f reaches obj again the outermost rbRecHash on this goroutine returns rbRecHashValue.
func rbRecHash(obj any, f func() Integer) (h Integer) {
	l := rbRecList()
	outer := true
	for _, e := range *l {
		if e.kind == 2 {
			outer = false
			if e.a == obj {
				panic(rbRecThrow{})
			}
		}
	}
	*l = append(*l, rbRecEntry{2, obj, nil})
	defer func() {
		rbRecPop(l)
		if outer {
			if r := recover(); r != nil {
				if _, ok := r.(rbRecThrow); !ok {
					panic(r)
				}
				h = rbRecHashValue
			}
		}
	}()
	return f()
}
