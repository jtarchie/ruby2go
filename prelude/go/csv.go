//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"strings"
)

func rbCSVStrOpt(opts *Hash[Symbol, any], key, def string) string {
	if v, ok := opts.vals[Symbol(key)]; ok && v != nil {
		if s, ok2 := v.(String); ok2 {
			return string(s)
		}
	}
	return def
}

func rbCSVBoolOpt(opts *Hash[Symbol, any], key string) bool {
	if v, ok := opts.vals[Symbol(key)]; ok && v != nil {
		if b, ok2 := v.(Boolean); ok2 {
			return bool(b)
		}
	}
	return false
}

// rbCSVParse splits s into rows; rowSep "" auto-detects LF/CRLF, skipBlanks drops empty lines, err is MRI's MalformedCSVError message.
func rbCSVParse(s, sep, quote, rowSep string, skipBlanks bool) (rows [][]*String, err string) {
	line := 1
	i := 0
	auto := rowSep == ""
	atEOL := func(j int) bool {
		if auto {
			return j >= len(s) || s[j] == '\n' || strings.HasPrefix(s[j:], "\r\n")
		}
		return j >= len(s) || strings.HasPrefix(s[j:], rowSep)
	}
	eol := func(j int) int {
		if auto {
			if strings.HasPrefix(s[j:], "\r\n") {
				return j + 2
			}
			return j + 1
		}
		return j + len(rowSep)
	}
	for i < len(s) {
		var row []*String
		if atEOL(i) {
			if !skipBlanks {
				rows = append(rows, row)
			}
			i = eol(i)
			line++
			continue
		}
		for {
			if strings.HasPrefix(s[i:], quote) {
				start := line
				var b strings.Builder
				j := i + len(quote)
				for {
					if j >= len(s) {
						return nil, fmt.Sprintf("Unclosed quoted field in line %d.", start)
					}
					if strings.HasPrefix(s[j:], quote) {
						if strings.HasPrefix(s[j+len(quote):], quote) {
							b.WriteString(quote)
							j += 2 * len(quote)
							continue
						}
						j += len(quote)
						break
					}
					if s[j] == '\n' {
						line++
					}
					b.WriteByte(s[j])
					j++
				}
				if !atEOL(j) && !strings.HasPrefix(s[j:], sep) {
					return nil, fmt.Sprintf("Illegal quoting in line %d.", line)
				}
				row = append(row, Ref(String(b.String())))
				i = j
			} else {
				j := i
				for !atEOL(j) && !strings.HasPrefix(s[j:], sep) {
					if strings.HasPrefix(s[j:], quote) {
						return nil, fmt.Sprintf("Illegal quoting in line %d.", line)
					}
					j++
				}
				if j == i {
					row = append(row, nil)
				} else {
					row = append(row, Ref(String(s[i:j])))
				}
				i = j
			}
			if atEOL(i) {
				break
			}
			i += len(sep)
		}
		rows = append(rows, row)
		if i < len(s) {
			i = eol(i)
		}
		line++
	}
	return rows, ""
}

// rbCSVLine is one generated row: nil is empty (or "" if force), fields holding the separator, quote, a line break, or force are quoted with the quote doubled.
func rbCSVLine(row []any, sep, quote, rowSep string, force bool) string {
	var b strings.Builder
	for i, v := range row {
		if i > 0 {
			b.WriteString(sep)
		}
		if v == nil {
			if force {
				b.WriteString(quote + quote)
			}
			continue
		}
		f := string(rbToS(v))
		_, isStr := v.(String)
		if force || isStr && f == "" || strings.Contains(f, sep) || strings.Contains(f, quote) || strings.ContainsAny(f, "\r\n") {
			f = quote + strings.ReplaceAll(f, quote, quote+quote) + quote
		}
		b.WriteString(f)
	}
	b.WriteString(rowSep)
	return b.String()
}
