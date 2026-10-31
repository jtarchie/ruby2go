package compiler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// MRI's attach_function defines a method at run time; it is synthesized from the literal types instead (decision 375).

// ffiTypes are prelude/ffi.rb's TYPES as Fiddle TYPE_* codes (LP64).
var ffiTypes = map[string]int{
	"void": 0, "bool": 11, "char": 2, "uchar": -2, "int8": 2, "uint8": -2, "short": 3, "ushort": -3, "int16": 3, "uint16": -3,
	"int": 4, "uint": -4, "int32": 4, "uint32": -4, "long": 5, "ulong": -5, "int64": 5, "uint64": -5, "long_long": 5, "ulong_long": -5,
	"size_t": -5, "ssize_t": 5, "float": 7, "double": 8, "string": 10, "pointer": 1,
	"buffer_in": 1, "buffer_out": 1, "buffer_inout": 1, "varargs": 9,
}

// ffiRubyType is the Ruby type a call returns for a Fiddle TYPE_* code.
func ffiRubyType(code int) string {
	if code == 10 {
		return "String?"
	}
	return fiddleRubyType(code)
}

// ffiLibrary reports a module that extends FFI::Library.
func ffiLibrary(f *File, cls *Class) bool {
	for _, e := range cls.extends {
		if strings.TrimPrefix(constText(f, e.ref.node), "FFI::") == "Library" {
			return true
		}
	}
	return false
}

// ffiParam is one literal type: its Fiddle code and, for a callback or enum, the kind and the type's name.
type ffiParam struct {
	code int
	kind string
	name string
}

// ffiType resolves a literal type (a Symbol through the module's typedefs, or Struct.by_ref) to its Fiddle code.
func (c *Compiler) ffiType(f *File, cls *Class, at parser.Node, n parser.Node) ffiParam {
	if call, ok := n.(*parser.CallNode); ok && call.Receiver != nil {
		switch call.Name {
		case "by_ref":
			return ffiParam{code: 1, kind: "ptr"}
		case "by_value":
			c.errorf(f, at, "FFI::Library#attach_function: a struct by value is not supported (decision 375)")
		}
	}
	s, ok := n.(*parser.SymbolNode)
	if !ok {
		c.errorf(f, at, "FFI::Library#attach_function needs literal Symbol types, read at compile time (decision 375)")
	}
	name := s.Unescaped.Value
	for range 32 {
		next, ok := c.ffiAliases[cls][name]
		if !ok {
			break
		}
		name = next
	}
	switch c.ffiNamed[cls][name] {
	case "callback":
		return ffiParam{code: 1, kind: "callback", name: name}
	case "enum":
		return ffiParam{code: 4, kind: "enum", name: name}
	}
	if name == "varargs" {
		if !c.env.CGO {
			c.errorf(f, at, "FFI::Library#attach_function: a varargs C function needs --cgo (purego has no C varargs; decisions 326, 426)")
		}
		return ffiParam{code: 9, kind: "varargs"}
	}
	code, ok := ffiTypes[name]
	if !ok {
		c.errorf(f, at, "FFI::Library#attach_function: unknown type :%s", name)
	}
	if code == 1 {
		return ffiParam{code: 1, kind: "ptr"}
	}
	return ffiParam{code: code}
}

// collectFFICall reads a literal typedef/attach_function in a Library module; the call itself still runs at load.
func (c *Compiler) collectFFICall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) {
	if n.Name != "attach_function" && n.Name != "typedef" && n.Name != "callback" && n.Name != "enum" || f.isPrelude() || !ffiLibrary(f, cls) {
		return
	}
	args := callArgs(n)
	if c.ffiAliases == nil {
		c.ffiAliases = map[*Class]map[string]string{}
	}
	if c.ffiAliases[cls] == nil {
		c.ffiAliases[cls] = map[string]string{}
	}
	if c.ffiNamed == nil {
		c.ffiNamed = map[*Class]map[string]string{}
	}
	if c.ffiNamed[cls] == nil {
		c.ffiNamed[cls] = map[string]string{}
	}
	sym := func(i int) string {
		s, ok := args[i].(*parser.SymbolNode)
		if !ok {
			c.errorf(f, n, "FFI::Library#%s needs literal Symbols, read at compile time (decision 375)", n.Name)
		}
		return s.Unescaped.Value
	}
	if n.Name == "callback" || n.Name == "enum" {
		if len(args) >= 1 {
			if s, ok := args[0].(*parser.SymbolNode); ok {
				c.ffiNamed[cls][s.Unescaped.Value] = n.Name
			}
		}
		return
	}
	if n.Name == "typedef" {
		if len(args) == 2 {
			if call, ok := args[0].(*parser.CallNode); ok && call.Name == "by_ref" && call.Receiver != nil {
				c.ffiAliases[cls][sym(1)] = "pointer" // a struct pointer typedef is a plain pointer
			} else {
				c.ffiAliases[cls][sym(1)] = sym(0)
			}
		}
		return
	}
	c.attachFFI(ctx, f, cls, n, scope)
}

// attachFFI synthesizes the typed module method for a literal attach_function.
func (c *Compiler) attachFFI(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) {
	args := callArgs(n)
	if len(args) < 3 {
		c.errorf(f, n, "FFI::Library#attach_function needs a name, argument types and a return type")
	}
	nameSym, ok := args[0].(*parser.SymbolNode)
	if !ok {
		c.errorf(f, n, "FFI::Library#attach_function needs literal Symbols, read at compile time (decision 375)")
	}
	name, rest := nameSym.Unescaped.Value, args[1:]
	switch rest[0].(type) {
	case *parser.SymbolNode, *parser.StringNode: // attach_function :ruby_name, :c_name, [...], ret
		rest = rest[1:]
	}
	if len(rest) < 2 {
		c.errorf(f, n, "FFI::Library#attach_function needs argument types and a return type")
	}
	arr, ok := rest[0].(*parser.ArrayNode)
	if !ok {
		c.errorf(f, n, "FFI::Library#attach_function needs a literal Array of argument types (decision 375)")
	}
	params := make([]string, 0, len(arr.Elements))
	types := make([]string, 0, len(arr.Elements))
	exprs := make([]string, 0, len(arr.Elements))
	variadic := false
	for i, el := range arr.Elements {
		p := fmt.Sprintf("a%d", i)
		arg := p
		switch t := c.ffiType(f, cls, n, el); t.kind {
		case "varargs":
			if i != len(arr.Elements)-1 {
				c.errorf(f, n, "FFI::Library#attach_function: :varargs must be the last argument type")
			}
			variadic = true
			continue
		case "ptr":
			arg = fmt.Sprintf("::FFI.__ptr(%s)", p)
		case "callback":
			arg = fmt.Sprintf("__ffi_cb(%s, %s)", ":"+t.name, p)
		case "enum":
			arg = fmt.Sprintf("__ffi_enum_in(%s, %s)", ":"+t.name, p)
		}
		params = append(params, p)
		exprs = append(exprs, arg)
		types = append(types, "untyped")
	}
	if len(params) > 8 && !c.env.CGO {
		c.errorf(f, n, "FFI::Library#attach_function: more than 8 arguments are stack-passed and need --cgo (decisions 326, 426)")
	}
	rp := c.ffiType(f, cls, n, rest[1])
	ret := rp.code
	if c.fiddleSlots == nil {
		c.fiddleSlots = map[*Class]int{}
	}
	fn := fmt.Sprintf("__ffi_func_at(%d, %q)", c.fiddleSlots[cls], name)
	c.fiddleSlots[cls]++
	call := fmt.Sprintf("::Fiddle.__call(%s)", strings.Join(append([]string{fn, strconv.Itoa(len(params))}, exprs...), ", "))
	switch n := len(params); {
	case variadic: // type and value pairs after the fixed arguments (decision 426)
		call = fmt.Sprintf("%s.call(%s)", fn, strings.Join(append(exprs, "*::FFI.__varargs(rest)"), ", "))
		params, types = append(params, "*rest"), append(types, "*untyped")
	case n > 8:
		call = fmt.Sprintf("%s.call(%s)", fn, strings.Join(exprs, ", "))
	case n >= 1 && n <= 3:
		call = fmt.Sprintf("::Fiddle.__call%d(%s)", n, strings.Join(append([]string{fn}, exprs...), ", "))
	}
	retType := ffiRubyType(ret)
	switch rp.kind {
	case "ptr":
		call, retType = "::FFI::Pointer.__wrap("+call+")", "FFI::Pointer"
	case "enum":
		call, retType = fmt.Sprintf("__ffi_enum_out(:%s, %s)", rp.name, call), "untyped"
	case "callback":
		c.errorf(f, n, "FFI::Library#attach_function: a callback cannot be a return type (decision 375)")
	}
	src := fmt.Sprintf("#: (%s) -> %s\ndef self.%s(%s) = %s\n",
		strings.Join(types, ", "), retType, name, strings.Join(params, ", "), call)
	sf, def := c.synthDef(ctx, f, n, "FFI function", src, synthOpt{Origin: f.synthOrigin(), NoPath: true})
	c.addMethod(sf, cls, def, false, scope)
	// The gem also defines a private instance method, for a class that extends Library and calls it from its own methods.
	inst := fmt.Sprintf("#: (%s) -> %s\ndef %s(%s) = ::%s.%s(%s)\n",
		strings.Join(types, ", "), retType, name, strings.Join(params, ", "), cls.RubyName, name, strings.Join(params, ", "))
	isf, idef := c.synthDef(ctx, f, n, "FFI function", inst, synthOpt{Origin: f.synthOrigin(), NoPath: true})
	c.addMethod(isf, cls, idef, true, scope)
}
