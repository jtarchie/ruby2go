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
		return string(rbStringInspectAs(string(s), false)) + ": " + string(rbInspect(v))
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
		Hash_Op_idxSet(out, ck, cv)
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

// rbKeySet is a set keyed by eql?/hash, as Hash keys are (Array#-, #&).
type rbKeySet[K comparable] struct {
	m   map[K]bool
	idx rbKeyIndex[K]
}

func rbNewKeySet[K comparable](xs []K) *rbKeySet[K] {
	s := &rbKeySet[K]{m: map[K]bool{}, idx: rbKeyIndex[K]{plain: rbPlainKey[K]()}}
	for _, x := range xs {
		s.put(x)
	}
	return s
}

func (s *rbKeySet[K]) put(x K) {
	k, h, byValue := s.idx.find(x)
	if !s.m[k] {
		s.m[k] = true
		if byValue {
			s.idx.add(k, h)
		}
	}
}

func (s *rbKeySet[K]) has(x K) bool {
	k, _, _ := s.idx.find(x)
	return s.m[k]
}

// rbHashSplat is `{ **o }` inside a Hash literal: o's pairs in order, later keys overwriting.
func rbHashSplat[K, V comparable](h, o *Hash[K, V]) *Hash[K, V] {
	for _, k := range o.keys {
		if v, ok := o.vals[k]; ok {
			Hash___Set(h, k, v)
		}
	}
	return h
}

// rbDig is Array#dig and Hash#dig: each key indexes the value before it,
// through its untyped view; nil ends the walk, as MRI's does.
func rbDig(v any, keys []any) any {
	for _, k := range keys {
		switch x := rbUnbox(v).(type) {
		case nil:
			return nil
		case interface{ _ToAny() *Array[any] }:
			i, ok := rbUnbox(k).(Integer)
			if !ok {
				panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(k) + " into Integer"))))
			}
			a := *x._ToAny()
			if i < 0 {
				i += Integer(len(a))
			}
			if i < 0 || int(i) >= len(a) {
				return nil
			}
			v = a[i]
		case interface{ _ToAny() *Hash[any, any] }:
			v, _ = x._ToAny().rbGet(rbUnbox(k))
		default:
			panic(NewTypeError(Ref(String(rbClassName(x) + " does not have #dig method"))))
		}
	}
	return rbUnbox(v)
}
