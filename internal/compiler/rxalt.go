package compiler

// regexp2 for patterns RE2 rejects, emitted only into programs that may need it so the rest skip the dependency (decision 242).

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// rxMatchTimeout bounds one regexp2 match, which unlike RE2's can take exponential time; MRI memoizes and has no such blowup (decision 242).
var rxMatchTimeout = 10 * time.Second

// rxAltReady registers the engine as a variable, not in init: a program's pattern variables initialize before init runs, so they name it to go after it.
var rxAltReady = func() bool {
	rxAltCompile = func(pattern string) (rxAlt, error) {
		// RE2 mode keeps \w, \d and \s ASCII, as Ruby's; the translator already spelled Onigmo's classes and flags.
		re, err := regexp2.Compile(rxAltCaret(pattern), regexp2.RE2)
		if err != nil {
			return nil, fmt.Errorf("regexp2: %w", err)
		}
		re.MatchTimeout = rxMatchTimeout
		return rxAlt2{re}, nil
	}
	return true
}()

// rxAltCaret spells Onigmo's ^ (a line start, but not the end after a final newline, which regexp2's (?m)^ also is) outside classes.
func rxAltCaret(pattern string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '\\' && i+1 < len(pattern):
			b.WriteString(pattern[i : i+2])
			i++
			continue
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '^' && !inClass:
			b.WriteString(`(?:\A|(?<=\n)(?!\z))`)
			continue
		}
		b.WriteByte(pattern[i])
	}
	return b.String()
}

type rxAlt2 struct{ re *regexp2.Regexp }

func (a rxAlt2) names() []string {
	nums := a.re.GetGroupNumbers()
	out := make([]string, len(nums))
	for i, n := range nums {
		if name := a.re.GroupNameFromNumber(n); name != strconv.Itoa(n) {
			out[i] = name
		}
	}
	return out
}

// findAll converts regexp2's rune offsets to the byte offsets the rest of the runtime uses.
func (a rxAlt2) findAll(s string, from, n int) ([][]int, error) {
	byteAt := make([]int, 0, len(s)+1) // rune index -> byte offset
	for i := range s {
		byteAt = append(byteAt, i)
	}
	byteAt = append(byteAt, len(s))
	start := utf8.RuneCountInString(s[:from])
	var out [][]int
	m, err := a.re.FindRunesMatchStartingAt([]rune(s), start)
	for err == nil && m != nil && (n < 0 || len(out) < n) {
		groups := m.Groups()
		loc := make([]int, 2*len(groups))
		for i, g := range groups {
			loc[2*i], loc[2*i+1] = -1, -1
			if len(g.Captures) > 0 {
				loc[2*i], loc[2*i+1] = byteAt[g.Index], byteAt[g.Index+g.Length]
			}
		}
		out = append(out, loc)
		if n >= 0 && len(out) == n {
			break // no search past the last wanted match, which could only time out
		}
		m, err = a.re.FindNextMatch(m)
	}
	switch {
	case err == nil:
		return out, nil
	case strings.HasPrefix(err.Error(), "match timeout"): // regexp2 words it as plain text, quoting the whole subject
		return nil, errRxTimeout
	}
	return nil, fmt.Errorf("regexp2: %w", err)
}
