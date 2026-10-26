//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"golang.org/x/text/encoding/charmap"
)

// rbSingleByte are the single-byte encodings x/text's charmap holds, by MRI's name (decision 380). Each table is
// checked byte by byte and character by character against MRI's transcoder.
var rbSingleByte = map[string]*charmap.Charmap{
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
	"IBM855": charmap.CodePage855, "IBM860": charmap.CodePage860, "IBM862": charmap.CodePage862,
	"IBM863": charmap.CodePage863, "IBM865": charmap.CodePage865, "IBM866": charmap.CodePage866,
	"macRoman": charmap.Macintosh, "macCyrillic": charmap.MacintoshCyrillic,
}

// rbXDecode1 is rbDecode1 for an x/text encoding; ok is false for any other.
func rbXDecode1(enc string, b []byte) (r rune, n, st, again int, ok bool) {
	if cm := rbSingleByte[enc]; cm != nil {
		r, fixed := rbSBDecodeFix[enc][b[0]]
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
	return 0, 0, 0, 0, false
}

// rbXEncode1 is rbEncode1 for an x/text encoding; known is false for any other.
func rbXEncode1(out []byte, enc string, r rune) (o []byte, defined, known bool) {
	if cm := rbSingleByte[enc]; cm != nil {
		if c, fixed := rbSBEncodeFix[enc][r]; fixed {
			return append(out, c...), c != "", true
		}
		c, ok := cm.EncodeRune(r)
		if !ok {
			return out, false, true
		}
		return append(out, c), true, true
	}
	return out, false, false
}
