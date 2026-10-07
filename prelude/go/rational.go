//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math/big"
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
	q, m := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int))
	_ = m // Euclidean: the denominator is positive, so q is the floor
	return rbBigToInt(q)
}
