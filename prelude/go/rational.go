//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math/big"
	"regexp"
	"strings"
)

func rbRat(n, d int64) *Rational {
	out := &Rational{}
	out.v.SetFrac64(n, d)
	return out
}

func rbRatOp(a, b *Rational, op func(z, x, y *big.Rat) *big.Rat) *Rational {
	out := &Rational{}
	op(&out.v, &a.v, &b.v)
	return out
}

// rbBigToInt raises where MRI would answer a Bignum (decision 35).
func rbBigToInt(b *big.Int) Integer {
	if !b.IsInt64() {
		panic(NewRangeError(Ref(String(b.String() + " overflows Integer (64-bit; no Bignum)"))))
	}
	return Integer(b.Int64())
}

func rbRatFloor(r *big.Rat) Integer {
	q, _ := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int)) // Euclidean: the denominator is positive, so q is the floor
	return rbBigToInt(q)
}

var rbStrToRRe = regexp.MustCompile(`^\s*([-+]?(?:\d[\d_]*)?(?:\.\d[\d_]*)?(?:[eE][-+]?\d+)?)(?:/(\d[\d_]*))?`)

// rbStrToR is String#to_r: the leading rational ("1/3", "0.75", "-2.5e2", "1.5/2"), 0 when there is none; a zero
// denominator is MRI's ZeroDivisionError.
func rbStrToR(s string) *Rational {
	m := rbStrToRRe.FindStringSubmatch(s)
	out := &Rational{}
	num := strings.ReplaceAll(m[1], "_", "")
	if num == "" || num == "-" || num == "+" || strings.HasPrefix(strings.TrimLeft(num, "+-"), "e") {
		return out
	}
	if _, ok := out.v.SetString(num); !ok {
		return &Rational{}
	}
	if m[2] != "" {
		den, _ := new(big.Rat).SetString(strings.ReplaceAll(m[2], "_", ""))
		if den.Sign() == 0 {
			panic(NewZeroDivisionError(Ref(String("divided by 0"))))
		}
		out.v.Quo(&out.v, den)
	}
	return out
}
