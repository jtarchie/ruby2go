//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbStrictInt is Kernel#Integer(str, base): String#to_i's syntax, but the
// whole string bar surrounding whitespace must be the number. Base 0 (or
// -1) reads a 0b/0o/0d/0x prefix or a leading 0 as octal; another base
// still allows its own prefix.
func rbStrictInt(s String, base int) Integer {
	if base == 1 || base > 36 || base < -36 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(base)))))
	}
	bad := func() {
		panic(NewArgumentError(Ref("invalid value for Integer(): " + rbStringInspect(string(s)))))
	}
	t := strings.Trim(string(s), " \t\n\v\f\r")
	sign := ""
	if t != "" && (t[0] == '+' || t[0] == '-') {
		sign, t = t[:1], t[1:]
	}
	auto := base == 0 || base == -1
	if base < 0 {
		base = -base
	}
	prefixed := false
	if len(t) >= 2 {
		pb := map[string]int{"0b": 2, "0o": 8, "0d": 10, "0x": 16}[strings.ToLower(t[:2])]
		if pb != 0 && (auto || pb == base) {
			base, t, prefixed = pb, t[2:], true
		}
	}
	if auto && !prefixed {
		base = 10
		if len(t) > 1 && t[0] == '0' {
			base = 8
		}
	}
	if t == "" || t[0] == '_' || t[len(t)-1] == '_' || strings.Contains(t, "__") || strings.ContainsAny(t, "+-") {
		bad()
	}
	digits := strings.ReplaceAll(t, "_", "")
	n, err := strconv.ParseInt(sign+digits, base, 64)
	if errors.Is(err, strconv.ErrRange) { // MRI returns a Bignum (decision 35)
		panic(NewRangeError(Ref(String(sign + digits + " overflows Integer (64-bit; no Bignum)"))))
	}
	if err != nil {
		bad()
	}
	return Integer(n)
}
