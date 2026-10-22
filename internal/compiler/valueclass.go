package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// valueKind is which of Struct.new / Data.define declared a value class.
type valueKind uint8

const (
	valueStruct valueKind = iota
	valueData
)

// String is the Ruby-facing name: it prints into errors and the generated inspect/pp text.
func (k valueKind) String() string {
	if k == valueData {
		return "data"
	}
	return "struct"
}

// superName is the core class a value class inherits from.
func (k valueKind) superName() string {
	if k == valueData {
		return "Data"
	}
	return "Struct"
}

// valueSpec is what a Struct.new / Data.define class declared.
type valueSpec struct {
	members []string // in order
	kind    valueKind
	kwInit  bool // Struct.new(..., keyword_init: true): new takes keywords only
}

// valueClass recognizes `Struct.new(...)` and `Data.define(...)`.
func valueClass(n parser.Node) (*parser.CallNode, valueKind) {
	call, ok := n.(*parser.CallNode)
	if !ok {
		return nil, valueStruct
	}
	r, ok := call.Receiver.(*parser.ConstantReadNode)
	switch {
	case !ok:
	case r.Name == "Struct" && call.Name == "new":
		return call, valueStruct
	case r.Name == "Data" && call.Name == "define":
		return call, valueData
	}
	return nil, valueStruct
}

// collectValueClass declares `Name = Struct.new(:a, :b) do ... end` or
// `Name = Data.define(...)` as a class. Member types come from a `#: T`
// after each member on its own line (rbs-inline's form), or a trailing
// `#: [A, B]` on the assignment. The generated members are written as Ruby
// and parsed; they and the block body live in the outer lexical scope, as
// a block does not open a constant scope in Ruby.
func (c *Compiler) collectValueClass(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, kind valueKind, scope []*Class) {
	c.collectValueClassAs(ctx, f, n, call, kind, scope, false)
}

// collectValueClassAs is collectValueClass; hidden: no constant names the class (a bare `Struct.new(:a)`, a `< Struct.new(:a)` super).
func (c *Compiler) collectValueClassAs(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, kind valueKind, scope []*Class, hidden bool) {
	members := make([]string, 0, len(callArgs(call)))
	syms := make([]parser.Node, 0, len(callArgs(call)))
	kwInit := "nil"
	for _, a := range callArgs(call) {
		if kw, ok := a.(*parser.KeywordHashNode); ok && kind == valueStruct && len(kw.Elements) == 1 { // `keyword_init: true` (decision 189)
			if as, ok := kw.Elements[0].(*parser.AssocNode); ok {
				if k, ok := as.Key.(*parser.SymbolNode); ok && k.Unescaped.Value == "keyword_init" {
					switch as.Value.(type) {
					case *parser.TrueNode:
						kwInit = "true"
					case *parser.FalseNode:
						kwInit = "false"
					case *parser.NilNode:
					default:
						c.errorf(f, a, "keyword_init must be true, false or nil")
					}
					continue
				}
			}
		}
		sym, ok := a.(*parser.SymbolNode)
		if !ok {
			c.errorf(f, a, "%s members must be symbols", kind)
		}
		if m := sym.Unescaped.Value; !isIdentMethod(m) || strings.HasSuffix(m, "?") || strings.HasSuffix(m, "!") { // it is spliced into generated Ruby (`@m`, `self.m`)
			c.errorf(f, a, "%s member :%q is not a valid member name (letters, digits and _ only)", kind, m)
		}
		members = append(members, sym.Unescaped.Value)
		syms = append(syms, a)
	}
	if len(syms) != len(callArgs(call)) {
		trimmed := *call
		trimmed.Arguments = &parser.ArgumentsNode{Location: call.Arguments.Location, Arguments: syms}
		call = &trimmed
	}
	types := c.memberTypes(f, n, call, members)
	full := qualify(scope, n.Name)
	line := f.line(n.Location.StartOffset)
	declare := c.declareClass
	if hidden {
		declare = c.declareHidden
	}
	cls := declare(f, full, line, false)
	cls.superRef = &constRef{node: &parser.ConstantReadNode{Name: kind.superName()}, file: f}
	cls.value = &valueSpec{members: members, kind: kind, kwInit: kwInit == "true"}
	label := "internal error: generated " + kind.String() + " does not parse"
	sf := c.synthFile(ctx, f, n, label, valueClassSource(kind, full, members, types, kwInit), synthOpt{Origin: f.synthOrigin(), Pad: line - 1, Pin: true, NoPath: true})
	sf.valueGen = true
	gen, ok := sf.Root.Statements.Body[0].(*parser.ClassNode)
	if !ok {
		c.errorf(f, n, "%s: no class generated", label)
	}
	c.collectBody(ctx, sf, cls, gen.Body, scope, false)
	for _, m := range cls.MethodList {
		m.valueGen = true
	}
	if bn, ok := call.Block.(*parser.BlockNode); ok && bn.Body != nil {
		gen := slices.Clone(cls.MethodList)
		c.collectBody(ctx, f, cls, bn.Body, scope, false)
		// MRI defines the accessors on the new class but the rest on
		// Struct/Data, so a block def overrides those and super reaches
		// them: keep each one under a hidden private name.
		for _, g := range gen {
			m := cls.Methods[g.Name]
			if m == g || g.Kind != kindDef {
				continue
			}
			m.structDef = g
			g.Name, g.GoName, g.Private = "__struct_"+g.Name, g.GoName+"_struct", true
			cls.Methods[g.Name] = g
			cls.MethodList = append(cls.MethodList, g)
		}
	}
}

// memberTypes reads member types: one `#: T` per member line, else a
// trailing `#: [A, B]` tuple on the assignment.
func (c *Compiler) memberTypes(f *File, n *parser.ConstantWriteNode, call *parser.CallNode, members []string) []rbs.Type {
	args := callArgs(call)
	perLine := true
	types := make([]rbs.Type, len(members))
	for i, a := range args {
		ln := f.line(a.GetLocation().StartOffset)
		t, ok := f.trailing[ln]
		if !ok || ln == f.line(n.Location.StartOffset) || (i > 0 && ln == f.line(args[i-1].GetLocation().StartOffset)) {
			perLine = false
			break
		}
		rt, err := rbs.ParseType(t)
		if err != nil {
			c.errorf(f, a, "%v", err)
		}
		types[i] = rt
	}
	if perLine {
		return types
	}
	ann := f.trailingAnnotation(n)
	if ann == "" { // untyped members, as gem code's (decision 172): rack's `MultipartInfo = Struct.new :params, :tmp_files`
		if !c.gemFile(f) {
			c.warn(f, n, "%s's members are untyped; give them types (`:a, #: T` per line, or `#: [A, B]` after the definition) for static calls (decision 179)", n.Name)
		}
		for i := range types {
			types[i] = rbs.Untyped{}
		}
		return types
	}
	t, err := rbs.ParseType(ann)
	tup, ok := t.(rbs.Tuple)
	if err != nil || !ok || len(tup.Elems) != len(members) {
		c.errorf(f, n, "`#: %s` must be a %d-element tuple of member types", ann, len(members))
	}
	return tup.Elems
}

// valueClassSource is the Ruby for a Struct's or Data's generated members.
func valueClassSource(kind valueKind, full string, members []string, types []rbs.Type, kwInit string) string {
	var b strings.Builder
	b.WriteString("class GeneratedValueClass\n")
	if kind == valueStruct {
		// MRI's Struct includes Enumerable over its values. Here each struct class does: through the Struct base, the
		// generated to_h's Symbol keys would fail the override check against Enumerable#to_h.
		if !slices.Contains(members, "each") && !slices.Contains(members, "to_a") { // Enumerable calls both on self, so a member of either name has no Enumerable (ponytail: MRI's would call the accessor)
			b.WriteString("  include Enumerable #[untyped]\n")
		}
		fmt.Fprintf(&b, "  #: () -> bool?\n  def self.keyword_init? = %s\n", kwInit)
	}
	for i, m := range members {
		if slices.Contains(members[:i], m) { // a duplicate member raises at run time; declare its accessors once
			continue
		}
		fmt.Fprintf(&b, "  attr_reader :%s #: %s\n", m, types[i])
		if kind == valueStruct { // a writer, not attr_writer: a frozen struct raises, as MRI's
			fmt.Fprintf(&b, "  #: (%s) -> %s\n  def %s=(__v)\n    raise FrozenError, \"can't modify frozen #{self.class}: #{inspect}\" if frozen?\n    @%s = __v\n  end\n", types[i], types[i], m, m)
		}
	}
	// A struct's trailing nilable members may be omitted (Ruby fills in
	// nil); every Data member is required.
	firstOpt := len(members)
	for kind == valueStruct && firstOpt > 0 && nilableRBS(types[firstOpt-1]) {
		firstOpt--
	}
	// Parameters are __v0, __v1, ... and members are read as self.m, so
	// a member may be named like a keyword (:end) or like a generated
	// parameter (:other).
	sigs := make([]string, len(members))
	params := make([]string, len(members))
	vars := make([]string, len(members))
	syms := make([]string, len(members))
	reads := make([]string, len(members))
	eqs := make([]string, len(members))
	insp := make([]string, len(members))
	pairs := make([]string, len(members))
	for i, m := range members {
		vars[i] = fmt.Sprintf("__v%d", i)
		sigs[i], params[i] = types[i].String(), vars[i]
		if i >= firstOpt {
			sigs[i], params[i] = "?"+sigs[i], vars[i]+" = nil"
		}
		syms[i] = ":" + m
		reads[i] = "self." + m
		eqs[i] = "self." + m + " == __other." + m
		insp[i] = m + "=#{self." + m + ".inspect}"
		pairs[i] = m + ": self." + m
	}
	fmt.Fprintf(&b, "  #: (%s) -> void\n  def initialize(%s)\n", strings.Join(sigs, ", "), strings.Join(params, ", "))
	for i, m := range members {
		fmt.Fprintf(&b, "    @%s = %s\n", m, vars[i])
	}
	b.WriteString("  end\n")
	fmt.Fprintf(&b, "  #: () -> Array[Symbol]\n  def self.members = [%s]\n", strings.Join(syms, ", "))
	// a member's accessor wins over a same-named helper, as in MRI; generated code calls the always-emitted `__` twins
	pub := func(name, src string) {
		if !slices.Contains(members, name) {
			b.WriteString(src)
		}
	}
	fmt.Fprintf(&b, "  #: () -> Array[Symbol]\n  def __members = [%s]\n", strings.Join(syms, ", "))
	pub("members", "  #: () -> Array[Symbol]\n  def members = __members\n")
	fmt.Fprintf(&b, "  #: () -> Hash[Symbol, untyped]\n  def __to_h = { %s }\n", strings.Join(pairs, ", "))
	pub("to_h", "  #: () -> Hash[Symbol, untyped]\n  def to_h = __to_h\n")
	// MRI's ==/eql? want the same class, not a subclass
	if len(eqs) == 0 {
		eqs = []string{"true"}
	}
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def ==(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    %s\n  end\n", full, strings.Join(eqs, " && "))
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def eql?(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    __to_h.eql?(__other.__to_h)\n  end\n", full)
	pub("hash", "  #: () -> Integer\n  def hash = __to_h.hash\n")
	fields := ""
	if len(insp) > 0 {
		fields = " " + strings.Join(insp, ", ")
	}
	fmt.Fprintf(&b, "  #: () -> String\n  def __inspect = \"#<%s #{__class_name}%s>\"\n", kind, fields) // the class's own name: MRI does not call an overridden `name`
	pub("inspect", "  #: () -> String\n  def inspect = __inspect\n")
	pub("to_s", "  #: () -> String\n  def to_s = __inspect\n") // MRI's to_s is inspect's C function, not a call of the method
	// pp's Struct/Data layout (decision 113): the kind and the values, in members order
	fmt.Fprintf(&b, "  #: () -> String\n  def __pp_kind = %q\n", kind)
	fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def __pp_values = [%s]\n", strings.Join(reads, ", "))
	b.WriteString("  #: () -> Array[Symbol]\n  def __pp_members = __members\n")
	// MRI's deconstruct_keys: every member for nil, {} for more keys than members, else the requested keys up to the first non-member (decision 143)
	whens := make([]string, len(members))
	for i, m := range members {
		whens[i] = fmt.Sprintf("      when %s then __h[%s] = self.%s\n", syms[i], syms[i], m)
	}
	if len(members) == 0 {
		whens = []string{"      when nil then return __h\n"} // no member names: the first key ends it
	}
	pub("deconstruct_keys", fmt.Sprintf("  #: (Array[Symbol]?) -> Hash[Symbol, untyped]\n  def deconstruct_keys(__keys)\n    return __to_h if __keys.nil?\n"+
		"    __h = {} #: Hash[Symbol, untyped]\n    return __h if __keys.size > %d\n    __i = 0\n    while __i < __keys.size\n      case __keys.fetch(__i)\n%s      else return __h\n      end\n      __i += 1\n    end\n    __h\n  end\n",
		len(members), strings.Join(whens, "")))
	if kind == valueStruct {
		fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def __to_a = [%s]\n", strings.Join(reads, ", "))
		pub("to_a", "  #: () -> Array[untyped]\n  def to_a = __to_a\n")
		pub("values", "  #: () -> Array[untyped]\n  def values = __to_a\n")
		pub("deconstruct", "  #: () -> Array[untyped]\n  def deconstruct = __to_a\n")
		pub("size", fmt.Sprintf("  #: () -> Integer\n  def size = %d\n", len(members)))
		pub("length", fmt.Sprintf("  #: () -> Integer\n  def length = %d\n", len(members)))
		pub("each", "  #: () { (untyped) -> void } -> void\n  def each\n    __to_a.each { |__x| yield __x }\n  end\n")
		pub("each_pair", "  #: () { (Symbol, untyped) -> void } -> void\n  def each_pair\n    __to_h.each { |__k, __x| yield __k, __x }\n  end\n")
		// [] and []= take a member's index, Symbol or String, with MRI's errors
		gets, sets := make([]string, 0, len(members)+1), make([]string, 0, len(members)+1)
		for _, m := range members {
			gets = append(gets, fmt.Sprintf("    when %q then self.%s\n", m, m))
			sets = append(sets, fmt.Sprintf("    when %q then self.%s = __v\n", m, m))
		}
		if len(members) == 0 {
			gets, sets = []string{"    when nil then nil\n"}, []string{"    when nil then nil\n"} // a case needs a when
		}
		index := fmt.Sprintf("      __i = __k < 0 ? __k + %d : __k\n      raise IndexError, \"offset #{__k} too small for struct(size:%d)\" if __i < 0\n      raise IndexError, \"offset #{__k} too large for struct(size:%d)\" if __i >= %d\n", len(members), len(members), len(members), len(members))
		fmt.Fprintf(&b, "  #: (Integer | Symbol | String) -> untyped\n  def [](__k)\n    if __k.is_a?(Integer)\n%s      return __to_a.fetch(__i)\n    end\n    case __k.to_s\n%s    else raise NameError, \"no member '#{__k}' in struct\"\n    end\n  end\n", index, strings.Join(gets, ""))
		fmt.Fprintf(&b, "  #: (Integer | Symbol | String, untyped) -> untyped\n  def []=(__k, __v)\n    __name = __k.to_s\n    if __k.is_a?(Integer)\n%s      __name = __members.fetch(__i).to_s\n    end\n    case __name\n%s    else raise NameError, \"no member '#{__k}' in struct\"\n    end\n    __v\n  end\n", index, strings.Join(sets, ""))
		extra := pub
		extra("values_at", "  #: (*Integer) -> Array[untyped]\n  def values_at(*__is) = __is.map { |__i| self[__i] }\n")
		extra("dig", "  # @dynamic\n  #: (Integer | Symbol | String, *untyped) -> untyped\n  def dig(__k, *__rest)\n    __v = self[__k]\n    return __v if __rest.empty? || __v.nil?\n    return __v.dig(__rest.fetch(0)) if __rest.size == 1\n    __v.dig(__rest.fetch(0), __rest.fetch(1))\n  end\n") // ponytail: no splat into a dynamic call, so dig goes 3 deep; build the call per depth when deeper is needed
		extra("select", "  #: () { (untyped) -> bool } -> Array[untyped]\n  def select(&__blk) = __to_a.select(&__blk)\n")
		extra("filter", "  #: () { (untyped) -> bool } -> Array[untyped]\n  def filter(&__blk) = __to_a.select(&__blk)\n")
	} else {
		pub("deconstruct", fmt.Sprintf("  #: () -> Array[untyped]\n  def deconstruct = [%s]\n", strings.Join(reads, ", ")))
		// `with(k: v)` compiles to this: a copy of the receiver's class
		fmt.Fprintf(&b, "  #: (%s) -> ::%s\n  def __with(%s) = self.class.new(%s)\n", strings.Join(sigs, ", "), full, strings.Join(vars, ", "), strings.Join(vars, ", "))
	}
	b.WriteString("end\n")
	return b.String()
}

func nilableRBS(t rbs.Type) bool {
	switch t.(type) {
	case rbs.Optional, rbs.Nil, rbs.Untyped:
		return true
	case rbs.Bool, rbs.Bot, rbs.Intersection, rbs.Name, rbs.Proc, rbs.Self, rbs.Instance, rbs.Singleton, rbs.Tuple, rbs.Union, rbs.Void, rbs.Literal, rbs.Record: // never holds nil
	}
	return false
}

// valueRoot finds the Struct/Data class that declared cls's members.
func (c *Class) valueRoot() *Class {
	for k := c; k != nil; k = k.Super {
		if k.value != nil {
			return k
		}
	}
	return nil
}
