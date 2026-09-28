//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

func rbCSVOpt(opts *Hash[Symbol, String], key, def string) string {
	if v, ok := opts.vals[Symbol(key)]; ok {
		return string(v)
	}
	return def
}

// rbCSVParse splits s into rows of fields; err is MRI's MalformedCSVError message.
func rbCSVParse(s, sep, quote string) (rows [][]*String, err string) {
	line := 1
	i := 0
	atEOL := func(j int) bool { return j >= len(s) || s[j] == '\n' || strings.HasPrefix(s[j:], "\r\n") }
	eol := func(j int) int {
		if strings.HasPrefix(s[j:], "\r\n") {
			return j + 2
		}
		return j + 1
	}
	for i < len(s) {
		var row []*String
		if atEOL(i) {
			rows = append(rows, row)
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

// rbCSVLine is one generated row: nil is empty, "" and fields holding the
// separator, quote or a line break are quoted, with the quote doubled.
func rbCSVLine(row []any, sep, quote string) string {
	var b strings.Builder
	for i, v := range row {
		if i > 0 {
			b.WriteString(sep)
		}
		if v == nil {
			continue
		}
		f := string(rbToS(v))
		_, isStr := v.(String)
		if isStr && f == "" || strings.Contains(f, sep) || strings.Contains(f, quote) || strings.ContainsAny(f, "\r\n") {
			f = quote + strings.ReplaceAll(f, quote, quote+quote) + quote
		}
		b.WriteString(f)
	}
	b.WriteString("\n")
	return b.String()
}
