//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbNumArg checks that a Complex part is a real number.
func rbNumArg(a any) any {
	a = rbUnbox(a)
	switch a.(type) {
	case Integer, Float, *Rational:
		return a
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(a) + " into Complex"))))
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

func rbNumTo(a any, level int) any {
	switch level {
	case 1:
		if i, ok := a.(Integer); ok {
			return rbRat(int64(i), 1)
		}
	case 2:
		return Float(rbNumFloat(a))
	}
	return a
}

func rbNumFloat(a any) float64 {
	switch v := a.(type) {
	case Integer:
		return float64(v)
	case Float:
		return float64(v)
	case *Rational:
		return float64(v.ToF())
	}
	panic(rbConvError(a, "Float"))
}

func rbNumOp(op byte, a, b any) any {
	level := max(rbNumLevel(a), rbNumLevel(b))
	if op == '/' && level == 0 {
		level = 1 // Integer parts divide exactly, as MRI's quo
	}
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	switch level {
	case 0:
		x, y := a.(Integer), b.(Integer)
		switch op {
		case '+':
			return x.Op_plus(y)
		case '-':
			return x.Op_minus(y)
		}
		return x.Op_mul(y)
	case 1:
		x, y := a.(*Rational), b.(*Rational)
		switch op {
		case '+':
			return x.Op_plus(y)
		case '-':
			return x.Op_minus(y)
		case '*':
			return x.Op_mul(y)
		}
		return x.Op_div(y)
	}
	x, y := a.(Float), b.(Float)
	switch op {
	case '+':
		return x + y
	case '-':
		return x - y
	case '*':
		return x * y
	}
	return x / y
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

func rbNumEq(a, b any) bool {
	level := max(rbNumLevel(a), rbNumLevel(b))
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	if level == 1 {
		return a.(*Rational).v.Cmp(&b.(*Rational).v) == 0
	}
	return a == b
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
	return rbHypot(rbNumFloat(c.re), rbNumFloat(c.im))
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

func rbNumAbs(a any) any {
	switch v := a.(type) {
	case Integer:
		return v.Abs()
	case Float:
		return Float(math.Abs(float64(v)))
	}
	return a.(*Rational).Abs()
}

func rbComplexArg(c *Complex) float64 {
	return rbAtan2(rbNumFloat(c.im), rbNumFloat(c.re))
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
	return &Complex{Float(rf * rbCos(tf)), Float(rf * rbSin(tf))}
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

// rbSummer is MRI's ary_sum/enum_sum: an exact Integer/Rational phase, then
// Kahan-Babuska compensated Float addition once a Float appears.
type rbSummer struct {
	exact    any // Integer or *Rational; nil until the first value
	floating bool
	f, c     float64
}

func (s *rbSummer) add(x any) {
	x = rbUnbox(x)
	if !s.floating {
		if _, isFloat := x.(Float); !isFloat {
			if s.exact == nil {
				s.exact = Integer(0)
			}
			s.exact = rbNumCanon(rbNumOp('+', s.exact, rbNumArg(x)))
			return
		}
		s.floating = true
		if s.exact != nil {
			s.f = rbNumFloat(s.exact)
		}
	}
	s.addFloat(rbNumFloat(rbNumArg(x)))
}

// addFloat is add for a Float, which a typed Float sum calls without boxing.
func (s *rbSummer) addFloat(v float64) {
	switch {
	case math.IsNaN(s.f):
	case math.IsNaN(v):
		s.f = v
	case math.IsInf(v, 0):
		if math.IsInf(s.f, 0) && math.Signbit(v) != math.Signbit(s.f) {
			s.f = math.NaN()
		} else {
			s.f = v
		}
	case math.IsInf(s.f, 0):
	default:
		t := s.f + v
		if math.Abs(s.f) >= math.Abs(v) {
			s.c += (s.f - t) + v
		} else {
			s.c += (v - t) + s.f
		}
		s.f = t
	}
}

func (s *rbSummer) result() any {
	if s.floating {
		return Float(s.f + s.c)
	}
	if s.exact == nil {
		return Integer(0)
	}
	return s.exact
}

// rbComplexCmp is MRI's nucomp_cmp: only two real values (an imaginary part of zero, exact or not) compare.
func rbComplexCmp(a, b *Complex) *Integer {
	if !rbNumZero(a.im) || !rbNumZero(b.im) {
		return nil
	}
	return rbNumCmp(a.re, b.re)
}

// rbNumCmp orders two Complex parts in the tower; NaN compares to nothing.
func rbNumCmp(a, b any) *Integer {
	level := max(rbNumLevel(a), rbNumLevel(b))
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	switch level {
	case 0:
		return Ref(a.(Integer).Op_cmp(b.(Integer)))
	case 1:
		return Ref(Integer(a.(*Rational).v.Cmp(&b.(*Rational).v)))
	}
	switch x, y := a.(Float), b.(Float); {
	case x < y:
		return Ref(Integer(-1))
	case x > y:
		return Ref(Integer(1))
	case x == y:
		return Ref(Integer(0))
	}
	return nil
}
