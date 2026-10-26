//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import "math"

// rbFloatPow is Float#**, correctly rounded as MRI's libm pow() nearly always is (Go's math.Pow is often an ulp or more off); each fast path rounds only when it provably gives rbFloatPowExact's result (decision 389), and math.Pow keeps C99's special values and results past the normal range.
func rbFloatPow(x, y float64) float64 {
	switch {
	case y == 2: // MRI's own shortcut; one rounding already
		return x * x
	case x == 0 || y == 0 || x == 1 || x == -1 || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0),
		x < 0 && y != math.Trunc(y):
		return math.Pow(x, y)
	case y >= -64 && y <= 64 && y == math.Trunc(y):
		if r, ok := rbPowInt(x, int(y)); ok {
			return r
		}
	}
	if r, ok := rbPowFast(x, y); ok {
		return r
	}
	return rbFloatPowExact(x, y)
}

// rbPowRound rounds h+lo when no rounding midpoint lies within err of it; err includes rbFloatPowExact's own error (under 2^-84), so both round alike.
func rbPowRound(h, lo, err float64) (float64, bool) {
	err = (err + 0x1p-75) * math.Abs(h)
	a := h + (lo - err)
	return a, a == h+(lo+err)
}

// rbPowInt is x**n for 1 <= |n| <= 64 by square-and-multiply in double-double: a dozen products where rbPowFast takes a log and an exp.
func rbPowInt(x float64, n int) (float64, bool) {
	m := n
	if m < 0 {
		m = -m
	}
	ph, pl := 1.0, 0.0
	if m&1 == 1 {
		ph = x
	}
	bh, bl := x, 0.0
	for m >>= 1; m > 0; m >>= 1 {
		bh, bl = rbDDMul(bh, bl, bh, bl)
		if m&1 == 1 {
			ph, pl = rbDDMul(ph, pl, bh, bl)
		}
	}
	// Every partial power lies between x and the result, so this range keeps all the products' low words normal.
	if a := math.Abs(ph); !(a > 0x1p-900 && a < 0x1p900) {
		return 0, false
	}
	if n < 0 {
		qh := 1 / ph
		ql := (math.FMA(-qh, ph, 1) - qh*pl) * qh
		ph, pl = qh, ql
	}
	return rbPowRound(ph, pl, 0x1p-90) // 12 products and a quotient, each under 2^-100
}

// rbPowFast is x**y by 128-entry log and exp tables (as musl's pow), rounded by rbPowRound under a bound on its error; about 1 call in 1000 falls back.
func rbPowFast(x, y float64) (float64, bool) {
	// |x| = 2^k * z, z in [0x1.6955p-1, 0x1.6955p0); log z = log c + log1p(r), r = z/c - 1.
	ix := math.Float64bits(x) &^ (1 << 63)
	if ix < 1<<52 { // subnormal
		ix = math.Float64bits(math.Abs(x)*0x1p52) - 52<<52
	}
	tmp := ix - 0x3fe6955500000000
	c := &rbPowLogT[tmp>>45&127]
	k := float64(int64(tmp) >> 52)
	z := math.Float64frombits(ix - tmp&(0xfff<<52))
	r := math.FMA(z, c[0], -1) // exact: c[0] has 8 bits and |r| < 2^-7
	// log1p(r) - r is under 2^-15, so far out of range is known before the series.
	if p0 := y * (k*rbPowLn2h + c[1] + r); math.Abs(p0) > 760+math.Abs(y)*0x1p-13 {
		return rbPowEdge(x, y, p0)
	}
	r2 := float64(r * r)
	r2l := math.FMA(r, r, -r2)
	t := r2 * r * (1.0/3 + r*(-1.0/4+r*(1.0/5+r*(-1.0/6+r*(1.0/7+r*(-1.0/8+r*(1.0/9+r*(-1.0/10+r*(1.0/11)))))))))
	// log|x| = k*ln2 + log c + r - r^2/2 + t, summed as a double-double.
	s1, e1 := rbTwoSum(float64(k*rbPowLn2h), c[1])
	s2, e2 := rbTwoSum(s1, r)
	s3, e3 := rbTwoSum(s2, -r2/2)
	lo := e1 + e2 + e3 + (k*rbPowLn2l + c[2] - r2l/2 + t)
	lh := s3 + lo
	ll := lo - (lh - s3)
	// Absolute error of lh+ll: t's roundings, the low sum's (k*ln2l), the rest relative.
	errL := 0x1p-49*math.Abs(r2*r) + 0x1p-80*math.Abs(k) + 0x1p-90*math.Abs(lh)
	ph := float64(y * lh)
	pl := math.FMA(y, lh, -ph) + y*ll
	if !(ph > -708 && ph < 709) {
		return rbPowEdge(x, y, ph)
	}
	// exp(p) = 2^(n/128) * exp(r1), r1 = p - n*ln2/128, |r1| <= 2^-8.4; n*L1 and the difference are exact.
	n := math.Round(ph * rbPowN2)
	r1, r1l := rbTwoSum(math.FMA(-n, rbPowL1, ph), pl-n*rbPowL2)
	q := r1 * r1 * (1.0/2 + r1*(1.0/6+r1*(1.0/24+r1*(1.0/120+r1*(1.0/720+r1*(1.0/5040))))))
	s := r1l + r1l*r1 + q
	ni := int64(n)
	tt := &rbPowExpT[ni&127]
	m := float64(tt[0] * r1)
	me := math.FMA(tt[0], r1, -m)
	h, e := rbTwoSum(tt[0], m)
	lo = e + (me + (tt[1] + (tt[1]*r1 + tt[0]*s)))
	// exp's own error (truncation, q's and lo's roundings) is under 2^-66.
	a, ok := rbPowRound(h, lo, math.Abs(y)*errL+0x1p-64)
	if !ok {
		return 0, false
	}
	a *= math.Float64frombits(uint64(ni>>7+1023) << 52) // the result is normal: ph is in range
	if x < 0 && math.Mod(y, 2) != 0 {
		a = -a
	}
	return a, true
}

// rbPowEdge is x**y for a ph = y*log|x| (error under 2^-60) near or past the normal range, where rbFloatPowExact calls math.Pow beyond its own cutoffs.
func rbPowEdge(x, y, ph float64) (float64, bool) {
	switch {
	case ph > -708.395 && ph < 709.795: // either side of rbFloatPowExact's cutoffs
		return 0, false
	case ph >= -760 && ph <= 760: // ponytail: math.Pow's subnormal results differ from MRI's in 2 of 5 (decision 389); round them at 2^-1074 by adding 1.0 first (musl's exp specialcase)
		return math.Pow(x, y), true
	}
	r := math.Inf(1) // what math.Pow returns there, without its squaring loop's branch misses
	if ph < 0 {
		r = 0
	}
	if x < 0 && math.Mod(y, 2) != 0 {
		r = -r
	}
	return r, true
}

// rbFloatPowExact evaluates exp(y*log|x|) in double-double (error under 2^-84) and rounds once.
func rbFloatPowExact(x, y float64) float64 {
	const ln2lo = 0x1.abc9e3b39803fp-56 // math.Ln2 - float64(math.Ln2)
	// log|x| = e*ln2 + 2s*sum(t^n/(2n+1)), with s = (f-1)/(f+1), t = s*s < 0.03
	// for f in [sqrt(1/2), sqrt(2)); f-1 is exact.
	f, e := math.Frexp(math.Abs(x))
	if f < math.Sqrt2/2 {
		f, e = 2*f, e-1
	}
	dh, dl := rbTwoSum(f, 1)
	sh := (f - 1) / dh
	sl := (math.FMA(-sh, dh, f-1) - sh*dl) / dh
	th, tl := rbDDMul(sh, sl, sh, sl)
	ah, al := 0.0, 0.0
	for n := 18; n >= 0; n-- {
		c := rbPowInv[n]
		if n > 6 { // terms under 2^-39 need no low word
			ah = ah*th + c[0]
			continue
		}
		ah, al = rbDDMul(ah, al, th, tl)
		ah, al = rbDDAdd(ah, al, c[0], c[1])
	}
	lh, ll := rbDDMul(sh, sl, 2*ah, 2*al)
	fe := float64(e)
	kh := float64(fe * math.Ln2)
	lh, ll = rbDDAdd(kh, math.FMA(fe, math.Ln2, -kh)+fe*ln2lo, lh, ll)
	// x**y = 2^k * exp(r), r = y*log|x| - k*ln2, |r| <= ln2/2
	ph := float64(y * lh)
	pl := math.FMA(y, lh, -ph) + y*ll
	if !(ph > -708.39 && ph < 709.79) { // subnormal or overflowing
		return math.Pow(x, y)
	}
	k := math.Round(ph * math.Log2E)
	kh = float64(k * math.Ln2)
	rh, rl := rbDDAdd(ph, pl, -kh, -math.FMA(k, math.Ln2, -kh)-k*ln2lo)
	// expm1(r/4) by Taylor, then expm1(2a) = expm1(a)*(expm1(a)+2) twice.
	rh, rl = rh/4, rl/4
	eh, el := 0.0, 0.0
	for n := 16; n >= 1; n-- {
		c := rbPowFact[n]
		if n > 7 { // terms under 2^-39 of the first
			eh = (eh + c[0]) * rh
			continue
		}
		eh, el = rbDDAdd(eh, el, c[0], c[1])
		eh, el = rbDDMul(eh, el, rh, rl)
	}
	for range 2 {
		mh, ml := rbDDAdd(eh, el, 2, 0)
		eh, el = rbDDMul(eh, el, mh, ml)
	}
	res, _ := rbDDAdd(1, 0, eh, el)
	res = math.Ldexp(res, int(k))
	if x < 0 && math.Mod(y, 2) != 0 {
		return -res
	}
	return res
}

// rbPowInv[n] is 1/(2n+1) and rbPowFact[n] is 1/n!, as double-doubles.
var rbPowInv, rbPowFact = func() (inv, fact [19][2]float64) {
	nf := 1.0
	for n := range inv {
		d := float64(2*n + 1)
		if n > 0 {
			nf *= float64(n)
		}
		inv[n] = [2]float64{1 / d, math.FMA(-1/d, d, 1) / d}
		fact[n] = [2]float64{1 / nf, math.FMA(-1/nf, nf, 1) / nf}
	}
	return inv, fact
}()

// Double-double arithmetic: a value is hi+lo with |lo| <= ulp(hi)/2. The
// float64() around a product stops Go fusing it into a later add (an FMA),
// which would break the error-free split.
func rbTwoSum(a, b float64) (float64, float64) {
	s := a + b
	bb := s - a
	return s, (a - (s - bb)) + (b - bb)
}

func rbDDAdd(ah, al, bh, bl float64) (float64, float64) {
	s, e := rbTwoSum(ah, bh)
	e += al + bl
	h := s + e
	return h, e - (h - s)
}

func rbDDMul(ah, al, bh, bl float64) (float64, float64) {
	p := float64(ah * bh)
	e := math.FMA(ah, bh, -p) + (ah*bl + al*bh)
	h := p + e
	return h, e - (h - p)
}

// Below is `go -C scripts/pow-tables run .` (decision 389): 1/c has 8 bits so z/c - 1 is exact, and rbPowLn2h (42 bits) and rbPowL1 (35 bits) times any k or n in range are exact.
var rbPowLogT = [128][3]float64{ // 1/c, log(c) as a double-double, per interval of z
	{0x1.6ap+00, -0x1.62c82f2b9c795p-02, -0x1.7b7af915300e5p-57},
	{0x1.68p+00, -0x1.5d1bdbf5809cap-02, -0x1.4236383dc7fe1p-56},
	{0x1.66p+00, -0x1.5767717455a6cp-02, -0x1.526adb283660cp-56},
	{0x1.64p+00, -0x1.51aad872df82dp-02, -0x1.3927ac19f55e3p-59},
	{0x1.62p+00, -0x1.4be5f957778a1p-02, 0x1.259b35b04813dp-57},
	{0x1.6p+00, -0x1.4618bc21c5ec2p-02, -0x1.f42decdeccf1dp-56},
	{0x1.5ep+00, -0x1.404308686a7e4p-02, 0x1.0bcfb6082ce6dp-56},
	{0x1.5cp+00, -0x1.3a64c556945eap-02, 0x1.c68651945f97cp-57},
	{0x1.5ap+00, -0x1.347dd9a987d55p-02, 0x1.4dd4c580919f8p-57},
	{0x1.58p+00, -0x1.2e8e2bae11d31p-02, 0x1.8f4cdb95ebdf9p-56},
	{0x1.56p+00, -0x1.2895a13de86a3p-02, -0x1.7ad24c13f040ep-56},
	{0x1.56p+00, -0x1.2895a13de86a3p-02, -0x1.7ad24c13f040ep-56},
	{0x1.54p+00, -0x1.22941fbcf7966p-02, 0x1.76f5eb09628afp-56},
	{0x1.52p+00, -0x1.1c898c16999fbp-02, 0x1.0e5c62aff1c44p-60},
	{0x1.5p+00, -0x1.1675cababa60ep-02, -0x1.ce63eab883717p-61},
	{0x1.4ep+00, -0x1.1058bf9ae4ad5p-02, -0x1.89fa0ab4cb31dp-58},
	{0x1.4cp+00, -0x1.0a324e27390e3p-02, -0x1.7dcfde8061c03p-56},
	{0x1.4ap+00, -0x1.0402594b4d041p-02, 0x1.28ec217a5022dp-57},
	{0x1.4ap+00, -0x1.0402594b4d041p-02, 0x1.28ec217a5022dp-57},
	{0x1.48p+00, -0x1.fb9186d5e3e2bp-03, 0x1.caaae64f21acbp-57},
	{0x1.46p+00, -0x1.ef0adcbdc5936p-03, -0x1.48637950dc20dp-57},
	{0x1.44p+00, -0x1.e27076e2af2e6p-03, 0x1.61578001e0162p-59},
	{0x1.42p+00, -0x1.d5c216b4fbb91p-03, -0x1.6e443597e4d4p-57},
	{0x1.4p+00, -0x1.c8ff7c79a9a22p-03, 0x1.4f689f8434012p-57},
	{0x1.4p+00, -0x1.c8ff7c79a9a22p-03, 0x1.4f689f8434012p-57},
	{0x1.3ep+00, -0x1.bc286742d8cd6p-03, -0x1.4fce744870f55p-58},
	{0x1.3cp+00, -0x1.af3c94e80bff3p-03, 0x1.398cff3641985p-58},
	{0x1.3ap+00, -0x1.a23bc1fe2b563p-03, -0x1.93711b07a998cp-59},
	{0x1.3ap+00, -0x1.a23bc1fe2b563p-03, -0x1.93711b07a998cp-59},
	{0x1.38p+00, -0x1.9525a9cf456b4p-03, -0x1.d904c1d4e2e26p-57},
	{0x1.36p+00, -0x1.87fa06520c911p-03, 0x1.bf7fdbfa08d9ap-57},
	{0x1.34p+00, -0x1.7ab890210d909p-03, -0x1.be36b2d6a0608p-59},
	{0x1.34p+00, -0x1.7ab890210d909p-03, -0x1.be36b2d6a0608p-59},
	{0x1.32p+00, -0x1.6d60fe719d21dp-03, 0x1.caae268ecd179p-57},
	{0x1.3p+00, -0x1.5ff3070a793d4p-03, 0x1.bc60efafc6f6ep-58},
	{0x1.3p+00, -0x1.5ff3070a793d4p-03, 0x1.bc60efafc6f6ep-58},
	{0x1.2ep+00, -0x1.526e5e3a1b438p-03, 0x1.746ff8a470d3ap-57},
	{0x1.2cp+00, -0x1.44d2b6ccb7d1ep-03, -0x1.9f4f6543e1f88p-57},
	{0x1.2cp+00, -0x1.44d2b6ccb7d1ep-03, -0x1.9f4f6543e1f88p-57},
	{0x1.2ap+00, -0x1.371fc201e8f74p-03, -0x1.de6cb62af18ap-58},
	{0x1.28p+00, -0x1.29552f81ff523p-03, -0x1.301771c407dbfp-57},
	{0x1.26p+00, -0x1.1b72ad52f67ap-03, -0x1.483023472cd74p-58},
	{0x1.26p+00, -0x1.1b72ad52f67ap-03, -0x1.483023472cd74p-58},
	{0x1.24p+00, -0x1.0d77e7cd08e59p-03, -0x1.9a5dc5e9030acp-57},
	{0x1.24p+00, -0x1.0d77e7cd08e59p-03, -0x1.9a5dc5e9030acp-57},
	{0x1.22p+00, -0x1.fec9131dbeabbp-04, 0x1.5746b9981b36cp-58},
	{0x1.2p+00, -0x1.e27076e2af2e6p-04, 0x1.61578001e0162p-60},
	{0x1.2p+00, -0x1.e27076e2af2e6p-04, 0x1.61578001e0162p-60},
	{0x1.1ep+00, -0x1.c5e548f5bc743p-04, -0x1.5d617ef8161b1p-60},
	{0x1.1cp+00, -0x1.a926d3a4ad563p-04, -0x1.942f48aa70ea9p-58},
	{0x1.1cp+00, -0x1.a926d3a4ad563p-04, -0x1.942f48aa70ea9p-58},
	{0x1.1ap+00, -0x1.8c345d6319b21p-04, 0x1.4a697ab3424a9p-61},
	{0x1.1ap+00, -0x1.8c345d6319b21p-04, 0x1.4a697ab3424a9p-61},
	{0x1.18p+00, -0x1.6f0d28ae56b4cp-04, 0x1.906d99184b992p-58},
	{0x1.16p+00, -0x1.51b073f06183fp-04, -0x1.a49e39a1a8be4p-58},
	{0x1.16p+00, -0x1.51b073f06183fp-04, -0x1.a49e39a1a8be4p-58},
	{0x1.14p+00, -0x1.341d7961bd1d1p-04, 0x1.b599f227becbbp-58},
	{0x1.14p+00, -0x1.341d7961bd1d1p-04, 0x1.b599f227becbbp-58},
	{0x1.12p+00, -0x1.16536eea37ae1p-04, 0x1.79da3e8c22cdap-60},
	{0x1.1p+00, -0x1.f0a30c01162a6p-05, -0x1.85f325c5bbacdp-59},
	{0x1.1p+00, -0x1.f0a30c01162a6p-05, -0x1.85f325c5bbacdp-59},
	{0x1.0ep+00, -0x1.b42dd711971bfp-05, 0x1.eb9759c130499p-60},
	{0x1.0ep+00, -0x1.b42dd711971bfp-05, 0x1.eb9759c130499p-60},
	{0x1.0cp+00, -0x1.77458f632dcfcp-05, -0x1.18d3ca87b9296p-59},
	{0x1.0cp+00, -0x1.77458f632dcfcp-05, -0x1.18d3ca87b9296p-59},
	{0x1.0ap+00, -0x1.39e87b9febd6p-05, 0x1.5bfa937f551bbp-59},
	{0x1.0ap+00, -0x1.39e87b9febd6p-05, 0x1.5bfa937f551bbp-59},
	{0x1.08p+00, -0x1.f829b0e7833p-06, -0x1.33e3f04f1ef23p-60},
	{0x1.08p+00, -0x1.f829b0e7833p-06, -0x1.33e3f04f1ef23p-60},
	{0x1.06p+00, -0x1.7b91b07d5b11bp-06, 0x1.5b602ace3a51p-60},
	{0x1.04p+00, -0x1.fc0a8b0fc03e4p-07, 0x1.83092c59642a1p-62},
	{0x1.04p+00, -0x1.fc0a8b0fc03e4p-07, 0x1.83092c59642a1p-62},
	{0x1.02p+00, -0x1.fe02a6b106789p-08, 0x1.e44b7e3711ebfp-67},
	{0x1.02p+00, -0x1.fe02a6b106789p-08, 0x1.e44b7e3711ebfp-67},
	{0x1p+00, 0, 0},
	{0x1p+00, 0, 0},
	{0x1.fcp-01, 0x1.010157588de71p-07, 0x1.46662d417cedp-62},
	{0x1.f8p-01, 0x1.0205658935847p-06, 0x1.27c8e8416e71fp-60},
	{0x1.f4p-01, 0x1.8492528c8cabfp-06, -0x1.d192d0619fa67p-60},
	{0x1.fp-01, 0x1.0415d89e74444p-05, 0x1.c05cf1d753622p-59},
	{0x1.ecp-01, 0x1.466aed42de3eap-05, -0x1.cdd6f7f4a137ep-59},
	{0x1.e8p-01, 0x1.894aa149fb343p-05, 0x1.a8be97660a23dp-60},
	{0x1.e4p-01, 0x1.ccb73cdddb2ccp-05, -0x1.e48fb0500efd4p-59},
	{0x1.e2p-01, 0x1.eea31c006b87cp-05, -0x1.3e4fc93b7b66cp-59},
	{0x1.dep-01, 0x1.1973bd1465567p-04, -0x1.7558367a6acf6p-59},
	{0x1.dap-01, 0x1.3bdf5a7d1ee64p-04, 0x1.7a976d3b5b45fp-59},
	{0x1.d6p-01, 0x1.5e95a4d9791cbp-04, 0x1.f38745c5c450ap-58},
	{0x1.d4p-01, 0x1.700d30aeac0e1p-04, -0x1.72566212cdd05p-61},
	{0x1.dp-01, 0x1.9335e5d594989p-04, -0x1.478a85704ccb7p-58},
	{0x1.ccp-01, 0x1.b6ac88dad5b1cp-04, -0x1.0057eed1ca59fp-59},
	{0x1.cap-01, 0x1.c885801bc4b23p-04, 0x1.a38cb559a6706p-58},
	{0x1.c6p-01, 0x1.ec739830a112p-04, -0x1.a2bf991780d3fp-59},
	{0x1.c4p-01, 0x1.fe89139dbd566p-04, -0x1.ac9f4215f9393p-58},
	{0x1.cp-01, 0x1.1178e8227e47cp-03, -0x1.0e63a5f01c691p-58},
	{0x1.bep-01, 0x1.1aa2b7e23f72ap-03, -0x1.c6ef1d9b2ef7ep-59},
	{0x1.bap-01, 0x1.2d1610c86813ap-03, -0x1.499a3f25af95fp-58},
	{0x1.b8p-01, 0x1.365fcb0159016p-03, 0x1.7d411a5b944adp-58},
	{0x1.b4p-01, 0x1.4913d8333b561p-03, -0x1.0d5604930f135p-58},
	{0x1.b2p-01, 0x1.527e5e4a1b58dp-03, -0x1.71a9682395bfdp-61},
	{0x1.aep-01, 0x1.6574ebe8c133ap-03, -0x1.d34f0f4621bedp-60},
	{0x1.acp-01, 0x1.6f0128b756abcp-03, -0x1.8de59c21e166cp-57},
	{0x1.aap-01, 0x1.7898d85444c73p-03, 0x1.ef8f6ebcfb201p-58},
	{0x1.a6p-01, 0x1.8beafeb38fe8cp-03, 0x1.55aa8b6997a4p-58},
	{0x1.a4p-01, 0x1.95a5adcf7017fp-03, 0x1.142c507fb7a3dp-58},
	{0x1.ap-01, 0x1.a93ed3c8ad9e3p-03, 0x1.bcafa9de97203p-57},
	{0x1.9ep-01, 0x1.b31d8575bce3dp-03, -0x1.6353ab386a94dp-57},
	{0x1.9cp-01, 0x1.bd087383bd8adp-03, 0x1.dd355f6a516d7p-60},
	{0x1.9ap-01, 0x1.c6ffbc6f00f71p-03, -0x1.8e58b2c57a4a5p-57},
	{0x1.96p-01, 0x1.db13db0d4894p-03, 0x1.aa11d49f96cb9p-58},
	{0x1.94p-01, 0x1.e530effe71012p-03, 0x1.2276041f43042p-59},
	{0x1.92p-01, 0x1.ef5ade4dcffe6p-03, -0x1.08ab2ddc708ap-58},
	{0x1.9p-01, 0x1.f991c6cb3b379p-03, 0x1.f665066f980a2p-57},
	{0x1.8cp-01, 0x1.07138604d5862p-02, 0x1.cdb16ed4e9138p-56},
	{0x1.8ap-01, 0x1.0c42d676162e3p-02, 0x1.162c79d5d11eep-58},
	{0x1.88p-01, 0x1.1178e8227e47cp-02, -0x1.0e63a5f01c691p-57},
	{0x1.86p-01, 0x1.16b5ccbacfb73p-02, 0x1.66fbd28b40935p-56},
	{0x1.84p-01, 0x1.1bf99635a6b95p-02, -0x1.12aeb84249223p-57},
	{0x1.82p-01, 0x1.214456d0eb8d4p-02, 0x1.f7ae91aeba60ap-57},
	{0x1.7ep-01, 0x1.2bef07cdc9354p-02, -0x1.82dad7fd86088p-56},
	{0x1.7cp-01, 0x1.314f1e1d35ce4p-02, -0x1.3d69909e5c3dcp-56},
	{0x1.7ap-01, 0x1.36b6776be1117p-02, -0x1.324f0e883858ep-58},
	{0x1.78p-01, 0x1.3c25277333184p-02, -0x1.2ad27e50a8ec6p-56},
	{0x1.76p-01, 0x1.419b423d5e8c7p-02, 0x1.0dbb243827392p-57},
	{0x1.74p-01, 0x1.4718dc271c41bp-02, 0x1.8fb4c14c56eefp-60},
	{0x1.72p-01, 0x1.4c9e09e172c3cp-02, -0x1.123615b147a5dp-58},
	{0x1.7p-01, 0x1.522ae0738a3d8p-02, -0x1.8f7e9b38a6979p-57},
	{0x1.6ep-01, 0x1.57bf753c8d1fbp-02, -0x1.0908d15f88b63p-57},
	{0x1.6cp-01, 0x1.5d5bddf595f3p-02, -0x1.6541148cbb8a2p-56},
}

var rbPowExpT = [128][2]float64{ // 2^(j/128) as a double-double
	{0x1p+00, 0},
	{0x1.0163da9fb3335p+00, 0x1.b61299ab8cdb7p-54},
	{0x1.02c9a3e778061p+00, -0x1.19083535b085dp-56},
	{0x1.04315e86e7f85p+00, -0x1.0a31c1977c96ep-54},
	{0x1.059b0d3158574p+00, 0x1.d73e2a475b465p-55},
	{0x1.0706b29ddf6dep+00, -0x1.c91dfe2b13c27p-55},
	{0x1.0874518759bc8p+00, 0x1.186be4bb284ffp-57},
	{0x1.09e3ecac6f383p+00, 0x1.1487818316136p-54},
	{0x1.0b5586cf9890fp+00, 0x1.8a62e4adc610bp-54},
	{0x1.0cc922b7247f7p+00, 0x1.01edc16e24f71p-54},
	{0x1.0e3ec32d3d1a2p+00, 0x1.03a1727c57b53p-59},
	{0x1.0fb66affed31bp+00, -0x1.b9bedc44ebd7bp-57},
	{0x1.11301d0125b51p+00, -0x1.6c51039449b3ap-54},
	{0x1.12abdc06c31ccp+00, -0x1.1b514b36ca5c7p-58},
	{0x1.1429aaea92dep+00, -0x1.32fbf9af1369ep-54},
	{0x1.15a98c8a58e51p+00, 0x1.2406ab9eeab0ap-55},
	{0x1.172b83c7d517bp+00, -0x1.19041b9d78a76p-55},
	{0x1.18af9388c8deap+00, -0x1.11023d1970f6cp-54},
	{0x1.1a35beb6fcb75p+00, 0x1.e5b4c7b4968e4p-55},
	{0x1.1bbe084045cd4p+00, -0x1.95386352ef607p-54},
	{0x1.1d4873168b9aap+00, 0x1.e016e00a2643cp-54},
	{0x1.1ed5022fcd91dp+00, -0x1.1df98027bb78cp-54},
	{0x1.2063b88628cd6p+00, 0x1.dc775814a8495p-55},
	{0x1.21f49917ddc96p+00, 0x1.2a97e9494a5eep-55},
	{0x1.2387a6e756238p+00, 0x1.9b07eb6c70573p-54},
	{0x1.251ce4fb2a63fp+00, 0x1.ac155bef4f4a4p-55},
	{0x1.26b4565e27cddp+00, 0x1.2bd339940e9d9p-55},
	{0x1.284dfe1f56381p+00, -0x1.a4c3a8c3f0d7ep-54},
	{0x1.29e9df51fdee1p+00, 0x1.612e8afad1255p-55},
	{0x1.2b87fd0dad99p+00, -0x1.10adcd6381aa4p-59},
	{0x1.2d285a6e4030bp+00, 0x1.0024754db41d5p-54},
	{0x1.2ecafa93e2f56p+00, 0x1.1ca0f45d52383p-56},
	{0x1.306fe0a31b715p+00, 0x1.6f46ad23182e4p-55},
	{0x1.32170fc4cd831p+00, 0x1.a9ce78e18047cp-55},
	{0x1.33c08b26416ffp+00, 0x1.32721843659a6p-54},
	{0x1.356c55f929ff1p+00, -0x1.b5cee5c4e4628p-55},
	{0x1.371a7373aa9cbp+00, -0x1.63aeabf42eae2p-54},
	{0x1.38cae6d05d866p+00, -0x1.e958d3c9904bdp-54},
	{0x1.3a7db34e59ff7p+00, -0x1.5e436d661f5e3p-56},
	{0x1.3c32dc313a8e5p+00, -0x1.efff8375d29c3p-54},
	{0x1.3dea64c123422p+00, 0x1.ada0911f09ebcp-55},
	{0x1.3fa4504ac801cp+00, -0x1.7d023f956f9f3p-54},
	{0x1.4160a21f72e2ap+00, -0x1.ef3691c309278p-58},
	{0x1.431f5d950a897p+00, -0x1.1c7dde35f7999p-55},
	{0x1.44e086061892dp+00, 0x1.89b7a04ef80dp-59},
	{0x1.46a41ed1d0057p+00, 0x1.c944bd1648a76p-54},
	{0x1.486a2b5c13cdp+00, 0x1.3c1a3b69062fp-56},
	{0x1.4a32af0d7d3dep+00, 0x1.9cb62f3d1be56p-54},
	{0x1.4bfdad5362a27p+00, 0x1.d4397afec42e2p-56},
	{0x1.4dcb299fddd0dp+00, 0x1.8ecdbbc6a7833p-54},
	{0x1.4f9b2769d2ca7p+00, -0x1.4b309d25957e3p-54},
	{0x1.516daa2cf6642p+00, -0x1.f768569bd93efp-55},
	{0x1.5342b569d4f82p+00, -0x1.07abe1db13cadp-55},
	{0x1.551a4ca5d920fp+00, -0x1.d689cefede59bp-55},
	{0x1.56f4736b527dap+00, 0x1.9bb2c011d93adp-54},
	{0x1.58d12d497c7fdp+00, 0x1.295e15b9a1de8p-55},
	{0x1.5ab07dd485429p+00, 0x1.6324c054647adp-54},
	{0x1.5c9268a5946b7p+00, 0x1.c4b1b816986a2p-60},
	{0x1.5e76f15ad2148p+00, 0x1.ba6f93080e65ep-54},
	{0x1.605e1b976dc09p+00, -0x1.3e2429b56de47p-54},
	{0x1.6247eb03a5585p+00, -0x1.383c17e40b497p-54},
	{0x1.6434634ccc32p+00, -0x1.c483c759d8933p-55},
	{0x1.6623882552225p+00, -0x1.bb60987591c34p-54},
	{0x1.68155d44ca973p+00, 0x1.038ae44f73e65p-57},
	{0x1.6a09e667f3bcdp+00, -0x1.bdd3413b26456p-54},
	{0x1.6c012750bdabfp+00, -0x1.2895667ff0b0dp-56},
	{0x1.6dfb23c651a2fp+00, -0x1.bbe3a683c88abp-57},
	{0x1.6ff7df9519484p+00, -0x1.83c0f25860ef6p-55},
	{0x1.71f75e8ec5f74p+00, -0x1.16e4786887a99p-55},
	{0x1.73f9a48a58174p+00, -0x1.0a8d96c65d53cp-54},
	{0x1.75feb564267c9p+00, -0x1.0245957316dd3p-54},
	{0x1.780694fde5d3fp+00, 0x1.866b80a02162dp-54},
	{0x1.7a11473eb0187p+00, -0x1.41577ee04992fp-55},
	{0x1.7c1ed0130c132p+00, 0x1.f124cd1164dd6p-54},
	{0x1.7e2f336cf4e62p+00, 0x1.05d02ba15797ep-56},
	{0x1.80427543e1a12p+00, -0x1.27c86626d972bp-54},
	{0x1.82589994cce13p+00, -0x1.d4c1dd41532d8p-54},
	{0x1.8471a4623c7adp+00, -0x1.8d684a341cdfbp-55},
	{0x1.868d99b4492edp+00, -0x1.fc6f89bd4f6bap-54},
	{0x1.88ac7d98a6699p+00, 0x1.994c2f37cb53ap-54},
	{0x1.8ace5422aa0dbp+00, 0x1.6e9f156864b27p-54},
	{0x1.8cf3216b5448cp+00, -0x1.0d55e32e9e3aap-56},
	{0x1.8f1ae99157736p+00, 0x1.5cc13a2e3976cp-55},
	{0x1.9145b0b91ffc6p+00, -0x1.dd6792e582524p-54},
	{0x1.93737b0cdc5e5p+00, -0x1.75fc781b57ebcp-57},
	{0x1.95a44cbc8520fp+00, -0x1.64b7c96a5f039p-56},
	{0x1.97d829fde4e5p+00, -0x1.d185b7c1b85d1p-54},
	{0x1.9a0f170ca07bap+00, -0x1.173bd91cee632p-54},
	{0x1.9c49182a3f09p+00, 0x1.c7c46b071f2bep-56},
	{0x1.9e86319e32323p+00, 0x1.824ca78e64c6ep-56},
	{0x1.a0c667b5de565p+00, -0x1.359495d1cd533p-54},
	{0x1.a309bec4a2d33p+00, 0x1.6305c7ddc36abp-54},
	{0x1.a5503b23e255dp+00, -0x1.d2f6edb8d41e1p-54},
	{0x1.a799e1330b358p+00, 0x1.bcb7ecac563c7p-54},
	{0x1.a9e6b5579fdbfp+00, 0x1.0fac90ef7fd31p-54},
	{0x1.ac36bbfd3f37ap+00, -0x1.f9234cae76cdp-55},
	{0x1.ae89f995ad3adp+00, 0x1.7a1cd345dcc81p-54},
	{0x1.b0e07298db666p+00, -0x1.bdef54c80e425p-54},
	{0x1.b33a2b84f15fbp+00, -0x1.2805e3084d708p-57},
	{0x1.b59728de5593ap+00, -0x1.c71dfbbba6de3p-54},
	{0x1.b7f76f2fb5e47p+00, -0x1.5584f7e54ac3bp-56},
	{0x1.ba5b030a1064ap+00, -0x1.efcd30e54292ep-54},
	{0x1.bcc1e904bc1d2p+00, 0x1.23dd07a2d9e84p-55},
	{0x1.bf2c25bd71e09p+00, -0x1.efdca3f6b9c73p-54},
	{0x1.c199bdd85529cp+00, 0x1.11065895048ddp-55},
	{0x1.c40ab5fffd07ap+00, 0x1.b4537e083c60ap-54},
	{0x1.c67f12e57d14bp+00, 0x1.2884dff483cadp-54},
	{0x1.c8f6d9406e7b5p+00, 0x1.1acbc48805c44p-56},
	{0x1.cb720dcef9069p+00, 0x1.503cbd1e949dbp-56},
	{0x1.cdf0b555dc3fap+00, -0x1.dd83b53829d72p-55},
	{0x1.d072d4a07897cp+00, -0x1.cbc3743797a9cp-54},
	{0x1.d2f87080d89f2p+00, -0x1.d487b719d8578p-54},
	{0x1.d5818dcfba487p+00, 0x1.2ed02d75b3707p-55},
	{0x1.d80e316c98398p+00, -0x1.11ec18beddfe8p-54},
	{0x1.da9e603db3285p+00, 0x1.c2300696db532p-54},
	{0x1.dd321f301b46p+00, 0x1.2da5778f018c3p-54},
	{0x1.dfc97337b9b5fp+00, -0x1.1a5cd4f184b5cp-54},
	{0x1.e264614f5a129p+00, -0x1.7b627817a1496p-54},
	{0x1.e502ee78b3ff6p+00, 0x1.39e8980a9cc8fp-55},
	{0x1.e7a51fbc74c83p+00, 0x1.2d522ca0c8de2p-54},
	{0x1.ea4afa2a490dap+00, -0x1.e9c23179c2893p-54},
	{0x1.ecf482d8e67f1p+00, -0x1.c93f3b411ad8cp-54},
	{0x1.efa1bee615a27p+00, 0x1.dc7f486a4b6bp-54},
	{0x1.f252b376bba97p+00, 0x1.3a1a5bf0d8e43p-54},
	{0x1.f50765b6e454p+00, 0x1.9d3e12dd8a18bp-54},
	{0x1.f7bfdad9cbe14p+00, -0x1.dbb12d006350ap-54},
	{0x1.fa7c1819e90d8p+00, 0x1.74853f3a5931ep-55},
	{0x1.fd3c22b8f71f1p+00, 0x1.2eb74966579e7p-57},
}

const (
	rbPowLn2h = 0x1.62e42fefa38p-01
	rbPowLn2l = 0x1.ef35793c7673p-45
	rbPowL1   = 0x1.62e42fefcp-08
	rbPowL2   = -0x1.c610ca86c3899p-44
	rbPowN2   = 0x1.71547652b82fep+07
)
