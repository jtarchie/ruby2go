//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	rbHTMLEscaper  = strings.NewReplacer("'", "&#39;", "&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")
	rbHTMLEntities = regexp.MustCompile(`&(amp|quot|gt|lt|apos|#[0-9]+|#[xX][0-9A-Fa-f]+);`)
)

func rbCGIUnescape(s string, plus bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '+' && plus {
			b.WriteByte(' ')
			continue
		}
		if c == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func rbHTMLUnescape(s string) string {
	return rbHTMLEntities.ReplaceAllStringFunc(s, func(m string) string {
		switch e := m[1 : len(m)-1]; e {
		case "amp":
			return "&"
		case "quot":
			return "\""
		case "gt":
			return ">"
		case "lt":
			return "<"
		case "apos":
			return "'"
		default:
			base, digits := 10, e[1:]
			if digits[0] == 'x' || digits[0] == 'X' {
				base, digits = 16, digits[1:]
			}
			v, err := strconv.ParseInt(digits, base, 32)
			if err != nil || v > utf8.MaxRune {
				return m
			}
			return string(rune(v))
		}
	})
}
