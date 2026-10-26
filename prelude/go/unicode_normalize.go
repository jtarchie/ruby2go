//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// rbUnicodeNormalize is String#unicode_normalize, by x/text's norm (Unicode 17, as MRI 4.0's tables; decision 380).
func rbUnicodeNormalize(s, form string) string {
	var f norm.Form
	switch form {
	case "nfc":
		f = norm.NFC
	case "nfd":
		f = norm.NFD
	case "nfkc":
		f = norm.NFKC
	case "nfkd":
		f = norm.NFKD
	default:
		panic(NewArgumentError(Ref(String("Invalid normalization form " + form + "."))))
	}
	if !utf8.ValidString(s) {
		panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
	}
	return f.String(s)
}
