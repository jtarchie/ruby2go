package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// multiByte are the multibyte encodings x/text holds, by MRI's name: their x/text table, the family a program links
// (rbUseEnc<family>), and the longest form ("3": EUC-JP's 0x8F, "4": GB18030's four bytes).
var multiByte = map[string]struct {
	enc    encoding.Encoding
	family string
	form   string
}{
	"Shift_JIS": {japanese.ShiftJIS, "Japanese", "2"}, "Windows-31J": {japanese.ShiftJIS, "Japanese", "2"},
	"EUC-JP": {japanese.EUCJP, "Japanese", "3"}, "CP51932": {japanese.EUCJP, "Japanese", "2"},
	"EUC-KR": {korean.EUCKR, "Korean", "2"}, "CP949": {korean.EUCKR, "Korean", "2"},
	"GBK": {simplifiedchinese.GBK, "SimplifiedChinese", "2"}, "GB2312": {simplifiedchinese.GBK, "SimplifiedChinese", "2"},
	"GB18030": {simplifiedchinese.GB18030, "SimplifiedChinese", "4"},
	"Big5":    {traditionalchinese.Big5, "TraditionalChinese", "2"}, "CP950": {traditionalchinese.Big5, "TraditionalChinese", "2"},
}

// mbSpec is an encoding's byte structure as MRI's transcoder checks it: which bytes stand alone, start two bytes and
// may follow a lead, and (form 3) what follows 0x8F or (form 4) the four-byte form's bytes.
type mbSpec struct {
	one, lead, trail, x1, x2, x3 [256]bool
	form                         string
}

type mbResult struct {
	kind  string // ok, undef, inv, inc
	cps   []rune
	n     int // bytes taken as the character or the error
	again int
}

// predict is the decoder the runtime implements (rbMBDecode1), for the generator to check against MRI's answers.
func (s *mbSpec) predict(b []byte) mbResult {
	c := func(n int) mbResult { return mbResult{kind: "char", n: n} }
	inv := func(n, again int) mbResult { return mbResult{kind: "inv", n: n, again: again} }
	inc := func(n int) mbResult { return mbResult{kind: "inc", n: n} }
	switch {
	case s.one[b[0]]:
		return c(1)
	case !s.lead[b[0]]:
		return inv(1, 0)
	case len(b) < 2:
		return inc(1)
	}
	switch {
	case s.form == "3" && b[0] == 0x8F: // EUC-JP's JIS X 0212: 0x8F and two more
		switch {
		case !s.x1[b[1]]:
			return inv(1, 1)
		case len(b) < 3:
			return inc(2)
		case !s.x2[b[2]]:
			return inv(2, 1)
		}
		return c(3)
	case s.form == "4" && s.x1[b[1]]: // GB18030's four bytes: a lead, 30-39, 81-FE, 30-39
		switch {
		case len(b) < 3:
			return inc(2)
		case !s.x2[b[2]]:
			return inv(2, 1)
		case len(b) < 4:
			return inc(3)
		case !s.x3[b[3]]:
			return inv(3, 1)
		}
		return c(4)
	}
	if !s.trail[b[1]] {
		return inv(1, 1)
	}
	return c(2)
}

func parseHex(s string) []byte {
	if s == "" {
		return nil
	}
	var b []byte
	for _, x := range strings.Split(s, ".") {
		n, _ := strconv.ParseUint(x, 16, 8)
		b = append(b, byte(n))
	}
	return b
}

func parseCPs(s string) []rune {
	var r []rune
	for _, x := range strings.Split(s, ".") {
		n, _ := strconv.ParseUint(x, 16, 32)
		r = append(r, rune(n))
	}
	return r
}

// xDecode is x/text's character for seq, nil where it has none.
func xDecode(e encoding.Encoding, seq []byte) []rune {
	out, err := e.NewDecoder().Bytes(seq)
	if err != nil || !utf8.Valid(out) || bytes.ContainsRune(out, utf8.RuneError) {
		return nil
	}
	return []rune(string(out))
}

func xEncode(e encoding.Encoding, r rune) []byte {
	out, err := e.NewEncoder().Bytes([]byte(string(r)))
	if err != nil {
		return nil
	}
	return out
}

type mbOut struct {
	name, family, form, conv string
	spec                     mbSpec
	decFix, encFix           []string // "SEQHEX=CPHEX" (CP -1: undefined), "CPHEX=BYTESHEX" ("": cannot encode)
}

func genMB(name string) mbOut {
	m := multiByte[name]
	cmd := exec.Command("ruby", "mbdump.rb", name, m.form)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	status := map[string]mbResult{}
	var order []string
	mriEnc := map[rune][]byte{}
	o := mbOut{name: name, family: m.family, form: m.form, conv: name}
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		switch f[0] {
		case "s":
			r := mbResult{kind: f[2]}
			switch r.kind {
			case "ok":
				r.cps = parseCPs(f[3])
			case "inv":
				r.n = len(parseHex(f[3]))
				if len(f) > 4 {
					r.again = len(parseHex(f[4]))
				}
			case "inc":
				r.n = len(parseHex(f[3]))
			}
			status[f[1]] = r
			order = append(order, f[1])
		case "e":
			n, _ := strconv.ParseUint(f[1], 16, 32)
			mriEnc[rune(n)] = parseHex(f[2])
		case "n":
			o.conv = f[1]
		}
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("ruby mbdump %s: %v", name, err)
	}
	h := func(b ...byte) string {
		var p []string
		for _, x := range b {
			p = append(p, strconv.FormatUint(uint64(x), 16))
		}
		return strings.Join(p, ".")
	}
	s := &o.spec
	s.form = m.form
	char := func(k string) bool { return k == "ok" || k == "undef" }
	for b := range 256 {
		r := status[h(byte(b))]
		s.one[b], s.lead[b] = char(r.kind), r.kind == "inc"
	}
	for b := range 256 {
		for l := 0x80; l < 256; l++ {
			if s.lead[l] && !(m.form == "3" && l == 0x8F) && char(status[h(byte(l), byte(b))].kind) {
				s.trail[b] = true
			}
		}
	}
	switch m.form {
	case "3":
		for b := 0xA1; b <= 0xFE; b++ {
			s.x1[b], s.x2[b] = true, true
		}
	case "4":
		for b := 0x30; b <= 0x39; b++ {
			s.x1[b], s.x3[b] = true, true
		}
		for b := 0x81; b <= 0xFE; b++ {
			s.x2[b] = true
		}
	}
	// the model must give MRI's answer for every input MRI was asked about
	bad := 0
	for _, k := range order {
		seq := parseHex(k)
		want, got := status[k], s.predict(seq)
		if got.kind == "char" && got.n < len(seq) { // two characters: MRI's answer is about both
			continue
		}
		ok := got.kind == "char" && char(want.kind) && got.n == len(seq) ||
			got.kind == want.kind && got.n == want.n && got.again == want.again
		if !ok {
			if bad < 5 {
				log.Printf("%s %s: MRI %+v, model %+v", name, k, want, got)
			}
			bad++
		}
	}
	if bad > 0 {
		log.Fatalf("%s: the byte model disagrees with MRI on %d inputs", name, bad)
	}
	for _, k := range order {
		want := status[k]
		if !char(want.kind) || s.predict(parseHex(k)).n != len(parseHex(k)) {
			continue
		}
		x := xDecode(m.enc, parseHex(k))
		switch {
		case want.kind == "undef" && x != nil:
			o.decFix = append(o.decFix, k+"=-1")
		case want.kind == "ok" && !slices.Equal(x, want.cps):
			if len(want.cps) != 1 {
				log.Fatalf("%s %s: MRI decodes to %d characters", name, k, len(want.cps))
			}
			o.decFix = append(o.decFix, k+"="+strconv.FormatInt(int64(want.cps[0]), 16))
		}
	}
	decFix := map[string]rune{}
	for _, f := range o.decFix {
		k, v, _ := strings.Cut(f, "=")
		n, _ := strconv.ParseInt(v, 16, 32)
		decFix[k] = rune(n)
	}
	// ourDecode is the runtime's decoder: the byte model, then a correction, then x/text
	ourDecode := func(b []byte) (rune, bool) {
		if len(b) == 0 {
			return 0, false
		}
		if p := s.predict(b); p.kind != "char" || p.n != len(b) {
			return 0, false
		}
		if r, ok := decFix[h(b...)]; ok {
			return r, r >= 0
		}
		x := xDecode(m.enc, b)
		return firstRune(x), len(x) == 1
	}
	for cp := rune(0); cp <= 0x10FFFF; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		// x/text's bytes count only if they decode back to cp here: its tables are often a superset (CP949 for EUC-KR)
		x := xEncode(m.enc, cp)
		if r, ok := ourDecode(x); x != nil && (!ok || r != cp) {
			x = nil
		}
		if want := mriEnc[cp]; !bytes.Equal(want, x) {
			o.encFix = append(o.encFix, strconv.FormatInt(int64(cp), 16)+"="+h(want...))
		}
	}
	log.Printf("%s: %d inputs agree; %d decode and %d encode corrections", name, len(order), len(o.decFix), len(o.encFix))
	return o
}

// bitset renders a [256]bool as Go source for a [32]byte.
func bitset(b *[256]bool) string {
	var out [32]byte
	for i, v := range b {
		if v {
			out[i/8] |= 1 << (i % 8)
		}
	}
	parts := make([]string, 32)
	for i, x := range out {
		parts[i] = fmt.Sprintf("0x%02X", x)
	}
	return "[32]byte{" + strings.Join(parts, ", ") + "}"
}

// mbProbe runs genMB alone (GEN_MB_PROBE=1): the corrections each encoding would need, nothing written.
func mbProbe(names []string) {
	for _, n := range names {
		genMB(n)
	}
}

func firstRune(r []rune) rune {
	if len(r) == 0 {
		return -1
	}
	return r[0]
}

// next is the structural successor of seq in its own form (same length), as the runtime's rbMBNext walks runs.
func (s *mbSpec) next(b []byte) []byte {
	b = slices.Clone(b)
	sets := []*[256]bool{&s.one}
	switch {
	case len(b) == 2:
		sets = []*[256]bool{&s.lead, &s.trail}
	case len(b) == 3:
		sets = []*[256]bool{nil, &s.x1, &s.x2}
	case len(b) == 4:
		sets = []*[256]bool{&s.lead, &s.x1, &s.x2, &s.x3}
	}
	for i := len(b) - 1; i >= 0; i-- {
		if sets[i] == nil {
			return nil // EUC-JP's 0x8F does not advance
		}
		for c := int(b[i]) + 1; c < 256; c++ {
			if sets[i][c] {
				b[i] = byte(c)
				return b
			}
		}
		for c := range 256 { // wrap this byte to its first allowed value and carry
			if sets[i][c] {
				b[i] = byte(c)
				break
			}
		}
	}
	return nil
}

// runs compresses "SEQ=CP" corrections in structural order into "SEQ=CP*N" runs (CP "-" undefined).
func (s *mbSpec) decRuns(fix []string) string {
	var out []string
	var start, prevSeq []byte
	var startCP, prevCP rune
	n := 0
	flush := func() {
		if n > 0 {
			v := "-"
			if startCP >= 0 {
				v = strconv.FormatInt(int64(startCP), 16)
			}
			out = append(out, hexb(start)+"="+v+"*"+strconv.Itoa(n))
		}
	}
	for _, f := range fix {
		k, v, _ := strings.Cut(f, "=")
		seq := parseHex(k)
		cp, _ := strconv.ParseInt(v, 16, 32)
		r := rune(cp)
		if n > 0 && bytes.Equal(s.next(prevSeq), seq) && (r < 0 && prevCP < 0 || r >= 0 && prevCP >= 0 && r == prevCP+1) {
			n++
		} else {
			flush()
			start, startCP, n = seq, r, 1
		}
		prevSeq, prevCP = seq, r
	}
	flush()
	return strings.Join(out, ";")
}

// encRuns compresses "CP=SEQ" corrections into "CP=SEQ*N" runs (SEQ "-" cannot encode).
func (s *mbSpec) encRuns(fix []string) string {
	var out []string
	var startCP, prevCP rune
	var start, prevSeq []byte
	n := 0
	flush := func() {
		if n > 0 {
			v := "-"
			if start != nil {
				v = hexb(start)
			}
			out = append(out, strconv.FormatInt(int64(startCP), 16)+"="+v+"*"+strconv.Itoa(n))
		}
	}
	for _, f := range fix {
		k, v, _ := strings.Cut(f, "=")
		c, _ := strconv.ParseInt(k, 16, 32)
		cp := rune(c)
		seq := parseHex(v)
		if n > 0 && cp == prevCP+1 && (seq == nil && prevSeq == nil || seq != nil && prevSeq != nil && len(seq) == len(prevSeq) && bytes.Equal(s.next(prevSeq), seq)) {
			n++
		} else {
			flush()
			startCP, start, n = cp, seq, 1
		}
		prevCP, prevSeq = cp, seq
	}
	flush()
	return strings.Join(out, ";")
}

func hexb(b []byte) string {
	var p []string
	for _, x := range b {
		p = append(p, strconv.FormatUint(uint64(x), 16))
	}
	return strings.Join(p, ".")
}

// families are the rbUseEnc<family> funcs, each linking one x/text package, in rbMBFamilies' order.
var families = []struct{ name, pkg, ident string }{
	{"Japanese", "golang.org/x/text/encoding/japanese", "japanese"},
	{"Korean", "golang.org/x/text/encoding/korean", "korean"},
	{"SimplifiedChinese", "golang.org/x/text/encoding/simplifiedchinese", "simplifiedchinese"},
	{"TraditionalChinese", "golang.org/x/text/encoding/traditionalchinese", "traditionalchinese"},
}

// xIdent is the x/text table's Go name in the generated file.
var xIdent = map[string]string{
	"Shift_JIS": "japanese.ShiftJIS", "Windows-31J": "japanese.ShiftJIS", "EUC-JP": "japanese.EUCJP", "CP51932": "japanese.EUCJP",
	"EUC-KR": "korean.EUCKR", "CP949": "korean.EUCKR", "GBK": "simplifiedchinese.GBK", "GB2312": "simplifiedchinese.GBK",
	"GB18030": "simplifiedchinese.GB18030", "Big5": "traditionalchinese.Big5", "CP950": "traditionalchinese.Big5",
}

// emitMB writes prelude/go/encoding_xtext_mb.go: per family, the func that links its tables into a program.
func emitMB(outs []mbOut) []string {
	var b strings.Builder
	b.WriteString("//go:build rugaroo_prelude\n\n// Code generated by scripts/gen-encoding-fixes from MRI " + rubyVersion() + " and x/text; DO NOT EDIT.\n\npackage prelude\n\nimport (\n")
	for _, f := range families {
		fmt.Fprintf(&b, "\t%q\n", f.pkg)
	}
	b.WriteString(")\n\n// rbMBFamilyOf is the family whose rbUseEnc func links an encoding's tables (decision 380).\nvar rbMBFamilyOf = map[string]int{\n")
	var conv []string
	for _, o := range outs {
		for i, f := range families {
			if f.name == o.family {
				fmt.Fprintf(&b, "%q: %d,\n", o.name, i)
			}
		}
		if o.conv != o.name {
			conv = append(conv, fmt.Sprintf("%q: %q,", o.name, o.conv))
		}
	}
	b.WriteString("}\n\n")
	for i, f := range families {
		fmt.Fprintf(&b, "// rbUseEnc%s links the %s encodings' tables; the compiler calls it where a program names one.\nfunc rbUseEnc%s() {\n\tif rbMBFamilies[%d].Load() != nil {\n\t\treturn\n\t}\n\tm := map[string]*rbMBEnc{\n", f.name, f.name, f.name, i)
		for _, o := range outs {
			if o.family != f.name {
				continue
			}
			s := &o.spec
			form := map[string]int{"2": 2, "3": 3, "4": 4}[o.form]
			fmt.Fprintf(&b, "%q: {x: %s, form: %d, one: %s, lead: %s, trail: %s, x1: %s, x2: %s, x3: %s,\ndecRuns: %q,\nencRuns: %q},\n",
				o.name, xIdent[o.name], form, bitset(&s.one), bitset(&s.lead), bitset(&s.trail), bitset(&s.x1), bitset(&s.x2), bitset(&s.x3), s.decRuns(o.decFix), s.encRuns(o.encFix))
		}
		fmt.Fprintf(&b, "\t}\n\trbMBFamilies[%d].CompareAndSwap(nil, &m)\n}\n\n", i)
	}
	b.WriteString("// rbUseEncAll links every family: an encoding named at run time could be any of them.\nfunc rbUseEncAll() {\n\trbUseEncSingleByte()\n")
	for _, f := range families {
		fmt.Fprintf(&b, "\trbUseEnc%s()\n", f.name)
	}
	b.WriteString("}\n")
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("gofmt mb: %v", err)
	}
	if err := os.WriteFile(root+"prelude/go/encoding_xtext_mb.go", src, 0o600); err != nil {
		log.Fatal(err)
	}
	return conv
}
