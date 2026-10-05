//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type Range_Any interface{ _ToAny() *Range[any] }

// rbSuccAny is succ for a class whose override narrows succ's result type, so its Go name differs (decision 8): DateTime.
type rbSuccAny interface{ rbSuccAny() any }

// rbRangeEach is MRI's Range#each for the element types with succ.
func rbRangeEach[E comparable](r *Range[E], yield func(E) bool) {
	rbRangeNoBegin(r)
	switch b := any(r.b).(type) {
	case Integer:
		e, _ := any(r.e).(Integer)
		for i := b; r.endless || i < e || (!r.excl && i == e); i++ {
			if !yield(any(i).(E)) {
				return
			}
		}
	case String:
		rbStrUpto(string(b), string(any(r.e).(String)), r.excl, r.endless, func(s string) bool { return yield(any(String(s)).(E)) })
	default:
		succ := func(x E) E { return any(x).(interface{ Succ() E }).Succ() }
		if _, ok := any(r.b).(interface{ Succ() E }); !ok {
			if _, ok := any(r.b).(rbSuccAny); !ok {
				panic(NewTypeError(Ref(String("can't iterate from " + rbClassName(r.b)))))
			}
			succ = func(x E) E { return any(x).(rbSuccAny).rbSuccAny().(E) }
		}
		for x := r.b; r.endless || rbCmp(x, r.e) < 0 || (!r.excl && rbCmp(x, r.e) == 0); x = succ(x) {
			if !yield(x) {
				return
			}
		}
	}
}

// rbStrUpto is MRI's rb_str_upto_each, minus its all-digits numeric mode.
func rbStrUpto(b, e string, excl, endless bool, yield func(string) bool) {
	if endless {
		for cur := b; ; cur = rbStrSucc(cur) {
			if !yield(cur) {
				return
			}
		}
	}
	if len(b) == 1 && len(e) == 1 && b[0] < 0x80 && e[0] < 0x80 {
		for c := int(b[0]); c < int(e[0]) || (!excl && c == int(e[0])); c++ {
			if !yield(string(rune(c))) {
				return
			}
		}
		return
	}
	if n := strings.Compare(b, e); n > 0 || (excl && n == 0) {
		return
	}
	after := rbStrSucc(e)
	for cur := b; cur != after; {
		next, last := "", !excl && cur == e
		if !last {
			next = rbStrSucc(cur)
		}
		if !yield(cur) || last {
			return
		}
		cur = next
		if (excl && cur == e) || len(cur) > len(e) || cur == "" {
			return
		}
	}
}

func rbIsAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// rbStrSucc is String#succ over ASCII: alphanumerics carry leftward past punctuation.
func rbStrSucc(s string) string {
	b := []byte(s)
	if len(b) == 0 {
		return ""
	}
	first := -1
	for i := len(b) - 1; i >= 0; i-- {
		c := b[i]
		if !rbIsAlnum(c) {
			continue
		}
		first = i
		switch c {
		case 'z':
			b[i] = 'a'
		case 'Z':
			b[i] = 'A'
		case '9':
			b[i] = '0'
		default:
			b[i]++
			return string(b)
		}
	}
	if first < 0 {
		for i := len(b) - 1; i >= 0; i-- {
			if b[i] != 0xff {
				b[i]++
				return string(b)
			}
			b[i] = 0
		}
		return "\x01" + string(b)
	}
	carry := map[byte]byte{'a': 'a', 'A': 'A', '0': '1'}[b[first]]
	return string(b[:first]) + string(carry) + string(b[first:])
}

func rbRangeCover[E comparable](r *Range[E], v E) bool {
	if !r.beginless && rbCmp(r.b, v) > 0 {
		return false
	}
	if r.endless {
		return true
	}
	c := rbCmp(v, r.e)
	return c < 0 || (c == 0 && !r.excl)
}

func rbRangeIntCount(b, e Integer, excl bool) Integer {
	if excl {
		e--
	}
	return max(e-b+1, 0)
}

func rbRangeStr[E comparable](r *Range[E], str func(any) String) String {
	op := ".."
	if r.excl {
		op = "..."
	}
	switch {
	case r.endless:
		return str(r.b) + String(op)
	case r.beginless:
		return String(op) + str(r.e)
	}
	return str(r.b) + String(op) + str(r.e)
}

// rbRangeEmpty reports a range that covers nothing (3..1, 1...1).
func rbRangeEmpty[E comparable](r *Range[E]) bool {
	if r.endless || r.beginless {
		return false
	}
	c := rbCmp(r.b, r.e)
	return c > 0 || c == 0 && r.excl
}

// rbRangeOverlap is Range#overlap?: each range starts before the other ends.
func rbRangeOverlap[E comparable](a, b *Range[E]) bool {
	if rbRangeEmpty(a) || rbRangeEmpty(b) {
		return false
	}
	before := func(x, y *Range[E]) bool { // x starts no later than y ends
		if x.beginless || y.endless {
			return true
		}
		c := rbCmp(x.b, y.e)
		return c < 0 || c == 0 && !y.excl
	}
	return before(a, b) && before(b, a)
}

// rbRangeBsearch is Range#bsearch over Integer bounds; other element types raise TypeError, as MRI does for non-numeric ones.
func rbRangeBsearch[E comparable](r *Range[E], ok func(E) bool) *E {
	lo, isInt := any(r.b).(Integer)
	if !isInt || r.beginless {
		panic(NewTypeError(Ref(String("can't do binary search for " + rbClassName(r.b)))))
	}
	var hi Integer
	if r.endless {
		hi = lo + 1
		for !ok(any(hi).(E)) {
			if hi > math.MaxInt/2 {
				return nil
			}
			hi = lo + (hi-lo)*2
		}
	} else {
		hi = any(r.e).(Integer)
		if r.excl {
			hi--
		}
	}
	var found *E
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if v := any(mid).(E); ok(v) {
			found, hi = &v, mid-1
		} else {
			lo = mid + 1
		}
	}
	return found
}

// rbRangeNoBegin is MRI's TypeError for iterating a beginless range, which has no first element.
func rbRangeNoBegin[E comparable](r *Range[E]) {
	if r.beginless {
		panic(NewTypeError(Ref(String("can't iterate from NilClass"))))
	}
}
