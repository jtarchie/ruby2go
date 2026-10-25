//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/big"
	"strconv"
)

// rbBigV is Bignum (decision 372): b only past 64 bits, so small values stay unboxed and == on two small ones is Go's.
type rbBigV struct {
	i Integer
	b *big.Int
}

// rbBigOf is b as a Bignum, small when it fits.
func rbBigOf(b *big.Int) Bignum {
	if b.IsInt64() {
		return Bignum{i: Integer(b.Int64())}
	}
	return Bignum{b: b}
}

// rbBigBig is x as a *big.Int the caller must not mutate.
func rbBigBig(x Bignum) *big.Int {
	if x.b != nil {
		return x.b
	}
	return big.NewInt(int64(x.i))
}

// rbBigIn is rbBigBig into z, so a small operand of a big operation does not allocate.
func rbBigIn(x Bignum, z *big.Int) *big.Int {
	if x.b != nil {
		return x.b
	}
	return z.SetInt64(int64(x.i))
}

// rbBigOp is a big operation on two operands, either of which may be small.
func rbBigOp(x, y Bignum, op func(z, a, b *big.Int) *big.Int) Bignum {
	var zx, zy big.Int
	return rbBigOf(op(new(big.Int), rbBigIn(x, &zx), rbBigIn(y, &zy)))
}

// rbBigDigits is Integer#digits: decimal digits, least significant first.
func rbBigDigits(x Bignum) *Array[Integer] {
	s := rbBigBig(x).String()
	out := &Array[Integer]{s: make([]Integer, len(s))}
	for i := range len(s) {
		out.s[i] = Integer(s[len(s)-1-i] - '0')
	}
	return out
}

// rbBigLit is an Integer literal past 64 bits (decision 372; a compile error's RangeError otherwise, decision 361).
func rbBigLit(s string) Bignum {
	b, _ := new(big.Int).SetString(s, 0)
	return rbBigOf(b)
}

// rbBignumToInt is x where an Integer is wanted: MRI's RangeError when it does not fit 64 bits.
func rbBignumToInt(x Bignum) Integer {
	if x.b != nil {
		panic(NewRangeError(Ref(String("bignum too big to convert into 'long'"))))
	}
	return x.i
}

func rbBigAdd(x, y Bignum) Bignum {
	if x.b == nil && y.b == nil {
		if r := x.i + y.i; (r > x.i) == (y.i > 0) {
			return Bignum{i: r}
		}
	}
	return rbBigOp(x, y, (*big.Int).Add)
}

func rbBigSub(x, y Bignum) Bignum {
	if x.b == nil && y.b == nil {
		if r := x.i - y.i; (r < x.i) == (y.i > 0) {
			return Bignum{i: r}
		}
	}
	return rbBigOp(x, y, (*big.Int).Sub)
}

func rbBigMul(x, y Bignum) Bignum {
	if x.b == nil && y.b == nil {
		if r, ok := rbIntMulOk(x.i, y.i); ok {
			return Bignum{i: r}
		}
	}
	return rbBigOp(x, y, (*big.Int).Mul)
}

// rbBigDivMod is Ruby's floored division and modulo; Go's big.Int.DivMod is Euclidean, so a negative divisor differs.
func rbBigDivMod(x, y Bignum) (Bignum, Bignum) {
	if x.b == nil && y.b == nil && (y.i != -1 || x.i != math.MinInt) {
		return Bignum{i: x.i.Op_div(y.i)}, Bignum{i: x.i.Op_mod(y.i)} // a zero divisor is Go's panic: ZeroDivisionError (rbWrapPanic)
	}
	var zx, zy big.Int
	d := rbBigIn(y, &zy)
	if d.Sign() == 0 {
		panic(NewZeroDivisionError(Ref(String("divided by 0"))))
	}
	q, r := new(big.Int).QuoRem(rbBigIn(x, &zx), d, new(big.Int))
	if r.Sign() != 0 && r.Sign() != d.Sign() {
		q.Sub(q, big.NewInt(1))
		r.Add(r, d)
	}
	return rbBigOf(q), rbBigOf(r)
}

func rbBigNeg(x Bignum) Bignum {
	if x.b == nil && x.i != math.MinInt {
		return Bignum{i: -x.i}
	}
	return rbBigOf(new(big.Int).Neg(rbBigBig(x)))
}

// rbBigPow is x ** e; a negative e answers as Integer#** does (Rational aside, decision 35).
func rbBigPow(x Bignum, e Integer) Bignum {
	if e < 0 && x.b == nil && x.i == 1 {
		return x
	}
	if e < 0 && x.b == nil && x.i == -1 {
		return Bignum{i: 1 - 2*(e&1)}
	}
	if e < 0 && x.b == nil && x.i == 0 {
		panic(NewZeroDivisionError(Ref[String]("divided by 0")))
	}
	if e < 0 {
		panic(NewRangeError(Ref(String(string(rbBigToS(x, 10)) + " ** " + strconv.Itoa(int(e)) + " is a Rational (Integer#** cannot return one)"))))
	}
	if x.b == nil {
		r, base, ok := Integer(1), x.i, true
		for n := e; ok; {
			if n&1 != 0 {
				if r, ok = rbIntMulOk(r, base); !ok {
					break
				}
			}
			if n >>= 1; n == 0 {
				return Bignum{i: r}
			}
			base, ok = rbIntMulOk(base, base)
		}
	}
	return rbBigOf(new(big.Int).Exp(rbBigBig(x), big.NewInt(int64(e)), nil))
}

// rbBigShl is x << n; a negative n shifts right, as MRI's.
func rbBigShl(x Bignum, n Integer) Bignum {
	if n < 0 {
		return rbBigShr(x, -n)
	}
	if x.b == nil && n < 63 && (x.i<<n)>>n == x.i {
		return Bignum{i: x.i << n}
	}
	return rbBigOf(new(big.Int).Lsh(rbBigBig(x), uint(n)))
}

// rbBigShr is x >> n, rounding toward negative infinity as MRI's (big.Int.Rsh is arithmetic too).
func rbBigShr(x Bignum, n Integer) Bignum {
	if n < 0 {
		return rbBigShl(x, -n)
	}
	if x.b == nil {
		if n > 63 {
			n = 63
		}
		return Bignum{i: x.i >> n}
	}
	return rbBigOf(new(big.Int).Rsh(x.b, uint(n)))
}

func rbBigCmp(x, y Bignum) int {
	if x.b == nil && y.b == nil {
		switch {
		case x.i < y.i:
			return -1
		case x.i > y.i:
			return 1
		}
		return 0
	}
	var zx, zy big.Int
	return rbBigIn(x, &zx).Cmp(rbBigIn(y, &zy))
}

func rbBigToS(x Bignum, base Integer) String {
	if base < 2 || base > 36 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(int(base))))))
	}
	if x.b == nil {
		return String(strconv.FormatInt(int64(x.i), int(base)))
	}
	return String(x.b.Text(int(base)))
}

func rbBigToF(x Bignum) Float {
	if x.b == nil {
		return Float(x.i)
	}
	f, _ := new(big.Float).SetInt(x.b).Float64()
	return Float(f)
}

// rbBigFromFloat is Float#to_i (and floor, ceil, round, truncate after rounding) past 64 bits.
func rbBigFromFloat(f Float) Bignum {
	if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
		panic(NewFloatDomainError(Ref(rbFloatToS(float64(f)))))
	}
	if f >= -(1<<63) && f < 1<<63 {
		return Bignum{i: Integer(f)}
	}
	b, _ := big.NewFloat(float64(f)).Int(nil)
	return rbBigOf(b)
}

// rbBigAny is x as the Bignum an untyped value holds: an Integer or a Bignum.
func rbBigAny(v any) (Bignum, bool) {
	switch v := v.(type) {
	case Bignum:
		return v, true
	case Integer:
		return Bignum{i: v}, true
	}
	return Bignum{}, false
}
