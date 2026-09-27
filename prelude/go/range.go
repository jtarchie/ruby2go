//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type Range_Any interface{ _ToAny() *Range[any] }

// rbRangeEach is MRI's Range#each for the element types with succ.
func rbRangeEach[E comparable](r *Range[E], yield func(E) bool) {
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
		panic(NewTypeError(Ref(String("can't iterate from " + rbClassName(r.b)))))
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
	if rbCmp(r.b, v) > 0 {
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
	if r.endless {
		return str(r.b) + String(op)
	}
	return str(r.b) + String(op) + str(r.e)
}
