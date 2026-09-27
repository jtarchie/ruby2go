package compiler

import (
	"strings"
	"unicode"
)

// Operator name table (README open decision 3): `_` + lowercase never comes out of camel-casing, so `plus`, Integer#div, IO#pos cannot meet an operator.
var opNames = map[string]string{
	"==": "Op_eq", "!=": "Op_ne", "<=>": "Op_cmp", "<": "Op_lt", "<=": "Op_le", ">": "Op_gt", ">=": "Op_ge",
	"+": "Op_plus", "-": "Op_minus", "*": "Op_mul", "/": "Op_div", "%": "Op_mod", "**": "Op_pow",
	"-@": "Op_neg", "+@": "Op_pos", "!": "Op_not", "~": "Op_inv", "<<": "Op_shl", ">>": "Op_shr",
	"&": "Op_bitAnd", "|": "Op_bitOr", "^": "Op_bitXor", "=~": "Op_eqTilde", "!~": "Op_notTilde", "===": "Op_eqq",
	"[]": "Op_idx", "[]=": "Op_idxSet", "`": "Op_backtick",
}

var suffixWords = map[string]bool{"q": true, "bang": true, "set": true}

// goMethodName maps a Ruby method name to an exported Go identifier.
// `end_with?` → EndWithQ, `upcase!` → UpcaseBang, `x=` → XSet, `__write` → __Write.
func goMethodName(name string) string {
	if n, ok := opNames[name]; ok {
		return n
	}
	suffix := ""
	switch {
	case strings.HasSuffix(name, "?"):
		suffix = "Q"
		name = name[:len(name)-1]
	case strings.HasSuffix(name, "!"):
		suffix = "Bang"
		name = name[:len(name)-1]
	case strings.HasSuffix(name, "="):
		suffix = "Set"
		name = name[:len(name)-1]
	}
	var b strings.Builder
	// Leading underscores are kept so private helpers stay distinct.
	i := 0
	for i < len(name) && name[i] == '_' {
		b.WriteByte('_')
		i++
	}
	parts := strings.Split(name[i:], "_")
	for j, part := range parts {
		if part == "" {
			b.WriteByte('_')
			continue
		}
		// `empty_q` → Empty_q, so it cannot meet `empty?` → EmptyQ
		if j > 0 && j == len(parts)-1 && suffix == "" && suffixWords[part] {
			b.WriteString("_" + part)
			continue
		}
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	b.WriteString(suffix)
	return b.String()
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
	// predeclared identifiers we'd rather not shadow
	"len": true, "cap": true, "new": true, "make": true, "append": true, "copy": true,
	"panic": true, "recover": true, "print": true, "println": true, "string": true,
	"int": true, "bool": true, "any": true, "error": true, "nil": true, "true": true, "false": true,
	"main": true, "init": true, "stdout": true, "self": false,
}

// goLocalName maps a Ruby local/param name to a Go identifier.
func goLocalName(name string) string {
	if goKeywords[name] {
		return name + "_"
	}
	return name
}

// goFieldName maps an ivar (`@x`) to a struct field name.
func goFieldName(ivar string) string {
	return goLocalName(strings.TrimPrefix(ivar, "@"))
}

// goFuncName maps a top-level Ruby def to a Go function name.
func goFuncName(name string) string {
	return "rb_" + goMethodName(name)
}
