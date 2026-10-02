//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// The numeric prefix String#to_i / #to_f read, as MRI scans it: leading
// whitespace, a sign, digits with single `_` between them; to_i takes a
// `0d` prefix, to_f a fraction and exponent. The rest of the string is ignored.
var (
	rbIntPrefix   = regexp.MustCompile(`\A[ \t\n\v\f\r]*([+-]?)(?:0[dD])?(\d+(?:_\d+)*)`)
	rbFloatPrefix = regexp.MustCompile(`\A[ \t\n\v\f\r]*([+-]?(?:\d+(?:_\d+)*(?:\.(?:\d+(?:_\d+)*)?)?|\.\d+(?:_\d+)*)(?:[eE][+-]?\d+(?:_\d+)*)?)`)
)

// SpecialCasing.txt's unconditional mappings, which MRI applies and Go's
// per-rune unicode.To* cannot (one rune to several): lower, title, upper.
var rbSpecialCasing = map[rune][3]string{
	'ß': {"ß", "Ss", "SS"}, 'İ': {"i̇", "İ", "İ"}, 'ŉ': {"ŉ", "ʼN", "ʼN"},
	'ǰ': {"ǰ", "J̌", "J̌"}, 'ΐ': {"ΐ", "Ϊ́", "Ϊ́"}, 'ΰ': {"ΰ", "Ϋ́", "Ϋ́"},
	'և': {"և", "Եւ", "ԵՒ"}, 'ẖ': {"ẖ", "H̱", "H̱"}, 'ẗ': {"ẗ", "T̈", "T̈"},
	'ẘ': {"ẘ", "W̊", "W̊"}, 'ẙ': {"ẙ", "Y̊", "Y̊"}, 'ẚ': {"ẚ", "Aʾ", "Aʾ"},
	'ὐ': {"ὐ", "Υ̓", "Υ̓"}, 'ὒ': {"ὒ", "Υ̓̀", "Υ̓̀"}, 'ὔ': {"ὔ", "Υ̓́", "Υ̓́"},
	'ὖ': {"ὖ", "Υ̓͂", "Υ̓͂"}, 'ᾀ': {"ᾀ", "ᾈ", "ἈΙ"}, 'ᾁ': {"ᾁ", "ᾉ", "ἉΙ"},
	'ᾂ': {"ᾂ", "ᾊ", "ἊΙ"}, 'ᾃ': {"ᾃ", "ᾋ", "ἋΙ"}, 'ᾄ': {"ᾄ", "ᾌ", "ἌΙ"},
	'ᾅ': {"ᾅ", "ᾍ", "ἍΙ"}, 'ᾆ': {"ᾆ", "ᾎ", "ἎΙ"}, 'ᾇ': {"ᾇ", "ᾏ", "ἏΙ"},
	'ᾈ': {"ᾀ", "ᾈ", "ἈΙ"}, 'ᾉ': {"ᾁ", "ᾉ", "ἉΙ"}, 'ᾊ': {"ᾂ", "ᾊ", "ἊΙ"},
	'ᾋ': {"ᾃ", "ᾋ", "ἋΙ"}, 'ᾌ': {"ᾄ", "ᾌ", "ἌΙ"}, 'ᾍ': {"ᾅ", "ᾍ", "ἍΙ"},
	'ᾎ': {"ᾆ", "ᾎ", "ἎΙ"}, 'ᾏ': {"ᾇ", "ᾏ", "ἏΙ"}, 'ᾐ': {"ᾐ", "ᾘ", "ἨΙ"},
	'ᾑ': {"ᾑ", "ᾙ", "ἩΙ"}, 'ᾒ': {"ᾒ", "ᾚ", "ἪΙ"}, 'ᾓ': {"ᾓ", "ᾛ", "ἫΙ"},
	'ᾔ': {"ᾔ", "ᾜ", "ἬΙ"}, 'ᾕ': {"ᾕ", "ᾝ", "ἭΙ"}, 'ᾖ': {"ᾖ", "ᾞ", "ἮΙ"},
	'ᾗ': {"ᾗ", "ᾟ", "ἯΙ"}, 'ᾘ': {"ᾐ", "ᾘ", "ἨΙ"}, 'ᾙ': {"ᾑ", "ᾙ", "ἩΙ"},
	'ᾚ': {"ᾒ", "ᾚ", "ἪΙ"}, 'ᾛ': {"ᾓ", "ᾛ", "ἫΙ"}, 'ᾜ': {"ᾔ", "ᾜ", "ἬΙ"},
	'ᾝ': {"ᾕ", "ᾝ", "ἭΙ"}, 'ᾞ': {"ᾖ", "ᾞ", "ἮΙ"}, 'ᾟ': {"ᾗ", "ᾟ", "ἯΙ"},
	'ᾠ': {"ᾠ", "ᾨ", "ὨΙ"}, 'ᾡ': {"ᾡ", "ᾩ", "ὩΙ"}, 'ᾢ': {"ᾢ", "ᾪ", "ὪΙ"},
	'ᾣ': {"ᾣ", "ᾫ", "ὫΙ"}, 'ᾤ': {"ᾤ", "ᾬ", "ὬΙ"}, 'ᾥ': {"ᾥ", "ᾭ", "ὭΙ"},
	'ᾦ': {"ᾦ", "ᾮ", "ὮΙ"}, 'ᾧ': {"ᾧ", "ᾯ", "ὯΙ"}, 'ᾨ': {"ᾠ", "ᾨ", "ὨΙ"},
	'ᾩ': {"ᾡ", "ᾩ", "ὩΙ"}, 'ᾪ': {"ᾢ", "ᾪ", "ὪΙ"}, 'ᾫ': {"ᾣ", "ᾫ", "ὫΙ"},
	'ᾬ': {"ᾤ", "ᾬ", "ὬΙ"}, 'ᾭ': {"ᾥ", "ᾭ", "ὭΙ"}, 'ᾮ': {"ᾦ", "ᾮ", "ὮΙ"},
	'ᾯ': {"ᾧ", "ᾯ", "ὯΙ"}, 'ᾲ': {"ᾲ", "Ὰͅ", "ᾺΙ"}, 'ᾳ': {"ᾳ", "ᾼ", "ΑΙ"},
	'ᾴ': {"ᾴ", "Άͅ", "ΆΙ"}, 'ᾶ': {"ᾶ", "Α͂", "Α͂"}, 'ᾷ': {"ᾷ", "ᾼ͂", "Α͂Ι"},
	'ᾼ': {"ᾳ", "ᾼ", "ΑΙ"}, 'ῂ': {"ῂ", "Ὴͅ", "ῊΙ"}, 'ῃ': {"ῃ", "ῌ", "ΗΙ"},
	'ῄ': {"ῄ", "Ήͅ", "ΉΙ"}, 'ῆ': {"ῆ", "Η͂", "Η͂"}, 'ῇ': {"ῇ", "ῌ͂", "Η͂Ι"},
	'ῌ': {"ῃ", "ῌ", "ΗΙ"}, 'ῒ': {"ῒ", "Ϊ̀", "Ϊ̀"}, 'ΐ': {"ΐ", "Ϊ́", "Ϊ́"},
	'ῖ': {"ῖ", "Ι͂", "Ι͂"}, 'ῗ': {"ῗ", "Ϊ͂", "Ϊ͂"}, 'ῢ': {"ῢ", "Ϋ̀", "Ϋ̀"},
	'ΰ': {"ΰ", "Ϋ́", "Ϋ́"}, 'ῤ': {"ῤ", "Ρ̓", "Ρ̓"}, 'ῦ': {"ῦ", "Υ͂", "Υ͂"},
	'ῧ': {"ῧ", "Ϋ͂", "Ϋ͂"}, 'ῲ': {"ῲ", "Ὼͅ", "ῺΙ"}, 'ῳ': {"ῳ", "ῼ", "ΩΙ"},
	'ῴ': {"ῴ", "Ώͅ", "ΏΙ"}, 'ῶ': {"ῶ", "Ω͂", "Ω͂"}, 'ῷ': {"ῷ", "ῼ͂", "Ω͂Ι"},
	'ῼ': {"ῳ", "ῼ", "ΩΙ"}, 'ﬀ': {"ﬀ", "Ff", "FF"}, 'ﬁ': {"ﬁ", "Fi", "FI"},
	'ﬂ': {"ﬂ", "Fl", "FL"}, 'ﬃ': {"ﬃ", "Ffi", "FFI"}, 'ﬄ': {"ﬄ", "Ffl", "FFL"},
	'ﬅ': {"ﬅ", "St", "ST"}, 'ﬆ': {"ﬆ", "St", "ST"}, 'ﬓ': {"ﬓ", "Մն", "ՄՆ"},
	'ﬔ': {"ﬔ", "Մե", "ՄԵ"}, 'ﬕ': {"ﬕ", "Մի", "ՄԻ"}, 'ﬖ': {"ﬖ", "Վն", "ՎՆ"},
	'ﬗ': {"ﬗ", "Մխ", "ՄԽ"},
}

// rbCaseMap is MRI's full case mapping: column i of rbSpecialCasing, else f.
func rbCaseMap(s string, i int, f func(rune) rune) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'ß' { // the lowest key: ASCII skips the lookup
			if m, ok := rbSpecialCasing[r]; ok {
				b.WriteString(m[i])
				continue
			}
		}
		b.WriteRune(f(r))
	}
	return b.String()
}

// rbSub replaces the first n (all if n < 0) occurrences of pat, expanding
// MRI's backslash sequences in rep: \0 \& match, \` \' pre/post-match,
// \\ a backslash; \1-\9 and \+ are empty (a String pattern has no groups).
// ponytail: \k<name> stays literal; MRI raises IndexError.
func rbSub(s, pat, rep string, n int) string {
	if !strings.Contains(rep, "\\") {
		return strings.Replace(s, pat, rep, n)
	}
	var b strings.Builder
	done, pos := 0, 0
	for ; n != 0; n-- {
		i := strings.Index(s[pos:], pat)
		if i < 0 {
			break
		}
		m, e := pos+i, pos+i+len(pat)
		b.WriteString(s[done:m])
		for j := 0; j < len(rep); j++ {
			if rep[j] != '\\' || j+1 == len(rep) {
				b.WriteByte(rep[j])
				continue
			}
			j++
			switch rep[j] {
			case '0', '&':
				b.WriteString(pat)
			case '`':
				b.WriteString(s[:m])
			case '\'':
				b.WriteString(s[e:])
			case '\\':
				b.WriteByte('\\')
			case '1', '2', '3', '4', '5', '6', '7', '8', '9', '+':
			default:
				b.WriteString(rep[j-1 : j+1])
			}
		}
		done, pos = e, e
		if pat == "" { // step past a rune so an empty pattern matches between each
			if pos == len(s) {
				break
			}
			_, w := utf8.DecodeRuneInString(s[pos:])
			pos += w
		}
	}
	b.WriteString(s[done:])
	return b.String()
}

// rbPad is n runes of pad repeated, as String#ljust/rjust/center fill.
func rbPad(pad string, n int) string {
	if pad == "" {
		panic(NewArgumentError(Ref(String("zero width padding"))))
	}
	r := []rune(pad)
	out := make([]rune, n)
	for i := range out {
		out[i] = r[i%len(r)]
	}
	return string(out)
}

// rbCharSet is a tr/count/squeeze character set: ranges (a-z), \-escapes,
// and a leading ^ to negate.
func rbCharSet(spec string) func(rune) bool {
	rs := []rune(spec)
	negate := len(rs) > 1 && rs[0] == '^'
	if negate {
		rs = rs[1:]
	}
	set := map[rune]bool{}
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			i++
			set[rs[i]] = true
		case i+2 < len(rs) && rs[i+1] == '-':
			for r := rs[i]; r <= rs[i+2]; r++ {
				set[r] = true
			}
			i += 2
		default:
			set[rs[i]] = true
		}
	}
	return func(r rune) bool { return set[r] != negate }
}

// rbSqueeze collapses runs of one repeated character that in accepts.
func rbSqueeze(s string, in func(rune) bool) string {
	var b strings.Builder
	prev := rune(-1)
	for _, r := range s {
		if r == prev && in(r) {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// rbStrToIBase is String#to_i(base) (base -8 is #oct: octal unless a
// prefix names another base). Leading whitespace, a sign and single
// underscores between digits are allowed; parsing stops at the first
// invalid digit.
func rbStrToIBase(s string, base int) int {
	if base == 1 || base > 36 || base < -8 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(base)))))
	}
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg, s = s[0] == '-', s[1:]
	}
	prefixes := map[string]int{"0b": 2, "0B": 2, "0o": 8, "0O": 8, "0x": 16, "0X": 16, "0d": 10, "0D": 10}
	if len(s) >= 2 {
		if pb, ok := prefixes[s[:2]]; ok && (base <= 0 || base == pb) {
			base, s = pb, s[2:]
		}
	}
	switch {
	case base == 0 && strings.HasPrefix(s, "0"):
		base = 8
	case base == 0:
		base = 10
	case base < 0:
		base = -base
	}
	n := 0
	for i := range len(s) {
		c := s[i]
		if c == '_' && i > 0 && i+1 < len(s) && s[i-1] != '_' {
			continue
		}
		d := 99
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		}
		if d >= base {
			break
		}
		n = n*base + d
	}
	if neg {
		return -n
	}
	return n
}

// rbSplitLimit is String#split(sep, limit); sep nil or " " is awk mode.
func rbSplitLimit(s string, sep *String, limit int) []string {
	awk := sep == nil || *sep == " "
	var parts []string
	switch {
	case limit == 1:
		if s == "" {
			return nil
		}
		return []string{s}
	case awk:
		rest := strings.TrimLeft(s, " \t\n\v\f\r")
		for rest != "" {
			if limit > 0 && len(parts) == limit-1 {
				parts = append(parts, rest)
				break
			}
			i := strings.IndexAny(rest, " \t\n\v\f\r")
			if i < 0 {
				parts = append(parts, rest)
				break
			}
			parts = append(parts, rest[:i])
			rest = strings.TrimLeft(rest[i:], " \t\n\v\f\r")
		}
		if limit < 0 && len(s) > 0 && strings.ContainsAny(s[len(s)-1:], " \t\n\v\f\r") {
			parts = append(parts, "")
		}
		return parts
	case limit > 0:
		parts = strings.SplitN(s, string(*sep), limit)
	default:
		parts = strings.Split(s, string(*sep))
	}
	if limit == 0 {
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	if s == "" {
		return nil
	}
	return parts
}

// rbStrDump is String#dump: inspect's escapes, with every non-ASCII character as \u and invalid bytes as \x.
func rbStrDump(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			fmt.Fprintf(&b, "\\x%02X", s[i])
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '#' && i+1 < len(s) && strings.IndexByte("{$@", s[i+1]) >= 0:
			b.WriteString("\\#")
		case r >= 0x20 && r < 0x7f:
			b.WriteRune(r)
		case r < 0x80:
			if e, ok := map[rune]string{'\n': "n", '\t': "t", '\r': "r", '\f': "f", '\v': "v", '\b': "b", '\a': "a", 0x1b: "e"}[r]; ok {
				b.WriteString("\\" + e)
			} else {
				fmt.Fprintf(&b, "\\x%02X", r)
			}
		case r > 0xffff:
			fmt.Fprintf(&b, "\\u{%X}", r)
		default:
			fmt.Fprintf(&b, "\\u%04X", r)
		}
		i += n
	}
	b.WriteByte('"')
	return b.String()
}

// rbUnpack is String#unpack for the common directives: a A Z (bytes as a
// String), C c S s L l Q q n N v V (integers; S/L/Q native little-endian,
// with < and > for an explicit order), U (UTF-8 codepoints), H h (hex),
// B b (bits), m (base64) and x/@ (skip, seek), each with a count or *. Integers past the end
// are nil, as MRI's.
func rbUnpack(s, format string) *Array[any] {
	out := &Array[any]{}
	pos := 0
	for i := 0; i < len(format); {
		d := format[i]
		i++
		if d == ' ' || d == '\t' || d == '\n' {
			continue
		}
		var order byte
		for i < len(format) && strings.IndexByte("<>_!", format[i]) >= 0 {
			if format[i] == '<' || format[i] == '>' {
				order = format[i]
			}
			i++
		}
		count, star := 1, false
		switch {
		case i < len(format) && format[i] == '*':
			star = true
			i++
		case i < len(format) && format[i] >= '0' && format[i] <= '9':
			j := i
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			count, _ = strconv.Atoi(format[i:j])
			i = j
		}
		rest := s[min(pos, len(s)):]
		switch d {
		case 'a', 'A', 'Z':
			n := count
			if star || n > len(rest) {
				n = len(rest)
			}
			v := rest[:n]
			switch {
			case d == 'Z' && star:
				if k := strings.IndexByte(rest, 0); k >= 0 {
					v, n = rest[:k], k+1
				}
			case d == 'Z':
				if k := strings.IndexByte(v, 0); k >= 0 {
					v = v[:k]
				}
			case d == 'A':
				v = strings.TrimRight(v, " \x00")
			}
			*out = append(*out, String(v))
			pos += n
		case 'x':
			if pos += count; pos > len(s) {
				panic(NewArgumentError(Ref(String("x outside of string"))))
			}
		case '@':
			pos = count
		case 'U':
			for n := 0; (star || n < count) && pos < len(s); n++ {
				r, size := utf8.DecodeRuneInString(s[pos:])
				*out = append(*out, Integer(r))
				pos += size
			}
		case 'H', 'h', 'B', 'b':
			per := map[byte]int{'H': 2, 'h': 2, 'B': 8, 'b': 8}[d]
			n := count
			if star || n > len(rest)*per {
				n = len(rest) * per
			}
			var b strings.Builder
			for k := 0; k < n; k++ {
				c := rest[k/per]
				switch d {
				case 'H':
					b.WriteByte("0123456789abcdef"[c>>(4*(1-k%2))&0xf])
				case 'h':
					b.WriteByte("0123456789abcdef"[c>>(4*(k%2))&0xf])
				case 'B':
					b.WriteByte('0' + c>>(7-k%8)&1)
				case 'b':
					b.WriteByte('0' + c>>(k%8)&1)
				}
			}
			*out = append(*out, String(b.String()))
			pos += (n + per - 1) / per
		case 'm':
			clean := strings.Map(func(r rune) rune {
				if strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", r) {
					return r
				}
				return -1
			}, rest)
			b, _ := base64.RawStdEncoding.DecodeString(clean)
			*out = append(*out, String(b))
			pos = len(s)
		default:
			size, signed, big := 0, strings.IndexByte("csqlj", d) >= 0, order == '>'
			switch d {
			case 'C', 'c':
				size = 1
			case 'S', 's', 'n', 'v':
				size, big = 2, big || d == 'n'
			case 'L', 'l', 'N', 'V':
				size, big = 4, big || d == 'N'
			case 'Q', 'q', 'J', 'j':
				size = 8
			default:
				panic(NewArgumentError(Ref(String("rb2go: unpack directive '" + string(d) + "' is not supported"))))
			}
			for n := 0; star && pos+size <= len(s) || !star && n < count; n++ {
				if pos+size > len(s) {
					*out = append(*out, nil)
					continue
				}
				var u uint64
				for k := range size {
					b := uint64(s[pos+k])
					if big {
						u = u<<8 | b
					} else {
						u |= b << (8 * k)
					}
				}
				v := Integer(u)
				if signed && size < 8 && u&(1<<(8*size-1)) != 0 {
					v -= Integer(1) << (8 * size)
				}
				*out = append(*out, v)
				pos += size
			}
		}
	}
	return out
}
