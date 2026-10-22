//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"unicode/utf8"
)

// MRI's identifier byte; >= 0x80 includes invalid bytes, as Go's regexp read them (U+FFFD).
func rbIdentByte(c byte, first bool) bool {
	return c >= 0x80 || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || !first && c >= '0' && c <= '9'
}

// rbIdentEnd is where the identifier at s[i:] ends; i when there is none.
func rbIdentEnd(s string, i int) int {
	if i >= len(s) || !rbIdentByte(s[i], true) {
		return i
	}
	i++
	for i < len(s) && rbIdentByte(s[i], false) {
		i++
	}
	return i
}

// rbIsLabelSymbol is whether s prints as `s: v` in a Hash (an identifier, then optional ? or !).
func rbIsLabelSymbol(s string) bool {
	e := rbIdentEnd(s, 0)
	if e == 0 {
		return false
	}
	return e == len(s) || e+1 == len(s) && (s[e] == '?' || s[e] == '!')
}

// rbIsPlainSymbol is whether s prints as :s with no quotes.
func rbIsPlainSymbol(s string) bool {
	if s == "" {
		return false
	}
	switch c := s[0]; {
	case rbIdentByte(c, true):
		e := rbIdentEnd(s, 0)
		return e == len(s) || e+1 == len(s) && (s[e] == '?' || s[e] == '!' || s[e] == '=')
	case c == '@':
		i := 1
		if i < len(s) && s[i] == '@' {
			i++
		}
		e := rbIdentEnd(s, i)
		return e > i && e == len(s)
	case c == '$':
		return rbIsGlobalSymbol(s)
	}
	switch s {
	case "[]", "[]=", "+", "-", "*", "/", "%", "<", ">", "!", "~", "^", "&", "|", "`", "**", "<=>", "==", "===", "=~", "!=", "!~", "<<", ">>", "<=", ">=", "+@", "-@":
		return true
	}
	return false
}

// rbIsGlobalSymbol is the `$` arm of rbIsPlainSymbol: $name, $-x, $1.., or one punctuation character.
func rbIsGlobalSymbol(s string) bool {
	if len(s) == 1 {
		return false
	}
	if e := rbIdentEnd(s, 1); e > 1 {
		return e == len(s)
	}
	switch d := s[1]; {
	case d == '-': // then one more character: an ASCII word character or a non-ASCII one
		if len(s) == 2 {
			return false
		}
		if c := s[2]; c < 0x80 {
			return len(s) == 3 && (c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z')
		}
		_, n := utf8.DecodeRuneInString(s[2:])
		return 2+n == len(s)
	case d >= '1' && d <= '9':
		for _, c := range []byte(s[2:]) {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	if len(s) != 2 {
		return false
	}
	switch s[1] {
	case '~', '*', '$', '?', '!', '@', '/', '\\', ';', ',', '.', '=', ':', '<', '>', '"', '&', '`', '\'', '+', '0':
		return true
	}
	return false
}

func rbSymbolInspect(s string) String {
	if rbIsPlainSymbol(s) {
		return String(":" + s)
	}
	return ":" + rbStringInspectAs(s, false)
}
