//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

// rbIntWrap is --int-overflow=wrap (decision 372): Integer +, -, * and -@ wrap as Go's int. The compiler flips the
// constant; a constant `if` costs nothing in Go's inlining budget, so the default build is the same code.
const rbIntWrap = false

// rbIntChecked* are Integer +, -, * and -@ with their overflow checks, for a file whose own `# int_overflow:` comment
// is raise or promote in a --int-overflow=wrap program, where rbIntWrap skips the checks in the methods (decision 381).
func rbIntCheckedplus(a, b Integer) Integer {
	r := a + b
	if (r > a) != (b > 0) {
		panic(rbIntOverflowed{a, b, "+"})
	}
	return r
}

func rbIntCheckedminus(a, b Integer) Integer {
	r := a - b
	if (r < a) != (b > 0) {
		panic(rbIntOverflowed{a, b, "-"})
	}
	return r
}

func rbIntCheckedmul(a, b Integer) Integer {
	r := a * b
	if (uint64(a)+1<<31)|(uint64(b)+1<<31) >= 1<<32 {
		hi, _ := bits.Mul64(uint64(a), uint64(b))
		if Integer(hi)-(a>>63&b)-(b>>63&a) != r>>63 {
			panic(rbIntOverflowed{a, b, "*"})
		}
	}
	return r
}

func rbIntCheckedneg(a Integer) Integer {
	if a == math.MinInt {
		panic(rbIntOverflowed{0, a, "-"})
	}
	return -a
}

// rbStrictInt is Kernel#Integer(str, base): String#to_i's syntax, but the
// whole string bar surrounding whitespace must be the number. Base 0 (or
// -1) reads a 0b/0o/0d/0x prefix or a leading 0 as octal; another base
// still allows its own prefix.
func rbStrictInt(s String, base int) Integer {
	if base == 1 || base > 36 || base < -36 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(base)))))
	}
	if n, ok := rbDigits(string(s)); ok && (base == 10 || (base == 0 || base == -1) && (len(s) == 1 || s[0] != '0')) {
		return Integer(n) // plain decimal digits: the common case, without the parse below
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
		pb := 0
		if t[0] == '0' {
			switch t[1] | 0x20 { // lower-cased
			case 'b':
				pb = 2
			case 'o':
				pb = 8
			case 'd':
				pb = 10
			case 'x':
				pb = 16
			}
		}
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
		return x >> -max(n, -63) // -MinInt would wrap
	}
	if x == 0 {
		return 0
	}
	if n >= 64 || x<<n>>n != x { // n == 63 fits only for -1
		rbIntOverflow(x, "<<", n)
	}
	return x << n
}

// rbIntShr is Integer#>>; a left shift by 2**63 (-MinInt) is a Bignum past anything but 0.
func rbIntShr(x, n Integer) Integer {
	if n == math.MinInt {
		if x != 0 {
			rbIntOverflow(x, ">>", n)
		}
		return 0
	}
	return rbIntShl(x, -n)
}

// rbIntRoundTo is Integer#floor/ceil/truncate/round(digits) (mode 0..3); digits >= 0 leave x as is.
func rbIntRoundTo(x, digits Integer, mode int) Integer {
	rbIntDigitsArg(digits)
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

// rbBigLiteral is an Integer literal past 64 bits, evaluated: MRI's Bignum, here RangeError (decisions 35, 361).
func rbBigLiteral(lit string) Integer {
	panic(NewRangeError(Ref(String(lit + " overflows Integer (64-bit; no Bignum)"))))
}
