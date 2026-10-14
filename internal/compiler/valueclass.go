package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// valueClass recognizes `Struct.new(...)` and `Data.define(...)`.
func valueClass(n parser.Node) (*parser.CallNode, string) {
	call, ok := n.(*parser.CallNode)
	if !ok {
		return nil, ""
	}
	r, ok := call.Receiver.(*parser.ConstantReadNode)
	switch {
	case !ok:
	case r.Name == "Struct" && call.Name == "new":
		return call, "struct"
	case r.Name == "Data" && call.Name == "define":
		return call, "data"
	}
	return nil, ""
}

// collectValueClass declares `Name = Struct.new(:a, :b) do ... end` or
// `Name = Data.define(...)` as a class. Member types come from a `#: T`
// after each member on its own line (rbs-inline's form), or a trailing
// `#: [A, B]` on the assignment. The generated members are written as Ruby
// and parsed; they and the block body live in the outer lexical scope, as
// a block does not open a constant scope in Ruby.
func (c *Compiler) collectValueClass(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, kind string, scope []*Class) {
	members := make([]string, 0, len(callArgs(call)))
	syms := make([]parser.Node, 0, len(callArgs(call)))
	kwInit := "nil"
	for _, a := range callArgs(call) {
		if kw, ok := a.(*parser.KeywordHashNode); ok && kind == "struct" && len(kw.Elements) == 1 { // `keyword_init: true` (decision 189)
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
	cls := c.declareClass(f, full, line, false)
	cls.superRef = &constRef{node: &parser.ConstantReadNode{Name: map[string]string{"struct": "Struct", "data": "Data"}[kind]}, file: f}
	cls.valueMembers, cls.valueKind, cls.kwInit = members, kind, kwInit == "true"
	// pad so the generated lines report the declaration's line
	src := strings.Repeat("\n", line-1) + valueClassSource(kind, full, members, types, kwInit)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.prelude)
	if err != nil {
		c.errorf(f, n, "internal error: generated %s does not parse: %v", kind, err)
	}
	sf.valueGen, sf.pinLine = true, line
	c.collectBody(ctx, sf, cls, sf.Root.Statements.Body[0].(*parser.ClassNode).Body, scope, false)
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
func valueClassSource(kind, full string, members []string, types []rbs.Type, kwInit string) string {
	var b strings.Builder
	b.WriteString("class GeneratedValueClass\n")
	if kind == "struct" {
		fmt.Fprintf(&b, "  #: () -> bool?\n  def self.keyword_init? = %s\n", kwInit)
	}
	accessor := map[string]string{"struct": "attr_accessor", "data": "attr_reader"}[kind]
	for i, m := range members {
		fmt.Fprintf(&b, "  %s :%s #: %s\n", accessor, m, types[i])
	}
	// A struct's trailing nilable members may be omitted (Ruby fills in
	// nil); every Data member is required.
	firstOpt := len(members)
	for kind == "struct" && firstOpt > 0 && nilableRBS(types[firstOpt-1]) {
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
	fmt.Fprintf(&b, "  #: () -> Array[Symbol]\n  def members = [%s]\n", strings.Join(syms, ", "))
	fmt.Fprintf(&b, "  #: () -> Hash[Symbol, untyped]\n  def to_h = { %s }\n", strings.Join(pairs, ", "))
	// MRI's ==/eql? want the same class, not a subclass
	if len(eqs) == 0 {
		eqs = []string{"true"}
	}
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def ==(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    %s\n  end\n", full, strings.Join(eqs, " && "))
	fmt.Fprintf(&b, "  #: (untyped) -> bool\n  def eql?(__other)\n    return false unless __other.is_a?(::%s)\n    return false unless __other.class.equal?(self.class)\n    to_h.eql?(__other.to_h)\n  end\n", full)
	b.WriteString("  #: () -> Integer\n  def hash = to_h.hash\n")
	fields := ""
	if len(insp) > 0 {
		fields = " " + strings.Join(insp, ", ")
	}
	fmt.Fprintf(&b, "  #: () -> String\n  def inspect = \"#<%s #{self.class.name}%s>\"\n", kind, fields)
	b.WriteString("  #: () -> String\n  def to_s = inspect\n")
	// pp's Struct/Data layout (decision 113): the kind and the values, in members order
	fmt.Fprintf(&b, "  #: () -> String\n  def __pp_kind = %q\n", kind)
	fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def __pp_values = [%s]\n", strings.Join(reads, ", "))
	// MRI's deconstruct_keys: every member for nil, {} for more keys than members, else the requested keys up to the first non-member (decision 143)
	whens := make([]string, len(members))
	for i, m := range members {
		whens[i] = fmt.Sprintf("      when %s then __h[%s] = self.%s\n", syms[i], syms[i], m)
	}
	if len(members) == 0 {
		whens = []string{"      when nil then return __h\n"} // no member names: the first key ends it
	}
	fmt.Fprintf(&b, "  #: (Array[Symbol]?) -> Hash[Symbol, untyped]\n  def deconstruct_keys(__keys)\n    return to_h if __keys.nil?\n"+
		"    __h = {} #: Hash[Symbol, untyped]\n    return __h if __keys.size > %d\n    __i = 0\n    while __i < __keys.size\n      case __keys.fetch(__i)\n%s      else return __h\n      end\n      __i += 1\n    end\n    __h\n  end\n",
		len(members), strings.Join(whens, ""))
	if kind == "struct" {
		fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def to_a = [%s]\n", strings.Join(reads, ", "))
		b.WriteString("  #: () -> Array[untyped]\n  def values = to_a\n")
		b.WriteString("  #: () -> Array[untyped]\n  def deconstruct = to_a\n")
		fmt.Fprintf(&b, "  #: () -> Integer\n  def size = %d\n", len(members))
		fmt.Fprintf(&b, "  #: () -> Integer\n  def length = %d\n", len(members))
		b.WriteString("  #: () { (untyped) -> void } -> void\n  def each\n    to_a.each { |__x| yield __x }\n  end\n")
		b.WriteString("  #: () { (Symbol, untyped) -> void } -> void\n  def each_pair\n    to_h.each { |__k, __x| yield __k, __x }\n  end\n")
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
		fmt.Fprintf(&b, "  #: (Integer | Symbol | String) -> untyped\n  def [](__k)\n    if __k.is_a?(Integer)\n%s      return to_a.fetch(__i)\n    end\n    case __k.to_s\n%s    else raise NameError, \"no member '#{__k}' in struct\"\n    end\n  end\n", index, strings.Join(gets, ""))
		fmt.Fprintf(&b, "  #: (Integer | Symbol | String, untyped) -> untyped\n  def []=(__k, __v)\n    __name = __k.to_s\n    if __k.is_a?(Integer)\n%s      __name = members.fetch(__i).to_s\n    end\n    case __name\n%s    else raise NameError, \"no member '#{__k}' in struct\"\n    end\n    __v\n  end\n", index, strings.Join(sets, ""))
		extra := func(name, src string) { // a member's accessor wins over a helper of its name (`:select`)
			if !slices.Contains(members, name) {
				b.WriteString(src)
			}
		}
		extra("values_at", "  #: (*Integer) -> Array[untyped]\n  def values_at(*__is) = __is.map { |__i| self[__i] }\n")
		extra("dig", "  # @dynamic\n  #: (Integer | Symbol | String, *untyped) -> untyped\n  def dig(__k, *__rest)\n    __v = self[__k]\n    return __v if __rest.empty? || __v.nil?\n    return __v.dig(__rest.fetch(0)) if __rest.size == 1\n    __v.dig(__rest.fetch(0), __rest.fetch(1))\n  end\n") // ponytail: no splat into a dynamic call, so dig goes 3 deep; build the call per depth when deeper is needed
		extra("select", "  #: () { (untyped) -> bool } -> Array[untyped]\n  def select(&__blk) = to_a.select(&__blk)\n")
		extra("filter", "  #: () { (untyped) -> bool } -> Array[untyped]\n  def filter(&__blk) = to_a.select(&__blk)\n")
	} else {
		fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def deconstruct = [%s]\n", strings.Join(reads, ", "))
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
	case rbs.Bool, rbs.Bot, rbs.Name, rbs.Proc, rbs.Self, rbs.Instance, rbs.Singleton, rbs.Tuple, rbs.Union, rbs.Void, rbs.Literal, rbs.Record: // never holds nil
	}
	return false
}

// valueRoot finds the Struct/Data class that declared cls's members.
func (c *Class) valueRoot() *Class {
	for k := c; k != nil; k = k.Super {
		if k.valueMembers != nil {
			return k
		}
	}
	return nil
}
