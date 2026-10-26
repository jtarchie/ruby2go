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
	"buffer_in": 1, "buffer_out": 1, "buffer_inout": 1,
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

// ffiType resolves a literal type Symbol through the module's typedefs to its Fiddle code.
func (c *Compiler) ffiType(f *File, cls *Class, at parser.Node, n parser.Node) int {
	s, ok := n.(*parser.SymbolNode)
	if !ok {
		c.errorf(f, at, "FFI::Library#attach_function needs literal Symbol types, read at compile time (decision 375)")
	}
	name := s.Unescaped.Value
	for seen := 0; seen < 32; seen++ {
		next, ok := c.ffiAliases[cls][name]
		if !ok {
			break
		}
		name = next
	}
	if name == "varargs" {
		c.errorf(f, at, "FFI::Library#attach_function: a varargs C function cannot be called (purego has no C varargs; decision 326)")
	}
	code, ok := ffiTypes[name]
	if !ok {
		c.errorf(f, at, "FFI::Library#attach_function: unknown type :%s", name)
	}
	return code
}

// collectFFICall reads a literal typedef/attach_function in a Library module; the call itself still runs at load.
func (c *Compiler) collectFFICall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) {
	if n.Name != "attach_function" && n.Name != "typedef" || f.isPrelude() || !ffiLibrary(f, cls) {
		return
	}
	args := callArgs(n)
	if c.ffiAliases == nil {
		c.ffiAliases = map[*Class]map[string]string{}
	}
	if c.ffiAliases[cls] == nil {
		c.ffiAliases[cls] = map[string]string{}
	}
	sym := func(i int) string {
		s, ok := args[i].(*parser.SymbolNode)
		if !ok {
			c.errorf(f, n, "FFI::Library#%s needs literal Symbols, read at compile time (decision 375)", n.Name)
		}
		return s.Unescaped.Value
	}
	if n.Name == "typedef" {
		if len(args) == 2 {
			c.ffiAliases[cls][sym(1)] = sym(0)
		}
		return
	}
	if len(args) < 3 {
		c.errorf(f, n, "FFI::Library#attach_function needs a name, argument types and a return type")
	}
	name, rest := sym(0), args[1:]
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
	for i, el := range arr.Elements {
		c.ffiType(f, cls, n, el)
		params = append(params, fmt.Sprintf("a%d", i))
		types = append(types, "untyped")
	}
	if len(params) > 8 {
		c.errorf(f, n, "FFI::Library#attach_function: more than 8 arguments are stack-passed (decision 326)")
	}
	ret := c.ffiType(f, cls, n, rest[1])
	if c.fiddleSlots == nil {
		c.fiddleSlots = map[*Class]int{}
	}
	fn := fmt.Sprintf("__ffi_func_at(%d, %q)", c.fiddleSlots[cls], name)
	c.fiddleSlots[cls]++
	call := fmt.Sprintf("%s.call(%s)", fn, strings.Join(params, ", "))
	switch n := len(params); {
	case n >= 1 && n <= 3:
		call = fmt.Sprintf("::Fiddle.__call%d(%s)", n, strings.Join(append([]string{fn}, params...), ", "))
	case n <= 8:
		call = fmt.Sprintf("::Fiddle.__call(%s)", strings.Join(append([]string{fn, strconv.Itoa(n)}, params...), ", "))
	}
	src := fmt.Sprintf("#: (%s) -> %s\ndef self.%s(%s) = %s\n",
		strings.Join(types, ", "), ffiRubyType(ret), name, strings.Join(params, ", "), call)
	sf, def := c.synthDef(ctx, f, n, "FFI function", src, synthOpt{Origin: f.synthOrigin(), NoPath: true})
	c.addMethod(sf, cls, def, false, scope)
}
