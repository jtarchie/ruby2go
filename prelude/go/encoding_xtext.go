//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// rbSingleByte are the single-byte encodings x/text's charmap holds, by MRI's name (decision 380). Each table is
// checked byte by byte and character by character against MRI's transcoder.
// rbSingleByteTables is the table set rbUseEncSingleByte links; rbSingleByteOf reads it.
func rbSingleByteTables() map[string]rbSBCodec {
	return map[string]rbSBCodec{
		"ISO-8859-2": charmap.ISO8859_2, "ISO-8859-3": charmap.ISO8859_3, "ISO-8859-4": charmap.ISO8859_4,
		"ISO-8859-5": charmap.ISO8859_5, "ISO-8859-6": charmap.ISO8859_6, "ISO-8859-7": charmap.ISO8859_7,
		"ISO-8859-8": charmap.ISO8859_8, "ISO-8859-9": charmap.ISO8859_9, "ISO-8859-10": charmap.ISO8859_10,
		"ISO-8859-13": charmap.ISO8859_13, "ISO-8859-14": charmap.ISO8859_14, "ISO-8859-15": charmap.ISO8859_15,
		"ISO-8859-16":  charmap.ISO8859_16,
		"Windows-1250": charmap.Windows1250, "Windows-1251": charmap.Windows1251, "Windows-1252": charmap.Windows1252,
		"Windows-1253": charmap.Windows1253, "Windows-1254": charmap.Windows1254, "Windows-1255": charmap.Windows1255,
		"Windows-1256": charmap.Windows1256, "Windows-1257": charmap.Windows1257, "Windows-874": charmap.Windows874,
		"KOI8-R": charmap.KOI8R, "KOI8-U": charmap.KOI8U,
		"IBM437": charmap.CodePage437, "CP850": charmap.CodePage850, "IBM852": charmap.CodePage852,
		"IBM855": charmap.CodePage855, "CP852": charmap.CodePage852, "CP855": charmap.CodePage855,
		"ISO-8859-11": charmap.Windows874, "TIS-620": charmap.Windows874, "IBM860": charmap.CodePage860, "IBM862": charmap.CodePage862,
		"IBM863": charmap.CodePage863, "IBM865": charmap.CodePage865, "IBM866": charmap.CodePage866,
		"macRoman": charmap.Macintosh, "macCyrillic": charmap.MacintoshCyrillic,
	}
}

// rbSB is the linked single-byte tables and MRI's corrections to them, nil until rbUseEncSingleByte runs.
var rbSB atomic.Pointer[rbSBTables]

// rbSBCodec is what rugaroo asks of a single-byte table; kept code names it, not charmap's type, so a program that
// transcodes only UTF-16 does not import charmap, whose init builds every table.
type rbSBCodec interface {
	DecodeByte(b byte) rune
	EncodeRune(r rune) (byte, bool)
}

type rbSBTables struct {
	cm     map[string]rbSBCodec
	decFix map[string]map[byte]rune
	encFix map[string]map[rune]string
}

// rbUseEncSingleByte links the single-byte encodings' tables; the compiler calls it where a program names one.
func rbUseEncSingleByte() {
	if rbSB.Load() == nil {
		rbSB.CompareAndSwap(nil, &rbSBTables{rbSingleByteTables(), rbSBDecodeFix, rbSBEncodeFix})
	}
}

// rbSingleByteOf is enc's table, nil for an encoding that is not one of them.
func rbSingleByteOf(enc string) (rbSBCodec, *rbSBTables) {
	if !rbIsSingleByte[enc] {
		return nil, nil
	}
	t := rbSB.Load()
	if t == nil {
		panic("rugaroo: " + enc + "'s tables were not linked into this program (the compiler links an encoding it sees named)")
	}
	return t.cm[enc], t
}

// rbXDecode1 is rbDecode1 for an x/text encoding; ok is false for any other.
func rbXDecode1(enc string, b []byte) (r rune, n, st, again int, ok bool) {
	if cm, t := rbSingleByteOf(enc); cm != nil {
		r, fixed := t.decFix[enc][b[0]]
		if !fixed {
			if r = cm.DecodeByte(b[0]); r == 0xFFFD { // charmap's mark for a byte the table leaves undefined
				r = -1
			}
		}
		if r < 0 {
			return 0, 1, rbDecUndef, 0, true
		}
		return r, 1, rbDecOK, 0, true
	}
	if m := rbMBOf(enc); m != nil {
		r, n, st, again = m.decode1(b)
		return r, n, st, again, true
	}
	return 0, 0, 0, 0, false
}

// rbXEncode1 is rbEncode1 for an x/text encoding; known is false for any other.
func rbXEncode1(out []byte, enc string, r rune) (o []byte, defined, known bool) {
	if cm, t := rbSingleByteOf(enc); cm != nil {
		if c, fixed := t.encFix[enc][r]; fixed {
			return append(out, c...), c != "", true
		}
		c, ok := cm.EncodeRune(r)
		if !ok {
			return out, false, true
		}
		return append(out, c), true, true
	}
	if m := rbMBOf(enc); m != nil {
		o, defined = m.encode1(out, r)
		return o, defined, true
	}
	return out, false, false
}

// rbMBEnc is a multibyte encoding x/text holds (decision 380): MRI's byte structure (which bytes stand alone, lead,
// trail, and the forms EUC-JP's 0x8F and GB18030's four bytes take), x/text's table, and MRI's corrections to it as
// runs (scripts/gen-encoding-fixes, expanded on first use).
type rbMBEnc struct {
	x                            encoding.Encoding
	form                         int // 2, 3 (EUC-JP's 0x8F x1 x2) or 4 (GB18030's lead x1 x2 x3)
	one, lead, trail, x1, x2, x3 [32]byte
	decRuns, encRuns             string
	fixOnce                      sync.Once
	dec                          map[string]rune
	enc                          map[rune]string
	decoders, encoders           sync.Pool
}

// rbMBFamilies hold each family's encodings once its rbUseEnc func has linked them.
var rbMBFamilies [4]atomic.Pointer[map[string]*rbMBEnc]

// rbMBOf is enc's multibyte tables, nil for an encoding that is not one.
func rbMBOf(enc string) *rbMBEnc {
	f, ok := rbMBFamilyOf[enc]
	if !ok {
		return nil
	}
	m := rbMBFamilies[f].Load()
	if m == nil {
		panic("rugaroo: " + enc + "'s tables were not linked into this program (the compiler links an encoding it sees named)")
	}
	return (*m)[enc]
}

func rbBit(set *[32]byte, b byte) bool { return set[b/8]&(1<<(b%8)) != 0 }

// decode1 is MRI's transcoder walk: a byte that cannot continue the character makes the bytes before it invalid (and is
// read again), the input ending inside one is incomplete, and a whole character x/text and MRI have no mapping for is
// undefined.
func (m *rbMBEnc) decode1(b []byte) (r rune, n, st, again int) {
	switch {
	case rbBit(&m.one, b[0]):
		return m.char(b[:1])
	case !rbBit(&m.lead, b[0]):
		return 0, 1, rbDecInvalid, 0
	case len(b) < 2:
		return 0, 1, rbDecIncomplete, 0
	}
	var sets []*[32]byte
	switch {
	case m.form == 3 && b[0] == 0x8F:
		sets = []*[32]byte{&m.x1, &m.x2}
	case m.form == 4 && rbBit(&m.x1, b[1]):
		sets = []*[32]byte{&m.x1, &m.x2, &m.x3}
	default:
		sets = []*[32]byte{&m.trail}
	}
	for i, set := range sets {
		k := i + 1
		if k >= len(b) {
			return 0, k, rbDecIncomplete, 0
		}
		if !rbBit(set, b[k]) {
			return 0, k, rbDecInvalid, 1
		}
	}
	return m.char(b[:len(sets)+1])
}

// char is the character seq (whole by MRI's structure) stands for: MRI's correction, else x/text's.
func (m *rbMBEnc) char(seq []byte) (rune, int, int, int) {
	m.fixOnce.Do(m.expand)
	if r, ok := m.dec[string(seq)]; ok {
		if r < 0 {
			return 0, len(seq), rbDecUndef, 0
		}
		return r, len(seq), rbDecOK, 0
	}
	d, _ := m.decoders.Get().(*encoding.Decoder)
	if d == nil {
		d = m.x.NewDecoder()
	}
	defer m.decoders.Put(d)
	d.Reset()
	var buf [16]byte
	nd, ns, err := d.Transform(buf[:], seq, true)
	if r, size := utf8.DecodeRune(buf[:nd]); err == nil && ns == len(seq) && size == nd && r != utf8.RuneError {
		return r, len(seq), rbDecOK, 0
	}
	return 0, len(seq), rbDecUndef, 0
}

// encode1 is r's bytes: MRI's correction, else x/text's where they decode back to r here (x/text's tables are often a
// superset: CP949's for EUC-KR, GBK's for GB2312).
func (m *rbMBEnc) encode1(out []byte, r rune) ([]byte, bool) {
	m.fixOnce.Do(m.expand)
	if s, ok := m.enc[r]; ok {
		return append(out, s...), s != ""
	}
	e, _ := m.encoders.Get().(*encoding.Encoder)
	if e == nil {
		e = m.x.NewEncoder()
	}
	defer m.encoders.Put(e)
	e.Reset()
	var src [4]byte
	var dst [8]byte
	ns := utf8.EncodeRune(src[:], r)
	nd, _, err := e.Transform(dst[:], src[:ns], true)
	if err != nil || nd == 0 {
		return out, false
	}
	if back, n, st, _ := m.decode1(dst[:nd]); st != rbDecOK || n != nd || back != r {
		return out, false
	}
	return append(out, dst[:nd]...), true
}

// expand reads the generated runs: "SEQ=CP*N" (CP "-": undefined) and "CP=SEQ*N" (SEQ "-": cannot encode), each run
// N consecutive characters and sequences in structural order (rbMBNext).
func (m *rbMBEnc) expand() {
	m.dec, m.enc = map[string]rune{}, map[rune]string{}
	for run := range strings.SplitSeq(m.decRuns, ";") {
		k, v, ok := strings.Cut(run, "=")
		if !ok {
			continue
		}
		v, cnt, _ := strings.Cut(v, "*")
		n, _ := strconv.Atoi(cnt)
		seq := rbHexSeq(k)
		cp := rune(-1)
		if v != "-" {
			c, _ := strconv.ParseInt(v, 16, 32)
			cp = rune(c)
		}
		for i := range n {
			if cp >= 0 {
				m.dec[string(seq)] = cp + rune(i)
			} else {
				m.dec[string(seq)] = -1
			}
			seq = m.next(seq)
		}
	}
	for run := range strings.SplitSeq(m.encRuns, ";") {
		k, v, ok := strings.Cut(run, "=")
		if !ok {
			continue
		}
		v, cnt, _ := strings.Cut(v, "*")
		n, _ := strconv.Atoi(cnt)
		c, _ := strconv.ParseInt(k, 16, 32)
		var seq []byte
		if v != "-" {
			seq = rbHexSeq(v)
		}
		for i := range n {
			m.enc[rune(c)+rune(i)] = string(seq)
			if seq != nil {
				seq = m.next(seq)
			}
		}
	}
}

// next is seq's structural successor in its own form, as gen-encoding-fixes' runs count them.
func (m *rbMBEnc) next(seq []byte) []byte {
	b := append([]byte(nil), seq...)
	var sets []*[32]byte
	switch len(b) {
	case 1:
		sets = []*[32]byte{&m.one}
	case 2:
		sets = []*[32]byte{&m.lead, &m.trail}
	case 3:
		sets = []*[32]byte{nil, &m.x1, &m.x2}
	default:
		sets = []*[32]byte{&m.lead, &m.x1, &m.x2, &m.x3}
	}
	for i := len(b) - 1; i >= 0; i-- {
		if sets[i] == nil {
			return nil
		}
		for c := int(b[i]) + 1; c < 256; c++ {
			if rbBit(sets[i], byte(c)) {
				b[i] = byte(c)
				return b
			}
		}
		for c := range 256 {
			if rbBit(sets[i], byte(c)) {
				b[i] = byte(c)
				break
			}
		}
	}
	return nil
}

func rbHexSeq(s string) []byte {
	var b []byte
	for x := range strings.SplitSeq(s, ".") {
		n, _ := strconv.ParseUint(x, 16, 8)
		b = append(b, byte(n))
	}
	return b
}
