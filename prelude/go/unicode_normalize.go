//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

type rbNormData struct {
	ccc            map[rune]uint8
	decomp, kompat map[rune][]rune
	comp           map[[2]rune]rune
}

// rbNormTables parses the generated tables once, on the first normalization, so programs that never normalize pay nothing at start-up.
var rbNormTables = sync.OnceValue(func() *rbNormData {
	runes := func(s string) []rune {
		var out []rune
		for f := range strings.FieldsSeq(s) {
			n, _ := strconv.ParseUint(f, 16, 32)
			out = append(out, rune(n))
		}
		return out
	}
	d := &rbNormData{ccc: map[rune]uint8{}, decomp: map[rune][]rune{}, kompat: map[rune][]rune{}, comp: map[[2]rune]rune{}}
	for e := range strings.SplitSeq(rbNormCCCData, ";") {
		k, v, _ := strings.Cut(e, "=")
		c, _ := strconv.Atoi(v)
		d.ccc[runes(k)[0]] = uint8(c)
	}
	for _, t := range []struct {
		data string
		into map[rune][]rune
	}{{rbNormDecompData, d.decomp}, {rbNormKompatData, d.kompat}} {
		for e := range strings.SplitSeq(t.data, ";") {
			k, v, _ := strings.Cut(e, "=")
			t.into[runes(k)[0]] = runes(v)
		}
	}
	for e := range strings.SplitSeq(rbNormCompData, ";") {
		k, v, _ := strings.Cut(e, "=")
		pair := runes(k)
		d.comp[[2]rune{pair[0], pair[1]}] = runes(v)[0]
	}
	return d
})

// Hangul syllables decompose and compose by arithmetic (Unicode ch. 3.12), not by table.
const (
	rbHangulS, rbHangulL, rbHangulV, rbHangulT = 0xAC00, 0x1100, 0x1161, 0x11A7
	rbHangulVCount, rbHangulTCount             = 21, 28
	rbHangulSCount                             = 19 * rbHangulVCount * rbHangulTCount
)

// rbNormDecompose appends r's full decomposition; the compatibility table's entries may themselves decompose, the canonical one's are already full.
func rbNormDecompose(out []rune, r rune, compat bool, d *rbNormData) []rune {
	if s := r - rbHangulS; s >= 0 && s < rbHangulSCount {
		out = append(out, rbHangulL+s/(rbHangulVCount*rbHangulTCount), rbHangulV+s%(rbHangulVCount*rbHangulTCount)/rbHangulTCount)
		if t := s % rbHangulTCount; t != 0 {
			out = append(out, rbHangulT+t)
		}
		return out
	}
	if compat {
		if k, ok := d.kompat[r]; ok {
			for _, c := range k {
				out = rbNormDecompose(out, c, true, d)
			}
			return out
		}
	}
	if c, ok := d.decomp[r]; ok {
		return append(out, c...)
	}
	return append(out, r)
}

// rbUnicodeNormalize is String#unicode_normalize: decompose, put combining marks in canonical order, then (for nfc/nfkc) compose.
func rbUnicodeNormalize(s, form string) string {
	if form != "nfc" && form != "nfd" && form != "nfkc" && form != "nfkd" {
		panic(NewArgumentError(Ref(String("Invalid normalization form " + form + "."))))
	}
	if !utf8.ValidString(s) {
		panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
	}
	ascii := true
	for i := range len(s) {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	d := rbNormTables()
	compat := form == "nfkc" || form == "nfkd"
	var rs []rune
	for _, r := range s {
		rs = rbNormDecompose(rs, r, compat, d)
	}
	for i := 0; i < len(rs); {
		if d.ccc[rs[i]] == 0 {
			i++
			continue
		}
		j := i
		for j < len(rs) && d.ccc[rs[j]] != 0 {
			j++
		}
		slices.SortStableFunc(rs[i:j], func(a, b rune) int { return cmp.Compare(d.ccc[a], d.ccc[b]) })
		i = j
	}
	if form == "nfc" || form == "nfkc" {
		rs = rbNormCompose(rs, d)
	}
	return string(rs)
}

// rbNormCompose is canonical composition: a mark joins the last starter unless a mark of the same or higher class sits between them.
func rbNormCompose(rs []rune, d *rbNormData) []rune {
	if len(rs) == 0 {
		return rs
	}
	out := []rune{rs[0]}
	starter := 0
	last := int(d.ccc[rs[0]])
	if last != 0 {
		last = 256 // a leading mark is no starter: nothing composes with it
	}
	for _, r := range rs[1:] {
		cc := int(d.ccc[r])
		if c, ok := rbNormPair(out[starter], r, d); ok && (last < cc || last == 0) {
			out[starter] = c
			continue
		}
		if cc == 0 {
			starter = len(out)
		}
		last = cc
		out = append(out, r)
	}
	return out
}

// rbNormPair composes a starter and the character after it, Hangul by arithmetic.
func rbNormPair(a, b rune, d *rbNormData) (rune, bool) {
	if l, v := a-rbHangulL, b-rbHangulV; l >= 0 && l < 19 && v >= 0 && v < rbHangulVCount {
		return rbHangulS + (l*rbHangulVCount+v)*rbHangulTCount, true
	}
	if s, t := a-rbHangulS, b-rbHangulT; s >= 0 && s < rbHangulSCount && s%rbHangulTCount == 0 && t > 0 && t < rbHangulTCount {
		return a + t, true
	}
	c, ok := d.comp[[2]rune{a, b}]
	return c, ok
}
