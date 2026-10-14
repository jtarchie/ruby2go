//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// rbFormat is Kernel#format: each directive is MRI's, handed to fmt with
// the argument converted as MRI converts it. %<name>… and %{name} read
// a Hash argument. ponytail: a negative %x/%o/%b prints "-ff", not MRI's
// two's-complement "..f01"; %a is not supported.
func rbFormat(format string, args []any) string {
	var b strings.Builder
	next := 0
	arg := func() any {
		if next >= len(args) {
			panic(NewArgumentError(Ref(String("too few arguments"))))
		}
		next++
		return rbUnbox(args[next-1])
	}
	named := func(name string) any {
		if len(args) != 1 {
			panic(NewArgumentError(Ref(String("one hash required"))))
		}
		h, ok := rbUnbox(args[0]).(Hash_Any)
		if !ok {
			panic(NewArgumentError(Ref(String("one hash required"))))
		}
		v, ok := h._ToAny().vals[Symbol(name)]
		if !ok {
			panic(NewKeyError(Ref(String("key<" + name + "> not found"))))
		}
		return rbUnbox(v)
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		start := i
		i++
		if i >= len(format) {
			panic(NewArgumentError(Ref(String("incomplete format specifier; use %% (double %) instead"))))
		}
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}
		var v any
		have := false
		spec := []byte{'%'}
		for ; i < len(format); i++ {
			switch d := format[i]; {
			case strings.IndexByte("-+ 0#", d) >= 0:
				spec = append(spec, d)
				continue
			case d == '<' || d == '{':
				end := strings.IndexByte(format[i:], map[byte]byte{'<': '>', '{': '}'}[d])
				if end < 0 {
					panic(NewArgumentError(Ref(String("malformed name - unmatched parenthesis"))))
				}
				v, have = named(format[i+1:i+end]), true
				i += end
				if d == '{' {
					fmt.Fprintf(&b, string(spec)+"s", string(rbToS(v)))
					goto done
				}
				continue
			case d == '*':
				w, ok := arg().(Integer)
				if !ok {
					panic(NewTypeError(Ref(String("width must be an Integer"))))
				}
				spec = strconv.AppendInt(spec, int64(w), 10)
				continue
			case d >= '1' && d <= '9' && len(spec) == 1 && rbFormatDollar(format[i:]) > 0: // `%2$s`: argument 2 (1-based)
				j := strings.IndexByte(format[i:], '$')
				n, _ := strconv.Atoi(format[i : i+j])
				if n > len(args) {
					panic(NewArgumentError(Ref(String("too few arguments"))))
				}
				v, have = args[n-1], true
				i += j
				continue
			case d >= '0' && d <= '9' || d == '.':
				spec = append(spec, d)
				continue
			}
			break
		}
		if i >= len(format) {
			panic(NewArgumentError(Ref(String("malformed format string - " + format[start:]))))
		}
		if !have {
			v = arg()
		}
		b.WriteString(rbFormatOne(string(spec), format[i], v))
	done:
	}
	return b.String()
}

// rbFormatOne formats v for one directive; spec is its flags, width and precision.
func rbFormatOne(spec string, verb byte, v any) string {
	switch verb {
	case 's':
		return fmt.Sprintf(spec+"s", string(rbToS(v)))
	case 'p':
		return fmt.Sprintf(spec+"s", string(rbInspect(v)))
	case 'c':
		if n, ok := v.(Integer); ok {
			return fmt.Sprintf(spec+"c", rune(n))
		}
		r, _ := utf8.DecodeRuneInString(string(rbToS(v)))
		return fmt.Sprintf(spec+"c", r)
	case 'd', 'i', 'u':
		return fmt.Sprintf(spec+"d", rbFormatInt(v))
	case 'x', 'X', 'o', 'b', 'B':
		n := rbFormatInt(v)
		if n < 0 && !strings.ContainsAny(spec, "+ ") {
			return rbFormatTwos(spec, verb, n)
		}
		if n == 0 {
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

// rbFormatDollar is the length of a leading `digits$` (a positional argument), or 0.
func rbFormatDollar(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && s[i] == '$' {
		return i
	}
	return 0
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
	var out []byte
	for n != -1 {
		out = append([]byte{digits[n&(1<<bits-1)]}, out...)
		n >>= bits
	}
	top := digits[1<<bits-1]
	body := append([]byte{top}, out...)
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
	for ; fill > 0; fill-- {
		body = append([]byte{top}, body...)
	}
	s := prefix + ".." + string(body)
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", width, s)
	}
	return fmt.Sprintf("%*s", width, s)
}
