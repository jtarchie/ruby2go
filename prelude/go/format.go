//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// rbFormat is Kernel#format: each directive is MRI's, handed to fmt with
// the argument converted as MRI converts it. %<name>… and %{name} read
// a Hash argument. The directive parser follows MRI's sprintf.c (posarg:
// 0 none yet, >0 the unnumbered count, -1 numbered, -2 named), so its
// ArgumentErrors are MRI's.
func rbFormat(format string, args []any) string {
	var b strings.Builder
	posarg, nextarg := 0, 0
	raise := func(msg string) { panic(NewArgumentError(Ref(String(msg)))) }
	nextArg := func() any {
		switch posarg {
		case -1:
			raise("unnumbered(" + strconv.Itoa(nextarg+1) + ") mixed with numbered")
		case -2:
			raise("unnumbered(" + strconv.Itoa(nextarg+1) + ") mixed with named")
		}
		nextarg++
		posarg = nextarg
		if nextarg > len(args) {
			raise("too few arguments")
		}
		return rbUnbox(args[nextarg-1])
	}
	posArg := func(n int) any {
		switch {
		case posarg > 0:
			raise("numbered(" + strconv.Itoa(n) + ") after unnumbered(" + strconv.Itoa(posarg) + ")")
		case posarg == -2:
			raise("numbered(" + strconv.Itoa(n) + ") after named")
		case n < 1:
			raise("invalid index - " + strconv.Itoa(n) + "$")
		}
		posarg = -1
		if n > len(args) {
			raise("too few arguments")
		}
		return rbUnbox(args[n-1])
	}
	named := func(name string) any {
		switch {
		case posarg > 0:
			raise("named" + name + " after unnumbered(" + strconv.Itoa(posarg) + ")")
		case posarg == -1:
			raise("named" + name + " after numbered")
		}
		posarg = -2
		if len(args) != 1 {
			raise("one hash required")
		}
		h, ok := rbUnbox(args[0]).(Hash_Any)
		if !ok {
			raise("one hash required")
		}
		key := Symbol(name[1 : len(name)-1])
		v, ok := h._ToAny().rbGet(key)
		if !ok {
			e := NewKeyError(Ref(String("key" + name + " not found")))
			e.receiver, e.key, e.has_key = args[0], key, true
			panic(e)
		}
		return rbUnbox(v)
	}
	// num is MRI's GETNUM: the digits at format[i:] and the index after them; running off the end is malformed.
	num := func(i int, what string) (int, int) {
		n := 0
		for ; i < len(format) && format[i] >= '0' && format[i] <= '9'; i++ {
			n = n*10 + int(format[i]-'0')
			if n > math.MaxInt32 {
				raise(what + " too big")
			}
		}
		if i >= len(format) {
			raise("malformed format string - %*[0-9]")
		}
		return n, i
	}
	toInt := func(v any) int {
		var w Integer
		switch x := v.(type) {
		case Integer:
			w = x
		case Float:
			w = rbFloatToI(float64(x))
		default:
			panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
		}
		return rbNum2Int(w)
	}
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		i++
		if i >= len(format) {
			raise("incomplete format specifier; use %% (double %) instead")
		}
		var v any
		have, name := false, ""
		flags := ""
		width, prec := 0, 0
		hasWidth, hasPrec := false, false
		checkWidth := func() {
			if hasWidth {
				raise("width given twice")
			}
			if hasPrec {
				raise("width after precision")
			}
		}
		// aster is MRI's GETASTER at the `*` format[i]: the next argument, or argument N for `*N$`.
		aster := func(what string) int {
			n, j := num(i+1, what)
			if format[j] == '$' {
				i = j
				return toInt(posArg(n))
			}
			return toInt(nextArg())
		}
	directive:
		for ; ; i++ {
			if i >= len(format) {
				raise("malformed format string")
			}
			d := format[i]
			switch {
			case strings.IndexByte("-+ 0#", d) >= 0:
				if hasWidth {
					raise("flag after width")
				}
				if hasPrec {
					raise("flag after precision")
				}
				flags += string(d)
			case d >= '1' && d <= '9':
				n, j := num(i, "width")
				if format[j] == '$' {
					if have {
						raise("value given twice - " + strconv.Itoa(n) + "$")
					}
					v, have = posArg(n), true
					i = j
					continue
				}
				checkWidth()
				width, hasWidth = n, true
				i = j - 1
			case d == '<' || d == '{':
				closer := byte('>')
				if d == '{' {
					closer = '}'
				}
				end := strings.IndexByte(format[i:], closer)
				if end < 0 {
					raise("malformed name - unmatched parenthesis")
				}
				if name != "" {
					raise("named" + format[i:i+end+1] + " after " + name)
				}
				if have {
					raise("named" + format[i:i+end+1] + " after numbered")
				}
				name = format[i : i+end+1]
				v, have = named(name), true
				i += end
				if d == '{' {
					b.WriteString(rbFormatOne(rbFormatSpec(flags, width, hasWidth, prec, hasPrec), 's', v))
					break directive
				}
			case d == '*':
				checkWidth()
				hasWidth = true
				width = aster("width")
				if width < 0 {
					flags += "-"
					width = -width
				}
			case d == '.':
				if hasPrec {
					raise("precision given twice")
				}
				hasPrec = true
				if i+1 < len(format) && format[i+1] == '*' {
					i++
					if prec = aster("precision"); prec < 0 {
						hasPrec = false // a negative precision is ignored
					}
					continue
				}
				n, j := num(i+1, "precision")
				prec, i = n, j-1
			case d == '%':
				if flags != "" || hasWidth || hasPrec || have {
					raise("invalid format character - %")
				}
				b.WriteByte('%')
				break directive
			case strings.IndexByte("spcdiuxXobBaAfeEgG", d) >= 0:
				if !have {
					v = nextArg()
				}
				b.WriteString(rbFormatOne(rbFormatSpec(flags, width, hasWidth, prec, hasPrec), d, v))
				break directive
			case d > ' ' && d < 0x7f:
				raise("malformed format string - %" + string(d))
			default:
				raise("malformed format string")
			}
		}
	}
	if posarg >= 0 && nextarg < len(args) && (len(args) != 1 || !rbFormatIsHash(args[0])) && rbVerbose != nil && bool(*rbVerbose) {
		rbWarnLoc("too many arguments for format string")
	}
	return b.String()
}

func rbFormatIsHash(a any) bool {
	_, ok := rbUnbox(a).(Hash_Any)
	return ok
}

// rbFormatSpec is the fmt directive prefix for parsed flags, width and precision.
func rbFormatSpec(flags string, width int, hasWidth bool, prec int, hasPrec bool) string {
	spec := "%" + flags
	if hasWidth {
		spec += strconv.Itoa(width)
	}
	if hasPrec {
		spec += "." + strconv.Itoa(prec)
	}
	return spec
}

// rbFormatOne formats v for one directive; spec is its flags, width and precision.
func rbFormatOne(spec string, verb byte, v any) string {
	if s, ok := rbFormatFast(spec, verb, v); ok {
		return s
	}
	if verb == 's' || verb == 'p' || verb == 'c' {
		spec = rbFormatNoZeroFlag(spec) // the 0 flag pads numbers only: Go's fmt would zero-pad these
	}
	switch verb {
	case 's':
		return fmt.Sprintf(spec+"s", string(rbToS(v)))
	case 'p':
		return fmt.Sprintf(spec+"s", string(rbInspect(v)))
	case 'c':
		if k := strings.IndexByte(spec, '.'); k >= 0 {
			spec = spec[:k] // MRI ignores a precision on %c
		}
		var r rune
		if f, ok := v.(Float); ok {
			v = rbFloatToI(float64(f))
		}
		switch x := v.(type) {
		case Integer:
			if x < 0 || x > utf8.MaxRune || !utf8.ValidRune(rune(x)) {
				panic(NewArgumentError(Ref(String("invalid character"))))
			}
			r = rune(x)
		case String:
			if x == "" {
				return fmt.Sprintf(spec+"s", "")
			}
			r, _ = utf8.DecodeRuneInString(string(x))
		case nil:
			panic(NewTypeError(Ref(String("no implicit conversion from nil to integer"))))
		default:
			panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
		}
		return fmt.Sprintf(spec+"c", r)
	case 'd', 'i', 'u':
		return fmt.Sprintf(spec+"d", rbFormatInt(v))
	case 'x', 'X', 'o', 'b', 'B':
		n := rbFormatInt(v)
		if n < 0 && !strings.ContainsAny(spec, "+ ") {
			return rbFormatTwos(spec, verb, n)
		}
		if n == 0 {
			if verb == 'o' && strings.Contains(spec, "#") {
				if k := strings.IndexByte(spec, '.'); k >= 0 {
					spec = spec[:k] // %#o always shows the leading 0, even at precision 0
				}
			}
			spec = strings.ReplaceAll(spec, "#", "") // MRI's %#x of 0 is "0", with no prefix
		}
		flags := spec[1:]
		if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
			flags = flags[:k]
		}
		if verb != 'o' && strings.Contains(flags, "#") && strings.Contains(flags, "0") && !strings.Contains(flags, "-") && !strings.Contains(spec, ".") {
			// MRI's width counts the 0x/0b prefix, Go's does not
			w, _ := strconv.Atoi(strings.TrimLeft(spec[1:], "#0+ "))
			prefix := map[byte]string{'x': "0x", 'X': "0X", 'b': "0b", 'B': "0B"}[verb]
			body := fmt.Sprintf("%0*"+string(map[byte]byte{'x': 'x', 'X': 'X', 'b': 'b', 'B': 'b'}[verb]), max(w-len(prefix), 0), n)
			if verb == 'B' {
				body = strings.ToUpper(body)
			}
			return prefix + body
		}
		if verb == 'B' {
			return strings.ToUpper(fmt.Sprintf(spec+"b", n))
		}
		return fmt.Sprintf(spec+string(verb), n)
	case 'a', 'A':
		return rbFormatHexFloat(spec, verb, rbFormatFloat(v))
	case 'f', 'e', 'E', 'g', 'G':
		f := rbFormatFloat(v)
		if (verb == 'g' || verb == 'G') && !strings.Contains(spec, ".") {
			spec += ".6" // C's default precision; Go's %g is shortest-exact
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			s := "Inf"
			switch {
			case math.IsNaN(f):
				s = "NaN"
			case f < 0:
				s = "-Inf"
			case strings.Contains(spec, "+"):
				s = "+Inf"
			}
			w, _ := strconv.Atoi(strings.TrimLeft(strings.SplitN(spec[1:], ".", 2)[0], "-+ 0#"))
			if strings.Contains(spec, "-") {
				return fmt.Sprintf("%-*s", w, s)
			}
			return fmt.Sprintf("%*s", w, s)
		}
		return fmt.Sprintf(spec+string(verb), f)
	}
	panic(NewArgumentError(Ref(String("malformed format string - %" + string(verb)))))
}

// rbFormatInt reads a %d argument as MRI: a Float truncated, a String parsed.
func rbFormatInt(v any) int {
	switch n := v.(type) {
	case Integer:
		return int(n)
	case Float:
		return int(rbFloatToI(float64(n))) // truncates, as MRI; NaN and Inf are FloatDomainError, past 64 bits a RangeError (decision 35)
	case String:
		return int(rbStrictInt(n, 0)) // Kernel#Integer's rules: "1__2" is invalid
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into Integer"))))
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(v) + " into Integer"))))
}

func rbFormatFloat(v any) float64 {
	switch n := v.(type) {
	case Integer:
		return float64(n)
	case Float:
		return float64(n)
	case String:
		return float64(rbStrictFloat(n)) // Kernel#Float's rules
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into Float"))))
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(v) + " into Float"))))
}

// rbFormatHexFloat is %a/%A as C prints it: Go's hex float without the exponent's leading zero, then flags and width.
func rbFormatHexFloat(spec string, verb byte, f float64) string {
	prec := -1
	flags, rest := spec[1:], ""
	if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
		flags, rest = flags[:k], flags[k:]
	}
	width := rest
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		width = rest[:dot]
		prec, _ = strconv.Atoi(rest[dot+1:])
	}
	var s string
	switch {
	case math.IsNaN(f):
		s = "NaN"
	case math.IsInf(f, 0):
		s = map[bool]string{true: "Inf", false: "-Inf"}[f > 0]
	default:
		s = strconv.FormatFloat(f, 'x', prec, 64)
		if p := strings.LastIndexAny(s, "+-"); p > 0 && s[p-1] == 'p' {
			exp := strings.TrimLeft(s[p+1:], "0")
			if exp == "" {
				exp = "0"
			}
			s = s[:p+1] + exp
		}
		if p := strings.IndexByte(s, 'p'); strings.Contains(flags, "#") && !strings.Contains(s, ".") {
			s = s[:p] + "." + s[p:] // # forces the point, as C's
		}
	}
	if !strings.HasPrefix(s, "-") {
		switch {
		case strings.Contains(flags, "+"):
			s = "+" + s
		case strings.Contains(flags, " "):
			s = " " + s
		}
	}
	if verb == 'A' && !math.IsNaN(f) && !math.IsInf(f, 0) {
		s = strings.ToUpper(s)
	}
	w, _ := strconv.Atoi(width)
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", w, s)
	}
	if x := strings.IndexAny(s, "xX"); x >= 0 && strings.Contains(flags, "0") && len(s) < w {
		s = s[:x+1] + strings.Repeat("0", w-len(s)) + s[x+1:] // zeros go after the 0x, as C's
	}
	return fmt.Sprintf("%*s", w, s)
}

// rbFormatTwos is MRI's %x/%o/%b of a negative number without + or space: its two's complement with ".." for the
// endless leading ones (-196 is "..f3c"); zero padding and precision fill with that digit, both counting the "..".
func rbFormatTwos(spec string, verb byte, n int) string {
	bits := map[byte]uint{'x': 4, 'X': 4, 'o': 3, 'b': 1, 'B': 1}[verb]
	digits := "0123456789abcdef"
	if verb == 'X' {
		digits = "0123456789ABCDEF"
	}
	var rev []byte
	for n != -1 {
		rev = append(rev, digits[n&(1<<bits-1)])
		n >>= bits
	}
	slices.Reverse(rev)
	top := digits[1<<bits-1]
	body := append([]byte{top}, rev...)
	flags, rest := spec[1:], ""
	if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
		flags, rest = flags[:k], flags[k:]
	}
	var width int
	prec := -1
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		width, _ = strconv.Atoi(rest[:dot])
		prec, _ = strconv.Atoi(rest[dot+1:])
	} else {
		width, _ = strconv.Atoi(rest)
	}
	prefix := ""
	if strings.Contains(flags, "#") && verb != 'o' {
		prefix = map[byte]string{'x': "0x", 'X': "0X", 'b': "0b", 'B': "0B"}[verb]
	}
	fill := 0
	switch {
	case prec >= 0:
		fill = prec - 2 - len(body)
	case strings.Contains(flags, "0") && !strings.Contains(flags, "-"):
		fill = width - len(prefix) - 2 - len(body)
	}
	if fill > 0 {
		rbCheckAlloc(fill, 1)
		body = append(bytes.Repeat([]byte{top}, fill), body...)
	}
	s := prefix + ".." + string(body)
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", width, s)
	}
	return fmt.Sprintf("%*s", width, s)
}

// rbFormatNoZeroFlag drops the 0 from spec's flags (the run of -+ #0 after the %), leaving width digits alone.
func rbFormatNoZeroFlag(spec string) string {
	i := 1
	for i < len(spec) && strings.IndexByte("-+ #0", spec[i]) >= 0 {
		i++
	}

	return spec[:1] + strings.ReplaceAll(spec[1:i], "0", "") + spec[i:]
}
