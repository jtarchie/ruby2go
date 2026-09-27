//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// Ruby 3.4+ prints symbol keys as labels: `a: 1`, or `"a b": 1` when the
// symbol is not an identifier (rbIdentRe, prelude/symbol.rb).
var rbLabelSymbol = regexp.MustCompile(`\A` + rbIdentRe + `[?!]?\z`)

func rbInspectPair(k, v any) string {
	if s, ok := rbUnbox(k).(Symbol); ok {
		if rbLabelSymbol.MatchString(string(s)) {
			return string(s) + ": " + string(rbInspect(v))
		}
		return string(rbStringInspect(string(s))) + ": " + string(rbInspect(v))
	}
	return string(rbInspect(k)) + " => " + string(rbInspect(v))
}

// rbKeyIndex buckets the keys of a Hash (or uniq's seen set) that match
// by value (rbValueKey) by their hash. plain (rbPlainKey[K], computed
// once) skips it. It is not a Hash[K, ...] because Hash's methods would
// make uniq a Go instantiation cycle (decision 9).
type rbKeyIndex[K comparable] struct {
	plain  bool
	byHash map[uint64][]K
}

// find is the map key Ruby's eql? gives k: the stored key eql? to k, or
// k itself. Keys that do not match by value are their own map keys.
func (idx *rbKeyIndex[K]) find(k K) (key K, h uint64, byValue bool) {
	if idx.plain {
		return k, 0, false
	}
	if h, byValue = rbValueKey(k); !byValue {
		return k, 0, false
	}
	for _, c := range idx.byHash[h] {
		if rbKeyEql(c, k) {
			return c, h, true
		}
	}
	return k, h, true
}

func (idx *rbKeyIndex[K]) add(k K, h uint64) {
	if idx.byHash == nil {
		idx.byHash = map[uint64][]K{}
	}
	idx.byHash[h] = append(idx.byHash[h], k)
}

func (idx *rbKeyIndex[K]) remove(k K, h uint64) {
	idx.byHash[h] = slices.DeleteFunc(idx.byHash[h], func(c K) bool { return c == k })
}

// rbFrom converts v, a Hash of any instantiation, into this one: a
// copy, keys and values converted (rbConv), as Array's rbFrom.
func (*Hash[K, V]) rbFrom(v any) (*Hash[K, V], bool) {
	h, ok := v.(Hash_Any)
	if !ok {
		return nil, false
	}
	src := h._ToAny()
	if out, ok := any(src).(*Hash[K, V]); ok {
		return out, true
	}
	out := NewHash[K, V]()
	for _, k := range src.keys {
		ck, ok := rbConv[K](k)
		if !ok {
			return nil, false
		}
		cv, ok := rbConv[V](src.vals[k])
		if !ok {
			return nil, false
		}
		out.Op_idxSet(ck, cv)
	}
	return out, true
}

// rbGet is the value at k. It is kept out of line so that Hash#[] fits
// Go's inlining budget: inlined, the V? it returns is not heap-allocated.
//
//go:noinline
func (self *Hash[K, V]) rbGet(k K) (V, bool) {
	if !self.idx.plain {
		k, _, _ = self.idx.find(k)
	}
	v, ok := self.vals[k]
	return v, ok
}
