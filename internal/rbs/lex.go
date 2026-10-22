package rbs

import (
	"fmt"
	"strconv"
	"strings"
)

func tokenize(s string) ([]string, error) {
	var toks []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '-' && i+1 < len(s) && s[i+1] == '>':
			toks = append(toks, "->")
			i += 2
		case c == '\'':
			tok, ni, serr := scanSingle(s, i)
			if serr != nil {
				return nil, serr
			}
			toks = append(toks, tok)
			i = ni
		case c == '`': // a quoted name (`` `type` ``): any text but a backquote
			j := strings.IndexByte(s[i+1:], '`')
			if j <= 0 {
				return nil, fmt.Errorf("rbs: bad quoted name in %q", s)
			}
			toks = append(toks, s[i:i+j+2])
			i += j + 2
		case c == '"':
			tok, ni, serr := scanString(s, i)
			if serr != nil {
				return nil, serr
			}
			toks = append(toks, tok)
			i = ni
		case isNumberStart(s, i):
			tok, ni := scanNumber(s, i)
			toks = append(toks, tok)
			i = ni
		case c == '*' && i+1 < len(s) && s[i+1] == '*':
			toks = append(toks, "**")
			i += 2
		case c == ':' && i+1 < len(s) && s[i+1] == ':':
			// leading `::Foo`
			toks = append(toks, "::")
			i += 2
		case c == '=' && i+1 < len(s) && s[i+1] == '>':
			toks = append(toks, "=>")
			i += 2
		case strings.IndexByte("()[]{},|?*:&^<", c) >= 0:
			toks = append(toks, string(c))
			i++
		case isIdentStart(c):
			tok, ni, ierr := scanIdent(s, i)
			if ierr != nil {
				return nil, ierr
			}
			toks = append(toks, tok)
			i = ni
		default:
			return nil, fmt.Errorf("rbs: unexpected %q in %q", string(c), s)
		}
	}
	return toks, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isNumberStart reports whether an integer literal starts at i.
func isNumberStart(s string, i int) bool {
	c := s[i]
	return c >= '0' && c <= '9' || c == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'
}

// scanIdent reads one identifier or `A::B::C` name token.
func scanIdent(s string, i int) (string, int, error) {
	j := i
	for j < len(s) && (isIdentStart(s[j]) || s[j] >= '0' && s[j] <= '9') {
		j++
	}
	for j+1 < len(s) && s[j] == ':' && s[j+1] == ':' {
		k := j + 2
		if k == len(s) || !isIdentStart(s[k]) {
			return "", 0, fmt.Errorf("rbs: expected a name after \"::\" in %q", s)
		}
		for k < len(s) && (isIdentStart(s[k]) || s[k] >= '0' && s[k] <= '9') {
			k++
		}
		j = k
	}
	return s[i:j], j, nil
}

// scanString reads a `"..."` literal (with backslash escapes) as one token.
func scanString(s string, i int) (string, int, error) {
	j := i + 1
	for j < len(s) && s[j] != '"' {
		if s[j] == '\\' {
			j++
		}
		j++
	}
	if j >= len(s) {
		return "", 0, fmt.Errorf("rbs: unterminated string in %q", s)
	}
	return s[i : j+1], j + 1, nil
}

// scanSingle reads a `'...'` literal, whose only escapes are `\'` and `\\` (RBS's
// rbs_unescape.c), as the double-quoted token of the same text.
func scanSingle(s string, i int) (string, int, error) {
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\'':
			return strconv.Quote(b.String()), j + 1, nil
		case s[j] == '\\' && j+1 < len(s) && (s[j+1] == '\'' || s[j+1] == '\\'):
			j++
		}
		b.WriteByte(s[j])
	}
	return "", 0, fmt.Errorf("rbs: unterminated string in %q", s)
}

// paramName prints a parameter name, backquoted unless it is a plain identifier.
func paramName(n string) string {
	for i := range len(n) {
		if !isIdentStart(n[i]) && (i == 0 || n[i] < '0' || n[i] > '9') {
			return "`" + n + "`"
		}
	}
	return n
}

// varName is the parameter name tok spells, backquotes stripped, or "".
func varName(tok string) string {
	switch {
	case tok == "":
		return ""
	case tok[0] == '`':
		return tok[1 : len(tok)-1]
	case isIdentStart(tok[0]):
		return tok
	}
	return ""
}

// scanNumber reads an integer literal (with an optional sign and `_`).
func scanNumber(s string, i int) (string, int) {
	j := i
	if s[j] == '-' {
		j++
	}
	for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '_') {
		j++
	}
	return s[i:j], j
}
