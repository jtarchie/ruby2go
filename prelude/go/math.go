//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/big"
)

func rbMathDomain(bad Boolean, name string) {
	if bad {
		panic(NewMath_DomainError(Ref(String(`Numerical argument is out of domain - ` + name))))
	}
}

// Math functions are evaluated in 128-bit big.Float and rounded once, which is what
// libm (and so MRI) gives in all but rare hard cases; Go's math is only within 1 ulp.
const rbMathPrec = 128

func rbBF() *big.Float { return new(big.Float).SetPrec(rbMathPrec) }

func rbBFInt(n int64) *big.Float { return rbBF().SetInt64(n) }

var rbMathEps = new(big.Float).SetMantExp(big.NewFloat(1), -rbMathPrec-8)

// rbHalfPi is π/2 to 110 digits, enough to reduce arguments up to 1e30 exactly.
var rbHalfPi, _ = new(big.Float).SetPrec(400).SetString("1.57079632679489661923132169163975144209858469968755291048747229615390820314310449931401741267105853399107404326")

var rbPiBF = rbBF().Mul(rbHalfPi, rbBFInt(2))

// rbAtanhSeries is 2·atanh(z) = 2(z + z³/3 + z⁵/5 + …), for small |z|.
func rbAtanhSeries(z *big.Float) *big.Float {
	sum, term, z2 := rbBF().Set(z), rbBF().Set(z), rbBF().Mul(z, z)
	for n := int64(3); ; n += 2 {
		term.Mul(term, z2)
		t := rbBF().Quo(term, rbBFInt(n))
		sum.Add(sum, t)
		if rbBF().Abs(t).Cmp(rbMathEps) < 0 {
			return sum.Mul(sum, rbBFInt(2))
		}
	}
}

var rbLn2 = rbAtanhSeries(rbBF().Quo(rbBFInt(1), rbBFInt(3)))

var rbLn10 = rbBigLog(rbBFInt(10))

func rbBigLog(x *big.Float) *big.Float {
	m := rbBF()
	exp := x.MantExp(m) // x = m·2^exp, m in [0.5, 1)
	z := rbBF().Quo(rbBF().Sub(m, rbBFInt(1)), rbBF().Add(m, rbBFInt(1)))
	r := rbAtanhSeries(z)
	return r.Add(r, rbBF().Mul(rbLn2, rbBFInt(int64(exp))))
}

func rbBigExp(x *big.Float) *big.Float {
	kf, _ := rbBF().Quo(x, rbLn2).Float64()
	k := math.Round(kf)
	r := rbBF().Sub(x, rbBF().Mul(rbLn2, rbBF().SetFloat64(k)))
	const halvings = 8
	r.SetMantExp(r, -halvings)
	sum, term := rbBFInt(1), rbBFInt(1)
	for n := int64(1); ; n++ {
		term.Mul(term, r)
		term.Quo(term, rbBFInt(n))
		sum.Add(sum, term)
		if rbBF().Abs(term).Cmp(rbMathEps) < 0 {
			break
		}
	}
	for range halvings {
		sum.Mul(sum, sum)
	}
	return sum.SetMantExp(sum, int(k))
}

// rbBigSinCos is sin or cos of x after reduction by π/2 in 400 bits.
func rbBigSinCos(x float64, cos bool) *big.Float {
	bx := new(big.Float).SetPrec(400).SetFloat64(x)
	q, _ := new(big.Float).SetPrec(400).Quo(bx, rbHalfPi).Float64()
	k := math.Round(q)
	r := rbBF().Sub(bx, new(big.Float).SetPrec(400).Mul(rbHalfPi, new(big.Float).SetPrec(400).SetFloat64(k)))
	n := int(math.Mod(k, 4))
	if n < 0 {
		n += 4
	}
	if cos {
		n = (n + 1) % 4
	}
	// n: 0 sin r, 1 cos r, 2 -sin r, 3 -cos r
	sum, term, r2 := rbBFInt(1), rbBFInt(1), rbBF().Mul(r, r)
	start := int64(1)
	if n%2 == 0 {
		sum.Set(r)
		term.Set(r)
		start = 2
	}
	for i := start; ; i += 2 {
		term.Mul(term, r2)
		term.Quo(term, rbBFInt(i*(i+1)))
		term.Neg(term)
		sum.Add(sum, term)
		if rbBF().Abs(term).Cmp(rbMathEps) < 0 {
			break
		}
	}
	if n >= 2 {
		sum.Neg(sum)
	}
	return sum
}

// rbBigAtan reduces |z| to ≤ 1 and halves it twice before the Taylor series.
func rbBigAtan(z *big.Float) *big.Float {
	if z.Sign() == 0 {
		return rbBF()
	}
	neg := z.Sign() < 0
	a := rbBF().Abs(z)
	inv := a.Cmp(rbBFInt(1)) > 0
	if inv {
		a.Quo(rbBFInt(1), a)
	}
	for range 3 {
		s := rbBF().Sqrt(rbBF().Add(rbBFInt(1), rbBF().Mul(a, a)))
		a.Quo(a, s.Add(s, rbBFInt(1)))
	}
	sum, term, a2 := rbBF().Set(a), rbBF().Set(a), rbBF().Mul(a, a)
	for n := int64(3); ; n += 2 {
		term.Mul(term, a2)
		term.Neg(term)
		t := rbBF().Quo(term, rbBFInt(n))
		sum.Add(sum, t)
		if rbBF().Abs(t).Cmp(rbMathEps) < 0 {
			break
		}
	}
	sum.Mul(sum, rbBFInt(8))
	if inv {
		sum.Sub(rbBF().Set(rbHalfPi), sum)
	}
	if neg {
		sum.Neg(sum)
	}
	return sum
}

func rbBigAtan2(y, x *big.Float) float64 {
	if x.Sign() == 0 {
		r, _ := rbBF().Set(rbHalfPi).Float64()
		if y.Sign() < 0 {
			return -r
		}
		return r
	}
	a := rbBigAtan(rbBF().Quo(y, x))
	if x.Sign() < 0 {
		if y.Sign() < 0 {
			a.Sub(a, rbPiBF)
		} else {
			a.Add(a, rbPiBF)
		}
	}
	f, _ := a.Float64()
	return f
}

func rbF(b *big.Float) float64 {
	f, _ := b.Float64()
	return f
}

func rbFinite(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

func rbExp(x float64) float64 {
	if !rbFinite(x) || x > 710 || x < -746 {
		return math.Exp(x)
	}
	return rbF(rbBigExp(rbBF().SetFloat64(x)))
}

func rbLog(x float64) float64 {
	if !rbFinite(x) || x <= 0 {
		return math.Log(x)
	}
	return rbF(rbBigLog(rbBF().SetFloat64(x)))
}

func rbLogBase(x float64, base *big.Float) float64 {
	if !rbFinite(x) || x <= 0 {
		return math.Log(x)
	}
	return rbF(rbBF().Quo(rbBigLog(rbBF().SetFloat64(x)), base))
}

func rbSin(x float64) float64 {
	if !rbFinite(x) || math.Abs(x) > 1e30 || x == 0 {
		return math.Sin(x)
	}
	return rbF(rbBigSinCos(x, false))
}

func rbCos(x float64) float64 {
	if !rbFinite(x) || math.Abs(x) > 1e30 {
		return math.Cos(x)
	}
	return rbF(rbBigSinCos(x, true))
}

func rbTan(x float64) float64 {
	if !rbFinite(x) || math.Abs(x) > 1e30 || x == 0 {
		return math.Tan(x)
	}
	return rbF(rbBF().Quo(rbBigSinCos(x, false), rbBigSinCos(x, true)))
}

func rbAtan(x float64) float64 {
	if !rbFinite(x) || x == 0 {
		return math.Atan(x)
	}
	return rbF(rbBigAtan(rbBF().SetFloat64(x)))
}

func rbAtan2(y, x float64) float64 {
	if !rbFinite(x, y) || x == 0 || y == 0 {
		return math.Atan2(y, x)
	}
	return rbBigAtan2(rbBF().SetFloat64(y), rbBF().SetFloat64(x))
}

// rbAsin and rbAcos go through atan2 with √(1-x²) formed exactly.
func rbAsin(x float64) float64 {
	if !rbFinite(x) || x == 0 || math.Abs(x) >= 1 {
		return math.Asin(x)
	}
	bx := rbBF().SetFloat64(x)
	return rbBigAtan2(bx, rbBF().Sqrt(rbBF().Sub(rbBFInt(1), rbBF().Mul(bx, bx))))
}

func rbAcos(x float64) float64 {
	if !rbFinite(x) || math.Abs(x) >= 1 {
		return math.Acos(x)
	}
	bx := rbBF().SetFloat64(x)
	return rbBigAtan2(rbBF().Sqrt(rbBF().Sub(rbBFInt(1), rbBF().Mul(bx, bx))), bx)
}

// rbHyper is sinh (0), cosh (1) or tanh (2) from one exp.
func rbHyper(x float64, which int) float64 {
	if !rbFinite(x) || x == 0 || math.Abs(x) > 700 {
		return [...]func(float64) float64{math.Sinh, math.Cosh, math.Tanh}[which](x)
	}
	e := rbBigExp(rbBF().SetFloat64(x))
	inv := rbBF().Quo(rbBFInt(1), e)
	num := rbBF().Sub(e, inv)
	switch which {
	case 0:
		return rbF(num.Quo(num, rbBFInt(2)))
	case 1:
		s := rbBF().Add(e, inv)
		return rbF(s.Quo(s, rbBFInt(2)))
	}
	return rbF(num.Quo(num, rbBF().Add(e, inv)))
}

func rbCbrt(x float64) float64 {
	if !rbFinite(x) || x == 0 {
		return math.Cbrt(x)
	}
	bx, y := rbBF().SetFloat64(x), rbBF().SetFloat64(math.Cbrt(x))
	for range 3 {
		y2 := rbBF().Mul(y, y)
		y.Sub(y, rbBF().Quo(rbBF().Sub(rbBF().Mul(y2, y), bx), rbBF().Mul(y2, rbBFInt(3))))
	}
	return rbF(y)
}

func rbHypot(x, y float64) float64 {
	if !rbFinite(x, y) {
		return math.Hypot(x, y)
	}
	bx, by := rbBF().SetFloat64(x), rbBF().SetFloat64(y)
	return rbF(rbBF().Sqrt(rbBF().Add(rbBF().Mul(bx, bx), rbBF().Mul(by, by))))
}

// rbMathBig evaluates f in big precision and rounds once (decision 43). |x| below tiny
// returns x itself (f(x) ≈ x there); x == edge or a non-finite x goes to Go's math.
func rbMathBig(x, tiny, edge float64, f func(*big.Float) *big.Float) float64 {
	switch {
	case math.IsNaN(x) || math.IsInf(x, 0):
		return map[float64]func(float64) float64{-1: math.Log1p, 0: math.Expm1, 1: math.Acosh}[edge](x)
	case math.Abs(x) < tiny:
		return x
	case x == edge && edge == -1:
		return math.Inf(-1)
	case x == edge && edge == 1:
		return 0
	}
	return rbF(f(rbBF().SetFloat64(x)))
}

// rbAsinh is Math.asinh: ln(|x| + sqrt(x²+1)) with x's sign, in big precision.
func rbAsinh(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) < 1e-20 {
		return x
	}
	a := rbBF().SetFloat64(math.Abs(x))
	r := rbBF().Add(rbBF().Mul(a, a), rbBFInt(1))
	v := rbF(rbBigLog(r.Add(r.Sqrt(r), a)))
	return math.Copysign(v, x)
}

// rbSqrtPi is √π in rbMathPrec bits.
var rbSqrtPi = rbBF().Sqrt(rbPiBF)

// rbErfSeries is erf(x) = 2/√π · Σ (-1)ⁿ x^(2n+1) / (n!(2n+1)), for |x| < 6
// (the terms peak near e^(x²), which 128 bits still clear by 20 digits).
func rbErfSeries(x float64) *big.Float {
	bx := rbBF().SetFloat64(x)
	x2 := rbBF().Mul(bx, bx)
	sum, term := rbBF().Set(bx), rbBF().Set(bx) // term = (-1)ⁿ x^(2n+1)/n!
	for n := int64(1); ; n++ {
		term.Mul(term, x2)
		term.Quo(term, rbBFInt(-n))
		t := rbBF().Quo(term, rbBFInt(2*n+1))
		sum.Add(sum, t)
		if rbBF().Abs(t).Cmp(rbMathEps) < 0 {
			break
		}
	}
	sum.Mul(sum, rbBFInt(2))
	return sum.Quo(sum, rbSqrtPi)
}

// rbErfcCF is erfc(x) for x >= 3 by its continued fraction,
// e^(-x²)/√π · 1/(x + ½/(x + 1/(x + 3⁄2/(x + …)))), evaluated from the tail.
func rbErfcCF(x float64) *big.Float {
	bx := rbBF().SetFloat64(x)
	terms := int64(40 + 2000/(x*x))
	t := rbBF().Set(bx)
	for n := terms; n >= 1; n-- {
		t = rbBF().Add(bx, rbBF().Quo(rbBF().Quo(rbBFInt(n), rbBFInt(2)), t))
	}
	e := rbBigExp(rbBF().Neg(rbBF().Mul(bx, bx)))
	return e.Quo(e, rbBF().Mul(rbSqrtPi, t))
}

func rbErf(x float64) float64 {
	switch {
	case math.IsNaN(x) || x == 0:
		return x
	case math.Abs(x) >= 6:
		return math.Copysign(1, x)
	}
	return rbF(rbErfSeries(x))
}

func rbErfc(x float64) float64 {
	switch {
	case math.IsNaN(x):
		return x
	case x < -6:
		return 2
	case x > 27:
		return 0
	case x >= 3:
		return rbF(rbErfcCF(x))
	case x <= -3:
		return rbF(rbBF().Sub(rbBFInt(2), rbErfcCF(-x)))
	}
	return rbF(rbBF().Sub(rbBFInt(1), rbErfSeries(x)))
}

// rbBernoulli are B₂, B₄, …, B₄₀ (Akiyama–Tanigawa), for Stirling's series.
var rbBernoulli = func() []*big.Rat {
	const n = 40
	a := make([]*big.Rat, n+1)
	var out []*big.Rat
	for m := 0; m <= n; m++ {
		a[m] = big.NewRat(1, int64(m+1))
		for j := m; j >= 1; j-- {
			d := new(big.Rat).Sub(a[j-1], a[j])
			a[j-1] = d.Mul(d, big.NewRat(int64(j), 1))
		}
		if m >= 2 && m%2 == 0 {
			out = append(out, new(big.Rat).Set(a[0]))
		}
	}
	return out
}()

// rbLnGammaBig is ln Γ(z) for z >= 30 by Stirling's series.
func rbLnGammaBig(z *big.Float) *big.Float {
	half := rbBF().Quo(rbBFInt(1), rbBFInt(2))
	r := rbBF().Mul(rbBF().Sub(z, half), rbBigLog(z))
	r.Sub(r, z)
	r.Add(r, rbBF().Mul(half, rbBigLog(rbBF().Mul(rbPiBF, rbBFInt(2)))))
	zk, z2 := rbBF().Set(z), rbBF().Mul(z, z) // z^(2k-1)
	for i, b := range rbBernoulli {
		k := int64(i + 1)
		t := rbBF().SetRat(b)
		t.Quo(t, rbBF().Mul(rbBFInt(2*k*(2*k-1)), zk))
		r.Add(r, t)
		zk.Mul(zk, z2)
	}
	return r
}

// rbLnGammaShift is ln|Γ(x)| and Γ's sign: Γ(x) = Γ(x+N)/(x(x+1)…(x+N-1)), with x+N >= 30.
func rbLnGammaShift(x float64) (*big.Float, int) {
	bx := rbBF().SetFloat64(x)
	prod := rbBFInt(1)
	for bx.Cmp(rbBFInt(30)) < 0 {
		prod.Mul(prod, bx)
		bx.Add(bx, rbBFInt(1))
	}
	sign := prod.Sign()
	return rbBF().Sub(rbLnGammaBig(bx), rbBigLog(prod.Abs(prod))), sign
}

func rbGamma(x float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsInf(x, 1):
		return x
	case x == 0:
		return math.Copysign(math.Inf(1), x)
	case x == math.Trunc(x) && x < 0, math.IsInf(x, -1):
		rbMathDomain(true, "gamma")
	case x > 171.7:
		return math.Inf(1)
	}
	lg, sign := rbLnGammaShift(x)
	g := rbBigExp(lg)
	if sign < 0 {
		g.Neg(g)
	}
	return rbF(g)
}

func rbLgamma(x float64) (float64, int) {
	switch {
	case math.IsInf(x, -1):
		rbMathDomain(true, "lgamma")
	case math.IsNaN(x):
		return x, 1
	case math.IsInf(x, 1):
		return x, 1
	case x == 0:
		if math.Signbit(x) {
			return math.Inf(1), -1
		}
		return math.Inf(1), 1
	case x == math.Trunc(x) && x < 0:
		return math.Inf(1), 1
	case x == 1 || x == 2:
		return 0, 1
	}
	lg, sign := rbLnGammaShift(x)
	return rbF(lg), sign
}
