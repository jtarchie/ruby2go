//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
//
// BigDecimal (decision 112): sign, a non-negative mantissa and a decimal
// exponent (value = mant × 10^exp, trailing zeros stripped), or Infinity
// / NaN. The rules are bigdecimal 4.1's (its C source): a quotient gets
// max(precision(a), precision(b)) + 16 significant digits (at least 32)
// rounded half-up, add/sub/mult are exact unless given digits, Floats
// convert by their shortest representation capped at 16 digits.
package prelude

const (
	rbBDFinite = 0
	rbBDInf    = 1
	rbBDNaN    = 2

	rbBDDoubleFig = 16

	rbBDRoundUp       = 1
	rbBDRoundDown     = 2
	rbBDRoundHalfUp   = 3
	rbBDRoundHalfDown = 4
	rbBDRoundCeiling  = 5
	rbBDRoundFloor    = 6
	rbBDRoundHalfEven = 7
)

var (
	rbBDTen = big.NewInt(10)
	rbBDOne = big.NewInt(1)
)

// rbBDNew builds a normalized finite value from a signed mantissa.
func rbBDNew(mant *big.Int, exp int) *BigDecimal {
	neg := mant.Sign() < 0
	m := new(big.Int).Abs(mant)
	return rbBDNorm(neg, m, exp)
}

func rbBDNorm(neg bool, mant *big.Int, exp int) *BigDecimal {
	if mant.Sign() == 0 {
		return &BigDecimal{mant: new(big.Int), neg: neg}
	}
	m := new(big.Int).Set(mant)
	r := new(big.Int)
	for {
		q, rem := new(big.Int).QuoRem(m, rbBDTen, r)
		if rem.Sign() != 0 {
			break
		}
		m, exp = q, exp+1
	}
	return &BigDecimal{mant: m, exp: exp, neg: neg}
}

func rbBDSpecial(kind int, neg bool) *BigDecimal {
	return &BigDecimal{mant: new(big.Int), kind: kind, neg: neg}
}

func rbBDZero(neg bool) *BigDecimal { return &BigDecimal{mant: new(big.Int), neg: neg} }

func (x *BigDecimal) isZero() bool { return x.kind == rbBDFinite && x.mant.Sign() == 0 }

// digits is the mantissa's digit count (significant digits).
func (x *BigDecimal) digits() int {
	if x.mant.Sign() == 0 {
		return 0
	}
	return len(x.mant.String())
}

// exponent10 is MRI's #exponent: the power of ten of the 0.ddd form.
func (x *BigDecimal) exponent10() int {
	if x.isZero() || x.kind != rbBDFinite {
		return 0
	}
	return x.digits() + x.exp
}

// precision and scale as VpCountPrecisionAndScale counts them.
func (x *BigDecimal) precision() int {
	if x.isZero() || x.kind != rbBDFinite {
		return 0
	}
	e, n := x.exponent10(), x.digits()
	if e > 0 {
		return max(e, n)
	}
	return n - e
}

func (x *BigDecimal) scale() int {
	if x.isZero() || x.kind != rbBDFinite {
		return 0
	}
	return max(0, x.digits()-x.exponent10())
}

// signed is the mantissa with its sign.
func (x *BigDecimal) signed() *big.Int {
	if x.neg {
		return new(big.Int).Neg(x.mant)
	}
	return new(big.Int).Set(x.mant)
}

func rbBDPow10(n int) *big.Int { return new(big.Int).Exp(rbBDTen, big.NewInt(int64(n)), nil) }

// align scales both to the smaller exponent: a×10^e, b×10^e.
func rbBDAlign(a, b *BigDecimal) (*big.Int, *big.Int, int) {
	e := min(a.exp, b.exp)
	am := new(big.Int).Mul(a.signed(), rbBDPow10(a.exp-e))
	bm := new(big.Int).Mul(b.signed(), rbBDPow10(b.exp-e))
	return am, bm, e
}

// rbBDAddSub is a ± b, exact; prec > 0 rounds the result to that many significant digits.
func rbBDAddSub(a, b *BigDecimal, sub bool, prec int) *BigDecimal {
	if a.kind == rbBDNaN || b.kind == rbBDNaN {
		return rbBDSpecial(rbBDNaN, false)
	}
	bneg := b.neg != sub
	switch {
	case a.kind == rbBDInf && b.kind == rbBDInf:
		if a.neg == bneg {
			return a
		}
		return rbBDSpecial(rbBDNaN, false)
	case a.kind == rbBDInf:
		return a
	case b.kind == rbBDInf:
		return rbBDSpecial(rbBDInf, bneg)
	}
	am, bm, e := rbBDAlign(a, b)
	if sub {
		bm.Neg(bm)
	}
	sum := am.Add(am, bm)
	r := rbBDNew(sum, e)
	if sum.Sign() == 0 {
		r.neg = a.neg && bneg // -0 + -0 only
	}
	if prec > 0 {
		r = rbBDLeftRound(r, prec, rbBDRoundHalfUp, false)
	}
	return r
}

// rbBDMul is a × b, exact; prec > 0 rounds to that many significant digits.
func rbBDMul(a, b *BigDecimal, prec int) *BigDecimal {
	if a.kind == rbBDNaN || b.kind == rbBDNaN {
		return rbBDSpecial(rbBDNaN, false)
	}
	neg := a.neg != b.neg
	if a.kind == rbBDInf || b.kind == rbBDInf {
		if a.isZero() || b.isZero() {
			return rbBDSpecial(rbBDNaN, false)
		}
		return rbBDSpecial(rbBDInf, neg)
	}
	r := rbBDNorm(neg, new(big.Int).Mul(a.mant, b.mant), a.exp+b.exp)
	if prec > 0 {
		r = rbBDLeftRound(r, prec, rbBDRoundHalfUp, false)
	}
	return r
}

// rbBDDiv is a / b to ix significant digits (0: the operator's rule), rounded half-up.
func rbBDDiv(a, b *BigDecimal, ix int) *BigDecimal {
	if a.kind == rbBDNaN || b.kind == rbBDNaN || a.kind == rbBDInf && b.kind == rbBDInf {
		return rbBDSpecial(rbBDNaN, false)
	}
	neg := a.neg != b.neg
	switch {
	case a.kind == rbBDInf:
		return rbBDSpecial(rbBDInf, neg)
	case b.kind == rbBDInf:
		return rbBDZero(neg)
	case b.isZero():
		if a.isZero() {
			return rbBDSpecial(rbBDNaN, false)
		}
		return rbBDSpecial(rbBDInf, neg)
	case a.isZero():
		return rbBDZero(neg)
	}
	if ix == 0 {
		ix = max(a.precision(), b.precision()) + rbBDDoubleFig
		ix = max(ix, 2*rbBDDoubleFig)
	}
	// enough quotient digits for ix+1, then round at ix with the remainder as sticky
	s := ix + 2 + b.digits() - a.digits()
	if s < 0 {
		s = 0
	}
	num := new(big.Int).Mul(a.mant, rbBDPow10(s))
	q, rem := new(big.Int).QuoRem(num, b.mant, new(big.Int))
	r := rbBDNorm(neg, q, a.exp-b.exp-s)
	return rbBDLeftRound(r, ix, rbBDRoundHalfUp, rem.Sign() != 0)
}

// rbBDLeftRound rounds to ix significant digits (VpLeftRound); sticky says digits beyond the mantissa were nonzero.
func rbBDLeftRound(x *BigDecimal, ix int, mode int, sticky bool) *BigDecimal {
	if x.kind != rbBDFinite || x.isZero() {
		return x
	}
	return rbBDRoundAt(x, ix-x.exponent10(), mode, sticky)
}

// rbBDRoundAt rounds to nf digits after the decimal point (negative: before it), as VpMidRound.
func rbBDRoundAt(x *BigDecimal, nf int, mode int, sticky bool) *BigDecimal {
	if x.kind != rbBDFinite || x.isZero() {
		return x
	}
	drop := -(x.exp + nf) // mantissa digits to drop
	if drop <= 0 {
		return x
	}
	if drop > x.digits() { // rounding position left of every digit
		if mode == rbBDRoundCeiling && !x.neg || mode == rbBDRoundFloor && x.neg || mode == rbBDRoundUp {
			return rbBDNorm(x.neg, rbBDOne, -nf)
		}
		return rbBDZero(x.neg)
	}
	p := rbBDPow10(drop)
	q, rem := new(big.Int).QuoRem(x.mant, p, new(big.Int))
	half := new(big.Int).Div(p, big.NewInt(2))
	cmpHalf := rem.Cmp(half) // rem vs 5000...
	fracf := rem.Sign() != 0 || sticky
	further := cmpHalf != 0 || sticky // nonzero past the first dropped digit, when that digit is 5
	if cmpHalf > 0 {
		further = true
	}
	up := false
	switch mode {
	case rbBDRoundUp:
		up = fracf
	case rbBDRoundHalfUp:
		up = cmpHalf >= 0
	case rbBDRoundHalfDown:
		up = cmpHalf > 0 || cmpHalf == 0 && sticky
	case rbBDRoundHalfEven:
		up = cmpHalf > 0 || cmpHalf == 0 && (sticky || q.Bit(0) == 1)
	case rbBDRoundCeiling:
		up = fracf && !x.neg
	case rbBDRoundFloor:
		up = fracf && x.neg
	}
	_ = further
	if up {
		q.Add(q, rbBDOne)
	}
	return rbBDNorm(x.neg, q, -nf)
}

// rbBDDivmod is DoDivmod: floor (or truncate) quotient as an integer BigDecimal, and the remainder.
func rbBDDivmod(a, b *BigDecimal, truncate bool) (*BigDecimal, *BigDecimal) {
	if a.kind == rbBDNaN || b.kind == rbBDNaN || a.kind == rbBDInf && b.kind == rbBDInf {
		return rbBDSpecial(rbBDNaN, false), rbBDSpecial(rbBDNaN, false)
	}
	if b.isZero() {
		panic(NewZeroDivisionError(Ref(String("divided by 0"))))
	}
	if a.kind == rbBDInf {
		return rbBDSpecial(rbBDInf, a.neg != b.neg), rbBDSpecial(rbBDNaN, false)
	}
	if a.isZero() {
		return rbBDZero(false), a
	}
	if b.kind == rbBDInf {
		if !truncate && a.neg != b.neg {
			return rbBDNorm(true, rbBDOne, 0), b
		}
		return rbBDZero(false), a
	}
	am, bm, e := rbBDAlign(a, b)
	q, rem := new(big.Int).QuoRem(am, bm, new(big.Int)) // truncated
	if !truncate && rem.Sign() != 0 && (am.Sign() < 0) != (bm.Sign() < 0) {
		q.Sub(q, rbBDOne)
		rem.Add(rem, bm)
	}
	return rbBDNew(q, 0), rbBDNew(rem, e)
}

func rbBDCmp(a, b *BigDecimal) (int, bool) {
	if a.kind == rbBDNaN || b.kind == rbBDNaN {
		return 0, false
	}
	av, bv := 0, 0
	if a.kind == rbBDInf {
		av = 1
		if a.neg {
			av = -1
		}
	}
	if b.kind == rbBDInf {
		bv = 1
		if b.neg {
			bv = -1
		}
	}
	if av != 0 || bv != 0 {
		return cmp.Compare(av, bv), true
	}
	am, bm, _ := rbBDAlign(a, b)
	return am.Cmp(bm), true
}

// rbBDSign is #sign: 1/-1 zero, 2/-2 finite, 3/-3 infinite, 0 NaN.
func rbBDSign(x *BigDecimal) int {
	var s int
	switch {
	case x.kind == rbBDNaN:
		return 0
	case x.kind == rbBDInf:
		s = 3
	case x.isZero():
		s = 1
	default:
		s = 2
	}
	if x.neg {
		return -s
	}
	return s
}

// rbBDParse is VpAlloc: strict rejects anything but a decimal number
// with optional sign, `_` between digits, an exponent and surrounding
// spaces; loose takes the longest valid prefix (String#to_d).
func rbBDParse(s string, strict bool) (*BigDecimal, bool) {
	t := strings.TrimLeft(s, " \t\n\v\f\r")
	switch strings.TrimRight(t, " \t\n\v\f\r") {
	case "Infinity", "+Infinity":
		return rbBDSpecial(rbBDInf, false), true
	case "-Infinity":
		return rbBDSpecial(rbBDInf, true), true
	case "NaN":
		return rbBDSpecial(rbBDNaN, false), true
	}
	i := 0
	neg := false
	if i < len(t) && (t[i] == '+' || t[i] == '-') {
		neg = t[i] == '-'
		i++
	}
	var intPart, fracPart, expPart strings.Builder
	digitsAllowingUnderscore := func(b *strings.Builder, next func(byte) bool) bool {
		for i < len(t) {
			c := t[i]
			switch {
			case c >= '0' && c <= '9':
				b.WriteByte(c)
				i++
			case c == '_' && b.Len() > 0 && i+1 < len(t) && t[i+1] >= '0' && t[i+1] <= '9':
				i++
			case c == '_' && b.Len() > 0 && !strict:
				return false
			case c == '_':
				return !strict && b.Len() > 0 // loose: stop here
			default:
				return next(c)
			}
		}
		return true
	}
	ok := digitsAllowingUnderscore(&intPart, func(c byte) bool { return true })
	if !ok && strict {
		return nil, false
	}
	if i < len(t) && t[i] == '.' {
		i++
		digitsAllowingUnderscore(&fracPart, func(c byte) bool { return true })
	}
	expSeen, expNeg := false, false
	if i < len(t) && (t[i] == 'e' || t[i] == 'E' || t[i] == 'd' || t[i] == 'D') {
		expSeen = true
		i++
		if i < len(t) && (t[i] == '+' || t[i] == '-') {
			expNeg = t[i] == '-'
			i++
		}
		digitsAllowingUnderscore(&expPart, func(c byte) bool { return true })
	}
	rest := strings.TrimLeft(t[i:], " \t\n\v\f\r")
	if strict && rest != "" {
		return nil, false
	}
	if intPart.Len() == 0 && fracPart.Len() == 0 || expSeen && expPart.Len() == 0 {
		if strict {
			return nil, false
		}
		if intPart.Len() == 0 && fracPart.Len() == 0 {
			return rbBDZero(false), true
		}
		expSeen = false
	}
	mant, _ := new(big.Int).SetString(strings.TrimLeft(intPart.String()+fracPart.String(), "0")+"", 10)
	if mant == nil {
		mant = new(big.Int)
	}
	exp := -fracPart.Len()
	if expSeen {
		e, err := strconv.Atoi(expPart.String())
		if err != nil {
			return nil, false
		}
		if expNeg {
			e = -e
		}
		exp += e
	}
	return rbBDNorm(neg, mant, exp), true
}

// rbBDFromFloat is rb_float_convert_to_BigDecimal: digs 0 takes the
// shortest representation, capped at 16 digits; digs > 0 rounds to digs.
func rbBDFromFloat(f float64, digs int) *BigDecimal {
	switch {
	case math.IsNaN(f):
		return rbBDSpecial(rbBDNaN, false)
	case math.IsInf(f, 0):
		return rbBDSpecial(rbBDInf, f < 0)
	case f == 0:
		return rbBDZero(math.Signbit(f))
	case digs > rbBDDoubleFig:
		panic(NewArgumentError(Ref(String("precision too large."))))
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	if digs > 0 {
		s = strconv.FormatFloat(f, 'e', digs-1, 64)
	}
	mantS, expS, _ := strings.Cut(s, "e")
	neg := strings.HasPrefix(mantS, "-")
	mantS = strings.TrimPrefix(mantS, "-")
	intS, fracS, _ := strings.Cut(mantS, ".")
	ds := intS + fracS
	e, _ := strconv.Atoi(expS)
	if len(ds) > rbBDDoubleFig {
		ds = ds[:rbBDDoubleFig]
	}
	mant, _ := new(big.Int).SetString(ds, 10)
	return rbBDNorm(neg, mant, e-(len(ds)-1))
}

func rbBDFromInt(n Integer) *BigDecimal { return rbBDNew(big.NewInt(int64(n)), 0) }

// rbBDFromRational divides to prec significant digits (0: the operator's rule).
func rbBDFromRational(r *Rational, prec int) *BigDecimal {
	num := rbBDNew(new(big.Int).Set(r.v.Num()), 0)
	den := rbBDNew(new(big.Int).Set(r.v.Denom()), 0)
	return rbBDDiv(num, den, prec)
}

// rbBDArg converts an operand: BigDecimal, Integer, Float (shortest), Rational (prec digits).
func rbBDArg(v any, prec int) *BigDecimal {
	switch x := rbUnbox(v).(type) {
	case *BigDecimal:
		return x
	case Integer:
		return rbBDFromInt(x)
	case Float:
		return rbBDFromFloat(float64(x), 0)
	case *Rational:
		return rbBDFromRational(x, prec)
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into BigDecimal"))))
	}
	panic(NewTypeError(Ref(String(rbClassName(v) + " can't be coerced into BigDecimal"))))
}

// rbBDToS is #to_s: "E" form (0.ddde±n), or "F" (plain); a leading ' '
// or '+' sets the positive sign; a digit count groups the digits with
// spaces, in the mantissa (E) or on both sides of the point (F).
func rbBDToS(x *BigDecimal, format string) string {
	plus := ""
	if strings.HasPrefix(format, " ") || strings.HasPrefix(format, "+") {
		plus = format[:1]
		format = format[1:]
	}
	fixed := false
	groups := 0
	for _, c := range format {
		switch {
		case c == ' ':
		case c >= '0' && c <= '9':
			groups = groups*10 + int(c-'0')
		case c == 'F' || c == 'f':
			fixed = true
		}
	}
	sign := plus
	if x.neg {
		sign = "-"
	}
	switch x.kind {
	case rbBDNaN:
		return "NaN"
	case rbBDInf:
		return sign + "Infinity"
	}
	ds := x.mant.String()
	if x.isZero() {
		return sign + "0.0"
	}
	e := x.exponent10()
	if !fixed {
		if groups > 0 {
			ds = rbBDGroup(ds, groups, false)
		}
		return sign + "0." + ds + "e" + strconv.Itoa(e)
	}
	var intS, fracS string
	switch {
	case e <= 0:
		intS, fracS = "0", strings.Repeat("0", -e)+ds
	case e >= len(ds):
		intS, fracS = ds+strings.Repeat("0", e-len(ds)), "0"
	default:
		intS, fracS = ds[:e], ds[e:]
	}
	if groups > 0 {
		intS, fracS = rbBDGroup(intS, groups, true), rbBDGroup(fracS, groups, false)
	}
	return sign + intS + "." + fracS
}

// rbBDGroup inserts a space every n digits, counted from the right (an integer part) or the left.
func rbBDGroup(ds string, n int, fromRight bool) string {
	if n <= 0 || len(ds) <= n {
		return ds
	}
	var b strings.Builder
	if fromRight {
		head := len(ds) % n
		if head > 0 {
			b.WriteString(ds[:head])
		}
		for i := head; i < len(ds); i += n {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(ds[i : i+n])
		}
		return b.String()
	}
	for i := 0; i < len(ds); i += n {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(ds[i:min(i+n, len(ds))])
	}
	return b.String()
}

func rbBDToF(x *BigDecimal) Float {
	switch x.kind {
	case rbBDNaN:
		return Float(math.NaN())
	case rbBDInf:
		if x.neg {
			return Float(math.Inf(-1))
		}
		return Float(math.Inf(1))
	}
	f, _ := strconv.ParseFloat(rbBDToS(x, ""), 64)
	return Float(f)
}

// rbBDToI is the integer part, as Integer: past 64 bits there is no Bignum (decision 35).
func rbBDToI(x *BigDecimal) Integer {
	rbBDCheckNum(x)
	q := new(big.Int).Set(x.mant)
	if x.exp < 0 {
		q.Quo(q, rbBDPow10(-x.exp))
	} else {
		q.Mul(q, rbBDPow10(x.exp))
	}
	if x.neg {
		q.Neg(q)
	}
	if !q.IsInt64() {
		panic(NewRangeError(Ref(String(rbBDToS(x, "") + " overflows Integer (64-bit; no Bignum)"))))
	}
	return Integer(q.Int64())
}

// rbBDCheckNum is BigDecimal_check_num: an integer or rational view of NaN or Infinity is MRI's FloatDomainError.
func rbBDCheckNum(x *BigDecimal) {
	switch {
	case x.kind == rbBDNaN:
		panic(NewFloatDomainError(Ref(String("Computation results in 'NaN' (Not a Number)"))))
	case x.kind == rbBDInf && x.neg:
		panic(NewFloatDomainError(Ref(String("Computation results in '-Infinity'"))))
	case x.kind == rbBDInf:
		panic(NewFloatDomainError(Ref(String("Computation results in 'Infinity'"))))
	}
}

func rbBDToR(x *BigDecimal) *Rational {
	rbBDCheckNum(x)
	out := &Rational{}
	if x.exp >= 0 {
		out.v.SetInt(new(big.Int).Mul(x.signed(), rbBDPow10(x.exp)))
	} else {
		out.v.SetFrac(x.signed(), rbBDPow10(-x.exp))
	}
	return out
}

// rbBDSqrt is #sqrt(n): n significant digits (0: the value's digits + 16), rounded half-up.
func rbBDSqrt(x *BigDecimal, prec int) *BigDecimal {
	if x.kind == rbBDInf && !x.neg {
		return x
	}
	if x.neg && !x.isZero() {
		panic(NewFloatDomainError(Ref(String("sqrt of negative value"))))
	}
	if x.kind == rbBDNaN {
		panic(NewFloatDomainError(Ref(String("sqrt of 'NaN'(Not a Number)"))))
	}
	if x.isZero() {
		return x
	}
	if prec == 0 {
		prec = x.digits() + rbBDDoubleFig
	}
	return rbBDFromFloatFn(x, prec, func(z *big.Float) *big.Float { return z.Sqrt(z) })
}

// rbBDFromFloatFn applies fn at a binary precision past prec decimal digits and rounds the result to prec digits.
func rbBDFromFloatFn(x *BigDecimal, prec int, fn func(*big.Float) *big.Float) *BigDecimal {
	bits := uint(float64(prec+10)*3.33) + 64
	z := new(big.Float).SetPrec(bits).SetInt(x.signed())
	scale := new(big.Float).SetPrec(bits).SetInt(rbBDPow10(rbBDAbsInt(x.exp)))
	if x.exp < 0 {
		z.Quo(z, scale)
	} else {
		z.Mul(z, scale)
	}
	z = fn(z)
	return rbBDFromBigFloat(z, prec)
}

func rbBDAbsInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// rbBDFromBigFloat rounds a big.Float to prec significant decimal digits, half-up.
func rbBDFromBigFloat(z *big.Float, prec int) *BigDecimal {
	s := z.Text('e', prec+2) // prec+3 digits, then round the last two away
	mantS, expS, _ := strings.Cut(s, "e")
	neg := strings.HasPrefix(mantS, "-")
	mantS = strings.TrimPrefix(mantS, "-")
	intS, fracS, _ := strings.Cut(mantS, ".")
	ds := intS + fracS
	e, _ := strconv.Atoi(expS)
	mant, _ := new(big.Int).SetString(ds, 10)
	r := rbBDNorm(neg, mant, e-(len(ds)-1))
	return rbBDLeftRound(r, prec, rbBDRoundHalfUp, false)
}

// rbBDPow is #** / #power: an Integer exponent is exact (a negative one
// is 1 / x^n by the division rule); any other exponent goes through
// exp(y·ln x) at max(digits(x), digits(y), 16) + 16 significant digits.
func rbBDPow(x *BigDecimal, y any, prec int) *BigDecimal {
	yd := rbBDArg(y, 0)
	if x.kind == rbBDNaN || yd.kind == rbBDNaN {
		return rbBDSpecial(rbBDNaN, false)
	}
	if yd.isZero() {
		return rbBDNew(rbBDOne, 0)
	}
	if yd.kind == rbBDFinite && yd.exp >= 0 && prec == 0 { // an integer exponent
		n := new(big.Int).Mul(yd.mant, rbBDPow10(yd.exp))
		if !n.IsInt64() || n.Int64() > 1<<31 {
			panic(NewArgumentError(Ref(String("exponent too large"))))
		}
		k := int(n.Int64())
		if x.kind == rbBDInf {
			if yd.neg {
				return rbBDZero(false)
			}
			return rbBDSpecial(rbBDInf, x.neg && k%2 == 1)
		}
		if x.isZero() {
			if yd.neg {
				return rbBDSpecial(rbBDInf, x.neg && k%2 == 1)
			}
			return rbBDZero(false)
		}
		m := new(big.Int).Exp(x.mant, big.NewInt(int64(k)), nil)
		r := rbBDNorm(x.neg && k%2 == 1, m, x.exp*k)
		if yd.neg {
			return rbBDDiv(rbBDNew(rbBDOne, 0), r, 0)
		}
		return r
	}
	if x.neg && !x.isZero() {
		panic(NewMath_DomainError(Ref(String("Computation results in complex number"))))
	}
	if prec == 0 {
		prec = max(x.digits(), yd.digits(), rbBDDoubleFig) + rbBDDoubleFig
	}
	if x.isZero() {
		if yd.neg {
			return rbBDSpecial(rbBDInf, false)
		}
		return rbBDZero(false)
	}
	bits := uint(float64(prec+10)*3.33) + 64
	toFloat := func(v *BigDecimal) *big.Float {
		z := new(big.Float).SetPrec(bits).SetInt(v.signed())
		scale := new(big.Float).SetPrec(bits).SetInt(rbBDPow10(rbBDAbsInt(v.exp)))
		if v.exp < 0 {
			return z.Quo(z, scale)
		}
		return z.Mul(z, scale)
	}
	lx := rbBDLn(toFloat(x), bits)
	ylx := new(big.Float).SetPrec(bits).Mul(toFloat(yd), lx)
	return rbBDFromBigFloat(rbBDExp(ylx, bits), prec)
}

// rbBDLn is ln(z) for z > 0 at the given precision: argument reduction by powers of two, then the atanh series.
func rbBDLn(z *big.Float, bits uint) *big.Float {
	z = new(big.Float).SetPrec(bits).Set(z)
	k := 0
	two := big.NewFloat(2).SetPrec(bits)
	for z.Cmp(big.NewFloat(1.5)) > 0 {
		z.Quo(z, two)
		k++
	}
	for z.Cmp(big.NewFloat(0.75)) < 0 {
		z.Mul(z, two)
		k--
	}
	// ln z = 2 atanh((z-1)/(z+1))
	num := new(big.Float).SetPrec(bits).Sub(z, big.NewFloat(1))
	den := new(big.Float).SetPrec(bits).Add(z, big.NewFloat(1))
	t := new(big.Float).SetPrec(bits).Quo(num, den)
	t2 := new(big.Float).SetPrec(bits).Mul(t, t)
	sum := new(big.Float).SetPrec(bits).Set(t)
	term := new(big.Float).SetPrec(bits).Set(t)
	eps := new(big.Float).SetPrec(bits).SetMantExp(big.NewFloat(1), -int(bits)-4)
	for n := 3; ; n += 2 {
		term.Mul(term, t2)
		add := new(big.Float).SetPrec(bits).Quo(term, big.NewFloat(float64(n)))
		sum.Add(sum, add)
		if new(big.Float).Abs(add).Cmp(eps) < 0 {
			break
		}
	}
	sum.Mul(sum, two)
	ln2 := rbBDLn2(bits)
	return sum.Add(sum, new(big.Float).SetPrec(bits).Mul(ln2, big.NewFloat(float64(k))))
}

// rbBDLn2 is ln 2 by the atanh series at 1/3.
func rbBDLn2(bits uint) *big.Float {
	t := new(big.Float).SetPrec(bits).Quo(big.NewFloat(1), big.NewFloat(3))
	t2 := new(big.Float).SetPrec(bits).Mul(t, t)
	sum := new(big.Float).SetPrec(bits).Set(t)
	term := new(big.Float).SetPrec(bits).Set(t)
	eps := new(big.Float).SetPrec(bits).SetMantExp(big.NewFloat(1), -int(bits)-4)
	for n := 3; ; n += 2 {
		term.Mul(term, t2)
		add := new(big.Float).SetPrec(bits).Quo(term, big.NewFloat(float64(n)))
		sum.Add(sum, add)
		if add.Cmp(eps) < 0 {
			break
		}
	}
	return sum.Mul(sum, big.NewFloat(2))
}

// rbBDExp is e^z at the given precision: reduce by ln 2, Taylor series, square back.
func rbBDExp(z *big.Float, bits uint) *big.Float {
	ln2 := rbBDLn2(bits)
	kf := new(big.Float).SetPrec(bits).Quo(z, ln2)
	kInt, _ := kf.Int(nil)
	k := int(kInt.Int64())
	r := new(big.Float).SetPrec(bits).Sub(z, new(big.Float).SetPrec(bits).Mul(ln2, big.NewFloat(float64(k))))
	// r in [-ln2, ln2]: halve 8 times for fast convergence, then square back
	const halvings = 8
	r.Quo(r, big.NewFloat(1<<halvings))
	sum := big.NewFloat(1).SetPrec(bits)
	term := big.NewFloat(1).SetPrec(bits)
	eps := new(big.Float).SetPrec(bits).SetMantExp(big.NewFloat(1), -int(bits)-4)
	for n := 1; ; n++ {
		term.Mul(term, r)
		term.Quo(term, big.NewFloat(float64(n)))
		sum.Add(sum, term)
		if new(big.Float).Abs(term).Cmp(eps) < 0 {
			break
		}
	}
	for range halvings {
		sum.Mul(sum, sum)
	}
	return sum.SetMantExp(sum, k)
}

// rbBDRoundMode reads a rounding mode: a Symbol name or a ROUND_* Integer.
func rbBDRoundMode(v any) int {
	switch m := rbUnbox(v).(type) {
	case Symbol:
		switch string(m) {
		case "up":
			return rbBDRoundUp
		case "down", "truncate":
			return rbBDRoundDown
		case "half_up", "default":
			return rbBDRoundHalfUp
		case "half_down":
			return rbBDRoundHalfDown
		case "half_even", "banker":
			return rbBDRoundHalfEven
		case "ceiling", "ceil":
			return rbBDRoundCeiling
		case "floor":
			return rbBDRoundFloor
		}
	case Integer:
		if m >= 1 && m <= 7 {
			return int(m)
		}
	}
	panic(NewArgumentError(Ref(String("invalid rounding mode (" + string(rbToS(v)) + ")"))))
}

// rbBDHash hashes the normalized value, so equal values hash alike.
func rbBDHash(x *BigDecimal) Integer {
	return rbHash(String(rbBDToS(x, "")))
}

// rbBDCoercePrec is GetCoercePrec: the digits a Rational operand is divided to.
func rbBDCoercePrec(x *BigDecimal) int { return max((x.digits()+8)/9*9, 2*rbBDDoubleFig) }

// rbBDDigits checks a digits argument: MRI's "negative precision".
func rbBDDigits(n Integer) int {
	if n < 0 {
		panic(NewArgumentError(Ref(String("negative precision"))))
	}
	return int(n)
}

// rbBDRel is ==, <, <=, >, >= against any operand: NaN compares false,
// == with a non-number is false, an order with one raises MRI's ArgumentError.
func rbBDRel(x *BigDecimal, other any, op string) bool {
	var b *BigDecimal
	switch o := rbUnbox(other).(type) {
	case *BigDecimal:
		b = o
	case Integer, Float, *Rational:
		b = rbBDArg(o, rbBDCoercePrec(x))
	default:
		if op == "==" {
			return false
		}
		panic(NewArgumentError(Ref(String("comparison of BigDecimal with " + rbCmpName(other) + " failed"))))
	}
	c, ok := rbBDCmp(x, b)
	if !ok {
		return false
	}
	switch op {
	case "==":
		return c == 0
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	}
	return c >= 0
}

// rbCmpName names a failed comparison's operand as rb_cmperr does.
func rbCmpName(v any) string {
	switch rbUnbox(v).(type) {
	case nil, Boolean, Integer, Float, Symbol:
		return string(rbInspect(v))
	}
	return rbClassName(v)
}

// rbBDConvert is Kernel#BigDecimal.
func rbBDConvert(v any, digits int) *BigDecimal {
	if digits < 0 {
		panic(NewArgumentError(Ref(String("negative precision"))))
	}
	switch x := rbUnbox(v).(type) {
	case String:
		r, ok := rbBDParse(string(x), true)
		if !ok {
			panic(NewArgumentError(Ref(String("invalid value for BigDecimal(): " + string(rbStringInspect(string(x)))))))
		}
		return r
	case Float:
		return rbBDFromFloat(float64(x), digits)
	case *Rational:
		if digits == 0 {
			panic(NewArgumentError(Ref(String("can't omit precision for a Rational."))))
		}
		return rbBDFromRational(x, digits)
	case *BigDecimal:
		return x // digits do not apply to a BigDecimal, as in MRI
	}
	return rbBDArg(v, 0)
}
