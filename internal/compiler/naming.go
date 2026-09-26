package compiler

import (
	"strings"
	"unicode"
)

// Operator name table (README open decision 3). Must be injective.
var opNames = map[string]string{
	"==": "Eq", "!=": "Ne", "<=>": "Cmp", "<": "Lt", "<=": "Le", ">": "Gt", ">=": "Ge",
	"+": "Plus", "-": "Minus", "*": "Mul", "/": "Div", "%": "Mod", "**": "Pow",
	"-@": "Neg", "+@": "Pos", "!": "Not", "~": "Inv", "<<": "Shl", ">>": "Shr",
	"&": "BitAnd", "|": "BitOr", "^": "BitXor", "=~": "EqTilde", "!~": "NotTilde", "===": "Eqq",
	"[]": "Idx", "[]=": "IdxSet",
}

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
	for _, part := range strings.Split(name[i:], "_") {
		if part == "" {
			b.WriteByte('_')
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
