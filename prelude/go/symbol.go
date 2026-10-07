//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"regexp"
)

// MRI's identifier: an ASCII letter or `_`, or any non-ASCII character,
// then those or ASCII digits. \x60 is a backtick.
const rbIdentRe = `[A-Za-z_\x{80}-\x{10FFFF}][\w\x{80}-\x{10FFFF}]*`

var rbPlainSymbol = regexp.MustCompile(`\A(?:` + rbIdentRe + `[?!=]?|@@?` + rbIdentRe +
	`|\$(?:` + rbIdentRe + `|[~*$?!@/\\;,.=:<>"&\x60'+0]|-[\w\x{80}-\x{10FFFF}]|[1-9]\d*)` +
	`|\[\]=?|[+\-*/%<>!~^&|\x60]|\*\*|<=>|==|===|=~|!=|!~|<<|>>|<=|>=|[+\-]@)\z`)

func rbSymbolInspect(s string) String {
	if rbPlainSymbol.MatchString(s) {
		return String(":" + s)
	}
	return ":" + rbStringInspectAs(s, false)
}
