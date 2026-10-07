//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	rbShellUnsafe = regexp.MustCompile(`[^A-Za-z0-9_\-.,:+/@\n]`)
	rbShellDQEsc  = regexp.MustCompile("\\\\([$`\"\\\\\n])")
)

// rbShellSplit is MRI's Shellwords.shellsplit, whose scan regexp (\G and an
// atomic group, which RE2 lacks) is unrolled: each step reads optional
// whitespace, one word / '…' / "…" / backslash escape, then one optional
// separator, which ends the field.
func rbShellSplit(line string) []string {
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' }
	var words []string
	var field strings.Builder
	pos := 0
	for pos < len(line) {
		start := pos
		for pos < len(line) && isSpace(line[pos]) {
			pos++
		}
		if pos == len(line) {
			break
		}
		c, ok := line[pos], true
		switch c {
		case '\'':
			end := strings.IndexAny(line[pos+1:], "'\x00")
			if ok = end >= 0 && line[pos+1+end] == '\''; ok {
				field.WriteString(line[pos+1 : pos+1+end])
				pos += end + 2
			}
		case '"':
			i := pos + 1
			for i < len(line) && line[i] != '"' && line[i] != 0 {
				if line[i] == '\\' {
					if i+1 >= len(line) || line[i+1] == 0 {
						break
					}
					i++
				}
				i++
			}
			if ok = i < len(line) && line[i] == '"'; ok {
				field.WriteString(rbShellDQEsc.ReplaceAllString(line[pos+1:i], "$1"))
				pos = i + 1
			}
		case '\\':
			pos++
			if pos < len(line) && line[pos] != 0 {
				_, n := utf8.DecodeRuneInString(line[pos:])
				field.WriteString(line[pos : pos+n])
				pos += n
			} else {
				field.WriteByte('\\')
			}
		case 0:
			ok = false
		default:
			end := pos
			for end < len(line) && !isSpace(line[end]) && !strings.ContainsRune("\x00\\'\"", rune(line[end])) {
				end++
			}
			field.WriteString(line[pos:end])
			pos = end
		}
		if !ok {
			what := "Unmatched quote"
			if c == 0 {
				what = "Nul character"
			}
			end := pos + 1
			if end < len(line) && isSpace(line[end]) {
				end++
			}
			text := line[start:end]
			if start > 0 {
				text = "..." + text
			}
			panic(NewArgumentError(Ref(String(fmt.Sprintf("%s at %d: %s", what, start, text)))))
		}
		if pos == len(line) || isSpace(line[pos]) {
			if pos < len(line) {
				pos++
			}
			words = append(words, field.String())
			field.Reset()
		}
	}
	return words
}
