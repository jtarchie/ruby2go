//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// rbNumArg checks that a Complex part is a real number.
func rbNumArg(a any) any {
	a = rbUnbox(a)
	switch a.(type) {
	case Integer, Float, *Rational:
		return a
	}
	name := rbClassName(a)
	if a == nil {
		name = "nil"
	}
	panic(NewTypeError(Ref(String("can't convert " + name + " into Complex"))))
}

// rbNumLevel orders the tower: Integer < Rational < Float.
func rbNumLevel(a any) int {
	switch a.(type) {
	case *Rational:
		return 1
	case Float:
		return 2
	}
	return 0
}

func rbNumNeg(a any) any {
	switch v := a.(type) {
	case Integer:
		return v.Op_neg()
	case Float:
		return -v
	}
	return a.(*Rational).Op_neg()
}

func rbNumZero(a any) bool {
	switch v := a.(type) {
	case Integer:
		return v == 0
	case Float:
		return v == 0
	}
	return a.(*Rational).v.Sign() == 0
}

// rbNumExactZero is MRI's f_zero_p on an exact part: a Float zero is not exact.
func rbNumExactZero(a any) bool {
	_, isFloat := a.(Float)
	return !isFloat && rbNumZero(a)
}

func rbComplexMul(a, b *Complex) *Complex {
	re := rbNumOp('-', rbNumOp('*', a.re, b.re), rbNumOp('*', a.im, b.im))
	im := rbNumOp('+', rbNumOp('*', a.re, b.im), rbNumOp('*', a.im, b.re))
	return &Complex{re, im}
}

// rbComplexQuo is MRI's f_divide: exact parts divide exactly, Floats scale by the larger part.
func rbComplexQuo(a, b *Complex) *Complex {
	if rbNumLevel(b.re) == 2 || rbNumLevel(b.im) == 2 || rbNumLevel(a.re) == 2 || rbNumLevel(a.im) == 2 {
		ar, ai, br, bi := rbNumFloat(a.re), rbNumFloat(a.im), rbNumFloat(b.re), rbNumFloat(b.im)
		if math.Abs(br) >= math.Abs(bi) {
			r := bi / br
			n := br * (1 + r*r)
			return &Complex{Float((ar + ai*r) / n), Float((ai - ar*r) / n)}
		}
		r := br / bi
		n := bi * (1 + r*r)
		return &Complex{Float((ar*r + ai) / n), Float((ai*r - ar) / n)}
	}
	if rbNumZero(b.re) && rbNumZero(b.im) {
		panic(NewZeroDivisionError(Ref(String("divided by 0"))))
	}
	d := rbNumOp('+', rbNumOp('*', b.re, b.re), rbNumOp('*', b.im, b.im))
	num := rbComplexMul(a, &Complex{b.re, rbNumNeg(b.im)})
	return &Complex{rbNumCanon(rbNumOp('/', num.re, d)), rbNumCanon(rbNumOp('/', num.im, d))}
}

// rbNumCanon turns a whole Rational back into an Integer, as MRI's quo of Integers does inside Complex.
func rbNumCanon(a any) any {
	if r, ok := a.(*Rational); ok && r.v.IsInt() && r.v.Num().IsInt64() {
		return Integer(r.v.Num().Int64())
	}
	return a
}

func rbComplexPowInt(c *Complex, k int) *Complex {
	if k == 0 {
		return &Complex{Integer(1), Integer(0)}
	}
	if k < 0 {
		return rbComplexPowInt(rbComplexQuo(&Complex{Integer(1), Integer(0)}, c), -k)
	}
	x, z := c, c
	for n := k - 1; n > 0; n-- {
		for n%2 == 0 {
			x = rbComplexMul(x, x)
			n /= 2
		}
		z = rbComplexMul(z, x)
	}
	return z
}

func rbComplexAbsF(c *Complex) float64 {
	return math.Hypot(rbNumFloat(c.re), rbNumFloat(c.im))
}

func rbComplexAbs(c *Complex) any {
	if rbNumExactZero(c.re) {
		return rbNumAbs(c.im)
	}
	if rbNumExactZero(c.im) {
		return rbNumAbs(c.re)
	}
	return Float(rbComplexAbsF(c))
}

func rbComplexArg(c *Complex) float64 {
	return math.Atan2(rbNumFloat(c.im), rbNumFloat(c.re))
}

// rbComplexPolar is MRI's f_complex_polar_real.
func rbComplexPolar(r, theta any) *Complex {
	if rbNumZero(theta) {
		return &Complex{r, Float(0)}
	}
	if rbNumZero(r) {
		return &Complex{r, Float(0)}
	}
	if f, ok := theta.(Float); ok {
		switch float64(f) {
		case math.Pi:
			return &Complex{rbNumNeg(r), Float(0)}
		case math.Pi / 2:
			return &Complex{Float(0), r}
		}
	}
	rf, tf := rbNumFloat(r), rbNumFloat(theta)
	return &Complex{Float(rf * math.Cos(tf)), Float(rf * math.Sin(tf))}
}

// rbComplexReal raises MRI's RangeError when a conversion would drop a non-exact-zero imaginary part.
func rbComplexReal(c *Complex) {
	if !rbNumExactZero(c.im) {
		panic(NewRangeError(Ref(String("can't convert " + rbComplexFmt(c, rbToS) + " into " + "Integer"))))
	}
}

// rbComplexFmt is MRI's f_format: a part ending in a non-digit gets "*" before the "i".
func rbComplexFmt(c *Complex, str func(any) String) string {
	s := string(str(c.re))
	neg := false
	switch v := c.im.(type) {
	case Float:
		neg = math.Signbit(float64(v)) && !math.IsNaN(float64(v))
	case Integer:
		neg = v < 0
	case *Rational:
		neg = v.v.Sign() < 0
	}
	if neg {
		s += "-"
	} else {
		s += "+"
	}
	im := string(str(rbNumAbs(c.im)))
	s += im
	if last := im[len(im)-1]; last < '0' || last > '9' {
		s += "*"
	}
	return s + "i"
}

// rbComplexScalar is Complex op real (left: real op Complex), which MRI applies part by part rather than promoting the real.
func rbComplexScalar(op byte, c *Complex, r any, left bool) *Complex {
	switch {
	case op == '*':
		return &Complex{rbNumOp('*', c.re, r), rbNumOp('*', c.im, r)}
	case op == '/':
		return &Complex{rbNumCanon(rbNumOp('/', c.re, r)), rbNumCanon(rbNumOp('/', c.im, r))}
	case left && op == '-':
		return &Complex{rbNumOp('-', r, c.re), rbNumNeg(c.im)}
	case left:
		return &Complex{rbNumOp(op, r, c.re), c.im}
	}
	return &Complex{rbNumOp(op, c.re, r), c.im}
}

// rbComplexCmp is MRI's nucomp_cmp: only two real values (an imaginary part of zero, exact or not) compare.
func rbComplexCmp(a, b *Complex) *Integer {
	if !rbNumZero(a.im) || !rbNumZero(b.im) {
		return nil
	}
	return rbNumCmp(a.re, b.re)
}

const rbCUNum = `\d+(?:_\d+)*(?:\.\d+(?:_\d+)*)?(?:[eE][-+]?\d+(?:_\d+)*)?(?:/\d+(?:_\d+)*)?`

// rbComplexRe is MRI's string form of a Complex, alternatives ordered so leftmost-first finds the longest: polar `m@a`, `re±im i`, `im i`, `re`.
var rbComplexRe = regexp.MustCompile(`^(?:([-+]?` + rbCUNum + `)@([-+]?` + rbCUNum + `)|([-+]?` + rbCUNum + `)([-+](?:` + rbCUNum + `)?)[iIjJ]|([-+]?(?:` + rbCUNum + `)?)[iIjJ]|([-+]?` + rbCUNum + `))`)

// rbCNumVal is one part: Rational with a `/`, Float with `.` or an exponent, else Integer; a bare sign is ±1.
func rbCNumVal(s string) any {
	s = strings.ReplaceAll(s, "_", "")
	switch s {
	case "", "+":
		return Integer(1)
	case "-":
		return Integer(-1)
	}
	if strings.Contains(s, "/") {
		r := &Rational{}
		if _, ok := r.v.SetString(s); !ok {
			panic(NewZeroDivisionError(Ref(String("divided by 0"))))
		}
		return r
	}
	if strings.ContainsAny(s, ".eE") {
		f, _ := strconv.ParseFloat(s, 64)
		return Float(f)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, _ := new(big.Int).SetString(s, 10)
		return rbBigToInt(f)
	}
	return Integer(i)
}

// rbStrToC parses a Complex from the front of s; strict (Kernel#Complex) wants the whole string, whitespace aside, and raises otherwise, where String#to_c answers what it found (0 for nothing).
func rbStrToC(s string, strict bool) *Complex {
	if strict && strings.IndexByte(s, 0) >= 0 {
		panic(NewArgumentError(Ref(String("string contains null byte"))))
	}
	t := strings.TrimLeft(s, " \t\n\v\f\r")
	m := rbComplexRe.FindStringSubmatch(t)
	if strict && (m == nil || strings.TrimRight(t[len(m[0]):], " \t\n\v\f\r\x00") != "") {
		panic(NewArgumentError(Ref(String("invalid value for convert(): " + strconv.Quote(s)))))
	}
	switch {
	case m == nil || m[0] == "":
		return &Complex{Integer(0), Integer(0)}
	case m[1] != "":
		return rbComplexPolar(rbCNumVal(m[1]), rbCNumVal(m[2]))
	case m[3] != "":
		return &Complex{rbCNumVal(m[3]), rbCNumVal(m[4])}
	case m[6] != "":
		return &Complex{rbCNumVal(m[6]), Integer(0)}
	}
	return &Complex{Integer(0), rbCNumVal(m[5])}
}

// rbComplexMake is Kernel#Complex: a String parses, and Complex parts combine as re + im*i (MRI's nucomp_s_convert).
func rbComplexMake(re, im any) *Complex {
	re, im = rbUnbox(re), rbUnbox(im)
	if s, ok := re.(String); ok {
		re = rbStrToC(string(s), true)
	}
	if s, ok := im.(String); ok {
		im = rbStrToC(string(s), true)
	}
	if c, ok := re.(*Complex); ok {
		if i, ok := im.(Integer); ok && i == 0 {
			return c
		}
	}
	if c, ok := im.(*Complex); ok {
		r := re
		var ri any = Integer(0)
		if rc, ok := re.(*Complex); ok {
			r, ri = rc.re, rc.im
		}
		return &Complex{rbNumOp('-', r, c.im), rbNumOp('+', ri, c.re)}
	}
	if c, ok := re.(*Complex); ok {
		return &Complex{c.re, rbNumOp('+', c.im, rbNumArg(im))}
	}
	return &Complex{rbNumArg(re), rbNumArg(im)}
}

// rbComplexRect is Complex.rectangular: a Complex part is allowed only when its imaginary part is zero.
func rbComplexRect(re, im any) *Complex {
	part := func(a any) any {
		a = rbUnbox(a)
		if c, ok := a.(*Complex); ok {
			if !rbNumZero(c.im) {
				panic(NewTypeError(Ref(String("not a real"))))
			}
			return c.re
		}
		return rbNumArg(a)
	}
	return &Complex{part(re), part(im)}
}
