//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbStrictFloat is Kernel#Float(str): String#to_f's syntax (rbFloatPrefix),
// or a hex float, covering the whole string bar surrounding whitespace.
// Out-of-range values become ±Infinity or 0 as in MRI, minus its warning.
func rbStrictFloat(s String) Float {
	t := strings.Trim(string(s), " \t\n\v\f\r")
	body := strings.TrimLeft(t, "+-")
	var f float64
	var err error
	switch {
	case len(t)-len(body) > 1:
		err = strconv.ErrSyntax
	case len(body) > 2 && strings.EqualFold(body[:2], "0x"):
		if f, err = strconv.ParseFloat(t, 64); err != nil && !strings.ContainsAny(body, "pP") {
			f, err = strconv.ParseFloat(t+"p0", 64)
		}
	default:
		if m := rbFloatPrefix.FindStringSubmatchIndex(t); m != nil && m[3] == len(t) {
			f, err = strconv.ParseFloat(strings.ReplaceAll(t, "_", ""), 64)
		} else {
			err = strconv.ErrSyntax
		}
	}
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		panic(NewArgumentError(Ref("invalid value for Float(): " + rbStringInspect(string(s)))))
	}
	return Float(f)
}

// rbFloatStep is MRI's ruby_float_step: n steps counted up front with its
// rounding allowance, each value beg + i*unit, the last clamped to end.
func rbFloatStep(beg, end, unit float64, yield func(Float) bool) {
	if unit == 0 {
		panic(NewArgumentError(Ref(String("step can't be 0"))))
	}
	n := (end - beg) / unit
	errAllow := (math.Abs(beg) + math.Abs(end) + math.Abs(end-beg)) / math.Abs(unit) * 2.220446049250313e-16
	if math.IsInf(unit, 0) {
		if unit > 0 && beg <= end || unit < 0 && beg >= end {
			yield(Float(beg))
		}
		return
	}
	if errAllow > 0.5 {
		errAllow = 0.5
	}
	if n < 0 {
		return
	}
	n = math.Floor(n + errAllow)
	for i := 0.0; i <= n; i++ {
		d := i*unit + beg
		if unit >= 0 && end < d || unit < 0 && end > d {
			d = end
		}
		if !yield(Float(d)) {
			return
		}
	}
}
