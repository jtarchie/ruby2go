package compiler

import (
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"strings"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// Generic struct classes are monomorphized (decision 308): a template is never emitted, each closed instantiation is a cloned struct class.

// errNeedInstances: a compile named an instantiation it did not create before link; compile runs again with it.
var errNeedInstances = errors.New("generic instances to create")

// needInstances is the panic behind errNeedInstances.
type needInstances struct{}

// maxInstanceNesting: deeper type arguments are taken for polymorphic recursion (`Box[T]` naming `Box[Array[T]]`), which needs instances without end.
const maxInstanceNesting = 8

// isTemplate reports a generic struct class: instantiated per type arguments, never emitted itself.
func (c *Class) isTemplate() bool { return c.tmpl }

// markTemplates finds the templates before link: generic struct classes whose superclass is a struct class, and the
// unannotated helper classes nested in one, which take its parameters (decision 311).
func (c *Compiler) markTemplates() {
	defer c.markHelpers()
	for _, cls := range c.classList {
		if len(cls.TypeParams) == 0 || !cls.isStruct() || cls.universal || cls.metaOf != nil || cls.template != nil {
			continue
		}
		cls.tmpl = cls.superRef == nil
		if cls.superRef != nil {
			sup, _ := c.lookupConst(cls.superRef.file, cls.superRef.node, cls.superRef.scope)
			cls.tmpl = sup != nil && sup.isStruct() && !sup.IsModule && (sup.universal || sup.tmpl || len(sup.TypeParams) == 0)
		}
	}
}

// markHelpers makes each helper of a template a template over the same parameters: a plain struct class declared
// directly in the template's body, with no superclass, no type parameters, and no signature on any method.
func (c *Compiler) markHelpers() {
	for _, cls := range c.classList {
		i := strings.LastIndex(cls.RubyName, "::")
		if i < 0 || cls.tmpl || cls.template != nil || len(cls.TypeParams) > 0 || !cls.isStruct() || cls.universal || cls.metaOf != nil || cls.superRef != nil || cls.File.isPrelude() || len(cls.singletonDefs) > 0 || c.sigs[cls.RubyName] != nil {
			continue
		}
		outer := c.classes[cls.RubyName[:i]]
		if outer == nil || !outer.tmpl || outer.helperOf != nil || slices.ContainsFunc(cls.MethodList, func(m *Method) bool { return m.declared }) {
			continue
		}
		cls.tmpl, cls.helperOf, cls.TypeParams = true, outer, slices.Clone(outer.TypeParams)
	}
}

// helperArgs are the arguments a helper template takes where k's code names it: those of the instance of its outer
// class (or of the helper itself) along k's superclasses; nil outside any.
func helperArgs(helper, k *Class) []Type {
	for ; k != nil; k = k.Super {
		if k.template != nil && (k.template == helper.helperOf || k.template == helper) {
			return k.instArgs
		}
	}
	return nil
}

// linkSuperInst makes the superclass instance cls's superArgs name, in cls's scope (an instance's resolves its own parameters).
func (c *Compiler) linkSuperInst(cls *Class) {
	if len(cls.superArgs) == 0 || cls.tmpl || cls.superRef == nil {
		return
	}
	sup, _ := c.lookupConst(cls.superRef.file, cls.superRef.node, cls.superRef.scope)
	if sup == nil || !sup.tmpl {
		return
	}
	t := c.resolveType(rbs.Name{Name: sup.RubyName, Args: cls.superArgs, Absolute: true}, typeScope{class: cls, lex: cls.superRef.scope, srcPos: cls.srcPos})
	if tc, ok := t.(TClass); ok && tc.C.template == sup {
		cls.superInst = tc.C
	}
}

// instKey names an instantiation as RBS: the key instances are memoized and requested by.
func instKey(tmpl *Class, args []Type) string { return tmpl.RubyName + "[" + joinTypes(args) + "]" }

// instOf is tmpl at closed args; one requested after makeInstances is nil, and the compile restarts with it (instMissing).
func (c *Compiler) instOf(tmpl *Class, args []Type, at srcPos) *Class {
	key := instKey(tmpl, args)
	if inst := c.insts[key]; inst != nil {
		return inst
	}
	if typeDepth(TClass{C: tmpl, Args: args}) > maxInstanceNesting {
		c.errorAt(at, "%s instantiates %s without end (polymorphic recursion): a generic class's methods may not name it at type arguments built from its own", key, tmpl.RubyName)
	}
	if _, seen := c.instAt[key]; !seen && at.File != nil {
		c.instAt[key] = at // where checkBounds reports it, once link has the classes' ancestors
	}
	if !c.instOpen { // typed as the template until the restart, so one pass finds every sibling instance
		if !slices.Contains(*c.instReqs, key) {
			*c.instReqs = append(*c.instReqs, key)
		}
		c.instMissing = true
		return nil
	}
	return c.cloneTemplate(tmpl, args)
}

// checkInstanceBounds runs checkBounds on every instance, in link once supers and includes are resolved.
func (c *Compiler) checkInstanceBounds() {
	for _, cls := range c.classList {
		if t := cls.template; t != nil {
			c.checkBounds(t, cls.instArgs, c.instAt[instKey(t, cls.instArgs)])
		}
	}
}

// checkBounds rejects, in user code, an instance whose argument is not a subclass of its parameter's bound (`P <
// Comparable`): nil and untyped pass, as do a union's members each satisfying it. Gem code is not checked (its sigs
// bound what MRI never checks).
func (c *Compiler) checkBounds(tmpl *Class, args []Type, at srcPos) {
	if c.gemFile(tmpl.File) {
		return
	}
	for i, p := range tmpl.TypeParams {
		name := tmpl.bounds[p]
		if name == "" {
			continue
		}
		bound, _ := c.lookupName(c.nestingOf(tmpl), name)
		if bound == nil {
			c.errorAt(tmpl.srcPos, "unknown bound %s of %s's type parameter %s", name, tmpl.RubyName, p)
		}
		var bad Type
		var check func(Type)
		check = func(t Type) {
			switch t := t.(type) {
			case TClass:
				if !t.C.isSubclassOf(bound) {
					bad = t
				}
			case TUnion:
				for _, m := range t.Members {
					check(m)
				}
			case TOpt:
				check(t.Elem)
			case TTuple, TFunc:
				if !bound.universal {
					bad = t
				}
			case TAny, TNil, TVar, TVoid, nil:
			}
		}
		if check(args[i]); bad != nil {
			c.errorAt(at, "%s: %s is not a %s, the bound of %s's %s", instKey(tmpl, args), bad, bound.RubyName, tmpl.RubyName, p)
		}
	}
}

// maxInstanceRestarts bounds the compiles one program may take to find its instances: each finds every instance its pass names.
const maxInstanceRestarts = 32

// typeDepth is how deeply t nests type arguments.
func typeDepth(t Type) int {
	d := 0
	switch t := t.(type) {
	case TClass:
		for _, a := range t.Args {
			d = max(d, typeDepth(a))
		}
		for _, a := range t.C.instArgs {
			d = max(d, typeDepth(a))
		}
		if len(t.Args)+len(t.C.instArgs) > 0 {
			d++
		}
	case TOpt:
		d = typeDepth(t.Elem)
	case TTuple:
		for _, e := range t.Elems {
			d = max(d, typeDepth(e))
		}
		d++
	case TUnion:
		for _, e := range t.Members {
			d = max(d, typeDepth(e))
		}
	case TFunc:
		for _, p := range t.Params {
			d = max(d, typeDepth(p))
		}
		d = max(d, typeDepth(t.Ret)) + 1
	case TAny, TNil, TVar, TVoid, nil:
	}
	return d
}

// makeInstances creates the instances an earlier pass of this compile requested, before link.
func (c *Compiler) makeInstances(reqs *[]string, at map[string]srcPos) {
	c.insts, c.instReqs, c.instAt, c.instOpen = map[string]*Class{}, reqs, at, true
	c.markTemplates()
	for _, cls := range slices.Clone(c.classList) {
		c.linkSuperInst(cls) // `class IntBox < Box #[Integer]`
	}
	for i := 0; i < len(*reqs); i++ { //nolint:intrange // creating one may request its arguments' instances, which append
		t, err := rbs.ParseType("::" + (*reqs)[i])
		if err != nil {
			c.errorf(nil, nil, "generic instance %s: %v", (*reqs)[i], err)
		}
		_ = c.resolveType(t, typeScope{})
		if c.instMissing {
			c.errorf(nil, nil, "generic instance %s: not created", (*reqs)[i])
		}
	}
	c.instOpen = false
}

// cloneTemplate copies tmpl's collected declarations into a new class bound to args: link resolves the copies'
// signatures and ivars with tmpl's parameters bound (resolveNameType), as for any class.
func (c *Compiler) cloneTemplate(tmpl *Class, args []Type) *Class {
	key := instKey(tmpl, args)
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	inst := &Class{Name: fmt.Sprintf("%s_G%08x", tmpl.Name, h.Sum32()), RubyName: tmpl.RubyName, template: tmpl, instArgs: args,
		Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, srcPos: tmpl.srcPos, superRef: tmpl.superRef,
		ivarDecls: tmpl.ivarDecls, privateNew: tmpl.privateNew, undefs: tmpl.undefs, delegations: tmpl.delegations, superArgs: tmpl.superArgs}
	for _, inc := range tmpl.Includes {
		inc.Mod, inc.Args = nil, nil
		inst.Includes = append(inst.Includes, inc)
	}
	for _, m := range tmpl.MethodList {
		mm := *m
		mm.Owner = inst
		mm.pendingIdx = maps.Clone(m.pendingIdx)
		inst.MethodList = append(inst.MethodList, &mm)
		inst.Methods[mm.Name] = &mm
	}
	for _, iv := range tmpl.IvarList {
		cp := *iv
		cp.Owner = inst
		inst.IvarList = append(inst.IvarList, &cp)
		inst.Ivars[cp.Name] = &cp
	}
	for _, a := range c.inheritedAliases {
		if a.cls == tmpl {
			a.cls = inst
			c.inheritedAliases = append(c.inheritedAliases, a)
		}
	}
	tmpl.instances = append(tmpl.instances, inst)
	c.insts[key] = inst // before its superclass's, which may name it back
	c.linkSuperInst(inst)
	c.classList = append(c.classList, inst) // after its superclass's instance: ivar discovery visits a parent's writes first, as MRI defines it first (decision 345)
	return inst
}
