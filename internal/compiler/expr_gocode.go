package compiler

import (
	"regexp"
	"strings"
)

// pinnable is false for Go code whose value can't change: literals and nil.
func pinnable(code string) bool {
	switch {
	case code == "" || code == "nil" || code == "true" || code == "false":
		return false
	case code[0] == '"' || code[0] == '`' || code[0] >= '0' && code[0] <= '9':
		return false
	case code[0] == '-' && len(code) > 1 && code[1] >= '0' && code[1] <= '9':
		return false
	}
	return true
}

// valueOf runs a nil-typed call as a statement: `-> nil` methods have no Go result (isVoid).
func (f *fctx) valueOf(e expr) expr {
	if !isNil(e.typ) || e.done {
		return e
	}
	c := e.code
	for wrapped(c) {
		c = c[1 : len(c)-1]
	}
	if !strings.HasSuffix(c, ")") { // nil itself, or a temp holding it
		return e
	}
	f.emit("%s", c)
	return expr{code: "nil", typ: TNil{}}
}

// wrapped reports whether c's first "(" closes at its last ")", so the
// pair can go: `(f(x))` is, `(*File).Close(f)` is not.
func wrapped(c string) bool {
	if !strings.HasPrefix(c, "(") || !strings.HasSuffix(c, ")") {
		return false
	}
	depth := 0
	for i := 0; i < len(c); i++ {
		switch c[i] {
		case '"', '`', '\'': // skip a literal: its parens don't count
			q := c[i]
			for i++; i < len(c) && c[i] != q; i++ {
				if c[i] == '\\' && q != '`' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(c)-1
			}
		}
	}
	return false
}

// fitsValue is fits plus the one literal conversion Go makes exactly: an
// Integer literal where a Float is expected (Float's operators take
// Integers in Ruby). Any other literal of the wrong class, such as "a" for
// a Symbol, would convert silently too, so it is rejected.
func fitsValue(e expr, to Type) bool {
	return fits(e.typ, to) || e.lit && isClass(e.typ, "Integer") && isClass(stripOpt(to), "Float")
}

// materialize is e's code as a typed Go value: an untyped constant bound
// with := or used as a receiver would otherwise become int/string/bool.
func (f *fctx) materialize(e expr) string {
	if e.lit {
		return f.c.goType(e.typ) + "(" + e.code + ")"
	}
	return e.code
}

var simpleGo = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|\(\*[A-Za-z_][A-Za-z0-9_]*\)|[0-9.]+|"(?:[^"\\]|\\.)*")$`)

// isSimpleGo reports whether code can be evaluated twice without effects.
func isSimpleGo(code string) bool { return simpleGo.MatchString(code) }

// pinSimple returns code itself when it can be evaluated twice, else the temp it was just stored in.
func (f *fctx) pinSimple(code string) string {
	if !isSimpleGo(code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, code)
		code = tmp
	}
	return code
}

// discard evaluates e for its effects where a fold drops its value. It
// discards simple code too: readLocal already counted a local's read, so
// noteUnused will not, and Go rejects the unused variable.
func (f *fctx) discard(e expr) {
	if e.code != "" && e.code != "nil" && !e.lit {
		f.emit("_ = %s", e.code)
	}
}
