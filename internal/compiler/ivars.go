package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ivarKey names one class's ivar in the discovery-time sets (nilIvars, ivarReadFirst).
type ivarKey struct {
	cls  *Class
	name string
}

// declareIvarAnnotations declares each `# @rbs @x: T`, and each
// `# @rbs self.@x: T` on the class object.
func (c *Compiler) declareIvarAnnotations() {
	for _, cls := range c.classList {
		for _, d := range cls.ivarDecls {
			t := c.resolveType(d.rbs, typeScope{class: cls, lex: d.scope, srcPos: srcPos{cls.File, d.line}})
			owner, name := cls, d.name
			if iv, ok := strings.CutPrefix(name, "self."); ok {
				owner, name = cls.meta, iv
			}
			c.declareIvar(owner, name, t, cls.File, d.line)
		}
	}
}

// declareIvar records an ivar on the topmost class of the chain that
// mentions it, so subclasses share the parent's field.
func (c *Compiler) declareIvar(cls *Class, name string, t Type, f *File, line int) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			narrows := isAny(iv.Type) || isNil(t) && holdsNil(iv.Type) || unionHas(iv.Type, t) // a gem subclass's reader narrowing the parent's (`returns(String)` over Integer | String | nil)
			if !typeEq(iv.Type, t) && (!narrows || !c.gemFile(f)) {
				c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s has it as %s", name, t, k.Name, iv.Type)
			}
			return iv
		}
	}
	for _, mod := range ivarModules(cls) { // an included module's (decision 147): Homebrew's Downloadable `attr_reader :checksum`
		if iv := mod.Ivars[name]; iv != nil { // and Resource's `attr_writer :checksum` are one @checksum, as one object's are in MRI
			if !typeEq(iv.Type, t) && !isAny(iv.Type) && !c.gemFile(f) {
				c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s has it as %s", name, t, mod.Name, iv.Type)
			}
			return iv
		}
	}
	if c.nilIvars[ivarKey{cls, name}] {
		t = optOf(t) // this class assigns it nil too
	}
	for k := cls.Super; k != nil; k = k.Super { // an ancestor that assigned it only nil owns it, nilable (decision 182)
		if c.nilIvars[ivarKey{k, name}] {
			cls, t = k, optOf(t)
		}
	}
	if f != nil && c.gemFile(f) {
		for k := cls.Super; k != nil; k = k.Super { // a gem ancestor's attr_reader of it owns it: Formula's `class << self; attr_reader :loaded_from_api` (decision 259)
			if m := k.Methods[strings.TrimPrefix(name, "@")]; m != nil && (m.Kind == kindAttrReader && m.Attr == name || readsOnlyIvar(m, name)) {
				cls, t = k, optOf(t)
			}
		}
	}
	iv := &Ivar{Name: name, Type: t, Owner: cls}
	cls.Ivars[name] = iv
	cls.IvarList = append(cls.IvarList, iv)
	return iv
}

// subclassIvar moves an ivar a subclass declared up to cls, nilable, when cls assigns it only nil: one field holds it
// for the whole hierarchy, as one object's ivar is the same name in every method MRI runs on it.
func (c *Compiler) subclassIvar(cls *Class, name string) *Ivar {
	for _, sub := range cls.Subclasses {
		iv := sub.Ivars[name]
		if iv == nil {
			if moved := c.subclassIvar(sub, name); moved != nil {
				iv = moved
			} else {
				continue
			}
		}
		delete(iv.Owner.Ivars, name)
		iv.Owner.IvarList = slices.DeleteFunc(iv.Owner.IvarList, func(x *Ivar) bool { return x == iv })
		iv.Owner, iv.Type = cls, optOf(iv.Type)
		cls.Ivars[name] = iv
		cls.IvarList = append(cls.IvarList, iv)
		return iv
	}
	return nil
}

func (c *Compiler) findIvar(cls *Class, name string) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			return iv
		}
	}
	for _, mod := range ivarModules(cls) { // an included module's (decision 147)
		if iv := mod.Ivars[name]; iv != nil {
			return iv
		}
	}
	return nil
}

// declareComputedIvars declares, untyped on the method's class, each ivar a gem method sets by a name built from one
// of its parameters (`instance_variable_set("@tl_#{sym}", v)`, bindata's top_level_set) when every call of a method
// by that name on self passes a Symbol or String literal there: the names are those literals behind the prefix (decision 324).
func (c *Compiler) declareComputedIvars() {
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) || m.Node.Parameters == nil {
				continue
			}
			anyNode(m.Node.Body, func(n parser.Node) bool {
				prefix, param, ok := computedIvarSet(m, n)
				if !ok {
					return false
				}
				names, ok := c.literalSelfArgs(m.Name, param)
				for _, name := range names {
					if ok && c.findIvar(cls, prefix+name) == nil {
						c.declareIvar(cls, prefix+name, TAny{}, m.File, m.File.line(m.Node.Location.StartOffset))
					}
				}
				return false
			})
		}
	}
}

// computedIvarSet matches `instance_variable_set("@prefix#{param}", v)` in m: the prefix and the parameter's index.
func computedIvarSet(m *Method, n parser.Node) (string, int, bool) {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "instance_variable_set" || len(callArgs(call)) != 2 {
		return "", 0, false
	}
	str, ok := callArgs(call)[0].(*parser.InterpolatedStringNode)
	if !ok || len(str.Parts) != 2 {
		return "", 0, false
	}
	lit, ok1 := str.Parts[0].(*parser.StringNode)
	emb, ok2 := str.Parts[1].(*parser.EmbeddedStatementsNode)
	if !ok1 || !ok2 || emb.Statements == nil || len(emb.Statements.Body) != 1 || !strings.HasPrefix(lit.Unescaped.Value, "@") {
		return "", 0, false
	}
	rd, ok := emb.Statements.Body[0].(*parser.LocalVariableReadNode)
	if !ok || rd.Depth != 0 {
		return "", 0, false
	}
	for i, p := range m.Node.Parameters.Requireds {
		if r, ok := p.(*parser.RequiredParameterNode); ok && r.Name == rd.Name {
			return lit.Unescaped.Value, i, true
		}
	}
	return "", 0, false
}

// literalSelfArgs is the Symbol or String literal every receiverless (or self) call of name passes at position param;
// false when one passes anything else.
func (c *Compiler) literalSelfArgs(name string, param int) ([]string, bool) {
	var names []string
	ok := true
	for _, f := range c.files {
		anyNode(f.Root, func(n parser.Node) bool {
			call, isCall := n.(*parser.CallNode)
			if !isCall || call.Name != name {
				return false
			}
			if _, self := call.Receiver.(*parser.SelfNode); call.Receiver != nil && !self { // another object's method of that name
				return false
			}
			lit := ""
			if args := callArgs(call); param < len(args) {
				lit = literalName(args[param])
			}
			if lit == "" {
				ok = false
				return true
			}
			names = append(names, lit)
			return false
		})
	}
	return names, ok
}
