//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/big"
	"strings"
)

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
			if i == math.MaxInt { // i++ would wrap; MRI goes on into a Bignum only for an endless range
				if r.endless {
					rbIntOverflow(i, "+", 1)
				}
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

// rbRangeIntSum is the sum of b..e (b <= e), a RangeError where MRI's is a Bignum (decision 35).
func rbRangeIntSum(b, e Integer) Integer {
	if b > -1<<30 && b < 1<<30 && e > -1<<30 && e < 1<<30 { // (e-b+1)*(b+e) fits
		return (e - b + 1) * (b + e) / 2
	}
	n := new(big.Int).Sub(big.NewInt(int64(e)), big.NewInt(int64(b)))
	s := n.Add(n, big.NewInt(1)).Mul(n, big.NewInt(0).Add(big.NewInt(int64(b)), big.NewInt(int64(e))))
	if s.Rsh(s, 1); !s.IsInt64() {
		rbIntOverflow(b, "..sum..", e)
	}
	return Integer(s.Int64())
}

// rbRangeInclude is Range#include?: cover? for numbers, but String ranges iterate (MRI's rb_str_include_range_p), so ("a".."z") holds "b" and not "bb".
func rbRangeInclude[E comparable](r *Range[E], v E) bool {
	b, bok := any(r.b).(String)
	e, eok := any(r.e).(String)
	if !bok && !eok {
		return rbRangeCover(r, v)
	}
	if r.beginless || r.endless {
		panic(NewTypeError(Ref(String("cannot determine inclusion in beginless/endless ranges"))))
	}
	vs, ok := rbUnbox(any(v)).(String)
	if !ok || len(vs) > len(e) || len(vs) < len(b) {
		return false
	}
	found := false
	rbStrUpto(string(b), string(e), r.excl, false, func(s string) bool {
		found = s == string(vs)
		return !found
	})
	return found
}

// rbRangeIntCount is the number of integers in b..e; one past MaxInt is MRI's Bignum, so a RangeError (decision 35).
func rbRangeIntCount(b, e Integer, excl bool) Integer {
	if excl {
		if e == math.MinInt {
			return 0
		}
		e--
	}
	if e < b {
		return 0
	}
	if uint64(e)-uint64(b) >= math.MaxInt {
		rbIntOverflow(b, "..", e)
	}
	return e - b + 1
}

func rbRangeStr[E comparable](r *Range[E], str func(any) String) String {
	op := ".."
	if r.excl {
		op = "..."
	}
	switch {
	case r.inf: // `1..Float::INFINITY`, an endless Integer range (decision 140)
		return str(r.b) + String(op) + "Infinity"
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
		for step := Integer(1); ; step *= 2 { // MRI probes lo+1, lo+2, lo+4, ... into Bignums
			if lo > math.MaxInt-step {
				rbIntOverflow(lo, "+", step)
			}
			if hi = lo + step; ok(any(hi).(E)) {
				break
			}
			if step > math.MaxInt/2 {
				rbIntOverflow(lo, "+", step)
			}
		}
	} else {
		hi = any(r.e).(Integer)
		if r.excl {
			if hi == math.MinInt {
				return nil
			}
			hi--
		}
	}
	var found *E
	for lo <= hi {
		mid := lo + Integer(uint64(hi-lo)/2) // hi-lo wraps past MaxInt; as unsigned it is exact
		if v := any(mid).(E); ok(v) {
			found = &v
			if mid == lo { // mid-1 would wrap
				break
			}
			hi = mid - 1
		} else {
			if mid == hi { // mid+1 would wrap
				break
			}
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
