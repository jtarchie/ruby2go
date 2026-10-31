package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Under --cgo a by-value FFI::Struct goes through a C stub typed from the literal layout, so the C compiler does the ABI (decision 426).

// ffiLayout is one FFI::Struct or FFI::Union subclass's literal `layout`, read at compile time.
type ffiLayout struct {
	union  bool
	fields []ffiLayoutField
	ctype  string // its C typedef name, once declared in the preamble
}

// ffiLayoutField is one field: an FFI type Symbol or a nested by-value struct class, and an array length (0: a scalar).
type ffiLayoutField struct {
	sym    string
	nested string // a constant naming another Struct
	byRef  bool
	count  int
	at     parser.Node
}

// ffiStructKind reports whether cls subclasses FFI::Struct or FFI::Union directly, and which.
func ffiStructKind(cls *Class) (ok, union bool) {
	if cls.superRef == nil {
		return false, false
	}
	switch strings.TrimPrefix(constText(cls.superRef.file, cls.superRef.node), "FFI::") {
	case "Struct":
		return true, false
	case "Union":
		return true, true
	}
	return false, false
}

// collectFFILayout records a literal layout; a computed one only matters (as an error) once a by-value use needs it.
func (c *Compiler) collectFFILayout(f *File, cls *Class, n *parser.CallNode) {
	ok, union := ffiStructKind(cls)
	if !ok || n.Receiver != nil || f.isPrelude() {
		return
	}
	args := callArgs(n)
	l := &ffiLayout{union: union}
	for i := 0; i+1 < len(args); i += 2 {
		if _, isSym := args[i].(*parser.SymbolNode); !isSym {
			return
		}
		fl := ffiLayoutField{at: args[i+1]}
		t := args[i+1]
		if arr, isArr := t.(*parser.ArrayNode); isArr && len(arr.Elements) == 2 {
			n, isInt := arr.Elements[1].(*parser.IntegerNode)
			if !isInt {
				return
			}
			fl.count, _ = strconv.Atoi(f.text(n.GetLocation()))
			t = arr.Elements[0]
		}
		switch t := t.(type) {
		case *parser.SymbolNode:
			fl.sym = t.Unescaped.Value
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			fl.nested = constText(f, t)
		case *parser.CallNode:
			if t.Name != "by_ref" || t.Receiver == nil {
				return
			}
			fl.byRef = true
		default:
			return
		}
		if i+2 < len(args) {
			if _, explicit := args[i+2].(*parser.IntegerNode); explicit { // an explicit offset: C cannot say it
				return
			}
		}
		l.fields = append(l.fields, fl)
	}
	if c.ffiLayouts == nil {
		c.ffiLayouts = map[string]*ffiLayout{}
	}
	c.ffiLayouts[cls.RubyName] = l
}

// ffiStructNamed resolves a struct constant written inside owner to its recorded layout's Ruby name, inner scopes first.
func (c *Compiler) ffiStructNamed(owner, name string) (string, *ffiLayout) {
	for scope := owner; ; {
		full := name
		if scope != "" {
			full = scope + "::" + name
		}
		if l := c.ffiLayouts[full]; l != nil {
			return full, l
		}
		if scope == "" {
			return "", nil
		}
		k := strings.LastIndex(scope, "::")
		if k < 0 {
			scope = ""
		} else {
			scope = scope[:k]
		}
	}
}

// ffiCScalar is the C type of a Fiddle TYPE_* code (LP64).
var ffiCScalar = map[int]string{
	0: "void", 1: "void *", 2: "int8_t", -2: "uint8_t", 3: "int16_t", -3: "uint16_t", 4: "int32_t", -4: "uint32_t",
	5: "int64_t", -5: "uint64_t", 7: "float", 8: "double", 10: "const char *", 11: "_Bool",
}

// ffiStructC declares the C typedef for the struct named (in owner's scope) and returns its name.
func (c *Compiler) ffiStructC(f *File, at parser.Node, owner, name string, depth int) string {
	full, l := c.ffiStructNamed(owner, name)
	if l == nil {
		c.errorf(f, at, "FFI::Struct.by_value: %s needs a literal `layout` of Symbols, [type, n] arrays and nested Structs, without offsets (decision 426)", name)
	}
	if depth > 16 {
		c.errorf(f, at, "FFI::Struct.by_value: %s nests too deeply", name)
	}
	if l.ctype != "" {
		return l.ctype
	}
	var body strings.Builder
	for i, fl := range l.fields {
		ct := ""
		switch {
		case fl.byRef:
			ct = "void *"
		case fl.nested != "":
			ct = c.ffiStructC(f, fl.at, full, fl.nested, depth+1)
		default:
			code, ok := ffiTypes[fl.sym]
			if !ok || code == 0 || code == 9 {
				c.errorf(f, fl.at, "FFI::Struct.by_value: %s's field type :%s cannot be laid out in C", name, fl.sym)
			}
			ct = ffiCScalar[code]
		}
		dim := ""
		if fl.count > 0 {
			dim = "[" + strconv.Itoa(fl.count) + "]"
		}
		fmt.Fprintf(&body, " %s f%d%s;", ct, i, dim)
	}
	kind := "struct"
	if l.union {
		kind = "union"
	}
	l.ctype = fmt.Sprintf("rb_ffi_s%d", len(c.ffiStubTypes))
	c.ffiStubTypes = append(c.ffiStubTypes, fmt.Sprintf("typedef %s {%s } %s;", kind, body.String(), l.ctype))
	return l.ctype
}

// ffiStubParam is one argument or the result of a stub: a Fiddle code, or the C struct typedef it is by value.
type ffiStubParam struct {
	code  int
	ctype string // a struct by value
}

// ffiStub declares C stub n: arguments arrive as 64-bit words (a struct's address, a double's bits), the result via ret.
func (c *Compiler) ffiStub(params []ffiStubParam, ret ffiStubParam) int {
	n := len(c.ffiStubs)
	ctype := func(p ffiStubParam) string {
		if p.ctype != "" {
			return p.ctype
		}
		return ffiCScalar[p.code]
	}
	protos := make([]string, len(params))
	vals := make([]string, len(params))
	for k, p := range params {
		protos[k] = ctype(p)
		switch {
		case p.ctype != "":
			vals[k] = fmt.Sprintf("*(%s *)(uintptr_t)v[%d]", p.ctype, k)
		case p.code == 8:
			vals[k] = fmt.Sprintf("rb_ffi_d(v[%d])", k)
		case p.code == 7:
			vals[k] = fmt.Sprintf("rb_ffi_f(v[%d])", k)
		case p.code == 1 || p.code == 10:
			vals[k] = fmt.Sprintf("(%s)(uintptr_t)v[%d]", ffiCScalar[p.code], k)
		default:
			vals[k] = fmt.Sprintf("(%s)v[%d]", ffiCScalar[p.code], k)
		}
	}
	if len(protos) == 0 {
		protos = []string{"void"}
	}
	call := fmt.Sprintf("((%s (*)(%s))fn)(%s)", ctype(ret), strings.Join(protos, ", "), strings.Join(vals, ", "))
	body := call + "; *err = errno;"
	if ret.ctype != "" || ret.code != 0 {
		body = fmt.Sprintf("%s r = %s; *err = errno; memcpy(ret, &r, sizeof r);", ctype(ret), call)
	}
	c.ffiStubs = append(c.ffiStubs, fmt.Sprintf("static void rb_ffi_stub%d(uintptr_t fn, uint64_t *v, void *ret, int *err) { %s }", n, body))
	return n
}

// ffiStubPreamble is the stubs' C for the cgo preamble, kept only while rbFFIStub survives pruning (decision 426).
func (c *Compiler) ffiStubPreamble() string {
	if len(c.ffiStubs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("// rugaroo:if rbFFIStub\n#define _GNU_SOURCE\n#include <errno.h>\n#include <stdint.h>\n#include <string.h>\n#define RB_FFI_STUBS 1\n")
	b.WriteString("static double rb_ffi_d(uint64_t b) { double d; memcpy(&d, &b, sizeof d); return d; }\n")
	b.WriteString("static float rb_ffi_f(uint64_t b) { uint32_t x = (uint32_t)b; float f; memcpy(&f, &x, sizeof f); return f; }\n")
	for _, t := range c.ffiStubTypes {
		b.WriteString(t + "\n")
	}
	for _, s := range c.ffiStubs {
		b.WriteString(s + "\n")
	}
	b.WriteString("static void rb_ffi_stub(int n, uintptr_t fn, uint64_t *v, void *ret, int *err) {\n\tswitch (n) {\n")
	for k := range c.ffiStubs {
		fmt.Fprintf(&b, "\tcase %d: rb_ffi_stub%d(fn, v, ret, err); return;\n", k, k)
	}
	b.WriteString("\t}\n}\n// rugaroo:end\n")
	return b.String()
}

// ffiStubCall is an attach_function body through C stub n, for a signature with a struct by value: each argument as
// Fiddle would pass it (a struct as its address), a struct result copied into a new instance of its class.
func (c *Compiler) ffiStubCall(f *File, at parser.Node, cls *Class, fn string, rp ffiParam, params []ffiStubParam, exprs []string, variadic bool) string {
	if variadic {
		c.errorf(f, at, "FFI::Library#attach_function: a varargs function cannot take a struct by value")
	}
	ret := ffiStubParam{code: rp.code}
	if rp.kind == "byval" {
		ret.ctype = c.ffiStructC(f, at, cls.RubyName, rp.name, 0)
	}
	n := c.ffiStub(params, ret)
	codes := make([]string, len(params))
	vals := make([]string, len(params))
	for k, p := range params {
		codes[k], vals[k] = strconv.Itoa(p.code), exprs[k]
		if p.ctype == "" {
			vals[k] = fmt.Sprintf("::Fiddle.__arg_addr(%d, %s)", p.code, exprs[k])
		}
	}
	args := fmt.Sprintf("%d, %s.to_i, [%s], [%s]", n, fn, strings.Join(codes, ", "), strings.Join(vals, ", "))
	if rp.kind == "byval" {
		full, _ := c.ffiStructNamed(cls.RubyName, rp.name)
		return fmt.Sprintf("__s = ::%s.new\n  ::FFI.__stub(%s, __s.pointer.to_i, 0)\n  __s", full, args)
	}
	return fmt.Sprintf("::Fiddle.__ret(%d, ::FFI.__stub(%s, 0, %d))", rp.code, args, rp.code)
}
