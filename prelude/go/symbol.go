//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
	return ":" + rbStringInspect(s)
}
