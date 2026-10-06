//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
//
// REXML's tokenizer (decision 114): text and attribute values are kept as
// written (entities intact), as REXML's tree parser keeps them raw, so
// writing a parsed document back reproduces its escapes. Each event is an
// Array: [:xmldecl, version, encoding, standalone], [:start, name,
// [[attr, raw value], ...]], [:end, name], [:text, raw], [:comment, s],
// [:cdata, s], [:pi, target, content]. Errors are REXML::ParseException
// with the first line of REXML's message.
package prelude

func rbRexmlFail(msg string) {
	panic(NewREXML_ParseException(Ref(String(msg))))
}

func rbRexmlEvent(parts ...any) any {
	return &Array[any]{s: parts}
}

func rbRexmlIsSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func rbRexmlNameChar(c byte, first bool) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_', c == ':', c >= 0x80:
		return true
	case !first && (c >= '0' && c <= '9' || c == '-' || c == '.'):
		return true
	}
	return false
}

type rbRexmlScanner struct {
	s   string
	pos int
}

func (sc *rbRexmlScanner) name() string {
	start := sc.pos
	for sc.pos < len(sc.s) && rbRexmlNameChar(sc.s[sc.pos], sc.pos == start) {
		sc.pos++
	}
	return sc.s[start:sc.pos]
}

func (sc *rbRexmlScanner) skipSpace() {
	for sc.pos < len(sc.s) && rbRexmlIsSpace(sc.s[sc.pos]) {
		sc.pos++
	}
}

// attrs reads `name="value"` pairs up to `>` or `/>` (or `?>` for a declaration).
func (sc *rbRexmlScanner) attrs(elem string) [][2]string {
	var out [][2]string
	seen := map[string]bool{}
	for {
		sc.skipSpace()
		if sc.pos >= len(sc.s) {
			rbRexmlFail("Missing end tag for '" + elem + "'")
		}
		c := sc.s[sc.pos]
		if c == '>' || c == '/' || c == '?' {
			return out
		}
		n := sc.name()
		if n == "" {
			rbRexmlFail("Malformed XML: Invalid attribute name in '" + elem + "'")
		}
		sc.skipSpace()
		if sc.pos >= len(sc.s) || sc.s[sc.pos] != '=' {
			rbRexmlFail("Missing attribute value for '" + n + "'")
		}
		sc.pos++
		sc.skipSpace()
		if sc.pos >= len(sc.s) || sc.s[sc.pos] != '"' && sc.s[sc.pos] != '\'' {
			rbRexmlFail("Missing attribute value quote for '" + n + "'")
		}
		q := sc.s[sc.pos]
		end := strings.IndexByte(sc.s[sc.pos+1:], q)
		if end < 0 {
			rbRexmlFail("Missing attribute value end quote: <" + n + ">")
		}
		v := sc.s[sc.pos+1 : sc.pos+1+end]
		sc.pos += end + 2
		if seen[n] {
			rbRexmlFail("Duplicate attribute \"" + n + "\"")
		}
		seen[n] = true
		out = append(out, [2]string{n, v})
	}
}

func rbRexmlTokens(src string) *Array[any] {
	sc := &rbRexmlScanner{s: src}
	events := &Array[any]{}
	var open []string
	rootDone := false
	for sc.pos < len(src) {
		if src[sc.pos] != '<' {
			end := strings.IndexByte(src[sc.pos:], '<')
			if end < 0 {
				end = len(src) - sc.pos
			}
			text := src[sc.pos : sc.pos+end]
			sc.pos += end
			blank := strings.TrimLeft(text, " \t\r\n") == ""
			switch {
			case len(open) > 0:
				events.s = append(events.s, rbRexmlEvent(Symbol("text"), String(text)))
			case !blank && !rootDone:
				rbRexmlFail("Malformed XML: Content at the start of the document (got '" + strings.TrimRight(text, " \t\r\n") + "')")
			case !blank:
				rbRexmlFail("Malformed XML: Extra content at the end of the document (got '" + strings.TrimRight(text, " \t\r\n") + "')")
			case !rootDone:
				events.s = append(events.s, rbRexmlEvent(Symbol("text"), String(text)))
			}
			continue
		}
		rest := src[sc.pos:]
		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := strings.Index(rest[4:], "-->")
			if end < 0 {
				rbRexmlFail("Unclosed comment")
			}
			events.s = append(events.s, rbRexmlEvent(Symbol("comment"), String(rest[4:4+end])))
			sc.pos += 4 + end + 3
		case strings.HasPrefix(rest, "<![CDATA["):
			end := strings.Index(rest[9:], "]]>")
			if end < 0 {
				rbRexmlFail("Unclosed CDATA")
			}
			if len(open) == 0 {
				rbRexmlFail("Malformed XML: CDATA outside of the root element")
			}
			events.s = append(events.s, rbRexmlEvent(Symbol("cdata"), String(rest[9:9+end])))
			sc.pos += 9 + end + 3
		case strings.HasPrefix(rest, "<!"):
			rbRexmlFail("rb2go: REXML DOCTYPE and DTD declarations are not supported")
		case strings.HasPrefix(rest, "<?"):
			end := strings.Index(rest, "?>")
			if end < 0 {
				rbRexmlFail("Malformed XML: Unclosed processing instruction")
			}
			sc.pos += 2
			target := sc.name()
			if target == "xml" {
				if len(events.s) > 0 {
					rbRexmlFail("Malformed XML: XML declaration is not at the start")
				}
				var version, encoding, standalone any
				for _, kv := range sc.attrs("xml") {
					switch kv[0] {
					case "version":
						version = String(kv[1])
					case "encoding":
						encoding = String(kv[1])
					case "standalone":
						standalone = String(kv[1])
					}
				}
				events.s = append(events.s, rbRexmlEvent(Symbol("xmldecl"), version, encoding, standalone))
				sc.pos = len(src) - len(rest) + end + 2
				continue
			}
			content := strings.TrimLeft(rest[2+len(target):end], " \t\r\n")
			var c any
			if content != "" {
				c = String(content)
			}
			events.s = append(events.s, rbRexmlEvent(Symbol("pi"), String(target), c))
			sc.pos = len(src) - len(rest) + end + 2
		case strings.HasPrefix(rest, "</"):
			sc.pos += 2
			n := sc.name()
			sc.skipSpace()
			if sc.pos >= len(src) || src[sc.pos] != '>' {
				rbRexmlFail("Missing end tag for '" + n + "'")
			}
			sc.pos++
			if len(open) == 0 {
				rbRexmlFail("Unexpected top-level end tag (got '" + n + "')")
			}
			if open[len(open)-1] != n {
				rbRexmlFail("Missing end tag for '" + open[len(open)-1] + "' (got '" + n + "')")
			}
			open = open[:len(open)-1]
			events.s = append(events.s, rbRexmlEvent(Symbol("end"), String(n)))
			if len(open) == 0 {
				rootDone = true
			}
		default:
			sc.pos++
			n := sc.name()
			if n == "" {
				rbRexmlFail("Malformed XML: Invalid tag name")
			}
			if len(open) == 0 && rootDone {
				panic(NewREXML_ParseException(Ref(String("#<RuntimeError: attempted adding second root element to document>"))))
			}
			pairs := sc.attrs(n)
			attrs := &Array[any]{s: make([]any, 0, len(pairs))}
			for _, kv := range pairs {
				attrs.s = append(attrs.s, rbRexmlEvent(String(kv[0]), String(kv[1])))
			}
			events.s = append(events.s, rbRexmlEvent(Symbol("start"), String(n), attrs))
			if strings.HasPrefix(src[sc.pos:], "/>") {
				sc.pos += 2
				events.s = append(events.s, rbRexmlEvent(Symbol("end"), String(n)))
				if len(open) == 0 {
					rootDone = true
				}
				continue
			}
			if sc.pos >= len(src) || src[sc.pos] != '>' {
				rbRexmlFail("Missing end tag for '" + n + "'")
			}
			sc.pos++
			open = append(open, n)
		}
	}
	if len(open) > 0 {
		rbRexmlFail("Missing end tag for '/" + open[len(open)-1] + "'")
	}
	return events
}

// rbRexmlUnnormalize is Text::unnormalize: numeric references and the five predefined entities; any other reference stays as written.
func rbRexmlUnnormalize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	if !strings.Contains(s, "&") {
		return s
	}
	return rbRexmlRef.ReplaceAllStringFunc(s, func(ref string) string {
		body := ref[1 : len(ref)-1]
		switch {
		case strings.HasPrefix(body, "#x"):
			if n, err := strconv.ParseUint(body[2:], 16, 32); err == nil {
				return string(rune(n))
			}
		case strings.HasPrefix(body, "#"):
			if n, err := strconv.ParseUint(body[1:], 10, 32); err == nil {
				return string(rune(n))
			}
		}
		switch body {
		case "amp":
			return "&"
		case "lt":
			return "<"
		case "gt":
			return ">"
		case "quot":
			return "\""
		case "apos":
			return "'"
		}
		return ref
	})
}

var rbRexmlRef = regexp.MustCompile(`&(?:#[0-9]+|#x[0-9a-fA-F]+|[A-Za-z_:][-A-Za-z0-9_:.]*);`)

// rbRexmlNormalize is Text::normalize without a DOCTYPE: & first, then the predefined entities.
func rbRexmlNormalize(s string) string {
	return strings.NewReplacer("&", "&amp;", ">", "&gt;", "<", "&lt;", "\"", "&quot;", "'", "&apos;").Replace(s)
}

// rbRexmlEmit writes a formatter's text to its output: $stdout (nil), an IO, a StringIO, or anything with write.
func rbRexmlEmit(out any, s string) {
	switch o := rbUnbox(out).(type) {
	case nil:
		rbWriteOut(s)
	case interface{ Write(any) Integer }:
		o.Write(String(s))
	case String:
		panic(NewTypeError(Ref(String("rb2go: REXML writes to an IO or StringIO; a String cannot be appended in place (use to_s)"))))
	default:
		panic(NewTypeError(Ref(String("rb2go: REXML cannot write to " + rbClassName(out)))))
	}
}
