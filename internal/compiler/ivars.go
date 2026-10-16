package compiler

import (
	"slices"
	"strings"
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
