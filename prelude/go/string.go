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
