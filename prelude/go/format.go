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
		if verb == 'B' {
			return strings.ToUpper(fmt.Sprintf(spec+"b", rbFormatInt(v)))
		}
		return fmt.Sprintf(spec+string(verb), rbFormatInt(v))
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
		return int(n) // truncates, as MRI
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
