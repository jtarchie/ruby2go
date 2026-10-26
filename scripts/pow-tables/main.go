// Generates the tables of rbPowFast (prelude/go/floatpow.go) with math/big.
package main

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
)

func hx(f float64) string {
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'x', -1, 64)
}

const prec = 300

func bf(x float64) *big.Float { return new(big.Float).SetPrec(prec).SetFloat64(x) }

func blog(v *big.Float) *big.Float {
	// log v = 2 atanh(s), s = (v-1)/(v+1); v in [0.5, 2]
	one := bf(1)
	num := new(big.Float).SetPrec(prec).Sub(v, one)
	den := new(big.Float).SetPrec(prec).Add(v, one)
	s := new(big.Float).SetPrec(prec).Quo(num, den)
	s2 := new(big.Float).SetPrec(prec).Mul(s, s)
	sum := new(big.Float).SetPrec(prec)
	term := new(big.Float).SetPrec(prec).Set(s)
	for n := 0; n < 400; n++ {
		t := new(big.Float).SetPrec(prec).Quo(term, bf(float64(2*n+1)))
		sum.Add(sum, t)
		term.Mul(term, s2)
		if term.Sign() == 0 || term.MantExp(nil) < -prec-10 {
			break
		}
	}
	return sum.Mul(sum, bf(2))
}

func bexp(v *big.Float) *big.Float { // |v| <= 1
	sum := bf(1)
	term := bf(1)
	for n := 1; n < 200; n++ {
		term.Mul(term, v)
		term.Quo(term, bf(float64(n)))
		sum.Add(sum, term)
	}
	return sum
}

func dd(v *big.Float) (float64, float64) {
	h, _ := v.Float64()
	r := new(big.Float).SetPrec(prec).Sub(v, bf(h))
	l, _ := r.Float64()
	return h, l
}

// roundBits rounds v to n significant bits.
func roundBits(v *big.Float, n uint) float64 {
	f, _ := new(big.Float).SetPrec(n).SetMode(big.ToNearestEven).Set(v).Float64()
	return f
}

func main() {
	out := os.Stdout
	ln2 := blog(bf(2))
	const off = 0x3fe6955500000000
	one := math.Float64bits(1)
	fmt.Fprintln(out, "var rbPowLogT = [128][3]float64{ // 1/c, log(c) as a double-double, per interval of z")
	for i := uint64(0); i < 128; i++ {
		za := math.Float64frombits(off + i<<45)
		zb := math.Float64frombits(off + (i+1)<<45 - 1) // last z in the interval
		var invc float64
		if math.Float64bits(za) <= one && one <= math.Float64bits(zb) {
			invc = 1
		} else {
			c := new(big.Float).SetPrec(prec).Add(bf(za), bf(zb))
			c.Quo(c, bf(2))
			inv := new(big.Float).SetPrec(prec).Quo(bf(1), c)
			q := 7.0
			if za >= 1 {
				q = 8
			}
			f, _ := inv.Float64()
			invc = math.Round(f*math.Exp2(q)) / math.Exp2(q)
			// r = z*invc - 1 exact: its lowest bit is ulp(z)*2^-q, so |r| < 2^53 * that.
			for _, z := range []float64{za, zb} {
				r := new(big.Float).SetPrec(prec).Mul(bf(z), bf(invc))
				r.Sub(r, bf(1))
				ulp := math.Exp2(-52)
				if z < 1 {
					ulp = math.Exp2(-53)
				}
				rf, _ := r.Float64()
				if math.Abs(rf) >= math.Exp2(53)*ulp*math.Exp2(-q) || math.Abs(rf) >= 0x1p-7 {
					panic(fmt.Sprintf("interval %d: r %g not exact", i, rf))
				}
			}
		}
		lc := blog(bf(invc))
		lc.Neg(lc)
		h, l := dd(lc)
		fmt.Fprintf(out, "\t{%s, %s, %s},\n", hx(invc), hx(h), hx(l))
	}
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out, "\nvar rbPowExpT = [128][2]float64{ // 2^(j/128) as a double-double")
	for j := 0; j < 128; j++ {
		v := new(big.Float).SetPrec(prec).Mul(ln2, bf(float64(j)/128))
		h, l := dd(bexp(v))
		fmt.Fprintf(out, "\t{%s, %s},\n", hx(h), hx(l))
	}
	fmt.Fprintln(out, "}")
	ln2h := roundBits(ln2, 42)
	ln2l, _ := new(big.Float).SetPrec(prec).Sub(ln2, bf(ln2h)).Float64()
	ln2n := new(big.Float).SetPrec(prec).Quo(ln2, bf(128))
	l1 := roundBits(ln2n, 35)
	l2, _ := new(big.Float).SetPrec(prec).Sub(ln2n, bf(l1)).Float64()
	n2, _ := new(big.Float).SetPrec(prec).Quo(bf(128), ln2).Float64()
	fmt.Fprintf(out, "\nconst (\n\trbPowLn2h = %s\n\trbPowLn2l = %s\n\trbPowL1 = %s\n\trbPowL2 = %s\n\trbPowN2 = %s\n)\n", hx(ln2h), hx(ln2l), hx(l1), hx(l2), hx(n2))
}
