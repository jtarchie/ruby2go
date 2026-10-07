//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"strconv"
	"strings"
)

// rbStrictInt is Kernel#Integer(str, base): String#to_i's syntax, but the
// whole string bar surrounding whitespace must be the number. Base 0 (or
// -1) reads a 0b/0o/0d/0x prefix or a leading 0 as octal; another base
// still allows its own prefix.
func rbStrictInt(s String, base int) Integer {
	if base == 1 || base > 36 || base < -36 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(base)))))
	}
	bad := func() {
		panic(NewArgumentError(Ref("invalid value for Integer(): " + rbStringInspect(string(s)))))
	}
	t := strings.Trim(string(s), " \t\n\v\f\r")
	sign := ""
	if t != "" && (t[0] == '+' || t[0] == '-') {
		sign, t = t[:1], t[1:]
	}
	auto := base == 0 || base == -1
	if base < 0 {
		base = -base
	}
	prefixed := false
	if len(t) >= 2 {
		pb := map[string]int{"0b": 2, "0o": 8, "0d": 10, "0x": 16}[strings.ToLower(t[:2])]
		if pb != 0 && (auto || pb == base) {
			base, t, prefixed = pb, t[2:], true
		}
	}
	if auto && !prefixed {
		base = 10
		if len(t) > 1 && t[0] == '0' {
			base = 8
		}
	}
	if t == "" || t[0] == '_' || t[len(t)-1] == '_' || strings.Contains(t, "__") || strings.ContainsAny(t, "+-") {
		bad()
	}
	digits := strings.ReplaceAll(t, "_", "")
	n, err := strconv.ParseInt(sign+digits, base, 64)
	if errors.Is(err, strconv.ErrRange) { // MRI returns a Bignum (decision 35)
		panic(NewRangeError(Ref(String(sign + digits + " overflows Integer (64-bit; no Bignum)"))))
	}
	if err != nil {
		bad()
	}
	return Integer(n)
}

// rbIntShl is Integer#<< (a negative count shifts right, filling with the sign).
func rbIntShl(x, n Integer) Integer {
	if n <= 0 {
		return x >> min(-n, 63)
	}
	if x == 0 {
		return 0
	}
	if n >= 63 || x<<n>>n != x {
		rbIntOverflow(x, "<<", n)
	}
	return x << n
}

// rbIntRoundTo is Integer#floor/ceil/truncate/round(digits) (mode 0..3); digits >= 0 leave x as is.
func rbIntRoundTo(x, digits Integer, mode int) Integer {
	if digits >= 0 {
		return x
	}
	if digits < -18 { // 10**19 is past int64: only 0 or an overflow is left
		over := mode == 0 && x < 0 || mode == 1 && x > 0 || mode == 3 && (x >= 5e18 || x <= -5e18)
		if over {
			rbIntOverflow(x, "round to", digits)
		}
		return 0
	}
	p := Integer(1)
	for range -digits {
		p *= 10
	}
	q, r := x/p, x%p
	switch {
	case mode == 0 && r < 0:
		q--
	case mode == 1 && r > 0:
		q++
	case mode == 3 && 2*r >= p:
		q++
	case mode == 3 && 2*r <= -p:
		q--
	}
	v, ok := rbIntMulOk(q, p)
	if !ok {
		rbIntOverflow(x, "round to", digits)
	}
	return v
}
